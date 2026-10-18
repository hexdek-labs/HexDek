# r170 — the SLOW GAME line carries its seed, and two nil-receiver fixes

## What it changes

`grinder: SLOW GAME 599.9s turns=60 s/turn=10.0 decks=[...]`
→ `grinder: SLOW GAME 599.9s turns=60 s/turn=10.0 seed=1792283906113211000 decks=[...]`

One field. The seed already existed — `gameSeed` at `showmatch.go:2032`, also
stored on `gs.Seed`, and already printed for *aborts* by `abortContext`
(`game_census_r131.go:78`) since r131. It was simply out of scope at the
deferred log, because `gameSeed :=` is assigned eighty lines below the
`defer` that writes the line. So this round is a scope change plus a print,
and the key order matches `abortContext` so the two lines join.

## Why now

Nine capped pods have been logged since r166 and **not one is replayable**.
That has blocked every repro attempt in the r166–r169 chain: the quadratic
token-creation stall is pinned by goroutine dumps and CPU profiles, but
nobody can re-run the game that produced them. Duration, turns and s/turn
say a game was pathological; none of them let you run it again.

Two of those nine were logged in the r169 segment alone (23:28:06 and
00:11:01 on 2026-10-17/18), and both are gone.

## What it does NOT do

It does not make a past game replayable. The nine already logged stay
unreplayable; this only helps from the next one onward. It also does not
prove a seed is *sufficient* to reproduce a stall — the grinder draws its
pod from a shared RNG and the deck pool changes between restarts, so a
replay harness still has to pin the deck list too. The deck list is on the
same line, which is the point of putting the seed beside it rather than
somewhere else.

## Two nil-receiver defects found by testing it

The test discipline here is r168's: drive the production entry point, never
hand-feed the value the feature transports. Driving the real
`runOneGameFast` immediately surfaced a recovered panic — and then a second
one behind it.

**1. `hat.SelfPlayManager.RecordSamples` (`selfplay.go:76`).** A nil manager
deref. `WriteEnrichedSamples` calls it unconditionally, and all four call
sites invoke it on `sm.selfPlay` without a guard. This is the defect noted
in the standing brief as "a recovered nil-deref that prints in the hexapi
gate inside a test that still PASSES — pre-existing, unfiled". It has been
reported for twenty-four rounds and never filed, because nobody had a
reproduction. There is one now, and it is two lines.

**2. `muninnAdapter`, all six methods (`muninn_adapter.go`).** `sm.muninnSink`
is a typed nil in any `Showmatch` built without full init, so `EndGame`
panicked *on its own `a.batcher == nil` check*. Three of the six guarded the
batcher but not the receiver; `RecordParserGaps`, `RecordDeadTriggers` and
`RecordCrash` guarded neither.

`RecordCrash` is the one that matters: it is called from the grinder's
`recover()`, where a panic is not recoverable a second time and would take
one of two workers with it.

**Both are production-unreachable today** — `sm.selfPlay` and the muninn sink
are assigned unconditionally during `Showmatch` init (`showmatch.go:739`), so
a live server never has a nil there. This is a test-reachable robustness gap,
not a live bug, and it is written up that way deliberately rather than
inflated. What it *was* costing is real: a stack trace in the gate on every
run, inside tests that reported PASS.

## Tests

`grinder_slow_game_seed_r170_test.go` drives the real `runOneGameFast` over a
real 62-turn game (`maintenanceFixture`, `slowGameThreshold` lowered via a
`const`→`var` change made only for this) and **brackets the printed seed
between two `UnixNano` readings taken around the call**. A seed that is not
transported prints 0, and 0 is not between two timestamps. The other
direction — a game under the threshold logs nothing — is its own test.

`selfplay_nil_receiver_r170_test.go` and `muninn_adapter_nil_r170_test.go`
pin both nil defects in both directions: nil is inert, real still works.

`TestR170_AGrinderGameCompletesWithoutARecoveredPanic` is the assertion that
would have caught the original defect. Every existing test passed *through*
that panic because `recover()` swallowed it.

### The stamp caught me

The first run of the seed test passed. Stamping the raw line showed it was
passing while the game **crashed and was recovered** — the SLOW GAME defer
still ran, so the assertion was satisfied by a game that had panicked. The
rule is "stamp the raw flag or it passes vacuously", and this is the round
where it paid. The stamp also showed `turns=52`, which is what proved the
game was otherwise real rather than a stub.

## Controls: 4 red, and one that was not a control at all

- **A — re-introduce the `gameSeed :=` shadow.** *Not a valid control.* It
  does not compile: the hoisted `var gameSeed int64` becomes unused, which
  Go rejects. **The shadowing failure mode this round was designed around is
  compiler-prevented and cannot reach runtime.** Said plainly because the
  temptation is to count a build failure as a red test.
- **A2 — hoist the var but never assign it** (compiles; the realistic
  refactor hazard): `TestR170_SlowGameLineCarriesTheSeedTheGameActuallyUsed`
  red. This is the transport control.
- **C — revert the `RecordSamples` nil guard:** the nil test red *and*
  `TestR170_AGrinderGameCompletesWithoutARecoveredPanic` red.
- **D — revert the `EndGame` receiver guard:** `TestR170_NilAdapterSurvives
  EveryMethod` red.
- **E — the design-choice control: guard the whole of
  `WriteEnrichedSamples` instead of just the training trigger.**
  `TestR170_NilManagerStillWritesTheSamplesFile` red, hexapi still green —
  which is exactly the point: guarding too broadly silently drops every
  sample a fixture produces, and only the narrow guard keeps the useful half.

## Performance and ELO

No hot-path change. The added work is one `int64` in a log line that fires
above a 10-second threshold — nine times in two days. **No throughput claim
is made and none should be read in.** ELO comparable: nothing about game
selection, resolution or rating changes.
