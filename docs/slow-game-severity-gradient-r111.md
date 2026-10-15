# r111 — (16-CHECK): my hypothesis is refuted, and issue 16 was asking the wrong question

Two results. The first retracts something I raised last tick. The second is
larger: **the pathological slow games are not long because they have more
turns. They are long because each turn costs ~24x more.** That reframes issue
16, and every number below came out of data that has been sitting in
`server.log` for four ticks.

## 1. The free-mana hypothesis — REFUTED

r110 found seven cards combining a free-cost mana ability with an unenforced
timing restriction, and I flagged — explicitly as a hypothesis, not a claim —
that unbounded free mana might be stretching games. It is not.

This question, unlike r106's, **has a real base rate**, because it asks about
a property shared by many decks rather than about one deck's scheduling:

```
DECK POOL: 1756 deck files
  holding >=1 of the seven:  91   (5.18%)
     53 Vivi Ornitier · 12 Chandra, Hope's Beacon · 9 Wall of Roots
      8 Ramos, Dragon Engine · 8 Night of the Sweets' Revenge · 4 Mana Bloom

SLOW GAMES: 847
  distinct decks appearing in any slow game: 717
    of those, holding one of the seven:  25   (3.49%)   vs 5.18% pool
  slow-game deck SLOTS holding one:       77   (2.27%)
  slow games containing >=1 such deck:    72   (8.5%)
```

Expected at the game level if membership were independent of slowness:
`1 - (1 - 0.0518)^4 ≈ 19.2%`. **Observed 8.5%.** Under-represented on all
three measures. **Hypothesis retracted.**

(Huatli, Poet of Unity appears in zero pool decks, so six of the seven carry
the test.)

## 2. The loop deck's share INVERTS with severity

r106 reported "83–85% of every slow game contains the Sorin loop deck" and
said the absolute share was undecidable without a base rate. That is still
true of the absolute share. **But a gradient inside the same dataset needs no
external denominator**, and nobody had looked:

```
     duration   games        loopdeck    one-of-seven
        0-60s     582    525 ( 90.2%)     53 (  9.1%)
      60-120s     136    120 ( 88.2%)     10 (  7.4%)
     120-240s      77     64 ( 83.1%)      4 (  5.2%)
     240-480s      27     15 ( 55.6%)      2 (  7.4%)
        >480s      25      4 ( 16.0%)      3 ( 12.0%)

THIRTY WORST slow games: loopdeck in 6/30
```

Monotonic, 90.2% → 16.0%. If the deck's mandatory §104.4b loop were *causing*
pathological length, its share would rise with duration. It falls, hard.

r106's hypothesis **(a) "the loop makes games long"** is refuted for the
pathological tail. It survives only in a weak form — the deck could still be
nudging ordinary games just past the logger's (low) SLOW threshold, and the
0–60s bucket is 69% of all slow games. r106's **(b) "the grinder
over-schedules one deck"** is now the better explanation of the 85% headline.

## 3. The actual pathology is per-turn cost, not turn count

```
                 turns                     seconds per turn
  <60s     n=582  mean 48.9   min 25 max 80     median  0.47   max  1.57
 >=480s    n= 25  mean 60.7   min 41 max 78     median 11.16   max 19.18
```

**Turn count differs by 24%. Seconds-per-turn differs by ~24x.**

The worst games are not longer games. They are games whose every turn has
become enormously expensive. That is a board-state / evaluation cost problem,
not a game-length problem, and it is the same shape as the explained freeze
class (one huge game, gc climbing, process working).

And no single deck owns it — the most frequent deck in the `>=480s` bucket
appears in 4 of 25:

```
deck                                                    >=480s    <60s
krenko_mob_boss_b2_zdog97                                4/25    13/582
sorin_of_house_markov_… (the r106 loop deck)             4/25   525/582
ashnod_flesh_mechanist_b2_tyval                          3/25     0/582
winota_joiner_of_forces_b3_kaputtzimon                   3/25    12/582
```

The Sorin deck's 4/25 against 525/582 is the gradient in one line.

## What this changes

- The **deck-list diagnostic** I have been asking Josh or Stefan to approve
  (log deck lists on every game, not only slow ones) is **worth much less
  than I claimed**. It was designed to separate r106's (a) from (b). The
  gradient already does that, for free. I am withdrawing the request as
  framed. What would actually help is **per-turn timing**, not per-game deck
  lists.
- Issue 16's framing should change from "which deck makes games long" to
  **"what makes a turn cost 11 seconds"**. That is a profiling question. There
  is no pprof on .206, but seconds-per-turn is already derivable per game from
  the existing log line, so the cheap next step is to emit turn-level timing
  or a board-size counter on slow games.
- Nothing here touches the loop guards, and filed item **(e)** still stands.

## Scope and limits, stated plainly

- Only `SLOW GAME` lines carry a deck list, so every share above is a
  share **of slow games**. There is still no base rate for the loop deck
  across all games, and I am not claiming one.
- 7 of 3,388 deck slots did not resolve to a file on disk
  (`josh/gandhi_oloro`, `hex/yuriko_v1`) — 0.2%, cannot move any conclusion.
- The pool base rate is over 1,756 deck **files**; the grinder's ELO pool is
  1,424, so the scheduling denominator is not exactly this set. The
  under-representation (2.27% and 3.49% against 5.18%) is large enough that
  this does not change the direction, but it is why I give the three measures
  rather than one.
- `>=480s` is 25 games. The s/turn separation is enormous and the buckets
  below it are monotonic, but 25 is 25.

Reproducible: `scripts/audit_slowgame_decks_r111.py`,
`audit_slowgame_buckets_r111.py`, `audit_slowgame_worst_r111.py`, each via
`python3 -I` against `~/hexdek/server.log` and `~/hexdek/data/decks` on .206.

## Lesson 63

**"No base rate is obtainable" can be true of the absolute number and false
of the question.** r106 measured an 85% correlation, correctly said it could
not be divided by anything, and stopped — and the resolving evidence was
already in the same log, in two forms neither of which needs an external
denominator: a **gradient** across severity buckets, and a **derived rate**
(seconds per turn) rather than a raw total. When you cannot get a
denominator, look for a comparison **inside** the data, and check whether the
quantity you are correlating against is even the right one.
