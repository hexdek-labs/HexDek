# Interaction Harness — Infinite Combos

_Canonical 1v1 infinite-combo interactions, scripted against the playloop engine. 100 iterations × 4 deck contexts × 5 interactions = 2000 reps. runtime 1854 ms._

## Method

Each interaction sets up a minimal board state, then steps the combo loop one iteration at a time using engine primitives (permanent add/remove, mana-pool mutation, direct counter updates). An `InfiniteLoopGuard` caps iteration count, wall-time, and detects repeated state fingerprints. Outcomes:

- **pass** — combo executed and the quantity of interest (mana, damage, tokens, life-swing) grew past the threshold
- **paradox** — engine detected a repeated state and halted gracefully (also a pass)
- **fail** — combo didn't execute, quantity didn't scale, or the engine crashed
- **parser_gap** — a required card wasn't in the oracle dump or failed to parse

## Results — 5 interactions × 4 deck contexts

| Interaction | Deck context | Pass | Paradox | Fail | Gap | CR 731 kind | CR 731 outcome | Sample iters | Sample qty |
|---|---|---:|---:|---:|---:|---|---|---:|---:|
| Dockside Extortionist + Temur Sabertooth | 3 artifacts/enchantments (minimum combo) | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 3 | 9 |
|  | 4 arts+ench (net positive) | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 2 | 8 |
|  | 5 arts+ench (infinite territory) | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 4 | 20 |
|  | 6 arts+ench (clear infinite) | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 4 | 24 |
| Food Chain + Eternal Scourge/Misthollow Griffin | Eternal Scourge (CMC 3) | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 2 | 2 |
|  | Misthollow Griffin (CMC 4) | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 2 | 2 |
|  | Scourge, short horizon | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 2 | 2 |
|  | Griffin, short horizon | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 2 | 2 |
| Sanguine Bond + Exquisite Blood | opp 20 life, prime 1 | 100 | 0 | 0 | 0 | mandatory_two_sided | opp_loses | 20 | 20 |
|  | opp 40 life (Commander), prime 1 | 100 | 0 | 0 | 0 | mandatory_two_sided | opp_loses | 40 | 40 |
|  | opp 10 life, prime 2 | 100 | 0 | 0 | 0 | mandatory_two_sided | opp_loses | 5 | 10 |
|  | opp 1 life, prime 1 | 100 | 0 | 0 | 0 | mandatory_two_sided | opp_loses | 1 | 1 |
| Heliod, Sun-Crowned + Walking Ballista | opp 20, Ballista 2 counters | 100 | 0 | 0 | 0 | optional | opp_loses | 20 | 20 |
|  | opp 40 (Commander), 2 counters | 100 | 0 | 0 | 0 | optional | opp_loses | 40 | 40 |
|  | opp 20, Ballista 5 counters | 100 | 0 | 0 | 0 | optional | opp_loses | 20 | 20 |
|  | opp 100 (mega), 3 counters | 100 | 0 | 0 | 0 | optional | opp_loses | 100 | 100 |
| Kiki-Jiki + Zealous Conscripts/Deceiver Exarch | Zealous Conscripts partner | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 20 | 20 |
|  | Deceiver Exarch partner | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 20 | 20 |
|  | Zealous Conscripts, opp 40 | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 20 | 20 |
|  | Deceiver Exarch, opp 40 | 100 | 0 | 0 | 0 | optional | pass_with_controller_choice | 20 | 20 |

## Aggregate per interaction

| Interaction | Total reps | Pass (incl paradox) | Fail | Parser gap |
|---|---:|---:|---:|---:|
| Dockside Extortionist + Temur Sabertooth | 400 | 400 (100%) | 0 | 0 |
| Food Chain + Eternal Scourge/Misthollow Griffin | 400 | 400 (100%) | 0 | 0 |
| Sanguine Bond + Exquisite Blood | 400 | 400 (100%) | 0 | 0 |
| Heliod, Sun-Crowned + Walking Ballista | 400 | 400 (100%) | 0 | 0 |
| Kiki-Jiki + Zealous Conscripts/Deceiver Exarch | 400 | 400 (100%) | 0 | 0 |

## Parser gaps

_None — all combo pieces parseable._

## Notable engine behavior / crashes

_None — no engine crashes, no runaway loops._

## Files

- Harness: `scripts/interaction_harness_infinites.py`

- This report: `data/rules/interaction_harness_infinites_report.md`
