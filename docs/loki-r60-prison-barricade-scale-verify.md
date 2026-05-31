# Loki r60 — PR #800 (Prison Barricade) scale verification + KP follow-up

**Date:** 2026-05-30
**Branch:** `dev/loki-r60-verify-prison-barricade-r60` (cut from `origin/main` @ `4c33987b`)
**Command:** `go run ./cmd/hexdek-loki --games 5000 --seed 42`
**Worktree:** `.claude/worktrees/r60-11-feyd-slot`

## Headline

**5k seed-42 confirms PR #800 closure scales — and uncovers a separate Knowledge Pool residual that the 500-game window missed.**

PR #800 (2026-05-30, "fireTrigger dispatches against leaving perm for LTB events") was originally validated against 500 games / seed 42 and reported `ExileLinkageIntegrity 72 → 2 (-97%)`. The 500-game window only reproduces the first 1/10 of the seed-42 stream, so games > 500 (in particular game 1044 Myr Prototype and game 2029 Leonardo, Sewer Samurai) were not exercised. The 5k run reveals that:

1. **PR #800 closes Prison Barricade (game 2164, 42 hits) end-to-end** — exactly as designed.
2. **30 ExileLinkageIntegrity hits remain**, all tracing to **Knowledge Pool** as the source — a different exile shape (CastGrant tag, not LTBReturn) that PR #800's `fireTrigger` ctx-fallback alone doesn't close.

A follow-up fix (PR #817 — `dev/knowledge-pool-ltb-clear-r60`) closes the KP residual end-to-end. With both PRs in main, **ExileLinkageIntegrity is 0 across 5000 games / seed 42**.

## Run trajectory (chaos phase, seed 42)

| Run | Date | Total | ExileLinkageIntegrity | ZoneConservation | CardIdentity | Notes |
|-----|------|------:|----------------------:|-----------------:|-------------:|-------|
| Baseline (`docs/loki-r60-report.md`) | 2026-05-30 | 268 | **72** (4 games) | 192 | 4 | Pre-fix. |
| Post-PR #800 (this run) | 2026-05-30 | 224 | **30** (3 games) | 190 | 4 | Prison Barricade family closed; KP residual visible. |
| Post-PR #817 KP fix | 2026-05-30 | 154 | **0** (0 games) | 150 | 4 | ELI fully clean. |

Nightmare phase (10k boards): bit-stable clean across all 3 runs.

## Worst-cluster residuals after PR #800

| Card (exiled target) | Hits | Game | Inferred source |
|----------------------|-----:|-----:|-----------------|
| Leonardo, Sewer Samurai // Leonardo, Sewer Samurai | 26 | 2029 | Knowledge Pool (event log: `[5718] zone_change seat=0 source=Leonardo`, preceded by `[5717] enter_battlefield seat=2 source=Knowledge Pool`) |
| Myr Prototype | 2 | 1044 | Knowledge Pool (events show `zone_cast_grant_expired source=Knowledge Pool` immediately before `seat_eliminated seat=1`) |
| Great Hall of the Biblioplex | 2 | 149 | Knowledge Pool (land exiled into seat 2's exile via KP's "each player exiles top 3 of their library" ETB) |

All three pods feature Knowledge Pool in seat 1 or seat 2; the source-timestamps in the violation messages (58, 61, 68) match KP's `Permanent.Timestamp` at ETB time. KP stamps `Card.ExiledByTimestamp = perm.Timestamp` as an internal discovery tag for its cast-from-hand trigger (`per_card_batch_ai_r60.go:363, 398`), but the engine reuses that field as the canonical LTBReturn marker — so when KP dies the tag becomes a stale "linked to a dead source" pointer that the `ExileLinkageIntegrity` invariant correctly flags.

## Game 2029 forensic — Leonardo, Sewer Samurai

Event-log window captured via temporary `RecentEvents(gs, 800)` bump in `cmd/hexdek-loki/main.go`:

```
[5716] stack_resolve seat=2 source=Knowledge Pool target=seat0
[5717] enter_battlefield seat=2 source=Knowledge Pool target=seat0
[5718] zone_change seat=0 source=Leonardo, Sewer Samurai
[5719] trigger_evaluated seat=-1 source=card_exiled
[5720] trigger_evaluated seat=-1 source=zone_change
...
[6207-6224] long discard chain — game running through cleanup at turn 48+
```

Leonardo was in seat 0's library when KP ETB'd; KP's "each player exiles top 3" picked Leonardo into seat 0's exile and stamped `ExiledByTimestamp = KP.Timestamp = 68`. KP later left the battlefield, but no LTB cleanup of the tag ran.

## Game 1044 forensic — Myr Prototype

```
[5153] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[5154] zone_cast_grant_expired seat=2 source=Knowledge Pool target=seat0
[5155] seat_eliminated seat=1 source= amount=11
[5156] trigger_evaluated seat=-1 source=seat_eliminated
```

KP's controller (seat 1) was eliminated via `HandleSeatElimination`. That path removes permanents *directly* via `removePermanent` without firing `permanent_ltb` dispatch (the same gap PR #106 patched for `ExpireSourceGrants`). PR #800's `fireTrigger` ctx-fallback assumes the LTB event is FIRED — but in the seat-elim path, no trigger event is fired at all. So even with PR #800 + PR #817's per-card KP handler, this path still leaks. PR #817 therefore adds a **second** primitive — `ClearLinkedExileTagsForSource` — wired into `HandleSeatElimination`'s leaving-perm loop next to `ExpireSourceGrants`.

## Why the 500-game window missed it

Loki's seed scheme is `gameSeed = baseSeed*10000 + gameIdx`. The 500-game window covered games 0–499 only. The 3 KP-residual games (149 Great Hall, 1044 Myr Prototype, 2029 Leonardo) all fall outside that window except game 149 — which DID surface in the 500-game post-PR-#800 run (the "2 Great Hall hits" line in PR #800's verification table). The 500-game test was directionally correct about closure scale but mis-categorized the Great Hall hits as a separate non-Banisher shape; the deeper 5k run revealed they share root cause with Leonardo + Myr Prototype.

**Takeaway for future Loki verifications:** for any seed where the worst-cluster span concentrates beyond game N, run at least 5× N. The canonical-seed-42 sweep convention (see `docs/loki-r60-canonical-final.md`) is 5k games for closure verification; the 500-game iteration is fine for fast smoke-test but should not be relied on as evidence of full closure.

## Post-PR-#817 status

| | Pre-PR-#800 | Post-PR-#800 | Post-PR-#817 |
|---|---:|---:|---:|
| ExileLinkageIntegrity | 72 | 30 | **0** |
| Games hit | 4 | 3 | 0 |
| Distinct sources | 1 (Banisher Priest family) | 1 (Knowledge Pool) | — |
| Nightmare violations | 0 | 0 | 0 |

The remaining 154 violations in the post-#817 5k run are `ZoneConservation` (150 — the Phase E InstanceID gap-walk residuals, separate workstream) and `CardIdentity` (4 — Spikeshell Harrier game 4635, also called out in the original baseline report).

## Reproducers

```bash
git fetch origin main && git checkout -B repro origin/main

# Full 5k verification:
go run ./cmd/hexdek-loki --games 5000 --seed 42

# Single-game reproducers (require both PR #800 and PR #817 to all-pass):
go run ./cmd/hexdek-loki --games 150  --seed 42 --invariant exile-linkage-integrity  # game 149 Great Hall (Knowledge Pool ETB exile)
go run ./cmd/hexdek-loki --games 1045 --seed 42 --invariant exile-linkage-integrity  # game 1044 Myr Prototype (KP via seat-elim)
go run ./cmd/hexdek-loki --games 2030 --seed 42 --invariant exile-linkage-integrity  # game 2029 Leonardo (KP via DestroyPermanent)
go run ./cmd/hexdek-loki --games 2165 --seed 42 --invariant exile-linkage-integrity  # game 2164 Prison Barricade (already closed by PR #800)
```
