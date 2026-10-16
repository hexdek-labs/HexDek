# r152 — DependencyOrder built an n×n matrix per call and threw it away

Round r152. Commit `6a70f171`.

## The firing that wasn't an allocation excursion

The r150 trap fired at 21:23:47, 1.81× (30.5 MB/game vs median 16.9). It was
not an allocation burst:

```
21:22:17  427.8 g/min   alloc=3359MB
21:23:47  212.1 g/min   alloc=3237MB   <- FIRED
```

**Allocation went down.** What doubled MB/game was the denominator. The log
carries the reason: `SLOW GAME 506.3s turns=21 s/turn=24.1`, completing at
21:31:23 — and throughput returns to 385.9 g/min in the very next window at
21:31:47. With two workers, one game stuck for eight minutes halves completed
games while the other worker keeps allocating at its normal rate.

**This is a real limitation of the r150 detector, found in production.**
Per-game normalisation was chosen to remove the throughput confounder, and it
does — but only for throughput changes that are *unrelated* to allocation. A
stalled worker drops the denominator while the numerator keeps accruing from
the other worker, which inflates MB/game with no extra bytes created at all.
r150 measured 0 false positives over 142 windows; that series evidently
contained no multi-minute single-game stall.

It is nonetheless a **true positive for something else**, and a more useful
one: it caught a pathological game.

## What the window actually contained

```
                      steady state      stalled window
DependencyOrder          2.62%      ->     41.71% flat
applyLayer              13.76%      ->     50.09% cum
BaseCharacteristics     25.01%      ->     13.07% cum   (DOWN)
```

A ~16× shift in share, and specifically **`DependencyOrder`** — not
`BaseCharacteristics`, which every previous layers story in this lane was
built on. `BaseCharacteristics` went *down* relatively.

`-list` named a single line: `depends[i] = make([]bool, n)`, **1.13GB in
thirty seconds**.

## The defect

CR 613.8 dependency ordering materialised the full n×n bool matrix as **n
separate allocations on every call**, computed every cell, then:

```go
if !hasDeps {
    return effects   // already sorted by caller (timestamp order)
}
```

discarded all of it. Measured on the lifetime baseline, by original line
number:

| line | what | lifetime |
|---|---|---|
| 583 | `depends := make([][]bool, n)` | 2.05GB |
| 585 | `depends[i] = make([]bool, n)` | 1.27GB |
| 616 | `visited := make([]int, n)` | 38.04MB |
| 617 | `order := make(...)` | 71.03MB |

Lines 616–617 sit *after* the early return, so they only run when a
dependency exists. **Matrix 3.32GB against dependency-path 109MB is a 30:1
ratio** — the dependency path is almost never taken, so essentially the whole
3.32GB (2.54% of all allocation) was built and discarded.

Two further readings fall out of those numbers. Outer (2.05GB) exceeding rows
(1.27GB) means `8n > n²`, so **typical n is under 8**. In the stalled window
the same ratio inverts (1.13GB rows vs 131MB outer), implying **n ≈ 69** — a
board that had grown enough continuous effects for the quadratic term to bite.

## The fix

Allocate lazily and flat. Nothing is allocated unless a real dependency
exists; when one does, it is one allocation instead of n. The separate
`hasDeps` scan goes with it, because `depends == nil` *is* the answer it was
computing.

## What is and is not claimed

**Stated before measuring:** 2.54% is below what a whole-process per-game
comparison can resolve — the r149 methodology limit. No aggregate gain
predicted.

**Measured after deploying:** 16.7 MB/game against r151.1's 16.5–17.1.
Indistinguishable, as predicted. 408.5 g/min cum, gc/game 0.05–0.07,
turns/game 53.4–57.1, 0 aborts, 0 slow games.

**The claim is the site, and the shape of the win.** It removes nothing
visible from a healthy game and removes the dominant allocation from exactly
the pathological ones the profile caught. That cannot be demonstrated end to
end until another slow game happens, and it is not claimed until it is.

## Open

- **The detector's stall blind spot.** MB/game cannot distinguish "allocating
  more" from "completing fewer". A stalled-worker guard would need the
  in-flight game count, which the window does not currently carry. Worth
  doing — the firing was useful, but it was useful by accident.
- **`fireObserverETBTriggers:66`** — 404MB flat in the 19:42 capture, still
  open; the per-seat battlefield copy is defensive and needs a scratch buffer
  plus a re-entrancy check.
- **The eager `LogEvent` Details maps** — 129MB + 139MB at `resolveCounterMod`
  alone.
- **`applyLayer` and `BaseCharacteristics`** remain the steady-state top and
  are untouched. r152 took the one line the pathological capture actually
  named, not the cache split.
