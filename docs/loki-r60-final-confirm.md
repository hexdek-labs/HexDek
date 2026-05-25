# Loki R60 Final Confirmation

Date: 2026-05-24
Branch: `dev/loki-r60-final-confirm-r60`
Post: PR #190 (CardIdentity ability-stack-item false-positive fix)

Goal: confirm 0 violations across every seed previously tested in
the r60 stress / mega-stress / wide-seed sweeps, now that both
known invariant false positives (Gisa opp-only TriggerCompleteness
via PR #184; Necrogen Communion ability-stack-item CardIdentity via
PR #190) are closed.

## TL;DR

**10 of 10 seeds returned 0 violations and 0 crashes across both
chaos and nightmare phases.**

| Seed | Chaos (2K games) | Nightmare (10K boards) | Verdict |
|:----:|:----------------:|:----------------------:|:-------:|
| 42 | **0** | **0** | ✅ clean |
| 43 | **0** | **0** | ✅ clean |
| 99 | **0** | **0** | ✅ clean |
| 7 | **0** | **0** | ✅ clean |
| 1337 | **0** | **0** | ✅ clean |
| 2024 | **0** | **0** | ✅ clean |
| 2025 | **0** | **0** | ✅ clean |
| 31415 (π) | **0** | **0** | ✅ clean |
| 271828 (e) | **0** | **0** | ✅ clean |
| 161803 (φ) | **0** | **0** | ✅ clean |

Aggregate across the full wide-seed sweep:

- **20,000 chaos games + 100,000 nightmare boards** in total
- **0 invariant violations** across all 10 seeds × both phases
- **0 crashes / 0 panics / 0 recovers**

## Per-seed delta vs original surface

| Seed | Original | After r60 final waves | Δ |
|:----:|:--------:|:---------------------:|:-:|
| 42 | 0/0 | 0/0 | flat |
| 43 | 1/0 (District Mascot) | 0/0 | **−1** chaos |
| 99 | 0/0 | 0/0 | flat |
| 7 | 0/0 | 0/0 | flat |
| 1337 | 12/0 (SBA-cap zombie game) | 0/0 | **−12** chaos |
| 2024 | 0/0 | 0/0 | flat |
| 2025 | 0/0 | 0/0 | flat |
| 31415 | 2 chaos (Gisa FP) / 2 nightmare (Necrogen Communion FP) | 0/0 | **−2/−2** |
| 271828 | 0/2 nightmare (Necrogen Communion FP) | 0/0 | **−2** nightmare |
| 161803 | 0/0 | 0/0 | flat |

Every signature surfaced anywhere in the r60 stress / mega-stress /
post-Mascot final sweeps is now closed.

## Headline trajectory (full r60 era)

| Era | Seed | Violations |
|:---:|:----:|:----------:|
| r41 baseline | 41 | 1,652 |
| r44 | 41 | 402 |
| r60 round 1 | 41 | 52 |
| r60 round 2 | 41 | 10 |
| r60 round 3 | 41 / 42 | 0 / 6 |
| r60 final pre-Mascot | 42 / 43 | 0 / 1 |
| r60 final post-Mascot | 42 / 43 | 0 / 0 |
| r60 stress (pre-#178) | 99 / 7 / 1337 | 0 / 0 / 12 |
| r60 post-#178 | 42 / 43 / 99 / 7 / 1337 | 0 / 0 / 0 / 0 / 0 |
| r60 mega-stress (pre-#184/#190) | 2024 / 2025 / 31415 / 271828 / 161803 | 0/0 / 0/0 / 2+2 / 0+2 / 0/0 |
| r60 mega-stress post-#184 | (same) | 0/0 / 0/0 / 0+2 / 0+2 / 0/0 |
| **r60 final confirm (this run, post-#190)** | **all 10 seeds** | **0** across the board |

Per-game stochastic violation rate:

- r41 baseline: 33.0% per game (1,652 / 5,000)
- r60 final confirm: **0.0%** per game (0 / 20,000 chaos + 0 / 100,000 nightmare)
- Cumulative reduction: **−100%**

## Conclusion

The engine is **officially clean at the wide-seed level**: 10
seeds × 2,000 chaos games + 10,000 nightmare boards each, all
zero, both phases. Every chaos signature and every nightmare
signature surfaced across the entire r60 stress sweep is closed.

The r60 era is closed. Next residual will need a fresh seed sweep
or a deeper-coverage instrumentation pass (longer games, more
seats, exotic deck configurations) to surface.

## How to reproduce (full 10-seed wide-seed gauntlet)

```bash
for seed in 42 43 99 7 1337 2024 2025 31415 271828 161803; do
  go run ./cmd/hexdek-loki --games 2000 --seed "$seed"
done
```

Each invocation overwrites `data/rules/CHAOS_REPORT.md`. Total
runtime ≈ 5 minutes on the reference machine.
