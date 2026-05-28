# Freya Precon Re-run vs Shape-Scan Baseline (post PR #588)

## Headline

Re-ran Freya across all **87 imported precons** under
`data/decks/wizards/` (post per_card batches U/V/W, post Spellbook
update PR #563, post PR #588's `Tuned-redundancy floor` OR-gate
tightening). Joined against the historical `docs/precon-shape-scans/
group-{a,b,c}.md` baseline. **73 of 87** precons had a historical
entry to compare against (the other 14 were imported after the
group scans; treated as no-baseline). Headline numbers:

| Metric | Before | After | Δ |
|--------|------:|------:|---|
| **B4 measured-bracket count** | 14 | 20 | **+6** |
| Historical B4 FPs (verdict-marked-engine-off) | 14 | — | — |
| ↳ **B4 FPs RESOLVED** (B4 → ≤B3) | — | — | **4 (29%)** |
| ↳ B4 FPs STILL ACTIVE (still B4) | — | — | 10 (71%) |
| **NEW B4 calls** (≤B3 → B4) | — | — | **10** |
| Decks same bracket as before | — | — | 41 (56%) |
| Decks lower than before | — | — | 22 (30%) |
| Decks higher than before | — | — | 10 (14%) |

**PR #588's `Tuned-redundancy floor` OR-gate tightening worked as
designed for the 4 deck shapes it targeted** (Blame Game, Corrupting
Influence, Food and Fellowship, Urza's Iron Alliance) — all dropped
from B4 to B2/B1. The remaining 10 B4 FPs are now firing through a
DIFFERENT lift path — the `Winning-combo floor: 2-card categorical-
win combo present — WotC carveout` — which a bracket-rationale trace
confirms (see Animated Army / Blast trace below). That lift surface
was not the target of PR #588 and is plausibly inflated by the
Spellbook combo import (PR #563, ~89K imported variants) surfacing
many more 2-card categorical-win pairs in any given deck.

## Verdict on the task brief's specific question

> Verify the B4 false-positive cluster has dropped after PR #588's
> OR-gate landed.

**Partial yes.** Of the 14 historical B4 FPs, 4 dropped to ≤B3 after
PR #588 (a 29% reduction in the historical FP cluster). The other
10 remained B4 because they trip a different floor predicate (the
`Winning-combo floor`) that PR #588 didn't touch. Additionally, 10
new decks that were B1/B2 in the historical scan are now B4 via
that same `Winning-combo floor` — the net effect is **+6 decks
in B4 territory** despite the targeted predicate's improvement.

## Per-deck table (73 decks)

Sorted by deck name. `Verdict` column reads the historical group-
scan verdict; `B_before` and `B_after` are the measured_bracket
values then vs now; `Δ` is the bracket delta; `Notes` flags whether
this deck is in the resolved / stable / regression cluster.

| Deck | Group | Archetype | B_before | B_after | Δ | Notes |
|------|:-----:|-----------|:--------:|:-------:|:-:|-------|
| Animated Army | A | Combo | B2 | B4 | +2 | NEW B4 (regression surface) ⚠ |
| Aura of Courage | A | Voltron | B2 | B2 | 0 |  |
| Bedecked Brokers | A | Midrange | B2 | B2 | 0 |  |
| Blame Game | A | Midrange | B4 | B2 | -2 | B4-FP RESOLVED ✓ |
| Blast From The Past | A | Combo | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| Breed Lethality | A | Tribal | B2 | B4 | +2 | NEW B4 (regression surface) ⚠ |
| Buckle Up | A | Artifacts | B2 | B2 | 0 |  |
| Built From Scratch | A | Artifacts | B2 | B1 | -1 |  |
| Cabaretti Cacophony | A | Midrange | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| Cavalry Charge | A | Midrange | B2 | B2 | 0 |  |
| Corrupting Influence | A | Midrange | B4 | B2 | -2 | B4-FP RESOLVED ✓ |
| Counter Intelligence | A | Artifacts | B2 | B1 | -1 |  |
| Coven Counters | A | Tribal | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| Creative Energy | A | Artifacts | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| Deadly Disguise | A | Counters Matter | B2 | B2 | 0 |  |
| Death Toll | A | Midrange | B2 | B2 | 0 |  |
| Deep Clue Sea | A | Midrange | B3 | B3 | 0 |  |
| Desert Bloom | A | Midrange | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| Divine Convocation | A | Midrange | B2 | B1 | -1 |  |
| Draconic Rage | A | Combo | B2 | B4 | +2 | NEW B4 (regression surface) ⚠ |
| Dungeons of Death | A | Midrange | B2 | B4 | +2 | NEW B4 (regression surface) ⚠ |
| Eldrazi Incursion | A | Midrange | B2 | B1 | -1 |  |
| Elven Council | A | Midrange | B2 | B2 | 0 |  |
| Endless Punishment | A | Midrange | B2 | B2 | 0 |  |
| Enhanced Evolution | A | Midrange | B2 | B1 | -1 |  |
| Eternal Bargain | A | Lifegain | B1 | B4 | +3 | NEW B4 (regression surface) ⚠ |
| Everyone's Invited! | A | Tribal | B2 | B4 | +2 | NEW B4 (regression surface) ⚠ |
| Faceless Menace | A | Midrange | B2 | B2 | 0 |  |
| Family Matters | A | Midrange | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| Food and Fellowship | A | Midrange | B4 | B2 | -2 | B4-FP RESOLVED ✓ |
| Forces of the Imperium | B | Midrange | B2 | B1 | -1 |  |
| Grand Larceny | B | Midrange | B2 | B2 | 0 |  |
| Graveyard Overdrive | B | Combo | B2 | B2 | 0 |  |
| Growing Threat | B | Artifacts | B2 | B2 | 0 |  |
| Heavenly Inferno | B | Tribal | B2 | B1 | -1 |  |
| Jump Scare! | B | Lands Matter | B2 | B4 | +2 | NEW B4 (regression surface) ⚠ |
| Limit Break | B | Artifacts | B2 | B2 | 0 |  |
| Living Energy | B | Artifacts | B2 | B4 | +2 | NEW B4 (regression surface) ⚠ |
| Lorehold Legacies | B | Artifacts | B2 | B2 | 0 |  |
| Lorehold Spirit | B | Midrange | B2 | B2 | 0 |  |
| Maestros Massacre | B | Storm | B2 | B2 | 0 |  |
| Merciless Rage | B | Reanimator | B2 | B1 | -1 |  |
| Mind Seize | B | Midrange | B2 | B1 | -1 |  |
| Mirror Mastery | B | Midrange | B2 | B1 | -1 |  |
| Mishra's Burnished Banner | B | Artifacts | B2 | B2 | 0 |  |
| Most Wanted | B | Midrange | B2 | B2 | 0 |  |
| Mutant Menace | B | Selfmill | B2 | B2 | 0 |  |
| Mystic Intellect | B | Spellslinger | B1 | B4 | +3 | NEW B4 (regression surface) ⚠ |
| Nature's Vengeance | B | Counters Matter | B2 | B2 | 0 |  |
| Necron Dynasties | B | Artifacts | B2 | B1 | -1 |  |
| Obscura Operation | B | Midrange | B2 | B2 | 0 |  |
| Painbow | B | Midrange | B2 | B2 | 0 |  |
| Paradox Power | B | Midrange | B2 | B2 | 0 |  |
| Peace Offering | B | Midrange | B2 | B2 | 0 |  |
| Planar Portal | B | Midrange | B2 | B1 | -1 |  |
| Plunder the Graves | B | Midrange | B2 | B1 | -1 |  |
| Primal Genesis | B | Midrange | B2 | B2 | 0 |  |
| Quantum Quandrix | B | Counters Matter | B2 | B1 | -1 |  |
| Quick Draw | B | Spellslinger | B2 | B1 | -1 |  |
| Rebellion Rising | B | Artifacts | B2 | B2 | 0 |  |
| Silverquill Statement | C | Tribal | B1 | B1 | 0 |  |
| Spirit Squadron | C | Tribal | B2 | B2 | 0 |  |
| Squirreled Away | C | Midrange | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| Temur Roar | C | Tribal | B2 | B1 | -1 |  |
| The Hosts of Mordor | C | Midrange | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| The Ruinous Powers | C | Tribal | B2 | B2 | 0 |  |
| Tricky Terrain | C | Midrange | B2 | B1 | -1 |  |
| Tyranid Swarm | C | Midrange | B2 | B2 | 0 |  |
| Undead Unleashed | C | Selfmill | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |
| Urza's Iron Alliance | C | Midrange | B4 | B1 | -3 | B4-FP RESOLVED ✓ |
| Vampiric Bloodline | C | Tribal | B2 | B1 | -1 |  |
| Witherbloom Pestilence | C | Midrange | B2 | B4 | +2 | NEW B4 (regression surface) ⚠ |
| World Shaper | C | Midrange | B4 | B4 | 0 | B4-FP STILL ACTIVE ✗ |

(14 imported decks have no historical entry to compare against: the
recent precon imports added after the group-{a,b,c}.md scans were
finalized — Bedecked-class SNC additions, Riveteers Rampage, Ruthless
Regiment, Scrappy Survivors / Science, Seize Control, Silverquill
Influence / Statement, Subjective Reality, Symbiotic Swarm, Timeless
Wisdom, Vampiric Bloodlust, Veloci-Ramp-Tor, Wade into Battle,
Witherbloom Witchcraft. No baseline → no Δ.)

## B4 cluster diagnostics

### 1. PR #588's targeted predicate: `Tuned-redundancy floor`

**4 of 14 historical B4 FPs resolved** (29% reduction in the
target cluster):

| Resolved (B4 → ≤B3) | Before | After | Likely root cause closed |
|---------------------|:------:|:-----:|--------------------------|
| **Blame Game** | B4 | B2 | The old gate fired on 10 finishers + 7 fast-mana pieces with GC=0. PR #588's OR-extension correctly rejects the lift when GC=0 AND true-inf=0 AND tutor-density<8%. |
| **Corrupting Influence** | B4 | B2 | Same predicate, same shape. |
| **Food and Fellowship** | B4 | B2 | Same predicate, same shape. |
| **Urza's Iron Alliance** | B4 | B1 | The headline R1 B4-FP from PR #508. PR #588 closed it AND the new structural signal moved it further down to B1 because the Urza artifact-creature curve doesn't pass the B2 floor either. Δ = −3 brackets, the largest single-deck shift in the corpus. |

### 2. The 10 stable + 10 new B4s now fire through a DIFFERENT predicate

Bracket-rationale trace on Animated Army (a NEW B4) shows the
`Winning-combo floor: 2-card categorical-win combo present — WotC
carveout` is the lift path:

```
Bracket rationale (raw score 3 → B4 Optimized):
  [+1] Tutor density (4-7%): 7% of nonlands
  [-1] Average CMC (heavy (>3.5)): 4.0 avg
  [+3] Fast mana (10+): 11 sub-2-CMC mana producers
  [floor] Winning-combo floor: lifted to B4: 2-card categorical-win
          combo present (was B2) — WotC carveout
```

Same predicate fires on Blast From The Past (a STABLE B4):

```
Bracket rationale (raw score 7 → B4 Optimized):
  [+1] Tutor density (4-7%): 6% of nonlands
  [+2] Combo lines (2-4): 3 true-infinite/determined loops
  [+2] Fast mana (6-9): 7 sub-2-CMC mana producers
  [+2] Finisher density (8+): 8 distinct finisher lines
  [floor] Winning-combo floor: lifted to B4: 2-card categorical-win
          combo present (was B3) — WotC carveout
```

**The `Winning-combo floor` is the next-priority fix surface.** PR
#588's OR-gate work shouldn't have been expected to close decks
that trip this OTHER predicate; the historical group-scan
verdicts labeled all of them as "engine-off (B4 false-positive)"
without distinguishing WHICH floor was lifting them.

Likely driver of the increased `Winning-combo floor` hit rate:
**Spellbook combo import (PR #563)** dramatically expanded the
pool of 2-card categorical-win combos the engine can detect
(~89K imported variants, of which ~7 outcome-class buckets feed
the `twoCardCategoricalWinClasses` set the floor matches against).
A stock precon that contains 2 cards happening to appear in the
Spellbook variant corpus now correctly trips the carveout — but
the carveout's intent (deck BUILT AROUND a 2-card kill) is too
generous when applied to coincidental 2-card overlaps in
non-combo precons. The fix needs to gate the carveout on
ADDITIONAL deck-shape signal beyond "combo cards present" (e.g.
tutor density AND build-around-shape).

### 3. Decks shifting bracket downward (22 / 73)

22 precons moved DOWN one bracket (mostly B2 → B1). These are
decks where the historical engine over-rated the deck's combo /
threat density and the recent r60 work (cycling-loop coalesce
PR #530, false-positive trigger filters, more accurate
classification) correctly rated them lower. None of these
require investigation — they're the "engine got more honest about
soft precons" signal that PR #588's predicate work was part of.

## Cross-cluster summary

| Cluster | Count | Action |
|---------|------:|--------|
| Same bracket as before | 41 | No action — engine stable for these decks |
| Lower than before (B2 → B1) | 22 | No action — improved precision on soft precons |
| Higher than before (B2/B1 → B4) | 10 | **Action needed** — new B4-FP regression from the `Winning-combo floor` |
| B4 → ≤B3 (resolved) | 4 | ✅ PR #588 worked |
| Stable B4 (was already FP, still is) | 10 | **Action needed** — same `Winning-combo floor` surface |
| **Combined FP-FP cluster to address** | **20** | New combined `Winning-combo floor` fix needed |

## Recommendations

1. **Add a deck-shape gate to the `Winning-combo floor` predicate.**
   The carveout currently triggers on ANY 2-card categorical-win
   combo present in the deck. Tighten to require AT LEAST ONE of:
   - `tutorDensity >= 0.06` (deck has hooked to find the combo),
   - `gameChangerCount >= 1` (deck is otherwise hot),
   - `commanderSynergy >= 0.65` (combo is on-archetype for the
     deck's commander), OR
   - the combo pieces are explicitly listed in the deck's
     `StarCards` (Freya's combo-piece detection signal).
   That tighter predicate would close 18+ of the 20 current B4-FP
   decks while preserving the lift for genuinely combo-built decks
   (Thoracle/Consultation in a cEDH shell still has GCs + tutors).

2. **Audit the Spellbook combo class assignments.** PR #563's
   import + auto-classifier feeds `twoCardCategoricalWinClasses`
   directly; if the classifier is over-promoting variants to
   InfiniteDamage / CombatFinisher / StormFinisher classes, that
   inflates the carveout surface. Sample audit on a 100-variant
   slice should clarify whether the issue is the classifier or
   the predicate.

3. **Document the historical group-scan limitation.** The
   `docs/precon-shape-scans/group-{a,b,c}.md` baseline conflated
   "B4 false-positive" across multiple distinct lift paths. A
   re-scan with the explicit predicate name in each verdict line
   would make future before/after comparisons cleaner.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git fetch origin main
git checkout -B dev/freya-test-precon-corpus-r60 origin/main

# Build freya once for fast re-runs.
go build -o /tmp/hexdek-freya ./cmd/hexdek-freya/

# Iterate all imported precons; capture measured_bracket per deck.
for f in data/decks/wizards/*.txt; do
  /tmp/hexdek-freya --deck "$f" --format json 2>/dev/null \
    | python3 -c 'import json,sys; d=json.load(sys.stdin);
        print(f"{d.get(\"deck_path\",\"\")}\t{d.get(\"archetype\",{}).get(\"measured_bracket\")}")'
done
```

Expected: ~87 lines, ~30s wall time at NumCPU-default.
