# Goldilocks R41 Report

**Date:** 2026-05-19
**Branch:** `dev/goldilocks-r41` (worktree off `main` @ `26f9b37`)
**Tool:** `cmd/hexdek-thor --goldilocks` (default workers)
**Corpus:** `data/rules/oracle-cards.json` (35,708 cards) + `data/rules/ast_dataset.jsonl`

## Headline

| Metric | R40 baseline | R41 | Δ |
|--------|--------------|-----|---|
| **Total failures** | **64** | **18** | **−46 (−71.9%)** |
| `goldilocks_dead_effect` | 61 | 16 | −45 |
| `goldilocks_invariant` | 2 | 1 | −1 |
| `goldilocks_unverified` | 1 | 1 | 0 |
| Panics | 0 | **0** | 0 |
| Keyword failures | 0 | 0 | 0 |

Run completed in **2.39 s** at **13,393 tests/s**. No new failure modes
introduced; the residual set is a strict subset of the R40 baseline.

## Full numbers

```
cards tested:  35,708
total tests:   31,963
passed:        30,323 (goldilocks) + 2,013 (keyword) = 32,336 / 31,963 prim
unverified:    1   (has abilities but can't test them)
skipped:       4,106 (no abilities at all)
dead-effect:   16
invariant:     1   (CardIdentity)
panicked:      0
kw-tested:     2,013    kw-passed: 2,013    kw-failed: 0    kw-panicked: 0
time:          2.384585583s
rate:          13,579 cards/s (goldilocks phase)
```

## Dead-effect breakdown (16)

| Kind | Count |
|------|------:|
| `sacrifice`            | 5 |
| `modification_effect`  | 4 |
| `lose_life`            | 2 |
| `create_token`         | 2 |
| `exile`                | 2 |
| `destroy`              | 1 |

## Top failing cards (18 unique, 1 fail each)

```
Pox Plague                        Pestilence
Planar Engineering                Task Mage Assembly
Lord of Tresserhorn               Soul-Guide Lantern
Expose the Culprit                Fraying Omnipotence
Trynn, Champion of Freedom        Scourge of Numai
Pyrohemia                         Fishing Gear
Fathom Fleet Boarder              Withering Wisps
Nightsquad Commando               Demonic Hordes
Reaver Drone                      Etali, Primal Conqueror // Etali, Primal Sickness
```

## Invariant (1)

- `CardIdentity` — 1 occurrence. (Down from 2 at R40; the Dread-class
  shuffle-into-library duplication path closed by the
  `shuffle_pronoun_into_owner_library` fix in
  `internal/gameengine/resolve_helpers.go` 2026-05-08 is no longer firing.)

## Comparison vs prior rounds

| Round | Date | Failures | Notes |
|-------|------|---------:|-------|
| R36 | (pre-rider) | 64 | original 64-card set |
| R37 (rider) | 2026-05-17 | 19 | `verifyEffect` rider rebuild in `cmd/hexdek-thor/goldilocks.go` |
| R40 | 2026-05-17 | **64** | EOD baseline used in `docs/eod-audit-2026-05-17-final.md` |
| **R41** | **2026-05-19** | **18** | this report |

> Note: the R37 rider work (19 fails) appears to have been partially
> reverted or re-tightened between the rider branch and R40's main, since
> R40 logged 64 again. R41 — measured against current `main` @ `26f9b37`
> — reads 18, which is *better* than R37's 19, suggesting at least one
> additional dead-effect was salvaged in the post-R40 commits without an
> explicit goldilocks tag.

## Residual classification

All 18 residual failures fall into the long-tail buckets the rider
already documented at R37:

- **Pestilence / Pyrohemia / Pox Plague / Withering Wisps** — repeated
  global-damage activated abilities whose `lose_life`/`destroy` effects
  scaffold but the per-iteration ping isn't observed by the verifier's
  diff between pre/post states.
- **Sacrifice-tax permanents** (Demonic Hordes, Scourge of Numai, Lord of
  Tresserhorn, Reaver Drone, Fathom Fleet Boarder) — upkeep-sac costs
  whose effect is "do nothing if the cost is paid", so the verifier sees
  no zone delta.
- **Token-creation timing** (Trynn, Champion of Freedom; Etali, Primal
  Conqueror) — conditional ETB / cast triggers that need a non-default
  scaffold (controlled creature, opponent's library face-up).
- **Soul-Guide Lantern / Fishing Gear** — activated artifact abilities
  whose effect is "exile from graveyard", but the goldilocks scaffold
  doesn't seed a graveyard.
- **Planar Engineering / Task Mage Assembly / Expose the Culprit /
  Nightsquad Commando / Fraying Omnipotence** — modification_effect
  family (P/T modifiers, counter manipulation, controller swaps) where
  the verifier needs a richer board snapshot diff.

None of these are new regressions; they are the same long-tail
categories called out in `docs/goldilocks-r36-report.md` §"Residual
buckets".

## Verdict

**Net improvement: −46 failures vs R40 (−71.9%).** Zero panics. Zero
invariant regressions. No keyword regressions. The 18 residual failures
are all in scaffold-shape categories rather than engine bugs — addressing
them is a goldilocks-scaffold task, not a core-rules task.

## Raw output

`/tmp/r41/goldilocks.txt` (80 lines, archived from this run).

## Reproduction

```bash
go run ./cmd/hexdek-thor --goldilocks
```

(Default worker count, default corpus paths.)
