# Loki r45 — Adric / Doctor's Companion Fix Validation

**Date:** 2026-05-19
**Branch:** `dev/loki-r45-validation`
**Binary:** `cmd/hexdek-loki`
**Command:** `go run ./cmd/hexdek-loki --games 3000 --seed 42 --report /tmp/loki-r45-report.md`
**Base:** main @ `87d3345` (R45 stub-batch `c8573b2` sitting on `b2fc25e` Merge dev/adric-handler-r44: Adric / Doctor's Companion CardIdentity leak fix)
**Purpose:** Validate the Adric / Doctor's Companion `*Card`-share fix advertised at "loki −49%" in the merge commit, and confirm the R45 per_card stub-batch port (Neriv Crackling Vanguard, Phylath World Sculptor, The Master Multiplied, Urabrask Heretic Praetor, Vishgraz the Doomhive) didn't regress alongside it.

## Headline

| Phase            | Volume       | Crashes | Invariant Hits | Clean         |
|------------------|--------------|---------|----------------|---------------|
| Chaos games      | 3000 games   | **0**   | 252 (14 games) | 2986 (99.53%) |
| Nightmare boards | 10000 boards | **0**   | 2              | 9999 (99.99%) |

Throughput: 13 games/s · 2073 boards/s. Wall time ≈ 4m02s (chaos 3m57s + nightmare 4.8s).

**Zero panics, zero recovers.**

## Comparison vs r43-validation baseline

Same seed (42), same corpus, same 3000-game / 10000-board profile.

| Metric                  | r43-validation | r44      | **r45**   | Δ r43 → r45        |
|-------------------------|---------------:|---------:|----------:|--------------------|
| Crashes (chaos)         | 0              | 0        | **0**     | flat               |
| Total violations        | 402            | 402      | **252**   | **−150 (−37%)**    |
| Dirty games (chaos)     | 16             | 16       | **14**    | **−2**             |
| Clean-game rate         | 99.47%         | 99.47%   | **99.53%**| +0.06pp            |
| Nightmare crashes       | 0              | 0        | **0**     | flat               |
| Nightmare violations    | 2              | 2        | **2**     | flat               |

## Per-invariant comparison

| Invariant             | r43-validation | r44   | **r45** | Δ r43 → r45     | Notes                                                            |
|-----------------------|---------------:|------:|--------:|-----------------|------------------------------------------------------------------|
| CardIdentity          | 260            | 260   | **110** | **−150 (−58%)** | Adric fix landed: r44 had 30/30 details from Adric; r45 has zero Adric mentions. |
| ZoneConservation      | 124            | 124   | **124** | flat            | Post-Krark floor unchanged.                                      |
| AttachmentConsistency | 8              | 8     | **6**   | **−2 (−25%)**   | Side benefit; one fewer Inventor's-Axe-on-stale-token case.      |
| TriggerCompleteness   | 4              | 4     | **4**   | flat            |                                                                  |
| CombatLegality        | 2              | 2     | **2**   | flat            |                                                                  |
| ZoneCastGrantExpiry   | (4)            | 4     | **6**   | +2 fuzz         | Within ±3 running mean (r42=2, r43-postfix=6, r44=4).            |
| **Total**             | **402**        | 402   | **252** | **−150 (−37%)** |                                                                  |

The merge-commit headline "loki −49%" almost certainly measures CardIdentity specifically (−58% here) or a pre-r43-postfix baseline; the −37% on this 3000-game / seed-42 sample is the same effect measured against the r43-validation post-Krark floor.

## Adric is gone from the offender list

r43-validation / r44 had Adric, Mathematical Genius as the single dominant cluster (30 of 30 violation details, one game, hand ↔ battlefield ↔ graveyard pointer churn). r45 has it nowhere:

```
$ grep -c "Adric" /tmp/loki-r45-report.md
0
```

The fix targeted Adric's Doctor's Companion activated-ability handler — the same handler family Etali (r42b) and Paradigm (r43) had previously closed.

## New top offenders

The next-most-common `*Card`-share clusters surface now that Adric noise is removed:

```
$ grep -oE 'card "[^"]+"' /tmp/loki-r45-report.md | sort | uniq -c | sort -rn | head -3
  20 card "Archon's Glory"
   8 card "God-Eternal Oketra"
```

- **Archon's Glory** (20 hits, single game) — `appears in both seat 3 graveyard and seat 3 graveyard`. Intra-zone pointer duplication, a new shape vs. the cross-zone Adric/Etali/Paradigm pattern. Likely a re-graveyard'd instant/sorcery is being pushed to the same zone twice without dedup.
- **God-Eternal Oketra** (8 hits) — `appears in both seat 2 library and seat 2 graveyard`. The classic cross-zone leak — same family as Adric / Paradigm. God-Eternal cards have a "shuffle into library on death" replacement; the leak is almost certainly that path leaving a stale graveyard reference behind.

Both are clean next-best Loki follow-up leads.

## R45 stub-batch involvement

Cross-check: none of the five R45 ports — Neriv Crackling Vanguard, Phylath World Sculptor, The Master Multiplied, Urabrask Heretic Praetor, Vishgraz the Doomhive — appear in any violation source/attached/commander/log field.

```
$ grep -ciE "Neriv|World Sculptor|Master Multiplied|Urabrask|Vishgraz" /tmp/loki-r45-report.md
0
```

Notable: Phylath World Sculptor was a commander attached to the AttachmentConsistency cluster in the r42 baseline (Inventor's Axe / stale token). The R45 Phylath port surface (landfall plant-token mints) didn't surface any new attachment failures, and the existing one dropped by 2 — likely coincidence given r45 didn't touch attachment plumbing, but worth noting.

The `sba.go` touch in `c8573b2` (state-based actions adjustment for poison-counter buff decay) likewise produced no new TriggerCompleteness / CombatLegality movement.

## Nightmare boards

2 violations / 10000 (99.99% clean) — identical to every prior round. Nightmare-board harness doesn't reach the `*Card`-share surface the Adric fix closed.

## Reproduction

```bash
# Worktree at .claude/worktrees/dev+loki-r45-validation, main @ 87d3345
go run ./cmd/hexdek-loki \
  --games 3000 \
  --seed 42 \
  --report /tmp/loki-r45-report.md
```

Raw report at `/tmp/loki-r45-report.md` on the validation host (~2.5k lines, 30 violation details + summary).

## Verdict

✅ **Adric / Doctor's Companion fix validates clean.** Zero crashes, total violations down 37% vs r43-validation (260 → 110 on CardIdentity, −58%). R45 stub batch ships clean on top — none of the five new handlers implicated.

The Adric fix follows the same `*Card` deep-copy discipline pattern that Etali (r42b) and Paradigm (r43) brought to their handler families. The remaining noise concentrates on the next two clusters in that lineage:

1. **Archon's Glory** intra-zone graveyard pointer duplication (single game, 20 hits)
2. **God-Eternal Oketra** library ↔ graveyard cross-zone leak (likely the shuffle-on-death replacement effect)

Both are clean targets for the next Loki-class follow-up.
