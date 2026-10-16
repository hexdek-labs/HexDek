# r146 — the heap event is a rate event, the deck hypothesis is dead, and s/turn is the discriminator

## What happened since the 12:49 restart

Three more heap excursions. **All three were caught by the 6,442MB guard and the
process survived.** The 12:37 outage is therefore not unreproducible — it is a
recurring event that happened to win once.

| window | aborts | peak heap in an abort line |
|---|---|---|
| 13:23 | 320 | 7,195MB |
| 13:33–13:34 | 466 | 12,542MB |
| 14:12 | 13 | 6,963MB |

799 aborted games in total. **793 of them were at turn 10** — newborns. The guard
aborts whatever game checks in while the process heap is high; it does not abort
the game that consumed the memory. That is the collateral, not the cause.

## Three things that were not previously known

**1. `sys` is a ratchet.** 2,085MB → 13,206MB after the first excursion →
24,759MB after the second, and it has never come back down, on a 15.8GB box.
Each event permanently enlarges the reserved address space. `sys` drifting is
documented as benign; `sys` *stepping* is a scar and it counts the excursions.

**2. An abort burst looks like a throughput SPIKE, not a dip.** The 13:23:54
window read `861.7 g/min now` with `turns/game=22.0`. Aborted games complete
instantly and still count as completions. Any future reading of a high g/min
number has to check turns/game in the same window first.

**3. The Sorin deck is retired as the suspect.** It appeared at 14:15:58 at
`s/turn=0.2` and caused no excursion at all, and it is in none of the three
culprit games. Three excursions, three disjoint deck sets, no shared deck.

## s/turn separates them perfectly

Ten SLOW GAME lines since the marker:

- `s/turn` 22.0, 7.4, 4.5 — **three games, three abort bursts, same second.**
- `s/turn` 0.2, 0.2, 0.2, 0.2, 0.3, 0.3, 0.4 — **seven games, zero aborts.**

Zero false positives, zero false negatives, n=10, with an order of magnitude
between the two groups. Two previous predictors died on their false-positive
rate (gc/game at 17%, SAG at 21%); this one is being reported *with* the rate
rather than ahead of it. The honest caveat is n=3 on the positive side, and that
`s/turn` is only written at completion — it is a postmortem field today, not a
live one.

## The mechanism, with the right order of magnitude

High `s/turn` is what a game looks like when the hat is running rollouts.
`simulateRollout` and `multiRolloutForCard` each call `CloneForRollout` — a
318-line deep copy — and then run up to `rolloutDepth` **full simulated turns**
on the copy via `h.TurnRunner`. One real turn can therefore cost
`K candidates x n determinizations x depth` turns of allocation.

The clones are transient, not retained, so this is an allocation **rate**, which
is the shape the outage actually had (~300MB/s, not a leak). The observed
slowdown is ~110x normal `s/turn`; a multiplier of that size is the right order
of magnitude for the thing being explained.

**That is the test r145 said to apply, and it is why the previous reading was
wrong:** 118 token objects could never have been 18GB. This one can be.

Rollouts are bounded against cascades (`_rollout_trigger_cap = 200`) and do not
recurse — the inner hats are budget-0 heuristics. So the count of rollouts per
decision, not depth of recursion, is the term to measure.

## What r146 ships

Two numbers on the grinder line, the r142 pattern — publish a process-wide
value, never inspect another worker's state:

- **`alloc=NMB`** — `MemStats.TotalAlloc` delta'd per window. `heap` and `sys`
  are instantaneous samples taken every 30s, so a 35-second spike to 12GB can
  pass almost entirely *between* two of them. `TotalAlloc` is monotonic, so its
  delta is the bytes the window actually churned regardless of when GC ran.
  **This is the first number on the line that can see a rate event.**
- **`clones=N`** — `CloneForRollout` calls per window. Counted inside the
  function, after the nil guard, so a new caller cannot escape the census.

`clones` divided by `games` is the shape to watch.

## What would falsify this

If an excursion arrives with `clones/game` flat, rollout churn is the wrong
suspect and `alloc=` will still say where the bytes went. **The instrumentation
is designed to be able to kill its own hypothesis** — that is the point of
shipping it instead of asserting the mechanism.

## Still blocked, still the #1 item

A memory-capped place to run the repro: a non-prod host, a cgroup on .206, or
root to make one. `systemd-run --user` has no bus there and `ulimit -v` is
unreliable against Go's address-space reservation. **Deliberately running a
known-OOM workload on the live box is not covered by standing deploy authority.**
r146 is the thing that can be done without it — but prod now tells us when an
excursion happens and how many bytes moved, which is strictly better than the
postmortem we had.

The three infrastructure asks are unchanged and now have four events behind them
rather than one: `run-server.sh` is a `while true` inside the process group that
dies; there is no kernel log access, so an OOM kill still cannot be confirmed;
and there is no liveness monitoring.
