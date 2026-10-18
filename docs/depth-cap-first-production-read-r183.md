# The depth cap, measured in production — first read (r183)

r182 shipped per-game accounting for the dispatches that
`per_card/registry.go`'s trigger-depth cap REFUSES. It deployed
2026-10-18 20:14Z (binary `eecebb02d5b5f40b`, built from `f4c3e4d9`).
This is the first production readout. All figures below are from one
uninterrupted process, **10,357 games**.

```
guards=percard_dispatch_depth_cap:5532
tdepth=1:10365,2:9995,3:3566,4:1339,5:793,6:550,7:410,8:314,9:236
capswallow=games:236,max:334,cascademax:201,mixed:236,
           hist[1:236,2:236,4:200,8:154,16:93,32:33,64:14,128:8,256:3]
capfirst=creature_dies:154,life_lost:47,nonland_permanent_etb:17,
         permanent_ltb:5,zone_change:5,life_gained:3,graveyard_leave:2,
         permanent_etb:2,card_exiled:1
```

## CORRECTION TO docs/dispatch-depth-cap-r181.md

That document says the cap, "in most games, slices an ordinary
distribution and drops real triggers". **The first half of that is
wrong and I am retracting it.**

**236 of 10,357 games — 2.3% — hit the cap at all.** 97.7% of games
never reach depth 9. The depth survival curve makes the same point from
the other side: 10,365 games reach depth 1, 410 reach depth 7, 314 reach
depth 8, 236 reach depth 9. **The cap at 8 sits far above ordinary
play.** It is not slicing an ordinary distribution.

What misled me was `guards=percard_dispatch_depth_cap:5532`, a large
cumulative count that I read as broad. It is not broad, it is deep:
5,532 refusals concentrated into 236 games, a mean of 23 per affected
game. **A big aggregate over a small set of games looks exactly like a
small aggregate over many games until you measure the set.** That is the
same error shape as the SLOW GAME deck aggregation in r182, where "the
engine is slow" turned out to be one deck.

The second half of the r181 claim SURVIVES, and matters more now that it
is localised: inside those 236 games, 5,532 triggered abilities were
silently not dispatched. `creature_dies` is the first swallowed event in
154 of 236 games (65%), which is the aristocrats-after-a-wipe shape.
Those games produce wrong results and feed wrong ELO. **The harm is
concentrated, not diffuse — which makes it more fixable, not less real.**

## The discriminator: is it truncating a runaway, or slicing a wide board?

`cascademax` is the field that separates the two: a mutual trigger pair
re-enters without bound inside ONE top-level cascade, whereas a merely
wide cascade is bounded by board size.

| games  | affected | cascademax |
|-------:|---------:|-----------:|
|    484 |       14 |         29 |
|  2,339 |       55 |         91 |
|  5,794 |      138 |        150 |
| 10,357 |      236 |        201 |

**`cascademax` has not plateaued.** It has risen at every sample point,
and 201 swallows inside a single top-level cascade is already well above
any plausible board size (the heap-abort lines of comparable games print
`perms=44 tokens=6`).

**This is suggestive, not conclusive, and I am not going to overstate
it.** A maximum statistic rises with sample size even when the
underlying quantity is bounded; it only proves unboundedness if it never
stops. What can be said now is that no bound is in sight after 10,357
games, and that the shape is more consistent with re-entry than with
board width. **Re-measure this row every tick — a plateau would be the
finding, and would mean the opposite of what the trend currently hints.**

Note also that `max` (worst single GAME, 334) has been pinned since
~2,800 games while `cascademax` has kept climbing toward it. The worst
game's swallows are increasingly accounted for by ONE cascade rather
than many.

`mixed:236` — **every single affected game swallowed at least two
distinct event types; not one game was a single-event truncation.** That
is consistent with a cascade that crosses between death, life-loss and
ETB triggers, which is precisely the Bloodthirsty Conqueror /
Marauding Blight-Priest / Cruel Celebrant shape described in
`docs/stall-concentration-r182.md`.

## The two instruments cross-validate exactly

`tdepth`'s depth-9 bucket and `capswallow`'s game count are produced by
different code (`trigger_depth_hwm_r163.go` via a per-GameState
high-water mark; `depth_cap_swallow_r182.go` via a per-GameState
counter) from two call sites eight lines apart in `fireTrigger`. Every
game reaching depth 9 must swallow, so the two must be equal.

**They are equal on all 12 reporting windows** (14/14, 24/24, 34/34,
44/44, 55/55, 68/68, 78/78, 90/90, 98/98, 108/108, 122/122, 134/134).
That is an independent correctness check on r182's accounting, and it
passed.

(I briefly thought I had an 8-game discrepancy here. I had compared a
`tdepth` from a newer log line against a `capswallow` from an older one.
There is no discrepancy.)

## THE GAP THAT BLOCKS THE NEXT STEP

**We still cannot say which DECKS the 236 games were.** The cap counters
are process-global aggregates; the `grinder: SLOW GAME` line separately
carries `seed=` and `decks=[...]`. Nothing joins them.

Until they are joined, "the capped games are the Sorin deck" is a
hypothesis, not a measurement — the event mix is consistent with it, and
consistency is not attribution. Putting the per-game swallow count onto
the SLOW GAME line closes this, makes a capped game attributable to its
decks, and makes it re-runnable from its seed.

## What NOT to do yet

Do not move the cap. Raising it lets the re-entrant cascade run further;
lowering it makes more games wrong. **Depth is not the property that
distinguishes the two cases** — 201-in-one-cascade and 8-on-a-wide-board
are both "deep". The fix has to key on re-entry, not on depth. Get the
deck attribution first.

---

# AMENDMENT, same day — r184 deployed, and the first attributed game
# inverts the cascademax reading

r184 (`b84ed81f...`, pid 299664) put the per-game swallow count onto
the `SLOW GAME` line. The join works, and the first capped game it
named changes the picture.

```
SLOW GAME 67.1s turns=80 s/turn=0.8 seed=1792356558926485417
  decks=[winota_joiner_of_forces_b3 ozai_the_phoenix_king_b3
         olivia_opulent_outlaw_b3   nita_forum_conciliator_b3]
  swallowed=1920
```

**Two findings, and both cut against what this document said above.**

**1. The first attributable capped game is NOT the Sorin deck.** It is
**Winota**, the runner-up in the slow-game aggregation (12% of Oct-18
slow games). The Sorin hypothesis was built on the event mix being
*consistent* with the Conqueror / Blight-Priest pair, and the first
actual attribution landed elsewhere. **n=1 — this does not refute the
hypothesis, and one game is not a distribution.** But it is the first
piece of real attribution evidence we have, and it does not support it.

**2. The max / cascademax ratio has INVERTED, and that is the
discriminator.**

| process | max (worst GAME) | cascademax (worst CASCADE) | ratio |
|---|---:|---:|---:|
| r182 run, 10,357 games | 334 | 201 | 0.60 |
| r184 run, 3,507 games  | **1,920** | **58** | **0.03** |

In the r182 process one cascade accounted for most of the worst game's
swallows, which is what an unbounded re-entrant pair looks like. **In
the r184 process the worst game swallowed 1,920 dispatches while the
largest single cascade swallowed 58** — so those swallows are spread
over *at least* ~33 separate top-level cascades, and realistically far
more.

**That is not one runaway. That is the same moderate cascade recurring
hundreds of times across an 80-turn game** — the signature of a wide,
token-generating board producing many independent trigger cascades per
turn, which is exactly what Winota does.

**Retracting the hint, not the data.** Above, this document says
`cascademax` "has not plateaued" and that the shape "fits re-entry
better than width". The first sentence remains literally true of the
r182 series. **The inference I hung on it does not survive this
sample** — I was reading a rising maximum as evidence of unboundedness,
and the moment a game with 6x the swallows appeared, its largest
cascade was *three times smaller*. A rising max was sample-size
artefact, which is precisely the caveat stated and then under-weighted.

**What this means for the cap.** If the swallows are many small
cascades rather than one large one, then raising the cap does NOT
primarily risk letting a runaway run — it risks multiplying work across
hundreds of cascades. And lowering it clips more of an already-ordinary
distribution. **Still do not move it**, but the reason has changed: it
is not "the only brake on a runaway", it is a per-cascade limit being
asked to control a per-GAME volume problem. Those need different
instruments.

**Next:** accumulate joined lines until there are enough capped games
to rank DECKS by swallow volume (the r111 audit script was repaired
this round and now performs exactly this join). Do not generalise from
one game — including not generalising away from Sorin.
