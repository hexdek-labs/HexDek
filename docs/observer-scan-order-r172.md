# r172 — predicate ordering in the two observer-ETB board scans

## What this changes

Three reorderings. No new data structures, no caches, no new invalidation
story. Every change swaps two **pure** predicates so the selective one runs
before the expensive one.

| # | Site | Before | After |
|---|------|--------|-------|
| 1 | `Permanent.IsCreature` (`state.go`) | `!permSuppressesCreatureType(p) && hasType("creature")` | `hasType("creature") && !permSuppressesCreatureType(p)` |
| 2 | `FireEvolveTriggers` (`keywords_misc.go`) | `IsCreature(p)` then the evolve test | the evolve test then `IsCreature(p)` |
| 3 | `FireSoulbondTriggers` (`keywords_batch2.go`) | `IsCreature(p)` then `HasSoulbond(p)` | `HasSoulbond(p)` then `IsCreature(p)` |

`(!S && T)` and `(T && !S)` are the same answer for two pure predicates.
That is the whole correctness argument for #1, and #2/#3 are the same
argument applied to a loop filter.

## Why — measured, not guessed

Nine captures across two production stall episodes (EP4 at 03:00–03:04,
EP5 at 04:01–04:08 on 2026-10-18), plus three more from a third episode
after the worker change. The spine is identical in all of them:

```
resolveCreateToken -> FirePermanentETBTriggers -> fireObserverETBTriggers
```

`fireObserverETBTriggers` is **32.55% cum** of the stalled worker, and its
own body is only **0.8% flat** — it is a thin wrapper. The cost is the two
board-wide scans it calls at the end:

```
FireSoulbondTriggers   6.46s  16.19%   -> IsCreature 4.67s (72.29%)
FireEvolveTriggers     6.02s  15.08%   -> IsCreature 3.79s (62.96%)
```

So `IsCreature` alone is **8.46s of 39.91s — 21.2% of the stalled worker**,
reached from these two scans and almost nowhere else (90.5% of its total
samples come from exactly these two callers).

Breaking `IsCreature` down one level further is where it becomes
embarrassing:

```
(*Permanent).IsCreature            9.35s  23.43%
  permSuppressesCreatureType       6.84s  73.16%   (inlined)
    runtime.mapaccess1_faststr     5.46s  79.48%
  (*Permanent).hasType             2.45s  26.20%   (inlined)
```

**13.7% of the entire stalled worker is three string-keyed map lookups**
into `p.Flags` for `bestowed`, `reconfigured` and
`not_creature_while_impending` — three flags that are essentially never
set. The suppression check costs 2.8x what the type check costs, and it
was running first.

## The part worth reading: r80 already found this, one level down

`permHasTriggerEvent` carries this comment, written by r80:

> **ORDER MATTERS HERE, and getting it wrong cost ~55% of grinder
> throughput.** These callers (FireLandfallTriggers and friends) scan
> EVERY permanent on every land drop / ETB / spell cast, and almost none
> of them carry the trigger. r79 put HasLostAllAbilities first, so every
> scanned permanent paid a full layer-pipeline query.

That is precisely the defect fixed here — and `FireEvolveTriggers`, which
*calls* `permHasTriggerEvent`, still asked `IsCreature` first. **r80 fixed
the inside of the predicate and never looked at its callers.** A
predecessor who "already optimised" a function may have fixed a different
axis.

## What this does NOT do

- **It does not touch the quadratic.** `resolveCreateToken` fires ETB
  triggers inside its per-token loop, and each ETB walks every battlefield.
  That is the actual shape of the stall; this change makes each walk
  cheaper, not rarer. The bound on what this can win is the 21.2% above.
- **It does not change which permanents evolve or pair.** See the controls.
- **It does not help a swarm via change #1.** In a board of creature
  tokens `hasType` returns true and the suppression check still runs.
  Change #1 pays off on mixed boards; changes #2/#3 pay off in the swarm.

## A structural finding from the controls

`PairSoulbond` independently re-validates **every** condition the loop
pre-filters — `IsCreature` on both permanents, same controller, and
`IsPaired` on both. So the loop's `IsCreature` test and its
`IsPaired(newCreature)` break are **pure early-outs, not correctness
gates**. Removing either cannot change behaviour, which is why controls C
and D stay green. That is not a hole in the tests; it is a proof that the
soulbond reorder is safe by construction rather than by argument.

Control E (make `HasSoulbond` never answer true) reds three tests, which
is what establishes the positive soulbond tests are load-bearing.

## Controls

| Control | Builds | Result |
|---|---|---|
| A — `IsCreature` forgets the suppression check | yes | **3 RED** |
| B — evolve scan forgets `IsCreature` | yes | **2 RED** |
| C — soulbond scan forgets `IsCreature` | yes | green, **provably** (see above) |
| D — soulbond loses the relocated break | yes | green, **provably** (see above) |
| E — `HasSoulbond` never true | yes | **3 RED** |

Three red on genuine assertions; two green with a proven structural
reason. **No control was invalid** — all five compiled.

## Tests

Ten tests in `observer_scan_order_r172_test.go`. **All ten also pass at
HEAD, by design** — the change is behaviour-preserving, so they prove it
breaks nothing, not that it does something. The evidence that it *does*
something is the profile above and the benchmark.

## ELO

**Comparable.** No game outcome changes.

## Measurement honesty

The wall-clock benchmark was taken on a box under heavy contention (two
agents compiling, load average >20). Absolute ns/op from that run are not
quotable and are not quoted here. The load-independent evidence is the
production profile and the allocation counts.
