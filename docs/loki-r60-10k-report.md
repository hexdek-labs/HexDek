# Loki r60 10K-Game Fuzz Report

## Headline

**Zero failures across 10,000 chaos games AND 10,000 nightmare boards.**
Three consecutive clean Loki sweeps now (the 5K canonical-final, the
5K post-canonical re-confirmation in PR #553, and this 10K depth
extension). No new clusters at 10K depth vs the 5K baseline — the
canonical-seed engine continues to ride the r60 0/0/0 verdict
documented in `docs/loki-r60-canonical-final.md`.

## Run details

| Field | Value |
|-------|------:|
| Branch | `dev/loki-r60-10k-r60` (from `origin/main`) |
| Invocation | `go run ./cmd/hexdek-loki --games 10000 --seed 42` |
| Seed | 42 (canonical) |
| Seats | 4 |
| Max turns per game | 60 |
| Oracle corpus | `data/rules/oracle-cards.json` — 36,656 cards |
| AST corpus | `data/rules/ast_dataset.jsonl` |
| Workers | NumCPU (auto) |
| **Chaos games** | 10,000 |
| Chaos wall time | 1m 25.4s |
| Chaos throughput | 117 games/sec |
| **Nightmare boards** | 10,000 |
| Nightmare wall time | 517 ms |
| Nightmare throughput | 19,348 boards/sec |
| **Total wall time** | ~1m 26s |

## Summary table

| Category | Chaos (10K games) | Nightmare (10K boards) |
|----------|------------------:|------------------------:|
| Total runs | 10,000 | 10,000 |
| **Clean runs** | **10,000** | **10,000** |
| Violations | 0 | 0 |
| Crashes / recovers | 0 | 0 |
| Panics | 0 | 0 |
| Games / boards with ≥1 violation | 0 | 0 |

## Top clusters

There are no clusters — the run produced zero violations across every
invariant kind, every cluster bucket, and every depth probe. For
context the canonical r60 cluster taxonomy from `docs/loki-r60-canonical-final.md`
and the CLAUDE.md issue log Resolved table lists:

- `ZoneCastGrantExpiry` — `while_source_on_bf` / `until_end_of_turn` grants outliving their source permanent or game-end
- `CardIdentity` — same `*Card` pointer in two zones simultaneously (Adric / Oketra / Dread / Jaxis / Athreos / Zidane / Necrogen Communion clusters)
- `TriggerCompleteness` — trigger batch opened but never drained; opp-only creature_dies false positives
- `ResourceConservation` — Lost seat retained ManaPool / pending triggers
- `ZoneConservation` — real cards disappeared (Krark paradigm leak / Cerulean Sphinx duplication / Breya game-420 evacuate cluster)
- `AttachmentConsistency` — Aura / Equipment `AttachedTo` pointing at a permanent no longer on battlefield
- `CombatLegality` — attacker / blocker eligibility regressions
- `ReplacementCompleteness` — Rest in Peace ETB-after-zone-change false positive (one cluster, now suppressed by the post-RIP-ETB scan in `checkReplacementCompleteness`)
- `SBACompleteness` — Charix `ended=1` false positive (one cluster, suppressed) / Platinum Angel phantom-source leaks (one cluster, fixed by the `permIsOnAnyBattlefield` backstop in `pickReplacement`)
- `LifeConsistency` — seat alive at life ≤ 0 (subsumed by the SBA-cap mandatory-loop draw fix in `loop_shortcut.go`)
- `WinCondition` — post-elimination poison/commander-damage counter adjustments faking a loss-reason mismatch (suppressed by the `!s.LeftGame` guard added 2026-05-25)

All eleven historical buckets report **0** on this run. No new failure
categories were observed, so no additions to the CLAUDE.md issue log
Open table are warranted. The 11 historical clusters all remain
documented in the Resolved table as the audit trail of the r60
stabilization cycle.

## Trajectory vs prior sweeps

| Date | Sweep | Chaos failures | Nightmare failures | Notes |
|------|-------|--------------:|-------------------:|-------|
| 2026-05-08 (r41 baseline pre-followup) | 5K @ seed 42 | 1,652 | — | the original r41-era baseline (Cerulean Sphinx + paradigm + ZoneCastGrantExpiry clusters at full strength) |
| 2026-05-19 (r41 followup) | 5K @ seed 42 | 1,255 (−24%) | — | `docs/loki-r41-followup-report.md` — Cerulean Sphinx zone-leak closed |
| 2026-05-23 (r44/r45 sweep) | 5K @ seed 42 | 402 (−68% vs r41) | — | post-#106 source-LTB grant fix + AttachmentConsistency `DetachAll` |
| 2026-05-24 (r60 ROUND 2) | 5K @ seed 42 | 52 (−87% vs r44) | — | `docs/loki-r60-round2-report.md` — round 1 → 2 was −81%; per-cluster final state ZoneCastGrantExpiry 4 / TriggerCompleteness 2 / CardIdentity 2 / CombatLegality 2 |
| 2026-05-24 (r60 ROUND 2 post-fix) | 5K @ seed 42 | 10 (−81% vs round 1) | — | trigger-cap reset + invariant-filter pair |
| 2026-05-24 (r60 final) | 5K × 2 seeds (42, 43) | 1 (Charix `ended=1` false-positive) → 0 (suppressor) | — | `docs/loki-r60-final-report.md` |
| 2026-05-25 (r60 canonical-final) | 10K × 10 canonical seeds (100K chaos + 100K nightmare total) | **0** | **0** | `docs/loki-r60-canonical-final.md` — engine officially clean at canonical-seed scale; r60 era closed |
| 2026-05-26 (PR #553 re-verify) | 5K @ seed 42 | **0** | **0** | post per_card consumer-wiring waves (expend, tribute, class_level_up, gift, etc.) and post precon-vibes R3–R7 work |
| **2026-05-27 (this run)** | **10K @ seed 42** | **0** | **0** | 117 g/s chaos throughput; 19,348 b/s nightmare; no new categories surfaced at the 10K depth that didn't appear at 5K (because 5K already at 0 — the 2× depth extension just confirms the canonical seed's verdict holds) |

**Cumulative trajectory:** 1,652 (r41 baseline) → 0 (r60 canonical-final) = **−100%**. The 10K extension at canonical seed 42 reproduces the canonical-final verdict bit-for-bit.

## Anything new at 10K?

**No.** Three checks pin this:

1. **Headline counts**: chaos `violations: 0 (in 0 games)`; nightmare `violations: 0`. The 10K depth extension produced the same 0/0 verdict as the 5K runs at the same seed (PR #553's re-verify and the canonical-final 10K-per-seed sweep at seed 42 from `docs/loki-r60-canonical-final.md`).
2. **No new invariant kinds emitted**: the per-game / per-board summary lines stayed at the canonical zero across all 10K chaos games + 10K nightmare boards. The chaos report at `/tmp/loki_r60_10k_chaos.md` contains the standard configuration + summary blocks with empty Violations / Crashes sections.
3. **No panics or recovers**: zero across both phases. The trigger-depth (8) and trigger-total (2000) caps in `internal/gameengine/per_card/registry.go::fireTrigger` did not fire at the 10K depth — confirming that the per-turn-reset PR for `trigger_total` (2026-05-24 issue-log entry) holds at 2× the prior depth.

## CLAUDE.md issue-log impact

No new entries needed. The existing Resolved table fully captures the
11 historical r60 clusters that drove the 1,652 → 0 trajectory; this
report appends a 2026-05-27 row to the trajectory table above as the
3rd consecutive 0/0 sweep (canonical-final → PR #553 re-verify → 10K
depth extension).

## Reproducing

```bash
cd $(git rev-parse --show-toplevel)
git fetch origin main
git checkout -B dev/loki-r60-10k-r60 origin/main
go run ./cmd/hexdek-loki --games 10000 --seed 42 \
   --report /tmp/loki_r60_10k_chaos.md
```

Expected: clean — `violations: 0 (in 0 games)` for both chaos and
nightmare phases, ~1m 30s wall time on an NumCPU-default worker pool.
