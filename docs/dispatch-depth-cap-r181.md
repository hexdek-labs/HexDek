# r181 — the trigger-depth cap is silently dropping ordinary triggers

`guards=percard_dispatch_depth_cap` has been printing on every grinder line
for rounds and nobody, me included, had asked what it means. It means
**triggered abilities that should have fired did not fire.** This sizes it.

## The mechanism

`per_card/registry.go:636`, in `fireTrigger`:

```go
gs.Flags["trigger_depth"]++
defer func() { gs.Flags["trigger_depth"]-- }()
gs.NoteTriggerDepth(gs.Flags["trigger_depth"])
if gs.Flags["trigger_depth"] > 8 {
    gameengine.LogLoopGuardFired(gs, "percard_dispatch_depth_cap", ...)
    gs.LogEvent(... "capped": "trigger_depth" ...)
    return          // <- every matching handler is skipped
}
```

The `return` is unconditional: no handler on any permanent runs for that
event. The logged `trigger_evaluated` marker exists so the
TriggerCompleteness invariant does not flag the hole — i.e. **the hole is
deliberately hidden from our own correctness checker.**

## It is not catching loops. A different guard does that.

From one live r180 window:

```
guards = percard_dispatch_depth_cap:1857, sba_max_passes:12,
         trigger_drain_cap:1, trigger_loop_cap:7
```

`trigger_loop_cap` is the per-turn runaway-loop guard and it fired **7**
times. The depth cap fired **1,857** in the same window — 265x more. Whatever
the depth cap is stopping, it is overwhelmingly not the thing we built a loop
guard for.

## The depth histogram says depth 9 is ordinary play

`tdepth` records the deepest nesting each game actually reached:

```
depth   1     2     3     4    5    6    7    8    9
games  4218  4065  1367  456  264  187  142  105   88
ratio              0.34  0.33 0.58 0.71 0.76 0.74 0.84
```

From depth 5 the tail is a clean geometric decay with ratio ~0.75. **Depth 9
sits exactly on that trend line, not above it.** That is the decisive shape:

- If the cap were catching runaway loops, depth 9 would be hugely
  over-represented — every runaway terminates at exactly the cap, so they
  would pile up there.
- If the cap sat safely above normal play, there would be a cliff before it.

Neither is present. The cap is slicing through the middle of an ordinary
distribution. (The 8→9 ratio of 0.84 running slightly above trend is expected
and is not evidence of piling: a game that would have reached depth 12 can
only ever *record* 9, so bucket 9 absorbs the whole truncated tail.)

Extrapolating the 0.75 decay, depths 10/11/12 would be roughly 66/50/37 games
in this window — real cascades, silently truncated.

## What is being dropped

```
guardtop = zone_change:547  permanent_ltb:430  creature_dies:404
           permanent_etb:112  graveyard_leave:109
```

**`creature_dies` swallowed 404 times** is the one with teeth. That is the
aristocrats family — Blood Artist, Zulaport Cutthroat, Bastion of
Remembrance. A board wipe kills ten creatures, the cascade nests past 8, and
the remaining death triggers silently do not happen. The game then reaches a
different result, and that result goes into ELO.

`zone_change` and `permanent_ltb` at the top is the same story one level up:
the deepest cascades are the ones that move many objects at once.

## Size

1,857 swallowed dispatches against 88 games that reached depth 9 — about
**21 dropped dispatches per affected game**, in roughly **2% of games**.

21 is the number that argues against "these are loops". A genuine infinite
loop would swallow thousands in a single game and drag the mean far above 21.

## What I have NOT established

- Whether any individual capped game is a real loop. These are aggregates; I
  cannot separate a deep-but-finite cascade from a loop per game without a
  per-game breakdown.
- What the correct cap is. 8 is clearly below the natural tail, but raising it
  is a liveness change and the loop guard it was presumably protecting against
  is already handled separately by `trigger_loop_cap`.
- Whether the four other guards (`sba_max_passes`, `trigger_drain_cap`,
  `game_deadline`) have the same problem. `sba_max_passes:12` and
  `trigger_drain_cap:1` are small enough to park.

## Proposed next step, owner call

Instrument before changing: record the per-game count of swallowed dispatches
and the depth distribution *above* the cap by letting the counter run while
still returning. That distinguishes "deep cascade" from "loop" with data
instead of argument, and costs nothing but a counter. Raising or removing the
cap without that is guessing at a liveness boundary.
