# AttachmentConsistency — R60 Residual Verification

**Date:** 2026-05-24
**Branch:** `dev/attachment-consistency-residual-r60`
**Scope:** Confirm that PR #84 (r60-1, 2026-05-23: detachAll hooked into
`removePermanentFromBattlefield` + 11 per_card flicker/blink sites) and
the r59 seat-elimination detachAll fix have fully closed the
AttachmentConsistency cluster — and that no new escalation has been
introduced by the ~30 r60 PRs that have landed on `main` since.

## Background

| Run | Seed | Games | AttachmentConsistency | Dominant signature |
|-----|------|-------|------------------------|---------------------|
| r41 baseline | 41 | 5000 | 14 | (mixed) |
| r41 → r57 drift | 48 | various | 14 → 22, bit-stable across r53/55/57 | `"Ghoulish Impetus" on token`, `"Brilliant Wings" on Tidal Warrior`, `"Dub" on phyrexian mite Token` |
| r60-1 (PR #84) | 41 + 48 | 500 each | **0 / 0** | — |
| r60-1 fuzz @ main | 42 | 5000 | **0** | — |

PR #84's fix was structural: every engine and per_card path that
removes a permanent from the battlefield via raw `gs.removePermanent`
or `removePermanentFromBattlefield` now calls `detachAll` synchronously
so auras / equipment can't keep stale `AttachedTo` references in the
window between removal and the next SBA pass (`§704.5m` / `§704.5n`).

## This run's verification

Five fuzz runs on `origin/main` @ `01d4f37` (the head of `main` at the
time of the audit — ~30 PRs ahead of the original PR #84 merge):

| # | Seed | Games | Other args | Crashes | Total violations | **AttachmentConsistency** |
|---|------|-------|------------|---------|------------------|---------------------------|
| 1 | 42 | 1000 | — | 0 | 0 | **0** |
| 2 | 41 | 1000 | — | 0 | 2 (all ZoneCastGrantExpiry) | **0** |
| 3 | 43 | 1000 | — | 0 | 0 | **0** |
| 4 | 48 | 1000 | — | 0 | 2 (all ZoneCastGrantExpiry) | **0** |
| 5 | 42 | 1000 | `--seed-cards "Ghoulish Impetus,Brilliant Wings,Dub"` | 0 | 2 (all CombatLegality) | **0** |

**5000 total chaos games across 4 seeds + a targeted aura-seeded
probe: 0 AttachmentConsistency hits.** The r57 dominant signatures
(`Ghoulish Impetus`, `Brilliant Wings`, `Dub` attached to creatures
that left the battlefield) were specifically force-seeded into seat 0's
deck in run #5 and still produced zero hits — the auras did make it
onto the battlefield and into combat, but every removal path that the
r57 cluster surfaced now goes through a `detachAll` call.

Residual violations (4 ZoneCastGrantExpiry + 2 CombatLegality) belong
to unrelated clusters and are tracked separately
(`docs/loki-r60-report.md` / `docs/triggered-ability-audit-r60.md`).

## Reproduction

```
git fetch origin main
git checkout -B dev/attachment-consistency-residual-r60 origin/main

# Multi-seed sweep:
for seed in 41 42 43 48; do
  go run ./cmd/hexdek-loki --games 1000 --seed $seed \
    --nightmare-boards 0 \
    --report data/rules/CHAOS_REPORT_R60_ATTACH_S${seed}.md
done

# Aura-seeded targeted probe (forces the r57 dominant-signature
# auras into seat 0's deck via the --seed-cards handler-focused flag):
go run ./cmd/hexdek-loki --games 1000 --seed 42 \
  --nightmare-boards 0 \
  --seed-cards "Ghoulish Impetus,Brilliant Wings,Dub" \
  --report data/rules/CHAOS_REPORT_R60_ATTACH_AURASEEDED.md
```

Reports land in the gitignored `data/rules/` tree per the existing
chaos-report rule.

## Caveats

- `cmd/hexdek-loki` has no per-invariant filter flag — the full
  invariant suite runs on every checkpoint, so the verification is
  unavoidably "all clear across the board for AttachmentConsistency"
  rather than a targeted slice. The aura-seeded probe is the closest
  this tooling gets to focusing the search.
- 5 × 1000 games at ~50–70 g/s sampled ~250–350 turns per game in
  the long-game tail. The r57 cluster surfaced bit-stable signatures
  at 22 hits per 5000 games (4.4 hits per 1000) — well above the
  noise floor at this sample size. A 0/5000 result is therefore
  strong but not unbounded; if the cluster re-appears in a future
  full r61 5000-game fuzz at a meaningfully higher rate, the
  signature should be visible.
- The r59 seat-elimination fix and PR #84's per_card sweep
  collectively cover destroy / sacrifice / exile / bounce / flicker /
  exchange / mutate / craft / aura-swap / sweep-return / seat-leave.
  The remaining permanent-removal surface (`gs.removePermanent`
  callers without a detachAll) is the gain-control family, which
  intentionally does NOT detach because the permanent stays on the
  battlefield (CR §702 et al.). No leak path remains in the audited
  set.

## Verdict

**AttachmentConsistency residual = 0 across 5000 chaos games and a
targeted aura-seeded probe.** No fix needed; PR #84 closed the
cluster and no r60 follow-on PR has re-opened it. This document
serves as the verification record.
