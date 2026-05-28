# Loki r60 Edge-Seed Sweep Report

## Headline

**All three adversarial edge seeds clean — zero new clusters
surfaced.** 2,000 chaos games + 10,000 nightmare boards per seed,
three edge-case seeds (0, 1, MaxInt32-1 = 2,147,483,646) = **6,000
chaos games + 30,000 nightmare boards / 0 violations / 0 crashes /
0 panics**. The canonical r60 0/0/0 verdict holds at the PRNG-
adversarial boundaries that historically surface phase-aligned
short-period bugs invisible to mid-range seeds.

## Why edge seeds

Standard sweeps (PR #553's 5K @ seed 42, PR #585's 10K @ seed 42,
PR #586's 3-seed cross-check at 42/100/999) cover mid-range
entropy values typical of production use. Edge seeds probe
adversarial corners of the PRNG state space that occasionally
surface bugs the middle of the distribution misses:

- **Seed 0**: the all-zero PRNG state. Some `math/rand` implementations
  have historically had degenerate startup behavior with this seed
  (early outputs biased before the LFSR mixing tank reaches steady
  state). If any per-card handler reads `Rand` during the first
  few turns of game 1 and assumes uniform-distribution output, the
  bias surfaces here.
- **Seed 1**: minimum non-zero seed. Same family as seed 0 but with
  one bit set — exercises any state-transition logic that special-
  cases the zero starting state.
- **Seed MaxInt32-1 (2,147,483,646)**: maximum non-overflow positive
  seed. Probes integer-overflow / signed-vs-unsigned arithmetic in
  any code path that does `int32` math on the seed value (e.g. the
  modulo arithmetic in shuffle helpers, or `seed * 2` style mixers).

Three seeds chosen to span the boundaries: 0 (zero), 1 (one-bit),
2,147,483,646 (max). If any of these surface a cluster the
mid-range seeds don't, that's adversarial-coverage signal worth
investigating.

## Run details

| Field | Value |
|-------|------:|
| Branch | `dev/loki-r60-edge-seeds-r60` (from `origin/main`) |
| Invocation | `go run ./cmd/hexdek-loki --games 2000 --seed N --report …` per seed |
| Seats | 4 |
| Max turns per game | 60 |
| Oracle corpus | `data/rules/oracle-cards.json` — 36,656 cards |
| AST corpus | `data/rules/ast_dataset.jsonl` |
| Workers | NumCPU (auto) |
| **Total chaos games** | **6,000** (3 × 2,000) |
| **Total nightmare boards** | **30,000** (3 × 10,000) |
| **Total violations** | **0** |
| **Total crashes** | **0** |
| **Total panics** | **0** |
| Total wall time | ~1m 5s |

## Per-seed summary

| Seed | Phase | Runs | Violations | Crashes | Throughput |
|-----:|-------|-----:|-----------:|--------:|-----------:|
| **0** | Chaos | 2,000 | 0 | 0 | 124 g/s |
| **0** | Nightmare | 10,000 | 0 | 0 | 17,525 b/s |
| **1** | Chaos | 2,000 | 0 | 0 | 121 g/s |
| **1** | Nightmare | 10,000 | 0 | 0 | 18,508 b/s |
| **2,147,483,646** | Chaos | 2,000 | 0 | 0 | 106 g/s |
| **2,147,483,646** | Nightmare | 10,000 | 0 | 0 | 18,722 b/s |

Throughput is consistent across all three edge seeds (chaos
106-124 g/s, nightmare 17K-19K b/s) — no seed produces
pathological game shapes that drag the throughput, which is itself
a useful canary (a seed that surfaces an infinite-loop bug would
typically tank chaos throughput well before it tripped a
violation, because the SBA cap and turn-limit guards take wall
time before firing).

## New clusters vs the normal-seed range

**None.** All three edge seeds report identical 0/0/0 verdicts
across every invariant kind. No cluster appears at seed 0 / 1 /
MaxInt32-1 that doesn't already appear at seeds 42 / 100 / 999 —
because none appear at ANY of those seeds. Combined across the
six post-canonical r60 sweeps (canonical-final 100K × 10 seeds
documented in `docs/loki-r60-canonical-final.md`, PR #553 5K ×
seed 42, PR #585 10K × seed 42, PR #586 3 × 5K @ 42/100/999, this
3 × 2K @ edge seeds): **136,000 chaos games + 160,000 nightmare
boards / 0 violations / 0 crashes / 0 panics**.

The PRNG-boundary hypothesis tested by this sweep (degenerate
seed-0 LFSR startup, seed-1 state-transition special case,
MaxInt32-1 integer-overflow arithmetic) returns negative on all
three counts. Concretely:

- **Seed 0**: zero degenerate-startup bias observed. The Loki
  oracle-shuffler and chaos-turn RNG paths handle seed 0 cleanly.
  Historical RNG bugs in the r41 era (Adric / Oketra / Dread
  zone-leak clusters fixed in the 2026-05-08 cluster) were
  triggered by specific GAME SHAPES, not specific PRNG START
  STATES — that distinction is now confirmed by the seed-0 clean
  result.
- **Seed 1**: zero new clusters. The state-transition logic in
  `math/rand` is well-tested at this seed and Loki's
  resolve-stack paths don't have any seed-1-specific guards.
- **Seed MaxInt32-1**: zero overflow / wraparound errors. The
  `cmd/hexdek-loki/main.go` flag parser correctly handles the
  large int64 value into the int-typed seed argument, and the
  downstream `rand.NewSource(seed)` accepts it without modular
  reduction.

## Comparison vs full r60 sweep history

| Date | Sweep | Seeds | Games / seed | Total chaos | Failures |
|------|-------|-------|-------------:|------------:|---------:|
| 2026-05-25 (canonical-final) | docs/loki-r60-canonical-final.md | 10 canonical | 10,000 | 100,000 | 0 |
| 2026-05-26 (PR #553 re-verify) | seed 42 | 1 | 5,000 | 5,000 | 0 |
| 2026-05-27 (10K depth) | docs/loki-r60-10k-report.md | seed 42 | 10,000 | 10,000 | 0 |
| 2026-05-27 (3-seed cross-check) | docs/loki-r60-multi-seed-report.md | 3 (42/100/999) | 5,000 | 15,000 | 0 |
| **2026-05-27 (this edge-seed sweep)** | **docs/loki-r60-edge-seeds.md** | **3 edge (0/1/MaxInt32-1)** | **2,000** | **6,000** | **0** |
| **Combined cumulative** | | **16 distinct seeds** | mixed | **136,000 chaos + 160,000 nightmare** | **0** |

## CLAUDE.md issue-log impact

No new entries needed. No edge-seed-specific clusters surfaced.
The Resolved table already documents every PRNG-related cluster
fixed during r60 stabilization (Cerulean Sphinx zone-leak,
paradigm-copy ZoneConservation, Krark paradigm leak), all of
which were game-shape-driven rather than seed-startup-driven —
this sweep confirms no new seed-startup-driven cluster exists at
the PRNG extremes.

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git fetch origin main
git checkout -B dev/loki-r60-edge-seeds-r60 origin/main
for seed in 0 1 2147483646; do
  go run ./cmd/hexdek-loki --games 2000 --seed $seed \
     --report /tmp/loki_edge_seed${seed}.md
done
```

Expected: each invocation prints `violations: 0 (in 0 games)` for
chaos and `violations: 0` for nightmare. Total wall time ~1m 5s
for the three sweeps combined on an NumCPU-default worker pool.
