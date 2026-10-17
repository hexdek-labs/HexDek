# r161 — the wall-clock backstop disarmed itself

## What r160 bought

r160 published per-guard counters because all nine loop guards logged
only to the event log and the grinder runs `EventLogNone`, which retains
nothing. The round shipped with one claim: *the next long game reports
itself.* It did, within an hour, and the answer was not the one I
expected.

## What production said

Two long games on the r160 segment (2026-10-17, 07:41–09:04):

```
SLOW GAME  677.2s turns=72 s/turn=9.4     completed 08:05:07
SLOW GAME 1693.9s turns=53 s/turn=32.0    completed 08:59:33
```

Against `perGameWallClock = 10 * time.Minute`. The guard counter went
`game_deadline:1` at 08:05:26 and `game_deadline:2` at 09:01:26 — one
increment per long game, each appearing in the window immediately after
the game completed.

So the cap **does** fire. It fired on both. And both games then ran on
for **77s** and **1094s** respectively. The second spent over eighteen
minutes simulating a game already recorded as a draw.

Before r160 this was unanswerable: "never fired" and "fired and said
nothing" produced identical evidence. The counter separated them, and
the count being exactly 1 per game is itself the diagnosis.

## The defect

`CheckGameDeadline` opened with

```go
if gs.Flags != nil && gs.Flags["ended"] == 1 {
    return false
}
```

which reads as double-end protection and is in fact the hole.

`EndGameAsLoopDraw` marks every seat lost, clears the stack and sets
`ended = 1`. It cannot interrupt the turn that is currently executing.
The grinder's turn loop breaks on `gs.CheckEnd()` — which sits
**between** turns:

```go
for turn := 1; turn <= showmatchMaxTurn; turn++ {
    tournament.TakeTurn(gs)        // the spin lives in here
    ...
    if gs.CheckEnd() { break }     // never reached
}
```

A spin inside a single `TakeTurn` never reaches it. That is the exact
scenario the deadline exists for — the file's own header says so: *"A
turn cap cannot bound a single turn."*

So the sequence was: the first check site trips at 600s, ends the game,
returns into the spin — and from that moment every one of the three
check sites (`stack_drain`, `trigger_drain`, `turn_phase_boundary`)
answers "no deadline problem", because the game is now ended. The
backstop switched itself off at the instant it started working.

`s/turn=32.0` on the 1693.9s game confirms the shape: 53 turns, not a
turn-count explosion — individual turns costing half a minute each.

## The fix

Report the breach every time; end the game only once.

```go
if !GameDeadlineExceeded(gs) {
    return false
}
if gs.Flags != nil && gs.Flags["ended"] == 1 {
    return true          // keep unwinding, don't re-end, don't re-count
}
```

Ordering matters and is load-bearing: the early return must key on the
**breach**, not on the flag. Keying it on the flag would make every
ended game report a deadline breach, including the overwhelming majority
that never armed one — tests, parity and the deterministic harnesses.
Control C pins that.

## What this does not fix

**It does not stop the spin; it stops the simulation of an ended game.**
Whatever loop is costing 32 seconds per turn is still uncapped, and
finding it is a separate round. What changes is that the game now
unwinds at the first check site it reaches after the deadline instead of
running to completion. The three sites are hit often in ordinary play,
but a loop that reaches none of them is still unbounded — the deadline
is a backstop, not a scheduler preemption.

**The bound is therefore "until the next check site", not 600s.** On the
evidence here the first site was reached promptly in both games (the
guard fired), so the expected improvement is most of the 77s and most of
the 1094s — but that is a prediction, not a measurement, and the next
round should judge it on the SLOW GAME census rather than on throughput.

## Correction carried forward

r160's doc recorded three untested hypotheses for the long games: the
deadline flag being lost from `gs.Flags` mid-game, `EndGameAsLoopDraw`
not stopping the turn loop, and the long games not being grinder games.

The first and third are **wrong**: the flag survived (the guard fired)
and both games were grinder games. The second is **right in substance
and wrong in mechanism** — `EndGameAsLoopDraw` does end the game
correctly; it is `CheckGameDeadline`'s own early return that prevents
anything from acting on it afterwards. The brief's instruction not to
guess was the right one; the guess closest to the truth was still wrong
about where the defect lived.

## Throughput note

The r160 segment's raw median reads 266.0 g/min against r159's 451.6.
That is not a regression. Three episodes of exactly-halved throughput
(one worker of two held by a long game) cover 40 of the segment's 77
minutes. Clean bands:

```
                     n    g/min median   MB/game median
r159                83       451.6           14.91
r160 clean bands    74       447.1           15.01
```

Unchanged on both axes, as predicted for pure instrumentation.
`bumpLoopGuard` and `LogLoopGuardFired` do not appear at all in a
400-node CPU profile of the live process.

The halved bands carry a distinct fingerprint — g/min ~232 with MB/game
rising 15.0 → 17.0, while turns/game and gc/game hold steady — which is
worth keeping: it identifies "one worker held by a long game" from the
grinder line alone, without waiting for the SLOW GAME line that is only
written at completion.

## Controls

3 controls, all red, each naming different tests (4 of 5 tests went red
across them; the nil-safety assertion never reds, by design).

- **A** — restore `ended == 1 → return false`: reds
  `ExpiredDeadlineStillReportsAfterTheGameEnded` and
  `TheGameIsEndedAndCountedExactlyOnce`.
- **B** — remove the short-circuit so an ended game re-ends and re-counts
  on every call: reds `TheGameIsEndedAndCountedExactlyOnce`.
- **C** — drop the breach check so the branch keys on the flag: reds
  `UnarmedGameIsUnaffectedByTheEndedFlag` and
  `UnexpiredDeadlineReportsNothingWhenEnded`.

The count test asserts on `LoopGuardCounts()["game_deadline"]`, i.e. the
same number production prints, rather than on an invisible side effect.

### A stale test that codified the defect

`TestGameDeadline_AlreadyEndedIsInert` failed on the first gate. Its
comment states the intent correctly — *"once a game is over the guard
must not re-fire and re-log on every subsequent loop iteration"* — but
its assertion was `CheckGameDeadline(...) == false`, conflating whether
the guard **fires** with what the call site is **told**. The fix
preserves the stated intent exactly and changes only the second.

The assertion was corrected to check the intent directly: the guard
counter does not increment and no new event is logged. It is not a
control and stays green under control A; it does red under control B,
so it is not vacuous. Same class as the r60 combat-damage test that had
codified the behaviour it was supposed to prevent.

## Also found, not fixed — the depth cap nobody could see

r160's counters surfaced something larger than the deadline:

```
percard_dispatch_depth_cap:  24,991
sba_max_passes:                  68
trigger_loop_cap:                57
trigger_drain_cap:               21
game_deadline:                    2
```

`percard_dispatch_depth_cap` fires when `fireTrigger`'s
`gs.Flags["trigger_depth"]` exceeds 8, and on firing it **returns
without dispatching any handler for that event**. Over 33,192 games it
fired 30,201 times:

```
per game   median 0.38   mean 0.93   p90 2.19   max 8.56
windows with zero firings: 10 of 188
```

So it is continuous rather than episodic — it fires in 95% of 30-second
windows, and no long game is needed to provoke it. `PushPerCardTrigger`'s comment asserts "legitimate
nesting is shallow — fireTrigger's own dispatch-depth cap is 8"; that
assertion has never been measurable until now, and 24,991 firings
contradict the premise it rests on.

This is a rules-accuracy question, not a performance one: every firing
is a per-card triggered ability that did not go on the stack. Whether
those are genuine runaways being correctly contained or real chains
being truncated cannot be told from a total — the counter needs to carry
the **event name**, which is a one-line generalisation of r160's
`bumpLoopGuard` key. That is the next round, and it should be measured
before anything is changed.
