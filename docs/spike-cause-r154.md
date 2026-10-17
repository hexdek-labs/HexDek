# r154 — a spike firing now says which side of the rate moved

Round r154. Commit `f92513c8`.

## Why

MB/game rises either because the process allocated more, or because fewer
games completed. Those are different events with different diagnoses, and the
last two rounds are the proof:

```
r152  21:23:17  alloc=3417MB  140 games
      21:23:47  alloc=3237MB  106 games   <- fired 1.81x
      allocation FELL; a 506s game stalled one of two workers

r153  22:28:22  alloc=3580MB  222 games
      22:28:52  alloc=4868MB  165 games   <- fired 1.85x
      22:29:22  alloc=7239MB  108 games   <- fired 4.20x
      a real burst; allocation nearly doubled, gc/game 0.05 -> 0.22
```

Same detector, opposite meanings. Telling them apart meant going back to the
log and reading neighbouring windows by hand, **three rounds running**. The
line can say it itself.

The recorder now carries the previous window's raw numerator and denominator;
a firing reports both deltas and a short tag. The raw figures are printed in
full beside the tag, so a reader who disagrees with the label can still do the
arithmetic.

## It also narrows r152's own finding

The brief recorded the stalled-worker blind spot as general. **It is not.**
r153's segment contains a **620.4s** slow game and nothing fired, because
allocation fell in proportion to throughput (2858MB → ~1700–2000MB), so
MB/game stayed near normal:

```
23:16:28  365.9 g/min  alloc=2858MB
23:16:58  190.0 g/min  alloc=1946MB   20.5 MB/game
23:18:28  213.9 g/min  alloc=1803MB   16.9 MB/game
23:19:28  230.0 g/min  alloc=1725MB   15.0 MB/game
```

The false positive requires a stall whose game is **also allocation-heavy** —
which is exactly a game worth catching. In r152's case the stalled game *was*
the `DependencyOrder` pathological one. So the blind spot is real but narrower
than recorded, and the remaining gap is diagnosis, not detection. That is what
r154 closes.

## A correction to r153's reported number

r153 claimed −13% median. That came from an **11-window sample on a fresh
process**. Recomputed over whole segments of ~110 windows each, dropping the
first window of each (prevAlloc starts at 0):

| segment | n | median | mean | p25 | p75 | max |
|---|---|---|---|---|---|---|
| r151.1 | 113 | 17.67 | 19.36 | 16.74 | 19.97 | 32.72 |
| r152 | 104 | 16.33 | 17.12 | 15.83 | 17.16 | 67.03 |
| r153 | 111 | 15.14 | 15.68 | 14.74 | 16.11 | 25.24 |

**r152 → r153 is −7.3% median / −8.4% mean, not −13%.** The 5–8% prediction
was accurate, not conservative; I overcorrected on a small sample and said so.

`r151.1 → r152` also shows −7.6%, but that segment is confounded — the huginn
history was still filling to its cap during it — and r152 predicted no gain.
**No gain is retro-claimed for r152 on confounded data.**

Methodology note for future rounds: **measure a deploy over a whole segment,
not the first ten windows**, and report median alongside mean.

## Claims

**Pure instrumentation.** No behavioural regression can be red for this;
reverting it does not break a game, it blinds the reader. What *is* testable is
`spikeCause`'s judgement, and that is pinned by replaying both real firings.

**Measured after deploying:** 15.1 MB/game against r153's 15.14 median,
421.7 g/min, gc/game 0.05, 0 aborts, 0 slow games, 0 spikes. Unchanged, as
predicted for a log line.

## Open

- **`BaseCharacteristics:326`, `c := &Characteristics{}`** — 2.79GB in the r153
  burst, and still the steady-state top at 25.59% cum. The struct itself is the
  cost, so it probably cannot be removed without the charCache split.
- **`fireObserverETBTriggers:66`** — 404MB flat; defensive copy, needs a scratch
  buffer and a re-entrancy check.
- **The charCache split** — ~72% of r153's excursion bytes under
  `GetEffectiveCharacteristics`. Whole round, invalidation matrix first.
