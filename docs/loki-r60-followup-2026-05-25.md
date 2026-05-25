# Loki R60 Follow-up — 2026-05-25 Verification

> **TL;DR — Clean.**
> 3-seed × 5,000-game Loki sweep (42, 99, 1337) returns **0 invariant violations, 0 crashes, 0 panics** across 15,000 chaos games + 30,000 nightmare boards. r60 canonical-final result holds.

| | |
|---|---|
| **Verification branch** | `dev/loki-r60-followup-fuzz` |
| **Run date** | 2026-05-25 |
| **Engine release** | r60 (post-canonical, latest main) |
| **Companion** | [`docs/loki-r60-canonical-final.md`](loki-r60-canonical-final.md) — the 100K-game / 10-seed canonical baseline |

---

## Why we ran this

`docs/loki-r60-canonical-final.md` declared the engine **OFFICIALLY CLEAN at canonical-seed scale** (100,000 chaos games × 10 seeds, 0 violations). This follow-up re-runs the 3 most-recurring-residual seeds from the r60 stress cycle history (42 = base canonical, 99 = surfaced the Myr Moonvessel pending-trigger residual at 10K, 1337 = surfaced the Zidane EOT control-return residual at 10K) at the standard 5K depth to confirm the canonical result holds after the day's PR landings (#440 cEDH seat-bias docs, #439 API index, plus the trailing 12-fix audit).

## Methodology

Standard Loki harness at `cmd/hexdek-loki` with the default flags: 4 seats, 60-turn cap, 10K nightmare boards, every invariant active. No filter, no seed-cards forcing, no commander pin. Default progress-every and worker pool sizing.

```bash
go run ./cmd/hexdek-loki --games 5000 --seed 42
go run ./cmd/hexdek-loki --games 5000 --seed 99
go run ./cmd/hexdek-loki --games 5000 --seed 1337
```

Each invocation overwrites `data/rules/CHAOS_REPORT.md`; the per-seed results below are captured from the report after each run completed.

## Results

| Seed | Chaos games | Chaos violations | Chaos duration | Chaos throughput | Nightmare boards | Nightmare violations | Nightmare throughput | Verdict |
|:----:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| 42 | 5,000 | **0** | 1m2.863s | 80 games/sec | 10,000 | **0** | 13,453 boards/sec | ✅ CLEAN |
| 99 | 5,000 | **0** | 1m4.511s | 78 games/sec | 10,000 | **0** | 11,527 boards/sec | ✅ CLEAN |
| 1337 | 5,000 | **0** | 1m9.624s | 72 games/sec | 10,000 | **0** | 13,396 boards/sec | ✅ CLEAN |
| **TOTAL** | **15,000** | **0** | 3m17.0s | ~77 g/s avg | **30,000** | **0** | ~12,800 b/s avg | ✅ CLEAN |

- 0 crashes, 0 concessions, 0 panics across all 3 runs
- 0 invariant violations across chaos (15,000 games) and nightmare (30,000 boards)
- All 3 seeds report `Verdict: CLEAN` in the harness output

## Comparison to baseline

| Run | Seeds | Total games | Total violations | Per-game rate |
|:---|:---|:---:|:---:|:---:|
| r41 baseline | 41 (×1) | 5,000 | 1,652 | 33.0% |
| r60 canonical-final (#427) | 42/43/99/7/1337/2024/2025/π/e/φ (×10) | 100,000 | 0 | 0.00% |
| **r60 followup (this run)** | **42, 99, 1337 (×3)** | **15,000** | **0** | **0.00%** |

The followup reproduces the canonical-final per-game rate exactly. No new clusters surfaced; the historical residuals at seed 99 (Myr Moonvessel — closed by PR #402) and seed 1337 (Zidane EOT — closed by PR #399) remain closed.

## Verdict

**Clean.** The engine remains officially clean at the canonical-seed scale established by PR #427. Today's PR landings (#440 cEDH seat-bias gauntlet, #439 API index docs, et al.) did not regress engine behavior. No new regressions to report; no follow-up chase needed.

## Reproducibility

```bash
for seed in 42 99 1337; do
  go run ./cmd/hexdek-loki --games 5000 --seed "$seed"
done
```

Total wall time ≈ 3m20s on the reference machine. Each run writes a fresh `data/rules/CHAOS_REPORT.md`.

## See also

- [`docs/loki-r60-canonical-final.md`](loki-r60-canonical-final.md) — the 100K-game / 10-seed canonical baseline (PR #427)
- [`docs/release-notes-r60.md`](release-notes-r60.md) — the r60 cycle release context with the 12-fix audit trail
- `CLAUDE.md` Issue Log → Resolved table — the per-fix audit trail for the 12 r60 lifecycle / invariant / per_card closures
