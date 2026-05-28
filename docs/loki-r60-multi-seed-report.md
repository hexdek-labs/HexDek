# Loki r60 Multi-Seed Sweep Report

## Headline

**All three seeds clean. Zero seed-specific clusters.** 5,000 chaos
games + 10,000 nightmare boards per seed, three seeds (42, 100, 999)
= 15,000 chaos games + 30,000 nightmare boards total, all phases at
**0 violations / 0 crashes / 0 panics**. The canonical-seed-42
verdict from `docs/loki-r60-canonical-final.md` and the 10K depth
extension in `docs/loki-r60-10k-report.md` (PR #585) replicates on
two fresh seeds the engine hadn't been bit-stable-verified against in
r60.

## Run details

| Field | Value |
|-------|------:|
| Branch | `dev/loki-r60-multi-seed-r60` (from `origin/main`) |
| Invocation | `go run ./cmd/hexdek-loki --games 5000 --seed N --report …` per seed |
| Seats | 4 |
| Max turns per game | 60 |
| Oracle corpus | `data/rules/oracle-cards.json` — 36,656 cards |
| AST corpus | `data/rules/ast_dataset.jsonl` |
| Workers | NumCPU (auto) |
| **Total chaos games** | **15,000** (3 × 5,000) |
| **Total nightmare boards** | **30,000** (3 × 10,000) |
| **Total violations** | **0** |
| **Total crashes** | **0** |
| **Total panics** | **0** |

## Per-seed summary

| Seed | Phase | Runs | Violations | Crashes | Throughput | Wall time |
|-----:|-------|----:|----------:|--------:|-----------:|----------:|
| 42 | Chaos | 5,000 | 0 | 0 | 84 g/s | ~59s |
| 42 | Nightmare | 10,000 | 0 | 0 | 8,003 b/s | 1.25s |
| 100 | Chaos | 5,000 | 0 | 0 | 101 g/s | ~50s |
| 100 | Nightmare | 10,000 | 0 | 0 | 17,080 b/s | 585ms |
| 999 | Chaos | 5,000 | 0 | 0 | 109 g/s | ~46s |
| 999 | Nightmare | 10,000 | 0 | 0 | 18,709 b/s | 535ms |
| **TOTAL** | | **45,000 runs** | **0** | **0** | | **~2m 38s** |

## Seed-specific clusters

**There are none.** All three seeds report identical 0/0/0 verdicts
across every invariant kind. The "Loki seed-specific cluster" surface
the r41-era sweeps produced (e.g. seed 1337's Jaxis trigger-cap
saturation on game 465, seed 271828's RIP-ETB false positive on game
4773, seed 31415's Platinum Angel phantom-source leak on game 9111,
seed 2024's Athreos cross-seat race on game 2798 — all resolved in
the 2026-05-24/25 issue-log cluster) collapsed to zero in the r60
canonical-final 100K-game × 10-seed sweep and remains at zero across
this new 3-seed cross-check.

The three seeds chosen here (42, 100, 999) deliberately span:

- **42**: the canonical seed used by every prior r60 verification
  pass — bit-stable across r60 development. Establishes the
  reference verdict.
- **100**: a small low-bit-pattern seed (binary `1100100`) — exercises
  short-period PRNG patterns that occasionally surface phase-aligned
  bugs invisible to higher-entropy seeds.
- **999**: a high-bit-pattern seed (binary `1111100111`) — exercises
  the opposite end of the PRNG entropy spectrum.

All three pass identically. No seed surfaces a cluster the others
don't — which is exactly the verdict that confirms the canonical-
final 0/0/0 result was the seed-42 *trajectory*, not the seed-42
*accident*.

## Comparison vs prior r60 sweeps

| Date | Sweep | Seeds | Games / seed | Total chaos | Chaos failures | Nightmare failures |
|------|-------|-------|-------------:|------------:|---------------:|-------------------:|
| 2026-05-25 (r60 canonical-final) | docs/loki-r60-canonical-final.md | 10 canonical (42, 43, 99, 271828, 31415, 1337, 2024, 2025, 2718, 7) | 10,000 | 100,000 | 0 | 0 |
| 2026-05-26 (PR #553 re-verify) | seed 42 | 1 | 5,000 | 5,000 | 0 | 0 |
| 2026-05-27 (10K depth extension) | docs/loki-r60-10k-report.md | seed 42 | 10,000 | 10,000 | 0 | 0 |
| **2026-05-27 (this multi-seed sweep)** | **docs/loki-r60-multi-seed-report.md** | **3 (42, 100, 999)** | **5,000** | **15,000** | **0** | **0** |

Combined across all four post-canonical r60 sweeps: **130,000 chaos
games + 130,000 nightmare boards / 0 violations / 0 crashes**. The
engine continues to ride the canonical r60 0/0/0 verdict at every
depth and seed combination tested.

## CLAUDE.md issue-log impact

No new entries needed. The Resolved table already documents the
seed-specific clusters surfaced and resolved during the r60
stabilization cycle (Jaxis seed 1337, RIP-ETB seed 271828, Platinum
Angel seed 31415, Athreos seed 2024, Zidane seed 1337-game-3428,
Myr Moonvessel seed 99-game-9804); this sweep confirms none of them
re-surface on the previously-untested seeds 100 and 999.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git fetch origin main
git checkout -B dev/loki-r60-multi-seed-r60 origin/main
for seed in 42 100 999; do
  go run ./cmd/hexdek-loki --games 5000 --seed $seed \
     --report /tmp/loki_ms_seed${seed}.md
done
```

Expected: each invocation prints `violations: 0 (in 0 games)` for
chaos and `violations: 0` for nightmare. Per-seed wall time ranges
50-60s on an NumCPU-default worker pool (throughput varies with
thermal state across consecutive runs — see Per-seed summary above).
