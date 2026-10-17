# r164 — caching the answer, not the needle

## The target

`strings.Contains` has been the largest single item in the binary for
fourteen rounds and nobody had touched it. On r163:

```
internal/stringslite.Index   12650ms cum   21.13% of ALL CPU
  indexbytebody               5540ms flat   9.25%
```

A `-peek` attributes it almost entirely to `internal/hat` predicates
reading the memoized lowered oracle text:

```
scoreOpponentGraveyard 14.15%   isPersistentDrawEngine  8.71%
isManaOnlyAbility       8.23%   isImpulseDrawEngine     7.59%
hasGraveyardCastKeyword 6.24%   scoreStaxLock           6.08%
scoreThreat             5.92%   isPersistentScryEngine  5.68%
```

## Why not the needle bitmask

The shape written into the brief — and into three previous briefs — was
a per-Card bitmask of NEEDLES, one bit per literal. There are 221
distinct needles across 285 call sites. That is a large table whose
entries mean nothing individually, and the needle is not the thing
anyone asks for twice.

What *is* asked for repeatedly is the **predicate**. Each of these is a
pure function of the lowered oracle text; that text is already memoized
per Card on the AST pointer (r157); and the same card is re-scored on
every evaluation pass of every turn. So cache the **answer**: one bit
per named predicate, about twenty instead of 221, each semantically
meaningful and each independently testable.

This is r157's invalidation reused a third time (r158's `kwNamesAST`,
r162's label map, now this), which is why it is keyed on `card.AST` and
never on a "have we computed this" bool. A DFC transform invalidates
every predicate for free. Control A reintroduces the r157 defect and
`TransformInvalidatesEveryBit` reds immediately.

## Scope

Five predicates, eight call sites. Picked from the profile's named
callers, not from the grep count:

```
isImpulseDrawEngine  isPersistentScryEngine  isPersistentDrawEngine
hasGraveyardCastKeyword  isManaOnlyAbility
```

together roughly 36% of the `strings.Contains` time, so about 7.6% of
all CPU. The three `score*` functions above inline their `Contains`
calls rather than going through named predicates; extracting those is a
separate round and deliberately not attempted here.

`isPersistentDrawEngine` took a `*Permanent` but reads nothing except
`p.Card`'s oracle text — which is what makes it cacheable. It is split
into a pure-text form and a thin shim, and a test pins that the two
still agree.

One incidental win: with the graveyard loop's predicate converted, its
`ot` local became dead, removing an `OracleTextLower` call per
graveyard card per pass.

## Hit rate is published, not assumed

`tp=` on the grinder line carries hits/total, with `tpunc=` if any
out-of-range bit ever falls back. **r155's lesson: I predicted the
charCache hit rate at under 25% and it was 98.1%.** I am not going to
guess this one — it reports itself.

An out-of-range bit returns the correct answer, uncached, and counts
itself. A wrong answer is a bug; a slow answer is a budget. Control D
makes it return false instead and the test reds.

## Correctness

The underlying string functions are unchanged. A differential test runs
every converted predicate against its original over eleven oracle-text
shapes including the empty string, in both directions — **a faster
predicate that answers differently is a rules change, not an
optimisation.** The whole `hat` package passes unmodified.

ELO comparable: no rules behaviour moves.

## Controls

5 controls, all red, each naming different tests.

- **A** — key the memo on a readiness bool instead of the AST (the r157
  defect): reds `TransformInvalidatesEveryBit`.
- **B** — cache only TRUE results: reds `FalseIsCachedAsWellAsTrue` and
  `BitsAreIndependent`. Caching only the positive would re-scan every
  card that does not match, which is the overwhelming majority.
- **C** — share one bit across all predicates: reds
  `BitsAreIndependent` and `TransformInvalidatesEveryBit`.
- **D** — out-of-range bit returns false rather than computing: reds
  `OutOfRangeBitIsCorrectButUncached`.
- **E** — revert the evaluator to the uncached predicates: reds
  `EvaluatorRoutesThroughTheMemo`.

**E is only a real control because of r163.** Last round an identical
control stayed green: the memo's own tests live in `internal/gameengine`
and call `TextPredicate` directly, so they cannot see whether the
evaluator routes through it. The wiring test in `internal/hat` was
written up front this time rather than discovered by a control that
refused to go red.

Writing the tests also caught two of my own mistakes: a fixture that
assumed `OracleTextLower` returns `"flying"` when it appends a trailing
space, and a test that reused bit 0 for two different predicates — the
second of which is a genuine misuse the design should make hard, and
the test now says so in place.

## Prediction

**3–6% throughput, allocation flat or slightly down.** Baselines: r163
452.2 g/min / 14.85 MB/game (n=61); r160 clean bands 447.1 / 15.01
(n=74). Judge it on the segment median with stalls identified, and on
`tp=`: **a hit rate below about 80% would mean the premise is wrong —
that cards are not in fact re-scored — and the round should be
reverted rather than extended.**

## r163, measured

Pure instrumentation, predicted no change, and there is none:

```
                 n    g/min median   MB/game median
r160 clean      74       447.1          15.01
r163            61       452.2          14.85
```

`NoteTriggerDepth` and `bumpTriggerDepth` do not appear in a 500-node
CPU profile.

The depth survival curve now has 19× the sample and holds:

```
depth      1      2      3      4     5     6     7     8     9
n=733    100%  96.9%  35.6%  13.8%  8.3%  6.0%  5.0%  3.7%  2.5%
n=13,776 100%  96.3%  34.3%  12.3%  7.2%  5.1%  3.8%  2.9%  2.2%
```

## r161, still unmeasured — fifth tick

Zero `game_deadline` firings in the r163 segment; longest game 80.8s.
Confirmed harmless, not confirmed effective.
