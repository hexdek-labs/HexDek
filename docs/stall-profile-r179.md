# r179 — what the stalls are actually made of

**Status:** measurement + one filed defect. No production code changed by
this document.

## The number that reframes the work

The r179 segment (32 min, prod pid 251413) spent **59% of its wall clock
held in a stall** — 19 of 32 minutes, **~5,564 games forgone**.

```
clean band      897.0 g/min  (n=20)
current hold    658.0        73.4% of clean   (one worker held: 75.0% predicted)
deep hold       426.0        47.5% of clean   (two workers held: 50.0% predicted)
```

The one-held / two-held fingerprint now holds for a **fifth and sixth**
time against a prediction made from first principles.

**This dwarfs everything else on the perf board.** Recent rounds shaved
1-11% off individual functions. Stalls are eating well over half the
machine. Any further micro-optimisation is rounding error next to this.

## Two independent stall profiles agree

CPU profiles captured live by the stall watcher at 13:48:56 (the deep
two-worker hold) and 14:11:59 (a later one-worker hold). Top leaves:

```
                              13:48:56      14:11:59
(*Permanent).hasType            5.88%         3.60%
Registry.triggerTableFor        5.87%         4.71%   (cum 9.09% / 7.33%)
runtime.mapaccess1_faststr      5.74%         5.35%
(*GameState).HasKeywordOf       4.43%         4.09%
strings.ToLower                 3.02%         3.13%
cardIsToken                     2.12%         3.41%
(*Card).DisplayName             2.61%            —
```

Cumulative at 13:48:56: `ActivateAbility` 51%, `FireCardTrigger` 38%,
`per_card.fireTrigger` 38%, `FirePermanentETBTriggers` 29%.

**The headline is that the top costs are not game logic.** They are
trigger-dispatch overhead: string-keyed map lookups, lowercasing, type
and keyword predicates. `DisplayName` is a bare field read, so its 2.6%
is not slowness — it is call count. **A stall is a dispatch storm.**

## FILED DEFECT — soulbond is O(n^2) over a token swarm

`FireSoulbondTriggers` (`keywords_batch2.go:188`) is an ETB observer:
`observer_triggers.go:185` calls it for **every creature that enters**.
Its loop (f) then walks **the controller's entire battlefield**, calling
`HasSoulbond` on each permanent.

So N creatures entering costs N battlefield scans — **quadratic in
exactly the token-swarm shape that produces these stalls**. Measured
cost: `HasSoulbond` cum **6.18s of 79.75s = 7.75%** of the deep-stall
profile.

`HasSoulbond` is not cheap per call: it routes through `keywordActive`
to `gs.HasKeywordOf`, the layer-aware query.

**Soulbond is a 2012 mechanic on a handful of cards. In the
overwhelming majority of games no permanent has it, and the entire scan
is pure loss.**

r172 already reordered the predicates in this loop and that reorder
stands — it reduced the constant. It did not and could not remove the
scan.

### Why this is filed and not fixed

The obvious fix is to skip loop (f) when the seat controls no soulbond
source. Doing that needs cached state, and **the cheap invalidation key
does not work**: `gs.charCacheEpoch` is bumped by
`InvalidateCharacteristicsCache`, which fires on any characteristics
mutation **including a permanent entering**. In a token swarm it bumps
on every ETB, so a memo keyed on it would hit 0% in precisely the case
that matters.

A correct fix therefore needs new tracked state (a per-seat count of
soulbond sources, maintained on enter / leave / keyword grant / keyword
loss / control change). That is an invalidation design with several
ways to be subtly wrong, and getting it wrong silently breaks a real
mechanic. **Owner call — high conviction on the finding, low on the fix
shape.**

The same shape is worth auditing across the other ETB observers in
`observer_triggers.go`: any per-ETB full-battlefield scan for a rare
keyword has this profile.

## Not claimed

- That r179 was supposed to reduce stalls. It fixed an unbounded
  retained allocation; stalls are a separate, older problem.
- That these two profiles characterise every stall. They are two
  samples that agree with each other and with the six previously
  profiled episodes.
- Any causal claim about the hang from these profiles.
