# Loki R60 — 10K Seed 0 Regression Run

Date: 2026-05-26
Branch: `dev/loki-r60-regressions-r60`
Context: r60's canonical-final verdict (`docs/loki-r60-canonical-final.md`,
2026-05-25) ran 10K × 10 canonical seeds = 100K games + 100K nightmare
boards CLEAN, with all 12 stress-cycle residuals resolved. The
canonical seed set was `{42, 43, 99, 7, 1337, 2024, 2025, 31415, 271828, 161803}`.
Seed 0 was **not** part of that gauntlet. This run extends bit-stability
evidence to a new, previously-untested seed at full canonical depth.

## TL;DR

**20K runs (10K chaos + 10K nightmare) at seed 0 = 0 violations,
0 crashes, 0 panics.** No new clusters, no regressions, no residual.
Engine remains OFFICIALLY CLEAN; r60 era stays closed.

| Phase | Runs | Violations | Crashes | Verdict |
|:------|---:|:---:|:---:|:---:|
| Chaos games | 10,000 | **0** | **0** | ✅ clean |
| Nightmare boards | 10,000 | **0** | **0** | ✅ clean |

Auto-generated config + summary from `hexdek-loki --games 10000 --seed 0`
follows below.

---

Generated: 2026-05-26T21:02:52-07:00

## Configuration

| Parameter | Value |
|-----------|-------|
| Oracle Corpus | 36656 cards |
| Legendary Creatures | 3433 |
| Total Games | 10000 |
| Seed | 0 |
| Permutations | 1 |
| Seats | 4 |
| Max Turns | 60 |
| Nightmare Boards | 10000 |

## Summary

### Chaos Games

| Metric | Count |
|--------|-------|
| Duration | 4m9.769s |
| Throughput | 40 games/sec |
| Crashes | 0 (in 0 games) |
| Invariant Violations | 0 (in 0 games) |
| Clean Games | 10000 |

### Nightmare Boards

| Metric | Count |
|--------|-------|
| Duration | 1.463s |
| Throughput | 6836 boards/sec |
| Crashes | 0 |
| Invariant Violations | 0 |
| Clean Boards | 10000 |

## Verdict: CLEAN

All 10000 chaos games and 10000 nightmare boards passed all invariant checks with zero crashes.
