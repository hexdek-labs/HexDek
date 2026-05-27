# Loki r60 Fuzz Report — 5000 games / seed 42

**Date:** 2026-05-26
**Branch:** `dev/loki-r60-fuzz` (cut from `origin/main`)
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42`
**Worktree:** `.claude/worktrees/r60-11-feyd-slot`
**Raw log:** `/tmp/loki-r60-raw.log`
**Auto-generated report:** `data/rules/CHAOS_REPORT.md`

## Headline

**0 crashes / 0 invariant violations across 5000 chaos games + 10000 nightmare boards.**

The engine is bit-stable clean on seed 42 at r60. This supersedes both the round-1 r60 report previously at this path (52 violations, 2026-05-24) and the round-2 report (10 violations, also 2026-05-24). It is the first fully-clean 5000-game run on the canonical seed since the Loki invariant suite was introduced.

## Run Output

```
=== CHAOS GAMES COMPLETE ===
  games:           5000
  duration:        49.002s
  throughput:      102 games/sec
  crashes:         0 (in 0 games)
  violations:      0 (in 0 games)
  clean games:     5000

=== NIGHTMARE BOARDS COMPLETE ===
  boards:          10000
  duration:        615ms
  throughput:      16258 boards/sec
  crashes:         0
  violations:      0
  clean boards:    10000

Verdict: CLEAN
```

## Top Invariant Clusters (by frequency)

No invariant clusters observed. Every tracked category — `CardIdentity`, `ZoneConservation`, `ZoneCastGrantExpiry`, `TriggerCompleteness`, `SBACompleteness`, `LifeConsistency`, `AttachmentConsistency`, `CombatLegality`, `ResourceConservation`, `ReplacementCompleteness`, `StackIntegrity` — reported 0 violations across 0 games.

## Trajectory vs. CLAUDE.md Baselines (seed 42, 5000 games)

| Milestone | Total Violations | Δ vs Previous | Δ vs r41 Baseline |
|-----------|-----------------:|--------------:|------------------:|
| **r41** (baseline)            | 1652 | — | — |
| **r44** (post Cerulean Sphinx + Krark paradigm + Adric / Oketra) | 402  | −1250 / −76% | −76% |
| **r60 round 1**               | 52   | −350  / −87% | −96.9% |
| **r60 round 2** (PR #106 ZoneCastGrantExpiry + #110 batchH + #124 AttachmentConsistency + trigger-cap) | 10   | −42   / −81% | −99.4% |
| **r60 round 3 — THIS RUN**    | **0**  | **−10 / −100%** | **−100%** |

### Per-cluster comparison

| Invariant              | r41 | r44 | r60 r1 | r60 r2 | **r60 r3 (this run)** |
|------------------------|----:|----:|-------:|-------:|----------------------:|
| CardIdentity           | 832 | 260 |    2   |    —   | **0** |
| ZoneConservation       | 790 | 124 |    0   |    —   | **0** |
| ZoneCastGrantExpiry    |   8 |   4 |   42   |    4   | **0** |
| TriggerCompleteness    |   8 |   4 |    6   |    2   | **0** |
| AttachmentConsistency  |  14 |   8 |    0   |    0   | **0** |
| CombatLegality         |   — |   2 |    2   |    2   | **0** |
| SBACompleteness        |   — |   — |    —   |    1   | **0** |
| LifeConsistency        |   — |   — |    —   |    1   | **0** |
| **Total**              |1652 | 402 |   52   |   10   | **0** |

### What closed the residual 10 (round 2 → round 3)

Cross-referenced from the CLAUDE.md Resolved Issue Log:

- **SBA cap-draw seat-loss** (seed 1337 g465, 2026-05-24) — CR §704.3 mandatory-loop draw now marks every non-Lost seat Lost with reason "mandatory loop draw (CR 104.4b via SBA cap)" so `CheckEnd` actually terminates and SBAs aren't permanently muted. Closed the seat-0-alive-at-life-0 LifeConsistency / SBACompleteness pair.
- **Athreos cross-seat reanimate race** (seed 2024 g2798, 2026-05-24) — `athreosShroudDies` now scans owner's graveyard before delegating to `enterBattlefieldWithETB`, mirroring the Gisa §704.6d race-loser pattern. Closed 24× CardIdentity.
- **Charix `ended`-skip mirror in SBA completeness check** (seed 2025 g3180, 2026-05-24) — invariant false-positive: three stacked +8/-8 mods resolved post-game-end, §704.5f never re-ran. Toughness check now skips at `ended=1`; life check still fires (preserves cap-draw counterfactual).
- **Necrogen Communion ability log-label** (seeds 31415 + 271828 nightmare, 2026-05-24) — `checkCardIdentity` now skips stack items with `item.Source != nil` OR `item.Kind ∈ {triggered, activated}`. Closed 4× nightmare CardIdentity false-positive.
- **Gisa opp-only trigger filter** (seed 31415 g237, 2026-05-24) — `opponentOnlyCreatureDiesTriggers` map gates 6 bearers (Gisa, Reaper King No More, Toxrill, Yahenni, Grave Pact, Grave Betrayal). Closed 2× TriggerCompleteness false-positive.
- **District Mascot Static `etb_with_counters`** (seed 43 g1003, 2026-05-24) — new `ApplyStaticETBCounters` wired into `resolvePermanentSpellETB` + `FirePermanentETBTriggers`. Closed the lone 0/0-creature SBACompleteness residual.
- **Zidane Tantalus Thief EOT control-return** (seed 1337 g8921, 2026-05-25) — EOT closure now battlefield-scans before re-adding; per CR §611.3c the duration's expiry no-ops if the perm isn't on the battlefield. Closed the 40× CardIdentity deep-leak signature, the largest single residual of the 10K-depth sweep.
- **Seat-elimination ExpireSourceGrants** (seeds 42 + 31415 + 271828, 2026-05-24) — `HandleSeatElimination` was the lone LTB-equivalent path missing `ExpireSourceGrants(gs, p.Timestamp)`. Adding it closed 8× ZoneCastGrantExpiry across 3 seeds, including the seed-42 round-2 residual (Kess + Compound Fracture).

## Worst NEW Regression

**None.** This run is a strict superset-clean of every prior r60 round. No invariant category, no signature, no commander correlation regressed — every category that previously fired is now at 0.

The CLAUDE.md open Issue Log contains 3 entries on other seeds (ReplacementCompleteness ×1 on seed 271828 g4773, ResourceConservation ×2 on seed 99 g9804, SBACompleteness ×6 on seed 31415) — none of those seeds were exercised by this run, so the open log is unchanged.

## Caveats

- This run exercises **seed 42 only**. The CLAUDE.md open issues are seed-specific (271828, 99, 31415) and remain unverified against the latest engine state by this run.
- Run is **AST + oracle backed** via symlinks into the main checkout's `data/rules/` (the worktree's `data/rules/` had the directory but the two large gitignored corpora were absent before this run — symlinks point at `/Users/joshuawiedeman/Documents/GitHub/HexDek/data/rules/{ast_dataset.jsonl,oracle-cards.json}`).
- Performance: 102 g/s chaos throughput, 16258 b/s nightmare — both in line with recent r60 runs.
- Throughput improved markedly vs round 1 (70 g/s → 102 g/s, +46%) without any explicit perf work — consistent with the elimination of failure-path observation overhead (no violation paths means no detail-capture allocations).

## Reproduction

```
git fetch origin main
git checkout -B dev/loki-r60-fuzz origin/main
# (worktree only) symlink AST + oracle from main checkout if missing:
#   ln -sf <main>/data/rules/ast_dataset.jsonl data/rules/ast_dataset.jsonl
#   ln -sf <main>/data/rules/oracle-cards.json data/rules/oracle-cards.json
go run ./cmd/hexdek-loki --games 5000 --seed 42
```

## Recommended next moves

1. **Wide-seed gauntlet** (`--seed 43,99,1337,2024,2025,31415,271828`) — verify the round-2/round-3 fixes don't leave seed-specific residuals on the seeds where they were originally surfaced.
2. **Depth bump on seed 42** (50000+ games) — Zidane was 1 leak across 10K games of seed 1337; there may be similar long-tail signatures hiding past the 5K horizon on seed 42 too.
3. **Close the 3 open Issue Log entries** (seeds 271828 / 99 / 31415) — these are the next concrete fix targets and are decoupled from seed 42 entirely.
