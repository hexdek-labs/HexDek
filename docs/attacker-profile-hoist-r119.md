# r119 — hoisting the attacker-only block predicates, and three dead hypotheses

## What this round was

(16-TIMING-4), ranked first out of r118: `keywordActive` / `HasKeywordOf`
measured at 33% cumulative of `DeclareBlockers`, with
`GetEffectiveCharacteristics` at 20% underneath. The r118 brief carried a
specific hypothesis, written by me:

> `GetEffectiveCharacteristics` at 20% suggests recomputation, and if so the
> cache invalidation is the real bug, not the call count.

**That was wrong, and it is retracted.** So were the next two guesses. The
fix that landed is the fourth thing I tried, and the three failures are
recorded here because each one looked right on the way in.

## Hypothesis 1 — the layer cache is missing. FALSE.

`go tool pprof -peek 'GetEffectiveCharacteristics$'`:

```
                     70ms  100% |   HasKeywordOf
 10ms  1.33%  1.33%   70ms  9.33% | GetEffectiveCharacteristics
                     60ms 85.71% |   runtime.mapaccess2_fast64
```

No `applyLayer` beneath it at all. **The cache hits 100% of the time** and
the entire cost is the pointer-keyed map lookup. The epoch invalidation is
working exactly as designed. A single `-peek` killed a hypothesis that had
been promoted to ranked-first on the strength of a cumulative percentage.

## Hypothesis 2 — memoize the lowercased keyword set. MEASURED SLOWER.

`HasKeywordOf` scans `Chars.Keywords` and `Chars.Abilities` linearly calling
`strings.ToLower` on each entry. Since the characteristics are cached and
epoch-keyed, a lowercased set could hang off the same cache entry and
inherit its invalidation for free. Implemented; measured back-to-back on the
same machine:

```
                baseline(r118)   with memoized set
n=640             1525 ns/pair      1743 ns/pair
n=2560            1577 ns/pair      1784 ns/pair
```

**13% slower.** The reason is the fixture, and it is an r118 lesson landing
on me one round later: the benchmark's creatures **have no keywords at
all**, so those two loops never execute and never called `ToLower` even
once. The 38.9% attributed to `strings.ToLower` was normalising the QUERY —
`want := strings.ToLower(strings.TrimSpace(kw))` — not the permanent's list.
I had optimised a cost the measurement did not contain, and paid a map
allocation per permanent for it. Reverted.

## Hypothesis 3 — give the query an allocation-free fast path. ALSO SLOWER.

If the cost is normalising a compile-time-constant keyword on every call,
skip `ToLower` when the string is already lowercase ASCII:

```
                baseline(r118)   with lowerKeyword fast path
n=640             1525 ns/pair      1619 ns/pair
n=2560            1577 ns/pair      1670 ns/pair
```

**`strings.ToLower` already does exactly this.** It scans for an uppercase
ASCII byte and returns the original string unchanged when there is none —
no allocation. My version added a second scan in front of the one the
standard library already performs. Reverted. Worth stating plainly: the
premise was not merely unprofitable, it was *already implemented*, and
reading `strings.ToLower`'s source before writing a replacement would have
cost thirty seconds.

## What was actually true — call count, not per-call cost

With per-call cost ruled out three ways, the remaining lever is how often
the calls happen. `LandwalkType` asks `keywordActive` once for **each of the
five basic landwalk keywords** and returns `""` for nearly every creature;
it alone was 50% of all `keywordActive` time. And `DeclareBlockers` iterates
blockers *inside* a single attacker, so every attacker-only predicate in
`canBlockGS` — flying, unblockable, landwalk — was re-derived once per
blocker.

That is the r118 per-pair-predicate shape, and the contract written in r118
names this exact case as the next instance.

`attackerBlockProfile` holds the three attacker-only results.
`canBlockGS` keeps its signature and behaviour — it builds a profile and
delegates — so its ~20 other callers are untouched. The two sites that test
many blockers against one attacker (`DeclareBlockers`' candidate loop and
`sanitizeDeclaredBlockers`) build the profile once and call
`canBlockProfiled`.

```
             r118 baseline   r119
n=40           1823            1484 ns/pair
n=160          1949            1211
n=640          1706            1255
n=1280         1462            1198
n=2560         1691            1226
```

**1.23x–1.38x, still flat.** Run-to-run noise on this machine is about 7%
(the n=2560 baseline read 1577 and 1691 on two runs an hour apart), so the
delta is comfortably outside it, but single-run comparisons against a
number from a previous round are not trustworthy and every figure above is
from one back-to-back pair.

Like r118 this is a **constant factor**: it divides the attacker-side work
by the blocker count and leaves the O(pairs) shape alone, which is correct.
Across r118 and r119 together, `DeclareBlockers` at n=2560 has gone from
2561 to 1226 ns/pair.

## Verification

Semantics are preserved, so the guards pass both ways by construction and
were mutation-checked. The regression tests the **entry point with two
attackers whose evasion differs**, because the failure mode a hoist
introduces is a profile built for one attacker and applied to the next —
and `canBlockGS` still builds its own profile, so no per-pair test can see
it. Mutations: profile built once outside the attacker loop → red (both
tests); landwalk never populated → red; flying never populated → red.
