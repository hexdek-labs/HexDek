import os, re, collections

LOG   = "/home/josh/hexdek/server.log"
DECKS = "/home/josh/hexdek/data/decks"

SEVEN = [
    "Chandra, Hope's Beacon",
    "Huatli, Poet of Unity",
    "Mana Bloom",
    "Night of the Sweets' Revenge",
    "Ramos, Dragon Engine",
    "Vivi Ornitier",
    "Wall of Roots",
]
SEVEN_L = [s.lower() for s in SEVEN]
LOOP_DECK = "sorin_of_house_markov_sorin_ravenous_neonate_b2_khazad_doom_JfUAlsri"

# ---- 1. index every deck file, record which of the seven it holds -------
deck_cards = {}          # "owner/stem" -> set(indices into SEVEN)
for root, _dirs, files in os.walk(DECKS):
    owner = os.path.basename(root)
    for fn in files:
        if not fn.endswith(".txt"):
            continue
        stem = fn[:-4]
        try:
            body = open(os.path.join(root, fn), errors="replace").read().lower()
        except Exception:
            continue
        hits = {i for i, c in enumerate(SEVEN_L) if c in body}
        deck_cards[f"{owner}/{stem}"] = hits

pool_total = len(deck_cards)
pool_with  = sum(1 for v in deck_cards.values() if v)
print(f"DECK POOL: {pool_total} deck files on disk")
print(f"  containing >=1 of the seven free-mana cards: {pool_with}  ({100.0*pool_with/pool_total:.2f}%)")
per_card_pool = collections.Counter()
for v in deck_cards.values():
    for i in v:
        per_card_pool[i] += 1
for i, c in sorted(per_card_pool.items(), key=lambda kv: -kv[1]):
    print(f"     {c:5d}  {SEVEN[i]}")
if not per_card_pool:
    print("     (none)")
print()

# ---- 2. walk every SLOW GAME line --------------------------------------
# r184 — THIS REGEX HAD BEEN DEAD SINCE r142 AND THE SCRIPT SILENTLY
# MATCHED NOTHING ON EVERY CURRENT LOG. It pinned `turns=(\d+) decks=`
# as adjacent, and two later changes inserted fields between them:
# r142 added " s/turn=" and r170 added " seed=". The guard below then
# fired SystemExit on every run, so this was an audit that could only
# ever report failure. Tolerant `.*?` between the two anchors now, so a
# third insertion does not kill it again — the fragile part was the
# adjacency assumption, not any one field.
#
# `swallowed=` (r184) is the per-game depth-cap swallow count, optional
# because logs written by an older binary do not carry it. None means
# "this build did not report it"; 0 means "this game swallowed nothing".
# Those are different facts and the script must not conflate them.
line_re = re.compile(
    r"SLOW GAME ([0-9.]+)s turns=(\d+).*? decks=\[([^\]]*)\]"
    r"(?: swallowed=(\d+|n/a))?"
)
slow = []
unresolved = collections.Counter()
for L in open(LOG, errors="replace"):
    m = line_re.search(L)
    if m:
        sw = m.group(4)
        sw = None if sw in (None, "n/a") else int(sw)
        slow.append((float(m.group(1)), int(m.group(2)), m.group(3).split(), sw))

print(f"SLOW GAMES in server.log: {len(slow)}")
if not slow:
    raise SystemExit("no slow games parsed — the regex or the log is wrong")

games_with_seven = 0
games_with_loop  = 0
slot_total = 0
slot_with_seven = 0
for _dur, _turns, decks, _sw in slow:
    anyseven = False
    for d in decks:
        slot_total += 1
        if d in deck_cards:
            if deck_cards[d]:
                slot_with_seven += 1
                anyseven = True
        else:
            unresolved[d] += 1
        if LOOP_DECK in d:
            pass
    if anyseven:
        games_with_seven += 1
    if any(LOOP_DECK in d for d in decks):
        games_with_loop += 1

print(f"  slow games containing >=1 deck with one of the seven: {games_with_seven}  ({100.0*games_with_seven/len(slow):.1f}%)")
print(f"  slow games containing the r106 loop deck:             {games_with_loop}  ({100.0*games_with_loop/len(slow):.1f}%)")
print()
print(f"  slow-game deck SLOTS: {slot_total}, resolved to a file: {slot_total-sum(unresolved.values())}")
print(f"  slots whose deck holds one of the seven: {slot_with_seven}  ({100.0*slot_with_seven/max(1,slot_total):.2f}%)")
print(f"  POOL BASE RATE for comparison:                        {100.0*pool_with/pool_total:.2f}%")
print()
if unresolved:
    print(f"  UNRESOLVED deck ids (not found on disk): {len(unresolved)} distinct, {sum(unresolved.values())} slots")
    for d, c in unresolved.most_common(5):
        print(f"     {c:5d}  {d}")

# ---- 3. the worst games ------------------------------------------------
print()
print("TEN WORST SLOW GAMES:")
for dur, turns, decks, sw in sorted(slow, reverse=True)[:10]:
    s = any(deck_cards.get(d) for d in decks)
    l = any(LOOP_DECK in d for d in decks)
    swtxt = "  -" if sw is None else f"{sw:3d}"
    print(f"  {dur:7.1f}s turns={turns:3d}  seven={'Y' if s else 'n'}  loopdeck={'Y' if l else 'n'}  swallowed={swtxt}")


# ---- 4. r184 — the depth-cap join -------------------------------------
# The whole point of putting swallowed= on the SLOW GAME line: a capped
# game can now be attributed to its DECKS. Before r184 the cap counters
# were process-global and this join was impossible.
reported = [r for r in slow if r[3] is not None]
print()
if not reported:
    print("DEPTH-CAP JOIN: no SLOW GAME line carries swallowed= "
          "(pre-r184 binary) — nothing to join.")
else:
    capped = [r for r in reported if r[3] > 0]
    print(f"DEPTH-CAP JOIN: {len(reported)} slow games report swallowed=, "
          f"{len(capped)} of them swallowed >0")
    if capped:
        per_deck = collections.Counter()
        per_deck_games = collections.Counter()
        for _d, _t, decks, sw in capped:
            for dk in set(decks):
                per_deck[dk] += sw
                per_deck_games[dk] += 1
        print("  decks present in the most CAPPED slow games:")
        for dk, c in per_deck_games.most_common(10):
            print(f"     games={c:5d}  swallows={per_deck[dk]:7d}  {dk}")
        print()
        print("  NOTE: a deck is credited for every capped game it was "
              "PRESENT in, which is not the same as causing it — four "
              "decks sit at every table. Compare against how often each "
              "deck appears in slow games overall before concluding.")
