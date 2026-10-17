# r156 — the characteristics cache slot moves onto the Permanent

Round r156. Commit `d322969d`.

## The round r155 pointed at was not the round the profile pointed at

r155 ended saying the next item was the base/layered charCache split. Before
building it I took a 30-second CPU profile of prod — the same "measure the
premise" move that made r155 worth doing.

```
GetEffectiveCharacteristics = 8.77% of all CPU

CALLERS                        INSIDE
IsCreatureOf       60.00%      BaseCharacteristics        35.62%
HasTypeOf          16.57%      runtime.mapaccess2_fast64  32.76%
PowerOf             8.95%      applyLayer                 15.43%
HasSupertypeOf      4.38%
HasKeywordOf        4.00%
HasLostAllAbilities 3.24%
ToughnessOf         1.90%
```

**A third of the cost is the cache lookup itself** — on the 98.1%-hit path,
paid on essentially every one of ~430,000 calls per game. The base/layered
split does not touch that; it addresses the 1.9% that miss.

Also worth recording for later: **60% of all calls come from `IsCreatureOf`**,
a predicate that wants one boolean and gets the whole characteristics struct.

## The change

The Permanent pointer **is** the identity here — `Permanent` has no ID field —
so `map[*Permanent]entry` and a field on the Permanent are the same key, one
reached by a hash and one by a dereference. Moved it.

Three things fall out:

1. The map probe becomes a field compare.
2. `InvalidateCharacteristicsCache` no longer clears anything — removing a
   `clear()` from ~440 invalidations per game.
3. **r73's retention bug becomes structurally impossible rather than fixed.**
   r73 added the `clear()` because the map key kept every permanent it ever
   saw reachable for the life of the GameState. With the entry on the
   permanent, the GameState holds no reference at all.

## The risk the map did not have — found by probe, before shipping

A map lives on one GameState and cannot serve another's entry. A field on the
Permanent can. I tested it rather than reasoning about it:

```
two GameStates, same permanent, same epoch
gsA.PowerOf = 5 (correct)   gsB.PowerOf = 5 (WRONG, want 2)
COLLISION CONFIRMED: gsB served gsA's cached entry
```

Nothing shares a permanent today — `Clone` builds each one from an explicit
field literal and remaps every pointer through `permMap`, and the only `*perm`
value copy in the file is a `ZoneCastPermission`. But *"nothing does today"* is
not a safety property, and this is the cache whose corruption is silent: a
wrong power, type or keyword with no error anywhere.

So the slot carries a **GameState identity** as well, assigned lazily so that a
bare `&GameState{}` literal (which tests and some callers build) gets a
distinct one instead of sharing the zero value.

## r73's regressions are ported, not deleted

Its retention assertion counted map entries, which no longer exist. It becomes
a **derived gate**: no field on `GameState` may be a map keyed by `*Permanent`.
That pins the r73 property deterministically and forever, and goes red the day
someone reintroduces the shape.

## A comment I had to correct

I wrote that the nil check on `charChars` was load-bearing for the cold read —
a fresh GameState at epoch 0 meeting a fresh Permanent at charEpoch 0. **A
control that removed it stayed green.** `charGS` already rejects that case,
because a fresh permanent is at charGS 0 and an identity is never 0. The check
stays as one pointer compare of defence; the comment now says what it actually
buys. Same shape as r150's control D.

## Prediction and result

**Stated before measuring:** a CPU/throughput win of ~2–4%, and a small
allocation change (~1–3%) that may not clear the resolution limit.

```
                   n    alloc median    alloc mean    g/min median
r155             172         15.11          15.26           418.1
r156              27         15.06          15.25           432.1
```

**Throughput +3.3% median / +3.2% mean — inside the predicted band.
Allocation unchanged**, which is the half of the prediction that said it might
not resolve. 0 aborts, 0 slow games, 0 spikes.

**Provisional.** n=27 against n=172 is a small sample, and short samples have
misled me twice today — including last tick's "r155 ≈ 14.8 MB/game", which over
the full 172-window segment is **15.11**. Re-measure next tick.

## Open

- **`IsCreatureOf` is 60% of all characteristics calls.** A predicate that
  needs one boolean pays for the whole struct. Unexamined.
- **The base/layered split** — still real, still correctness-critical, but now
  known to address the 1.9% miss path rather than the lookup cost.
- **`fireObserverETBTriggers:66`** — 404MB flat, defensive copy, scratch buffer
  plus a re-entrancy check.
- **The eager `LogEvent` Details maps** — 129MB + 139MB at `resolveCounterMod`.
