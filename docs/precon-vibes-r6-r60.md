# Precon Vibes-Bracket Calibration (R60, Round 6)

> **HISTORICAL — `plays_like` removed (dev/remove-plays-like-r60).** This doc references the `plays_like` signal and its `estimatePlaysLike()` simulator as a discrete code path. That scaffolding has since been removed end-to-end (Go, JSON, DB-comment, UI, glossary, tests). `measured_bracket` is now the canonical felt-power measurement; the "no wincon → floppy" semantics that `plays_like` tracked are queued for absorption into `measured_bracket` as a separate calibration chapter. The Plays-Like column has been dropped from the ranked table below; historical findings and disagreement-rate rollups are preserved as research evidence.

## Why this is round 6

R1 (PR #508), R2 (PR #523), R3 (PR #532), and R5 (whenever it merges — R4 never landed per the note in R5's writeup) built the unedited-WotC-precon vibes-bracket baseline at N=60 decks. The cumulative finding through R5 was a stable ~11-13% B4 false-positive rate against the `Tuned-redundancy floor` predicate, an ~88% direction-consistent `plays_like`-cooler-than-`measured` bias, and a B4-fix design (PR #513 recommendation) with one known counter-example (Creative Energy from R3 satisfies the `gameChangerCount >= 1` disjunct via Mana Vault). R6 adds 15 more disjoint precons to bring the corpus to 75. Per dev-1's R5 note: 30+ unimported precons remain in the WotC catalog.

The point of R6 is to (a) keep pinning down the predicate-mis-fire rate at the N=75 sample size, (b) check whether the modern-precon power-creep observed in R5 continues to drive `measured_bracket` cooler than the heuristic declared-bracket rule, and (c) test PR #513's recommended fix against any new shape of false-positive R6 surfaces. Spoiler: R6 finds **4 B4 false-positives (highest single-wave rate yet)** AND **a second counter-example** to the recommended fix (Graveyard Overdrive's 9% tutor density satisfies the proposed `tutorDensity >= 0.08` disjunct).

**Naming convention (post-PR #529 refactor):** `bracket` on `DeckProfile`/`archetype` is the **declared** value (auto-stamped B2 for any deck under `data/decks/wizards/`). `measured_bracket` is what `estimateMeasuredBracket` computes. R6 reports `measured_bracket` in the table and tabulates Δ as **declared − measured** (R1-R5 sign convention) — negative Δ = engine hotter than declared, positive Δ = engine cooler.

## Methodology

Identical to R3-R5:
- 15 NEW Moxfield community uploads of stock WotC precon decklists. Zero overlap with the R1+R2+R3+R5 set of 60 — every Moxfield ID below was cross-referenced against `ls data/decks/wizards/*.txt` before picks were locked in. All from the canonical "Commander Precons" Moxfield namespace.
- `go run ./cmd/hexdek-import/ --moxfield <url> --owner wizards` — Freya auto-runs on import.
- `measured_bracket` extracted via `go run ./cmd/hexdek-freya/ --deck <path> --format json` (the strategy.json sidecar still hasn't been updated to expose `measured_bracket` — its `bracket` field is now the rubber-stamped declared value). `plays_like` is still only available in the strategy.json sidecar.

R6 picks deliberately mine the previously-untouched corners of the WotC catalog: all 3 R6 Era-1 picks come from C15 (entirely fresh — R1-R5 never picked from C15), R6 Era-3 picks span the previously-untouched ONE / NEC product lines, and R6 Era-5 includes TDM (Tarkir Dragonstorm, the most recent set as of 2026-05-26).

## Ranked Table

| # | Era | Precon | Commander | Archetype | Meas Brkt | Cmdr Syn % | Win Lines | Combo Dens | Chain Depth (avg/max) | Recursion Ratio | GC | Power % | Mana | avgCMC | **Decl Brkt** | Δ |
|---|-----|--------|-----------|-----------|:---------:|:----------:|:---------:|:----------:|:---------------------:|:---------------:|:--:|:-------:|:----:|:------:|:--------------:|:-:|
|  1 | C15 | Wade Into Battle      | Kalemne, Disciple of Iroas    | tribal       | **4 Optimized** | 28.3 |  3 | 2 | 2.25 / 3 | 0.00 | 0 | 45 | B | 4.27 | **2** | **−2** |
|  2 | C15 | Plunder the Graves    | Meren of Clan Nel Toth        | midrange     | 2 Core          | 39.3 |  3 | 2 | 2.80 / 3 | 0.40 | 0 | 50 | A | 4.00 | **2** | ✓ |
|  3 | C15 | Seize Control         | Mizzix of the Izmagnus        | spellslinger | **1 Exhibition**| 73.3 |  3 | 1 | 2.33 / 3 | 0.33 | 0 | 63 | A | 3.83 | **3** | +2 |
|  4 | C19 | Merciless Rage        | Anje Falkenrath               | midrange     | 2 Core          | 55.9 |  3 | 1 | 2.62 / 3 | 0.25 | 0 | 38 | D | 4.19 | **2** | ✓ |
|  5 | C20 | Ruthless Regiment     | Jirina Kudro                  | midrange     | 2 Core          | 14.3 |  3 | 1 | 2.50 / 3 | 0.50 | 0 | 45 | C | 3.22 | **2** | ✓ |
|  6 | C21 | Silverquill Statement | Breena, the Demagogue         | tribal       | **1 Exhibition**| 50.8 |  3 | 1 | 2.40 / 3 | 0.40 | 0 | 60 | B | 3.88 | **3** | +2 |
|  7 | AFR | Aura of Courage       | Galea, Kindler of Hope        | voltron      | 2 Core          | 70.5 |  4 | 1 | 2.50 / 3 | 0.50 | 0 | 53 | C | 2.77 | **2** | ✓ |
|  8 | NEC | Maestros Massacre     | Anhelo, the Painter           | storm        | 2 Core          | 71.0 |  2 | 1 | 3.00 / 3 | 0.00 | 0 | 43 | B | 3.65 | **2** | ✓ |
|  9 | ONE | Corrupting Influence  | Ixhel, Scion of Atraxa        | midrange     | **4 Optimized** | 11.5 |  3 | 1 | 0.00 / 0 | 0.00 | 0 | 45 | C | 3.26 | **2** | **−2** |
| 10 | LTR | Food and Fellowship   | Sam + Frodo                   | midrange     | **4 Optimized** | 55.0 |  3 | 2 | 2.50 / 3 | 0.50 | 0 | 63 | B | 3.28 | **3** | **−1** |
| 11 | PIP | Mutant Menace         | The Wise Mothman              | selfmill     | 2 Core          | 70.0 |  3 | 1 | 2.60 / 3 | 0.60 | 0 | 55 | B | 3.52 | **2** | ✓ |
| 12 | PIP | Scrappy Survivors     | Dogmeat, Ever Loyal           | voltron      | **1 Exhibition**| 100.0 |  3 | 1 | 2.80 / 3 | 0.40 | 0 | 58 | B | 2.82 | **2** | +1 |
| 13 | MH3 | Graveyard Overdrive   | Disa the Restless             | combo        | **4 Optimized** | 63.3 |  3 | 2 | 2.57 / 3 | 0.43 | 0 | 60 | B | 3.68 | **3** | **−1** |
| 14 | BLB | Peace Offering        | Ms. Bumbleflower              | midrange     | 2 Core          | 80.3 |  3 | 1 | 2.00 / 2 | 0.50 | 0 | **81** | A | 3.21 | **4** | **+2** |
| 15 | TDM | Temur Roar            | Ureni of the Unwritten        | tribal       | 2 Core          | 14.5 |  3 | 1 | 2.00 / 2 | 0.00 | 0 | 45 | B | 4.05 | **2** | ✓ |

**Δ column:** declared_bracket − measured_bracket. `✓` = exact match. Negative Δ = engine hotter than declared. Positive Δ = engine cooler than declared.

**Aggregate:** 7/15 exact, 10/15 within ±1. **4/15 B4 false-positives** (Wade Into Battle, Corrupting Influence, Food and Fellowship, Graveyard Overdrive) — single-wave high so far. **3/15 B1 false-positives** (Seize Control, Silverquill Statement, Scrappy Survivors), continuing the R3 trend. **1/15 inverse miss** (Peace Offering at power_pct=81 / mana A / synergy 80% measures at B2).

## Cross-validation: cumulative 75-deck corpus

| Metric | R1 | R2 | R3 | R5 | R6 | **All 75** |
|---|:-:|:-:|:-:|:-:|:-:|:-:|
| Exact match | 11/15 (73%) | 12/15 (80%) | 8/15 (53%) | 4/15 (27%) | 7/15 (47%) | **42/75 (56%)** |
| Within ±1 | 13/15 (87%) | 14/15 (93%) | 12/15 (80%) | 13/15 (87%) | 10/15 (67%) | **62/75 (83%)** |
| B4 false-positives | 2 | 1 | 3 | 2 | **4** | **12/75 (16%)** |
| B1 false-positives | 1 | 0 | 3 | 0 | 3 | **7/75 (9%)** |
| Inverse "engine cooler than power_pct says" misses | 0 | 0 | 0 | 1 (Most Wanted) | 1 (Peace Offering) | **2/75 (3%)** |
| `plays_like` vs `measured` disagree | 9/15 | 8/15 | 8/15 | 8/15 | 8/15 | **41/75 (55%)** |
| Disagreements with `plays_like` COOLER | 8/9 | 7/8 | 7/8 | 8/8 | **8/8** | **38/41 (93%)** |

### What replicates and what's new

- **B4 false-positive rate is climbing** — 10%, 13%, 16% across the three N-cuts (N=30, N=45, N=75). The trend is real, not noise: as the corpus tilts toward modern precons (which ship more finishers per anthem and more fast-mana), the `Tuned-redundancy floor` predicate fires more often. **R6 is the first single-wave to hit 4 B4 false-positives**, equal to R1+R2 combined.
- **Tuned-redundancy floor is now 12-for-12** across the cumulative corpus. Every B4 false-positive across all 5 waves goes through this single predicate at `cmd/hexdek-freya/archetype.go:1229`. Settled.
- **Ceiling→floor override pattern: 6-for-12.** Half of the B4 false-positives have an explicit GC=0 ceiling demotion that the floor then undoes. R6 added 3 more (Wade Into Battle, Food and Fellowship, Graveyard Overdrive) on top of R1's Blast from the Past, R2's Family Matters, R3's Squirreled Away.
- **plays_like-cooler-than-measured direction is now 93% direction-consistent across 41 disagreements.** This is no longer "hypothesis from the data" — it's a firmly confirmed systematic bias. Fix surface is `estimatePlaysLike()`.
- **NEW IN R6:** Graveyard Overdrive (Disa the Restless) is a **second counter-example** to PR #513's recommended fix. Disa ships 7+ tutors (Entomb / Final Parting / Diabolic Tutor / Eladamri's Call class) which compute to 9% nonland tutor density — above the proposed disjunct's `tutorDensity >= 0.08` threshold. Under the recommended fix as-designed, the floor would STILL fire and Graveyard Overdrive would STILL land at B4. Combined with R3's Creative Energy (GC=1 from Mana Vault), this is now a 2-deck pattern: **single-marker disjuncts are too loose for stock precons that happen to ship the marker as part of WotC's stock content**.

## Findings

### 1. **R6 surfaces a second PR #513 fix counter-example: Graveyard Overdrive (MH3 Disa) trips the proposed `tutorDensity >= 0.08` disjunct** (severity: HIGH, **CRITICAL — pattern with R3's Creative Energy**)

```
Bracket rationale (raw score 7 → B4 Optimized):
  [+2] Tutor density (8-11%): 9% of nonlands
  [+2] Combo lines (2-4): 4 true-infinite/determined loops
  [-1] Average CMC (heavy (>3.5)): 3.7 avg
  [+2] Fast mana (6-9): 6 sub-2-CMC mana producers
  [+2] Finisher density (8+): 26 distinct finisher lines
  [ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
  [floor]   Tuned-redundancy floor: lifted to B4: 26 finishers + 6 fast-mana pieces (was B2)
```

The recommended PR #513 predicate:
```go
tunedRedundancy := finisherCount >= 8 && ctx.fastManaCount >= 6 &&
                   (ctx.gameChangerCount >= 1 || trueInfCount >= 1 ||
                    ctx.tutorDensity >= 0.08 ||              // ← Disa trips this
                    (ctx.avgCMC < 3.0 && manaGradeAtLeast(report, "B")))
```

Disa: GC=0, true_inf=0 (the "4 combo lines" are graveyard-loop false-positives, not actually true-infinites), tutorDensity=0.09 (above the 0.08 threshold), avgCMC=3.68 (above 3.0 — last conjunct fails). The third disjunct is satisfied by the stock precon's tutor suite, and the floor still fires.

**This is the second confirmed precon where the PR #513 fix as-designed wouldn't help.** R3 noted that single-GC precons (Creative Energy with Mana Vault) trip the first disjunct; R6 confirms that high-tutor-density graveyard precons trip the third. The "single independent B4 marker is sufficient" structure of the OR-chain is fundamentally too loose for stock precons.

**Refined fix options (carrying forward R3's α/β):**
- **Option α (raise GC threshold):** `gameChangerCount >= 2`. Catches Creative Energy but doesn't catch Disa (still 9% tutor).
- **Option β (require corroborating signal):** keep `gameChangerCount >= 1` but ALSO require `power_percentile >= 65` OR `tutor_density >= 0.06`. Catches Creative Energy. Does NOT catch Disa (power 60 < 65, tutor 9% > 6%).
- **NEW Option γ (raise tutor threshold + require corroborating signal):** AND-chain instead of OR-chain. `tunedRedundancy := finisherCount >= 8 && fastMana >= 6 && AT LEAST TWO OF (gc≥1, trueInf≥1, tutorDensity≥0.10, avgCMC<3.0, manaGrade≥A)`. Catches both Creative Energy (only GC=1 passes) and Disa (only tutor 9% passes — needs second signal that doesn't fire).
- **Option δ (precon-namespace gate):** any deck under `data/decks/wizards/` skips the tuned-redundancy floor entirely. Cleanest fix, but tightly couples the floor logic to the corpus directory — fragile if precons move.

R6 recommends 7174n1c look at options γ or δ rather than α/β; the single-disjunct OR-chain pattern is now demonstrably insufficient.

### 2. **R6 adds 3 more Tuned-redundancy floor hits — Wade Into Battle, Corrupting Influence, Food and Fellowship** (severity: HIGH)

#### 2A. Wade Into Battle (C15 Kalemne) — ceiling→floor override, fourth instance
```
[ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B3 on raw score)
[floor]   Tuned-redundancy floor: lifted to B4: 10 finishers + 6 fast-mana pieces (was B2)
```
Boros Giant tribal precon from 2015. Identical pattern shape to Blast from the Past (R1), Family Matters (R2), Squirreled Away (R3). The fact that a 2015 precon (one of the oldest in the corpus) trips the same pattern as 2024 precons confirms the predicate isn't specific to modern product — it's specific to ANY stock precon with anthem-flavored finishers and the standard precon ramp suite.

#### 2B. Corrupting Influence (ONE Ixhel) — pure floor-only, "35 distinct finisher lines"
```
Bracket rationale (raw score 4 → B4 Optimized):
  [+2] Fast mana (6-9): 6 sub-2-CMC mana producers
  [+2] Finisher density (8+): 35 distinct finisher lines
  [floor] Tuned-redundancy floor: lifted to B4: 35 finishers + 6 fast-mana pieces (was B2)
```
Phyrexian toxic precon from 2023. **35 distinct finisher lines is the highest finisher count in any precon in the 75-deck corpus** — pure manifestation of the finisher-line dedup bug PR #513 §3 noted. Toxic/proliferate decks like Ixhel get every proliferate enabler × every toxic creature counted as a separate finisher line; combinatorial blowup. Even a stricter floor predicate wouldn't fix this until the finisher counter itself is de-duplicated.

#### 2C. Food and Fellowship (LTR Sam + Frodo) — combines the floor pattern WITH the consume-once loop bug
```
Bracket rationale (raw score 8 → B4 Optimized):
  [+1] Tutor density (4-7%): 6% of nonlands
  [+3] Combo lines (5+): 41 true-infinite/determined loops
  [+2] Fast mana (6-9): 6 sub-2-CMC mana producers
  [+2] Finisher density (8+): 10 distinct finisher lines
  [ceiling] GC=0 ceiling: capped at B2: no Game Changers and no true-infinite combo (was B4 on raw score)
  [floor]   Tuned-redundancy floor: lifted to B4: 10 finishers + 6 fast-mana pieces (was B2)
```
Hobbit/Food tribal. **41 detected "true-infinite/determined loops"** — same consume-once token bug R3 flagged on Squirreled Away (287 loops on Hazel's food tribal). Both food precons trip the same false-positive: the loop detector treats food generation + food sacrifice as a renewable cycle when each food token is consumed permanently. Combined with cycling lands (Blast from the Past R1's 4 false combos, Timeless Wisdom R2's 27,879 false combos), the bug now has 4 confirmed instances across 2 consume-once mechanics (cycling, food).

### 3. **R6 adds 3 more B1 false-positives — pattern from R3 holds** (severity: MEDIUM-HIGH, R3 finding strongly reaffirmed)

R6 has 3 measured-B1 calls:
- **Seize Control (C15 Mizzix):** measured=1, declared=3. power_pct=63, synergy=73%, mana_grade=A, archetype=spellslinger. The deck's signal mass (a real Izzet storm/spell engine) is invisible to the score ladder because it has no GC, no true-infinite, and tutor density is low. Falling all the way to B1 is harsh.
- **Silverquill Statement (C21 Breena):** measured=1, declared=3. power_pct=60, synergy=51%, mana_grade=B. WB Inkling tribal that the ladder simply doesn't reward.
- **Scrappy Survivors (PIP Dogmeat):** measured=1, declared=2. commander_synergy **100%** (highest in R6) but power_pct=58, archetype=voltron. Mono-color voltron precon. The score ladder has nothing to reward voltron with.

Combined with R1's Mystic Intellect, R3's Mirror Mastery + Eternal Bargain + Planar Portal, R6 now brings the B1 false-positive total to **7/75 (9%)**. The pattern is consistent across 5 distinct archetypes (jeskai flashback, RUG copy-spells, Esper lifegain, Izzet artifacts, Izzet storm, WB Inkling tribal, Boros voltron). The common shape: a deck whose synergy/power/mana cross-references say "B2-B3" but whose `measured_bracket` falls to B1 because the score ladder doesn't reward the archetype.

This is the COLD-floor mirror of the B4 over-firing problem — same structural issue, opposite direction. Fix surface: tighten the "no positive ladder score → B1" path to consult `power_percentile`, `commander_synergy`, or `archetype` as a B2 floor.

### 4. **Peace Offering (BLB Ms. Bumbleflower) — second inverse miss, mirror to R5's Most Wanted** (severity: HIGH, R5 pattern reproduces)

| Signal | Value |
|---|---|
| `power_percentile` | **81** (highest in R6) |
| `mana_base_grade` | A |
| `commander_synergy` | 80.3% |
| `archetype` | midrange |
| `game_changer_count` | 0 |
| `true_infinites` | 0 |
| `measured_bracket` | **2 Core** |

R5's Most Wanted (OTJ Olivia) showed power_pct=81 / A mana / synergy=87% landing at B2 because GC=0 + no true-infinite fires the ceiling. Peace Offering shows the IDENTICAL signal profile — power_pct=81, A mana, synergy=80% — and lands at B2 for the same reason.

This is a clean confirmation that **the GC=0 ceiling itself has a calibration bug at the high-power end**: when `power_percentile >= 75 AND mana_grade = A AND commander_synergy >= 75%`, the deck is empirically a B3-B4 deck whether or not it ships a Game Changer card. The ceiling's binary "GC=0 OR true_inf=0 → B2 max" is correct as a default but should have an escape hatch for the obviously-tuned cross-signal profile. Fix surface: same source location as PR #513's ceiling logic (`cmd/hexdek-freya/archetype.go:1168`).

### 5. **plays_like-cooler-than-measured: 93% direction-consistent at N=75** (severity: MEDIUM, **CONFIRMED SYSTEMIC**)

R6: 8/8 disagreements are plays_like-cooler-than-measured. Cumulative across 75 decks: 38/41 disagreements (93%) in this direction. The R1+R2+R3 hypothesis from PR #525's R2 findings doc is now strongly confirmed. Fix surface unchanged: `estimatePlaysLike()` at `cmd/hexdek-freya/archetype.go:1246`.

## Cumulative summary (75 stock precons across R1-R3 + R5-R6)

| Finding | Status |
|---|---|
| Tuned-redundancy floor mis-fires on stock precons | **12/75 (16%)** — predicate confirmed systemic; rate climbing as corpus tilts modern |
| Ceiling→floor override pattern | **6/12 false-positives** — half of all B4 mis-fires |
| PR #513's recommended fix catches all known false-positives | **NO — 2 counter-examples now: Creative Energy (R3, GC=1) and Graveyard Overdrive (R6, tutor=9%).** Options γ/δ recommended (see §1) |
| Consume-once loop detector mis-fires | **4 decks confirmed** (Blast cycling, Timeless cycling, Squirreled food, Food&Fellowship food) — multi-mechanic |
| Finisher-line counter inflation | **Up to 35 finishers on one stock precon** (Corrupting Influence). Independent fix needed regardless of floor predicate |
| B1 false-positives on the "cold" end | **7/75 (9%)** — Mystic Intellect R1, Mirror+Eternal+Planar R3, Seize+Silverquill+Scrappy R6 |
| Inverse miss: high-power precon measures as B2 due to GC=0 ceiling | **2/75 (3%)** — Most Wanted R5, Peace Offering R6 |
| `plays_like` under-rates `measured_bracket` | **38/41 disagreements (93%)** — systemic bias confirmed |

## Open decisions for 7174n1c (carried forward, updated by R6)

1. **PR #513's recommended fix needs refinement before shipping** — 2 counter-examples now. Decide between options γ (multi-signal AND-chain) and δ (precon-namespace gate). Both close all known cases; γ is more general, δ is simpler.
2. **Fix the consume-once loop detector** — 4-deck confirmed across cycling + food. Probably the highest-leverage single fix because it ALSO causes the win-line count blowup the UI surfaces.
3. **Investigate the B1 floor case** — 7/75 (9%) rate is large enough to warrant a sibling-trace to PR #513. Same shape, opposite direction.
4. **Investigate the GC=0 ceiling escape hatch for high-cross-signal decks** — 2 confirmed instances (Most Wanted, Peace Offering). Fix surface in same source location as the existing ceiling.
5. **Trace `estimatePlaysLike()`** — 93% direction-consistent bias at N=75 is decisive. No further data needed.
6. **Fix the finisher-line counter dedup** — independent of bracket logic. Probably a finisher-classifier change in `cmd/hexdek-freya/advanced.go`.

## Reproducing

```bash
# Re-import any R6 precon:
go run ./cmd/hexdek-import/ --moxfield https://moxfield.com/decks/<id> --owner wizards

# Trace any deck's bracket call:
go run ./cmd/hexdek-freya/ --deck data/decks/wizards/<slug>.txt 2>&1 | grep -A 15 'Bracket rationale'

# Extract measured_bracket (NOT from strategy.json — use --format json):
go run ./cmd/hexdek-freya/ --deck <path> --format json | jq '.archetype | {bracket, measured_bracket, bracket_rationale}'
```

15 R6 source URLs (all `https://moxfield.com/decks/<id>` in the `Commander Precons` namespace, none overlapping R1/R2/R3/R5):

| Precon | Moxfield ID |
|--------|-------------|
| Wade Into Battle (C15) | `mxn1Iad46kSGT5NR6N_3_g` |
| Plunder the Graves (C15) | `EK6p0em0TEiKr4lVHSgWnA` |
| Seize Control (C15) | `qfOr1nvopUeSE7VOvrWx-A` |
| Merciless Rage (C19) | `z_LC4U8zVE6k4Z7EoxPV0g` |
| Ruthless Regiment (C20) | `7a4bmlDVbEeIlvXYrlTIIA` |
| Silverquill Statement (C21) | `_dM2RHtVoUqDHjMko8X4pQ` |
| Aura of Courage (AFR) | `C-kanWXqiE2nxFWMEouhAw` |
| Maestros Massacre (NEC) | `flLw5YvIOE2tBprZ_GTsmQ` |
| Corrupting Influence (ONE) | `PvZAYgl5MUWsWFdKKdyIww` |
| Food and Fellowship (LTR) | `S3X49Miklk6zsQk9VSrt2Q` |
| Mutant Menace (PIP) | `EE44WgAwhUOI0XRfFQqolQ` |
| Scrappy Survivors (PIP) | `Z2KWcwO1u0GvQC4gbVjPZw` |
| Graveyard Overdrive (MH3) | `p9lI8QQGH0eEeJmaPX0KVQ` |
| Peace Offering (BLB) | `bTMR5Ab1PU-5UzXaQ_OsgQ` |
| Temur Roar (TDM) | `dp8QKvCr3EqGF9-qu5Zzfg` |
