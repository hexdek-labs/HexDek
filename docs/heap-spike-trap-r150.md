# r150 — the heap excursion is an allocation-RATE event, and the process now profiles itself

Continues `docs/heap-outage-r145.md`, `docs/heap-rate-r146.md`,
`docs/allocation-r148.md`. Written 2026-10-16.

## What happened

Three excursions in eighty minutes. The third killed the server.

| | time | `sys` before → after | peak heap | aborts | the game that fits |
|---|---|---|---|---|---|
| 1 | 17:35 | 1,226 → 5,162MB | 1,382MB | 1 (+1 deferred) | `SLOW GAME 43.6s s/turn=0.7` |
| 2 | 18:21–18:22 | 5,180 → 24,743MB | **17,445MB** | 610 | **`SLOW GAME 1536.7s turns=65 s/turn=23.6`** |
| 3 | ~18:52 | — | **18,498MB** | 250 | not yet identified |

Excursion 3 reached 18,498MB on a box with 15,834MB of RAM. The process
died and took the wrapper with it, because `run-server.sh` is a
`while true` loop living inside the process group that gets killed. That
is open item (1) and this is the second time it has cost real downtime:
the last log line is 18:52:38 and the restart listened at 18:56:51, so
about four minutes, ended only because somebody happened to look.

The kill cannot be *confirmed* as an OOM kill, because there is still no
kernel log access on .206 (open item (2)). An unconfirmable cause is not
a confirmed one. What is certain is the heap figure and the fact that it
exceeded physical memory.

## The fork from r146 is resolved: bytes are CREATED, not retained

`docs/heap-rate-r146.md` wrote down the fork in advance — if `alloc=`
spikes the engine is making garbage, and if `alloc=` stays flat while the
heap climbs the engine is retaining it, which would be a within-game leak
and would explain why a soft `GOMEMLIMIT` was powerless.

`alloc=` spikes, and not subtly:

```
18:21:21  21394 games  126.1 g/min  turns/game=56.5  heap=817MB    alloc=1718MB
18:21:51  21647 games  231.9 g/min  turns/game=33.0  heap=11262MB  alloc=20202MB
18:22:21  22127 games  969.4 g/min  turns/game=10.1  heap=17445MB  alloc=12050MB
18:22:51  22455 games  655.9 g/min  turns/game=45.8  heap=828MB    alloc=4278MB
```

Twenty gigabytes churned in one thirty-second window against a ~2,250MB
baseline. The instrument built in r146 killed its own author's hypothesis
then (`clones=0`) and has now answered the question it was actually built
for. It is a garbage-creation event.

Note the 969.4 g/min at `turns/game=10.1` in the third line: that is the
abort burst reading as a throughput spike, exactly as r146 documented.
Check `turns/game` before believing a high `g/min`.

## Why the obvious next step does not work

"`alloc=` spikes → `-list` the top mover" is the right instinct and it
cannot be executed. Go's allocs profile is **cumulative over the process
lifetime**. At a steady ~4,600MB per thirty seconds, a process up for
three hours has allocated well over a terabyte, and a 30GB burst inside
that rounds to roughly two percent spread across whatever the burst
touched. Taking a profile after an excursion measures steady state.

The burst is only visible as the **difference** between a profile from
before it and one from during it — which is what `go tool pprof -base`
computes, and which requires somebody to be at the keyboard at the moment
an unpredictable event begins. Nobody is.

## What r150 builds

The process takes its own profiles. A rolling baseline is written every
ten windows (five minutes), and when a detector fires, a spike profile is
written immediately and the baseline rotation is frozen so the pre-spike
baseline cannot be overwritten by the next rotation.

`internal/hexapi/alloc_spike_r150.go`. The detector is deliberately
separate from the file writing: it is pure, allocation-free (pinned by
`testing.AllocsPerRun` at zero — the thing watching allocation must not
allocate) and testable against a real log series.

**The quantity is MB per GAME, not per window.** The per-window figure is
a rate and throughput here swings between a 250 g/min and a 550 g/min
regime. Across the 137 healthy windows of this run the per-window figure
spans 1,925–4,919MB (2.56×) while the per-game figure spans 13.0–23.7MB
(1.82×).

**The threshold is 1.5× the trailing median of twenty windows, and it was
measured rather than chosen.** Over the 142 windows of the run:

| threshold | firings | false positives |
|---|---|---|
| 1.5× | 3 | **0** |
| 2.0× | 1 | 0 — but misses excursion 1 entirely |

1.5× is the lowest multiple that never fires on a healthy window and the
highest that still catches both excursions. Below it the predictor starts
reporting ordinary windows; above it, it sleeps through the smaller
event.

**A plain cooldown would have missed the event.** Excursion 2 opened at
27.3 MB/game and peaked two windows later at 172.7. A cooldown long
enough to survive a 600-abort storm spends its one profile on the
precursor and sleeps through the peak — which the control below
demonstrates, not predicts. So a window that exceeds the last reported
one by 2× fires regardless of the cooldown.

Replayed over the real series the detector fires at windows 10, 101 and
103: both excursions caught, the peak reported, **zero false positives in
142 windows**.

## Controls — four, three red, and the green one found a hole

| control | result |
|---|---|
| A — threshold 1.5× → 1.0× | **RED**, five false positives on healthy windows |
| B — drop the escalation clause | **RED**, fires at [10, 101] and never reports the 172.67 peak |
| C — let empty (0-game) windows into the median | **green at first** |
| D — per-window instead of per-game | green, and it corrected me |

Control C staying green meant the test had a hole, not that the guard was
unnecessary. The test flooded the ring with zeros, which drives the
median to zero and hits the `median <= 0` rejection — the easy case. The
case that bites is *half* a ring of zeros: it halves the median and
manufactures a spike on a perfectly ordinary window. Re-aimed at every
window in the sequence rather than only the one after it, the control
goes red, and it fires on an ordinary 17 MB/game window at a median
dragged to 8.50.

Control D corrected a claim I had written into the source. The comment
asserted that per-window normalisation would produce false positives,
carried over from an earlier analysis against a *global* median. Replayed
through this detector — trailing median plus cooldown — the per-window
series also yields zero false positives on this run; it reports two of
the three windows, losing the precursor. The per-game choice buys the
precursor and buys not re-tuning the threshold when throughput shifts
regime. It does not buy a cleaner run, and the comment now says so.

## What this does NOT do

It does not fix anything. The engine still allocates twenty gigabytes in
thirty seconds under some condition, and r150 does not know what that
condition is. This is instrumentation whose entire purpose is that the
*next* excursion arrives with its own evidence attached.

It also cannot be red in a behavioural regression. Reverting it does not
break a game; it blinds the operator. The tests pin the detector's
judgement, which is the part that can be wrong.

## Reading the next one

```
grep 'ALLOC SPIKE' ~/hexdek/server.log
ls /tmp/hexdek-allocprof/
go tool pprof -top -sample_index=alloc_space \
  -base /tmp/hexdek-allocprof/alloc-baseline-frozen-<ts>.pb.gz \
  <binary matching the running sha> /tmp/hexdek-allocprof/alloc-spike-<ts>.pb.gz
```

The `-base` is the whole point: without it the result is steady state
again. The binary must match the running sha or the symbols are wrong.

## Circumstantial, and labelled as such

Two things are suggestive and neither is established.

The **s/turn discriminator** held for a third time. The 1,536.7s game at
s/turn=23.6 is the slowest ever recorded here and it completed at the
exact minute excursion 2 peaked. That is now three excursions and three
coincidences with no false positives, but the SLOW GAME line is written
at *completion* and carries no seed, so it is a postmortem correlation
and not a repro.

The abort census at the moment of the kill shows two of four seats with
`lib=0` and `gy=90`/`gy=93` **at turn 10** — two decked seats inside ten
turns, which looks like a self-mill loop. This is a *bystander* game
aborted by the ceiling, not the culprit, and the "deck" hypothesis is
already on the dead list. Recorded because it is the first census with a
distinctive shape, not because it is a lead. The trap will name the site;
reading tea leaves will not.

## Still open for a human

1. **`run-server.sh` must become a systemd unit.** Second outage caused
   by the supervisor dying with the thing it supervises.
2. No kernel log access on .206 — an OOM kill cannot be confirmed.
3. No liveness monitoring. Both outages were found by a person looking.
