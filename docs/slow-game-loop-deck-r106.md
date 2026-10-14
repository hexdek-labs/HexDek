# r106 — The slow-game cluster is one deck, and it holds a mandatory loop

**Filed, not fixed.** Date: 2026-10-14. Source: the brief's standing instruction to
instrument a second >180s grinder freeze.

## Why this round happened

The freeze watch says an episode over ~180s that self-clears is ordinary, but a
second one is "worth instrumenting". r101 had one ~270s episode; this tick produced
a 270s and a 120s alongside three short ones. That is the second, so it got measured.

## The freezes are not the r88 stall class

```
FREEZE at 19943 from 21:53:45 to 21:58:15 = 270s
FREEZE at 20946 from 22:04:15 to 22:06:15 = 120s
```

PID unchanged (2809597), zero panics, health 200, one wrapper. During the 270s stall
the grinder counter sat at 19943 while the **gc counter kept climbing (197 → 201)** —
the process was working, not hung. Each stall is bracketed by a single enormous
`SLOW GAME`:

```
21:58:25  SLOW GAME 665.9s turns=52
22:06:44  SLOW GAME 790.9s turns=78
```

So these are individual games of 11 and 13 minutes, not a hang. `kill -QUIT` was
correctly not needed.

### The memory jump is real but benign

At 21:58:15 `sys` went 2057MB → 15377MB and stayed there (15404MB at 22:41). That
looks alarming next to the 2026-09-21 grinder-OOM incident, so it was checked:

```
Mem: total 15834MB   used 2164MB   free 11278MB   available 13669MB
heap across the whole window: 597–986MB, flat
```

`sys` is Go's reserved address space, expanded once by the 665s game and never
released; the live heap never moved. Not a leak, not a risk. Recorded so the next
reader does not re-investigate it.

## The actual finding: 83–85% of every slow game contains ONE deck

Across every `SLOW GAME` line in the log:

```
PRE-r104   669 slow games   Sorin/khazad_doom in 571   (85%)
POST-r104   53 slow games   Sorin/khazad_doom in  44   (83%)
```

`moxfield/sorin_of_house_markov_sorin_ravenous_neonate_b2_khazad_doom_JfUAlsri`.
Stable across 722 slow games and both sides of the r104 deploy, so it is long-standing
and nothing to do with that change.

The deck is not malformed: 99 cards plus commander, 1832 bytes against a pool mean of
1744 across 1,267 files.

## Why it is slow — hypothesis, mechanically supported

The list contains a canonical **mandatory** life-loop:

```
1 Bloodthirsty Conqueror     whenever an opponent loses life, you gain that much life
1 Vito, Thorn of the Dusk Rose    whenever you gain life, each opponent loses that much
1 Marauding Blight-Priest    whenever you gain life, each opponent loses 1 life
1 Cruel Celebrant            + Anointed Procession, Ashnod's Altar
```

Conqueror plus either drain is an unbounded loop. CR §104.4b makes a mandatory loop
that no player can break a **draw**; an engine without that detection grinds it
against whatever iteration bound exists.

**Both halves are implemented and reachable** — checked rather than assumed, per the
r105 lesson:

```
per_card/bloodthirsty_conqueror.go          dedicated handler
per_card/vito_thorn_of_the_dusk_rose.go     dedicated handler
AST  Bloodthirsty Conqueror   Triggered(opponent_loses_life -> gain_life)
AST  Vito                     Triggered(gain_life -> targeted_effect)
AST  Marauding Blight-Priest  Triggered(gain_life -> lose_life)
AST  Cruel Celebrant          Triggered(self_and_another -> untyped_effect)
```

All four `fully_parsed=True`.

## What is NOT established

**No base rate is obtainable from the log.** Only `SLOW GAME` lines carry a deck list,
so "85% of slow games" cannot be divided by "share of all games". Two hypotheses
survive and both would be defects:

- **(a)** the deck's loop makes games pathologically long;
- **(b)** the grinder over-schedules one deck out of a 1,424-deck ELO pool.

The `/api/leaderboard` endpoint does not settle it — it returns 28 rows (deck_meta),
and **every one reports `games_played: 0`**, which is a separate write-only-field
smell worth its own look.

One piece of evidence cuts against (a) being the whole story: the two worst games
(790.9s and 665.9s) **do not contain this deck**, and in the six games over 180s it
appears in only three. So it dominates the moderate tail, not the extremes.

## Why this is filed and not fixed

Filed item **(e)** says of a different Sorin interaction: *"a legal win, NOT a §104.4b
draw; do not widen the loop guards."* That is a different mechanic — cascade, not a
life loop — but loop guards are exactly where a careless fix here would land, and that
warning exists because widening them was wrong once already. A §104.4b draw-detection
change is a rules-correctness decision with outcome-digest consequences, not a
performance tweak.

The cheap, safe next step is **not** a guard change: log the deck list on every game,
not only slow ones, so the base rate exists and (a) can be separated from (b).

**Lesson 58: a 100% correlation with no denominator is not a cause.** 85% of slow
games sharing one deck is a strong signal and still cannot distinguish "this deck is
slow" from "this deck is always playing". Before acting on a dominant correlate, ask
what the measurement could not have shown you — here, every non-slow game.
