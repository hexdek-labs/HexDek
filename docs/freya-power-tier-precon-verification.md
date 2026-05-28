# Freya Power-Tier Classifier — Precon Corpus Verification

## Headline

**Verified the cEDH power-tier classifier (PR #714) against 87
imported WotC precons under `data/decks/wizards/`. Initial run
surfaced 1 over-classification (silverquill_influence → T4 "High
Power") — a real classifier bug. Closed the bug by adding a T4+
floor gate. Re-verified: 87/87 precons land at T1-T3, 0
over-classifications, matching the WotC bracket framework's design
intent (unedited precons are B2-B3 by design).**

## Run setup

- **Branch**: `dev/freya-power-tier-tests-r60`
- **Corpus**: `data/decks/wizards/*.txt` (87 imported WotC
  Commander precons spanning Commander 2011 through Edge of
  Eternities Commander 2025)
- **Classifier**: `ClassifyCEDHPowerTier` in
  `cmd/hexdek-freya/power_tier_cedh.go` (shipped PR #714)
- **Test**: `TestCEDHPowerTier_PreconCorpusVerification` in
  `cmd/hexdek-freya/power_tier_precon_verification_r60_test.go`

## Initial verification result (pre-fix)

| Tier | Label | Count | % |
|-----:|-------|------:|--:|
| T1 | Casual | 35 | 40.2% |
| T2 | Casual | 41 | 47.1% |
| T3 | Upgraded Precon | 10 | 11.5% |
| **T4** | **High Power** | **1** | **1.1%** |
| T5 | cEDH | 0 | 0.0% |

**Over-classification**: 1 precon
(`silverquill_influence_secrets_of_strixhaven`) reached T4
"High Power" despite being an unedited WotC precon.

### Root cause of the over-classification

Silverquill Influence signal breakdown:

| Signal | Vote | Measurement |
|--------|-----:|-------------|
| Game Changers | T1 | 0 GC |
| Mana Base Grade | T4 | Grade B |
| Nonland Tutors | T2 | 1 nonland tutor |
| Win-Line Confidence | T4 | 0.56 weighted avg |
| Interaction Package | T1 | 0.00 score |
| Avg CMC | T4 | 2.97 CMC |

Sorted votes: 1 / 1 / 2 / 4 / 4 / 4. Upper-mid median = position 3
= T4. The deck was lifted to "High Power" by three form-signals
(Mana B, decent Win-Line Confidence, low CMC) despite **0 Game
Changers, 1 tutor, 0.00 interaction score**.

This is structurally wrong: a deck with no GCs, no tutor depth,
AND no interaction can't meaningfully race regardless of how clean
its mana base / curve / win-line consistency reads. The cEDH
power-tier classifier's purpose is to discriminate cEDH-shape
(racing) from non-cEDH-shape, and form-signals alone don't make a
deck cEDH-shaped.

### Fix shipped (within this PR)

Added a **T4+ floor gate** to `ClassifyCEDHPowerTier` at
`cmd/hexdek-freya/power_tier_cedh.go`: to reach T4 (High Power) or
T5 (cEDH), the deck must trip at least 2 of three cEDH-shape
discriminators:

1. `GameChangerCount ≥ 1` (any WotC Game Changer at all)
2. `NonLandTutorCount ≥ 3` (real tutor density)
3. `InteractionPackage.Score ≥ 0.30` (non-trivial defensive
   package)

Decks with median votes ≥ 4 but fewer than 2 discriminators tripped
are capped at T3. Decks scoring median ≤ 3 are unaffected.

The gate runs BEFORE the existing B5 confirmation gate (so a
median-5 deck with insufficient discriminators is first demoted to
T3 by the T4+ floor, not T4 by the B5 gate). The two gates
together form the cEDH-shape ladder: T4 requires real
discriminator presence, T5 requires elevated discriminator
threshold + low CMC.

## Post-fix verification result

| Tier | Label | Count | % |
|-----:|-------|------:|--:|
| T1 | Casual | 35 | 40.2% |
| T2 | Casual | 41 | 47.1% |
| **T3** | **Upgraded Precon** | **11** | **12.6%** |
| T4 | High Power | 0 | 0.0% |
| T5 | cEDH | 0 | 0.0% |

**Over-classifications: 0.** Silverquill Influence correctly
demoted from T4 → T3 ("Upgraded Precon") — the gate flagged it as
having 0 of 3 discriminators (GC=0, Tutors=1<3, Interaction=0.00).

## Per-tier interpretation

### T1 Casual (35 / 40.2%)

The bulk of the older / non-curated precon line: Commander 2011
through Commander 2019 era, plus the lower-power thematic precons
from set-specific runs (Warhammer 40K Necron Dynasties, MH3
Eldrazi Incursion, AFR Dungeons of Death, the 5 Murders at Karlov
Manor precons, etc.). T1 score range 7-12.

Representative T1 names:
- `merciless_rage_commander_2019_precon_decklist.txt` (score 7)
- `tricky_terrain_modern_horizons_3_commander_precon_decklist.txt`
  (score 7)
- `eternal_bargain_commander_2013_precon_decklist.txt` (score 10)
- `mystic_intellect_commander_2019_precon_decklist.txt` (score 9)

### T2 Casual (41 / 47.1%)

The largest bucket — the modern WotC default for precon power.
Strixhaven (Quandrix / Witherbloom / Silverquill / Lorehold
Statement), Streets of New Capenna, Bloomburrow, AFR Aura of
Courage, etc. T2 score range 9-16.

Representative T2 names:
- `lorehold_legacies_commander_2021_precon_decklist.txt` (score 16)
- `seize_control_commander_2015_precon_decklist.txt` (score 16)
- `grand_larceny_outlaws_of_thunder_junction_commander_precon_decklist.txt` (score 15)

### T3 Upgraded Precon (11 / 12.6%)

The elevated precons. Roughly the ones with higher GC density
(Doctor Who's Blast from the Past has Time Stop, Time Spiral)
or higher tutor support (Ruthless Regiment, Tyranid Swarm) or
both. T3 score range 14-17.

Representative T3 names:
- `blast_from_the_past_doctor_who_commander_precon_decklist.txt`
  (score 17)
- `ruthless_regiment_commander_2020_precon_decklist.txt` (score 17)
- `tyranid_swarm_warhammer_40_000_commander_precon_decklist.txt`
  (score 16)
- `silverquill_influence_secrets_of_strixhaven_commander_precon_decklist.txt`
  (score 16) — post-fix; was T4 pre-fix

### T4 High Power (0 / 0.0%) ✓

Zero — matches expectation. Unedited WotC precons should not
reach High Power.

### T5 cEDH (0 / 0.0%) ✓

Zero — matches expectation. Unedited WotC precons cannot be cEDH.

## Why this matters

The verification did three things:

1. **Validated the classifier behaves correctly on the WotC-design
   power band.** 87/87 precons land in the documented intended
   range. The classifier passes the structural sanity check that
   "WotC precons read as casual / upgraded-precon, not High Power
   / cEDH."

2. **Surfaced and closed a real classifier bug.** The
   silverquill_influence T4 over-classification was a genuine miss
   — the upper-mid median tiebreak (which fixed the cEDH-B5 close
   misses during PR #714 calibration) introduced a sibling failure
   mode where 3 high form-signals + 3 low discriminator signals
   could elevate to T4. The T4+ floor gate closes the failure
   mode without regressing PR #714's calibration test (13/16
   exact still holds, voja ±2 divergence still documented).

3. **Established a regression line for future classifier tuning.**
   Any future threshold retune must keep this test green; if a
   tuning change re-introduces a precon T4+ over-call, the
   verification fails fast and surfaces which precon and which
   signal vote pattern caused it.

## Regression test

`TestCEDHPowerTier_PreconCorpusVerification` is now a permanent
regression test. The pass condition is binary: zero precons at
T4 or T5. Any over-classification is a hard test failure with the
deck name, tier, label, signal breakdown, and verdict logged.

Companion `TestCEDHPowerTier_PreconStatsDump` provides per-signal
distribution stats across the corpus for future tuning work.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git checkout dev/freya-power-tier-tests-r60
cd cmd/hexdek-freya
go test -run TestCEDHPowerTier_PreconCorpusVerification -count=1 -v .
```

Expected: `PASS`, tier distribution `T1=35 / T2=41 / T3=11 / T4=0
/ T5=0`, "Over-classification (T4+) count: 0".

## Calibration regression check

Re-ran the existing PR #714 calibration test
(`TestCEDHPowerTier_Calibration`) post-fix: still 13/16 exact on
the 16-deck reference corpus, voja still ±2 documented divergence,
all 9 cEDH-B5 decks still land at T5. The T4+ floor gate doesn't
regress the calibration — the cEDH and B4 nightmare decks have
GC≥3, tutors≥3, interaction≥0.42, so the discriminator gate
trivially passes (≥2 markers tripped).

## CLAUDE.md issue-log impact

Recommended Resolved-table entry:

> | 2026-05-28 | Precon verification PR #715 | **Freya cEDH power-tier classifier T4 false-positive on silverquill_influence_secrets_of_strixhaven precon** | Added T4+ floor gate to `ClassifyCEDHPowerTier` requiring ≥2 of {GC≥1, NonLandTutors≥3, InteractionPackage≥0.30} for any tier ≥4 call. Verified across all 87 imported WotC precons under `data/decks/wizards/`: 35 T1 / 41 T2 / 11 T3 / 0 T4 / 0 T5 — matches WotC bracket framework's design intent for unedited precons. PR #714's 16-deck calibration unaffected (13/16 exact still holds, all 9 cEDH-B5 decks still T5). |
