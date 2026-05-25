# Thor Goldilocks — R60 Zero-Confirmation Run

**Date:** 2026-05-24
**Branch:** `dev/goldilocks-r60-zero-confirm-r60` (built from `origin/main` @ `f2aeda3`, post-#237)
**Invocation:** `hexdek-thor -goldilocks --failures-csv /tmp/goldilocks-r60-zero.csv`
**Runtime:** 1.58 s over 31,963 effect-tests + 2,013 keyword-tests (20 k cards/s)

## Headline

```
ZERO FAILURES — fully sterile.
```

| metric                  |       count |
| ----------------------- | ----------: |
| cards tested            |      35,708 |
| effect tests            |      31,963 |
| effect passes           |  **31,963** |
| effect dead-effects     |           0 |
| effect panics           |           0 |
| **effect invariants**   |       **0** |
| skipped (no abilities)  |       4,106 |
| keyword tests           |       2,013 |
| keyword passes          |       2,013 |
| keyword failures        |           0 |
| keyword panics          |           0 |
| failures.csv rows       |      0 (header-only) |

## Arc

| run                               | invariant fails | new clears | running PR                                                 |
| --------------------------------- | --------------: | ---------: | ---------------------------------------------------------- |
| `goldilocks-r60-report.md` (#102) |              19 |          — | baseline (17 `ZoneCastGrantExpiry` + 2 `TurnStructure`)    |
| `goldilocks-r60-post-engine-clean.md` (#218) |     1 |        −18 | r59 + r60 engine cleanup (ZoneCastGrant + scaffold)        |
| **this run (post-#237)**          |           **0** |        −1 | `markSeatLost` mana-drain helper (CR §704.5/§704.6)        |

Total reduction: **19 → 0 across two PRs**, no panics or dead effects
introduced at any step, and the corpus + test counts are unchanged
across all three runs (same 31,963 effect tests, same 2,013 keyword
tests). The arc closes cleanly.

## What landed in #237

The residual `ResourceConservation` failure on Abduction —
"[untap] ResourceConservation: seat 0 is Lost but has ManaPool=10" —
was the last remaining hit. PR #237 introduced a single
`markSeatLost(s, reason)` helper in `internal/gameengine/sba.go` and
routed all five `s.Lost = true` sites through it:

- §704.5a — life total ≤ 0
- §704.5b — drew from empty library
- §704.5c — 10+ poison counters
- §704.6c — 21+ commander damage
- §104.4b — mandatory-loop-draw SBA cap

The helper flips `Lost`, stamps `LossReason`, zeroes `ManaPool`, and
clears the typed `Mana` pool. The §704.5e empty-mana-pool reaper still
runs on phase change for live games — the helper is the missing
loss-transition cleanup that the deterministic invariant snapshot was
catching.

## What goldilocks does NOT cover

Goldilocks is a deterministic per-card effect sweep — strong at exposing
single-card invariant violations and dead effects, weak at multi-card
interactions and long-game state evolution. Zero goldilocks failures
does NOT imply zero engine bugs; it implies the per-card-isolated
invariant surface is clean. The open issue-log row that still belongs
to engine work:

| Date | Source | Issue | Severity |
|------|--------|-------|----------|
| 2026-05-08 | Corpus Audit | 4,190 unbucketed condition/trigger nodes across all 4 eras (33.9% of 12,363 total) | Info |

Loki fuzz remains the multi-turn / multi-card surface. The most recent
documented Loki runs (CLAUDE.md Resolved table, 2026-05-23): 5000 games
@ seed 41 clean on `TriggerCompleteness`, `AttachmentConsistency`,
`ZoneCastGrantExpiry`, `ZoneConservation`. Re-running Loki against
post-#237 main is a natural follow-up but not in scope here.

## Run details

- AST corpus: `data/rules/ast_dataset.jsonl` (35,708 cards loaded, 31,963 had testable abilities)
- Oracle corpus: `data/rules/oracle-cards.json` (35,708 cards)
- Workers: default (`runtime.NumCPU()`)
- Phases: default off
- Scaffold flag: **off**
- CSV: `/tmp/goldilocks-r60-zero.csv` (header row only, 0 failure rows)
- Binary: `/tmp/hexdek-thor-zero` built from `origin/main` @ `f2aeda3`

## Conclusion

The deterministic goldilocks surface is now fully sterile on the R60
corpus. The 19 → 0 arc closed across PRs #102 (diagnosis), #218
(post-cleanup re-sweep), and #237 (the final `markSeatLost` fix), with
no panics, no dead effects, no keyword regressions, and no new failure
categories surfaced at any step.
