# Thor Goldilocks — R60 Post-Engine-Clean Re-Sweep

**Date:** 2026-05-24
**Branch:** `dev/goldilocks-r60-post-audit-r60` (built from `origin/main` @ `f1de9f2`)
**Invocation:** `hexdek-thor -goldilocks --failures-csv /tmp/goldilocks-r60-post.csv`
**Runtime:** 505 ms over 31,963 effect-tests + 2,013 keyword-tests (63 k tests/s)
**Baseline compared:** `docs/goldilocks-r60-report.md` (PR #102, same corpus, pre-r59/r60 engine fixes)

## Headline

| metric                  | baseline (r60 report) | this run | delta |
| ----------------------- | --------------------: | -------: | ----: |
| cards tested            |                35,708 |   35,708 |     0 |
| effect tests            |                31,963 |   31,963 |     0 |
| effect passes           |                31,944 |   31,962 |   +18 |
| effect panics           |                     0 |        0 |     0 |
| effect dead-effects     |                     0 |        0 |     0 |
| **invariant fails**     |                **19** |    **1** | **−18 (−95%)** |
| keyword tests           |                 2,013 |    2,013 |     0 |
| keyword fails           |                     0 |        0 |     0 |

**Result: 19 → 1 invariant failures. Both pre-existing clusters cleared.
One new single-card failure surfaced (different invariant family).**

## What cleared since #102

| invariant              | baseline | now | cleared | landed in |
| ---------------------- | -------: | --: | ------: | --------- |
| `ZoneCastGrantExpiry`  |       17 |   0 |     −17 | r59 (impulse_play arms, commits `c6c6…` / `c711…`) + r60 (heist / `may_play_exiled_free` arms — branch `dev/zonecast-grant-expiry-r60`, 2026-05-23 issue-log row) |
| `TurnStructure`        |        2 |   0 |      −2 | scaffold `gs.Step = "begin_combat"` → `"beginning_of_combat"` in `cmd/hexdek-thor/conditional_setup.go:5023` (one-line scaffold fix from the #102 recommended-fix block) |

Both fixes match the recommended fixes called out in the #102 report. The
ZoneCastGrantExpiry cluster was correctly diagnosed as broader than the
Loki r41 8-hit estimate — the EOT reaper / per-call-site duration stamp
fix has now caught every observed leak in the deterministic sweep.

## What's new

### `ResourceConservation` (1) — **new category, new card**

```
Abduction
  goldilocks_invariant
  [untap] ResourceConservation: seat 0 is Lost but has ManaPool=10
```

Single card, single message. Not in #102, not in CLAUDE.md issue log.

**Shape.** The invariant runs at the test post-snapshot and fires when a
seat is `Lost=true` but still holds mana. ManaPool=10 matches the
scaffold mana stamp used widely in `cmd/hexdek-thor/advanced_mechanics.go`
and `claim_verifier.go` (`gs.Seats[0].ManaPool = 10`), so the scaffold
front-loads seat 0 with mana, then something during the Abduction
ability test transitions seat 0 to Lost without draining the pool.

**Where Lost gets set without a mana drain.** Every `s.Lost = true` site
in `internal/gameengine/sba.go` (lines 193, 321, 351, 373, 1711) toggles
the flag and writes a `LossReason`, but none clear `s.ManaPool` or
`s.Mana`. The §704.5e empty-mana-pool reaper runs on phase change, not
on the loss transition — so a seat that loses mid-step retains whatever
pool the scaffold (or a mana-ability) put there until the next phase
boundary, which the invariant snapshot beats.

**Why Abduction specifically.** Oracle text:

> Enchant creature
> When this Aura enters, untap enchanted creature.
> You control enchanted creature.
> When enchanted creature dies, return that card to the battlefield
> under its owner's control.

The "return to owner's control" branch routes through the die-trigger
resolver, which is the goldilocks effect under test. The scaffold path
for that test most likely puts seat 0 at a life total that flips to
Lost during the test (commander damage, life loss from the test
fixture, or a scaffolded mass-removal precondition), leaving the
seat 0 ManaPool=10 untouched. Abduction is the *trigger* — the bug is
in the Lost-transition mana cleanup, not in the card.

**Severity.** Low. One card surfaces it in the deterministic sweep;
Loki has not flagged it in any recent fuzz pass (no `ResourceConservation`
row in the CLAUDE.md issue log). The window where a Lost seat carries
mana is small (next phase boundary clears it via the §704.5e reaper),
and downstream play is unaffected because a Lost seat can't spend
anything. The invariant catches a *bookkeeping* leak, not a
correctness regression.

**Recommended fix (engine, ~5 LOC).** Introduce a `markSeatLost(s,
reason)` helper that sets `Lost`, `LossReason`, and zeroes `ManaPool`
+ `Mana` (if non-nil), then use it at all five `s.Lost = true` sites
in `sba.go`. Alternative: extend `checkResourceConservation` to
auto-drain Lost-seat mana with a one-line `s.ManaPool = 0` reaper
before the assertion — but that hides rather than fixes the underlying
state-cleanup gap, so the helper approach is preferred.

A targeted regression in `internal/gameengine/sba_lost_resource_drain_r60_test.go`
that marks a seat Lost via each of the five sites and asserts
`ManaPool == 0 && (Mana == nil || Mana.Total() == 0)` would pin it.

## Categorization

| category                            | count | status |
| ----------------------------------- | ----: | ------ |
| Pre-existing, cleared by r59/r60    |    18 | ✅ done |
| New, single-card single-invariant   |     1 | open — log row recommended below |

## Issue-log addition

Recommended addition under "Open" in CLAUDE.md:

| Date | Source | Issue | Severity | Notes |
|------|--------|-------|----------|-------|
| 2026-05-24 | Goldilocks R60 post-engine-clean | **ResourceConservation — Lost seats retain ManaPool until next phase**. Single card (Abduction) trips it deterministically in goldilocks after the scaffold front-loads `ManaPool=10`. All five `s.Lost = true` sites in `internal/gameengine/sba.go` (193, 321, 351, 373, 1711) skip mana cleanup; the §704.5e empty-pool reaper only runs on phase change, so the post-snapshot invariant catches the residual pool. | Low | One-helper engine fix: `markSeatLost(s, reason)` that zeroes `ManaPool` + `Mana`, used at all five sites. Loki hasn't surfaced this in any recent run (no live-game-visible misbehavior — Lost seats can't spend mana — purely a bookkeeping leak). |

## Issue-log rows that can move to Resolved

The pre-existing r60 row covering the `ZoneCastGrantExpiry` Loki cluster
is already in the Resolved table (2026-05-23, `dev/zonecast-grant-expiry-r60`).
The deterministic goldilocks sweep confirms zero remaining hits across
the full corpus, so no follow-up needed.

The recommended-additions block in the #102 report has been overtaken
by the fixes themselves and no longer needs to land in the Open table —
both rows it proposed are now zero.

## Run details

- AST corpus: `data/rules/ast_dataset.jsonl` (35,708 cards loaded, 31,963 had testable abilities)
- Oracle corpus: `data/rules/oracle-cards.json` (35,708 cards)
- Workers: default (`runtime.NumCPU()`)
- Phases: default off
- Scaffold flag: **off**
- CSV: `/tmp/goldilocks-r60-post.csv` (1 row)
- Binary: `/tmp/hexdek-thor` built from `origin/main` @ `f1de9f2`

## Conclusion

The r59/r60 engine cleanup work has driven the goldilocks failure rate
from 19 to 1 (0.06% → 0.003%) without introducing any new panics, dead
effects, or keyword regressions. The single remaining failure is a
small bookkeeping leak on the Lost-seat transition — not a correctness
issue and not exercised by Loki fuzz, so it's open-but-low-priority for
a one-helper engine fix.
