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
