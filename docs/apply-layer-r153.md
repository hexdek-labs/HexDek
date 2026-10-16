# r153 — applyLayer allocated its candidate slice before the filter

Round r153. Commit `b5eb5d26`.

## A real burst this time

The trap fired twice: 22:28:53 (1.85×) and 22:29:22 (4.20×, escalation
beating the cooldown). r152's own lesson applied first — **check whether the
numerator moved** — and here it did:

```
22:28:22  444.0 g/min  alloc=3580MB  gc/game=0.05
22:28:52  330.0 g/min  alloc=4868MB  gc/game=0.10   <- 1.85x
22:29:22  216.0 g/min  alloc=7239MB  gc/game=0.22   <- 4.20x
22:29:52  405.8 g/min  alloc=3505MB  gc/game=0.06   <- over
```

Allocation doubled and gc/game went 0.05 → 0.22. 12,107MB over 273 games
against ~4,368MB of steady state: **64% excursion**, the cleanest burst
capture in this lane.

## r152 confirmed end to end

`DependencyOrder` is **1.32% of this burst**, down from **41.71%** in the
pre-fix pathological window, and 115MB of `applyLayer`'s 19.23GB lifetime.

r152 said the pathological win "cannot be demonstrated end to end until
another slow game happens, and it is not claimed until it is." It happened,
and it holds.

## The burst is layers-dominated — properly, this time

Decomposing observed share against steady state at 64/36:

| | steady | observed | ⇒ excursion |
|---|---|---|---|
| `GetEffectiveCharacteristics` (cum) | 37.56% | 59.92% | **~72%** |
| `BaseCharacteristics` (cum) | 25.59% | 31.50% | ~35% |
| `applyLayer` (cum) | 11.18% | 27.29% | ~36% |
| `DependencyOrder` | — | 1.32% | ~1% |

**This is not the r150 claim retracted in r151.** That one rested on a
baseline 90% contaminated by steady state. This rests on a 64%-excursion
capture with the numerator verified to have risen.

## The defect

`-list` named the line. `applyLayer:775`:

```go
candidates := make([]*ContinuousEffect, 0, 4)
for _, ce := range gs.ContinuousEffects {
    ... filter ...
}
if len(candidates) == 0 {
    return
}
```

The slice was allocated **before** the filter, so every call paid for it even
when nothing matched — and `GetEffectiveCharacteristics` makes **thirteen**
`applyLayer` calls per computation (one per layer plus each PT sublayer)
while a board's effects concentrate in one or two buckets.

| line | lifetime |
|---|---|
| 775 `candidates := make(..., 0, 4)` | **15.87GB — 9.23% of ALL allocation** |
| 794 `candidates = append(...)` | 839MB |
| 804 `sort.SliceStable` | 1.42GB |
| 818 `DependencyOrder` | 115MB |

Appends at 839MB against the make at 15.87GB means the cost is the **calls**,
not the sizes — regrowth past cap 4 is negligible. Allocate on first match:
the matching path is byte-for-byte unchanged, the empty path allocates
nothing.

Same shape as r152's matrix, one function up, **in the file r152 was already
editing**.

## The control that lied

The first control allocated 0 and did not go red. That was not evidence the
fix was pointless: a trimmed helper that merely returns the slice lets escape
analysis keep it on the **stack**, because the caller discards it. The real
`applyLayer` hands `candidates` to `sort.SliceStable` and `DependencyOrder`,
so it escapes and is heap-allocated at the make site — on every call,
including the ones matching nothing.

**A control has to reproduce the escape or it is measuring a different
program.** Rewritten as a faithful copy: 1 allocation per non-matching call.

## Prediction and result

**Stated before measuring:** unlike r152 this is above the ~5% aggregate
resolution limit, so a visible reduction is expected, order 5–8% per game.

**Measured:**

```
r152   ~16.7 MB/game        408.5 g/min cum   gc/game 0.05-0.07
r153    14.6 median         432.3 g/min cum   gc/game 0.05
        15.37 mean (one 21.9 outlier window, limited=33)
```

**−8% on the mean, −13% on the median.** The prediction held and was
conservative. Throughput interval 420–470 against 390–445; that is suggestive
but a fresh process flatters `g/min cum`, so it is not claimed separately.

0 aborts, 1 slow game, 0 spikes across the measured interval.

## Open

- **`BaseCharacteristics:326`, `c := &Characteristics{}` — 2.79GB in the
  burst**, and `BaseCharacteristics` is still the steady-state top at 25.59%
  cum. The struct itself is the cost; this is the next line the profile
  names.
- **The detector's stalled-worker blind spot** (r152) is still unfixed. It
  needs the in-flight game count.
- **`fireObserverETBTriggers:66`** — 404MB flat, defensive copy, needs a
  scratch buffer and a re-entrancy check.
- **The charCache split** remains the big correctness-critical round, and is
  now better motivated than ever: 72% of excursion bytes sit under
  `GetEffectiveCharacteristics`. Write the invalidation matrix first.
