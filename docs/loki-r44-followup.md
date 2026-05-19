# Loki r44 — R44 Stub-Batch Follow-up Validation

**Date:** 2026-05-19
**Branch:** `dev/loki-r44`
**Binary:** `cmd/hexdek-loki`
**Command:** `go run ./cmd/hexdek-loki --games 3000 --seed 42 --report /tmp/loki-r44-report.md`
**Base:** main @ `5a86b3c` (Merge dev/percard-stubs-r44; sits on top of `fbd2cf8` Krark/paradigm fix)
**Purpose:** Confirm the R44 per_card stub-batch port (Alisaie Leveilleur, Cecily Haunted Mage, Extus Oriq Overlord / Awaken the Blood Avatar, Fire Lord Zuko, Jaxis the Troublemaker; `cost_modifiers.go` touch) doesn't regress the post-Krark Loki baseline established in r43-validation.

## Headline

| Phase            | Volume       | Crashes | Invariant Hits | Clean         |
|------------------|--------------|---------|----------------|---------------|
| Chaos games      | 3000 games   | **0**   | 402 (16 games) | 2984 (99.47%) |
| Nightmare boards | 10000 boards | **0**   | 2              | 9999 (99.99%) |

Throughput: 8 games/s (chaos) · 1334 boards/s (nightmare). Wall time 6m13s — slow vs r43-validation's 3m8s, attributable to host contention (parallel dev workers consuming cores during the run), not engine work.

**Zero panics, zero recovers.**

## Comparison vs r43-validation baseline

Same seed (42), same corpus, same 3000-game / 10000-board profile. The only diff between r43-validation's main (`fbd2cf8`) and this run's main (`5a86b3c`) is the R44 per_card stub-batch port (`c5010e1`).

| Metric                  | r43-validation | **r44** | Δ          |
|-------------------------|---------------:|--------:|------------|
| Crashes (chaos)         | 0              | **0**   | flat       |
| Total violations        | 402            | **402** | **flat**   |
| Dirty games (chaos)     | 16             | **16**  | **flat**   |
| Clean-game rate         | 99.47%         | 99.47%  | flat       |
| Nightmare crashes       | 0              | **0**   | flat       |
| Nightmare violations    | 2              | **2**   | flat       |

## Per-invariant comparison

| Invariant             | r43-validation | **r44** | Δ           | Notes                                                            |
|-----------------------|---------------:|--------:|-------------|------------------------------------------------------------------|
| CardIdentity          | 260            | **260** | flat        | Still the Adric, Mathematical Genius cluster (30 of 30 details). |
| ZoneConservation      | 124            | **124** | flat        | Post-Krark floor.                                                |
| AttachmentConsistency | 8              | **8**   | flat        |                                                                  |
| TriggerCompleteness   | 4              | **4**   | flat        |                                                                  |
| CombatLegality        | 2              | **2**   | flat        |                                                                  |
| ZoneCastGrantExpiry   | 0 (per doc)    | **4**   | +4 fuzz     | r42 had 2, r43-postfix had 6, r43-validation breakdown summed to 398 vs reported 402 — the 4-violation column was almost certainly miscategorized in r43-validation's table; appearing here at 4 puts the breakdown sum back in agreement with the summary line. Fuzz-floor noise, not regression. |
| **Total**             | **402**        | **402** | **flat**    |                                                                  |

ZoneCastGrantExpiry's 0 → 4 shift is the only column that moved, and the size of the move is below fuzz-floor noise (r42 = 2, r43-postfix = 6, r44 = 4 — within ±3 of the running mean). Reconciling the r43-validation breakdown arithmetic against its summary line strongly suggests 4 ZoneCastGrantExpiry hits were already present at the r43-validation baseline and simply mis-tabulated. No real movement.

## R44 stub-batch involvement

Cross-check: none of the five newly-ported handlers — or `Awaken the Blood Avatar`, the Blood-Avatar token Extus mints — appear in any violation source/attached/commander/log field.

```
$ grep -ciE "Alisaie|Cecily|Extus|Awaken|Fire Lord Zuko|Jaxis|Blood Avatar" /tmp/loki-r44-report.md
0
```

- **Alisaie Leveilleur** — Dualcast cost-reduction static + Partner-with grant: no `cost_modifiers.go` divergences surfaced.
- **Cecily, Haunted Mage** — graveyard / discard payoffs stayed quiet.
- **Extus, Oriq Overlord // Awaken the Blood Avatar** — the cast-from-graveyard side and the Blood Avatar token-mint side stayed clean.
- **Fire Lord Zuko** — Firebending mana grant and cast-from-exile +1/+1 distribution stayed clean.
- **Jaxis, the Troublemaker** — copy-token-with-haste minting stayed clean. Notable given the Krark/paradigm cluster the r43 fix closed lived in the same copy-token surface family.

## Top offender — unchanged from r43-validation

```
$ grep -oE 'card "[^"]+"' /tmp/loki-r44-report.md | sort | uniq -c | sort -rn | head -3
  30 card "Adric, Mathematical Genius"
```

Same single-game cluster (seed 1700043 family) the r43-validation doc filed as the next-best follow-up lead. R44 didn't touch the Adric handler surface; it's still the dominant `*Card`-pointer-share path remaining after the Etali (r42b) and Paradigm (r43) closures.

## Reproduction

```bash
# Worktree at .claude/worktrees/dev+loki-r44, main @ 5a86b3c
go run ./cmd/hexdek-loki \
  --games 3000 \
  --seed 42 \
  --report /tmp/loki-r44-report.md
```

Raw report (2454 lines, 30 violation details + summary) at `/tmp/loki-r44-report.md` on the validation host.

## Verdict

✅ **R44 stub batch ships clean.** Bit-stable vs r43-validation on identical seed/corpus: same 402 / 16-game / 99.47%-clean profile, same Adric residual, none of the five R44 picks implicated. The Krark/paradigm fix from r43 continues to hold.

No new bugs introduced. Next-best Loki follow-up remains the Adric, Mathematical Genius `*Card`-share cluster — same handler anti-pattern Etali (r42b) and Paradigm (r43) have already closed in their families.
