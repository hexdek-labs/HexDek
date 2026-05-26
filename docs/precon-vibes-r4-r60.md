# Precon Vibes-Bracket Calibration Baseline (R60, Round 4)

> **HISTORICAL — `plays_like` removed (dev/remove-plays-like-r60).** This doc references the `plays_like` signal and its `estimatePlaysLike()` simulator as a discrete code path. That scaffolding has since been removed end-to-end (Go, JSON, DB-comment, UI, glossary, tests). `measured_bracket` is now the canonical felt-power measurement; the "no wincon → floppy" semantics that `plays_like` tracked are queued for absorption into `measured_bracket` as a separate calibration chapter. The Plays-Like column has been dropped from the ranked table below; historical findings and disagreement-rate rollups are preserved as research evidence.

## Why this is round 4

R1 (PR #508) imported 15 stock WotC precons, reported **2/15** B4 false-positives, and PR #513 traced both to a single predicate in `archetype.go` (the "Tuned-redundancy floor"). R2 (PR #523) ran the same exercise on 15 disjoint precons and found **1/15** B4 false-positive matching the same predicate and same execution path. R3 (PR #532) added 15 more disjoint precons, found **3/15** B4 false-positives — all still hitting the Tuned-redundancy floor — and surfaced one case (Creative Energy) the PR #513 recommendation would NOT have caught.

R4 (this) adds a fourth disjoint 15-deck wave to bring the corpus to **60 precons**. Pre-authorized by 7174n1c.

The point of R4 is twofold: (a) further pin down the predicate-mis-fire rate from R1+R2+R3, and (b) check whether PR #531's *new* "Winning-combo floor" (just merged — broadened 2-card win carveout) interacts well with the heuristic combo detector. Spoiler from the table below: it does not. Two R4 decks land at B4 via the *new* floor, both because the combo detector's known false-positive surface (cycling lines, token-pair "loops", ETB-recurrence pairs) triggers the carveout.

**Naming note (post-refactor):** `bracket` on `DeckProfile`/`archetype` is the **declared** value (auto-stamped B2 for any deck under `data/decks/wizards/` via `isWizardsPrecon`); `measured_bracket` is what the signal estimator (`estimateMeasuredBracket`) computes. R4 reports `measured_bracket` in the table.

## Methodology

Identical to R3:
- 15 NEW Moxfield community uploads of stock WotC precon decklists, zero overlap with the R1+R2+R3 set of 45. All from the canonical "Commander Precons" Moxfield namespace.
- `go run ./cmd/hexdek-import/ --moxfield <url> --owner wizards` — Freya auto-runs on import.
- Metrics extracted via `go run ./cmd/hexdek-freya/ --deck <path> --format json`. `plays_like` lives only in the strategy.json sidecar (it is not yet exposed on the freya JSON `archetype` block) and is loaded from there.

Era spread skews more recent than R1-R3 because the 45-deck overlap set already drained the C-series and many era-2 precons; R4 picks up STX, PIP (Fallout), MKM, MOM, OTJ, DFT, FIN — the corners of the precon catalog R1-R3 didn't yet reach.

## Ranked Table

| # | Era | Precon | Commander | Archetype | Meas Brkt | Cmdr Syn % | Win Lines | Combo Dens | Chain Depth (avg/max) | Recursion Ratio | GC | Power % | Mana | avgCMC | **Decl Brkt** | Δ |
|---|-----|--------|-----------|-----------|:---------:|:----------:|:---------:|:----------:|:---------------------:|:---------------:|:--:|:-------:|:----:|:------:|:--------------:|:-:|
| 1 | OTJ | Desert Bloom | Hakim, Loreweaver | midrange | **4 Optimized** | 74.6 | 48 | 2 | 3.00 / 3 | 0.83 | 0 | 45 | D | 3.58 | **2** | **+2** |
| 2 | STX | Lorehold Spirit | Osgir, the Reconstructor / Quintorius, Field Historian | combo | 2 Core | 69.4 | 9 | 2 | 2.83 / 3 | 0.83 | 0 | 63 | C | 3.21 | **2** | ✓ |
| 3 | PIP | Science! | Dr. Madison Li | artifacts | **4 Optimized** | 98.4 | 16 | 2 | 2.71 / 3 | 0.43 | 0 | 63 | C | 3.24 | **2** | **+2** |
| 4 | MKM | Blame Game | Anzrag, the Quake-Mole | midrange | **4 Optimized** | 40.3 | 12 | 1 | 2.40 / 3 | 0.40 | 0 | 55 | B | 3.65 | **2** | **+2** |
| 5 | MKM | Deadly Disguise | Giada, Font of Hope / Kotis, the Fangkeeper | counters | 2 Core | 17.7 | 12 | 1 | 2.00 / 2 | 0.00 | 0 | 25 | F | 3.84 | **2** | ✓ |
| 6 | STX | Witherbloom Pestilence | Willowdusk, Essence Seer | midrange | 2 Core | 50.0 | 85 | 1 | 2.83 / 3 | 0.50 | 0 | 55 | B | 3.05 | **2** | ✓ |
| 7 | DFT | Living Energy | Satya, Aetherflux Genius | artifacts | 2 Core | 83.6 | 8 | 1 | 2.40 / 3 | 0.60 | 0 | 68 | B | 3.67 | **2** | ✓ |
| 8 | MKM | Deep Clue Sea | Morska, Undersea Sleuth | midrange | **3 Upgraded** | 93.5 | 20 | 1 | 2.50 / 3 | 0.50 | 1 | 48 | D | 3.71 | **2** | +1 |
| 9 | OTJ | Grand Larceny | Gonti, Canny Acquisitor | midrange | 2 Core | 60.7 | 6 | 1 | 2.33 / 3 | 0.67 | 0 | 68 | A | 3.54 | **2** | ✓ |
| 10 | MOM | Cavalry Charge | Bright-Palm, Soul Awakener | midrange | 2 Core | 55.0 | 9 | 1 | 2.40 / 3 | 0.40 | 0 | 58 | C | 3.33 | **2** | ✓ |
| 11 | MOM | Growing Threat | Kasla, the Broken Halo / Moira and Teshar | artifacts | **4 Optimized** | 88.5 | 4 | 1 | 2.50 / 3 | 0.50 | 0 | 50 | A | 4.10 | **2** | **+2** |
| 12 | MOM | Divine Convocation | Eluge, the Shoreless Sea | midrange | 2 Core | 58.3 | 15 | 1 | 2.50 / 3 | 0.75 | 0 | 58 | B | 3.70 | **2** | ✓ |
| 13 | OTJ | Most Wanted | Olivia, Opulent Outlaw | midrange | 2 Core | 87.1 | 20 | 1 | 2.50 / 3 | 0.75 | 0 | 81 | A | 3.26 | **2** | ✓ |
| 14 | OTJ | Quick Draw | Stella Lee, Wild Card | midrange | 2 Core | 90.2 | 4 | 1 | 2.50 / 3 | 0.25 | 0 | 76 | A | 3.28 | **2** | ✓ |
| 15 | FIN | Limit Break | Tifa, Martial Artist | artifacts | 2 Core | 82.3 | 8 | 1 | 2.50 / 3 | 0.25 | 0 | 63 | B | 3.02 | **2** | ✓ |

**Aggregate:** 11/15 exact, 12/15 within ±1. **4/15 B4 false-positives** (Desert Bloom, Science!, Blame Game, Growing Threat) — the highest single-round rate of R1-R4.

> Δ column is `measured_bracket − declared_bracket`. Negative Δ = engine measured COLDER than the stock-precon stamp; positive Δ = engine measured HOTTER. Sign convention matches R1-R3.

## Cross-validation: combined 60-deck corpus

| Metric | R1 | R2 | R3 | R4 | **All 60** |
|---|:-:|:-:|:-:|:-:|:-:|
| Exact match | 11/15 (73%) | 12/15 (80%) | 8/15 (53%) | 11/15 (73%) | **42/60 (70%)** |
| Within ±1 | 13/15 (87%) | 14/15 (93%) | 12/15 (80%) | 12/15 (80%) | **51/60 (85%)** |
| B4 false-positives | 2 | 1 | 3 | **4** | **10/60 (17%)** |
| B1 false-positives (vibes-cooler) | 1 | 0 | 3 | 0 | **4/60 (7%)** |
| plays_like vs measured disagree | 9/15 | 8/15 | 8/15 | 9/15 | **34/60 (57%)** |
| plays_like cooler than measured | 8/9 | 7/8 | 7/8 | 8/9 | **30/34 (88%)** |

**Headlines:**
- The B4 false-positive RATE climbed each round (R1 13%, R2 7%, R3 20%, R4 27%) and now sits at **17% (10/60) across the combined corpus**. The R3→R4 jump correlates with PR #531 merging the *Winning-combo floor* (see Finding 1 below).
- B1 false-positives, which spiked in R3 (3/15), drop back to zero in R4 — consistent with R4's era skew (modern precons rarely score low enough on the raw ladder to hit the B1 path).
- The `plays_like-under-rates-measured` direction stays at 88% of disagreements (30/34 combined) — five rounds straight of the same one-sided bias.

## Findings

### 1. **PR #531's "Winning-combo floor" interacts with combo-detector false-positives — TWO new R4 cases** (severity: HIGH, **CRITICAL NEW SUBFINDING, post-merge regression**)

PR #531 (`feat(freya): r60 — extra-turn signal + broadened 2-card win carveout`) merged at commit `670d92a` and added a new floor predicate — `Winning-combo floor` — that lifts a deck to B4 when "a 2-card categorical-win combo is present, per WotC's combo carveout." The intent is correct: if a deck literally runs the Thoracle + Consultation pair, it plays at B4 regardless of GC count. The problem in R4 is that the floor consumes the **heuristic combo detector's output**, which has a known false-positive surface (cycling lines, food-token "loops", token-pair "loops", ETB-recurrence pairs — see R3 §2). Two R4 precons hit the floor via false-positive combos:

#### 1A. Desert Bloom (OTJ Hakim) — false-positive "trigger loop" → Winning-combo floor

```
Bracket rationale (raw score 6 → B4 Optimized):
  [+2] Tutor density (8-11%): 8% of nonlands
  [+3] Combo lines (5+): 28 true-infinite/determined loops
  [-1] Average CMC (heavy (>3.5)): 3.6 avg
  [+2] Finisher density (8+): 18 distinct finisher lines
  [floor] Winning-combo floor: lifted to B4: 2-card categorical-win combo present (was B3) — WotC carveout
```

Hakim is the OTJ Desert/lands precon. The combo detector reports **27 determined loops + 1 true-infinite** — the true-infinite is `Titania, Protector of Argoth + Sand Scout trigger each other in a loop` (a generic "trigger loop" heuristic match, not an actual infinite — landfall + sand-creature ETB doesn't repeat without an outlet). All 27 determined loops are `Magmatic Insight + <cycling land>` pairs (Magmatic Insight discards a land for cards; cycling lands draw on discard; the loop detector counts the pair as a card-feedback cycle even though Magmatic Insight is a one-shot). Same shape as R3 §2's food-token / squirrel-trigger blowup.

The Winning-combo floor then sees "≥1 true-infinite present" and lifts B3 → B4. Without the false-positive Titania+Sand-Scout entry, the floor wouldn't fire and the deck would correctly land at B3 (where the raw score+ceiling left it).

#### 1B. Growing Threat (MOM Kasla / Moira and Teshar) — false-positive ETB-recurrence loop → Winning-combo floor

```
Bracket rationale (raw score 3 → B4 Optimized):
  [+1] Tutor density (4-7%): 5% of nonlands
  [+1] Combo lines (1): 1 true-infinite/determined loop
  [-1] Average CMC (heavy (>3.5)): 4.1 avg
  [+2] Fast mana (6-9): 6 sub-2-CMC mana producers
  [floor] Winning-combo floor: lifted to B4: 2-card categorical-win combo present (was B2) — WotC carveout
```

Raw score lands at B2 (3 points). The single "true-infinite" the detector reports is `Moira and Teshar + First-Sphere Gargantua` — Moira and Teshar is the MOM artifact-graveyard commander, First-Sphere Gargantua is a Necron with ETB drain. The detector treats "ETB triggers + graveyard recursion" as a renewable loop even though there's no actual repeat (Moira recurs once per turn, First-Sphere Gargantua isn't free-to-recast, no outlet for the would-be loop). The Winning-combo floor lifts B2 → B4 — a **TWO-bracket** jump from a single false-positive combo.

**Both 1A and 1B are post-#531 regressions.** Before PR #531 merged, neither deck would have hit a floor predicate (the Tuned-redundancy floor requires ≥8 finishers AND ≥6 fast-mana — Desert Bloom has 18 finishers but the ceiling would have stopped the lift; Growing Threat has fewer finishers than the threshold). The new floor is strictly more permissive.

**Recommendation:** the Winning-combo floor should require ≥1 **CURATED** combo (from the `KnownCombos` database) rather than ≥1 heuristic-detected combo. The heuristic detector's known false-positive surface (cycling, food, token-pairs, ETB recurrence) makes "1+ true-infinite or determined" insufficient evidence for a 2-bracket lift. Alternative: only lift on ≥1 true-infinite AND the combo carries an outlet card from the deck (the existing `OUTLETS IN DECK:` check).

### 2. **Tuned-redundancy floor STILL mis-fires — 8/60 cumulative confirmation** (severity: HIGH, **CONFIRMS R1+R2+R3**)

Two R4 decks hit the legacy Tuned-redundancy floor, bringing the cumulative count to **8 precons across 60** (R1: Urza+Blast, R2: Family Matters, R3: Squirreled Away, R4: Science!, Blame Game).

#### 2A. Science! (PIP Dr. Madison Li) — classic ceiling→floor override

```
Bracket rationale (raw score 7 → B4 Optimized):
  [+1] Tutor density (4-7%): 5% of nonlands
  [+2] Combo lines (2-4): 3 true-infinite/determined loops
  [+2] Fast mana (6-9): 9 sub-2-CMC mana producers
  [+2] Finisher density (8+): 11 distinct finisher lines
  [ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
  [floor]   Tuned-redundancy floor: lifted to B4: 11 finishers + 9 fast-mana pieces (was B2)
```

**Fifth occurrence** of the GC=0 ceiling → Tuned-redundancy floor override pattern documented in R1 (Blast from the Past), R2 (Family Matters), and R3 (Squirreled Away). Same exact rationale trace: ceiling explicitly demotes with reason *"no Game Changers and no true-infinite combo"*; floor immediately re-lifts to B4. Five-deck confirmation removes any remaining "specific deck quirk" interpretation. The fix surface remains PR #513's recommendation, with R3's option-α/β refinement open.

#### 2B. Blame Game (MKM Anzrag) — floor-only, no raw-score B4 markers

```
Bracket rationale (raw score 4 → B4 Optimized):
  [+1] Tutor density (4-7%): 4% of nonlands
  [-1] Average CMC (heavy (>3.5)): 3.6 avg
  [+2] Fast mana (6-9): 7 sub-2-CMC mana producers
  [+2] Finisher density (8+): 10 distinct finisher lines
  [floor] Tuned-redundancy floor: lifted to B4: 10 finishers + 7 fast-mana pieces (was B2)
```

Same shape as Forces of the Imperium (R3 §1A) — raw score lands at B2, no ceiling fires, floor lifts directly B2 → B4 (skipping B3). PR #513's recommended predicate would catch this: GC=0, true_inf=0, tutor=4% (below 8%), avgCMC=3.6 (above 3.0) → no conjunct satisfied → floor wouldn't fire.

### 3. **Combo-detector false-positive surface widens further — R4 surfaces three new substrates** (severity: HIGH, **EXPANDS R3 §2**)

R1+R2 (cycling lands), R3 (food tokens, squirrel ETB triggers) — and now R4 adds:

| Substrate | Deck | False-loop signature |
|---|---|---|
| **discard-cycle pairs** | Desert Bloom | 27 `Magmatic Insight + <cycling land>` pairs — discard-for-card paired with cycling-for-card |
| **Detective/Clue-token pairs** | Deep Clue Sea | 13 entries: `Sophia, Dogged Detective + Tireless Tracker` (clue token loop), `Tireless Tracker + Graf Mole` (life-from-clue loop), `Killer Service + Graf Mole` (token-creation → life-pay loop) |
| **ETB-recurrence pairs** | Growing Threat | `Moira and Teshar + First-Sphere Gargantua` — ETB drain creature paired with graveyard-recur commander |

R3's diagnosis was correct: the bug is not specific to cycling. R4 confirms that **any pair of cards where the detector sees "trigger that produces X" matched with "consumer of X"** counts as a renewable cycle, even when the producer is one-shot, the consumer needs to be re-cast, or the trigger isn't actually a loop closing back on itself. Cumulative confirmed substrates: cycling, food, squirrel-ETB, discard, detective-clue, ETB-recurrence. **6 confirmed substrate classes across 60 decks.** The detector's "renewable producer" check needs explicit exclusions for one-shot trigger keywords AND a "loop must actually close" graph check (not just "A produces what B consumes AND B produces what A consumes").

This is now blocking accurate B4 detection: R4's two Winning-combo-floor false-positives (Finding 1) both hinge on heuristic-detector errors, so fixing the detector also closes those.

### 4. **Deep Clue Sea (MKM Morska) lands at B3 — driven by FP combos** (severity: LOW, but flagged for ladder review)

```
Bracket rationale (raw score 6 → B3 Upgraded):
  [+1] Game Changers (1): 1 on WotC list
  [+3] Combo lines (5+): 13 true-infinite/determined loops
  [-1] Average CMC (heavy (>3.5)): 3.7 avg
  [+2] Fast mana (6-9): 7 sub-2-CMC mana producers
  [+1] Finisher density (4-7): 5 distinct finisher lines
```

GC=1 → B3 ceiling, raw score reaches 6, lands at B3. The +3 "Combo lines (5+)" contribution is the dominant signal — and as Finding 3 documents, those 13 lines are all detective-clue token pair false-positives. Without the FP combos, raw score would be 3 and the deck would land at B2. The +1 Δ is technically within the ±1 tolerance window, but the entire B3 call is being driven by combo-detector noise. **Logged as a "borderline correct call for the wrong reasons" entry.**

### 5. **Multi-precon MOM era — Cavalry Charge / Divine Convocation correctly hold B2** (severity: LOW, positive finding)

Three MOM (March of the Machine) precons in R4 — Cavalry Charge, Growing Threat, Divine Convocation. Two of three (Cavalry Charge + Divine Convocation) correctly land at B2 with no floor mis-fire. The third (Growing Threat) trips the new Winning-combo floor (Finding 1B) but only because of a single FP combo. The era itself doesn't have a systematic miscalibration — the FP rate within MOM is 1/3, in line with the corpus-wide 4/15 R4 rate.

## Cumulative summary (60 stock precons)

| Finding | Status |
|---|---|
| Tuned-redundancy floor mis-fires on stock precons | **8/60 (13%)** — 5-deck ceiling-override pattern + 3 floor-only firings. Predicate confirmed systemic across four independent waves. |
| Winning-combo floor mis-fires on heuristic-combo FPs | **2/60 (3%)** — both R4 (Desert Bloom, Growing Threat). NEW post-#531 regression. |
| Floor overrides GC=0 ceiling unconditionally | **4/60 (7%)** — Blast (R1), Family Matters (R2), Squirreled Away (R3), Science! (R4) |
| PR #513's recommended predicate fix would catch all known false-positives | **NO — Creative Energy (R3) remains a counter-example.** Fix needs refinement (R3 options α/β). |
| Cycling/consume-once loop detector mis-fires | **6 substrate classes confirmed** (cycling, food, squirrel-ETB, discard, detective-clue, ETB-recurrence) across at least 6 decks |
| `plays_like` under-rates measured bracket | **30/34 disagreements in same direction (88%)** — four rounds confirmed |
| B1 false-positives on the algorithm's "cold" end | **4/60 (7%)** — all in R1+R3; R4 era skew (no C-series) shielded R4 from the surface |

## Open decisions for 7174n1c (carried, updated)

1. **Implement PR #513's recommended fix** (engine work). R4 confirms the fix as-designed catches 7 of 8 known Tuned-redundancy false-positives (the same as R3 +1 — both new R4 floor-firings are within the fix's catch). Decide whether to (a) ship as-designed and accept the 1/60 Creative Energy edge case, or (b) tighten with R3's option α or β before shipping.
2. **Tighten the Winning-combo floor predicate** (engine work, NEW from R4). The new floor consumes the heuristic combo detector's output; given the detector's known FP surface, the floor needs either (a) require ≥1 CURATED combo from `KnownCombos`, or (b) require ≥1 true-infinite with at least one in-deck outlet (the existing `OUTLETS IN DECK:` check). Without this guard, the floor adds 2/15 false-positive lifts per wave — and the rate may keep climbing as the detector encounters new substrates.
3. **Fix the consume-once loop detector** — now 6-substrate confirmed across at least 6 decks. R3's "2-3 line addition" estimate is probably understated; the detector needs a "loop must close" graph check, not just a renewable-producer exclusion list. Highest-leverage fix because it would also close finding 1 and the Combo-lines score-ladder inflation.
4. **Trace `estimatePlaysLike()`** — direction-consistent bias across 30/34 disagreements over four rounds, now extremely strongly motivated.

## Reproducing

```bash
# Re-import any R4 precon:
go run ./cmd/hexdek-import/ --moxfield https://moxfield.com/decks/<id> --owner wizards

# Trace any deck's bracket call:
go run ./cmd/hexdek-freya/ --deck data/decks/wizards/<slug>.txt 2>&1 | grep -A 15 'Bracket rationale'

# Extract measured_bracket directly:
go run ./cmd/hexdek-freya/ --deck <path> --format json | jq '.archetype | {bracket, measured_bracket, bracket_rationale}'
```

15 R4 source URLs (all `https://moxfield.com/decks/<id>` in the `Commander Precons` namespace, none overlapping R1, R2, or R3):

| Precon | Moxfield ID |
|--------|-------------|
| Desert Bloom (OTJ) | `5MXFLs15ck6nh85X5VEjyQ` |
| Lorehold Spirit (STX) | `8erwc7vsiEanV87GMM3IuQ` |
| Science! (PIP / Fallout) | `BA1a99vfi0W0YLlGC97t-A` |
| Blame Game (MKM) | `BPBVD7NsT0SznrmdG4l3Tw` |
| Deadly Disguise (MKM) | `c9xQjfvGwkCMFmQWcdOKVw` |
| Witherbloom Pestilence (STX) | `CIe9QwsuMUm0xHZVdXcZJQ` |
| Living Energy (DFT / Aetherdrift) | `jWSmSfkdGEijTfTX_d8qNQ` |
| Deep Clue Sea (MKM) | `kudcIOMGwkuN0hi99q10MQ` |
| Grand Larceny (OTJ) | `kwiSILSLR0ic9U38G00JZQ` |
| Cavalry Charge (MOM) | `PrHvjllh3kuzVuSV2wyhCQ` |
| Growing Threat (MOM) | `Rwg0cZ0Yqk2ujq_GXIQA2g` |
| Divine Convocation (MOM) | `SfNm_azRAUGtkhIuesdBIw` |
| Most Wanted (OTJ) | `V766u1HzgUCREUUSgsnfFA` |
| Quick Draw (OTJ) | `wpucJrNlHUSL8zosshILkQ` |
| Limit Break (FIN / Final Fantasy) | `xNhT2XmIrkOu2lXb4-vjhg` |
