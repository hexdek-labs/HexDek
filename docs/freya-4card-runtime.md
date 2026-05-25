# Freya 4-card combo detection — runtime tradeoff

## Context

Freya's `FindLoops` (in `cmd/hexdek-freya/analysis.go`) detects resource-flow
cycles among deck cards. Prior to r60 the search topped out at 3-card cycles
because the quadruple combinatorial cost C(100,4) ≈ 3.9M against a 100-card
EDH deck is ~24× the cost of triples (C(100,3) ≈ 161K). Naively extending
the triple loop would push a single `freya` run from sub-second to tens of
seconds on combo-heavy decks.

## Approach taken in r60

Two-part containment:

1. **Prefilter to flow-active candidates.** Only profiles with non-empty
   `Produces` AND non-empty `Consumes` can participate in a cycle by
   definition. In typical EDH decks this drops the working set from ~100 to
   20–50; combo decks reach ~60.
2. **Hard cap at 70 candidates.** Beyond that threshold the 4-card pass is
   skipped entirely (pair + triple detection still run). The skip is silent
   — a deck dense enough to exceed 70 flow-active cards is already heavily
   over-covered by the pair/triple results.
3. **24-permutation enumeration generated structurally**, not hand-listed.
   The nested distinct-index loop (`i,j,k,l` all distinct in [0,4)) emits
   exactly 4! = 24 orderings by construction. `TestQuadPermutationCoverage`
   pins that invariant, mirroring the r59 lesson where the triple-combo
   table silently shipped 2/6 perms for months.

## Measured runtime

Worst-case benchmark (`BenchmarkFindLoopsQuad_WorstCase`) packs the
candidate pool to the 70-card cap with every card fully resource-flowing:

| Candidates | C(n,4)  | FindLoops wall time |
|------------|---------|---------------------|
| 30         | 27,405  | ~100 ms (estimated) |
| 50         | 230,300 | ~750 ms (estimated) |
| 70 (cap)   | 916,895 | ~3.1 s (measured)   |

VirtualApple @ 2.50 GHz, darwin/amd64. Scales as O(n⁴) on the candidate
count, so the cap is the load-bearing knob.

## When this becomes the bottleneck

- A future deck shape that legitimately exceeds 70 flow-active cards and
  needs a true 4-card combo found — open a follow-up to either raise the
  cap with a benchmark refresh, or move to a graph-walk that doesn't visit
  every quadruple (e.g. seeded BFS from each candidate node along
  `Produces ∩ next.Consumes` edges, depth-limited to 4).
- The 5-card extension. C(70,5) × 120 perms is two orders of magnitude
  worse. That would require the graph-walk approach, not the brute-force
  pattern used here for pairs/triples/quads.

## Files

- `cmd/hexdek-freya/analysis.go` — `FindLoops`, `checkQuadCombo`,
  `tryQuadCycle`
- `cmd/hexdek-freya/quad_cycle_test.go` — 4 tests + 1 benchmark
- `cmd/hexdek-freya/triple_cycle_test.go` — r59 precedent for the
  permutation-coverage discipline
