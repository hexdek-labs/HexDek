# Precon Vibes-Bracket Calibration Baseline (R60, Round 3)

> **HISTORICAL — `plays_like` removed (dev/remove-plays-like-r60).** This doc references the `plays_like` signal and its `estimatePlaysLike()` simulator as a discrete code path. That scaffolding has since been removed end-to-end (Go, JSON, DB-comment, UI, glossary, tests). `measured_bracket` is now the canonical felt-power measurement; the "no wincon → floppy" semantics that `plays_like` tracked are queued for absorption into `measured_bracket` as a separate calibration chapter. The Plays-Like column has been dropped from the ranked table below; historical findings and disagreement-rate rollups are preserved as research evidence.

## Why this is round 3

R1 (PR #508) imported 15 stock WotC precons and reported 2/15 B4 false-positives, traced (PR #513) to a single predicate at `cmd/hexdek-freya/archetype.go:1229`. R2 (PR #523) ran the same exercise on 15 disjoint precons and found 1/15 B4 false-positive matching the same predicate and same execution path, confirming the trace was systemic rather than sample-specific. R3 (this) adds a third disjoint 15-deck wave to bring the corpus to 45 precons. Pre-authorized by 7174n1c.

The point of R3 is twofold: (a) further pin down the rates from R1+R2, and (b) test whether the recommended fix from PR #513 would actually have caught every B4 false-positive that the predicate produces — by finding cases the recommendation might miss.

**Naming note (post-refactor):** Since R1 the bracket plumbing was refactored. `bracket` on `DeckProfile`/`archetype` is now the **declared** value (auto-stamped B2 for any deck under `data/decks/wizards/` via `isWizardsPrecon`); `measured_bracket` is what the signal estimator (`estimateMeasuredBracket`, formerly `estimateBracket`) actually computes. R3 reports `measured_bracket` as the column previously called `mechanical_bracket`. R1's table was retroactively updated; R2's numbers are unchanged because the precon stamp didn't change which bracket the signal estimator computed.

## Methodology

Identical to R2:
- 15 NEW Moxfield community uploads of stock WotC precon decklists, zero overlap with the R1+R2 set of 30. Same 5-era distribution (3 per era). All from the canonical "Commander Precons" Moxfield namespace.
- `go run ./cmd/hexdek-import/ --moxfield <url> --owner wizards` — Freya auto-runs on import.
- Metrics extracted via `go run ./cmd/hexdek-freya/ --deck <path> --format json` (NOT strategy.json — that file now has `bracket` rubber-stamped to 2 for wizards/precons and does not yet expose `measured_bracket`).

## Ranked Table

| # | Era | Precon | Commander | Archetype | Meas Brkt | Cmdr Syn % | Win Lines | Combo Dens | Chain Depth (avg/max) | Recursion Ratio | GC | Power % | Mana | avgCMC | **Decl Brkt** | Δ |
|---|-----|--------|-----------|-----------|:---------:|:----------:|:---------:|:----------:|:---------------------:|:---------------:|:--:|:-------:|:----:|:------:|:--------------:|:-:|
| 1 | C11 | Mirror Mastery | Riku of Two Reflections | midrange | **1 Exhibition** | 46.6 | 4 | 1 | 2.50 / 3 | 0.25 | 0 | 55 | A | 4.26 | **2** | −1 |
| 2 | C13 | Eternal Bargain | Oloro, Ageless Ascetic | lifegain | **1 Exhibition** | 34.5 | 3 | 2 | 3.00 / 3 | 0.00 | 0 | 50 | B | 3.81 | **2** | −1 |
| 3 | C14 | Built from Scratch | Daretti, Scrap Savant | artifacts | 2 Core | 75.9 | 3 | 1 | 2.60 / 3 | 0.60 | 0 | 60 | A | 3.79 | **2** | ✓ |
| 4 | C19 | Faceless Menace | Kadena, Slinking Sorcerer | midrange | 2 Core | 18.6 | 4 | 1 | 2.40 / 3 | 0.40 | 1 | 43 | D | 3.59 | **2** | ✓ |
| 5 | C20 | Enhanced Evolution | Otrimi, the Ever-Playful | midrange | 2 Core | 33.9 | 3 | 1 | 2.00 / 2 | 1.00 | 0 | 50 | B | 3.68 | **2** | ✓ |
| 6 | C21 | Lorehold Legacies | Osgir, the Reconstructor | artifacts | 2 Core | 100.0 | 4 | 1 | 2.67 / 3 | 0.67 | 0 | 63 | A | 3.87 | **2** | ✓ |
| 7 | AFR | Planar Portal | Prosper, Tome-Bound | midrange | **1 Exhibition** | 86.7 | 3 | 1 | 3.00 / 3 | 0.50 | 0 | 73 | A | 3.62 | **2** | −1 |
| 8 | MID | Undead Unleashed | Wilhelt, the Rotcleaver | selfmill | 2 Core | 71.2 | 3 | 1 | 2.75 / 3 | 0.75 | 0 | 55 | B | 3.97 | **2** | ✓ |
| 9 | DMU | Painbow | Jared Carthalion | midrange | 2 Core | 68.3 | 3 | 0 | 2.25 / 3 | 0.75 | 0 | 48 | F | 3.63 | **2** | ✓ |
| 10 | 40K | Forces of the Imperium | Inquisitor Greyfax | midrange | **4 Optimized** | 66.1 | 3 | 1 | 2.67 / 3 | 0.33 | 0 | 58 | B | 3.81 | **2** | **+2** |
| 11 | LTR | The Hosts of Mordor | Sauron, Lord of the Rings | midrange | **3 Upgraded** | 32.8 | 4 | 2 | 2.50 / 3 | 0.50 | 1 | 63 | A | 4.08 | **2** | +1 |
| 12 | WHO | Paradox Power | The Thirteenth Doctor | midrange | 2 Core | 62.9 | 3 | 1 | 2.40 / 3 | 0.60 | 0 | 63 | B | 3.44 | **2** | ✓ |
| 13 | MH3 | Creative Energy | Satya, Aetherflux Genius | artifacts | **4 Optimized** | 47.5 | 4 | 1 | 2.50 / 3 | 0.00 | 1 | 58 | B | 3.85 | **2** | **+2** |
| 14 | BLB | Squirreled Away | Hazel of the Rootbloom | midrange | **4 Optimized** | 52.5 | 3 | 1 | 3.00 / 3 | 0.67 | 0 | 58 | B | 3.31 | **2** | **+2** |
| 15 | DSK | Endless Punishment | Valgavoth, Harrower of Souls | midrange | 2 Core | 31.1 | 3 | 2 | 2.50 / 3 | 0.25 | 0 | 68 | A | 3.46 | **2** | ✓ |

**Aggregate:** 8/15 exact, 12/15 within ±1. **3/15 B4 false-positives** (Forces of the Imperium, Creative Energy, Squirreled Away).

> Δ column is `measured_bracket − declared_bracket`. Negative Δ = engine measured COLDER than the stock-precon stamp; positive Δ = engine measured HOTTER. Sign convention matches R1.

## Cross-validation: combined 45-deck corpus

| Metric | R1 | R2 | R3 | **All 45** |
|---|:-:|:-:|:-:|:-:|
| Exact match | 11/15 (73%) | 12/15 (80%) | 8/15 (53%) | **31/45 (69%)** |
| Within ±1 | 13/15 (87%) | 14/15 (93%) | 12/15 (80%) | **39/45 (87%)** |
| B4 false-positives | 2 | 1 | **3** | **6/45 (13%)** |
| B1 false-positives (vibes-cooler) | 1 | 0 | **3** | **4/45 (9%)** |
| plays_like vs measured disagree | 9/15 (60%) | 8/15 (53%) | 8/15 (53%) | **25/45 (56%)** |
| plays_like cooler than measured | 8/9 | 7/8 | 7/8 | **22/25 (88%)** |

**Headline:** R1+R2's findings continue to hold on R3. The B4 false-positive RATE ticks up (13% across 45 decks vs the 10% point estimate from R1+R2) but stays in the same regime. The plays_like-under-rates-measured direction stays 88% of disagreements.

## Findings

### 1. **6-for-6 on the predicate — and R3 surfaces a case the PR #513 fix would MISS** (severity: HIGH, **CRITICAL NEW SUBFINDING**)

All three R3 B4 false-positives hit the `Tuned-redundancy floor` at `cmd/hexdek-freya/archetype.go:1229`. Per-deck:

#### 1A. Forces of the Imperium (40K Greyfax) — pure floor-only false-positive
```
Bracket rationale (raw score 3 → B4 Optimized):
  [-1] Average CMC (heavy (>3.5)): 3.8 avg
  [+2] Fast mana (6-9): 7 sub-2-CMC mana producers
  [+2] Finisher density (8+): 22 distinct finisher lines
  [floor] Tuned-redundancy floor: lifted to B4: 22 finishers + 7 fast-mana pieces (was B2)
```
Same shape as **Urza's Iron Alliance (R1)** — raw score lands at B2, no ceiling fires, floor lifts directly B2 → B4 (skipping B3). The "22 distinct finisher lines" is an extreme version of the finisher-counter inflation R1 flagged: Greyfax is an Esper artifact/equipment commander whose precon ships a long tail of anthem-flavor mass-pump effects each counted as a separate finisher line.

#### 1B. Squirreled Away (BLB Hazel) — ceiling→floor override pattern, third occurrence
```
Bracket rationale (raw score 7 → B4 Optimized):
  [+3] Combo lines (5+): 287 true-infinite/determined loops
  [+2] Fast mana (6-9): 7 sub-2-CMC mana producers
  [+2] Finisher density (8+): 9 distinct finisher lines
  [ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
  [floor]   Tuned-redundancy floor: lifted to B4: 9 finishers + 7 fast-mana pieces (was B2)
```
**Identical** to the ceiling→floor override pattern documented for Blast from the Past (R1) and Family Matters (R2). Three precons across three independent waves now show the exact same rationale trace: GC=0 ceiling explicitly demotes to B2 with reason *"no Game Changers and no true-infinite combo"*, then the tuned-redundancy floor immediately re-lifts to B4. This is now beyond dispute.

#### 1C. Creative Energy (MH3 Satya) — **the case PR #513's recommended fix would NOT catch**
```
Bracket rationale (raw score 7 → B4 Optimized):
  [+1] Game Changers (1): 1 on WotC list
  [+1] Tutor density (4-7%): 4% of nonlands
  [+2] Combo lines (2-4): 3 true-infinite/determined loops
  [-1] Average CMC (heavy (>3.5)): 3.9 avg
  [+2] Fast mana (6-9): 6 sub-2-CMC mana producers
  [+2] Finisher density (8+): 9 distinct finisher lines
  [floor] Tuned-redundancy floor: lifted to B4: 9 finishers + 6 fast-mana pieces (was B3)
```
First R-series false-positive where the raw ladder reaches B3 on the ladder (raw=7 with GC=1, GC=1-3 ceiling allows up to B3) and the floor lifts B3 → B4. PR #513's recommended predicate tightening was:
```go
tunedRedundancy := finisherCount >= 8 && ctx.fastManaCount >= 6 &&
                   (ctx.gameChangerCount >= 1 || trueInfCount >= 1 ||
                    ctx.tutorDensity >= 0.08 ||
                    (ctx.avgCMC < 3.0 && manaGradeAtLeast(report, "B")))
```
Creative Energy satisfies the FIRST disjunct (`gameChangerCount >= 1`) because it ships Mana Vault on the WotC GC list. Under the recommended fix, the floor would STILL fire, and the deck would STILL land at B4.

**Implication:** the recommended fix is correct for the 5 of 6 known false-positives whose pattern is "floor fires despite no B4 markers anywhere", but the predicate's `gameChangerCount >= 1` conjunct is too weak when combined with the floor's existing behavior. A single GC card (Mana Vault, Smothering Tithe, Sol Ring's-WotC-controversial-cousin, etc.) shipped in a stock precon should not be sufficient evidence to lift the deck two brackets above its stock-precon baseline.

**Two refinement options** (each closes Creative Energy):
- **Option α:** raise the GC threshold to `gameChangerCount >= 2` — single-GC precons stop tripping the floor.
- **Option β:** keep `gameChangerCount >= 1` but ALSO require `power_percentile >= 65` OR `tutor_density >= 0.06` — a single GC alone is insufficient; needs a corroborating tuning signal. Creative Energy fails both (power=58%, tutor=4%).

These are not yet implemented; flagging for 7174n1c alongside the existing PR #513 recommendation. Neither breaks R1/R2 — both Urza/Blast/Family Matters have GC=0 and would still be correctly demoted under either α or β.

### 2. **Cycling-explosion bug is more general than R1+R2 thought — Squirreled Away shows 287 loops, food-token mechanic** (severity: MEDIUM-HIGH, **EXPANDS R1+R2 §2**)

R1+R2 flagged the cycling-loop combo-detector blowup on Blast from the Past (4 false combos) and Timeless Wisdom (27,879 false combos) — both involving cycling lands as the false-loop substrate. R3 finds the same bug pattern on **Squirreled Away (BLB Hazel)** with **287 detected loops** — and Hazel's deck doesn't run cycling. The false-positive substrate this time is **food tokens** (eat-to-trigger consume-once tokens) and **squirrel ETB triggers**. Sample loop signature:

> 287 "true-infinite/determined" entries — the loop detector pairs every food-generating card with every food-payoff card as a renewable cycle, even though each food token is consumed permanently when sacrificed.

The bug is therefore not specific to cycling — it's the general "consume-once trigger treated as renewable producer" pattern. Cycling, food, treasure, blood, kicker, suspend, encore, evoke, dash, foretell, plot, and any other "do-it-once-then-the-card-is-gone" mechanic plausibly trip it. Fix surface (still un-investigated): the loop detector's "renewable producer" check needs an explicit exclusion list of one-shot trigger keywords.

### 3. **B1 false-positives appear in R3 — possible parallel to PR #513 §2** (severity: MEDIUM, NEW)

R3 shows 3 B1 ("Exhibition") calls on decks the vibes test predicts B2:
- **Mirror Mastery (C11 Riku):** measured=1, raw_score=1. Riku is a vanilla copy commander; raw score is low because nothing on the ladder hits. But the deck has commander_synergy=46.6%, a clear archetype (midrange), and mana_grade=A. Calling this Exhibition reads cold.
- **Eternal Bargain (C13 Oloro):** measured=1, raw_score=0. Oloro is a lifegain control commander. raw=0 on the ladder. Lifegain isn't represented as a "winning archetype" in the score ladder, so the deck has no positive contribution. Probably correctly cold for an unedited C13 precon, but borderline.
- **Planar Portal (AFR Prosper):** measured=1, raw_score=1. Power_pct=73 (highest in R3 corpus), commander_synergy=86.7%, mana_grade=A. This deck has the HIGHEST cross-reference power signal in R3 AND lands at B1. Strong B1 false-positive signal — calling Prosper "Exhibition" is plainly wrong.

The R1 finding "B1 call on Mystic Intellect" had the same shape: a deck whose cross-reference metrics (synergy + mana + power_pct) all point at B2-B3 lands at B1 because the score ladder doesn't reward the deck's specific shape. This is now 4 cases across 45 decks (R1 Mystic Intellect, R3 Mirror Mastery + Eternal Bargain + Planar Portal — though Eternal Bargain is borderline). The fix surface is the same as PR #513 §2's R1 suggestion: tighten the "no score, no B1" path to consult power_percentile or commander_synergy as a B2 floor.

### 4. **The Hosts of Mordor (LTR Sauron) lands at B3 — defensible** (severity: LOW)

Sauron precon: raw=6 with GC=1 (Sauron's the One Ring? or Cabal Coffers?), mana_grade=A, power_pct=63, combo_density=2. The GC=1-3 ceiling correctly caps at B3, and the deck lands there. This is plausibly the correct call for the LTR Mordor precon — it's a tuned tribal Orc/Wraith aggro/midrange that some communities argue plays at B3 anyway. Logged as a +1 Δ in the table but flagged here as "engine may be right; vibes prediction may be conservative."

### 5. **plays_like under-rating reproduces, direction unchanged** (severity: MEDIUM, **CONFIRMS R1+R2 §3**)

R3: 8/15 disagree, 7/8 in the "plays_like cooler than measured" direction. Combined across 45 decks: 25/45 (56%) disagree, **22/25 (88%) in the same direction**. The `estimatePlaysLike()` simulator continues to under-rate "Core" precons as "Exhibition" — at this point this is a fixable systematic bias, not noise. Fix surface unchanged: `cmd/hexdek-freya/archetype.go:1246`.

## Cumulative summary (45 stock precons)

| Finding | Status |
|---|---|
| Tuned-redundancy floor mis-fires on stock precons | **6/45 (13%)** — predicate confirmed systemic |
| Floor overrides GC=0 ceiling unconditionally | **3/45 (7%)** — Blast (R1), Family Matters (R2), Squirreled Away (R3) — pattern confirmed |
| PR #513's recommended predicate fix would catch all known false-positives | **NO — Creative Energy is a counter-example.** Fix needs refinement (options α/β above) |
| Cycling/consume-once loop detector mis-fires | **3 decks confirmed** (Blast, Timeless Wisdom, Squirreled Away). Not specific to cycling — also affects food tokens. |
| `plays_like` under-rates measured bracket | **22/25 disagreements in same direction (88%)** — systemic bias confirmed |
| B1 false-positives on the algorithm's "cold" end | **4 decks** (Mystic Intellect R1, Mirror Mastery R3, Eternal Bargain R3, Planar Portal R3) — parallel issue to B4 over-firing |

## Open decisions for 7174n1c (carried from R2 summary, updated)

1. **Implement PR #513's recommended fix** (engine work). R3 shows the fix as-designed catches 5 of 6 known false-positives. Decide whether to (a) ship as-designed and accept the 1/45 Creative Energy edge case, or (b) tighten with option α or β before shipping.
2. **Fix the consume-once loop detector** — now 3-deck confirmed; the food-token reproduction widens the scope from "cycling-only" to "consume-once mechanic in general." Probably a 2-3 line addition to the loop-detector's "renewable producer" check.
3. **Investigate the B1 floor case** — 4 known false-positives across 45 decks (9% rate). Same scale of issue as the B4 ceiling case PR #513 traced.
4. **Trace `estimatePlaysLike()`** — direction-consistent bias across 22/25 disagreements, now strongly motivated.

## Reproducing

```bash
# Re-import any R3 precon:
go run ./cmd/hexdek-import/ --moxfield https://moxfield.com/decks/<id> --owner wizards

# Trace any deck's bracket call (the "Bracket rationale" header shows all signals + adjustments):
go run ./cmd/hexdek-freya/ --deck data/decks/wizards/<slug>.txt 2>&1 | grep -A 15 'Bracket rationale'

# Extract measured_bracket directly (NOT from strategy.json — use --format json):
go run ./cmd/hexdek-freya/ --deck <path> --format json | jq '.archetype | {bracket, measured_bracket, bracket_rationale}'
```

15 R3 source URLs (all `https://moxfield.com/decks/<id>` in the `Commander Precons` namespace, none overlapping R1 or R2):

| Precon | Moxfield ID |
|--------|-------------|
| Mirror Mastery (C11) | `70auYSm75E-Iwf4Oc0g7Lg` |
| Eternal Bargain (C13) | `S2lFhE-IPUe6etycgIBVow` |
| Built from Scratch (C14) | `dDRyzSWi_kegywH3X_WtQA` |
| Faceless Menace (C19) | `gBEKby17r0OMidKZzGZfYQ` |
| Enhanced Evolution (C20) | `LWso_KepWEGAF8XjYQuH9Q` |
| Lorehold Legacies (C21) | `Vgzv5-wfcUqwUbd77Flqhw` |
| Planar Portal (AFR) | `3JwMHo6ANUeV4t-0eNLA5A` |
| Undead Unleashed (MID) | `DzZRd4RZDEWK4FwRJdSccw` |
| Painbow (DMU) | `VI61h_QqUEKf5tLvM5Pdcg` |
| Forces of the Imperium (40K) | `pkvfXrVXCkyG5L3OzShe2Q` |
| The Hosts of Mordor (LTR) | `Z4cD-XEZRUuH4W9jB_SvuA` |
| Paradox Power (WHO) | `l-e7-HDtsk2hXy5HK7ASuQ` |
| Creative Energy (MH3) | `IDfskZ7CAE2kBVwbWv3cpQ` |
| Squirreled Away (BLB) | `sxWCdd1NJ0q5JytH2RJyuA` |
| Endless Punishment (DSK) | `pnGkUqRJ5EyzT3J7OSlWeg` |
