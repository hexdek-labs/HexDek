# Stack & Timing Interaction Harness Report

_100 iterations × 4 deck contexts per interaction = 400 reps per interaction. 5 interactions total._

## Classification

- **pass** — engine behavior matches the rules expectation
- **fail** — engine produced a rules-incorrect outcome
- **paradox** — engine raised / self-contradicted (e.g. unbounded stack, exception)
- **parser_gap** — structural parse is present but the resolver/engine can't evaluate it (neutral, not a rules violation)

## Summary matrix

| Interaction | pass | fail | paradox | parser_gap |
|---|---:|---:|---:|---:|
| **stifle_vs_fetch** — Stifle counters fetchland trigger | 400 | 0 | 0 | 0 |
| **split_second** — Counterspell denied by split-second | 0 | 0 | 0 | 400 |
| **mindslaver_loop** — Mindslaver + Academy Ruins cross-turn loop | 0 | 0 | 0 | 400 |
| **teferi_instant** — Teferi sorcery-speed-only opp restriction | 400 | 0 | 0 | 0 |
| **panglacial_wurm** — Cast from library during search | 0 | 0 | 0 | 400 |

## stifle_vs_fetch

_Stifle counters fetchland trigger_

| Deck context | pass | fail | paradox | parser_gap |
|---|---:|---:|---:|---:|
| Burn | 100 | 0 | 0 | 0 |
| Control | 100 | 0 | 0 | 0 |
| Creatures | 100 | 0 | 0 | 0 |
| Ramp | 100 | 0 | 0 | 0 |

## split_second

_Counterspell denied by split-second_

| Deck context | pass | fail | paradox | parser_gap |
|---|---:|---:|---:|---:|
| Burn | 0 | 0 | 0 | 100 |
| Control | 0 | 0 | 0 | 100 |
| Creatures | 0 | 0 | 0 | 100 |
| Ramp | 0 | 0 | 0 | 100 |

**Representative notes:**

- [Burn] Krosan Grip resolved but target artifact wasn't destroyed — destroy resolver targeting gap (not a split-second bug)

## mindslaver_loop

_Mindslaver + Academy Ruins cross-turn loop_

| Deck context | pass | fail | paradox | parser_gap |
|---|---:|---:|---:|---:|
| Burn | 0 | 0 | 0 | 100 |
| Control | 0 | 0 | 0 | 100 |
| Creatures | 0 | 0 | 0 | 100 |
| Ramp | 0 | 0 | 0 | 100 |

**Representative notes:**

- [Burn] Mindslaver loop mechanics work (activate→gy→recurse→draw) but 'control target player' effect is UnknownEffect — loop is mechanically valid, payoff unrealized

## teferi_instant

_Teferi sorcery-speed-only opp restriction_

| Deck context | pass | fail | paradox | parser_gap |
|---|---:|---:|---:|---:|
| Burn | 100 | 0 | 0 | 0 |
| Control | 100 | 0 | 0 | 0 |
| Creatures | 100 | 0 | 0 | 0 |
| Ramp | 100 | 0 | 0 | 0 |

## panglacial_wurm

_Cast from library during search_

| Deck context | pass | fail | paradox | parser_gap |
|---|---:|---:|---:|---:|
| Burn | 0 | 0 | 0 | 100 |
| Control | 0 | 0 | 0 | 100 |
| Creatures | 0 | 0 | 0 | 100 |
| Ramp | 0 | 0 | 0 | 100 |

**Representative notes:**

- [Burn] Panglacial Wurm cast-from-library static not parsed as a recognizable Modification kind (likely 'custom' with long-tail)

## Findings

- **stifle_vs_fetch**: all 400/400 pass.
- **split_second**: 100% parser_gap — interaction cannot be evaluated end-to-end yet.
- **mindslaver_loop**: 100% parser_gap — interaction cannot be evaluated end-to-end yet.
- **teferi_instant**: all 400/400 pass.
- **panglacial_wurm**: 100% parser_gap — interaction cannot be evaluated end-to-end yet.

## Files written

- `data/rules/interaction_harness_timing_report.md`
