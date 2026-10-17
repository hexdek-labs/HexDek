# r163 — asking the depth question from the other side

## The measurement I planned cannot exist

r162 attributed `percard_dispatch_depth_cap`'s firings to their events.
Over 50 minutes: 17,046 firings, and 87% of them on five of the most
ordinary events in the game.

```
zone_change:5045  permanent_ltb:3913  creature_dies:3737
permanent_etb:1079  nonland_permanent_etb:1061
```

Not only the death path, as the first 235-game window suggested — the
ETB path too.

I wrote the next step into the brief as a histogram of how far past 8
the depth runs: a tight cluster at 9 meaning a cap set below ordinary
play, a run to hundreds meaning a real loop. **That histogram cannot
produce information.** `fireTrigger` increments, tests `> 8`, and
returns *without dispatching*, so nothing nested can run and the depth
observed at the cap site is always exactly 9.

Probed rather than reasoned, with a deliberately infinite recursive
handler driving the real dispatcher:

```
PROBE: guard fired 4 times, MAX depth recorded = 9
```

The instruction was mine, written one round earlier, and it was wrong.

## The question asked the other way round

Not *how far is the cap exceeded* — that is unanswerable and would be
uninformative if it weren't. The question is **what depth do ordinary
games reach**.

- If the distribution dies out at 3 or 4 and only a handful of games
  ever touch 9, the cap sits far above real play and its firings are
  real loops being correctly contained.
- If games routinely sit at 7 and 8, the cap is clipping ordinary
  cascades, and every one of those 17,046 firings is a triggered
  ability that silently did not happen.

So r163 publishes a **survival curve**: for each depth `d`, the number
of games that reached at least `d`. A game climbing to 7 passes through
1..7 and contributes one to each, which is what makes the curve
readable as "87% of games get this deep".

## Cost, and why it is a field and not a flag

`fireTrigger` is a hot path. The high-water mark lives in an unexported
**GameState field** — one inlineable int comparison per dispatch — and
the shared atomic is touched only when a game sets a *new* maximum, so
at most once per level, roughly nine times per game against ~430,000
characteristic lookups.

Putting it in `gs.Flags` would have cost a map hash on every dispatch
to save nothing, so it is deliberately not there. The consequence is
that `CloneForRollout`, which builds from an explicit field list, does
not copy it: a rollout clone starts at zero and may re-count levels its
parent already counted. The grinder reports `clones=0`, and a clone
re-counting a level it genuinely re-reached is not wrong for a survival
curve. Written down rather than discovered later.

`triggerDepthMax` bounds the histogram at 32. The cap is 8 today so
nothing above 9 is reachable, but this must not become an unbounded
array if that constant ever moves. Control C pins it.

## This is pure instrumentation

**The behaviour axis of this round cannot go red**, and saying so is
better than letting five green controls imply otherwise. No cap moved,
no dispatch changed, no rule changed. ELO comparable.

## Controls

5 controls, all red, each naming different tests.

- **A** — count every visit instead of the high-water mark: reds
  `ReClimbingTheSameGameDoesNotDoubleCount`. Without the mark, a game
  with many shallow cascades reads as a deep one.
- **B** — record only the peak, decrementing the previous level: reds
  six of the eight unit tests. The curve stops being a survival curve.
- **C** — remove the defensive upper bound: reds `DepthIsBounded`.
- **D** — delete the production call site in `fireTrigger`: reds
  `FireTriggerFeedsTheDepthCurve`.
- **E** — move the recording *after* the cap check so level 9 is never
  seen: reds `FireTriggerFeedsTheDepthCurve`.

**D and E are the ones worth noting.** My first attempt at D deleted
the production call site and every test stayed green — because the
unit tests in `internal/gameengine` call `NoteTriggerDepth` directly
and cannot see whether `fireTrigger` calls it. That control tested
nothing. The fix was not a better control but a missing test:
`TestR163_FireTriggerFeedsTheDepthCurve` drives the real dispatcher
under the real `EventLogNone` policy, and D and E both red against it.

**E's first attempt silently failed to apply** — its second needle
matched three places in the file, the assert threw, and the shell
printed `ok` from the unmodified run directly beneath the traceback.
The r152 trap again, caught only because the traceback was visible in
the same output. Re-anchored on a unique surrounding comment.

The wiring test also pins the probe's finding as a permanent assertion:
depth 10 must never be recorded, because `fireTrigger` returns at `>8`
without dispatching.

## r161 still unmeasured

Third tick running. Its prediction is the slow-game census — a game
tripping the ten-minute cap should finish near 600s rather than 1694s.
The r162 segment contains **zero** `game_deadline` firings and its
longest game is 30.9s, so the fixed path has never run.

r161 remains confirmed harmless and **not** confirmed effective.
