# r160 — nine loop guards that cannot be seen

## What I was trying to do

r159's open item was the 2148-second game: one pathological game held
one of two grinder workers for 35.8 minutes. The brief said **check for
an existing per-game duration cap before building one**, and that check
paid immediately — one exists.

`perGameWallClock = 10 * time.Minute`, armed in `runOneGameFast`, the
grinder's own game function. Three check sites: `stack_drain`,
`trigger_drain`, and `turn_phase_boundary` inside `takeTurnImpl`, which
the grinder reaches every phase of every turn.

**So the cap is armed, reachable, and constantly evaluated.** And:

```
grep -c 'game_deadline' server.log        ->  0
longest SLOW GAMEs in the whole log:  3102.6s  2148.3s  1536.7s  1256.7s  1162.0s
```

**Five games over 19 minutes against an armed 10-minute cap**, and not
one trace of the guard that was supposed to stop them.

## Why I could not tell what happened

`LogLoopGuardFired` writes exactly one place — the event log. The
grinder runs `EventPolicy = EventLogNone`, whose `LogEvent` case is
literally:

```go
case EventLogNone:
    // Retain nothing; hats observe the local copy.
```

So **every loop guard that fires in production leaves no trace at
all.** Nine of them: `move_no_progress`, `trigger_loop_cap`,
`trigger_drain_cap`, `sba_max_passes`, `drain_iteration_cap`, three
`percard_dispatch_*` caps, and `game_deadline`.

"The cap never fired" and "the cap fired and said nothing" produce
identical evidence. I cannot fix a guard I cannot observe, so this
round makes it observable and nothing else.

## I have hit this exact class before and swept one member

From r142's own comment, still in `showmatch.go`:

> r141 enforced the limiter but logged it through LogEvent, and the
> grinder runs with EventPolicy = EventLogNone, which retains nothing
> — so the only number available said zero no matter what happened.

r142 found the defect, fixed it for **the trigger limiter**, and left
the nine loop guards with the identical shape. That is a direct miss
against the standing rule *audit the whole family before fixing one
member* (r132/r137/r148/r157).

## A claim of mine that was never verifiable

`game_deadline.go`'s header says that during the 2026-10-08 sixteen-day
stall, "zero loop guards had ever fired on that box", and uses that to
argue a deadline is different in kind from a counter guard.

**That observation could not have been made.** A guard that fired on
that box would have left exactly the same evidence as one that did not.
The conclusion may still be right; the evidence for it never existed.

## The change

Publish each firing as a process-wide counter and let the grinder print
it. `internal/gameengine` deliberately imports no `log` — it is a
library and no file in it prints — so this follows r155's shape for the
charCache hit rate rather than teaching the engine to log.

- `bumpLoopGuard(guard)` in `LogLoopGuardFired`, **before** the
  `gs == nil` guard: a guard that fires without a GameState still fired.
- `LoopGuardCounts()` snapshots only guards that actually fired.
- `LoopGuardSummary()` renders `game_deadline:2,sba_max_passes:1`,
  sorted, and returns `""` when nothing has fired — so the window line
  gains no permanent noise column.
- The grinder appends ` guards=...` only when non-empty.

## Controls

**5 of 5 red, E on the second attempt.**

| control | reds |
|---|---|
| A — bump moved inside the `gs == nil` guard | 2 |
| B — snapshot includes zero-valued guards | 1 |
| C — summary not sorted | 1 |
| D — bump is a no-op | 4 |
| E — `Store(1)` instead of `Add(1)` | **first attempt GREEN** |

E's first attempt patched the `LoadOrStore` branch, which runs only on
a guard's **first** firing; every subsequent bump takes the fast path
and still added correctly. **Here the control was invalid, not the
test** — the distinction matters, because r159's lesson was the
opposite one. Re-patched on the fast path, it reds 2.

The headline test is direction-paired in an unusual way: one test pins
that `EventLogNone` **does** drop the event (so the round is not
solving an imaginary problem) and the next pins that the counter
survives it.

## Prediction

**Pure instrumentation. No throughput or allocation change, and no
behaviour change of any kind** — a counter increment on a path that
fires at most a handful of times per process. The guard-counting code
itself is testable, which is why there are controls at all; the
*engine behaviour* half of this round cannot be red, and I am saying so
rather than implying the controls cover it.

What this buys is the next 20-minute game reporting itself instead of
requiring this archaeology.

## ELO

**Comparable.** Nothing in game resolution changes.
