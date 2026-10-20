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

---

# FIFTH AMENDMENT — r186 separates WIDE from RE-ENTRANT, and the answer
# is BOTH, in a ratio that explains every earlier flip-flop.

r186 (`401a6e2f`, pid 313383) classifies each top-level cascade by two
axes: how many DISTINCT events were swallowed, and whether the swallow
count exceeds the board's permanent high-water mark.

```
                  n <= boardMax        n > boardMax
 1 distinct       wave                 loop1
 2+ distinct      widemix              loopN
```

A wave of N permanents each firing once **cannot exceed the permanents
in play**; re-entry has no such ceiling. So `n <= boardMax` is a
reliable NEGATIVE — `wave`/`widemix` are trustworthy, `loop*` is a
loop-hunt candidate rather than a verdict.

## FIRST READ — n=88 attributed games

| shape | games | | swallows | | per game |
|---|---:|---:|---:|---:|---:|
| **widemix** (wide, multi-event, board-bounded) | 79 | 90% | 1,441 | 59% | median 12 |
| **loopN** (genuine re-entry, n > board) | 8 | 9% | 984 | 41% | median 133 |
| wave | 1 | 1% | 1 | 0% | 1 |

**Both mechanisms are real, and they are cleanly separable: 9% of
capped games are runaways and produce 41% of all swallowed dispatches;
90% are ordinary-but-wide boards and produce 59%.** Loop games swallow
~11x more each (median 133 vs 12) on bigger boards (median 56 vs 36).

## THIS RECONCILES EVERY EARLIER READING, INCLUDING THE RETRACTED ONES

- "It is one runaway cascade" (per-game ratio 1.00 at 81%) — **true but
  incomplete.** The swallows DO concentrate in one cascade. That
  cascade is usually **wide, not deep.** One cascade is not the same
  claim as one loop, and I treated them as the same claim.
- "Many moderate cascades, not one runaway" — wrong as stated, but it
  was reaching for the real majority case: wide boards.
- "It slices an ordinary distribution" (r181, which I retracted in the
  first amendment above) — **substantially right for 90% of capped
  games**, and I retracted it on a denominator argument that was itself
  correct. Both can hold: the cap fires in only 2.3% of games, AND
  within those it is mostly clipping wide ordinary boards.

**The pattern across five amendments: every reading was a projection of
a two-population mixture onto one axis.** Each axis I had — depth,
cascade count, game count, duration — made one population invisible.
The two-axis classifier is the first instrument that does not force a
choice, and it says the answer was never one of the options.

## CONSEQUENCE FOR THE CAP — the r181 claim, earned properly this time

**Raising the cap helps the 90% and hurts the 9%. Lowering it does the
reverse.** That is exactly "wrong instrument for both jobs", which r181
asserted and I retracted twice. It is now measured rather than
asserted: the two populations want opposite changes to the same
constant, so **no value of that constant is correct** and the fix has
to be a different mechanism — detect re-entry directly rather than
inferring it from depth.

## CAVEATS, STATED BEFORE THEY BREAK

**n=88 games.** Last tick three of my n=48 numbers broke at n=701, and
the extremes broke first. **Do not quote the 90/9 split as settled** —
re-measure every tick and expect it to move, especially the 9%.

`namecap:41` — 41 cascades overflowed the 4-name table, so `distinct`
is a lower bound there. That cannot flip a `widemix` verdict (more
names keeps it widemix) but it can hide a `loop1`.

**Known blind spot, from the agent and verified in its test:** a
strictly alternating two-event cycle is reported as `distinct=1`,
because every depth-9 attempt descends from depth-8 nodes of uniform
parity, so **only one of the two names is ever swallowed.** A textbook
Gerrard/Yarok mutual pair therefore reads as `loop1`, not `loopN`.
**(a) self-feeding and (b) mutual-pair are NOT distinguishable at the
cap site** — this contradicts my brief, which had predicted the
(a)/(c) pair would be the problem. Separating them needs the nesting
path, which means a push/pop on the hot uncapped dispatch path.

---

# SIXTH AMENDMENT — n=347. There are THREE regimes, not two, and my
# own volume split was a unit error.

## FIRST, THE ERROR, BECAUSE IT IS MINE AND IT IS THE SAME ONE TWICE

Last amendment I published "widemix 59% of swallows / loopN 41%", and
this tick's larger sample first gave me "82% / 18%". **Both numbers are
invalid.** On the `CAPPED GAME` line, `shape`/`distinct`/`dom`/`board`
describe the **PEAK CASCADE**, while `swallowed` is the **GAME TOTAL**.
Summing game totals and attributing them to the peak cascade's shape
compares two different units.

I caught it only because a row looked impossible: `swallowed=3122
board=32`, which by the classifier's own rule (`n > boardMax` → loop)
should not have been `widemix`. It was not a classifier bug — the
classifier is internally consistent on **347 of 347 rows**. That game's
peak cascade swallowed **7**, not 3,122.

**This is the error I wrote a memory about two amendments ago** (never
combine two numbers that describe different units) and then committed
again in my own analysis script. Noticing the rule is not the same as
applying it.

**What can honestly be said about volume: nothing per-shape.** The
per-game line cannot attribute swallows to cascade shapes, and the
process-wide `capshape=` histogram counts CASCADES without volume.
By cascade count: **widemix 1,412 / loopN 25 / wave 3** — 98% wide.

## THE THREE REGIMES (n=347 games)

| regime | definition | games | median swallows | median duration | max duration |
|---|---|---:|---:|---:|---:|
| **1 — one wide cascade** | `cascademax == swallowed` | 285 (82%) | 11 | 0.30s | 1.3s |
| **2 — re-entry** | `loopN` (n > boardMax) | 25 (7%) | 77 | **0.20s** | **0.7s** |
| **3 — fan-out** | >=20 separate cascades | 3 (1%) | **2,157** | **89.7s** | **203.3s** |

**The result that inverts an assumption I never examined: regime 2 —
the genuine runaways — are the FASTEST games in the entire capped
population.** Median 0.20s, max 0.7s. The cap catches re-entry
immediately and cheaply. **A runaway costs CORRECTNESS, not time.**

**All the time is in regime 3**, which is neither a wide cascade nor a
loop: it is **hundreds of independent small cascades across a long
game**. The worst had `swallowed=3122, cascademax=7` — at least **446
separate top-level cascades**, 80 turns, 203 seconds.

**Regime 3 is where the cap population and the stall population
meet.** Both of the only two capped games over 10 seconds are regime 3.
Earlier amendments called the two populations "mostly disjoint" without
being able to describe the overlap; this is the overlap.

## THE MECHANISM FOR REGIME 3 — one deck, in 3 of 3

**`winota_joiner_of_forces_b3_kaputtzimon` is at the table in all three
regime-3 games**, including both 10s+ games, and is separately 14% of
Oct-18 slow games (31/222) and the top deck by raw swallow volume.

Oracle text, fetched this session:

> **Winota, Joiner of Forces** — *"Whenever a non-Human creature you
> control attacks, look at the top six cards of your library. You may
> put a Human creature card from among them onto the battlefield tapped
> and attacking..."*

**That is one INDEPENDENT trigger per attacking creature, each putting
a permanent onto the battlefield.** A wide board of non-Human attackers
produces not one deep cascade but **one separate top-level cascade per
attacker** — which is regime 3's signature exactly, and explains
`cascademax=7` alongside `swallowed=3122`.

So the three regimes have three different mechanisms:
- **1, wide:** many permanents responding to one event.
- **2, re-entry:** a cascade feeding itself (the Yarok/Gerrard shape).
- **3, fan-out:** many independent cascades, one per attacker.

**n=3 for regime 3.** Winota in 3 of 3 with a mechanism that predicts
the signature is a strong lead, not a verdict.

## CONSEQUENCES

- **The cap still cannot be tuned**, and regime 3 makes it worse: that
  regime is not controlled by a per-cascade depth limit at all, since
  each of its 446 cascades is individually shallow.
- **Regime 2 is the cheapest to leave alone and the most expensive to
  get wrong.** It is fast precisely because the cap truncates it; the
  damage is silently dropped triggers.
- `namecap:928` of ~1,440 cascades — **64% overflowed the 4-name
  table**, so `distinct` is a saturated lower bound in two thirds of
  cascades and `dom%` is unreliable there. The table is undersized for
  the real width; that is a measurement limit to fix before leaning on
  `distinct`.

---

# SEVENTH AMENDMENT — r187: cascades COUNTED, not divided; and a
# contamination in every `distinct=` number published so far.

r187 (`9ce95c01`, pid 322631) adds a per-game **count** of cascades
that swallowed anything, and widens the distinct-event name table from
4 slots to 32.

## THE INFERENCE I HAD BEEN USING WAS WRONG BY 2x

Previous amendments estimated cascades per game as
`swallowed / cascademax`, described as a lower bound. First counted
data, n=91 games:

```
counted=69   my inferred bound >=34    swallowed=234  cascademax=7
counted= 9   inferred >=5              swallowed=35   cascademax=7
counted= 9   inferred >=7              swallowed=21   cascademax=3
```

**The bound was honest but weak — the worst game had twice as many
cascades as the division implied**, because the division assumes every
cascade ran at the maximum and real cascades vary. The regime-3 "at
least 446 cascades" figure in the sixth amendment should be read as a
floor that is probably far below the truth.

## THE REGIMES HOLD AT n=91 WITH COUNTED DATA

| regime | definition | n=91 | (n=347 inferred) |
|---|---|---:|---:|
| 1 — one swallowing cascade | `cascades == 1` | 82% | 82% |
| 2 — re-entry | `loop*` | 4% | 7% |
| 3 — fan-out | `cascades >= 20` | 1% | 1% |

Median cascades per game **1**, p90 **2**, max **69**. The shape of
the population is unchanged by measuring it properly, which is the
outcome worth having: the counter confirmed the classification rather
than overturning it.

## THE NAME TABLE WAS TRUNCATING, AND IS NOT ANY MORE

At 4 slots, `namecap` was firing on **64%** of cascades. At 32 slots
**`namecap` has disappeared from the grinder line entirely** — zero
overflows. Measured width: `distinct` median **3**, max **7**. So the
real cardinality sat just above the old ceiling, which is the worst
place for a ceiling to be: high enough to look plausible, low enough
to saturate most samples.

## CONTAMINATION IN EVERY `distinct=` NUMBER I HAVE PUBLISHED

Found by the agent, verified here: `fireTrigger` calls
`NoteDepthCapSwallow(event)` **before** `canonical :=
NormalizeEventSingle(event)` (`per_card/registry.go`, the
`depth > 8` branch precedes the normalisation). **The name table
therefore holds RAW, PRE-ALIAS event names**, and
`internal/gameengine/event_aliases.go` carries 241 entries.

Consequences for numbers already in this document:

- **`distinct` is an UPPER bound on canonical event cardinality.** Two
  aliases of one event count as two names.
- **`widemix` (2+ distinct) is over-reported** at the expense of
  `wave`; **`loopN` is over-reported** at the expense of `loop1`. A
  cascade can be pushed `loop1 -> loopN` by aliasing alone.
- **r182's `mixed` flag has the same contamination**, so "100% of
  capped games are mixed" is also an upper bound.

**What SURVIVES: everything resting on the board axis.** `n <=
boardMax` knows nothing about names, so **wave+widemix vs loop1+loopN
— the re-entry finding, the three regimes, and regime 2 being the
fastest population — are unaffected.** What is contaminated is only
the subdivision WITHIN each row of the 2x2.

Not fixed this round, deliberately: normalising at the cap site is one
map lookup on the capped path and therefore affordable, but it would
silently re-base every `distinct`/`dom%`/`shape` number against the
existing window. That is a measurement change to make on purpose, with
the discontinuity noted, not to slip into a counting round.

## METHOD NOTE — the brief sent the agent to the wrong site

I told it to count in `NoteCascadeRoot`'s existing `depthCapCascade !=
0` arm. That arm only runs when the NEXT cascade begins, so for the
82% of games whose single swallowing cascade is also their last, it
would have reported **`cascades=0`**. The agent read r186's own
`finishCascade` header, found the documented under-count, rejected the
placement, and counted at the `0 -> 1` transition inside
`NoteDepthCapSwallow` instead. Its mutation M1 implements my version:
**10 tests red.** Four agents in this chain have now contradicted a
brief of mine and been right each time.

---

## Eighth amendment (r188 tick) — n=1,795. Three published numbers broke, and there is a FOURTH population.

The r187 window now carries **1,795 capped games with the counted
`cascades=` field**, up from the n=91 the previous read was published
on. A 20x increase broke three numbers, all of them medians of tiny
subgroups — the same failure mode as n=48 -> n=701, extremes first.

### What held

| claim | n=91 | n=1,795 | verdict |
|---|---|---|---|
| regime 1 share (`cascades==1`) | 82% | **82.2%** | HELD exactly |
| regime 2 share (`loop*`) | 4-7% | **5.2%** | HELD, inside the band |
| median cascades/game | 1 | **1** | HELD |
| p90 cascades/game | 2 | **3** | essentially held |
| name table saturation | fixed at 32 | **0 of 1,795 at >=32; max distinct 9** | HELD — the bounded-counter fix is sound |

### What broke

**1. Regime 2 is NOT the fastest population.** Published as SETTLED:
"median 0.20s, the FASTEST games in the capped population". At n=1,795
regime 2's median is **0.30s — identical to regime 1's 0.30s.** The
0.20s came from roughly four games. The *interpretation* survives
(re-entry is caught instantly; a runaway costs correctness, not time)
but the comparative superlative does not. **Regime 2 is not
distinguishable from regime 1 by duration.**

**2. "ALL wall-clock cost is regime 3" is FALSE.** Measured share of
the 772.9s of capped wall-clock:

```
cascades==1      n=1476  82.2%   523.5s   67.7% of capped wall
loop* re-entry   n=  94   5.2%    31.8s    4.1%
cascades>=20     n=  38   2.1%   117.5s   15.2%
```

Regime 3 is **15.2%**, not all of it. Regime 1 dominates total
wall-clock purely by volume — 1,476 games at 0.30s outweigh 38
expensive ones. This is the inverse of the error in
[[feedback-a-big-aggregate-is-deep-or-broad]]: there I read a big
total as widespread; here I read a concentrated tail as if it were
the whole cost.

**3. Regime 3's cost figures were n~1-3.** Median duration 89.7s ->
**1.05s**; median swallows 2,157 -> **397**; max 203.3s -> 66.2s in
this window. Only **2 of 1,795** capped games reach 10s at all.

### NEW: a fourth population — huge boards with FEW cascades

Sorting by duration surfaced a shape absent from the three-regime
model: games lasting seconds with `cascades=1..6` and
**`board` in the 800-1,568 range** against a population median of 38.

```
  66.2s casc=258  sw=1806  board=51     <- regime 3
  10.6s casc=2    sw=47    board=1568   <- NEW
   9.5s casc=67   sw=469   board=51     <- regime 3
   8.4s casc=1    sw=21    board=1167   <- NEW
   5.4s casc=6    sw=15    board=1461   <- NEW
```

Unit note, since this is exactly where I have erred three times:
`board` describes the **peak cascade**, `cascades`/`swallowed` are
**game totals**. These are not combined below.

```
board>=300   n=17  0.9% of capped games   median dur 3.30s   median casc 6   8.4% of capped wall
board<300    n=1778                        median dur 0.30s   median casc 1
overlap with cascades>=20:  EXACTLY 0
```

**Two disjoint duration mechanisms.** `pearson(cascades,dur)=0.495`,
`pearson(swallowed,dur)=0.366` — moderate and neither dominant, which
is what a mixture of two causes looks like projected onto one axis.
Same shape as the error that cost three reversals; this time the
second axis was `board`, already in the line and read past.

### Attribution, conditional within the capped population

Raw multipliers against `/api/live/elo` lifetime games
(joined on `owner+"/"+deck_id`) give 205x / 354x, but those are
confounded: these decks are over-represented in capped games
generally. The honest test is **given this deck is in a capped game,
how often is it this sub-population**, against that
sub-population's base rate.

```
REGIME 3 (base rate 2.1% of capped games)
  34.6x   winota_joiner_of_forces_b3_kaputtzimon   11 of its 15 capped games (73%)
  14.8x   gerrard_weatherlight_hero_b4_bingusblaster  22 of its 70 (31%)  <- largest absolute

BIG BOARD (base rate 0.9%)
  89.8x   sorin_of_house_markov_..._b2_khazad_doom  17 of its 20 capped games (85%)
```

Winota's mechanism was already oracle-verified: *"Whenever a non-Human
creature you control attacks... put a Human creature card onto the
battlefield tapped and attacking"* = one independent trigger per
attacker = fan-out. **Sorin is a new mechanism**, oracle-verified this
session: the back face `Sorin, Ravenous Neonate` has **`+2: Create a
Food token`** — a permanent minted per activation with no self-limit,
and nothing in the deck consumes Food. That is a board-growth engine,
not a cascade engine, which is why it lands in the population with
`cascades=1..6`.

**This refines, and does not overturn, the earlier Sorin finding.**
"Sorin is a slowness phenomenon with no special relationship to the
cap" still holds: Sorin is in only 20 of 1,795 capped games (1.1%,
about expected). What is new is that **when a Sorin game is capped it
is almost always (85%) a huge-board game**, and huge boards are a
duration mechanism in their own right. The two phenomena touch here,
not through the cap.

### Consequence for the ETB scan class

`FireEvolveTriggers`, `FireSoulbondTriggers` and `ApplyGraft` all scan
every permanent and gate only on `IsCreature()`. Their cost is O(board).
The previous argument for them was "82% of capped games are wide boards
(median ~38)". The real argument is stronger and narrower: **there are
live production games reaching 1,568 permanents at peak cascade**, and
these three scans run across all four seats at that size. Still an
owner call; the evidence behind it is now a measured maximum rather
than a median.

### Standing caveat, unchanged

Every `distinct` / `dom%` / `shape` number remains **alias-contaminated**
(`NoteDepthCapSwallow` runs before `NormalizeEventSingle` against a
241-entry table), so `widemix` is over-reported vs `wave` and `loopN`
vs `loop1`. **Everything above rests on the board and cascade-count
axes, neither of which looks at event names.**

---

## Ninth amendment (r189 tick) — n=2,866. The populations held; the WALL-CLOCK SHARE did not, and it is not a quotable statistic.

n rose 1,795 -> 2,866. Every structural number held:

| | n=1,795 | n=2,866 |
|---|---|---|
| regime 1 (`cascades==1`) | 82.2% | **82.1%** |
| regime 2 (`loop*`) | 5.2% | **4.7%** |
| regime 3 (`cascades>=20`) | 2.1% | **2.3%** |
| big board (`board>=300`) | 0.9% | **0.9%** |
| median dur r1 / r2 / r3 | 0.30 / 0.30 / 1.05 | **0.30 / 0.30 / 1.05** |
| median cascades/game | 1 | **1** |
| r3 x big-board overlap | 0 | **0** |
| name-table saturation (>=32) | 0 | **0** (max distinct 10) |

### The number that moved, and why it should never have been published as one

Last tick I corrected **"ALL wall-clock cost is regime 3"** to
**"15.2%"**. This tick the same calculation gives **36.6%**. Nothing
about the engine changed — **one game arrived.**

```
                        449.5s game IN     drop top 1   drop top 2   drop top 5
regime 3 share              36.6%            12.7%        7.6%         6.9%
regime 1 share              54.9%            75.6%       80.1%        82.4%
```

**A single game is 27.4% of all capped wall-clock.** The top five
games — **0.17% of the sample** — are 33.4% of it. The duration
distribution is median 0.30s, p99 2.10s, p99.9 12.40s, **max 449.5s**:
a 36x jump from p99.9 to the maximum.

**So "15.2%" was not a measurement, it was one draw from a
tail-dominated estimator**, and I presented it with two significant
figures. The correction I made was right in substance and wrong in
form: I replaced an overclaim ("all") with a spurious precision.

**What is actually stable, and what should be quoted:**

- **The ordering: r1 > r3 > big board > r2 in every variant above.**
- **The qualitative correction HOLDS and is the real finding:** the
  rare expensive tail does NOT own the total. Regime 1 is between
  51% and 82% of capped wall-clock on every slicing — 2,353 cheap
  games outweigh everything else even when the single worst game in
  the window is included.
- **Regime 3's share is not estimable at this n.** Range 6.9%-36.6%.
  Quote the range or quote nothing.

This is the companion rule to the one written last tick. Computing
the share was right; treating a share of a heavy-tailed total as a
point estimate was not. **A SHARE OF A HEAVY-TAILED TOTAL IS ITSELF
A TAIL STATISTIC.**

### The two extremes confirm the two mechanisms, by opposite routes

The two most expensive games in the window are the cleanest evidence
yet that these are genuinely different phenomena:

```
449.5s  cascades=550  swallowed=3871  board=115    <- fan-out
 12.4s  cascades=1    swallowed=9     board=1980   <- board growth
```

The 12.4s game took **one cascade and nine swallows**. The depth cap
is essentially irrelevant to it; it is slow because there are 1,980
permanents. The 449.5s game has a board of 115 — unremarkable — and
550 separate top-level cascades.

**Predicted-deck check, and it paid out.** Winota was named last tick
as regime 3's mechanism from oracle text alone (one independent
trigger per attacker). **Winota is in BOTH the 449.5s and the 66.2s
games** — the two worst in the window. Sorin is in the 12.4s
board=1,980 game. Each extreme belongs to the deck predicted for its
population.

### Conditional enrichment — stable across the n increase

```
                                        n=1,795            n=2,866
regime 3   winota_..._kaputtzimon    11/15  34.6x     16/24  67%  28.9x
           gerrard_..._bingusblaster 22/70  14.8x     41/130 32%  13.7x
big board  sorin_..._khazad_doom     17/20  89.8x     24/27  89%  101.9x
```

All three held their rank and rough magnitude. Sorin remains
near-deterministic: **89% of his capped games are big-board ones.**

### New maximum for the record

`board` max 1,568 -> **1,980** permanents at peak cascade.
`cascades` max 258 -> **550**. Population median board is still **38**.

### Other instruments this tick

Slow games: Sorin **61.1% of Oct-19 (n=36)**, against 39% of Oct-18
(n=222). Consistent across two reads today (65.5% at n=29) but still
a small sample — **quote the Oct-18 figure.** No new grinder gaps
>=180s. Both heap-abort strings unchanged since Oct 16.

Alias contamination unchanged and unaddressed: `distinct`/`dom%`/
`shape` remain upper bounds. Everything above rests on the board and
cascade-count axes, which do not read event names.

---

## Tenth amendment (r190 tick) — n=4,397. The populations held a THIRD time, and a rank-based instrument succeeds where the share failed.

n: 1,795 -> 2,866 -> **4,397**. Third consecutive read, no structural
movement:

| | n=1,795 | n=2,866 | n=4,397 |
|---|---|---|---|
| regime 1 `cascades==1` | 82.2% | 82.1% | **81.6%** |
| regime 2 `loop*` | 5.2% | 4.7% | **4.9%** |
| regime 3 `cascades>=20` | 2.1% | 2.3% | **2.6%** |
| big board `board>=300` | 0.9% | 0.9% | **0.9%** |
| median dur r1/r2/r3 | .30/.30/1.05 | .30/.30/1.05 | **.30/.30/1.00** |
| median cascades/game | 1 | 1 | **1** |
| r3 x big-board overlap | 0 | 0 | **0** |
| name table >=32 | 0 | 0 | **0** (max distinct 10) |

### The share LOOKED stable this tick. That is a coincidence, not convergence.

Regime 3's wall-clock share reads **36.3%**, against 36.6% last tick.
That near-identity is worthless: the tail composition changed
completely. Last tick's top five were `[449.5, 66.2, 12.4, 10.6, 9.5]`;
this tick's are **`[449.5, 224.6, 66.2, 48.2, 18.4]`** — two new
large games arrived and held the ratio up by accident.

```
                 now    drop top1   drop top2   drop top3   drop top5
regime 3        36.3%     22.7%       13.4%       10.2%       7.8%
regime 1        54.5%     66.1%       74.0%       76.8%      79.7%
```

Range **7.8%-36.3%**, essentially last tick's 6.9%-36.6%. **A point
estimate that repeats is not thereby confirmed** — check whether the
observations underneath it are the same ones. Here they were not.

### What IS robust: Winota's position in the duration RANKING

Rank statistics survive heavy tails where means and shares do not.
Applying that deliberately this tick:

```
Winota is in 34 of 4,397 capped games — a 0.77% base rate.

   top-5  by duration:  4  (80%)   103x
   top-10 by duration:  5  (50%)    65x
   top-20 by duration:  8  (40%)    52x
   top-50 by duration: 12  (24%)    31x
```

A clean monotone decay from 103x to 31x, computed from ranks alone —
**no durations are summed, so no single game can move it.** This is
the instrument the wall-clock share should have been.

Winota was named as regime 3's mechanism **from oracle text alone**
two ticks ago, before any of these games existed. It is now in **four
of the five most expensive games in the window** (449.5s, 224.6s,
66.2s, 48.2s).

### Sorin has the OPPOSITE rank signature, and the mechanisms predict it

```
Sorin is in 38 of 4,397 capped games — a 0.86% base rate.

   top-5  by duration:  0  ( 0%)     0x
   top-10 by duration:  4  (40%)    46x
   top-20 by duration: 10  (50%)    58x
   top-50 by duration: 24  (48%)    56x
```

**Zero in the top five, then ~50% from top-10 outward.** Sorin does
not produce extreme outliers; he dominates the moderately-slow band.

This is exactly what the two mechanisms imply, and neither was fitted
to it:

- **Fan-out (Winota)** — one independent trigger per attacker, each
  opening its own cascade. Attackers multiply, so the cost is
  **unbounded** and lands in the extreme tail.
- **Board growth (Sorin)** — `+2: Create a Food token`, one permanent
  per activation. That is **bounded by the number of turns**, so it
  produces a consistent multi-second band and never an outlier.

Two mechanisms, two different rank signatures, each matching its
mechanism's growth law. This is the first prediction in this chain
that constrains the SHAPE of the distribution rather than naming a
deck.

### One number to soften: regime 2's tail

Regime 2's maximum was 2.6s at n=2,866 and is **18.4s** now — a
single game, `cascades=1 cascademax=135 shape=loopN board=99`, one
cascade swallowing 135 events. **Exactly 1 of 217 `loop*` games
reaches 5s; the other 216 are all under 2.6s.** So "a runaway costs
correctness, not time" holds in the body of the distribution but is
not absolute. State it as 216 of 217.

### Enrichments, stable across three reads

```
                        n=1,795      n=2,866      n=4,397
winota    regime 3       34.6x        28.9x       26.1x  (23 of 34, 68%)
gerrard   regime 3       14.8x        13.7x       13.6x  (76 of 216, 35%)
sorin     big board      89.8x       101.9x      101.2x  (35 of 38, 92%)
```

### Other instruments

Watcher rotated at 149 files (62/hour, matching). No new grinder gaps
>=180s. Both heap-abort strings unchanged since Oct 16. Board maximum
still 1,980; cascades maximum still 550. Alias contamination in
`distinct`/`dom%`/`shape` unchanged — everything above rests on the
board, cascade-count and duration-rank axes, none of which read event
names.

---

## Eleventh amendment (r191 tick) — the watcher's FILL RATE was the signal. Throughput dips are long-game worker occupancy, NOT an r187 regression, and my pre-registered revert gate is confounded.

### How this started: an ops number that refused to fit

READ 0 showed **104 watcher files in 47 minutes = 133/hour**, against
the **62/hour** model confirmed on the two previous runs. The fill
rate had doubled.

`stallwatch7.sh` captures when `g/min now` sits **below 760 for three
consecutive windows**. So its fill rate is not housekeeping — **it is
the stall indicator.** A doubled capture rate means throughput spent
twice as long in the degraded band. The rotation threshold was the
least interesting thing about that number.

### Throughput by hour, and the gate I nearly tripped

```
hour             median    p25     <760    <840
2026/10/18 16     907.6   862.8      5%     12%
2026/10/18 19     846.9   668.2     36%     44%
2026/10/18 23     873.0   769.9     23%     33%
2026/10/19 01     671.0   636.2     80%     85%
2026/10/19 04     769.5   619.8     48%     66%
```

r187 deployed at **00:12Z**. Pre-deploy hourly medians run 847-908;
post-deploy they run 650-854. **My pre-registered revert threshold is
"clean median below 840", and three post-deploy hours are under it.**

### It is not r187. It is worker occupancy, and the correlation is -0.892

.206 runs **four grinder workers**, so a single long game removes 25%
of capacity for its whole duration. Summing `SLOW GAME` durations per
hour against a fixed denominator of **4 x 3600 = 14,400 worker-seconds**:

```
hour            median  slow-secs  % of 4w-hour   905*(1-frac)
2026/10/18 16    907.6       259       1.8%            889
2026/10/18 19    846.9      1548      10.8%            808
2026/10/19 01    671.0      2814      19.5%            728
2026/10/19 02    839.2       953       6.6%            845
2026/10/19 03    854.3      1113       7.7%            835
2026/10/19 04    769.5      2338      16.2%            758

pearson(occupancy fraction, hourly median g/min) = -0.892  (13 full hours)
```

A one-parameter model — clean rate 905 scaled by remaining capacity —
tracks the observed median within a few percent across the whole
range. **The hours that breached my revert threshold are exactly the
hours with the heaviest long-game occupancy.** There is no r187
signal left to find once occupancy is accounted for.

Only 05:00 misses (observed 650, predicted 867), and that is an
artifact with a known sign: a game is logged when it **finishes**, so
a partial hour under-counts the games still running inside it.

### The gate is wrong, and it is my own

**"Clean median below 840" cannot distinguish a code regression from
an unlucky hour of deck pairings.** It fires on the deck mix. Had I
applied it as written this tick I would have rolled back r187 for a
Winota game and a handful of 600-second stalls.

**Replacement: gate on the RESIDUAL**, observed median minus
`905 x (1 - occupancy)`. That holds the deck mix constant and leaves
only what the code did. The raw median stays as a service-health
number; it is not a regression detector. Recorded here rather than
changed silently.

This is the same error class as the wall-clock share two ticks ago —
**a statistic whose denominator moves for reasons unrelated to what I
am trying to measure.** The fix is the same: find the denominator
that is actually fixed. Worker-seconds per hour is genuinely constant
at 14,400, which is why this instrument works where the share did not.

### Where the occupancy comes from: a 600s wall that leaks

Distribution of the 1,477 `SLOW GAME` durations:

```
    10-20s   622
    20-60s   436
    60-120s  179
   120-300s  117
   300-590s   35
   590-610s   45   <- spike
     >610s    43   max 3,102.6s
```

**45 games land in 590-610s, every one between 599.1 and 600.3**,
against only 35 in the whole 300-590s range. That is a 600-second
limit firing. But **43 games run past it, out to 3,102.6s — 51
minutes on one of four workers.** Whatever enforces 600s is not
enforced on these. Worth a look on its own; not chased this tick.

### The four populations — fourth consecutive read, n=5,214

```
                    n=1795  n=2866  n=4397  n=5214
1 cascades==1        82.2%   82.1%   81.6%   81.8%
2 loop* re-entry      5.2%    4.7%    4.9%    4.9%
3 cascades>=20        2.1%    2.3%    2.6%    2.4%
medians r1/r2/r3   .30/.30/1.05  ->  .30/.30/0.90
```

Rank signatures, unchanged to within a point:

```
Winota (42 of 5,214, base 0.81%)   top5 80% 99x | top10 50% 62x | top20 40% 50x | top50 24% 30x
Sorin  (43 of 5,214, base 0.82%)   top5  0%  0x | top10 40% 49x | top20 50% 61x | top50 50% 61x
```

**And this now joins up with the throughput finding.** Winota owns
the extreme tail of game duration; the extreme tail of game duration
is what consumes worker capacity; worker capacity is what sets
throughput. The chain from one card's oracle text to the hourly
throughput number is complete and measured at every link.

---

## Twelfth amendment (r192) — the 600s wall is `perGameWallClock`, nothing bypasses it, and the occupancy numerator was misattributed

Three results this read, at **n=6,375** capped games. One is a correction to
something this document has asserted twice.

### A. CORRECTION — "r3 × big-board overlap is EXACTLY 0" is false from n=4,397

The eighth amendment introduced the big-board population and recorded an
overlap of exactly 0 with regime 3. The ninth, tenth and eleventh repeated it.
Recomputing the raw predicate over the same log, truncated to each historical n:

```
n= 1795  raw overlap (cascades>=20 AND board>=300) = 0
n= 2866  raw overlap                                = 0
n= 4397  raw overlap                                = 1   <- already 1 when "0" was published
n= 5214  raw overlap                                = 1   <- and again
n= 6375  raw overlap                                = 1
```

The game is the 224.6s Winota game, `cascades=414 board=573`. It was in the
data when the claim was last restated, so the claim was wrong at the time of
writing, not overtaken by events.

What survives: the two mechanisms are still overwhelmingly disjoint — **1 game
in 6,375**. What does not survive is the word *exactly*. A count of zero in a
sample is "none observed in n", never "cannot happen"; this is the same error
as writing "none in 48" as "none", and it is the second time it has bitten
this document.

### B. The 600s wall — found, and it is not a bypass

`internal/hexapi/showmatch.go:52`:

```go
perGameWallClock = 10 * time.Minute
```

Armed via `gameengine.SetGameDeadline` at showmatch.go:1442 and :2185; enforced
by `CheckGameDeadline`, which is called from exactly four sites —
`turn_phase_boundary` (tournament/turn.go:109), `token_create`
(resolve.go:2411), `trigger_drain` (trigger_batch.go:167) and `stack_drain`
(stack.go:107).

**Nothing escapes the check.** In this server run (pid 322631, since 00:12Z):

```
SLOW GAMEs >=590s this run : 9    (599.6 2033.3 599.1 871.0 603.5 599.1 599.6 599.7 1400.8)
guards=game_deadline       : 9
```

Nine for nine. The r191 framing — "something enforces 600s and something else
escapes it" — was wrong. The detection is complete; the **unwind** is what
leaks.

The site attribution was already in the log, in a field this document had not
read. `LoopGuardLabelsFor("game_deadline")` (r167) is wired into the grinder
line at `grinder_line_r176.go:97` as `deadlines=`:

```
deadlines=token_create:5,stack_drain:2,trigger_drain:1,turn_phase_boundary:1
```

**Only 1 of 9 was noticed at a turn/phase boundary** — and that is the one
place where returning true actually unwinds the game, because the grinder's
end-check sits between turns. The other 8 are noticed deep inside a turn,
where the bail exits only the inner loop and the turn carries on. That is
exactly the shape the file's own r161 comment documented and it reproduces
cleanly here:

```
6 of 9 ended within 3.5s of the deadline   (599.1 - 603.5)
3 of 9 ran on as already-ended draws       (+271s, +801s, +1433s)
```

`token_create` being the majority site joins this to the stall population:
Sorin (`+2: Create a Food token`, nothing consumes them) is **40% of Oct-18
slow games (95/238)** and 92% of his capped games are big-board. The deadline
is noticed inside the token loop because the token loop is where the time goes.

Fix direction (not done, owner call): the unwind must propagate out of the
in-progress turn rather than only marking the game ended. The four call sites
are sufficient for detection and demonstrably so.

### C. The occupancy numerator was misattributed — spreading it is strictly better

A game is logged when it FINISHES, so r191 charged every one of its seconds to
its final hour. A 2,033s game occupies one worker across parts of three hours
but was billed entirely to the third. Spreading each SLOW GAME's seconds across
the hours it actually ran:

```
hour            median  naive_occ  res_naive   true_occ  res_true
2026/10/18 11    696.2     10.5%       -113      14.1%       -81
2026/10/18 13    707.9      7.9%       -125       7.9%      -125
2026/10/19 01    671.0     19.5%        -57      21.2%       -43
2026/10/19 05    651.8     12.4%       -141      19.1%       -81

mean |residual|   naive 44.0   ->   spread 33.9      (25 four-worker hours)
```

The worst hour halves (−141 → −81) because its true occupancy was 19.1%, not
12.4%. **Use the spread numerator from here on.**

`10/18 13` at −125 is fully explained and is not an engine signal: that hour
carries the known 1,201s grinder gap (13:05:18 → 13:25:19) and logged only 79
g/min samples against 120. **The per-hour sample count is a free gap detector —
read it before reading a residual.**

Verdict on r187, on the corrected instrument: post-r187 full hours (10/19
00–05) mean residual **−14**; pre-r187 four-worker hours (10/18 05–23) mean
**+2**. The 16-point difference is well inside the hour-to-hour scatter
(sd ≈ 45), and no two consecutive hours sit below −60. **No regression. Do not
revert.**

A monitor-contamination check, since the heavy-capture hours are also the
negative-residual hours: a capture is a 20s CPU profile plus two goroutine
dumps, and 36 captures put profiling active ~20% of hour 05. At the few percent
a Go CPU profile costs, that bounds the watcher's own effect near **1%, ~8
g/min** — two orders off the −141 it would have had to explain. The watcher is
not the cause. The correlation is also partly circular by construction: the
watcher only fires when throughput is already low.

### D. Standing measurements

```
watcher fill rate        136 files / 73.1 min = 111.6/hour   (rotated on rate)
four populations (n=6375)   r1 82.0%   r2 5.0%   r3 2.3%   big-board 0.8%
  shares at 1795/2866/4397/5214/6375:  82.2/82.1/81.6/81.8/82.0
                                        5.2/ 4.7/ 4.9/ 4.9/ 5.0
                                        2.1/ 2.3/ 2.6/ 2.4/ 2.3
                                        0.9/ 0.9/ 0.9/ 0.9/ 0.8
medians r1/r2/r3        0.30s / 0.30s / 0.90s      median cascades 1, p90 3
maxima                  board 3,058 (was 1,980)  cascades 550  capped 449.5s  slow 3,102.6s
name table              0 of 6,375 reached >=32; max distinct 10
rank signatures   Winota 52/6375 (0.82%)  top5 80% 98x | top10 50% 61x | top20 50% 61x | top50 28% 34x
                  Sorin  49/6375 (0.77%)  top5  0%  0x | top10 30% 39x | top20 40% 52x | top50 54% 70x
regime 2 tail           n=316, max 18.4s, 315 of 316 under 2.6s
```

Fifth consecutive hold on the population shares. The top-5 by duration is
**not** the same set as last read — `[449.5, 224.6, 66.2, 48.2, 18.4]` became
`[449.5, 224.6, 150.4, 147.6, 66.2]` — so the stability in the rank signatures
is a property of the ranking, not of a frozen sample. Note the new 150.4s game
is `cascades=1 board=249`: a regime-1 game that is big-board in everything but
the 300 threshold.

Both abort strings unchanged since Oct 16. Only grinder gap >=180s remains the
known Oct 18 13:05 (1201s).

---

## Thirteenth amendment (r193) — the deadline invariant holds a second read; a second guard surfaces, and the instrument that would attribute it is wired for only two guards

**n=7,207** capped games.

### A. The 600s invariant replicates

```
SLOW GAMEs >=590s this run : 10   (+599.9 since r192)
guards=game_deadline       : 10
deadlines=token_create:6,stack_drain:2,trigger_drain:1,turn_phase_boundary:1
```

Ten for ten, a second consecutive read. The new game was noticed at
`token_create` (5 -> 6) and **unwound promptly, at 599.9s**. That is worth
recording because it refines r192: the noticing SITE does not by itself
determine whether the game unwinds. `token_create` produced both a clean
599.9s stop and the +1,433s overrun. What decides the outcome is how much work
the in-progress turn has left, not which inner loop happened to look at the
clock.

### B. Throughput — no regression, and hour 06 shows why the spread matters

```
hour            median  naive_occ  res_naive   true_occ  res_true
2026/10/19 05    651.8     12.4%      -141      19.1%       -81
2026/10/19 06    801.8     15.3%       +35       8.6%       -26

mean |residual|  naive 43.6   spread 33.5
```

Hour 06 **changes sign** under the corrected numerator: naive +35, spread −26.
It had been credited with seconds belonging to games that actually ran in 05.
A sign flip is a stronger demonstration than r192's magnitude change — the
naive instrument was not merely imprecise, it pointed the wrong way.

05 = −81 and 06 = −26, so **no two consecutive hours below −60. The gate does
not fire.** Post-r187 mean residual −15.6 over seven full hours against
pre-r187 +2, well inside the sd ≈ 45 scatter. **No regression.**

Watcher fill rate **57.6/hour** (46 files / 47.9 min), down from 111.6. Per the
standing rule that the fill rate is itself the stall indicator, the halving
says there were materially fewer sub-760 windows this hour — consistent with
the residuals above. No rotation (projected ~104 by next tick).

### C. Sixth consecutive hold

```
                  n=5214  n=6375  n=7207
1 cascades==1      81.8%   82.0%   82.0%
2 loop* re-entry    4.9%    5.0%    4.9%
3 cascades>=20      2.4%    2.3%    2.3%
4 board>=300        0.9%    0.8%    0.8%

Winota  56/7207 (0.78%)  top5 80% 103x | top10 50% 64x | top20 50% 64x | top50 26% 33x
Sorin   57/7207 (0.79%)  top5  0%   0x | top10 30% 38x | top20 40% 51x | top50 60% 76x
```

Maxima unchanged (board 3,058; cascades 550; capped 449.5s). Name table still
0 of 7,207 at >=32. Regime 2 still 352 of 353 under 2.6s.

### D. NEW — a second guard moved, and I cannot attribute it

`percard_dispatch_total_cap` went **347 -> 2,116** inside a single 30-second
window at 06:35:26. It is not a slowly-accumulating counter:

```
2026/10/18 18:50:16   1028
2026/10/18 18:52:46  10096      <- ~9,000 in 150 seconds
2026/10/19 01:25:26    347      <- this run, first appearance
2026/10/19 06:35:26   2116      <- this run, one window
```

It resets with the process, and it arrives in bursts. Two bursts this run, each
confined to one 30s window.

**I could not attribute either burst, and the reason is an instrument gap I
walked into and had to back out of.** My first read of the label counters said
the guard fires exclusively on `permanent_etb`, which would have tied it
straight to item (17), the ETB scan class. That was wrong twice over:

1. The value I read (`permanent_etb:9699`) is a **pre-restart** figure. The
   guard counter for this run reads 2,116; 9,699 belongs to the 10/18 18:52
   burst.
2. `guardtop=` is a **top-5 column**, and the current top 5 is entirely
   `percard_dispatch_depth_cap` labels in the tens of thousands. The total-cap
   label is not in it at all. So "exclusively `permanent_etb`" was a selection
   effect: `permanent_etb` is simply the only one of this guard's labels that
   ever grows large enough to become visible.

Three labels have been observed for this guard across the whole log —
`permanent_etb`, `creature_dies`, `creature_attacks`. **Which of them owns the
current 2,116 is unknown, and the aggregate alone cannot tell deep from broad.**

This is precisely the failure r167 wrote `LoopGuardLabelsFor` to fix, and its
own comment says so: a guard "rank ~15th by volume and therefore invisible".
That renderer prints EVERY label for ONE named guard and is already wired —
but for only two guards, `game_deadline` and `DepthCapFirstEventGuard`
(`grinder_line_r176.go:97` and `:133`). Wiring a third call for
`percard_dispatch_total_cap` would answer this outright, costs a few bytes on a
line only when the guard fires, and reuses machinery that is already written
and already tested.

**Proposed, not done — needs the owner gate and a deploy.** Recording it here
so the next read does not repeat the same mis-attribution.

The general lesson, which is the same one three amendments running: a
top-N column answers "what is busiest", never "what is this specific thing
doing". When the question names a guard, read a per-guard instrument.

---

## Fourteenth amendment (r194) — the rank instrument has two flaws I had not seen: k=5 is a 5-observation statistic, and half the top-20 are not cap-driven at all

**n=7,987** capped games. Standing results first, then the correction.

### A. Standing — all three invariants replicate

```
SLOW GAMEs >=590s this run : 11      guards=game_deadline : 11      <- 11/11, third read
deadlines=token_create:7,stack_drain:2,trigger_drain:1,turn_phase_boundary:1

populations (seventh consecutive hold)   r1 82.0%  r2 4.9%  r3 2.3%  big 0.8%
watcher fill rate   124 files / 95.3 min = 78.0/hour   (rotated; was 57.6)
percard_dispatch_total_cap   FROZEN at 2,116 all hour   <- confirms (6b): bursty, not accumulating

hour            median  naive_occ  res_naive  true_occ  res_true
2026/10/19 05    651.8     12.4%      -141      19.1%      -81
2026/10/19 06    801.8     15.3%       +35       8.6%      -26
2026/10/19 07    802.0      8.9%       -22       8.9%      -22
mean |residual|  naive 42.8 -> spread 33.1
```

No two consecutive hours below −60. **Gate does not fire. No regression.** Both
abort strings still frozen at Oct 16.

### B. CORRECTION 1 — the top-5 rank statistic is fragile, and I have been quoting it as a headline

Winota's top-5 share went **80% (103x) -> 60% (76x)** this read. Nothing about
Winota changed. One new game arrived at rank 5 — `114.3s cascades=1 board=90`,
no Winota — and displaced Winota's 66.2s game out of the set.

**A rank over k=5 is a 5-observation statistic.** One arrival moves it 20
points, and because the base rate is ~0.8%, that 20 points reads as a 27-point
swing in the enrichment multiple. I adopted rank statistics at r190 because
they survive heavy tails, which is true and still true — but tail-robustness
and small-n robustness are independent properties, and fixing the first did
nothing for the second. The same "a median of four games is not a median" rule
applies to ranks and I did not apply it.

**Quote top-20 and top-50. They moved 50->45% and 26->30% on the same data.**

### C. CORRECTION 2 — half of the top 20 by duration are not cap-driven games

The deeper problem. `CAPPED GAME` lines exist for any game that touched the
depth cap at all, so ranking that population by duration silently mixes two
different things:

```
top-10 by duration        swallowed    verdict
   449.5s                      3871    cap load-bearing
   224.6s                      2901    cap load-bearing
   150.4s                         9    CAP INCIDENTAL
   147.6s                      2648    cap load-bearing
   114.3s                         6    CAP INCIDENTAL
    66.2s                      1806    cap load-bearing
    48.2s                      1498    cap load-bearing
    38.9s                        70    CAP INCIDENTAL
    18.4s                       135    cap load-bearing
    15.7s                        19    CAP INCIDENTAL

of the top 20 by duration, 10 have swallowed<100
```

A 150-second game that swallowed 9 triggers did not spend its time on the cap.
It is in the population by accident. **So "deck X is in k% of the longest
capped games" has been answering a slightly different question than the one I
thought it answered.**

Only **321 of 7,987 (4%)** of capped games have `swallowed>=100`.

### D. What that correction does to the three mechanisms

Re-ranking inside the cap-load-bearing subset, and — because the threshold is
my choice, not a property of the data — sweeping it:

```
  thr      n    winota top20   sorin top20   gerrard top20
   20   2270         60%           35%            0%
   50    633         75%           10%            5%
  100    321         80%            0%           10%
  200    180         80%            0%           15%
  500     73         50%            0%           50%
 1000     32         35%            0%           60%
```

**Sorin's exit is MONOTONE across the whole sweep — 35% to 0%.** The more
load-bearing the cap, the less he appears. That is robust to the threshold and
it corrects his story: his mid-band position in the all-capped ranking came
from games where **the cap is incidental**, which is exactly what a board-growth
mechanism should look like. Board growth was never a cap phenomenon; the
instrument now agrees with the mechanism instead of muddling it.

**Winota and Gerrard SWAP, and I will not name a winner.** Winota leads at
moderate thresholds and Gerrard at extreme ones. Part of this is structural:
`swallowed` and duration are correlated, so thresholding on one and ranking by
the other partly conditions on the outcome, and at high thresholds the ranking
mostly rewards whoever has the most such games — which is Gerrard (39.6% of the
load-bearing pool by base rate, the largest single contributor).

**Had I picked 100 and reported it, I would have published "Winota is 100% of
the top-5 cap-driven games" as a clean result.** It is an artifact of the cut.

### E. What still stands

- Winota's mechanism (unbounded fan-out) remains the only one that produces
  games in the thousands of swallows AND hundreds of seconds. The four longest
  cap-load-bearing games are all his, at swallowed 1,498–3,871.
- Sorin's mechanism (board growth, bounded by turn count) stands and is now
  better supported: he is absent from cap-driven duration at every threshold.
- Gerrard generates cap-load-bearing games in volume but not in length.
- **A new, small, genuinely unexplained class:** 2 games of 7,987 are long
  (150.4s, 114.3s) with `cascades=1`, small boards, and almost no swallows.
  They are 2 of the 6 games over 60 seconds. No mechanism here explains them
  and I am not going to invent one — recorded so the next read does not
  mistake them for fan-out.

## Fifteenth amendment (r195) — the throughput model's SLOPE is occupancy-biased; the watcher's fill rate is ~88% an occupancy meter

n = 9,322 capped games (prev 7,987). Standing re-measure first, then the finding.

### A. Standing re-measure — all holds

```
populations (EIGHTH consecutive hold)   r1 82.0  r2 4.7  r3 2.3  big 0.8
   (r2 drifted 4.9 -> 4.7; r3 x big-board overlap still 1, never "exactly 0")
deadline            11 games >=590s vs guards=game_deadline:11   <- FOURTH consecutive 11/11
   same three overruns: +271s, +801s, +1433s; the other 8 stopped within 3.5s
percard_dispatch_total_cap   FROZEN at 2,116 for a second hour — bursty, confirmed again
abort strings       22 / 3,628, both unchanged since Oct 16
grinder gaps >=180s none new; latest remains the known Oct-18 13:25 (1201s)
stalls              Oct-18 holds at Sorin 40% (95/238), Winota 14%
rank signatures     winota top20 45% / top50 30%    sorin top20 40% / top50 60%
   threshold sweep REPLICATES r194 exactly: Sorin monotone 30%->0% (robust);
   Winota/Gerrard swap with the cut (65/80/85/90/55/40 vs 0/0/5/10/40/55) — NO WINNER STATED
watcher fill rate   9.5 files/hour  <- 8x below the previous low (62, 133, 111.6, 57.6, 78.0)
```

The fill-rate collapse is REAL, not a wedged watcher: the process was mid-capture at the
read, and the trigger condition itself collapsed — three-in-a-row sub-760 windows went
66 (hour 05) -> 35 (07) -> **4** (08). 12 files = 3 captures matches 5 trigger windows.

### B. NEAR-MISS, caught before publishing — a "Winota surge" that was arithmetic

Winota's cumulative capped count went 63 -> 92 while the population grew 7,987 -> 9,322
(+46% vs +17%), which reads as a 3x rate increase. It is not. Per-hour share:

```
10/18 21..23   1.7%  2.3%  1.6%
10/19 04..08   0.9%  1.5%  0.7%  1.1%  1.7%
```

0.7-2.3%, no trend. The "surge" came from differencing two CUMULATIVE totals computed
over differently-filtered sets. **Differencing two cumulative totals is not a rate.**

### C. THE FINDING — the residual gate has an occupancy-dependent bias

`residual = median - 905*(1-occupancy)` is a ONE-parameter through-origin model. If 905 is
wrong the residual is not orthogonal to occupancy, and it is not:

```
corr(occupancy, residual) = -0.393        (should be ~0 if the slope is right)
fitted  median = -249.6 + 1179.2*(1-occupancy)     mean |resid| 30.5 -> 27.2
corr(occupancy, fitted-residual) = +0.000 (by construction)
```

**Consequence — the bias has a direction, and it is large against a -60 threshold:**

```
                                905 model     fitted slope
lowest-occupancy third  (1.8-5.2%)    +4.8          -9.3
highest-occupancy third (9.8-21.5%)  -19.9          -1.7
```

The 905 model makes BUSY hours look ~25 g/min worse and QUIET hours better. Reading that
spread as "the engine degrades under load" would have been a slope artifact.

**It changes which hour is flagged today.** Hour 08 is a QUIET hour (occupancy 2.3%) at
residual -54, which the 905 model under-states: under the fitted slope it is **-72**.

```
hours below -60:  905 model -> 10/18 11, 10/19 05
                  fitted    -> 10/18 11, 10/19 08
```

**NEITHER MODEL PRODUCES TWO CONSECUTIVE HOURS BELOW -60. THE GATE DOES NOT TRIP.**

### D. I am NOT re-basing the model — the slope is marginal

```
fitted slope 1179 +/- 129 (SE)   normal 95% CI [920, 1438]   excludes 905
bootstrap 2.5-97.5%              [899, 1425]                 INCLUDES 905 (2.7% below)
occupancy range only 1.8% - 21.5%
```

The two CI methods disagree about whether 905 is excluded, which is exactly the regime
where a finding must not be declared. A 2-parameter fit also beats a 0-parameter one for
free, and the fitted intercept (-249.6) is physically meaningless outside the observed
range. **Recorded as a known bias with a direction, not a correction.** What would settle
it is high-occupancy hours — the slope is poorly determined over a 20-point x-range.

### E. Operational note for the next tick

Hour 08 is the reading to watch, NOT to dismiss: it is the quietest hour in the table
(2.3% occupancy, 15.0% sub-760, 4 trigger windows) and still ran 54 below model. If hour
09 also comes in negative, that is two consecutive and the gate trips on a QUIET stretch,
which no stall-based explanation covers.

### F. The watcher's fill rate is mostly an occupancy meter

```
corr(occupancy, sub760%) = +0.937      (r^2 = 0.88)
corr(sub760%, residual)        = -0.548
corr(sub760%, fitted-residual) = -0.195   <- most of it was the slope artifact
```

The brief calls the fill rate "the stall indicator". It is ~88% explained by how many long
games are occupying workers, which is already measured directly and better by occupancy.
Once the slope artifact is removed, only -0.195 of independent signal remains. The fill
rate remains useful as a CHEAP liveness/orientation number and for sizing the watcher's
own cost — but it should not be read as evidence about engine stalls on its own.

## Sixteenth amendment (r196) — r187 LEAKS HEAP AT +26 MB/h; r186 DOES NOT. Two controls, no CI overlap.

n = 10,331 capped games (prev 9,322). Standing first, then the finding.

### A. Standing re-measure — all holds, and (1c) resolves WITHOUT an alarm

```
populations (NINTH consecutive hold)   r1 82.3  r2 4.8  r3 2.2  big 0.8
deadline        12 games >=590s vs guards=game_deadline:12   <- FIFTH consecutive
   new game stopped CLEANLY at 599.8s via token_create (now :8) — again, the SITE
   does not determine the unwind. Same three overruns: +271s, +801s, +1433s.
total_cap       FROZEN at 2,116 for a THIRD hour
abort strings   22 / 3,628, unchanged since Oct 16;  no new grinder gaps >=180s
stalls          Oct-18 holds exactly: Sorin 40% (95/238), Winota 14%
ranks           winota top20 45% / top50 32%    sorin top20 40% / top50 58%
   sweep replicates a third time: Sorin monotone 30%->0%; Winota/Gerrard swap. No winner.
watcher         68 files / 135.7 min = 30.1/hour cumulative (56 in the last hour alone)
```

**(1c) RESOLVED.** Hour 09 came in at **-39** (full 120 samples, occupancy 6.4%); under the
fitted slope -46. Hour 08 was -54 / -72. **Neither model gives two consecutive hours below
-60 — the gate does not trip.** `corr(time, residual) = -0.145`: no drift.

### B. THE FINDING — r187 leaks heap; r186 does not

07/08/09 being three mild negatives in a row sent me to the pre/post-r187 split (r187
deployed 00:12Z 10/19; before that r186). The throughput gap survived every control I
tried, so I looked at memory, age-matched by process.

```
run                          samples   span    heap slope MB/h    95% CI
r186  Oct18 06:27-13:24          791   6.6h      +4.91 +/- 1.71   [ +1.5,  +8.3]
r186  Oct18 13:24-Oct19 00:12   1285  10.8h      -0.70 +/- 0.81   [ -2.3,  +0.9]
r187  Oct19 00:12-now           1220  10.2h     +26.32 +/- 0.93   [+24.5, +28.2]
```

**No overlap with either control.** RSS moves faster still: r186 -2.36 and +9.21 MB/h,
r187 **+41.55** MB/h. Age-matched heap by process age makes it visual:

```
age h   r186 heap   r187 heap
    0         941         920
    3         965        1021
    6        1015        1116
    9         970        1193
   10         940        1216
```

**The innocent explanation is ruled out:** `ELO pool` is 1425 and perfectly constant in
BOTH runs, so this is not the ratings table growing.

**Operational size: the ceiling is 6442MB** (`process heap NNNNMB exceeds the 6442MB
ceiling`). From 1219MB at +26.3 MB/h that is **~8.3 days** to an abort. Not urgent —
and that is exactly why it was never caught: **the leak is masked by restarts.** The
process almost never survives 8 days between deploys and crashes.

### C. Throughput: a SEPARATE result, not the leak's symptom

```
age-matched mean g/min   r186 813.7   r187 765.0    -48.7  (-6.0%), raw
occupancy-controlled residual        pre +10.6  post -28.3   (-38.9)
  under the fitted slope (removes the r195 bias)  +8.8 -> -23.6   (-32.5)
  paired BY HOUR OF DAY (controls diurnal mix)    -34.1, 4/5 hours negative, t=-2.02
  permutation test, unpaired                      p = 0.013 one-sided
```

Robust to which slope I use, so it is not the r195 artifact. **But I am NOT claiming the
leak causes the slowdown:** within-run `corr(heap, g/min)` is only **-0.186** for r187 and
-0.283 for r186 — if heap growth were throttling throughput that correlation should be
strong and stronger than the control's. Two findings about the same build, reported
separately.

### D. What is weak here, stated plainly

- **n=1 process on the treatment side.** Two r186 controls, one r187 run. The within-run
  slope is tight (1,220 samples) but "r187 leaks" rests on a single process. **The current
  process is still running — re-measure its slope next tick; linear continuation at ~26
  MB/h would settle it.**
- **I could not diff the binaries.** r186 `401a6e2f` is not in the worktree's object store,
  so I have no idea what changed and cannot name a suspect line.
- The paired hour-of-day test is n=5 (t=-2.02); the permutation p=0.013 is the stronger leg.
- heap= is a sawtooth sampled point-in-time; mitigated by ~120 samples/hour, not eliminated.

### E. FILED, NOT ACTED ON

Rollback binary `~/hexdek/hexdek-server.bak-pre-r187-20261019` (= r186) is in place. **A
rollback is a production action and an OWNER CALL — not taken.** Nothing here is urgent at
8 days of headroom. The cheap next step that needs no decision is simply to re-read the
slope next tick.

## Seventeenth amendment (r197) — CORRECTION: the r187 heap growth is DECELERATING. "8.3 days to the ceiling" is WITHDRAWN.

n = 11,173 capped games (prev 10,331). The correction first, because it is mine.

### A. CORRECTION — r196's headline extrapolation was invalid

r196 published **"+26.32 MB/h, ~8.3 days to the 6442MB ceiling"**. The rate is not
constant, so the extrapolation does not hold. Re-fit over the full run (n=1,316, 10.95h):

```
full-run heap slope   +25.20 +/- 0.85 MB/h      <- matches r196; the SLOPE replicated
DECELERATION TEST
  first half   n=658   heap +27.89 +/- 2.23   sys +74.90 +/- 1.56
  second half  n=658   heap +15.79 +/- 2.53   sys +36.14 +/- 0.47
quadratic      heap = 923 +36.70*t -1.050*t^2   (curvature clearly negative)
```

**The half-slope CIs do not overlap** (first half low end 23.4, second half high end 20.9).
The growth rate roughly HALVED across the run, and `sys` fell even harder, 74.9 -> 36.1.

**WITHDRAWN:** the 8.3-day figure, and the word "leak" as the characterisation. A constant
rate was never tested — I fitted one line to a bending curve and extrapolated 8 days out.

**WHAT SURVIVES, and it is a weaker claim:** r187 still reaches a **higher steady state**
than r186. The two r186 controls were flat (-0.70 +/- 0.81 and +4.91 +/- 1.71) at ~900-950MB
across 10.8h and 6.6h; r187 rose 920 -> ~1194 and is now bending over. That is roughly
**+250MB of baseline**, on a box with 15.8GB and a 6442MB ceiling. **Not operationally
interesting.** The r186-vs-r187 difference is real; the trajectory is not a leak.

**I am NOT predicting the plateau either.** The quadratic's vertex (~1244MB at ~17.5h) is
as much an extrapolation as the line was, and a downward-bending parabola is physically
wrong past its peak. What the DATA says is only that the rate is falling. Another few
hours of this run settles where it lands.

### B. A near-miss in the OPPOSITE direction, same tick

READ 0 returned `heap=952MB`, down from 1219MB, and my first reaction was that it had
collapsed and the whole finding was dead. **It had not.** Heap is a ~500MB sawtooth:

```
age h   mean   min    max
    0    920   736   1088
   10   1194   947   1470
```

952 sits near the bottom of age 10's own range. **A single point sample cannot confirm or
refute a regression over 1,316 samples** — it nearly made me over-correct as badly as I had
over-claimed. Both errors were stopped by the same thing: fitting the series instead of
reading a number. Note also that r196's "age 10 = 1216" came from n=20; with n=116 it is
1194.

### C. The throughput result is UNCHANGED and is still a separate finding

```
age-matched over 0-10.9h   r186 813.7 g/min (n=1285)   r187 762.9 (n=1310)   -6.2%
```

Unaffected by the correction above, and still not attributed to memory.

### D. Standing re-measure — everything holds

```
populations (TENTH consecutive hold)   r1 82.2  r2 4.8  r3 2.2  big 0.8
deadline      14 games >=590s vs guards=game_deadline:14   <- SIXTH consecutive
  both new games stopped CLEANLY (599.3, 600.1); token_create now :10
total_cap     FROZEN at 2,116 for a FOURTH hour
residual      08 -54, 09 -39, 10 -13 — recovering; no two consecutive below -60
abort strings 22 / 3,628 unchanged since Oct 16; no new grinder gaps >=180s
stalls        Oct-18 holds exactly: Sorin 40% (95/238), Winota 14%
ranks         winota top20 50% / top50 34%   sorin top20 35% / top50 58%
  sweep replicates a FOURTH time: Sorin monotone 35%->0%; Winota/Gerrard swap. No winner.
watcher       116 files / 186.5 min = 37.3/hour — no rotation (116 <= 120, projected 153)
r3 x big-board overlap is now 2 in 11,173 (was 1) — still NEVER "exactly 0"
```

## Eighteenth amendment (r198) — the deceleration replicates; and a NEGATIVE result on attributing the total_cap burst

n = 11,921 capped games (prev 11,173).

### A. r187 heap — the r197 correction REPLICATES

```
full run n=1413 over 11.77h   heap +24.33 +/- 0.77   sys +40.40 +/- 0.47
SPLIT-HALF          first  n=706  heap +31.97 +/- 2.03   sys +69.39 +/- 1.42
                    second n=707  heap +18.69 +/- 2.28   sys +38.78 +/- 0.37
  CIs do not overlap (27.9 vs 23.3) — deceleration confirmed a second time
THIRDS   0.0-3.9h +28.42 +/- 3.58 | 3.9-7.8h +25.29 +/- 4.07 | 7.8-11.8h +18.29 +/- 4.25
hourly mean heap: 919 1012 1007 1020 1056 1108 1116 1138 1135 1192 1192 1205
```

**The thirds decline monotonically but their CIs OVERLAP — that finer resolution is
suggestive, not established.** The split-half is the result that holds.

**A free consistency check:** the full-run linear slope keeps falling as the run extends —
**26.32 (10.2h) -> 25.20 (10.95h) -> 24.33 (11.77h)**. That is exactly what a straight-line
fit does to a decelerating curve, and it is independent confirmation that r196's
constant-rate extrapolation was the wrong model.

Still rising, just slower (1192 -> 1192 -> 1205). **Still NOT predicting where it lands.**
Throughput result unchanged and still separate.

### B. `percard_dispatch_total_cap` BURST CAUGHT LIVE — and a failed attribution

Frozen at 2,116 for FOUR hours, then:

```
2026/10/19 11:37:26   4,333
2026/10/19 11:37:56   6,829
2026/10/19 11:38:26   8,016      <- ~5,900 in 90 seconds, then frozen again
```

Three bursts now characterised (10/18 18:51-18:52, 10/19 06:35, 10/19 11:37-11:38): all
**compressed into 60-90s, totally frozen between.** The bursty reading is now well
supported.

**`guardtop=` structurally cannot show it, confirmed again:** the current top-5 is
103,825 / 81,177 / 78,141 / 22,341 / 21,562 — all `percard_dispatch_depth_cap` labels. A
guard at 8,016 cannot enter that list. This is exactly (6b)'s point.

**NEGATIVE RESULT — I tried to shortcut the label wiring and it does not work.** Idea:
games log when they FINISH, so cross-reference decks in games finishing inside each burst
window. Gerrard appeared in both readable windows and looked like a hit:

```
                              gerrard share
base rate, all 16,338 games        3.8%
10/19 06:35-06:40 (burst)          5.1%   (6/118)
10/19 11:37-11:42 (burst)          5.5%   (4/73)
--- CONTROLS, quiet windows ---
10/19 09:37-09:42                  5.0%   (4/80)
10/19 10:37-10:42                  4.4%   (3/68)
10/19 07:35-07:40                  5.8%   (4/69)
```

**The controls read the same as the bursts. There is no signal.** I would have reported a
Gerrard association off the two burst numbers alone; three quiet windows cost one command
and killed it.

**Why the method cannot work here, stated so nobody retries it:** at ~800 games/min across
4 workers, a 90-second burst overlaps roughly 1,200 games, of which only a handful are
logged at all (only capped/slow games emit a line). The window cannot isolate one game.
**(6b)'s one-line `LoopGuardLabelsFor("percard_dispatch_total_cap")` wiring remains the
ONLY path to attribution.** Still owner-gated, still needs a deploy.

### C. Standing re-measure

```
populations (ELEVENTH consecutive hold)  r1 82.3  r2 4.8  r3 2.1  big 0.7
deadline      14 games >=590s vs guards=game_deadline:14   <- SEVENTH consecutive
              no new >=590s games this tick
residual      08 -54, 09 -39, 10 -13; hour 11 partial (117 samples) — not judged
abort strings 22 / 3,628 unchanged since Oct 16; no new grinder gaps >=180s
r3 x big-board overlap still 2 in 11,921 — never "exactly 0"
watcher       136 files / 234.1 min = 34.9/hour -> ROTATED (over the 120 threshold)
              new pids 400766/400768, verified single instance, server untouched
```

---

## Nineteenth amendment (r199) — n=13,064 capped games, r187 run at 12.88h

### A. The heap deceleration still holds, but the deceleration is itself decaying

Same process (pid 322631, no restart), so this is the same single r187
treatment run, now 12.88h and n=1,548 samples.

```
full run      heap +23.98 +/- 1.33   sys +39.02 +/- 0.77
SPLIT-HALF    first  n=774  heap +31.19 +/- 3.48   sys +62.48 +/- 2.47
              second n=774  heap +20.56 +/- 3.99   sys +38.47 +/- 0.58
              -> CIs [27.71, 34.67] vs [16.57, 24.55] STILL DISJOINT
THIRDS        +24.18 +/- 6.18 | +16.96 +/- 6.71 | +21.49 +/- 7.63
hourly mean heap: 916 1002 996 1017 1052 1096 1123 1128 1144 1172 1205 1193 1234 1271
```

**The split-half result replicates for a third time and remains the
only claimable resolution.**

**THE THIRDS DID NOT REPLICATE, AND THAT MATTERS.** At r198 the thirds
read +28.42 / +25.29 / +18.29 — monotone decline. They now read
+24.18 / +16.96 / +21.49, which is not monotone. At r198 the CIs
overlapped and the monotone reading was explicitly NOT claimed. That
restraint was correct: the pattern is gone one tick later. Logged as a
worked example — an overlapping-CI pattern is a coin flip, and
publishing it would have required a retraction here.

### B. The free consistency check has nearly stopped moving — watch this

Full-run linear slope as the run extends:

```
  r196  10.20h   26.32
  r197  10.95h   25.20     delta -1.12 over 0.75h   = -1.49 / h
  r198  11.77h   24.33     delta -0.87 over 0.82h   = -1.06 / h
  r199  12.88h   23.98     delta -0.35 over 1.11h   = -0.32 / h
```

A straight line fitted through a bending curve has its slope dragged
down as data extends. That drag is **decaying fast**: -1.49, -1.06,
-0.32 per hour. In the same direction, the second-half slope is RISING
across ticks: 15.79 (r197) -> 18.69 (r198) -> 20.56 (r199).

**Read: the curve is straightening.** Per the standing rule, this is
NOT a prediction of a plateau and NOT a return to the withdrawn linear
claim. It is the one thing the brief flagged as new if it happened, so
it is recorded as a watch item, not a finding. Residual variance has
also grown (full-run CI +/- 0.77 -> +/- 1.33 despite more n), meaning
the recent hours swing wider.

**STILL WITHDRAWN AND NOT RE-ASSERTED:** "+26.3 MB/h leak" and the word
"leak". What survives is unchanged: r187 settles ~250MB above r186's
flat baseline, ceiling 6442MB on a 15.8GB box, nothing is heading
anywhere.

### C. The 905-vs-fitted question has resolved itself — do not re-base

At r195 the two-parameter refit looked clearly better. At 18 more hours
of data it largely does not:

```
                    r195 (n=28)      r199 (n=36)
mean |residual|   30.5 -> 27.2     34.9 -> 33.3
corr(occ, resid)  -0.393 -> 0.000  -0.211 -> +0.116
```

The occupancy bias in the 905 model has **weakened on its own**
(-0.393 -> -0.211), and the fitted model now over-corrects in the
opposite direction (+0.116). The refit was partly fitting noise in a
narrow occupancy range. **The r195 decision not to re-base is
vindicated.** Keep 905; keep knowing its direction.

### D. Residual: gate NOT tripped, but seven consecutive negative hours

Every hour carries n=120 g/min samples — **no grinder downtime**, so
every residual below is a real reading.

```
hour   n    median   occ%    r905    rfit
 06   120   801.8    8.6%     -26     -27
 07   120   802.0    8.9%     -22     -22
 08   120   830.0    2.3%     -54     -72
 09   120   808.1    6.4%     -39     -46
 10   120   811.1    8.9%     -13     -13
 11   120   817.5    2.2%     -67     -86
 12   120   786.9    7.5%     -50     -55
```

**GATE NOT TRIPPED** under either model: it requires two CONSECUTIVE
hours below -60, and hour 11 (-67) is followed by hour 12 (-50).
Under the fitted model hour 11 reads -86, still with no qualifying
neighbour.

Hours 06-12 are seven consecutive negatives. **This is deliberately NOT
being promoted to a finding.** The pre-registered gate is on magnitude;
inventing a run-length gate after seeing a run is how a null becomes a
result. Residuals are also autocorrelated, so a naive run test is
invalid. `corr(time, residual)` is -0.161 (was -0.145) — still no
drift. Recorded as a watch item for the next tick.

### E. `percard_dispatch_total_cap` — a FOURTH event, and it breaks the size pattern

```
 01:25     0 -> 347
 06:35   347 -> 2,116      then frozen FOUR hours
 11:37  2,116 -> 4,333 -> 6,829 -> 8,016   (~5,900 in 90s)
 12:36  8,016 -> 8,193     (+177, single 30s window)   <- NEW
```

The r198 characterisation "compressed into 60-90s with a total freeze
between" survives as to TIMING, but **the magnitudes span 30x** and
"burst" is the wrong word for a +177 step. Amended: these are brief
isolated events of widely varying size, separated by total freezes.

**NOT RETRIED, BY DESIGN:** the finish-time cross-reference. r198
proved it cannot work (~400 games finish even in a single 30s window,
and only capped/slow games emit a line). A +177 event is small enough
to plausibly be one or two games, which makes attribution more
valuable, not more tractable. **The one-line `LoopGuardLabelsFor`
wiring remains the only path. Owner call.**

### F. Standing re-measure

```
            9322 10331 11173 11921 13064
r1 casc==1  82.0  82.3  82.2  82.3  82.3
r2 loop*     4.7   4.8   4.8   4.8   4.9
r3 casc>=20  2.3   2.2   2.2   2.1   2.1
big >=300    0.8   0.8   0.8   0.7   0.7
```

- **Populations hold for a TWELFTH consecutive read.**
- **r3 x big-board overlap is now 3 in 13,064** (was 2 at 11,921, 1 at
  9,322, 0 at 2,866). Still creeping, exactly as warned. Never "zero".
- **600s deadline: 15/15 — EIGHTH consecutive full hold.** `deadlines=`
  reads token_create:11, stack_drain:2, trigger_drain:1,
  turn_phase_boundary:1 = 15, against 15 games >=590s this run. The
  same three games overrun as draws (+1433s, +801s, +271s); no new
  overruns. 0 of 13,064 reached distinct>=32; max distinct 11.
- **Grinder gaps >=180s: no new ones.** Only the known Oct-18
  13:05:18 -> 13:25:19 (1201s). Both abort strings frozen since Oct 16.
- **Rank signatures** (top-20/top-50, never top-5 alone): Winota base
  1.09%, top20 55% / top50 38%. Sorin base 0.70%, top20 25% / top50 52%.
- **Threshold sweep replicates a FIFTH time** — Sorin's exit monotone
  (15/15/0/0/0/0), Winota and Gerrard still swapping at 500/1000, and
  thr=200 again reads Winota 100%, the known artifact. Note Sorin's
  thr=20 level moved 35% -> 15% between ticks: the SHAPE replicates,
  the LEVELS are small-k noisy.
- **Stalls, Oct-18 (complete day, the quotable one): Sorin 40%
  (95/238), Winota 14%** — unchanged. Oct-19 is partial, not quoted.
- **Watcher fill rate 66.7/hour** (76 files / 68.4 min), roughly double
  r198's 34.9. Projected ~143 at next tick, under the 200 gate, so NOT
  rotated. Per (0b) this is ~88% an occupancy meter, not a stall signal.

### G. A process error worth recording

`resid2.py` reads `gmin.txt`, which I did not refresh at the start of
this tick. The residual table it printed was **last tick's data wearing
this tick's timestamp**, and it would have been published as a fresh
reading. Caught because the table ended at hour 10 when hours 11 and 12
exist. The two new hours are the only ones that moved — so the stale
run would have silently hidden the single most interesting number in
the section (hour 11 at -67).

**Rule: a script with a hardcoded input filename does not announce that
its input is stale. Refresh every input at the top of the tick, or have
the script print its input's mtime and row count.**

---

## Twentieth amendment (r201) — n=13,757 capped; THE DECELERATION WAS A STARTUP TRANSIENT

### A. I have to correct my own last three reports

At r198 and r199 I reported, twice, that the r187 heap growth was
**decelerating**, on a split-half test whose CIs were disjoint. That
characterisation is **wrong, and it was wrong for a reason I can now
name.**

This tick the falling full-run slope **stopped falling and ticked up**:

```
  r196  10.20h   26.32
  r197  10.95h   25.20     -1.49 / h
  r198  11.77h   24.33     -1.06 / h
  r199  12.88h   23.98     -0.32 / h
  r201  13.63h   24.30     +0.43 / h    <- BROKE
```

The brief said a stabilising slope would be the new finding. It is, but
not in the direction of a plateau. Dropping the first T hours and
re-splitting what remains:

```
cut      n          first half              second half         verdict
0.0h   1637   +30.96 +/- 3.24 (n=818)  +23.51 +/- 3.67 (n=819)  disjoint, gap 0.53
2.0h   1397   +28.68 +/- 4.22          +24.13 +/- 4.70          OVERLAP
3.0h   1277   +27.16 +/- 4.92          +23.88 +/- 5.47          OVERLAP
4.0h   1157   +22.32 +/- 5.70          +24.51 +/- 6.47          OVERLAP (sign REVERSES)
5.0h   1037   +13.98 +/- 6.83          +22.27 +/- 7.57          OVERLAP
6.0h    917   +19.40 +/- 8.26          +25.40 +/- 9.20          OVERLAP
```

**The deceleration exists only when the first two hours are included,
and at full span its margin had already collapsed to 0.53.** Decomposed:

```
transient  0-2h   +70.79 +/- 19.24   n= 240
steady     2h+    +23.54 +/-  1.58   n=1397   (11.6 hours)
steady     4h+    +22.02 +/-  2.16   n=1157
```

The series is a **mixture of a ~2h startup ramp and a steady state.**
Fitting one line across both produces an apparent bend. That is the
standing rule — *repeated flip-flopping on good data is the tell of a
mixture; add an axis, not more n* — and adding the age axis resolved it
in one command. r198 and r199 were measuring the transient.

### B. The age-matched control, same method, is the cleanest result in this file

Both runs, first two hours dropped, identical fit:

```
r186 (control, age 2h -> 10.8h)    -4.24 +/- 2.19 MB/h   n=1047
r187 (treatment, age 2h -> 13.6h) +23.54 +/- 1.58 MB/h   n=1397
```

Nowhere near each other. **r187 grows in steady state and r186 does
not.** This is the same conclusion as r196 but on a properly matched
control with the transient removed from both sides.

### C. What is and is not withdrawn — read this carefully

- **STILL WITHDRAWN: "+26.3 MB/h" and "~8.3 days to the 6442MB
  ceiling".** The rate was wrong and the timeline was asserted without
  a linearity test.
- **NOW WITHDRAWN: "the growth is decelerating".** It is not. That was
  an artifact of the startup transient, and I published it twice.
- **REOPENED: the substance.** A sustained +23.5 +/- 1.6 MB/h over 11.6
  hours against a flat control is the thing I originally flagged, now
  better evidenced than when I first raised it.
- **STILL NOT OFFERED: any extrapolation.** 11.6 hours of undetectable
  curvature does not license a multi-day projection, and "no
  deceleration detected" is not "proven constant" — the post-4h halves
  carry CIs of +/-5.7 and +/-6.5. Headroom is ~5,100MB on a 15.8GB box.
  **I am deliberately not dividing one by the other.** The arithmetic
  that produced "8.3 days" would produce a similar number today, and it
  would be exactly as unjustified.

**The honest shape of my error:** at r197 I withdrew a claim for a
*correct* reason (no linearity test) and replaced it with a *wrong*
characterisation (deceleration) that I then replicated twice without
testing whether the apparent bend was a mixture. Withdrawing an
overclaim does not make the replacement safe.

### D. Standing re-measure

```
            9322 10331 11173 11921 13064 13757
r1 casc==1  82.0  82.3  82.2  82.3  82.3  82.3
r2 loop*     4.7   4.8   4.8   4.8   4.9   5.0
r3 casc>=20  2.3   2.2   2.2   2.1   2.1   2.1
big >=300    0.8   0.8   0.8   0.7   0.7   0.7
```

- **Populations hold for a THIRTEENTH consecutive read.**
- **600s deadline: 17/17 — NINTH consecutive full hold.**
  `deadlines=token_create:13,stack_drain:2,trigger_drain:1,turn_phase_boundary:1`
  = 17, against 17 games >=590s this run. Two new deadline games since
  r199, both stopping cleanly; the same three historical overruns
  (+271s, +801s, +1433s) and no new ones.
- **r3 x big-board overlap unchanged at 3 in 13,757.** Never "zero".
- **Grinder gaps >=180s: still 5, none new.** Both abort strings frozen
  since Oct 16. `percard_dispatch_total_cap` frozen at **8,193** for
  ~1h15m — no fifth event.
- **Ranks:** Winota base 1.10%, top20 55% / top50 38%. Sorin base
  0.71%, top20 25% / top50 52%. Stable.
- **Stalls, Oct-18 (the quotable complete day): Sorin 40% (95/238),
  Winota 14%** — unchanged.
- **Watcher: 140 files / 112.8 min = 74.5/hour, over the 120 gate ->
  ROTATED.** New pids 413211/413213, verified single instance, 0 files,
  9 archive dirs, server pid 322631 untouched.

### E. A reporting honesty note

**The residual table is unchanged from r199 and is not a fresh
reading.** Both ticks ran inside hour 13, so no new complete hour
exists; the last complete hour is still 12. Reproduced here only so the
table is not silently absent:

```
hour   n    median   occ%    r905    rfit
 08   120   830.0    2.3%     -54     -72
 09   120   808.1    6.4%     -39     -46
 10   120   811.1    8.9%     -13     -13
 11   120   817.5    2.2%     -67     -86
 12   120   786.9    7.5%     -50     -55
```

Gate still not tripped. This is the same class of hazard as r199's
stale `gmin.txt`: an unchanged table looks like a confirmation unless
you say why it could not have changed.

## Twenty-first amendment (r202) — n=15,072 capped games, heap n=1,792

**A. POST-TRANSIENT HEAP: NO CURVATURE INSIDE THE POST-2h WINDOW. The r201 correction holds.**

```
full-run slope   26.32 (10.20h) -> 25.20 -> 24.33 -> 23.98 -> 24.30 -> 23.28 (14.92h)
  cut      n             first half            second half   verdict
  0.0h   1792   +30.08 +/- 2.85        +21.49 +/- 3.25       DISJOINT gap 2.50
  2.0h   1552   +26.49 +/- 3.56        +19.70 +/- 4.12       OVERLAP
  3.0h   1432   +24.32 +/- 4.15        +18.02 +/- 4.67       OVERLAP
  4.0h   1312   +21.78 +/- 4.84        +18.38 +/- 5.32       OVERLAP
  5.0h   1192   +18.69 +/- 5.55        +19.34 +/- 6.21       OVERLAP
  6.0h   1072   +26.88 +/- 6.62        +25.96 +/- 7.24       OVERLAP
```

**THE T=0 GAP WIDENED 0.53 -> 2.50 AND THAT IS STRUCTURAL, NOT A RETURN OF DECELERATION.**
The 0-2h transient is a FIXED block of 240 high-slope points permanently sitting in the
first half of any T=0 split. As n grows the CIs narrow while the transient's contribution
does not, so the T=0 test will drift further "disjoint" forever. **The T=0 split can never
clear and must not be quoted again.** Under the old reading I would have reported
"deceleration is back" this tick. It is not: every cut from 2h on overlaps, and at 5h and
6h the two halves are equal to within 0.7 MB/h.

**B. NEW INSTRUMENT — THE SPAN-CAPPED CONTROL. Strongest separation measured so far.**
The age-matched control still compares unequal spans (control 8.6h, treatment 12.9h).
Capping the treatment at the control's exact age range removes that asymmetry:

```
r186 control    age 2h -> 10.6h    -4.35 +/-  2.26 MB/h   n=1024
r187 treatment  age 2h -> 14.9h   +22.36 +/-  1.36 MB/h   n=1552
r187 CAPPED at the control's span (2h -> 10.6h)
                                  +25.99 +/-  2.40 MB/h   n=1030
```
Matching the spans exactly makes the separation LARGER, not smaller. The result does not
depend on the treatment run being longer.

**C. STEADY-STATE SLOPE DRIFTED DOWN BUT NOT OFF ~23.5.**
```
                 r201               r202
steady 2h+   +23.54 +/- 1.58   +22.36 +/- 1.36   (CI [21.00, 23.72])
steady 4h+   +22.02 +/- 2.16   +20.76 +/- 1.80
```
Both fell by ~1.2, comfortably inside the intervals, and 23.5 is still inside the 2h+ CI —
**but only just.** This is the pre-registered watch item from r201 and it is now at its
edge. If 2h+ reads below 21 next tick with a comparable CI, that IS a departure. **Do not
call it one yet, and do not re-open "decelerating" if it does — a lower steady slope on a
longer window is not the same claim.**

**D. THE 0-2h TRANSIENT IS UNCHANGED BECAUSE ITS WINDOW IS CLOSED — NOT BECAUSE IT
REPLICATED.** +70.79 +/- 19.24, n=240, byte-identical to r201. Same 240 observations. It
can only be re-measured on a NEW process start.

**E. STATUS OF EVERY CLAIM — unchanged from r201.** Still withdrawn: "+26.3 MB/h", "~8.3
days", "the growth is decelerating". Reopened and now better evidenced: a steady ~+22 MB/h
against a flat control. **Still not offered: any extrapolation.**

**F. THE 600s WALL — 18/18, TENTH CONSECUTIVE CLEAN READ.**
```
deadlines=token_create:14,stack_drain:2,trigger_drain:1,turn_phase_boundary:1  = 18
games >=590s this run = 18
```
The run-on set is unchanged: 2033.3s, 1400.8s, 871.0s; everything else landed in [599.1,
603.5]. The unwind leak is still exactly 3 of 18.

**G. `percard_dispatch_total_cap` — TWO NEW EVENTS, SIX TOTAL.**
```
10/19 12:36   8,016 ->  8,193   (+177)
10/19 14:29   8,193 ->  8,939   (+746)
10/19 14:46   8,939 ->  9,504   (+565)
```
Two medium events 17 minutes apart is a new sub-pattern — previous events were separated
by hours of total freeze. Still invisible to `guardtop=`: the top-5 is all
`percard_dispatch_depth_cap` (zone_change 135,583 / permanent_ltb 105,854 / creature_dies
102,084 / permanent_etb 29,392 / graveyard_leave 28,329), so a guard at 9,504 can never
enter. **(6b) remains the only path to attribution.**

**H. POPULATIONS — FOURTEENTH CONSECUTIVE HOLD, n=15,072.**
```
            9322 10331 11173 11921 13064 13757 15072
r1 casc==1  82.0  82.3  82.2  82.3  82.3  82.3  82.2
r2 loop*     4.7   4.8   4.8   4.8   4.9   5.0   5.0
r3 casc>=20  2.3   2.2   2.2   2.1   2.1   2.1   2.1
big >=300    0.8   0.8   0.8   0.7   0.7   0.7   0.7
```
r3 x big overlap 6 (was 3). Maxima unchanged: board 3,058, cascades 555, capped 449.5s,
SLOW GAME 3,102.6s. 0 of 15,072 reached distinct>=32; max distinct 11.

**I. SPREAD RESIDUAL — TWO GENUINELY NEW COMPLETE HOURS. Gate NOT tripped.**
```
hour   n    median   occ%   r_spread
 08   120   830.0    2.3%     -54
 09   120   808.1    6.4%     -39
 10   120   811.1    8.9%     -13
 11   120   817.5    2.2%     -67
 12   120   786.9    7.5%     -50
 13   120   780.5    8.9%     -44    NEW
 14   120   786.1    7.9%     -48    NEW
mean |residual| 35.6 spread / 43.4 naive
```
Every hour n=120 — no grinder downtime. Hour 15 is partial (16 samples), not judged.
**Hours 06-14 are now NINE consecutive negatives**, up from seven. The gate is on
MAGNITUDE (two consecutive below -60) and only hour 11 qualifies. **Still a watch item.
Do not invent a run-length gate after seeing a longer run.**

**J. THRESHOLD SWEEP — SIXTH REPLICATION. Sorin's monotone exit holds.**
```
  thr      n    winota top20   sorin top20   gerrard top20
   20   4211         70%           20%            0%
   50   1173         80%           15%            0%
  100    594         95%            0%            0%
  200    329        100%            0%            0%
  500    130         75%            0%           25%
 1000     54         55%            0%           45%
```
Winota/Gerrard swapped direction again at thr=500 and 1000 (70/30 -> 75/25, 50/50 ->
55/45). **The shape replicates; the levels are small-k noise. Still no winner named.**
All-capped ranks: Winota base 0.88% top20 55% / top50 42%; Sorin base 0.68% top20 25% /
top50 50%.

**K. OCT-18 STALLS UNCHANGED** — Sorin 95/238 (40%), Winota 33/238 (14%). Oct-19 reads 57%
at n=192 and is PARTIAL — not quoted.

**L. GRINDER GAPS >=180s: still 5.** Newest remains 10/18 13:05:18 -> 13:25:19 (1201s).
Both abort strings frozen (22 / 3,628, last 10/16 17:35).

**M. CAUGHT BEFORE PUBLISHING — A STALE LABEL ON A SCRIPT.** `r194.py` prints the
unexplained class under the heading "r1 plain, board<300, cascades==1", but its `grp()`
assigns `r1 plain` as the RESIDUAL bucket, not `cascades==1`. A third row appeared this
tick (14.3s, turns=80, board=59, sw=43, **casc=2**) which the heading would have let me
report as the class growing to three. **It is not a member. The (5b) class is still exactly
2** (150.4s and 114.3s, both cascades==1). A script's printed description is not its
filter — read the code, not the header.

## Twenty-second amendment (r203) — n=15,741 capped games, heap n=1,878

**A. THE r202 WATCH ITEM IS WITHDRAWN. The "steady-state slope is drifting down" was a
LENGTHENING-WINDOW ARTIFACT — the same error as the retired T=0 split, one level in.**

At r202 I reported the steady 2h+ slope falling 23.54 -> 22.36 and flagged it as a live
watch item "at its edge". This tick it read 22.11, which looked like the drift continuing.
It is not a drift. **Each tick's published figure reproduces EXACTLY from today's data at
that tick's own span:**

```
                recomputed from TODAY'S log        published
  r201  2h -> 13.60h   +23.67 +/- 1.58  n=1394      23.54
  r202  2h -> 14.92h   +22.36 +/- 1.36  n=1552      22.36   <- exact
  r203  2h -> 15.63h   +22.07 +/- 1.27  n=1636      (new)
```

No new data produced the change. **The number fell purely because the window got longer**,
and the window gets longer every tick by construction. I was comparing two fits over
DIFFERENT spans and reading the difference as change over time. **A statistic tracked
tick-over-tick must have its window held fixed, or you are measuring your window, not the
system.** This is the third instrument in three ticks to fail on exactly this axis (the
T=0 split, the "unchanged" transient, now this).

**B. REPLACEMENT INSTRUMENT — FIXED-WIDTH TRAILING BLOCKS. Underpowered; says no change.**
```
  age  2.0- 5.0h   +28.68 +/- 10.79 MB/h  n=360
  age  5.0- 8.0h   +16.20 +/- 11.70 MB/h  n=360
  age  8.0-11.0h   +22.98 +/- 12.65 MB/h  n=360
  age 11.0-14.0h   +35.15 +/- 13.02 MB/h  n=360
  age 14.0-15.6h   +38.31 +/- 32.06 MB/h  n=198  (partial)
```
**No block differs significantly from any other** — every interval overlaps every other.
**The instrument cannot resolve anything smaller than roughly a 2x change**, because the
~500MB sawtooth over a 3h span gives a +/-11-13 MB/h standard error. That is a real limit,
not a result: a 3h block is too short to say anything useful here.

**PREDICTION MADE IN WRITING BEFORE RUNNING IT, AND IT WAS WRONG.** I predicted "roughly
equal blocks with wide CIs". The CIs are wide and nothing is significant, so the
inferential call held — but the point estimates are not roughly equal (16.2 to 38.3) and
they run UPWARD over the last three blocks, not flat. **Do not read that as acceleration.**
It is three overlapping intervals. Recording the miss because the ordering was the part I
would have quoted if it had agreed with me.

**C. THE SPAN-CAPPED CONTROL IS FROZEN BY CONSTRUCTION — +25.99 +/- 2.40, n=1030,
IDENTICAL TO r202. THAT IS NOT A REPLICATION.** Its window is defined by r186's span,
which is fixed, so it cannot move until a new control run exists. Same closed-window
caveat as the 0-2h transient (+70.79 +/- 19.24, n=240, unchanged for the third tick).

**D. THE FINDING ITSELF IS UNTOUCHED BY ANY OF THIS.**
```
r186 control    age 2h -> 10.6h    -4.35 +/-  2.26 MB/h   n=1024
r187 treatment  age 2h -> 15.6h   +22.11 +/-  1.26 MB/h   n=1638
r187 CAPPED to the control's own span   +25.99 +/- 2.40   n=1030
```
Every post-2h cut of the drop-first-T sweep still OVERLAPS (2h/3h/4h/5h/6h), and at 5h and
6h the halves are equal to within 0.7. **Still withdrawn: "+26.3 MB/h", "~8.3 days",
"decelerating". Still not offered: any extrapolation.**

**E. THE 600s WALL — 20/20, ELEVENTH CONSECUTIVE CLEAN READ.**
```
deadlines=token_create:16,stack_drain:2,trigger_drain:1,turn_phase_boundary:1  = 20
games >=590s this run = 20
```
Two new, both clean (599.5, 599.8). The run-on set is still the SAME 3 (2033.3s, 1400.8s,
871.0s); everything else landed in [599.1, 603.5]. The unwind leak is 3 of 20.

**F. POPULATIONS — FIFTEENTH CONSECUTIVE HOLD, n=15,741.** 82.2 / 5.0 / 2.1 / 0.7.
r3 x big overlap 6. 0 of 15,741 reached distinct>=32; max distinct 11.

**TWO MAXIMA MOVED AND BOTH ARE CORRECTIONS TO STANDING TEXT:**
- **Peak board 3,058 -> 4,720.** This is the number item (17) leans on; quote 4,720.
- **Regime 2's maximum duration 18.4s -> 29.9s.** Standing item (5) says "regime 2's max is
  18.4s but nearly all are under 2.6s". **The 18.4 figure is superseded.** The shape of the
  claim survives — the median is still 0.30s — but the stated maximum was wrong within one
  tick of being written down, which is what a max does on a heavy tail.

**G. RESIDUAL TABLE UNCHANGED, AND IT COULD NOT HAVE CHANGED.** r202 ran at 15:08Z and r203
at 15:51Z — **both inside clock hour 15**, which is still partial (102 of ~120 samples).
The last complete hour is still 14. Hours 06-14 remain nine consecutive negatives; only
hour 11 (-67) is below the gate and the gate needs TWO consecutive. **NOT TRIPPED.** Every
complete hour n=120, so no grinder downtime.

**H. THRESHOLD SWEEP — SEVENTH REPLICATION.**
```
  thr      n    winota top20   sorin top20   gerrard top20
   20   4397         65%           20%            0%
   50   1232         80%           10%            0%
  100    627         90%            0%            0%
  200    349         95%            0%            0%
  500    135         75%            0%           20%
 1000     55         55%            0%           40%
```
Sorin's monotone exit holds a seventh time. Winota/Gerrard still swap. All-capped ranks:
Winota base 0.88% top20 55% / top50 42%; Sorin base 0.67% top20 20% / top50 48%.

**I. `total_cap` FROZEN at 9,504 since 14:46 — no seventh event.** Six events stand.
Grinder gaps >=180s still 5. Both abort strings frozen (22 / 3,628, last 10/16 17:35).
Oct-18 stalls unchanged: Sorin 95/238 (40%), Winota 33/238 (14%). Oct-19 partial, not
quoted. The (5b) unexplained class is still exactly 2.

**J. WATCHER ROTATED — and my own archive bookkeeping was wrong.** 164 files / 120.2 min =
**81.9/hour**, over the 120-file trigger. Rotated to pids 426854/426856, verified 2 procs /
0 files / server pid untouched. Fill rate by tick: 9.5 -> 30.1 -> 37.3 -> 34.9 -> 66.7 ->
74.5 -> 55.9 -> **81.9**. The r202 dip did not persist.

While rotating I found **432 capture files sitting loose at the root of
`~/stallwatch-archive`**, from an early rotation that moved contents instead of the
directory. I have been reporting "9 archive dirs" for several ticks; the true structure was
9 dirs plus a 432-file pile. Rehomed into `stallwatch-20261018T1105Z-loose/`; the archive
is now 11 directories and 0 loose files. Nothing live was touched.

## Twenty-third amendment (r204) — n=16,699 capped games, heap n=1,995

**A. THE POPULATION TABLE HAS BEEN DESCRIBED WRONG FOR FIFTEEN READS. Three errors, all in
the standing block, all confirmed by reading the scripts rather than trusting the heading.**

`regimes.py` lines 17-19 compute the three groups as **independent overlapping filters**:
```
r1=[r for r in rows if r['casc']==1]
r2=[r for r in rows if r['shape'].startswith('loop')]
r3=[r for r in rows if r['casc']>=20]
```
There is **no priority logic anywhere in the script**, and **no big-board group at all** —
that comes from `board.py`, a separate script with its own independent filter.

**ERROR 1 — "Groups MUTUALLY EXCLUSIVE: r3 > big > r2 > r1" is FALSE.** They are four
independent filters that **sum to 90.0%**, not 100%.
```
  r1 x r2 : 699 games in BOTH      r1 x big : 41
  r2 x big:   5                    r3 x big :  4
```
**ERROR 2 — "r3 x big-board overlap is 6" IS THE WRONG PAIR.** `regimes.py` prints
`overlap loop*&>=20`, which is **r2 x r3 = 6**. The actual **r3 x big overlap is 4.** The
standing warning "NEVER say exactly 0" survives — it is 4, not 0 — but the figure 6 has
been attached to the wrong pair of populations in every report that quoted it.

**ERROR 3 — under the partition the brief DESCRIBES, 14.5% of games are UNCLASSIFIED.**
```
  TRUE exclusive partition (r3 > big > r2 > r1):
  r1 77.9%  r2 4.9%  r3 2.2%  big 0.6%  UNCLASSIFIED 14.5%   (sums to 100)
```
That 14.5% bucket — cascades 2-19, board<300, non-loop — **has never appeared in any
report.** Under the exclusive reading r1 is 77.9%, not 82.3%.

**WHAT SURVIVES: the stability streak itself.** The same computation has been run every
tick and produced the same numbers, so "these shares have not moved in sixteen reads" is
still true and still meaningful. **What was wrong is the description, not the measurement.**
Future reports must say *four overlapping filters*, quote the 14.5% unclassified bucket if
they claim a partition, and attribute the overlap 6 to r2 x r3.

This is the same failure as the `r194.py` heading caught at r202 — a script's prose
description diverging from its code — but on a headline table rather than a footnote.
**Read the code of every script whose output you have been quoting, not just the one that
surprised you.**

**B. HEAP — NEW INSTRUMENT, AND IT IS THE BEST ONE IN THE FILE: GC-FLOOR REGRESSION.**

Point samples on a ~500MB sawtooth are meaningless and a fit through all of them mixes
live set with garbage. Binning to 1h and regressing the **per-bin minimum** (the GC floor)
is immune to sawtooth phase entirely:
```
  r187 treatment  TROUGH (live set)  +16.30 +/-  2.72 MB/h   n=14 bins  (2h -> 16.6h)
  r187 treatment  PEAK               +22.25 +/-  4.76 MB/h   n=14 bins
  r186 control    TROUGH              -0.20 +/- 11.59 MB/h   n= 8 bins  (2h -> 10.6h)
  r186 control    PEAK               -11.44 +/- 18.62 MB/h   n= 8 bins
  SPAN-MATCHED TROUGH (both 2h -> 10.6h):
    r186 control    -0.20 +/- 11.59      r187 treatment  +21.09 +/- 5.85
```
**THE GC FLOOR IS RISING. That is a qualitatively stronger statement than "the heap
grows" — it means memory is surviving collection, which is what retention looks like.**
Every earlier instrument measured total heap, which cannot separate a growing live set
from a faster garbage churn.

**CAVEATS THAT BIND THIS RESULT:**
- **The control arm is weak: 8 bins, +/-11.59. It is "not distinguishable from zero", NOT
  "proven flat".** The span-matched pair is disjoint but barely (15.24 vs 11.39).
- Per-bin minimum is a biased estimator of the true floor (it depends on sampling
  density), but every bin holds ~120 samples, so the bias is near-constant and the SLOPE
  is approximately unbiased. The levels are not trustworthy; the slope is.
- Peak rises faster than trough (22.25 vs 16.30), which would mean the sawtooth is also
  widening — **but those CIs overlap, so that is not claimed.**

**C. THE PARTIAL TRAILING BLOCK LOOKED LIKE THE BLOCKS SEPARATING. IT IS NOT ESTABLISHED.**
The newest block jumped to +49.01 +/- 16.40 and tested z=+3.03 against everything before
it. Three reasons it does not stand:
1. **It is a PARTIAL block (2.6h of 3h) whose endpoint is unconstrained.** The sawtooth
   check shows it **starts near a trough and ends near a peak** (last 3h: min 996, max
   1579, final sample 1465; previous 3h ended at 1077, near its trough). That alone
   inflates the fitted slope.
2. Among COMPLETE blocks only **1 of 6** pairwise comparisons separated (5h vs 11h,
   z=-2.12). The Bonferroni threshold for 6 comparisons is z~2.64. **Not significant.**
3. **The phase-immune instrument says no.** A split-half on the trough series gives first
   half +18.63 +/- 6.97, second half +11.17 +/- 7.91 — **OVERLAP, no acceleration.**

**PREDICTION RECORDED BEFORE RUNNING, AND IT WAS RIGHT THIS TIME** (noting it because the
r203 prediction was wrong): I predicted the rise was partly a sawtooth-phase artifact
because a partial block ending at an arbitrary instant has an unconstrained endpoint, while
allowing some of it might be real since the 11-14h complete block was already the highest.
Both halves of that held.

**D. THE 2h+ FIGURE WENT UP THIS TICK — 22.07 -> 22.74 — WHICH INDEPENDENTLY CONFIRMS THE
r203 WITHDRAWAL.** I withdrew "the steady slope is drifting down" last tick as a
lengthening-window artifact. It has now moved in the opposite direction on a still-longer
window. A number that wanders both ways with the window is not a trend. `r203chk.py` again
reproduces every past published figure exactly at its own span.

**E. THE 600s WALL — 20/20, TWELFTH CONSECUTIVE CLEAN READ.** No new overruns this tick;
the run-on set is still the SAME 3 (2033.3s, 1400.8s, 871.0s). The unwind leak is 3 of 20.

**F. SPREAD RESIDUAL — ONE NEW COMPLETE HOUR.**
```
hour   n    median   occ%   r_spread
 12   120   786.9    7.5%     -50
 13   120   780.5    8.9%     -44
 14   120   786.1    7.9%     -48
 15   120   765.8   10.1%     -47    NEW
mean |residual| 35.9 spread / 43.7 naive
```
**Hours 06-15 are now TEN consecutive negatives.** Gate is on MAGNITUDE (two consecutive
below -60) and only hour 11 (-67) qualifies. **NOT TRIPPED.** Every complete hour n=120, so
no grinder downtime. Hour 16 partial (99 samples), not judged.

**G. THRESHOLD SWEEP — EIGHTH REPLICATION, AND SORIN NO LONGER EXITS TO ZERO.**
```
  thr      n    winota top20   sorin top20   gerrard top20
   20   4639         70%           20%            0%
   50   1316         80%           15%            0%
  100    671         90%            5%            0%
  200    372         95%            5%            0%
  500    141         85%            5%           10%
 1000     58         65%            5%           30%
```
Sorin is still **weakly monotone** (20/15/5/5/5/5) but the "exits to 0 above thr=100"
framing is now false — he holds 5% to thr=1000. **At n=58 that 5% is ONE GAME.** This is
precisely the small-k noise the standing note warns about, and it is a useful reminder that
a streak described as "monotone and therefore robust" was always one game from losing its
cleanest feature. The monotone direction survives; "exits to zero" does not.
All-capped ranks: Winota base 0.88% top20 **60%** / top50 **44%**; Sorin base 0.67% top20
20% / top50 48%.

**H. UNCHANGED.** `total_cap` frozen at 9,504 since 14:46 (~2h, no seventh event). Grinder
gaps >=180s still 5. Both abort strings frozen (22 / 3,628, last 10/16 17:35). Oct-18
stalls Sorin 95/238 (40%), Winota 14% — Oct-19 partial, not quoted. Peak board 4,720.
Regime 2 max 29.9s. The (5b) unexplained class is still exactly 2.

**I. WATCHER NOT ROTATED.** 52 files / 57.6 min = **54.2/hour**; projected 106 at next
tick, below both triggers. Archive is 11 dirs / 0 loose files, as corrected at r203.

---

## Twenty-fourth amendment — r205 (n=17,848 capped games, run age 17.9h)

### A. THE RESIDUAL GATE HAS TRIPPED FOR THE FIRST TIME

The pre-registered gate is "a SUSTAINED residual below about −60, meaning TWO
CONSECUTIVE HOURS". Spread model, today:

```
hour   n     median g/min   occ%   r_spread
 13   120       782.0        8.9%    -44
 14   120       788.3        7.9%    -48
 15   120       767.9       10.1%    -47
 16   120       769.7        8.4%    -60   <- on the line
 17   120       733.0        9.7%    -85   <- clearly below
```

Hour 16 sits EXACTLY on the −60 threshold and hour 17 is well past it. I am
calling this TRIPPED rather than arguing the boundary: softening a
pre-registered gate after seeing the data is the same error as inventing one.

**n=120 for both hours, so the free gap detector does NOT excuse them**, and
occupancy was unremarkable (8.4% / 9.7%) — this is not a slow-game load spike.

### B. TWO LEADING EXPLANATIONS ELIMINATED

**B1. The watcher is NOT the cause.** Its fill rate this tick was a record
106.3/hour (236 files / 133.3 min), far above the ~1% overhead measured at
r192, so "do not re-litigate" did not apply — the conditions had changed.
Capture files per hour tracked the residual exactly (hour 15: 6, hour 16: 86,
hour 17: 124 vs −47 / −60 / −85). But captures TRIGGER on low throughput, so
the correlation is expected and proves nothing by itself. Lead/lag separates
cause from symptom:

```
lag   corr(capture activity[t], g/min[t+lag])
 -2   -0.657      g/min BEFORE the capture
 -1   -0.671      <- PEAK
 +0   -0.547
 +1   -0.460      g/min AFTER the capture
 +2   -0.436
 +6   -0.216
```

The coupling peaks BEFORE the capture and DECAYS after. Overhead would depress
g/min most DURING the 20s profile. This is the symptom signature.
**CAVEAT THAT BINDS IT: files are written at capture END, so lag −1 is roughly
capture ONSET, not a clean lead.** The decay after onset is the load-bearing
part, not the sign of the peak.

**B2. GC PHASE pressure is NOT the cause.** If a rising heap were costing
throughput through instantaneous collection pressure, the within-hour
heap↔g/min coupling should STRENGTHEN late. It weakens:

```
mean within-hour corr(g/min, heap), hours <=10 : -0.225  (11 hours)
mean within-hour corr(g/min, heap), hours >=14 : -0.130  ( 4 hours)
```

### C. CORRECTION TO THE SCOPE OF MY OWN TEST IN B2

**B2 rules out PHASE coupling, not LEVEL coupling, and those are different
claims.** The within-hour sawtooth can only see whether throughput dips when
the heap is instantaneously high. It cannot see whether a permanently LARGER
retained live set costs a steady amount every hour — that effect has no
within-hour signature at all, because the level barely moves inside an hour.

At the hourly level the two do move together, from hour 08 onward:

```
hour  median g/min  heap floor        hour  median g/min  heap floor
 08       830.1       924 MB           13       782.0      1010 MB
 09       808.2       931 MB           14       788.3       996 MB
 10       812.4       947 MB           15       767.9      1025 MB
 11       818.3       944 MB           16       769.7      1040 MB
 12       787.7      1015 MB           17       733.0      1090 MB
```

**This is NOT offered as evidence.** Both series trend with time over the same
16 hours, so they are obliged to correlate; with n=16 autocorrelated hours
there is no honest test available in this data. The standing "throughput is
NOT attributed to memory" therefore STANDS — but it now stands on a narrower
basis than I had been treating it as, and I had been quoting the weak
within-run corr(heap, g/min) = −0.186 as though it settled the level question.
It never addressed the level question.

### D. A PREDICTION WRITTEN DOWN BEFORE THE EVENT

A restart resolves C immediately and is the only thing that will. At restart
the heap floor resets to ~736 MB while the binary, decks and load are unchanged.

- **If the LEVEL story is right:** throughput returns to ~830 g/min and the
  residual returns to near zero, then both decay again over the following hours.
- **If it is wrong:** throughput stays near ~733 g/min at a 736 MB floor.

A restart is ALSO the only way to re-measure the 0-2h transient and the only
way to strengthen the 8-bin control arm. **I am not restarting production to
run this test** — it resolves itself on the next restart, whenever that comes.

### E. THE NEW PRIMARY INSTRUMENT HAS THE SAME LENGTHENING-WINDOW PROPERTY

The GC-floor fit runs "age 2h -> now", so `now` advances every tick and the
slope moves BY CONSTRUCTION — exactly the defect that killed four instruments
before it. It read 16.30 last tick and 15.93 this tick. **That is the window.**

```
tick   span            trough slope        n    published
r204   2h -> 16.60h   +16.30 +/- 2.72     14    +16.30 +/- 2.72   <- exact
r205   2h -> 17.87h   +15.93 +/- 2.38     15    (new)
```

`r205chk.py` reproduces r204's figure exactly at its own span. **DO NOT REPORT
THE FLOOR SLOPE AS FALLING.** Fixed-window trough fits, all overlapping:

```
   2.0h ->  8.0h   +18.01 +/-  9.31   n= 6 bins
   2.0h -> 12.0h   +17.36 +/-  4.74   n=10 bins
   8.0h -> 16.0h   +12.41 +/-  6.03   n= 8 bins
```

Span-matched control comparison is UNCHANGED and could not have changed — the
control's span is closed (−0.20 ± 11.59 vs +21.09 ± 5.85). **NOT a replication.**

### F. THE r204 PARTIAL BLOCK COMPLETED, AND THE CAUTION PAID OUT

Published at r204 as "NOT ESTABLISHED": the age 14-17h block read
**+49.01 ± 16.40** as a partial, z=+3.03. Now complete it reads
**+33.15 ± 13.34** — 16 MB/h lower. The stated reason (a partial window
starting near a trough and ending near a peak) was the correct reason.
Pairwise separation among the 5 complete blocks is still 1 of 10 at z=−2.12,
against a Bonferroni threshold of ~2.81. **Still not established.**
The new partial tail (17.0-17.9h) reads +66.19 ± 88.55 — uninformative, quoted
only so nobody later mistakes it for a signal.

### G. STANDING RE-MEASURE

- **600s wall: 23 deadlines vs 23 games >=590s — THIRTEENTH consecutive.**
  `token_create:18, stack_drain:2, trigger_drain:2, turn_phase_boundary:1`
- **Populations, SEVENTEENTH stable read (four OVERLAPPING filters, sum 89.8%):**
  r1 82.2% | r2 4.9% | r3 2.1% | big 0.6%. Overlaps r2xr3 = 6, r3xbig = 4.
- **Threshold sweep, NINTH replication — levels IDENTICAL to r204** despite n
  growing: winota 70/80/90/95/85/65, sorin 20/15/5/5/5/5, gerrard 0/0/0/0/10/30.
- Unexplained class still EXACTLY 2 (150.4s, 114.3s).
- `total_cap` frozen at 9,504; aborts frozen (22 / 3,628); EXITED 2 = 7; gaps 5.
- Maxima unchanged: board 4,720, cascades 735 (capped-parse), regime-2 dur 29.9s.
- Oct-18 stalls unchanged (Sorin 95/238 = 40%). Oct-19 partial, not quoted.
- Watcher ROTATED at 11:06Z (pids 441680/441682); archive 12 dirs, 0 loose.


---

## Twenty-fifth amendment — r206 (n=18,563 capped games, run age 18.9h)

### A. THE COLLAPSE — HOUR 18 READ −251, THREE CONSECUTIVE HOURS PAST THE GATE

```
hour   n     median g/min   occ%    r_spread
 16   120       769.7        8.4%     -60
 17   120       733.0       10.0%     -82
 18   120       478.0       19.5%    -251
 19    18       280.5         —        —     PARTIAL, NOT JUDGED
```

n=120 for hour 18 — complete, no gap. No grinder gaps >=180s anywhere in the run.
Health 200, pprof 200, all abort counters frozen, EXITED 2 still 7.

**THE CAVEAT THAT BINDS THIS, AND IT IS LARGE: hour 18 is the
HIGHEST-OCCUPANCY HOUR OF THE ENTIRE RUN (19.5% against 2-10% typical), and
item (1b) records that the 905 model's known bias direction is to make BUSY
HOURS LOOK WORSE.** The two previous high-occupancy hours read −43 (21.2%)
and −81 (19.1%), so −251 is far beyond them — but the instrument is being
read exactly where its misspecification is largest, and I am not going to
pretend otherwise. Hour 18 carried four ~600s games plus a 934.7s overrun.

Heap floors over the same window: 1025, 1040, 1090, 1121, then **1396** in
hour 19 — a +275 MB step against a ~16 MB/h trend.

### B. ELIMINATED THIS TICK

- **Goroutine leak: DEAD.** `goroutine profile: total 18`. Not a leak.
- **Grinder gap: DEAD.** Zero gaps >=180s; n=120 every hour.
- **Deck spike: NOT SUPPORTED.** Hour 18's slow games are the usual mix.

### C. A LOG LINE I HAD NEVER AGGREGATED — AND MY OWN MISREADING OF IT, CAUGHT INSIDE THE TICK

`grinder: ALLOC SPIKE` has fired **64 times all-time, 29 in this run**, and I
have never once aggregated it. It carries a per-window allocation figure, a
games count, and the detector's own rolling median of MB/game.

The rolling median sat at **14.7-15.9 MB/game for eighteen hours**, then:

```
19:02:02   vs median 51.1   (77.0 MB/game)
19:10:33   vs median 68.0  (213.7 MB/game)
```

**I FIRST READ THAT AS "PER-GAME ALLOCATION QUADRUPLED" AND CALLED IT A
BETTER EXPLANATION OF THE COLLAPSE THAN ANYTHING TESTED AT r205. THAT WAS
WRONG.** The detector prints both sides and I read past them:

```
18:37:01   alloc  7639MB / 314 games =  24.3 MB/game
18:37:31   alloc 11683MB / 229 games =  51.0
19:02:02   alloc  9081MB / 118 games =  77.0
19:10:33   alloc  9615MB /  45 games = 213.7
```

**Allocation per window is roughly FLAT at 7-12 GB. The GAMES DENOMINATOR
collapsed 314 -> 45.** MB/game is a ratio and it moved because of its
denominator. The log line literally says **"DENOMINATOR FELL — fewer games
completed, check for a SLOW GAME"** in its own text.

**So ALLOC SPIKE is a SYMPTOM of the throughput collapse, not a cause — the
same error class as the watcher at r205, found the same way.** It remains
worth aggregating as a cheap detector; it is NOT evidence of an allocation
regression, and no such regression is claimed.

### D. I NEARLY PUBLISHED A BROKEN STREAK OFF MY OWN STALE SNAPSHOT

The 600s wall has matched exactly for thirteen reads. This tick it appeared
to break: **deadlines=28 vs 27 games >=590s.** I was about to report the
first excess, with "a game is mid-unwind" as the explanation.

**It was my own error.** `slow.txt` was pulled at 19:09; the deadline total
came from a live query at 19:19; a 599.7s game logged at 19:11:18 in between.
**Two numbers from two different moments.** Re-pulled in a single snapshot:

```
deadlines=token_create:22,stack_drain:2,trigger_drain:2,turn_phase_boundary:2  = 28
games >=590s this run                                                          = 28
```

**28/28 — FOURTEENTH CONSECUTIVE. The streak holds.** (0c) says refresh every
input at the top of the tick; the sharper form is **two numbers being compared
must come from the SAME snapshot**, which a top-of-tick refresh does not
guarantee when one side is queried live later.

### E. THE GC FLOOR WENT BACK UP, CONFIRMING THE r205 WITHDRAWAL

```
r204   2h -> 16.60h   +16.30 +/- 2.72   n=14   published 16.30  <- EXACT
r205   2h -> 17.87h   +15.93 +/- 2.38   n=15
r206   2h -> 18.93h   +16.54 +/- 2.21   n=16
```

Down then up. **A number that wanders both ways with the window is not a
trend** — the same confirmation r204 gave for the r203 withdrawal, now for the
floor. Trough split-half still OVERLAPS (+21.09 ± 5.85 vs +16.93 ± 6.24).
Hour 19's 1396 MB floor is in a partial bin and NOT yet in the regression.
Minor: `r205chk.py` labels its new row "r205"; the span printed beside it is
correct, so the row is unambiguous, but the label is stale.

### F. THE WATCHER SATURATED AND TURNED ITSELF OFF

It ran 18:06:11 -> 19:07:24 (61.2 min), produced **241 files**, hit its 240-file
/ 60-capture cap and **exited**. It was blind from 19:07 until the relaunch.

**Fill rate 236/hour — but that is the INSTRUMENT'S CEILING, not a measurement
of the system.** 60 captures in 61 minutes is back-to-back capturing, which
means throughput sat under 760 almost continuously. **A saturated counter
cannot report a rate**, so the fill-rate series (… 54.2 -> 106.3 -> 236) is
right-censored at this end and must not be extrapolated.

**OPERATIONAL PROBLEM WORTH NAMING: the watcher self-terminates after ~1 hour
at this capture rate, so it goes blind exactly when conditions are worst.**
Rotated; archive now 13 dirs, 0 loose; relaunched and verified (2 procs).

### G. STANDING RE-MEASURE

- **600s wall 28/28, FOURTEENTH consecutive** (see D).
- **Populations, EIGHTEENTH stable read (four OVERLAPPING filters):**
  r1 82.3% | r2 4.9% | r3 2.1% | big 0.6%. Overlap r2xr3 = 6.
- **Threshold sweep, TENTH replication.** One level moved: thr=20 winota
  70% -> 65%. Everything else identical. Sorin still 20/15/5/5/5/5.
- `total_cap` frozen at 9,504. Aborts frozen (22 / 3,628). Gaps 5.
- Oct-18 stalls unchanged (Sorin 95/238 = 40%). Oct-19 partial, not quoted.
- Maxima unchanged.


### H. THE COLLAPSE WINDOW NAMES A SUBSYSTEM — ReplEvent

The engine has been writing allocation profiles to `/tmp/hexdek-allocprof/`
all along (**122 files**) and I have never read one. Diffing the 19:10 spike
profile against the 16:43 one (same process, same Build ID
`d463a22e`, no restart, `alloc_space` is cumulative so the diff is exact)
gives allocation IN the collapse window: 1,752.91 GB over 2.45h, which is
**14.0% of the run's lifetime allocation**. That is the uniform expectation
for every function.

```
Share of each function's LIFETIME allocation landing in the 2.45h window:

  ReplEvent.SetCount                   282.4GB /  293.3GB   96.3%   6.90x
  NewReplEvent                         191.6GB /  230.5GB   83.1%   5.96x
  ---------------------------------------------------------------------
  collectPresentInstanceIDsForSweep     61.8GB /  606.8GB   10.2%   0.73x
  applyLayer                            33.1GB /  325.9GB   10.2%   0.73x
  json.MarshalIndent                    26.1GB /  257.1GB   10.2%   0.73x
  GameState.Snapshot                    53.0GB /  522.5GB   10.2%   0.73x
  per_card.fireTrigger                  53.0GB /  523.3GB   10.1%   0.73x
  PriorityRound                         24.1GB /  239.4GB   10.1%   0.72x
  GameState.LogEvent                    31.1GB /  309.1GB   10.1%   0.72x
  cardColors                            67.7GB /  677.0GB   10.0%   0.72x
  BaseCharacteristics                  252.6GB / 2528.6GB   10.0%   0.72x
  partitionTypes                        29.0GB /  294.4GB    9.8%   0.71x
```

**EVERY OTHER FUNCTION SITS IN A 9.8-10.2% BAND. That tight clustering IS the
control** — the whole engine allocated proportionally through the window, and
exactly two functions, both in the replacement-effect event machinery, did
not. `pickReplacement` (31.95GB) also enters the window's top-16 without
appearing in the lifetime top-14.

**WHAT THIS IS: the first concrete lead on what is different about the
collapse window, and it names a subsystem.**
**WHAT THIS IS NOT: a mechanism.** Concentration is not mechanism — this is
equally consistent with one replacement-heavy game or deck running in that
window as with a regression, and `BaseCharacteristics` remains the run's
largest allocator overall by a wide margin. **No code change is proposed and
none should be until the window's games are identified.**

**NEXT STEP (cheap, next tick): identify which games ran 16:43-19:10 and
whether a replacement-effect-heavy deck is over-represented among them.** The
profiles are read-only artifacts already on disk; `go tool pprof` is NOT
installed on .206, so `scp -O` them off and diff locally.

---

## Twenty-sixth amendment — r207 (n=18,969 capped games, run age 19.9h)

### A. AN ACUTE MEMORY EVENT STARTED AT ~19:10Z. THIS IS A REGIME CHANGE, NOT THE +16 MB/h DRIFT.

Five-minute trace, clock time:

```
  time    n   g/min med   heap min   heap max   sys max
  19:00   10     283.5       1396       1593      2300
  19:10   10     316.0       1586       2024      3071
  19:20   10     367.9       2194       2569      4005
  19:30   10     286.2       2609       3071      4487
  19:40   10     414.2       2711       3155      4491
  19:50   10     423.9       3070       3401      4495
  20:00    7     406.1       3168       3499      5241
```

**The floor climbed 1396 -> 3168 MB in one hour: +1772 MB/h against the
standing +16 MB/h. The sawtooth amplitude is also collapsing** (19:55 range
328 MB vs the usual ~500) — GC still runs and recovers less each cycle.
Process RSS 4,406 MB; `GOMEMLIMIT=10GiB`; box has 10.3 GB of 15.8 GB
available, so **no OOM risk and no action taken.**

### B. IT IS NOT ONE PATHOLOGICAL GAME — GAMES KEPT COMPLETING THROUGHOUT

```
19:00:26  836,609 games        19:42:26  849,757
19:12:26  839,828              19:54:26  854,624
19:24:26  844,154              20:06:26  859,559
```

**~1,400-2,500 games completed every 6 minutes while the floor climbed and
never recovered.** Thousands of games began and ended inside the window.
**That is retention ACROSS game boundaries — leak-shaped, not a single stuck
game.** No grinder gaps; n=120 for every complete hour.

### C. THE ALLOC PROFILES NOW NAME THE SUBSYSTEM UNAMBIGUOUSLY

Diff of 19:10 -> 19:25 (15 minutes, 272.44 GB allocated, same process, same
Build ID `d463a22e`):

```
  ReplEvent.SetCount   109.66GB  40.25%
  NewReplEvent          73.43GB  26.95%
  pickReplacement       12.24GB   4.49%
                                ------
  replacement machinery           71.7%
  BaseCharacteristics   15.51GB   5.69%  <- the run's LARGEST lifetime allocator
```

**Its share rose 16.1% -> 40.3% between the r206 window and this one, and
`BaseCharacteristics` has been displaced from the top.** At r206 I wrote that
the ReplEvent concentration was "not a mechanism" and could be one
replacement-heavy deck. **(B) now rules that out** — it persists across
thousands of completed games and is intensifying.

**STILL NOT PROVEN: that these allocations are the RETAINED bytes.** An
`alloc_space` profile counts bytes allocated, not bytes still live. The
retention is inferred from the non-recovering floor, and the allocation
dominance is measured separately. **The clean next step is an `inuse_space`
profile, which names what is actually held** — read-only, and not yet taken.

### D. A NAMED TRIPWIRE, WITH THE EXTRAPOLATION CAVEAT ATTACHED

The engine's own ceiling is concrete, from its abort text:

```
grinder: aborting game at turn 10 — process heap 7231MB exceeds the 6442MB ceiling
```

That counter (`process heap`) has been **frozen at 3,628 since Oct 16** and
has not moved. At the current heap (3,476 MB) and the observed one-hour rate,
the ceiling is roughly 1.7 hours away.

**THE CAVEAT BINDS: that is a ONE-HOUR rate, and a slope only says "rising" —
an extrapolation needs "the rate is constant", which I have NOT established.
This is NOT an estimate of time-to-failure.** It is offered only because it
names a **cheap observable with a known baseline**: if the `process heap`
abort count rises above 3,628, the ceiling was reached. **Check it in READ 0
every tick.**

### E. THE GC-FLOOR REGRESSION HAS NOT SEEN THE EVENT YET — AND I NEARLY SAID IT HAD

```
r204   2h -> 16.60h   +16.30 +/- 2.72   n=14   published 16.30  <- EXACT
r205   2h -> 17.87h   +15.93 +/- 2.38   n=15
r206   2h -> 18.93h   +16.54 +/- 2.21   n=16
r207   2h -> 19.85h   +17.20 +/- 2.09   n=17
```

**My first reading was that the instrument had broken — reporting +17.20 MB/h
while the last hour ran at +1772. That was wrong.** The hourly age-bins end at
the 18-19h bin (clock 18:12-19:12, floor 1121); the 19:12-20:05 climb falls
outside the last complete bin and **is not in the fit at all.** The instrument
is **LAGGING, not broken.**

**WHAT THIS MEANS FOR NEXT TICK: the event will enter as ONE bin and will pull
the line hard. When it does, check that a single bin is not doing all the
work** — and prefer the pre-break fit (2h -> 18h: **+16.54 ± 2.21**, which
reproduces r206 exactly) as the "old regime" number. **A line fitted across a
regime change is the mixture error from r201 in a new costume.**
The split-half still prints "OVERLAP — no acceleration detected", which is
**true of its window and false of the system.** Do not quote it this tick.

### F. STANDING RE-MEASURE

- **600s wall: deadlines 29 vs 29 games >=590s — FIFTEENTH consecutive.**
  Both from ONE snapshot, per the r206 lesson.
  `token_create:23, stack_drain:2, trigger_drain:2, turn_phase_boundary:2`
- **Residual, FOUR consecutive hours past the gate and escalating:**
  hour 16 −60, 17 −82, 18 −251, **19 −451** (n=120 each, no gaps).
  The (1b) caveat still binds hour 18 (19.5% occupancy); **hour 19's occupancy
  was an ordinary 10.0%, so that caveat does NOT excuse −451.**
- **Populations, NINETEENTH stable read (four OVERLAPPING filters):**
  r1 82.3% | r2 5.0% | r3 2.1% | big 0.6%. Overlap r2xr3 = 6.
- **Threshold sweep, ELEVENTH replication.** thr=20 winota back to 70% (was
  65% at r206, 70% before) and sorin 20->15: **small-k wobble, as warned.**
- `total_cap` frozen at 9,504. Aborts frozen (22 / 3,628). Gaps 5. EXITED 2 = 7.
- **Watcher: 182 files / 46 min = 237/hour, at the instrument ceiling again
  (second consecutive tick).** Rotated before it could self-terminate;
  archive 14 dirs, 0 loose; relaunched and verified (2 procs).


### G. THE inuse PROFILE SAYS ReplEvent IS CHURN, NOT THE RETAINED SET

Taken live at 20:14Z (read-only `/debug/pprof/heap`). Total sampled live
heap **2.44 GB**:

```
  text/tabwriter.(*Writer).append    0.68GB  28.09%
  text/tabwriter.(*Writer).addLine   0.48GB  19.83%
  os.readFileContents                0.41GB  16.83%
  ReplEvent.SetCount                 0.15GB   6.10%
  NewReplEvent                       0.11GB   4.41%
  deckparser.buildCard               0.08GB   3.17%
  deckparser.CloneLibrary            0.07GB   2.73%
  ...
  runtime/pprof.writeHeapInternal    (cum)    49.62%
```

**ReplEvent is ~10% of live heap while being 71.7% of allocation. It is
CHURN — allocated and collected — NOT the retained set.** Section (C)
established that ReplEvent dominates *allocation*; it was heading toward the
inference that ReplEvent is therefore what is being retained. **That
inference is now refuted by direct measurement and is withdrawn before it
was ever published.** The (C) caveat — "an `alloc_space` profile counts bytes
allocated, not bytes still live" — was the right caveat and it paid out
within the hour.

**What holds the live set is still unidentified.** The honest answer is that
this profile cannot say, because **~1.21 GB of its own 2.44 GB is
`runtime/pprof.writeHeapInternal` and its tabwriter buffers — the profiling
machinery itself.** Capturing a heap profile on this process allocates over a
gigabyte. **Measuring perturbed the thing measured**, and any future inuse
read needs that subtracted or avoided.

### H. THE WATCHER IS EXONERATED AGAIN — BY A NATURAL EXPERIMENT ALREADY IN THE DATA

(G) raised a fair worry: tabwriter buffers come from `debug=1` goroutine
dumps, which is exactly what the stall watcher does, and the watcher had been
running **saturated** (back-to-back captures, ~236/hour) right as the heap
took off. The r205 lead/lag exoneration was measured at ~106/hour, so it did
not cover this regime.

**It did not need re-running. The watcher DIED at 19:07:24** (hit its 240-file
cap, per r206) **and was not relaunched until ~19:22.** During those ~15
minutes with no watcher running at all:

```
  19:10   heap min 1586
  19:20   heap min 2194      +608 MB with ZERO watcher captures
```

**The heap climbed hardest while the watcher was dead.** That is a cleaner
control than any correlation — an unplanned 15-minute off-period in the
middle of the event. The watcher is not driving the memory event.

**Worth keeping anyway: a `debug=1` goroutine dump allocates ~1 GB on this
process.** That is a real cost of the instrument, separate from whether it
caused this event, and it argues for shortening the dump rather than raising
the caps if the watcher is ever retuned.

---

## Twenty-seventh amendment — r208 (n=19,478 capped games, run age 20.9h)

### A. CORRECTION TO r207: THE SAWTOOTH AMPLITUDE EXPANDED, IT DID NOT COLLAPSE

At r207 I wrote "sawtooth amplitude is COLLAPSING (328 MB range vs the usual
~500)". **That is wrong, and the error is a window-length mismatch — my own
recurring unit error in a new place.** The 328 came from a **5-minute**
bucket; the "usual ~500" was an **hourly** amplitude. A 5-minute window
cannot contain a full sawtooth cycle, so it will almost always read smaller.

Measured consistently, per complete hour:

```
hour   floor   peak   AMPLITUDE
 17    1090   1538      448
 18    1121   1592      471
 19    1396   3436     2040
 20    3168   4386     1218
```

**The amplitude roughly QUADRUPLED during the event.** The inference I drew
from the false reading — "GC runs and recovers less each cycle" — is
**withdrawn**. GC is moving more per cycle, not less.

### B. THE FLOOR'S 10-MINUTE INCREMENTS ARE FALLING — REPORTED, NOT PROJECTED

```
  time    floor   d(floor)/10min        time    floor   d(floor)/10min
  19:00    1396                         20:10    3388     +220
  19:10    1586     +190                20:20    3580     +192
  19:20    2194     +608                20:30    3684     +104
  19:30    2609     +415                20:40    3813     +129
  19:40    2711     +102                20:50    3896      +83
  19:50    3070     +359                21:00    3832      -64   PARTIAL n=18
  20:00    3168      +98
```

Hourly: **+1772 (h19) -> +874 (h20).**

**I AM NOT CALLING THIS DECELERATION.** "The growth is decelerating" is the
single claim I got most wrong in this investigation — told to Josh three
times before r201 showed it was a startup transient. What is different here
is that these are **raw observed increments, not a fitted line**, which is
what made the r201 version an artifact. **What is NOT different: a floor over
a 10-minute bucket is a biased minimum over ~20 samples, the newest bucket is
partial, and two hours is not a trend.** The observation stands; no
projection is offered and none should be inferred.

### C. THE TRIPWIRE METRIC IS HeapAlloc, NOT sys — I ALMOST COMPARED THE WRONG NUMBER

READ 0 showed `sys=6176MB` against a "6442MB ceiling" and that looked
alarming. **It is the wrong comparison.** `showmatch.go:2309` prints
`float64(m.HeapAlloc)/1e6` against `maxProcessHeap`:

```go
case heapAbortCeiling:
    log.Printf("grinder: aborting game at turn %d — process heap %.0fMB exceeds the %.0fMB ceiling (%s) %s",
        turn, float64(m.HeapAlloc)/1e6, float64(maxProcessHeap)/1e6, ...)
```

**The ceiling reads HeapAlloc — the grinder line's `heap=` field.** Current
heap 4,065 MB, hour-20 peak 4,386 MB, ceiling 6,442 MB. `sys` is a different
quantity and is NOT what aborts. **The tripwire counter remains FROZEN AT
3,628.** Box has 9.7 GB of 15.8 GB available; nothing touched.

### D. THE DROP-LAST-BIN CHECK FIRED EXACTLY AS PREDICTED

r207 said: when the event enters the GC-floor regression, check whether one
bin carries it. It does.

```
  full 2h -> 20.9h       +28.17 +/- 13.15   n=18
  DROP LAST BIN          +17.20 +/-  2.09   n=17
  DROP LAST TWO BINS     +16.54 +/-  2.21   n=16
  pre-break 2h -> 18h    +16.54 +/-  2.21   n=16   (reproduces r206 EXACTLY)
```

**+28.17 is one bin, and including it inflates the CI more than sixfold
(±2.09 -> ±13.15). DO NOT QUOTE +28.17 AS A RATE.** The old-regime number
remains **+16.54 ± 2.21**. The last complete age-bin is still 19-20h
(clock 19:12-20:12, floor 1758) while the actual 20:50 floor is 3,896 — **the
regression is STILL lagging the event**, as at r207.

### E. THE inuse DIFF FAILED TO NAME THE LEAK — AND THE TWO REASONS ARE REUSABLE

r207's named next step was two `inuse` profiles 20+ min apart, diffed.
Taken at 20:14Z and 21:14Z. **It did not work, and I am reporting that
rather than mining it for a headline.** Top movers:

```
  text/tabwriter.(*Writer).append    +175.09MB
  text/tabwriter.(*Writer).addLine   +171.89MB
  ReplEvent.SetCount                  +22.01MB
  bytes.growSlice                     -16MB
  fireObserverETBTriggers              +6.44MB
  NewReplEvent                         -5MB
  ...202 further nodes dropped at <= 11.02MB each
```

**Reason 1 — the profiler still dominates.** The top two movers are pprof's
own tabwriter buffers, +347 MB of the +373 MB accounted. Engine-side, the
largest mover is +22 MB.

**Reason 2, AND THIS IS THE REUSABLE ONE — I diffed two point samples on a
sawtooth without matching their phase.** The reported net total (+2,203 MB)
is dominated by where in the GC cycle each capture happened to land, not by
retention. **I solved exactly this problem for the heap SERIES at r204 by
regressing the per-bin minimum, and then walked straight into it again with
profiles.**

**WHAT A WORKING VERSION NEEDS:** capture both profiles at a MATCHED sawtooth
phase (poll `heap=` and fire the capture just after a trough), with the
watcher stopped, and discount `runtime/pprof` + `text/tabwriter` nodes.
Until then, **what holds the live set remains unidentified** — and
`ReplEvent` growing only +22 MB live in an hour is further evidence it is
churn, consistent with r207.

### F. STANDING RE-MEASURE

- **600s wall: deadlines 31 vs 31 games >=590s — SIXTEENTH consecutive**,
  both from ONE snapshot.
  `token_create:25, stack_drain:2, trigger_drain:2, turn_phase_boundary:2`
- **Residual, FIVE consecutive hours past the gate, now plateauing:**
  16 −60, 17 −82, 18 −251, 19 −451, **20 −432** (n=120 each, no gaps).
  Occupancy for hour 20 was an ordinary 11.4%, so the (1b) busy-hour caveat
  does not excuse it. **Throughput has levelled ~370 g/min** while the heap
  kept climbing — the two series are still moving independently.
- **Populations, TWENTIETH stable read (four OVERLAPPING filters):**
  r1 82.3% | r2 4.9% | r3 2.1% | big 0.6%. Overlap r2xr3 = 6.
- **Threshold sweep, TWELFTH replication.** thr=50 winota 80->85, sorin
  15->10; thr=500 gerrard 10->5. **All small-k wobble at n=165 and below —
  NOT movement**, same as the r206/r207 thr=20 flip-flop.
- `total_cap` frozen at 9,504. Aborts frozen (22 / 3,628). Gaps 5. EXITED 2 = 7.
- **Watcher: 240 files in 60.6 min — AT THE CAP for the third tick running.**
  Rotated before it could self-terminate; archive 15 dirs, 0 loose;
  relaunched and verified (2 procs).


---

## Twenty-eighth amendment — r209 (n=20,038 capped games, run age 22.0h)

### A. LAST TICK'S REFUSAL TO CALL IT DECELERATION PAID OUT WITHIN ONE TICK

At r208 the 10-minute floor increments were falling (+608 down to +83) and I
reported them while explicitly declining to call it deceleration, on the
grounds that "the growth is slowing" is the claim I got most wrong in this
investigation. **They went back up.**

```
  time    floor   d/10min        time    floor   d/10min
  20:00    3168                  21:20    4444     +334
  20:30    3684     +104         21:30    4415      -29
  20:50    3896      +83         21:40    4438      +23
  21:00    3832      -64         21:50    4462      +24
  21:10    4110     +278         22:00    4553      +91
                                 22:10    4854     +301  PARTIAL n=3
```

Hourly: **+1772 (h19) -> +664 (h20) -> +721 (h21).** Not monotone. **The
increments are NOISY, not trending**, and a two-point read of them would have
produced a published claim that was wrong one hour later. The discipline held
this time; recording it because the near-miss is the evidence that the rule
is load-bearing.

### B. THE RESIDUAL HELD AT −446 IN THE QUIETEST HOUR OF THE SEQUENCE

```
hour   n     median g/min   occ%    r_spread
 18   120       478.0       19.5%    -251
 19   120       363.9       10.0%    -451
 20   120       370.1       11.4%    -432
 21   120       413.2        5.1%    -446
```

**SIX consecutive hours past the gate.** Hour 21 ran at **5.1% occupancy —
the lowest of the whole sequence** — and still read −446. Item (1b) records
that the 905 model's bias makes **quiet hours look BETTER**, so if anything
−446 is understated here. The busy-hour caveat that bound hour 18 now works
in the opposite direction and cannot explain hour 21.

**Throughput is RECOVERING while the heap climbs**: 366 (h19) -> 372 (h20)
-> 413 (h21) -> 436 (h22 partial), against floors 1396 -> 3168 -> 3832 ->
4553. **The two series continue to move independently** and should not be
described as one phenomenon.

### C. THE GC-FLOOR REGRESSION IS NOW FULLY DOMINATED BY THE BREAK

```
  full 2h -> 22.0h     +60.88 +/- 40.05   n=19   <- DO NOT QUOTE
  DROP LAST BIN        +28.17 +/- 13.15   n=18   (= r208's headline exactly)
  DROP LAST TWO        +17.20 +/-  2.09   n=17   (= r207's headline exactly)
  pre-break 2h -> 18h  +16.54 +/-  2.21   n=16   (= r206's headline exactly)
```

**Each successive dropped bin recovers the PREVIOUS TICK'S published number
exactly.** That is a clean demonstration that every new bin since the break
is its own outlier, and that the headline figure is now reporting the newest
bin rather than the system. The CI has gone ±2.21 -> ±13.15 -> ±40.05.
**The old-regime number remains +16.54 ± 2.21 and nothing else from this
instrument is quotable while the regime is broken.**
The regression also still lags: its newest complete age-bin reads floor 3388
while the live 22:10 floor is 4,854.

### D. THE PHASE-MATCHED inuse CAPTURE — BUILT, AND MY FIRST ATTEMPT SILENTLY DID NOT RUN

r208's named next step was two `inuse` profiles captured at a MATCHED
sawtooth phase. Implemented as `~/phasecap.sh` on .206: poll the grinder
line's `heap=` every 10s, fire a capture only when the value DROPS by
>=300 MB (i.e. immediately after a GC), repeat 13 minutes later. Watcher
stopped for the whole window.

**The first attempt never ran and I nearly did not notice.** I wrote the
script through a nested heredoc over ssh AND launched it in the same
backgrounded call. The heredoc quoting failed, the file was never created,
and because the write and the launch were bundled into one backgrounded
command, **the failure produced no visible error** — I only caught it by
checking for the log file afterwards. Rewritten as a local file, `scp -O`'d
across, `bash -n` syntax-checked, existence verified, and only then launched.

**THE LESSON, WHICH IS A PROCESS ONE: DO NOT BUNDLE A STEP THAT NEEDS
VERIFYING WITH A STEP THAT NEEDS BACKGROUNDING.** The backgrounding hides the
verification.

### E. STANDING RE-MEASURE

- **600s wall: deadlines 32 vs 32 games >=590s — SEVENTEENTH consecutive**,
  both from ONE snapshot.
  `token_create:26, stack_drain:2, trigger_drain:2, turn_phase_boundary:2`
- **Tripwire NOT tripped: `process heap` aborts still 3,628** (frozen since
  Oct 16). Heap 4,755 MB against the 6,442 MB HeapAlloc ceiling; hour-21
  peak 5,046 MB. Box 9.0 GB free of 15.8 GB. **No extrapolation offered.**
- **Populations, TWENTY-FIRST stable read (four OVERLAPPING filters):**
  r1 82.3% | r2 4.9% | r3 2.1% | big 0.6%. Overlap r2xr3 = 6.
- **Rank signatures:** Winota base 0.87%, top20 65%, top50 46%; Sorin base
  0.64%, top20 15%, top50 42%. Within the usual small-k wobble.
- **Threshold sweep, THIRTEENTH replication — every level IDENTICAL to r208**
  (70/85/90/95/90/75, 15/10/5/5/5/5, 0/0/0/0/5/20) despite n growing.
- `total_cap` frozen at 9,504. Aborts frozen (22 / 3,628). **Zero grinder
  gaps >=180s this run.** EXITED 2 = 7.
- Oct-18 stalls unchanged (Sorin 95/238 = 40%). Oct-19 still partial.
- **Watcher: 224 files / 56.7 min = 237/hour — at the ceiling a FOURTH tick
  running.** Rotated; archive 16 dirs, 0 loose. **Deliberately left STOPPED
  for the phase-matched capture window** and relaunched after.


### r209 ADDENDUM — THE PHASE-MATCHED DIFF LANDED, AND IT REVERSES THE WORKING STORY

Capture A 22:19:33Z (heap=4672MB), capture B 22:42:08Z (heap=5209MB), both
fired immediately after a GC drop >=300MB, watcher stopped for the whole
window. **The method worked.**

**FIRST, A NEAR-MISS ON READING THE OUTPUT.** The diff prints
`Showing nodes accounting for 365.38MB, 11.47% of 3186.65MB total`, which
reads as "only 11% accounted, 187 nodes dropped, the growth is DIFFUSE".
**That reading is wrong.** The 3,186.65MB denominator is not the net change.
The absolute totals are:

```
  profile A   3.25 GB live
  profile B   3.61 GB live
  net growth  ~368 MB   vs top-16 sum 365.38 MB
```

**The top 16 nodes are essentially ALL of the growth.**
**A PERCENTAGE IN A pprof DIFF IS NORMALISED AGAINST A DENOMINATOR THAT IS
NOT THE NET CHANGE — COMPARE THE ABSOLUTE TOTALS.**

**SECOND, WHAT THE GROWTH ACTUALLY IS:**

```
  text/tabwriter.(*Writer).append    +273.59MB
  text/tabwriter.(*Writer).addLine    +85.40MB
                                     ---------
  profiler's own buffers             +358.99MB   ~97% of all live growth

  CreateCreatureToken                 +18.51MB   <- largest ENGINE mover, new
  NewReplEvent                        +15.00MB
  os.readFileContents                 +10.60MB
  ReplEvent.SetCount                   +7.00MB
  pickReplacement                      +6.50MB
  bytes.growSlice                     -47.43MB
  huginnAdapter.IngestCoTriggers (cum)-31.83MB
  Snapshot / LogEvent / BaseCharacteristics / cardColors   all NEGATIVE
                                     ---------
  ENGINE-SIDE NET                        ~+6 MB over 23 minutes
```

**AT PHASE-MATCHED POINTS 23 MINUTES APART, THE ENGINE'S OWN LIVE SET GREW
BY ROUGHLY 6 MB.** Over the same window the reported heap floor rose by
hundreds of MB.

**THIS CUTS AGAINST THE RETENTION STORY FOR THIS WINDOW.** If the live set is
flat at matched phase, the floor's rise is not a growing live set — it is
garbage not yet collected, or the measurement apparatus, or both.

**THREE CAVEATS THAT BIND IT, AND THEY ARE LARGE:**
1. **23 minutes is a short window** against an event measured in hours.
   One window is not the event.
2. **97% of the measured growth is MY OWN INSTRUMENT.** Four heap profiles
   have now been captured on this process (r207, r208, r209a, r209b) and
   each allocates ~1 GB of tabwriter buffer. **Whether those buffers are
   RETAINED across captures is now an open question, and if they are, I have
   been adding to the heap I am measuring.** That must be checked before any
   further inuse work.
3. **It does NOT overturn the r207 natural experiment**, where the heap
   climbed +608 MB in 15 minutes with no watcher and no captures of mine.
   The underlying event is not mine. What is now in question is how much of
   the *recent* heap is my own measurement.

**NEXT STEP, AND IT IS A CONTROL, NOT MORE MEASUREMENT: stop capturing for a
full hour, then take ONE phase-matched profile and compare its tabwriter
total against r209b's.** If tabwriter has fallen back, the buffers are
transient and (2) is closed. If it has not, my own profiling is part of the
heap growth and every inuse number so far needs re-reading.

**STATUS: "retention across game boundaries" is NOT withdrawn — it rests on
the floor never recovering across thousands of completed games — but it is
now UNDER CHALLENGE from a direct measurement, and the challenge is cleaner
than the evidence for it. Do not quote either side as settled.**

---

## Twenty-ninth amendment — r210 (n=20,976 capped games, run age 23.8h)

### A. THE TRIPWIRE TRIPPED. 96 GAMES DESTROYED IN TWO SECONDS.

`process heap` aborts went **3,628 -> 3,724**, the first movement since Oct 16.
This was the pre-registered observable and it fired.

```
2026/10/19 22:24:49  aborting game at turn 20 — process heap 6444MB exceeds the 6442MB ceiling
2026/10/19 22:24:49  aborting game at turn 10 — process heap 6454MB ...
2026/10/19 22:24:49  aborting game at turn 70 — process heap 6457MB ...
   ... 96 aborts total ...
2026/10/19 22:24:51  aborting game at turn 10 — process heap 6888MB ...
   worst values seen: 6990, 7007 (x2), 7026, 7028, 7038 (x2), 7048 MB
```

**All 96 fall inside a TWO-SECOND window, 22:24:49 -> 22:24:51.** Not a
sustained condition — a brief excursion. The shape is a fast-fail loop: once
the heap is over the ceiling, every game aborts at its first turn checkpoint,
so the grinder burns through ~96 games in two seconds before the heap drops
back. **Those 96 games produced no valid result.**

### B. THE SAMPLED `heap=` SERIES NEVER SAW IT — EVERY PEAK I HAVE QUOTED IS A LOWER BOUND

Per-minute maxima from the grinder line around the burst:

```
   22:23  max 5122      22:24  max 4961   <- 96 ABORTS HERE, at 6444-7048MB
   22:25  max 5229      22:26  max 5367
```

**The sampler read 4,961 MB in the very minute the abort path was reading
6,444-7,048 MB.** The grinder line samples every ~30 s; `CheckGameDeadline`'s
sibling heap guard calls `runtime.ReadMemStats` at turn boundaries. **A
two-second, +2 GB excursion is invisible to a 30-second sampler.**

**THIS BINDS EVERY HEAP NUMBER IN THIS DOCUMENT:**
- **PEAKS are LOWER BOUNDS** — short spikes are missed entirely.
- **FLOORS are UPPER BOUNDS** — a true post-GC trough is equally brief and
  equally likely to be sampled late.
- **AMPLITUDE is therefore understated at BOTH ends.**
The floor-slope work survives (a slowly-rising floor is still a rising floor)
but **no peak or amplitude figure should be treated as the real extreme**,
including the r208 "amplitude expanded ~4x" correction, which was itself a
correction of an earlier error and is still measured on this sampler.

### C. THE CAPTURE-RETENTION CONTROL IS ANSWERED — FROM DATA ALREADY ON DISK

r209 left an open worry: four heap profiles had been taken, each allocating
~1 GB of tabwriter, and **97% of r209's measured live growth was that
tabwriter.** If those buffers were RETAINED, I had been inflating the heap I
was measuring. The planned control was a fresh capture after a no-capture
hour. **It was not needed.** The floor series already tests it:

```
   20:00  floor 3168
   20:10  floor 3388   <- r207 capture      next bin +192
   21:10  floor 4110   <- r208 capture      next bin +334
   22:10  floor 4587   <- r209a capture     next bin  +38
   22:40  floor 5209   <- r209b capture     next bin  +34
   23:50  floor 5402
```

**No ~1 GB step at any capture point, and the TOTAL floor rise across all
four captures is +2,234 MB — less than the 4 GB that four retained buffers
would require on their own.** The rise is smooth, and it was already underway
before any capture existed (1396 -> 3168 during h19->h20, zero captures).

**THE BUFFERS ARE TRANSIENT. The r209 caveat (2) CLOSES and the r209
engine-side reading — live set ~flat at matched phase — STANDS.**

### D. I DECLINED TO RUN THE NAMED CONTROL, DELIBERATELY

The tick's instruction was to take one fresh phase-matched profile. **I did
not, and would not have even without (C).** Current heap 5,601 MB with
hour-23 peaks at 5,940 MB against a 6,442 MB ceiling: headroom at peak is
~500 MB and a capture costs ~1 GB. **Taking it would probably have caused
another abort burst and destroyed another ~100 games.**

"Read-only profiles are fine" was written when the ceiling was far away. It
is not a standing permission once the headroom is smaller than the
instrument. **An observation that costs production work is not free just
because it is read-only.**

### E. A SECOND THRESHOLD IS NOW IN RANGE, AND IT IS NOT THE SAME ONE

```
  HeapAlloc (what aborts)   5,601 MB   ceiling 6,442 MB   <- TRIPPED at 22:24
  sys                       8,804 MB   GOMEMLIMIT 10,240 MB
  process RSS / box         7.7 GB used of 15.8 GB, 8.1 GB available
```

**`sys` is a different quantity from the abort metric** (r208 correction) —
but it has its own limit, `GOMEMLIMIT=10GiB`, and at 8,804 MB it is inside
1.5 GB of it. Approaching GOMEMLIMIT makes Go's collector run continuously
rather than fail, which would show up as throughput loss, not aborts.
**Watch both, and do not conflate them.** No extrapolation offered for either.

### F. STANDING RE-MEASURE

- **600s wall: deadlines 33 vs 33 games >=590s — EIGHTEENTH consecutive**,
  both from ONE snapshot.
  `token_create:26, turn_phase_boundary:3, stack_drain:2, trigger_drain:2`
- **Residual, EIGHT consecutive hours past the gate:**
  18 −251, 19 −451, 20 −432, 21 −446, **22 −400, 23 −428** (n=120 each).
  **Throughput has levelled ~390-435 g/min** while the floor kept rising.
- **GC floor: the drop-last-bin nesting holds for a third tick.**
  full `+124.62 ± 59.51` -> drop one `+94.42 ± 53.24` -> drop two `+60.88 ±
  40.05` (= r209's headline) -> pre-break `+16.54 ± 2.21`. **Only +16.54 is
  quotable.**
- **Populations, TWENTY-SECOND stable read (four OVERLAPPING filters):**
  r1 82.3% | r2 4.9% | r3 2.1% | big 0.6%. Overlap r2xr3 = 6.
- **Threshold sweep, FOURTEENTH replication.** Larger wobble than usual at
  the small-k end: thr=1000 winota 75->90 and gerrard 20->5 at **n=69**;
  thr=500 gerrard 5->0 at n=175. **Small-k noise, NOT movement** — the
  standing warning covers exactly this.
- `total_cap` frozen at 9,504. `exceeds budget` frozen at 22. **Zero grinder
  gaps this run.** EXITED 2 = 7.
- **Watcher: capped out at 242 files and had EXITED again.** Rotated and
  relaunched, verified 2 procs; archive 17 dirs, 0 loose.

