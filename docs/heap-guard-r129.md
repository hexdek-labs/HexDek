# r129 — the guard looked every tenth turn

## What the log says

Prod died twice on 2026-10-15, unattended both times, taking the server
**and its supervising `run-server.sh` wrapper** with it. The second one:

```
20:46:21  aborting game at turn 60 — heap delta 2553MB exceeds budget
20:46:28  aborting game at turn 20 — heap delta 3769MB
20:46:34  aborting game at turn 50 — heap delta 4715MB
20:46:47  aborting game at turn 40 — heap delta 5895MB
```

**Every abort turn is a multiple of ten.** So is every abort in the
history — 20, 30, 40, 50, 60. That is not a coincidence about games; it
is the guard's sampling rate:

```go
if turn%10 == 0 {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    ...
}
```

A guard that looks every ten turns cannot bound anything that grows
faster than ten turns. Measured across the log, the heap at the last
sample before an abort has a **median of 941MB** — against 728MB for a
sagged window that *doesn't* end in an abort. There is no ramp to catch.
The process goes from under a gigabyte to 15216MB inside a single
30-second reporting window.

## Two defects, not one

### 1. Sampling rate

Past half the budget the guard now samples **every turn**. Below that it
keeps the 1-in-10 cadence, so a normal game pays nothing extra —
`runtime.ReadMemStats` stops the world, which is why the escalation has
to be conditional rather than just turning the cadence up for everyone.

### 2. Attribution — the one that makes the delta untrustworthy

`runtime.ReadMemStats` reports the **whole process**, and the grinder
runs **2 workers** inside it:

```
grinder: starting 2 parallel workers (NumCPU=8 GOMAXPROCS=2, GOGC=50, memlimit=8GB)
```

So `HeapAlloc - heapBaseline` is *not* this game's allocation. It
includes whatever the other worker did, and the baseline was itself
captured while the other worker was holding memory.

**One runaway game therefore aborts the innocent game on the other
worker.** The four aborts in 26 seconds above are four different games at
four different turns — the guard killing bystanders while the guilty game
runs on to its own next checkpoint. That is why "the guard fired three
times and still lost": it was firing at the wrong games.

Go exposes no per-goroutine allocation counter, so the delta cannot be
made per-game here. It is kept as a coarse pressure signal, and an
**absolute process ceiling** is added underneath it — attribution-free,
and the number that actually has to hold to keep the box alive. The box
has 15.8GB and runs under `memlimit=8GB`; the ceiling is 6GB.

## What this does and does not fix

It does **not** explain why a game allocates gigabytes. That cause is
still unknown and still open.

It makes the guard able to *act* on it — to abort at 2GB rather than
discovering the problem at 15GB, nine turns too late.

**The wrapper remains the real exposure.** `run-server.sh` is a
`while true` loop inside the process group that dies, so when the group
goes nothing restarts the server. Twice today prod stayed down until a
human looked. That is an ops change (a supervisor outside the group —
systemd, or a watchdog on another box), not an engine change, and it is
left for a human.

## The sag hypothesis, corrected

Last tick I proposed that the throughput sag is an early warning for this
crash and suggested a monitor. **Measured this round across all 18
aborts, and it does not hold in the form I pitched it.**

| claim | result |
|---|---|
| a sag precedes the crash | **confirmed** — 10/12 abort events; the 2 misses have no grinder rows at all |
| a sag predicts a crash | **false** — 118 sag episodes, 25 followed by an abort: **21% precision** |
| heap rises during the sag | **false** — crash-sags peak at a median 941MB vs 728MB for quiet ones |

Four out of five sags are benign, so a monitor built on it would restart
prod four unnecessary times per crash averted. A sag is a **necessary
condition, not a predictor**, and the "heap grows while the game is
captured" mechanism I described is wrong — the growth is sudden, inside
one sampling window, which is the same fact that makes the 10-turn
cadence fatal.

Tried and rejected as a second discriminator: peak heap during the sag
catches 2 of 25 crash-sags (8% recall) at any threshold that keeps
precision up.
