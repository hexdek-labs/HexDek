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
