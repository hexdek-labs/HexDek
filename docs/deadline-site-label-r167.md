# r167 — make the deadline say which site caught the game

## The verdict that could not be read

r166 moved the wall-clock deadline check inside `resolveCreateToken`'s
per-token loop. Two hours later, on 2026-10-17, the first qualifying
game finally arrived:

```
18:11:52   game starts (inferred: completion minus duration)
18:12:09   throughput collapses 482 -> 346 -> 260 g/min
18:12:55   first goroutine capture
18:18:45   sixth goroutine capture, identical
18:21:52   SLOW GAME 599.9s turns=52 s/turn=11.5
18:22:09   guards=game_deadline:1   (first window showing it)
```

The game was cut at **599.9s against a 600.0s cap**. The pre-fix
comparables ran 2199.1s, 1442.7s and 1365.6s.

And the log could not say which of the four check sites did the
cutting. `guards=` counts per guard, not per label. `guardtop=` renders
labels, but only the top five, and the five busiest pairs were per-card
dispatch counters in the thousands:

```
guardtop=percard_dispatch_depth_cap/zone_change:13228,
         percard_dispatch_depth_cap/permanent_ltb:9474,
         percard_dispatch_depth_cap/creature_dies:9129,
         percard_dispatch_depth_cap/permanent_etb:3696,
         percard_dispatch_depth_cap/graveyard_leave:3624
```

`game_deadline` fires **once per game**. It can never compete for a
top-5 slot against a counter that fires thousands of times an hour, so
the single most consequential firing in the log is the one guaranteed
to be invisible. The field built to show labels structurally cannot
show this one.

## The fix

`LoopGuardLabelsFor(guard)` renders **every** label for one named
guard, sorted heaviest first with ties broken by name. The grinder line
gains a ` deadlines=` field, empty until a game actually hits its cap:

```
guards=game_deadline:1 deadlines=token_create:1 guardtop=...
```

Costs a few bytes, on a ~400-byte line, only in the windows where a
game was capped.

Exact `guard + "/"` prefix, not a substring match: `Contains` would
also match a differently-named guard ending in this one, and would
match the label half of every pair.

## What the goroutine dumps already show

Twelve dumps over six minutes, taken by the read-only watcher, all
carry identical pointers — same `GameState 0x2629d5f5fb08`, same
`Permanent 0x2629dadc4908`, same `CreateToken 0x2629c7ce3400`, same
ability index:

```
ActivateAbility(gs, 0, perm, 1, nil)
 └ DrainStack
    └ ResolveStackTop → resolveActivatedAbility → ResolveEffect
       └ resolveCreateToken
          └ FirePermanentETBTriggers → fireTrigger → lookupCandidates
```

So **r166's path provably ran**: the worker was inside
`resolveCreateToken`'s loop, in a deadline-armed grinder game, for at
least 5m50s of a game that was then cut at its cap. The three
pre-existing sites all sit *above* `ResolveEffect` and none of them
could have been reached during that window.

What this still does **not** prove is which site did the cut. The
watcher exhausted its eight captures at 18:19, and the game ended at
18:21:52 — three unobserved minutes in which the resolution could in
principle have returned and been caught at `stack_drain` instead. The
pre-fix binary did produce one 599.3s cut (15:41, d=5), so ending near
600s is not by itself unique to r166.

That three-minute gap is the whole reason for this round. One field
closes it permanently, for this firing and every future one.

## Verification

Controls 3/3, semantics only:

| control | what it disables | tests red |
|---|---|---|
| A | the production call site in `showmatch.go` | 1 |
| B | exact prefix → substring match | 1 |
| C | truncate to the top 5, as `guardtop` does | 1 |

**Control C went green the first time, and that was a hole in the
tests, not a harmless control.** The cap is applied after the guard
filter, and every test until then gave `game_deadline` a single label,
so the cap had nothing to cut. `TestR167_DoesNotTruncateAGuardWithMany\
Labels` was written to close it — `percard_dispatch_depth_cap` already
carries at least five labels in production, so a cap would reintroduce
exactly the blindness this round removes. No unbounded-growth risk:
`bumpLoopGuardLabel` already bounds distinct-pair cardinality.

The wiring test lives in **hexapi**, not gameengine — r163's lesson,
where a control deleting the production call site left the whole gate
green. It asserts on the source rather than driving the code, because
the grinder line is emitted from inside a loop that cannot be invoked
in isolation. That is cruder than driving the code and is stated in the
test itself; its only job is to make control A go red, and it does.

## Scope

This is pure instrumentation. It changes no rules, no evaluation and no
decision path, so **it cannot be red on the behaviour axis** and no
claim is made that it can. ELO comparable.

It does not make r166 effective, and does not make it measured. It
makes the next capped game self-describing instead of requiring a
goroutine watcher to have been running at the right moment.
