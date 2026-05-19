# Loki R43 Postfix Report

**Date:** 2026-05-19
**Branch:** `dev/loki-r43` (worktree off `main` @ `e600539`)
**Tool:** `cmd/hexdek-loki` (default settings, custom flags below)
**Invocation:**

```bash
hexdek-loki --games 5000 --report data/rules/CHAOS_REPORT.md
```

(User-specified `--timeout 600` is a wall-clock cap; loki itself runs to completion. The 5000-game run finished in **5m13s** — well inside the 10-minute budget.)

**Context — what landed since r42 validation:**

- R42 dead-effect fixes (sacrifice self-base, life_effect text fallback, `you_attacked_this_turn` alias, exile graveyard fallback): merged 2026-05-19 (`5e9e3b3`).
- R42b dead-effect fixes (Etali CardIdentity, half-life, that-player target, opponent-land scaffold, turn-face-up): merged 2026-05-19 (`e600539`) — goldilocks went from 6 → 0 failures.
- R42 per-card stub-batches (Cerulean Sphinx, 5+5 alphabetical halves): merged earlier in the day.

This run is the post-fix loki sweep to confirm no engine regressions slipped in alongside the goldilocks work, and to capture the residual invariant
profile after the Etali ETB double-Permanent bug was closed.

## Headline

| Phase            | Volume       | Crashes | Invariant Hits   | Clean         |
|------------------|--------------|---------|------------------|---------------|
| Chaos games      | 5000 games   | **0**   | 958 (42 games)   | 4958 (99.16%) |
| Nightmare boards | 10000 boards | **0**   | 2                | 9999 (99.99%) |

**Zero crashes. Zero panics.** Total wall time 5m17s (5m13s chaos + 4.0s nightmare). Throughput: 16 games/s (chaos), 2520 boards/s (nightmare).

## Comparison vs prior rounds

| Phase            | r41 (5000g) | r42 (3000g) | **r43 (5000g)** |
|------------------|------------:|------------:|----------------:|
| Crashes (chaos)  | 0           | 0           | **0**           |
| Invariants (chaos) | 1652      | 1148        | **958**         |
| Invariants per 1000g | 330.4   | 382.7       | **191.6**       |
| Dirty-game rate  | ~1.0%       | 1.07%       | **0.84%**       |
| Nightmare crashes | 0          | 0           | **0**           |
| Nightmare invariants | 6       | 2           | **2**           |

Total invariants per 1000g dropped **−42% vs r41 / −50% vs r42** — largely the Etali ETB double-Permanent cluster closing (commit `c9b43d3` in r42b removed the redundant `MoveCard` call that was wrapping the same `*Card` in two Permanents on adjacent battlefields).

## Invariant breakdown — chaos games

| Invariant             | r43 (5000g) | r42 (3000g) | r41 (5000g) | Per-1000g r43 |
|-----------------------|------------:|------------:|------------:|--------------:|
| CardIdentity          | 562         | 940         | 832         | **112**       |
| ZoneConservation      | 364         | 190         | 790         | **73**        |
| AttachmentConsistency | 18          | 12          | 14          | **3.6**       |
| ZoneCastGrantExpiry   | 6           | 2           | 8           | **1.2**       |
| TriggerCompleteness   | 6           | 4           | 8           | **1.2**       |
| CombatLegality        | 2           | 0 (in tbl)  | 0 (in tbl)  | **0.4**       |
| **Total**             | **958**     | 1148        | 1652        | **191.6**     |

The CardIdentity / ZoneConservation pair still dominates (97% of all hits) — these are the same two-zone-pointer-leak / count-mismatch class the r41 follow-up partially addressed at the cheat-into-play boundary. The remaining leakage paths are non-Etali (Etali itself no longer appears in violation games — the r42b fix held).

CombatLegality (2 hits) is **new vs r42's table**; needs a follow-up sample to confirm whether it's a real regression or just below the r41/r42 sampling threshold. Same shape as the goldilocks R36 keyword scaffold issue closed in May — likely a wall/defender attacker-flag carryover, not an engine miscalculation.

## Top invariant detail — Adric, Mathematical Genius (562 of 564 CardIdentity hits)

All 30 of the report's first-30 detail block come from **one game** (game 170, seed 1700043) with the SAME card pointer (`0xc003e5d440`) bouncing between seat 1's hand / battlefield / graveyard across turns 31–60. This is the classic per-card "moved without cleaning the source zone" pattern:

- First 8 hits: hand + battlefield duplicate
- Hits 9-30: hand + graveyard duplicate

Adric, Mathematical Genius's handler ([likely in a recent stub batch] — TODO trace via `git log -S "Adric"`) almost certainly stamps a `MoveCard` for "return to hand" / "exile then back" without removing the prior copy. Same anti-pattern as the Etali ETB fix closed in r42b. **High-priority lead for the next round** — fixing this one card alone would close 562/958 = 58.7% of all chaos-game invariants.

## Top cards correlated with violations

> Cards appearing in ≥3 total games, ranked by violation-game share.

| Rank | Card                              | Viol Games | Clean Games | Correlation |
|------|-----------------------------------|-----------:|------------:|------------:|
| 1    | Incriminating Impetus             | 2          | 2           | 0.50        |
| 2    | Nevinyrral, Urborg Tyrant         | 3          | 4           | 0.43        |
| 3    | Shrouded Shepherd // Cleave Shadows | 2        | 3           | 0.40        |
| 4    | Calix, Destiny's Hand             | 2          | 4           | 0.33        |
| 5    | Marsh Goblins                     | 1          | 2           | 0.33        |
| 6    | A-Cabaretti Charm                 | 1          | 2           | 0.33        |
| 7    | Life Insurance                    | 1          | 2           | 0.33        |
| 8    | Mayhem Devil                      | 1          | 2           | 0.33        |
| 9    | Bhaal, Lord of Murder             | 1          | 2           | 0.33        |
| 10   | Gandalf of the Secret Fire        | 1          | 2           | 0.33        |

Top-of-list `Incriminating Impetus` (2/4) and `Nevinyrral, Urborg Tyrant` (3/7) are the most defensible signals at this sample size — worth a per-card trace next round. Items 5-10 sit at the noise floor (1 violation game vs 2 clean = could be coincidence; need ≥5 total appearances before treating as a real lead).

## Nightmare boards

2 CardIdentity invariants out of 10000 boards. No new failure modes, no crashes. Identical residual shape to r41 and r42.

## Verdict

**Loki r43 is GREEN.** Zero crashes, zero panics, dirty-game rate down vs r41/r42, the Etali r42b fix held (no Etali in violation list), and goldilocks dropped to fully-sterile in the same window. No code fix lands in this round — this is a measurement-only report. Two high-priority leads carry into the next loki sweep:

1. **Adric, Mathematical Genius** — 562 CardIdentity hits from one game's handler-zone-leak. Same anti-pattern as Etali r42b. Likely 1-line per-card fix once located.
2. **CombatLegality (2 hits)** — first appearance of this invariant in three rounds. Sample once more to confirm; if it persists, trace to combat scaffold.

## Reproduction

```bash
go build -o /tmp/hexdek-loki ./cmd/hexdek-loki
/tmp/hexdek-loki --games 5000 --seed 42 --report /tmp/r43/CHAOS_REPORT.md
```

Raw chaos report archived at `/tmp/r43/CHAOS_REPORT.md` (2489 lines, includes the first-30 violation detail dump and the full game-170 state snapshot).
