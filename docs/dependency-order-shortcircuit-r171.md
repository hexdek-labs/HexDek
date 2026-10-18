# r171 — DependencyOrder short-circuit, and a CR 613.8a gap filed not fixed

## What this changes

One guard in `DependencyOrder` (`internal/gameengine/layers.go`) and the
removal of a dead block in `effectDependsOn`.

`DependencyOrder` builds an n x n dependency matrix by calling
`effectDependsOn(a, b, gs)` for every ordered pair. r152 and r159 both
worked on this function and both attacked its **allocation** — r152 made
the matrix lazy, r159 made it borrowed. Neither touched the **scan**,
which is still O(n^2) and still runs in full.

`effectDependsOn` has three blocks. Only two of them can return true:

| block | fires when | can return true |
|---|---|---|
| 1 — layer-4 / layer-4 | `b.Layer == LayerType && a.Layer == LayerType` | **no — body is comments only** |
| 2 — ability | `b.Layer == LayerAbility && a.Layer <= LayerAbility` | yes |
| 3 — type-change | `b.Layer == LayerType && a.Layer > LayerType` | yes |

So an effect set containing no `LayerAbility` effect, and not containing
both a `LayerType` effect and one above it, **cannot produce a
dependency** — every one of the n^2 calls is guaranteed to return false.

`applyLayer` (layers.go:843) is the **only** caller in the repository, and
it filters candidates to a single `(layer, sublayer)` before calling. With
every effect at the same layer, block 3 is unreachable and block 1 returns
nothing, so **only the layer-6 pass can ever produce a dependency**.
`GetEffectiveCharacteristics` makes thirteen `applyLayer` calls. Twelve of
them were paying a full quadratic scan to rediscover "no dependency".

The guard is written against the general effect set rather than against
that single-layer invariant, so it stays correct if a future caller passes
a mixed slice. It is deliberately conservative: it can only ever cause the
scan to **run** when it need not have, never to be **skipped** when a
dependency was possible.

## Why now

The r170 segment stalled once — one game, 575.5s, 23 turns at 25.0 s/turn.
The watcher caught the whole episode: nine CPU captures, all inside the
band. This is the **third** distinct stall shape and it is not the token
swarm:

```
EP1  coatOfArmsBonus 47.57% cum / permSuppressesCreatureType 23.30%
EP2  permSuppressesCreatureType 16.88% / FireEvolveTriggers 14.61%
EP3  DependencyOrder 47.76% cum / effectDependsOn 39.37% / the grant predicate 17.82% flat
```

Stable across all nine captures: DependencyOrder 46.6–47.8%,
effectDependsOn 38.9–40.7%, predicate 16.3–19.6% flat.

The spine is `gameStatePressure -> effectiveBoardPower -> PowerOf (88.65%)
-> GetEffectiveCharacteristics -> applyLayer -> DependencyOrder`. In a
verified clean band `effectiveBoardPower` is 10.28% cum but `PowerOf` is
only 0.69% and `DependencyOrder` does not appear in the top 200 at all.
The AI's board evaluation is normal; what explodes is the layer
recomputation underneath it.

**`permSuppressesCreatureType` is 1.77% here.** The brief carried it as
"the common factor" across EP1 and EP2 and told me to price it first. I
did: it is common to the two token-swarm episodes, not to stalls in
general. Coat of Arms is absent from EP3 entirely.

## A hypothesis I killed with one query

Three of the four decks in the stalled pod are equipment/aura lists
(Sidar Jabari of Zhalfir, Killian Decisive Mentor x2), so I expected the
board to be thick with keyword-granting lords and was ready to report
that as the cause. I counted instead, against the live r140 AST dataset
on .206:

```
sidar_jabari_of_zhalfir_b3_defiledsin      95 cards   0 keyword_grant statics
killian_decisive_mentor_b3_auzara          83 cards   0
killian_decisive_mentor_b2_argentumrook    86 cards   0
korvold_fae_cursed_king_b4_omniblightking  85 cards   1   (Archetype of Aggression)
```

**One card in the whole pod.** 104 cards in the corpus carry the static
and exactly one of them is here. So the 17.82% flat is a SINGLE closure
called an enormous number of times, not a board full of lords.

That also corrects the shape of the cost. Block 2 only calls
`b.Predicate` when `b` is the LayerAbility effect, so one such `b` means
roughly n calls per `DependencyOrder`, not n^2. The quadratic part is the
loop and the per-pair entry checks (`DependencyOrder` 7.80% flat +
`effectDependsOn` 8.07% flat); the predicate and its defer wrapper
(17.82% + 10.52%) are linear in n but paid on every one of the thirteen
passes, for every permanent, for every board evaluation.

The guard removes twelve of those thirteen passes either way.

## What this does NOT do

**It does not fix the EP3 hot path.** The layer-6 pass is exactly as
expensive as before — that is where the 12.52s of the 19.11s lives. This
change removes the other twelve passes. Benchmarked, the layer-6 arm is
unchanged within noise.

**It will not show up in the clean band.** `DependencyOrder` is absent
from the clean band's top 200. A g/min A/B on ordinary games cannot
produce information here, so I am not running one and not claiming a
throughput number. The evidence is the benchmark below.

**The real prize is still open.** `DependencyOrder(candidates, gs)` does
not take the permanent being evaluated, and nothing it reads depends on
it — so for every permanent on the board, in the same epoch, it recomputes
a provably identical answer. Memoising it per `(layer, sublayer, epoch)`
is the change that would actually cut EP3. It needs an invalidation story
first: the grant predicate closes over `src.Controller` and reads
`t.Controller`, and a direct write to `Permanent.Controller` by a handler
would not bump `charCacheEpoch`. That is the same class as the charCache
base/layered split already on the backlog, and it is not guessed at here.

## The correctness finding — FILED, NOT FIXED

Block 1 called `b.Predicate(gs, a.SourcePerm)` inside a deferred `recover`
and threw the answer away; its `if` body was comments. Removing the call
is behaviour-preserving. **The gap it marked is real and is not closed.**

CR 613.8a, verified against `data/rules/MagicCompRules-20260819.txt`
in-session:

> An effect is said to "depend on" another if (a) it's applied in the same
> layer (and, if applicable, sublayer) as the other effect; (b) applying
> the other would change the text or the existence of the first effect,
> what it applies to, or what it does to any of the things it applies to;
> and (c) neither effect is from a characteristic-defining ability or both
> effects are from characteristic-defining abilities.

Three things follow:

1. **Two layer-4 type-changing effects can depend on each other** — Blood
   Moon + Urborg is the canonical pair — and this engine does not detect
   it. Block 1 was the only place that could have.
2. **Block 3 encodes a cross-layer dependency**, which 613.8a's clause (a)
   excludes. It is unreachable from `applyLayer` today, so it is latent
   rather than live, but it is wrong as written.
3. **613.8c is absent.** The order is computed once; the rule requires
   re-evaluation after each effect is applied.

Closing (1) is a rules change, not a cleanup: it would make a dependency
fire that never has, which moves real game outcomes and makes ELO
incomparable for boards with two layer-4 type-changers. It is filed.

Note also that `DependencyOrder`'s own doc comment claims it "produces
correct results for all canonical test cases (Humility + Opalescence,
Blood Moon + Urborg, Conspiracy + Opalescence)". **That claim is not
backed by anything in the function.** Conspiracy + Opalescence is covered
elsewhere — `layer_tribe_dependency_r63_test.go` exercises a layer-4
type-grant feeding a layer-7 anthem, which works because layer 4 applies
before layer 7 in the natural order, not because of `DependencyOrder`.

## Measurement

`DependencyOrder` called directly, median of 5, `-benchtime 300ms`:

| set | before | after | |
|---|---|---|---|
| layer 7, n=20 | 6,965 ns | 76.8 ns | 91x |
| layer 7, n=40 | 27,159 ns | 147.3 ns | 184x |
| layer 7, n=80 | 91,229 ns | 307.7 ns | **297x** |
| layer 4, n=20 | 13,423 ns | 75.1 ns | 179x |
| layer 4, n=40 | 51,059 ns | 148.9 ns | 343x |
| layer 4, n=80 | 203,345 ns | 321.2 ns | **633x** |
| layer 6, n=20 | 15,779 ns | 16,032 ns | unchanged |
| layer 6, n=40 | 60,823 ns | 61,110 ns | unchanged |
| layer 6, n=80 | 239,512 ns | 213,445 ns | unchanged (noise) |

Layer 4 was 2.2x slower than layer 7 before the change, at the same n, for
the same nil result. That gap **is** the dead block's discarded predicate
call plus its defer — the dead code, measured.

My first pass at this used `-benchtime 300x` and read layer 7 n=80 at
51,861 ns against the 91,229 ns above, and read layer 6 as 41% *slower*
after the change. Both were small-n artefacts. The count=5 numbers are the
ones to use.

**Prediction, stated before deploying: no measurable change in the clean
band.** Revert threshold: a clean-band median below 500 g/min, or any new
red in the gate.

## Tests

`internal/gameengine/dependency_order_shortcircuit_r171_test.go`, 6 tests.

Five of them are behaviour tests and **all five pass at HEAD** — which is
the point: r171 is behaviour-preserving on every semantic axis. They pin
that a layer-6 dependency still reorders, that the same pair without a
dependency keeps timestamp order, that six non-dependency layers pass
through untouched even with predicates that match everything, that a
mixed-layer type-change set still takes the full scan, and that nil/tiny
inputs are unchanged.

The sixth is the transport test (r168's lesson). A scan that finds nothing
and a scan that never ran return the identical slice, so the five above
cannot tell the guard apart from its absence. Before r171 a pure layer-4
set reached block 1, which **called the predicate**; that call is
observable. `TestR171_PureLayerTypeSetNeverInvokesAPredicate` asserts it
never runs, and it is the only one of the six that reds at HEAD.

## Controls: 3 red, 1 invalid and replaced

- **A — full HEAD `layers.go`.** 5 pass, transport test reds. Correct on
  both counts.
- **B — guard always skips.** **INVALID: it does not compile** (`declared
  and not used: hasAbility`). A control that fails to build is not a red
  test (r170). Replaced.
- **B2 — always skip, written so every variable stays used.** Compiles.
  Reds the two positive dependency tests.
- **C — guard inverted** (skip exactly when a dependency is possible).
  Reds the same two.

## ELO

Comparable. No rules behaviour changes: the only removed computation had
its result discarded, and the skipped scans provably returned false. The
613.8a gap is pre-existing and untouched.
