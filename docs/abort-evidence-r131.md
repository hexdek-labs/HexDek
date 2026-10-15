# r131 — the abort now names its own game

## The gap r130 ran into

r130 spent a round establishing which decks were in the 18 heap aborts,
and could answer it for **2**. The method was a join: an aborted game
still emits a `SLOW GAME` line whose `turns=` equals the abort turn, and
that line carries the deck list. The other 16 finished under the slow
threshold, emitted no such line, and are permanently anonymous.

Everything that join recovered is in scope at the abort site in
`runOneGameFast` — `deckKeys`, `gameSeed`, `gs`. The abort simply never
said any of it:

```
grinder: aborting game at turn 60 — heap delta 2553MB exceeds budget
```

A turn number and a delta. Nothing about which decks, which seed, or
what the game contained.

## Why instrumentation rather than another reproduction attempt

The event is roughly **1 in 30,000 games**. Two attempts across 425
games caught nothing, which is exactly what the arithmetic predicts —
about a 1% chance. Reproducing on demand is not a plan.

**And the seed is not a sufficient repro recipe**, which is worth
stating because the judge's own violation text says "repro and
sample-profile this seed" — a promise the system cannot keep. The game
RNG is seeded from the logged seed, but each seat's hat is built from
Curse DNA drawn out of a **mutable, evolving pool** using a *different*
RNG (`sm.getOrCreateCursePool(...).SelectForGame()`), and the pool's
contents change as the run proceeds. Replaying a seed gives you the same
shuffle and a differently-played game.

So the evidence has to be captured at the moment of failure, or it is
gone. That is what this round does.

## What the abort says now

```
grinder: aborting game at turn 60 — heap delta 2553MB exceeds budget
  (seed=1789817169370804852 decks=[moxfield/a moxfield/b moxfield/c moxfield/d])
  stack=7 seats=4 s0=[life=40 bf=5000(tok=4980) hand=3 gy=11 exile=2 lib=60] ...
  perms=5004 tokens=4980
```

`gameCensus` counts **objects** rather than sampling memory, deliberately:
a runaway should show as an absurd number of *something*, and which
something it is points straight at the mechanic. A 5,000-permanent
battlefield, a 9,000-card graveyard and a 400-deep stack are three
different bugs, and the census distinguishes them. It reads only
exported fields and allocates one string.

Both abort paths carry it — the per-game delta and r129's process
ceiling.

## Verification — stated plainly

**This is instrumentation, not a behaviour fix, so nothing here is "red
pre-change": the functions did not exist.** Saying otherwise would be
the vacuous-guard dressing-up this log has been caught at before.

What justifies the round is the measured gap — 2 of 18 — and what
verifies the tests is a **mutation check** rather than a revert. Three
mutations, each failing the arms that should catch it:

| mutation | result |
|---|---|
| drop the token count from the per-seat line | runaway + zone tests fail |
| skip nil seats silently instead of reporting them | nil-safety test fails |
| `abortContext` emits the seed without the decks | identity test fails |

All four tests fail under mutation, so none of them passes by accident.

## What this does not do

It does not explain the allocation. It makes the next occurrence — which
on current rates is a day or two away — explain itself, instead of
costing another round of log archaeology that comes up 2-for-18.
