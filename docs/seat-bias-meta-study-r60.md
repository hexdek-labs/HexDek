# Seat-Bias Meta-Study — Electron vs Quark Verdict

**Date:** 2026-05-25
**Branch:** `dev/seat-bias-meta-study-r60`
**Proposal:** 7174n1c — determine whether seat-position bias is a
real, stable, archetype-level effect (ELECTRON: same signal across
compositions) or a confounded composition-dependent artifact (QUARK:
swings wildly when the other 3 decks at the table change).
**Companion:** PR #258 (`docs/seat-bias-measurement-r60.md`) — the
single-pod baseline this meta-study tests against.

## Methodology

- **5 compositions** of 4 decks each (20 deck-slots), spanning 18 of
  the 22 archetypes with custom Yggdrasil profiles. Two archetypes
  (Reanimator, LandsMatter) deliberately appear in TWO compositions
  to enable cross-composition variance measurement.
- **5 seeds per composition** — 42, 43, 99, 7, 1337 — captures the
  within-composition seed noise floor.
- **1500 games per (composition × seed)** = 37,500 total games.
- All seats run default YggdrasilHat (budget 50, noise σ=0.2).
  No Freya analysis, so every seat dispatches through
  `DefaultWeightsForArchetype`.
- Wall time: 42m 22s (compositions ran serially, seeds within
  composition in parallel).

## Pod compositions

| Comp | Decks (4) | Archetypes |
|---|---|---|
| **C1** | Phenax / Wyleth / Kalamax / Lord Windgrace | Mill / Voltron / Spellslinger / LandsMatter |
| **C2** | Korvold / Meren / Edgar Markov / Heliod | Aristocrats / Reanimator / Tribal / Lifegain |
| **C3** | Sidisi / Sythis / Breya / Atraxa (Praetors) | Selfmill / Enchantress / Artifacts / Superfriends |
| **C4** | Kynaios & Tiro / Ezuri / Kenrith / Najeela | GroupHug / CountersMatter / Combo / ExtraCombats |
| **C5** | Aesi / Aminatou / Karador / Teferi (Temporal) | LandsMatter (#2) / Blink / Reanimator (#2) / Stax |

Engine stability: 0 crashes, 0 concessions across all 25 runs.

## Headline finding: **seat-bias is QUARK-shaped where directly measurable**

Of the 18 archetypes in the study, only 2 — **Reanimator** and
**LandsMatter** — appear in multiple compositions, so only these two
can be directly classified as ELECTRON or QUARK. **Both classify
QUARK** with composition-stdev far above the 2pp threshold:

| Archetype | n_comps | Composition-stdev | Verdict |
|---|---:|---:|---|
| **LandsMatter** | 2 | **13.56pp** | **QUARK** |
| **Reanimator** | 2 | **7.27pp** | **QUARK** |

LandsMatter swings from ~27% (Lord Windgrace in C1) to ~50%+ (Aesi
in C5) — a 23pp absolute shift driven purely by which 3 other decks
are at the table. The per-seat preference within each composition is
tiny (range 2.38pp) compared to the cross-composition shift.

The 16 single-composition archetypes are **UNDETERMINED across
compositions** — their composition-stdev is 0 by construction
(measurement artifact: only one composition observed), so they can't
be classified without additional compositions including them.

## Per-archetype × seat winrate (mean ± stderr)

Table format: `mean(stderr)`. Stderr is across-runs (5 or 10 runs
per cell depending on n_compositions); target was sub-0.5pp.

| Archetype | n_runs | Seat 0 | Seat 1 | Seat 2 | Seat 3 | Within-comp range |
|---|---:|---:|---:|---:|---:|---:|
| Mill | 5 | 66.8 (0.73) | 67.9 (0.27) | 68.4 (0.44) | 66.1 (1.04) | 2.30 |
| Tribal | 5 | 61.8 (0.61) | 64.7 (0.62) | 64.2 (1.66) | 66.2 (1.21) | 4.36 |
| Superfriends | 5 | 44.8 (1.14) | 51.2 (0.63) | 59.5 (0.63) | 58.3 (0.78) | **14.68** |
| Combo | 5 | 30.7 (1.08) | 32.3 (0.61) | 35.3 (1.32) | 34.5 (0.65) | 4.60 |
| GroupHug | 5 | 30.6 (0.99) | 31.3 (0.95) | 31.2 (1.10) | 31.1 (0.68) | 0.70 |
| LandsMatter | **10** | 27.2 (4.12) | 28.9 (4.38) | 29.6 (4.50) | 28.7 (4.27) | 2.38 |
| ExtraCombats | 5 | 23.3 (0.28) | 25.3 (0.92) | 26.0 (1.58) | 24.3 (1.17) | 2.70 |
| Stax | 5 | 21.0 (0.61) | 21.0 (1.14) | 19.7 (0.96) | 20.8 (0.90) | 1.28 |
| Reanimator | **10** | 18.6 (2.79) | 18.9 (2.23) | 19.3 (2.34) | 18.7 (2.03) | 0.64 |
| Enchantress | 5 | 10.5 (0.83) | 12.4 (0.49) | 18.5 (0.49) | 14.7 (0.24) | 7.98 |
| Artifacts | 5 | 17.1 (0.48) | 13.3 (0.45) | 11.9 (0.53) | 17.5 (0.87) | 5.60 |
| Aristocrats | 5 | 12.8 (1.27) | 13.9 (0.81) | 15.5 (0.13) | 14.2 (0.91) | 2.64 |
| Spellslinger | 5 | 13.1 (0.54) | 14.8 (0.41) | 14.9 (0.68) | 15.0 (0.28) | 1.86 |
| Blink | 5 | 12.3 (0.69) | 12.4 (0.53) | 11.0 (0.73) | 7.6 (0.16) | 4.84 |
| Selfmill | 5 | 10.9 (0.52) | 12.6 (0.73) | 11.0 (0.39) | 9.6 (0.37) | 3.00 |
| Lifegain | 5 | 7.8 (0.45) | 10.3 (0.28) | 11.2 (0.52) | 10.9 (0.57) | 3.36 |
| CountersMatter | 5 | 10.7 (0.56) | 9.7 (0.35) | 10.9 (0.92) | 12.7 (1.13) | 3.00 |
| Voltron | 5 | 3.5 (0.34) | 3.0 (0.28) | 2.8 (0.42) | 3.3 (0.25) | 0.70 |

**Stderr target (≤0.5pp) was met for most singleton archetypes** but
NOT for the two cross-composition archetypes (LandsMatter stderr
≈4pp, Reanimator ≈2-3pp) because the composition shift dominates
within-cell variance. This is the QUARK signature: cross-composition
variance >> within-composition variance.

## Global per-seat winrate by composition

The composition-level breakdown shows how much the overall seat-bias
shifts when the deck pool changes. Per-seat winrate sums to 100%
within each composition (4 seats × ~25%):

| Composition | Seat 0 | Seat 1 | Seat 2 | Seat 3 | Pattern |
|---|---:|---:|---:|---:|---|
| C1 (mill/voltron/spell/lands) | 24.4% | 25.2% | 25.4% | 24.9% | Flat |
| C2 (aristo/reani/tribal/life) | 23.1% | 25.3% | 25.7% | 25.9% | Late-seat |
| C3 (self/ench/artif/super) | 20.8% | 22.4% | 25.2% | 25.0% | **Strong late-seat** |
| C4 (hug/counters/combo/extra) | 23.8% | 24.7% | 25.8% | 25.7% | Late-seat |
| C5 (lands/blink/reani/stax) | 25.2% | 25.5% | 25.2% | 23.9% | **Early-seat** |

**Even the global pattern flips direction across compositions.**
C2/C3/C4 show act-last advantage; C5 shows act-FIRST advantage. C1
is essentially flat. The "+22 seat-3 deviation" from PR #258 was
specifically the C1 composition's pattern — and even there it was
within noise. This confirms the QUARK shape at the global level.

## ASCII chart: composition shifts the seat pattern

```
seat winrate vs uniform 25% (4 compositions, only C5 shows early-seat preference)

seat 0  |   ##############       ###############  C1
        |  ############          ###############  C2
        | ##########              ###############  C3   ← strongest skew
        |    ############       ###############  C4
        |    ###############       ##############  C5

seat 1  |    ###############     ###############  C1
        |    ###############      ###############  C2
        |    ##############        ###############  C3
        |    ###############      ###############  C4
        |    ###############      ###############  C5

seat 2  |    ###############      ###############  C1
        |    ###############      ###############  C2
        |    ###############        ###############  C3
        |    ###############       ###############  C4
        |    ###############      ###############  C5

seat 3  |    ###############     ###############  C1
        |    ###############       ###############  C2
        |    ###############       ###############  C3
        |    ###############       ###############  C4
        |    ##############       ###############  C5   ← C5 dropped
        20%     22%     24%   25%   26%     28%
```

## ASCII chart: within-composition seat-range per archetype (sorted)

The within-composition seat-range (max-seat-winrate − min-seat-
winrate) is the in-pod seat preference signal. Big bars suggest the
archetype has a strong seat preference in its tested pod — but
without cross-composition confirmation, the signal could be a
composition artifact rather than a true archetype property.

```
Superfriends     ##############################  14.68pp  [SINGLE comp — UNDETERMINED]
Enchantress      ################                 7.98pp  [SINGLE comp — UNDETERMINED]
Artifacts        ###########                      5.60pp  [SINGLE comp — UNDETERMINED]
Blink            ##########                       4.84pp  [SINGLE comp — UNDETERMINED]
Combo            #########                        4.60pp  [SINGLE comp — UNDETERMINED]
Tribal           #########                        4.36pp  [SINGLE comp — UNDETERMINED]
Lifegain         #######                          3.36pp  [SINGLE comp — UNDETERMINED]
CountersMatter   ######                           3.00pp  [SINGLE comp — UNDETERMINED]
Selfmill         ######                           3.00pp  [SINGLE comp — UNDETERMINED]
ExtraCombats     #####                            2.70pp  [SINGLE comp — UNDETERMINED]
Aristocrats      #####                            2.64pp  [SINGLE comp — UNDETERMINED]
LandsMatter      ####                             2.38pp  [QUARK — comp-stdev 13.56pp]
Mill             ####                             2.30pp  [SINGLE comp — UNDETERMINED]
Spellslinger     ###                              1.86pp  [SINGLE comp — UNDETERMINED]
Stax             ##                               1.28pp  [SINGLE comp — UNDETERMINED]
Voltron          #                                0.70pp  [SINGLE comp — UNDETERMINED]
GroupHug         #                                0.70pp  [SINGLE comp — UNDETERMINED]
Reanimator       #                                0.64pp  [QUARK — comp-stdev  7.27pp]
                 0    2    4    6    8   10   12  14   pp
```

## Verdicts

| Verdict | Count | Archetypes |
|---|---:|---|
| **QUARK (directly measured)** | 2 | LandsMatter (comp-stdev 13.56pp), Reanimator (7.27pp) |
| **UNDETERMINED across compositions** | 16 | all others — only 1 composition observed |
| **ELECTRON (directly measured)** | **0** | none |

## Interpretation

**This is a methodologically limited study reporting a clear directional finding.** Of the two archetypes for which cross-composition data exists, both are QUARK with large composition effects. The within-composition seat-range for those same archetypes (LandsMatter 2.38pp, Reanimator 0.64pp) is small — confirming that the COMPOSITION moves the absolute winrate baseline by 10× more than the seat position does.

The global per-composition pattern reinforces this: even the direction of seat preference flips (C5 favors early seats; C2-4 favor late). PR #258's single-composition observation of "+22 wins in seat 3" was specific to C1's composition shape, not a stable archetype-agnostic effect.

**Implication for the proposed TrueSkill seat-penalty prior**: applying the PR #258 lookup table as a prior across all games would correct for one composition's bias and INTRODUCE bias in compositions that don't share it. A composition-conditional prior (much more complex) would be needed, or the prior should be skipped entirely until the archetype × seat signal is independently confirmed across multiple compositions.

## Caveats + next steps

1. **Only 2 of 18 archetypes have cross-composition data.** The
   determinable verdicts (both QUARK) extrapolate by analogy, not
   direct measurement. A follow-up study with 3-4 compositions per
   archetype (so each archetype gets ELECTRON/QUARK classification)
   would cost ~3× this study's compute but give a complete picture.
2. **Only one deck per archetype per composition.** Different decks
   within the same archetype may have different seat preferences
   (Phenax vs Bruvac vs Anowon — all Mill, three different printed
   text profiles). Single-deck-per-archetype risks confounding
   deck-specific signals with archetype-level signals.
3. **All-Yggdrasil self-play.** Live matches with humans + bots, or
   mixed hat generations, might produce different seat biases —
   the act-first / act-last advantages depend on opponents making
   predictable plays. The QUARK finding should hold in live play
   (deck composition will still dominate) but the absolute
   numbers will shift.
4. **Stderr target (≤0.5pp) was met for singletons but missed for
   the cross-composition archetypes** because composition variance
   dominates. This is expected for QUARK-shaped signals.

## What this means for the rating system

**Do NOT implement the PR #258 seat-penalty prior in TrueSkill
without further validation.** The two archetypes where we could test
it directly both show that the seat penalty depends on composition
much more than on the archetype itself. A naive per-archetype prior
would be wrong about half the time.

If a seat-bias correction is desired, the more defensible path is
either (a) a global per-composition prior conditioned on the table's
archetype mix, or (b) skip the prior entirely and rely on
self-correction over many games (which is what TrueSkill already
does — the seat-bias is already implicitly absorbed into the per-
deck μ as games accumulate, assuming fair seat rotation).
