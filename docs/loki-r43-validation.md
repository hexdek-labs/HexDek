# Loki r43 — Krark / Paradigm Fix Validation

**Date:** 2026-05-19
**Branch:** `dev/loki-r43-validation`
**Binary:** `cmd/hexdek-loki`
**Command:** `go run ./cmd/hexdek-loki --games 3000 --seed 42 --report /tmp/loki-r43-report.md`
**Base:** main @ `fbd2cf8` (Merge dev/krark-zone-conservation-r43: paradigm copy leak fix)
**Purpose:** Validate that the paradigm/Krark `*Card`-leak-into-ParadigmExile fix (`d51d84d`) holds under the chaos gauntlet and that no regression slipped in alongside it.

## Headline

| Phase            | Volume       | Crashes | Invariant Hits | Clean         |
|------------------|--------------|---------|----------------|---------------|
| Chaos games      | 3000 games   | **0**   | 402 (16 games) | 2984 (99.47%) |
| Nightmare boards | 10000 boards | **0**   | 2              | 9999 (99.99%) |

Throughput: 16 games/s · 2978 boards/s. Wall time ≈ 3m8s (chaos 3m5.0s + nightmare 3.4s); well under the 600s cap.

**Zero panics, zero recovers.** Holds the line set in r41/r42/r43-postfix.

## Comparison vs prior rounds

| Phase                   | r41 (5000g) | r42 (3000g) | r43-postfix (5000g) | **r43-validation (3000g)** |
|-------------------------|------------:|------------:|--------------------:|---------------------------:|
| Crashes (chaos)         | 0           | 0           | 0                   | **0**                      |
| Invariants (chaos)      | 1652        | 1148        | 958                 | **402**                    |
| Invariants per 1000g    | 330.4       | 382.7       | 191.6               | **134.0**                  |
| Dirty-game rate         | ~1.14%      | 1.07%       | 0.84%               | **0.53%**                  |
| Nightmare crashes       | 0           | 0           | 0                   | **0**                      |
| Nightmare invariants    | 6           | 2           | 2                   | **2**                      |

Total invariants per 1000g dropped **−30% vs r43-postfix** (`e600539` → `fbd2cf8`, which is exactly the paradigm fix in `d51d84d`). Dirty-game rate is now under 1 in 200.

## Invariant breakdown — chaos games

| Invariant             | r43-validation (3000g) | r43-postfix (5000g) | r42 (3000g) | Per-1000g r43-validation |
|-----------------------|----------------------:|--------------------:|------------:|-------------------------:|
| CardIdentity          | 260                    | 562                  | 940         | **86.7**                 |
| ZoneConservation      | 124                    | 364                  | 190         | **41.3**                 |
| AttachmentConsistency | 8                      | 18                   | 12          | **2.7**                  |
| TriggerCompleteness   | 4                      | 6                    | 4           | **1.3**                  |
| CombatLegality        | 2                      | 2                    | 0           | **0.7**                  |
| ZoneCastGrantExpiry   | 0                      | 6                    | 2           | **0.0**                  |
| **Total**             | **402**                | 958                  | 1148        | **134.0**                |

**ZoneConservation per-1000g**: r43-postfix 72.8 → r43-validation 41.3 → **−43%**, slightly better than the merge commit's advertised "zone-cons −28%" headline. The paradigm-copy fix appears to have closed multiple downstream leaks at once.

**CardIdentity per-1000g**: r43-postfix 112.4 → r43-validation 86.7 → **−23%**, despite the paradigm fix not targeting CardIdentity directly. Likely a side benefit: paradigm copies that previously kept a stale `*Card` reference live across zones were also tripping CardIdentity once the copy resolved.

**ZoneCastGrantExpiry**: dropped to zero across this 3000-game sample. Was already low pre-fix (1.2/1000g), now within fuzz-noise threshold.

## Top offenders

Same shape as r43-postfix — the Adric, Mathematical Genius `*Card`-pointer-shared-across-zones cluster remains the largest residual lead:

```
$ grep -oE 'card "[^"]+"' /tmp/loki-r43-report.md | sort | uniq -c | sort -rn | head -3
  30 card "Adric, Mathematical Genius"
```

The detail-printed violations all attach to game 170 (seed 1700043). Adric is a Doctor Who set card; the failure mode is a tutor-from-opponent's-library / activate-from-graveyard path that re-fires the same `*Card` pointer across hand ↔ battlefield ↔ graveyard. Closing this needs the same `*Card` deep-copy discipline the Etali r42b fix (`c9b43d3`) and the paradigm r43 fix (`d51d84d`) brought to their respective handler families — likely Adric's "commit a crime" activated-ability handler is the carrier.

CombatLegality (2 hits / 3000g, also 2 in r43-postfix / 5000g) is the next-most-stable signal worth a confirmation sample at a higher game count.

## R43 stub-batch involvement

Cross-check: none of the R42b / R43 newly-ported handlers appear in the violation set.

```
$ grep -iE "Echocasting|Symposium|Paradigm" /tmp/loki-r43-report.md | grep -v paradigm_r43
(no matches)
```

The paradigm test fixture (`paradigm_r43_zone_test.go`) covered the original game-181 reproduction; the gauntlet sample confirms the fix generalises.

## Nightmare boards

2 violations across 10000 boards (99.99% clean) — identical count to r42 and r43-postfix. No regression; no improvement either (paradigm leak doesn't reach the nightmare-board synthetic harness).

## Reproduction

```bash
# Worktree at .claude/worktrees/dev+loki-r43-validation, main @ fbd2cf8
go run ./cmd/hexdek-loki \
  --games 3000 \
  --seed 42 \
  --report /tmp/loki-r43-report.md
```

Raw report (2454 lines, 30 violation details + summary) lives at `/tmp/loki-r43-report.md` on the validation host. Aggregate numbers above are what the repo retains.

## Verdict

✅ **Krark / paradigm fix validates clean.** Zero crashes, zero panics, ZoneConservation −43% per-1000g vs the pre-fix `r43-postfix` baseline, CardIdentity −23% per-1000g as a side benefit. No new regressions.

The remaining noise concentrates on the Adric `*Card`-share cluster — same handler anti-pattern Etali (r42b) and Paradigm (r43) closed. Next-best Loki follow-up: an Adric-targeted audit of activated-ability / tutor-from-opponent's-library `*Card` lifecycle.
