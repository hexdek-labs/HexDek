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

---

# SECOND AMENDMENT — the r184 join has a 99.3% sampling hole, and
# capped games are mostly NOT slow games

First full tick of r184 data, 13,509 games in one process.

```
13,509  games in the process
   280  hit the depth cap            (capswallow games:280 — 2.07%)
     8  crossed slowGameThreshold    (10s) and emitted a SLOW GAME line
     2  of those 8 had swallowed > 0
```

**278 of 280 capped games — 99.3% — are invisible to the join I shipped
last round.** `swallowed=` rides on the `SLOW GAME` line, which only
fires above a 10-second wall-clock threshold (`showmatch.go:1053`), and
capped games overwhelmingly finish faster than that.

This is the standing rule turned on its author: **a detector that
cannot see the cases you already know exist is not yet a detector.**
r184 was correct and useful; it simply answers the wrong sampling
question, and I did not notice because the one game it did catch was
spectacular enough (`swallowed=1920`) to look like a result.

## THE FINDING UNDERNEATH IT, WHICH IS BIGGER THAN THE BUG

**Depth-cap pressure and slow games are two different populations, and
this document has been treating them as one.** The cap lives in the
stall investigation because that is where it was found. The data says
that was a filing error:

- **Capped games are fast.** 99.3% complete inside ten seconds while
  swallowing dispatches. Whatever the cap is doing, it is not what
  makes games slow.
- **Slow games are mostly not capped.** 6 of the 8 slow games this
  process reported `swallowed=0`.

## AND IT WEAKENS THE SORIN CAUSAL STORY FURTHER

Of the 8 joined slow games, **Sorin
(`sorin_of_house_markov // sorin_ravenous_neonate`) was at the table in
6 — and swallowed ZERO in five of them.** The single Sorin game that
was capped swallowed 14, against Winota's 1,920.

**n=6 is not a result and this is not a refutation.** But the direction
is consistent and worth stating plainly: **Sorin makes games SLOW
without hitting the depth cap.** Slow-game frequency (where Sorin
dominates at ~39% of Oct-18 slow games, 76/196) and cap pressure (where
it so far does not appear) are measuring different things. They were
conflated because one deck topped the only ranking available.

## RATIO TRACKING (the discriminator, per the standing instruction)

| process / sample | max (worst GAME) | cascademax | ratio |
|---|---:|---:|---:|
| r182 run, 10,357 games |   334 | 201 | 0.60 |
| r184 run,  3,507 games | 1,920 |  58 | 0.03 |
| r184 run, 13,509 games | 1,920 | **147** | 0.077 |

**`cascademax` rose 58 → 147 inside one process while `max` did not
move.** That is the third independent demonstration that `cascademax`
is itself a maximum statistic climbing with sample size — the exact
property whose caveat I wrote and then leaned against last round. The
ratio remains an order of magnitude below the r182 run's, so the
"many moderate cascades, not one runaway" reading stands for now.

## NEXT

Attribute capped games independently of duration, so the ranking covers
280 games instead of 2. Until that lands, **no deck ranking from this
join should be quoted** — it is a 0.7% sample selected on a variable
(duration) that we now know is nearly uncorrelated with the thing being
measured, which is the worst kind of sample to generalise from.

---

# THIRD AMENDMENT — r185 settles it: IT IS ONE RUNAWAY CASCADE.
# My second amendment was wrong, and so was the ratio I retracted it with.

r185 (`cd8f2693`, pid 304711) emits a `CAPPED GAME` line for every
capped game regardless of duration, carrying a **per-game** cascademax.
First 48 attributed games:

```
capped games attributed                                48
duration: median 0.30s   p90 0.70s   max 1.40s   >=10s: ZERO
swallowed: median 10, max 537

games where ONE cascade accounts for ALL swallows:  44 of 48  (92%)
  ratio 1.00  44
  >=0.75       1
  >=0.50       3
  <0.50        0
among the 14 games with >=16 swallows: 13 of 14 at ratio 1.00
```

**The worst game: `swallowed=537, cascademax=537`. Every single
swallow in that game happened inside ONE top-level cascade.**

## I HAVE FLIPPED TWICE AND THIS IS WHY

The first amendment above said the shape fitted unbounded re-entry. The
second retracted it, on the strength of this:

```
r184 run: max 1920 (worst GAME)   cascademax 58 (worst CASCADE)   ratio 0.03
```

and concluded "many moderate cascades, not one runaway".

**That ratio is meaningless, and I should have caught it when I wrote
it.** `max` and `cascademax` are two INDEPENDENT process-wide maxima.
Nothing makes them come from the same game. Dividing the worst game's
total by some *other* game's worst cascade produces a number with no
referent. I noticed the concern in passing this tick — "the ratio
compares two maxima that may come from different games" — and had
already built the conclusion on it a tick earlier.

**The per-game measurement is the only one that could ever have
answered this**, and it says the opposite of the aggregate: the
swallows are not spread across many cascades, they are 92% of the time
*entirely inside one*.

**So the original reading was right, the retraction was wrong, and
both were produced by the same underlying error — reasoning about a
per-game property from process-wide aggregates.** The honest lesson is
not "I was right all along"; it is that neither the first claim nor its
retraction was entitled to be made from the data available at the time.

## WHAT IS NOW ESTABLISHED

1. **Capped games are a single runaway cascade, truncated.** 92% ratio
   1.00; 13 of 14 among the heavy games.
2. **Capped games are FAST.** Median 0.30s, max 1.40s, **none** reached
   the 10s slow-game threshold. The cap is doing its job quickly. This
   is why the r184 join saw 2 of 280.
3. **Depth-cap pressure and slow games remain disjoint populations.**
   Reconfirmed, now from the cap side with a complete sample.
4. **Sorin is in ZERO of the 48 capped games.** Not "under-represented"
   — absent. Sorin is a SLOWNESS phenomenon and has no measured
   relationship to the depth cap.

## THE DECKS (first complete attribution, n=48, still small)

| by swallow volume | swallows | games |
|---|---:|---:|
| marrow_gnawer_b3 | 604 | 3 |
| mayael_the_anima_b3 | 543 | 2 |
| avacyn_angel_of_hope_b4 | 537 | 1 |
| urza_chief_artificer_b4 | 537 | 1 |
| krenko_mob_boss_b4 | 449 | 3 |

**Marrow Gnawer (Rat tokens) and Krenko (Goblin tokens) lead on both
count and volume.** Consistent with a token swarm feeding an ETB/dies
cascade that re-enters. **n=48 — do not harden this into a suspect list
yet; one deck appearing beside another at a four-player table is not
attribution.**

## CONSEQUENCE FOR THE CAP

The earlier conclusion — "a per-cascade limit being asked to control a
per-game volume problem" — **is withdrawn.** It is a per-cascade limit
facing a per-cascade problem, which is the case where the instrument
and the pathology actually match. Raising it genuinely does risk
letting a runaway run. **Still do not move it**, but the target is now
clear: find what re-enters inside a single cascade on a token board.

---

# FOURTH AMENDMENT — n=701. Three of my own n=48 numbers were wrong,
# and the pick-frequency control finally exists.

14.6x the sample. **Every correction below is to a claim I made last
tick from n=48.**

```
capped games attributed                               701
ratio EXACTLY 1.00 (all swallows in ONE cascade)      565/701 = 81%
  ...among the 244 games with >=16 swallows           156/244 = 64%
  ratio <0.50                                          79 games
duration: median 0.30s  p90 0.60s  MAX 218.6s   >=10s: 1
swallowed: median 11  p90 41  max 3128
Sorin at the table                                    3 of 701
```

## CORRECTION 1 — "92% single cascade" was an n=48 overstatement

It is **81%** overall, and **64% among the heavy games** — where at
n=48 I reported 13 of 14 (93%). **The heavy games moved the FURTHEST
in the wrong direction**, which is the opposite of what the small
sample implied, and they are the games that matter most.

79 games put less than half their swallows in their biggest cascade.
**So both mechanisms are real:** a single re-entrant cascade is the
majority case, but roughly a third of heavy games genuinely spread
swallows across many cascades. The correct statement is "predominantly
one runaway cascade", not "it is one runaway cascade".

## CORRECTION 2 — "ZERO capped games reached 10s" is now false

**One game ran 218.6 seconds** with the cap firing. At n=48 the max was
1.4s and I said none reached the slow threshold; I should have written
"none in 48", because an absence in a small sample of a long-tailed
distribution is the least durable claim available. Median 0.30s and p90
0.60s still hold — **capped games are overwhelmingly fast, with a tail
that does reach the stall population.** The two populations are mostly
disjoint, not strictly disjoint.

## CORRECTION 3 — "Sorin is in ZERO of the capped games" is false

**3 of 701.** Against ~2 expected for an average deck, that is
unremarkable — so the *conclusion* (Sorin has no special relationship
to the depth cap) survives intact. But I stated "absent, not
under-represented" in bold off a sample of 48, and absence at n=48 was
never evidence of absence.

## THE CONTROL I OWED — observed vs expected from pick frequency

Deck selection is **not** uniform: `matchmaking.AssembleBracketPod`
weights by TrueSkill proximity, info-gain and bracket, so "expected =
4/poolsize" would have been wrong. The real denominator is each deck's
lifetime game count from `/api/live/elo` (1,425 decks).

**Join fix worth recording: `deck_id` is NOT unique in that table** —
`imported_deck` appears 4 times, plus 2 more collisions. Keying on
`deck_id` alone silently merged different owners' decks. `owner +
deck_id` is unique; all 1,090 observed decks then matched.

| deck | capped games | expected | **rate** | swallows |
|---|---:|---:|---:|---:|
| gerrard_weatherlight_hero_b4 | 20 | 1.98 | **10.1x** | 1,968 |
| krenko_mob_boss_b4 | 16 | 1.98 | **8.1x** | 918 |
| elas_il_kor_sadistic_pilgrim_b4 | 14 | 1.98 | 7.1x | 466 |
| kess_dissident_mage_b4 | 12 | 1.97 | 6.1x | 185 |
| marrow_gnawer_b3 | 11 | 1.98 | 5.5x | 815 |

By **volume** rather than rate: winota 4,396 (6 games), queen_marchesa
3,192 (7), **yarok_the_desecrated 3,173 (5)**, witherbloom 3,134 (2).

## THE MECHANISM, FROM VERIFIED ORACLE TEXT

Two names in those lists are not generic token decks — they are
cascade amplifiers, and both oracle texts were fetched this session:

- **Yarok, the Desecrated** — *"If a permanent entering causes a
  triggered ability of a permanent you control to trigger, that
  ability triggers an additional time."* **A literal ETB-trigger
  doubler**, i.e. a multiplier on exactly the quantity the cap
  measures. 3,173 swallows in 5 games.
- **Gerrard, Weatherlight Hero** (the #1 by rate, 10.1x) — *"When
  Gerrard dies, exile it and return to the battlefield all artifact and
  creature cards in your graveyard that were put there from the
  battlefield this turn."* **A dies-trigger that causes a mass
  simultaneous ETB wave** — death feeding entry feeding death.

That is a far more specific hypothesis than "token decks": the
population is enriched for **cards that multiply or re-enter the
ETB/dies loop**, which is consistent with `capfirst=creature_dies` at
64% of games.

**n=701 with ~2 expected per deck is still thin per-deck, and a rate of
10.1x on 20 observed games has a wide interval.** Treat the mechanism
as the finding and the individual names as leads.

## NEXT

Instrument which EVENT recurs *within* one cascade, not just which
event the cap first fires on. That distinguishes "Yarok doubled an ETB
wave" from "a dies/ETB pair is re-entering", and it is the last
question the current fields cannot answer.
