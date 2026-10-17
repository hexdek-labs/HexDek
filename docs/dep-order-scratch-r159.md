# r159 — the n² dependency matrix, borrowed instead of allocated

## The spike that found it

The r150 trap fired on the r158 binary, and r154's self-diagnosis
answered the first question without my having to derive it:

```
ALLOC SPIKE 28.4 MB/game vs median 14.6 (1.94x)
  [alloc 3064MB vs 3068MB prev (-0%), games 108 vs 169 prev (-36%):
   DENOMINATOR FELL — fewer games completed, check for a SLOW GAME]
```

There was a slow game: **109.0s, 53 turns, 2.1 s/turn**, logged a
minute later because slow games are logged at completion.

**The tag is a pointer, not a verdict**, so I read the capture. Diffed
against the frozen baseline:

```
DependencyOrder      1187MB   40.38% of the window's differenced profile
BaseCharacteristics   464MB   15.78%
applyLayer           1367MB   46.49% cum
```

and `-list` put it on one line:

```
layers.go:607   depends = make([]bool, n*n)      1.07GB
```

Against a steady-state share **below 2%** — `DependencyOrder` does not
appear in the frozen baseline's top ten at all, consistent with r153's
measured 1.32%.

## This is not a regression, it is r152's documented residual

r152 made the matrix **lazy**: nothing is allocated unless a real CR
§613.8 dependency exists, which removed the common case entirely. Its
comment, still in the file, says what it left behind:

> The window contained one game running at 24.1 s/turn for 506 seconds,
> which is what a quadratic per-call allocation looks like once a board
> grows enough effects.

That is this window, eight rounds later. **The slow game and the
allocation are one phenomenon**: the board grows continuous effects, n²
grows quadratically, every call pays it, the game crawls, fewer games
complete, and MB/game spikes. "DENOMINATOR FELL" is a correct
description of the *rate* and an incomplete one of the *cause*.

## The fix

Borrow the matrix from a per-GameState free list instead of allocating
it. The matrix never escapes — `DependencyOrder` returns the reordered
effects slice, never `depends`.

**A free list, not a single scratch field, because re-entrancy is real
here.** `effectDependsOn` calls `b.Predicate(gs, perm)`; a predicate
typically asks `gs.IsCreatureOf` or `gs.HasTypeOf`, which reach
`GetEffectiveCharacteristics` → `applyLayer` → `DependencyOrder`. A
nested call must not be handed the buffer its caller is filling.

Buffers above 65,536 bools are dropped rather than pooled, so one
pathological board does not pin memory for the life of the process.

## Measured in-test

```
bytes/call:  pooled=384  unpooled=960  saved=576   (the 24x24 matrix is 576 bytes)
allocs/call: pooled=2    unpooled=3
```

Exactly the matrix, and exactly one allocation.

## Two things I got wrong, both caught by controls

**1. My first headline asserted ZERO allocations and was simply wrong.**
`DependencyOrder` also builds `visited` (8n) and `order` (8n), which
this round does not touch. Those are linear; the matrix is the
quadratic one and the only one worth pooling. The test now measures
**bytes** against an unpooled control, because a round about an n²
term cannot be judged by an allocation *count*.

**2. My re-entrancy test was vacuous three times running, and a control
that refused to go red is the only reason I know.**

- First version: outer set small, nested set large — the nested call
  could not have reused the outer buffer even with pooling broken,
  because the capacity was too small. Green under the control.
- Second: inverted the sizes so the nested matrix fits inside the
  outer. Still green — the free list starts *empty*, so within one cold
  call there is nothing to mis-share.
- Third: warmed the list first. Still green.
- A probe to watch it was itself allocating and releasing buffers,
  **contaminating the thing it measured**.

Staging a real clobber depends on what is in the pool at that instant,
which depends on nesting history I could not make deterministic. So the
test now asserts **the invariant instead of the symptom**: *while a
buffer is in use it is not in the free list*. Nothing can hand it out
twice if it is not there to hand out. The control reds immediately.

## Controls

**5 of 5 red** — A (never pool) reds 3 including the precondition,
B (reuse without zeroing) reds the stale-true test, C (single scratch)
reds the invariant test **on the fourth attempt**, D (pool oversized
buffers), E (drop the nil-GameState guard).

## Prediction

**Median allocation unchanged, and that is the point.** Steady state
this line is under 2%, so the median cannot move. What should move is
the **tail**: r158's segment had max 28.37 MB/game against r157's 16.98.
I expect the max to come down and spike firings on stall windows to
become less frequent. Throughput unchanged — the n² *scan* in
`effectDependsOn` is untouched; only the allocation is.

This is the first round here whose prediction is about a tail rather
than a median, and it should be judged on max and on spike count, not
on the number I usually report.

## ELO

**Comparable.** The matrix contents and the resulting order are
identical; only its storage changed.
