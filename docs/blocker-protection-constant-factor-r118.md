# r118 — the remaining hot-path sweep, and a constant factor in blocker declaration

## What this round was

(16-TIMING-3b): r112 swept the state-based actions and r116 swept the untap
step, both O(n²) in board size, both found by the same shape — a battlefield
walk nested inside a battlefield walk. Combat, trigger dispatch and the stack
were never swept. r115's worker-capture games are wide-board decks, so board
size is the right variable.

## The detector first

`scripts/detect_nested_battlefield_walks_r116.go.txt` over `internal/gameengine`:

```
functions that walk the battlefield: 794
DIRECT nested battlefield walks: 3
  combat.go            propagandaTaxFor                 L2936 / L2947
  cost_modifiers.go    ScanCostModifiersWithContext     L520  / L660
  per_card/chishiro…   chishiroEndStep                  L65   / L76
INDIRECT: 4
  cost_modifiers.go    ScanCostModifiersWithContext  L830  calls CountCreaturesOnBattlefield()
  keywords_p0.go       CheckBestowFalloffs           L1029 calls permanentOnBattlefield()
  phases.go            PhaseInAll                    L815  calls PhaseIn()
  sba.go               sba704_5n                     L1171 calls permanentOnBattlefield()
```

Five of the seven were triaged benign in r116. The two that are new here:

- **`ScanCostModifiersWithContext` L830 → `CountCreaturesOnBattlefield`** sits
  inside a `case "Witherbloom, the Balancer":` arm — the same class as the three
  DIRECT hits. Bounded by copies of one card, not by board size.
- **`PhaseInAll` → `PhaseIn`** is gated on `p.PhasedOut`, which is false for
  essentially every permanent, and the inner walk is the indirect-attachment
  work that phasing a permanent in inherently requires.

So the detector is clean, and anything still superlinear is in code it cannot
see: a map rebuilt per call, a slice grown quadratically, a sort per element.

## The sweep

`internal/gameengine/hotpath_scaling_bench_r118_test.go`, board spread across
four seats (the production shape), n = 40 … 2560. **Normalised by the inherent
work** — trigger dispatch and the cleanup scans are legitimately O(board) and
are reported per permanent; blocking and combat damage are legitimately
O(attacker × blocker pairs) and are reported per pair. A flat per-unit figure
is a linear algorithm with a constant factor; only a climbing one is
algorithmic (r113).

| path | n=40 | n=2560 | verdict |
|---|---|---|---|
| `FirePhaseTriggers` | 25.2 ns/perm | 17.4 ns/perm | flat |
| `ScanExpiredDurations` | 97.9 ns/perm | 116.3 ns/perm | flat |
| `DeclareAttackers` | 478.8 ns/perm | 786.7 ns/perm | flat (per-doubling ratio ~2.1, not climbing) |
| `DealCombatDamageStep` | 678.6 ns/pair | 21.9 ns/pair | **falls** — it is O(attackers), not O(pairs); my denominator was the wrong one |
| `DeclareBlockers` | 2792 ns/pair | 2561 ns/pair | flat **but enormous** |

`FireCardTrigger` and `FireDelayedTriggers` measured flat at ~12 ns/op
regardless of n, which is an early return on an empty registration set, not a
board-scaling result. **Those two rows prove nothing** and are kept only so the
next round does not re-run them expecting a signal.

**Nothing in the sweep is superlinear.** The finding is the absolute number on
the one path that is inherently quadratic: at 2,561 ns per pair, a single
`DeclareBlockers` call costs 249 ms at n=1280 and 1.05 s at n=2560.

## The constant factor

Profile at n=640: `canBlockGS` 43% and `scoreBlock → incomingIsLethal` 37%,
both landing in **`attackerHasProtectionFromGS`, 36% cumulative**, inside which
`protectionColors` and `ProtectionTypes` each allocate a fresh map and walk the
permanent's entire `Flags` map and AST ability list.

Protection is a property of **one permanent**. It was being recomputed once per
(attacker, blocker) **pair**, twice over with the roles swapped. That is the
r112/r116 shape — a per-permanent predicate evaluated per pair — with one
difference that has to be said plainly: **the inner work is O(flags), not
O(board), so this was never an exponent.** It is a constant factor on an
inherently quadratic path, and it is reported as one.

The fix folds both functions into one `protectionProfile(p)` that makes a
single pass and allocates nothing when the permanent has no protection, which
is almost every permanent. `protectionColors` and `ProtectionTypes` remain as
thin wrappers for their other three callers.

```
                 before        after
n=40          2792 ns/pair  1905 ns/pair
n=640         2778 ns/pair  1602 ns/pair
n=1280        2436 ns/pair  1511 ns/pair   (249.4 ms -> 154.7 ms per call)
n=2560        2561 ns/pair  1566 ns/pair   (1.049 s  -> 0.641 s  per call)
```

**1.64x, and still flat** — the shape is unchanged because it was always
correct. This removes a term; it does not change the complexity, and it should
not be described as if it did.

## What is left, measured not guessed

Re-profiled after the change. `keywordActive` / `HasKeywordOf` is now the
dominant remaining term at **33% cumulative** via `canBlockGS`, with
`GetEffectiveCharacteristics` at 20% underneath it — the per-pair flying /
reach / menace / shadow checks going through the layer system. Same hoistable
shape, materially bigger job, and it touches the keyword authority split that
already has ~93 open call sites. **Its own round, not this one.**

## Coverage note

The AST plural forms had no test anywhere in the suite. The first draft of this
fix derived the singular key by substring containment, which silently works for
`creatures` / `artifacts` / `lands` and silently fails for `sorceries`, because
`"from sorceries"` does not contain `"from sorcery"`. It passed the entire
existing protection suite. `protection_profile_r118_test.go` covers all seven
types in both forms, all three flag vocabularies, and the `prot_type:` axis's
long-standing lack of value-gating.
