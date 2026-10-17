# r155 — the charCache hits 98.1%, and my prediction was wrong by a factor of four

Round r155. Commit `2e4f79bb`.

## Item 1 checked, and closed

The brief's r155 item 1 was `BaseCharacteristics:326`, `c := &Characteristics{`
— 2.79GB of the r153 burst — with the instruction to *check whether it is
removable before planning a round on it*.

**It is not.** `GetEffectiveCharacteristics` stashes the struct in
`gs.charCache` and returns the pointer, so it is retained by both the cache
and the caller. There is no standalone round there.

## Which reframes the question, and the reframe was wrong too

There is a cache, and `BaseCharacteristics` is still the steady-state
allocation top at 25.59% cum / 18.51% flat. The obvious reading: the cache
must be missing nearly every time, because `InvalidateCharacteristicsCache`
bumps a **global** epoch and has **371 production call sites**.

I wrote that reading into the brief across several rounds. Before building on
it I published the counter — per-`GameState` plain `uint64` (a game is
single-goroutine, so the hot path pays no atomic), folded into process-wide
atomics at game end, mirroring `limited=` (r142) and `alloc=` (r146).

**Prediction, stated before measuring: a low hit rate, under 25%.**

```
chit=98.5% (112333179/114008501) cinval/game=435
chit=98.0% (87142500/88903128)   cinval/game=430
chit=98.1% (86371270/88046509)   cinval/game=424
chit=98.1% (94729712/96520649)   cinval/game=441
chit=98.3% (95872443/97569897)   cinval/game=455
chit=98.1% (90468727/92210184)   cinval/game=447
```

**98.1%.** Wrong by a factor of four, and stable to a tenth of a percent
across every window.

## What the numbers actually say

| | per 30s window | per game |
|---|---|---|
| `GetEffectiveCharacteristics` calls | ~90,000,000 | **~430,000** |
| misses | ~1,740,000 | **~8,300** |
| global invalidations | ~93,000 | **~440** |

Misses ÷ invalidations ≈ **19** — each epoch bump forces roughly nineteen
permanents to rebuild on next access.

So `BaseCharacteristics` dominates allocation **not because the cache fails**,
but because the call volume is enormous: 1.9% of 430,000 calls is still 8,300
full rebuilds per game.

## What this changes about the next round

The charCache split is still the right target, but for a different reason than
I would have built it for:

- **Not** "make the cache hit" — it already hits 98.1%.
- **Yes** "stop ~440 global epoch bumps per game from forcing ~8,300 base
  rebuilds". A per-permanent version would let the BASE half survive a bump
  caused by a *different* permanent.
- The addressable share is up to **18.51% of all allocation** — larger than
  r153's 9.23%.

And a second direction that did not exist before this measurement:
**430,000 characteristics lookups per game is itself a number worth
questioning**, independently of what each one costs.

Had I skipped this round, the split would have been designed against a hit
rate of ~25% that does not exist.

## Claims

**Pure instrumentation.** No behavioural regression can be red; reverting it
does not change a game, it removes a number. What is testable is the counting,
and that is pinned.

**Measured after deploying:** ~14.8 MB/game, 435.1 g/min cum, gc/game 0.05,
0 aborts, 0 slow games, 0 spikes. No throughput cost from two increments on a
90M-calls-per-window path, as expected.

Controls: counting a hit unconditionally — the obvious off-by-one — reads 100%
where the real counters read 50%; dropping the miss counter reds 4 of 4.
`-race` clean on the concurrent-publish test.

## Open

- **The charCache base/layered split**, now specified by real numbers rather
  than by a guess. Still correctness-critical; still write the invalidation
  matrix first.
- **Why 430,000 characteristics lookups per game?** New, unexamined.
- **`fireObserverETBTriggers:66`** — 404MB flat, defensive copy, scratch buffer
  plus a re-entrancy check.
- **The eager `LogEvent` Details maps** — 129MB + 139MB at `resolveCounterMod`.
