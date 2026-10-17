# r162 — attributing a loop-guard firing to what it fired on

## Why

r160 made the nine loop guards countable. The first hour of data did
not say what r160 went looking for:

```
percard_dispatch_depth_cap:  30,201
sba_max_passes:                  88
trigger_loop_cap:                72
trigger_drain_cap:               24
game_deadline:                    2
```

The guard I had chased for two rounds fired twice. One I had never
looked at fired thirty thousand times — over 33,192 games, median 0.38
per game, max 8.56, nonzero in 178 of 188 windows. It is continuous,
not episodic, and no long game is needed to provoke it.

`percard_dispatch_depth_cap` fires in `fireTrigger`
(`per_card/registry.go:619`) when `gs.Flags["trigger_depth"]` exceeds 8,
and on firing it **returns without dispatching any handler for that
event**. Every firing is a per-card triggered ability that never
reached the stack. That is a rules-accuracy defect if the chains being
cut are legitimate, and correct containment if they are not.

## What a total cannot tell you

"Genuine runaway, correctly contained" and "ordinary trigger chain,
silently truncated" produce the same number. They are distinguished by
the **event being dispatched**: a `creature_etb` appearing in most
games means the cap is set below ordinary play; a single exotic event
dominating means a real loop in one card.

That event is already passed to `LogLoopGuardFired` in its details map
— and already discarded with the rest of the event, because the
grinder runs `EventLogNone`. This is r160's finding one level down: the
counter survived, the detail did not.

So r162 counts `(guard, label)` pairs beside r160's `(guard)` counts.
No cap is changed, no dispatch behaviour is touched. **Measure first.**

## Shape

`guards=` is deliberately untouched — it stays a small, stable,
always-parseable column. The attribution rides in its own field,
`guardtop=`, showing the five busiest pairs, heaviest first, ties
broken by name so the output is stable across runs. Both fields are
empty when nothing has fired, so the ordinary grinder line is
unchanged.

The label is drawn from an **allow-list** of details keys —
`event`, then `handler` — not from whatever the map happens to hold.
Keys carrying a depth, a count, a seat or a timestamp are excluded on
purpose: they are unbounded or near-unbounded, and a label like
`depth:9` attributes nothing. Control C pins that by letting a numeric
key through and watching two tests fail.

### Cardinality is bounded on purpose

These labels come from details maps written at twelve call sites and
nothing structurally stops a future one passing something unbounded.
Past 64 distinct labels for a guard, further new labels collapse into a
single `~other` bucket, so neither the map nor the log line can grow
without limit, and no firing is lost — the overflow bucket still
counts. **`~other` appearing at all is the signal that the allow-list
picked a key it should not have.** The bound being visible in the
output is the point.

The bound check is racy under concurrency and that is deliberate: it is
a safety ceiling, not a quota, and overshooting it by the number of
live workers cannot matter.

## This is pure instrumentation

**The behaviour axis of this round cannot go red, and I would rather
say so than let four green controls imply otherwise.** No cap moved, no
dispatch changed, no rule changed. The controls below test the
accounting, not the engine. ELO comparable.

## Controls

4 controls, all red, each naming different tests. 5 of 8 tests went red
across them; the premise test, the empty-output test and the
concurrency test never red, by design.

- **A** — don't attribute at all (r161 behaviour): reds
  `TheEventNameSurvivesTheDroppedEvent`, `HandlerIsAttributedToo`,
  `CardinalityIsBounded`.
- **B** — remove the cardinality bound: reds `CardinalityIsBounded`
  (the map grew to 104 entries against a bound of 64).
- **C** — let a numeric details key become a label: reds
  `TheEventNameSurvivesTheDroppedEvent` (attributed to `depth`) and
  `UnlabelledGuardIsCountedButNotAttributed` (attributed to `passes`).
- **D** — sort the summary by name instead of by count: reds
  `TopIsSortedByCountThenName`.

`TestR162_EventLogNoneStillDropsTheDetails` pins the premise of the
round as its own test — if `EventLogNone` ever starts retaining
events, this round is solving a problem that no longer exists and that
test says so. `TestR162_ConcurrentFiringIsSafe` runs under `-race`,
because two grinder workers fire these concurrently.

## r161, measured — and not measured

r161's own axis is the SLOW GAME census, and the segment available at
the time of writing **did not exercise it**: thirteen minutes, one slow
game of 19.8s, and `game_deadline` absent from the guards string
entirely. No game tripped the cap, so the fix did not run.

```
                     n    g/min median   MB/game median
r160 clean bands    74       447.1           15.01
r161 (partial)      34       453.1           14.81
```

Throughput and allocation are fine and slightly favourable, which is
the secondary axis. **r161 is so far confirmed harmless and not
confirmed effective** — the same distinction r159 earned, and it stands
until a game trips the deadline and finishes near 600s instead of at
1694s.
