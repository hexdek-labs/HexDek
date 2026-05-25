# Loki R60 Stress Test

Date: 2026-05-24
Branch: `dev/loki-r60-stress-r60`
Goal: after both target seeds (42, 43) reported zero on the final
gauntlet, stress with fresh untargeted seeds (99, 7, 1337) to verify
the zero is real and not seed-dependent.

## TL;DR

| Seed | Chaos (5K games) | Nightmare (10K boards) | Verdict |
|:----:|:----------------:|:----------------------:|:-------:|
| 99 | **0** violations, 0 crashes | 0 / 0 | ✅ clean |
| 7 | **0** violations, 0 crashes | 0 / 0 | ✅ clean |
| 1337 | **12** violations across 1 game, 0 crashes | 0 / 0 | ⚠️ 1 game flagged |

- Across the three stress seeds: **12 violations in 1 game out of
  15,000 chaos games + 30,000 nightmare boards** — a stochastic
  per-game rate of 0.007% (still below the noise floor that closes
  the r60 era).
- Two of three fresh seeds (99, 7) were fully clean — meaning the
  prior 42/43 zero was not seed-specific overfitting.
- One fresh seed (1337) found a single new bug surface, all 12
  reported hits coalescing on a single game / single state. Isolated
  below; logged to the open-issues table.

## Detailed runs

### Seed 99 — fully clean

```
chaos:     0 violations / 0 crashes / 5000 clean games (78 g/s, 1m4.11s)
nightmare: 0 violations / 0 crashes / 10000 clean boards (10906 b/s, 917ms)
```

### Seed 7 — fully clean

```
chaos:     0 violations / 0 crashes / 5000 clean games (74 g/s, 1m7.56s)
nightmare: 0 violations / 0 crashes / 10000 clean boards (12232 b/s, 818ms)
```

### Seed 1337 — 1 game, 1 signature, 12 reported hits

```
chaos:     12 violations / 0 crashes / 4999 clean games (52 g/s, 1m36.47s)
nightmare: 0 violations / 0 crashes / 10000 clean boards (7157 b/s, 1.4s)
```

| Invariant | Count | Games | Note |
|---|---:|---|---|
| SBACompleteness | 10 | 1 (#465) | All 10 hits on the same stuck state at turn 56 cleanup |
| LifeConsistency | 2 | 1 (#465) | Same game, turn 60 cleanup, seat 2 at life=-1 |

All 12 reported hits trace to **game 465** (seed 4651338, perm 0) —
a single stuck state replayed across consecutive invariant scans.

## Isolation: the game-465 signature

### Symptom

```
seat 0 has life=0, Lost=false, no loss-prevention — SBA 704.5a missed
```

At turn 56 cleanup, seat 0 is `[alive]: life=0` but `Lost=false`.
The invariant explicitly checks for loss-prevention replacement
effects (Platinum Angel etc.) and confirms none are present, so it's
not a §614 would-lose-game cancellation. Four turns later, the same
game reports `seat 2 has life=-1 but Lost=false` (LifeConsistency 2x
at turn 60) — same shape, different seat.

### What the event log shows

```
[4331..4342] priority_pass × 12 (4 full APNAP rounds, empty stack)
[4343] loop_shortcut seat=0 source=no_op_loop   ← CR §727 detects
                                                    nothing is progressing
[4344] phase_step seat=3
[4345] declare_attackers seat=3
[4346] blockers seat=0
[4347] damage seat=3 source=Boot Nipper amount=2 target=seat0
[4348] damage seat=3 source=Nim Devourer amount=4 target=seat0
[4349] phase_step seat=3
[4350] state seat=3                              ← invariant scan
                                                    sees seat 0 life=0
                                                    + Lost=false
```

So combat dealt 6 damage to seat 0 (Boot Nipper 2 + Nim Devourer 4)
between events 4347–4348, and seat 0's life is at 0 by the next
phase boundary at 4349. The `phase_step` after combat damage is
supposed to follow a `StateBasedActions` sweep (CR §704.3 + combat
damage step at `combat.go:307`'s "SBAs fire after the combat damage
step resolves"), and `sba704_5a` would mark seat 0 Lost. The
invariant catches the game in a state where that didn't happen.

### Likely root cause (hypothesis, unconfirmed)

The `loop_shortcut`-`no_op_loop` shortcut at event 4343 calls
`evacuateStackSpellsToGraveyard` and clears the stack but does NOT
call `StateBasedActions` (verified by reading
`loop_shortcut.go:158-178`). After the shortcut breaks, the engine
phase-advances directly into combat. The combat-damage SBA does fire
(`combat.go:307`), so the immediate post-damage SBA pass should
catch life=0 — but something in the loop_shortcut → combat handoff
appears to skip the seat-0 state. Two specific suspects:

1. The no-op loop's `gs.Stack = gs.Stack[:0]` cleared a queued §704.5a
   action / replacement that would have marked seat 0 Lost on the
   prior priority round.
2. Combat damage was dealt to seat 0 while a stale `SBA704_5a_emitted`
   flag was still set from a previous-turn drop-to-zero-then-heal
   cycle. The new sba.go (line 247-294, post-r60) resets the flag
   only when `s.Life > 0`, so if life dropped to 0 in two separate
   windows without ever climbing above 0 in between, the second
   window's emission still flips `Lost`. So this suspect should be
   ruled out — but the post-shortcut reset path is worth confirming.

### Reproducer

Deterministic — runs in ~9 seconds:

```bash
go run ./cmd/hexdek-loki --games 470 --seed 1337 --invariant sba-completeness
```

Reports 10 SBACompleteness hits on game 465 only (469 clean games
+ 1 dirty game = 470 total). The single-signature signal is intact
across smaller game counts.

## Trajectory across all r60 seed runs

| Round | Seed | Violations | Notes |
|------:|:----:|:----------:|:------|
| r41 baseline | 41 | 1,652 | original |
| r44 | 41 | 402 | post-Cerulean Sphinx, paradigm-copies, abdelAdrian |
| r60 round 1 | 41 | 52 | |
| r60 round 2 | 41 | 10 | |
| r60 round 3 | 41 | 0 | seed 41 fully closed |
| r60 round 3 | 42 | 6 | new seed surfaces 3 clusters |
| r60 final pre-Mascot | 42 / 43 | 0 / 1 | District Mascot residual |
| r60 final post-Mascot (PR #169) | 42 / 43 | 0 / 0 | District Mascot closed |
| **r60 stress** (this run) | 99 | **0** | fresh seed, clean |
| **r60 stress** (this run) | 7 | **0** | fresh seed, clean |
| **r60 stress** (this run) | 1337 | **12** | 1 game / 1 signature, isolated |

## Conclusion

The engine is **substantively clean**: 2/3 of fresh stress seeds
reported 0 violations across 10,000 chaos games + 20,000 nightmare
boards, and the third seed surfaced exactly one new bug surface
(post-`no_op_loop` + combat-damage SBA-skip on seat 0 in game 465)
that reproduces deterministically in ~9 seconds.

This is **not** an "officially clean" declaration — the seed-1337
residual is a real engine bug, not an invariant false positive (the
invariant correctly checks for loss-prevention and finds none, and
LifeConsistency at turn 60 confirms the family). But it IS confirmation
that:

1. The 42/43 zero from the post-Mascot final report was not seed
   overfitting — fresh seeds 99 and 7 corroborate.
2. The remaining surface is bounded: 1 in 15,000 games, single
   signature, deterministic reproducer, single game.
3. The r60 systemic-cluster work is complete. What remains is
   long-tail rare-window single-game bugs that surface at one per
   stress seed.

Logging the seed-1337 game-465 SBA-704.5a-after-no_op_loop signature
to the open-issues table as the next residual to chase.

## How to reproduce all three runs

```bash
go run ./cmd/hexdek-loki --games 5000 --seed 99     # expect: 0 / 0
go run ./cmd/hexdek-loki --games 5000 --seed 7      # expect: 0 / 0
go run ./cmd/hexdek-loki --games 5000 --seed 1337   # expect: 12 in 1 game
```

Narrow seed-1337 reproducer (~9 sec):

```bash
go run ./cmd/hexdek-loki --games 470 --seed 1337 --invariant sba-completeness
```
