# Loki r42 — R42 Stub-Batch Validation Run

**Date:** 2026-05-19
**Branch:** `dev/loki-r42-validate`
**Binary:** `cmd/hexdek-loki`
**Command:** `go run ./cmd/hexdek-loki --games 3000 --seed 42 --report /tmp/loki-r42-report.md` (wall-clock cap 600s)
**Purpose:** Regression-validate the R42 per_card stub-batch port (`fe520c7`) — Shiko and Narset, Unified · Rosnakht, Heir of Rohgahh · Jon Irenicus, Shattered One · Zhulodok, Void Gorger · The Second Doctor.

## Headline

| Phase            | Volume       | Crashes | Invariant Hits  | Clean       |
|------------------|--------------|---------|-----------------|-------------|
| Chaos games      | 3000 games   | **0**   | 1148 (32 games) | 2968 (98.93%) |
| Nightmare boards | 10000 boards | **0**   | 2               | 9999 (99.99%) |

Throughput: 21 games/s · 3659 boards/s. Wall time ≈ 2m27s (chaos 2m24.3s + nightmare 2.7s); well under the 600s cap.

**Zero panics, zero recovers.** Holds the line set in r41 (0 crashes / 5000 games) and the R40 nil-deref cluster closures (May 5 / May 11). No regression introduced by the R42 stub-batch port.

## Invariant Breakdown — Chaos Games

| Invariant             | r42 (3000g) | r41 (5000g) | Per-1000g rate r42 → r41 |
|-----------------------|-------------|-------------|---------------------------|
| CardIdentity          | 940         | 832         | 313 → 166                 |
| ZoneConservation      | 190         | 790         | 63 → 158                  |
| AttachmentConsistency | 12          | 14          | 4.0 → 2.8                 |
| TriggerCompleteness   | 4           | 8           | 1.3 → 1.6                 |
| ZoneCastGrantExpiry   | 2           | 8           | 0.7 → 1.6                 |
| **Total**             | **1148**    | 1652        |                           |

Dirty-game rate is flat-to-better: **32 / 3000 = 1.07%** here vs. **57 / 5000 = 1.14%** in r41. The bulk of the per-1000 spike in CardIdentity is a single-game cluster (see below) of the same shape r41 documented for Cerulean Sphinx — a different card, identical leak pattern.

## R42 Card Involvement

None of the five newly-ported cards appears as a source, attached entity, or commander in any violation:

```
$ grep -ciE "Shiko and Narset|Rosnakht|Jon Irenicus|Zhulodok|The Second Doctor|Kobolds of Kher" /tmp/loki-r42-report.md
0
```

- **Shiko and Narset, Unified** — no `flurry`-triggered draw or copy-branch ever showed up adjacent to a violation; the second-spell gate held under the fuzz.
- **Rosnakht, Heir of Rohgahh** — no Kobolds-of-Kher-Keep token leaked into ZoneConservation or AttachmentConsistency; the heroic mint stayed clean.
- **Jon Irenicus, Shattered One** — no `cant_be_sacrificed` / `goaded_until_turn` flags surfaced anomalies; end_step donate stayed contained (the partial breadcrumb correctly skips the control-change transfer so no half-state slipped past).
- **Zhulodok, Void Gorger** — the doubled `ApplyCascade` call did not produce ZoneConservation or library-bottom-shuffle anomalies under the gauntlet.
- **The Second Doctor** — the `no_max_hand_size` seat-flag stamp and end-of-turn group-draw stayed quiet.

## Dominant Cluster — Carryover, Not R42

**28 of 30 detail-printed violations** (and ~900 of 940 CardIdentity counts) attach to a single repeating commander/card combo:

- **Game 108** (seed 1080043) — `card "Adric, Mathematical Genius" appears in both seat 1 hand and seat 1 battlefield` then `…and seat 1 graveyard`. Commanders: Ezuri, Claw of Progress · The Master of Keys · Syr Ginger, the Meal Ender · Katara, Water Tribe's Hope.
- **Game 170** (seed 1700043) — second AttachmentConsistency cluster, Inventor's Axe attached to a token that's no longer on any battlefield (Tersa Lightshatter · Yargle, Glutton of Urborg · A-Phylath, World Sculptor · Sauron, Lord of the Rings).

Same shape as the r41 Cerulean Sphinx cheat-into-play `*Card` duplication described in `docs/loki-r41-report.md` and the r41 follow-up merge (`2f2caf1`, Cerulean Sphinx fix at −24% violation reduction). The new dominant card is Adric, Mathematical Genius — a Doctor Who set card the corpus loaded for the gauntlet. The r41 root-cause analysis still applies: a tutor / search-opponent's-library / Bribery-shaped path leaks the `*Card` pointer without removing it from the source zone. The post-Cerulean fix tackled the most common offender; Adric represents the next-most-common one to chase.

None of the R42 picks touch the cheat-into-play / tutor surface, so this cluster is carryover, not regression.

## Nightmare Boards

Only **2 violations across 10000 boards** (99.99% clean) — well below the r41 baseline of 6 / 10000. No R42 cards involved in either.

## Reproduction

```bash
# Worktree at /Users/joshuawiedeman/Documents/GitHub/HexDek/.claude/worktrees/dev+loki-r42-validate
go run ./cmd/hexdek-loki \
  --games 3000 \
  --seed 42 \
  --report /tmp/loki-r42-report.md
```

Raw artifacts are gitignored; the per-violation game-state dumps live at `/tmp/loki-r42-report.md` on the validation host (2454 lines, 30 violation details + summary). The aggregate numbers above are the only state the repo retains.

## Verdict

✅ **R42 stub batch ships clean.** Zero crashes, zero panics, dirty-game rate held vs. r41, none of the five new handlers implicated in any violation. The remaining noise is the upstream cheat-into-play `*Card` leak the r41 follow-up partially addressed — out of scope for this run.

The next-best lead for a Loki follow-up is the Adric / Inventor's Axe carryover cluster, which extends the same root cause r41 hit with Cerulean Sphinx. No new bugs surfaced by R42 itself.
