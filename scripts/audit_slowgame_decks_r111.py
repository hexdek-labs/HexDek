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
line_re = re.compile(r"SLOW GAME ([0-9.]+)s turns=(\d+) decks=\[([^\]]*)\]")
slow = []
unresolved = collections.Counter()
for L in open(LOG, errors="replace"):
    m = line_re.search(L)
    if m:
        slow.append((float(m.group(1)), int(m.group(2)), m.group(3).split()))

print(f"SLOW GAMES in server.log: {len(slow)}")
if not slow:
    raise SystemExit("no slow games parsed — the regex or the log is wrong")

games_with_seven = 0
games_with_loop  = 0
slot_total = 0
slot_with_seven = 0
for _dur, _turns, decks in slow:
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
for dur, turns, decks in sorted(slow, reverse=True)[:10]:
    s = any(deck_cards.get(d) for d in decks)
    l = any(LOOP_DECK in d for d in decks)
    print(f"  {dur:7.1f}s turns={turns:3d}  seven={'Y' if s else 'n'}  loopdeck={'Y' if l else 'n'}")
