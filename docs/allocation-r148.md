# r148 — the first fix the profiler aimed, and what it measured

r147 switched on pprof. r148 is the first round scoped by it rather than by a
hypothesis. Two sites, both named by the profile.

## 1. `fireTrigger`'s dedupe map — 4.75GB, 14.74% of all allocation

One line, `scanned[perm] = true`, was the largest allocation site in the
binary. A `map[*Permanent]bool` built from empty on **every trigger event**,
filled with **every permanent on every seat's battlefield**.

It bought one boolean. The battlefield scan cannot produce a duplicate — a
Permanent sits on exactly one seat's battlefield — so the map existed only so
the leaving-perm `collect` at the end of the function could ask *"did I already
handle this one?"*. That site's own comment says so in as many words.

A permanent is already-scanned exactly when it is **on a battlefield**. So
`onAnySeatBattlefield(gs, cp)` asks the battlefield once, at the one site that
cares, instead of n map inserts up front. The helper mirrors `fireTrigger`'s own
scan: same nil-seat skip, same pointer identity, because `Permanent` has no ID
field and the pointer *is* the object.

## 2. `isSignet` / `isTalisman` / `isMox` — 1.42GB for a ten-entry constant

Each built its lookup table inside the function, so every call allocated.
Hoisted to package-level maps. The slice-to-map conversion dedupes `"Mox Ruby"`,
which the old slice listed twice — harmless in a linear scan, harmless now.

## Measured, before and after, on production

Per-window `alloc=` is not the honest comparison on its own, because throughput
rose at the same time — more games per window would mask a per-game saving.
Normalising:

```
               alloc/window   games/window   MB per game
  r146/r147      ~5,560MB         ~274          20.3
  r148           ~4,846MB         ~287          16.9      -17%
```

The profile confirms it directly rather than by inference:

```
                              before            after
  isSignet                    1.42GB  4.4%      ABSENT
  fireTrigger (cum share)      9.71GB 30.06%    7.39GB 18.28%
  fireTrigger.func2 (collect)  4.76GB 14.74%    ABSENT
```

**I predicted ~19% and measured 17%.** gc/game went 0.07 → 0.06.

**What I am NOT claiming: a throughput win.** Readings moved from ~530–560 to
~546–588 g/min, but two points either side of a code change are not a
measurement, and this pool has shown that spread already. `alloc=` is a direct
measure and is what the claim rests on.

## What the after-profile says to do next

The pie is smaller, so the remaining shares are larger. The new top is
`BaseCharacteristics` at **23.36%, 9.45GB cum**, with `applyLayer` 12.08% and
`collectPresentInstanceIDsForSweep` 8.24% behind it — the layers system, which
recomputes characteristics constantly.

Also still untouched and now visible:

- **`os.readFileContents` 7.65%** — something is re-reading files during play.
  Worth finding out what, and whether it should be cached.
- **~1GB of `LogEvent` Details maps.** The grinder runs `EventPolicy =
  EventLogNone`, which retains nothing, but the `map[string]interface{}` is
  built **at the call site** and so is paid for in full before the sink drops
  it. `LogEvent` cannot fix this from the inside; it needs either guarded call
  sites or a lazy variant.

None of those are measured as fixes yet. They are named by the profile, which
is the only reason to believe they are worth the time.

---

# r149 — the fix worked exactly as designed at the site, and I cannot show it end to end

## What shipped

`collectPresentInstanceIDsForSweep` built its id set from an **empty** map and
then filled it with every card in every zone of every seat. The insert line was
**17.82GB, 8.58% of all allocation** — second only to the layers system.
Pre-sized from `countCardsForSweep(gs)`, which walks slice lengths only.

## Measured at the site — unambiguous

```
  before   present[c.InstanceID] = struct{}{}      17.82GB   8.58%
           present := map[string]struct{}{}        44.50MB
  after    present := make(..., countCards(gs))     1.52GB   4.24%
           (the insert line is GONE from the profile entirely)
```

The growth-and-rehash allocation is eliminated. What remains is one correctly
sized allocation. In-package the same population costs **4 allocations instead
of 11**.

## Not measured end to end, and I am not going to pretend otherwise

Per-game allocation went **16.88 → 16.73 MB/game, about 1%**. I predicted ~4%,
and the site arithmetic says ~4.4% should have been available.

**The two numbers do not reconcile and I cannot currently say why.** The honest
candidates: the two profiles cover very different durations (~1.5h versus
~6min), so their percentage shares have different denominators and are not
safely comparable at this precision; and a ~1% per-game difference is inside
the noise of comparing a handful of 30-second windows.

**So the claim is the site measurement, not the total.** r148's 17% was large
enough to clear that noise. This is not, and saying "r149 cut allocation 4%"
would be reporting arithmetic as though it were a measurement.

**Methodology limit for future rounds: whole-process per-game comparison does
not resolve a single site worth under ~5%.** To attribute a change that small,
compare two profiles of the same duration under the same workload, or do not
claim the aggregate at all.

Prod after: 562 g/min cum, turns/game 56.5, gc/game 0.06, heap 664–871MB,
0 aborts, 0 panics, no SLOW GAME.

## The layers system — investigated, deliberately not touched

`BaseCharacteristics` is the real top at **24.48%, 50.84GB cum**, and line 326
(`c := &Characteristics{`) alone is 27.75GB.

**It is not missing a cache.** `GetEffectiveCharacteristics` already has one —
`gs.charCache`, keyed by permanent pointer, epoch-guarded. It is invalidated
**globally** by `InvalidateCharacteristicsCache` on any counter add/remove,
modification append/expire, effect register/unregister, keyword stamp,
face-down toggle or attachment change. On a 100-permanent board that happens
constantly, so every permanent's characteristics get rebuilt over and over.

The real fix is splitting **base** invalidation (depends only on that
permanent's own state) from **layered** invalidation (depends on global state).
That is a correctness-critical redesign of the layers system — getting
invalidation wrong silently corrupts every characteristic in the engine — and
it is not something to start at the end of a round.

## One trap, recorded so the next attempt does not lose a round to it

**`cardColors` cannot be memoized by `*Card`.** It is 9.88GB under
`BaseCharacteristics` and looks like a textbook pure function of an immutable
card. `Card.Colors` is **reassigned at runtime** — `endure.go:66`,
`keywords_misc.go:1348`, `keywords_cr_aug_r64.go:73`, `keywords_p0.go:1055` —
so a cache keyed by the card pointer would serve stale colors into combat
protection and hexproof checks.

The right shape is a **WUBRG bitmask** instead of `map[string]struct{}`: all
seven call sites only range over it or test membership, so a bitmask allocates
nothing and needs no immutability assumption at all. Scoped, not started.

I checked that assumption instead of trusting it, which is the only reason this
is a note and not a bug.
