# r133 — the wrong game was dying

## The live instance

r129 named this defect and could only half-fix it. On 2026-10-15 at
23:27 it stopped being theoretical:

```
23:27:17  heap 2511MB, 130.4 g/min, gc 0.24        escalation arms
23:27:21  aborting game at turn 61 — delta 3363MB
23:27:21  SLOW GAME  13.8s turns=61   <- the game that was ABORTED
23:27:23  SLOW GAME 715.7s turns=73   <- finished normally, 2s later
```

**A 13.8-second game was killed while a 715.7-second game ran alongside
it and completed untouched.**

Two things in that same record are wins and should be read first:

- **turn 61 is the first non-multiple-of-ten abort in the entire log.**
  Every one of the previous 18 landed on 20/30/40/50/60 because the
  guard only looked every tenth turn. r129's escalation — every-turn
  sampling once the delta passes half the budget — caught this one
  mid-stride.
- **the server survived.** No restart between 21:33:04 and the next
  deploy at 23:35:39. This class previously took the process *and* its
  supervising wrapper down, twice in one day.

So the guard fired in time and the box lived. It just killed the wrong
game.

## Why it picks wrong

`runtime.ReadMemStats` reports the **whole process**, and the grinder
runs **2 workers** inside it. `HeapAlloc - heapBaseline` is therefore
not this game's allocation — it is everything both workers have done
since this game's baseline was captured.

The guard aborts whichever game reaches a checkpoint first. That is
uncorrelated with which game is allocating.

## The fix: compare, don't threshold

Go exposes no per-goroutine allocation counter, so the delta cannot be
attributed directly. But when the process is over budget, **the game
containing the most objects is the better suspect** — and that is a
*comparison*, not a threshold. No number has to be invented, and nothing
has to be calibrated against a distribution that does not exist yet
(no r131 census has been captured in the wild).

Each worker publishes its own `gameWeight` — stack depth plus every zone
of every seat, the same counts `gameCensus` prints, collapsed to one
number. When a game's delta trips and a **peer has published a larger
weight**, this game stands down and lets the peer abort at its own next
check.

### Race safety is why it is atomics, not a registry of games

Sizing up another worker's `*GameState` would be a data race on live
state. Instead each worker publishes only its own weight into an
`atomic.Int64`, and the guard reads peers' published integers and
nothing else. The registry is a map of those atomics under a mutex,
exercised concurrently in the tests under `-race`.

### What is deliberately never deferred

- **The absolute process ceiling.** Past 6GB the priority is relieving
  pressure, not assigning blame, so whoever notices aborts.
- **A tie.** Equal weights are no evidence, and deferring both would
  abort neither.

Deferring is only safe *because* of r129: every in-flight game samples
every turn once the delta passes half the budget, so the genuinely big
peer reaches its own check shortly. Without that escalation this change
would risk sparing everybody.

## Verification

Eight regressions, run under `-race`. **Two are red pre-change** — the
small-game deferral itself, and the released-slot test, which goes red
because its precondition is that deferral happens at all. The other six
are must-never-defer guards (ceiling, tie, sole game, weight accounting,
concurrency) that pass beforehand because pre-change nothing ever
deferred; they constrain the new code rather than proving the bug.

## What this does not do

It still does not explain **why** a game allocates gigabytes in under
30 seconds. It makes the abort land on the game that is actually big, so
that when r131's census finally prints, it describes the culprit instead
of a bystander.
