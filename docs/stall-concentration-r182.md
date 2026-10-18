# r182 — the stalls are not diffuse. One deck is in 70% of them.

Every round so far has treated "stalls" as a systemic property of the engine.
The grinder log has been naming the decks in every slow game the whole time,
and nobody had aggregated it.

## The measurement

`grinder: SLOW GAME <secs> ... decks=[a b c d]` — 1,327 of them in the log,
2,145 minutes of slow-game time, 902 distinct decks named.

```
   752.1 min   932 games  sorin_of_house_markov // sorin_ravenous_neonate
   176.9 min    83 games  winota_joiner_of_forces
   152.5 min    39 games  krenko_mob_boss
   129.6 min    41 games  krenko_mob_boss (second copy)
    67.1 min    11 games  wan_shi_tong_all_knowing
    66.9 min     6 games  lumra_bellow_of_the_woods
    ...
```

**One deck appears in 932 of 1,327 slow games — 70%** — and carries 35% of
all attributed slow-game time.

### The 70% is a lifetime figure, and the current rate is lower

Broken down by day, the concentration is real but less extreme now than the
lifetime number suggests — the lifetime figure is inflated by a burst on
Oct 13-15 (484 on Oct 14 alone):

```
2026/10/16   109 slow games   49 involve this deck   45%
2026/10/17    93              28                     30%
2026/10/18   158              56                     35%
```

So the honest live number is **~35% of recent slow games involve one deck**,
not 70%. That is still one deck out of 902 accounting for a third of them,
which is the point — but quote 35%, not 70%, for anything forward-looking.

### Why this is not a selection artifact

Normal games log nothing, so there is no direct base rate. But the shape of
the distribution over the 902 decks that *do* appear settles it:

```
median appearances  3
mean                5.9
2nd place          83
1st place         932      <- 11x the next worst, ~310x the median
once: 219 decks   2-5: 453   6-20: 204   >20: 26
```

No plausible non-uniformity in deck selection produces a single deck at 932
when the runner-up is 83. The decks below it are also thematically coherent —
Krenko (goblin tokens, twice), Marrow Gnawer (rat tokens, twice), Brudiclad
(token copies), Winota (creature-ETB triggers). **The slow decks are the
token-swarm and trigger-cascade decks**, exactly what the CPU profiles and
goroutine dumps have been pointing at.

## What is actually in the worst deck

The list is a textbook aristocrats-plus-tokens build. Three cards matter,
oracle text verified in-session against the live oracle endpoint rather than
from memory:

- **Cruel Celebrant** — "Whenever this creature or another creature or
  planeswalker you control dies, each opponent loses 1 life and you gain 1
  life."
- **Bloodthirsty Conqueror** — "Whenever an opponent loses life, you gain
  that much life."
- **Marauding Blight-Priest** — "Whenever you gain life, each opponent loses
  1 life."

Conqueror and Blight-Priest are a **mutual trigger pair**: a life loss makes
a life gain, which makes a life loss. With three opponents the branching
factor is 3 per cycle, so one creature death expands like this:

```
1 death -> Celebrant -> 3 opponents each lose 1
        -> Conqueror fires 3x (you gain)
        -> Blight-Priest fires per gain -> 9 more life-loss events
        -> Conqueror fires 9x ...
```

**Exponential, and rules-legal.** It terminates only because life totals are
finite, so it is not a CR §104.4b mandatory-loop draw — it must be played
out. Feeding it: **Adeline, Resplendent Cathar** (a token per opponent per
attack) doubled by **Anointed Procession**, plus free sacrifice outlets
(Ashnod's Altar, Bartolomé del Presidio) turning every token into another
death trigger.

## This corrects my own r181 claim, and the correction matters

`docs/dispatch-depth-cap-r181.md` argued the trigger-depth cap "is not
catching loops", from two pieces of evidence: `trigger_loop_cap` fired 7
times against the depth cap's 1,857, and the depth histogram decays
geometrically straight through the cap with depth 9 on the trend line.

**Both of those observations stand. The conclusion I drew from them was too
strong.** In this deck the cap is doing real containment work — it is the
only thing keeping an exponentially branching cascade finite in wall-clock
terms. I generalised from an aggregate to "the cap never catches anything
pathological", and here is a deck where it plainly does.

The sharper and more useful statement:

> **The depth cap is not a loop guard. It is a cascade truncator, and it is
> the wrong instrument for both of the jobs it is currently doing.**
>
> In the bulk of games it slices an ordinary distribution and silently drops
> real triggers — `creature_dies` 404 times per window, which is the
> aristocrats family not firing after a wipe. In this deck it contains a
> legal exponential cascade by producing a **wrong but fast** game result.
>
> Raising the cap makes this deck hang. Lowering it makes more games wrong.
> There is no correct value, because depth is not the thing that
> distinguishes the two cases.

## What follows

1. **The instrumentation is still the right next step**, and now has a named
   case to validate against. If this deck shows thousands of swallows in one
   game while the median capped game shows ~20, that single number separates
   "truncating a cascade" from "slicing ordinary play" and tells us whether
   one cap can ever serve both.

   **CORRECTION, same day.** I first specified this as "count attempted depth
   above the cap". **That measurement cannot exist**, and `r163`'s own file
   header (`trigger_depth_hwm_r163.go`) says so in capitals, with a probe: an
   infinite recursive handler fires the guard repeatedly and the maximum
   depth ever *recorded* is 9. Depth only climbs when a dispatch RUNS and
   nests; the cap returns before dispatching, so nothing can reach 10.
   Recording it would mean running the handler the cap exists to skip — a
   behaviour change, not instrumentation. I asked for it anyway because I
   wrote the brief without reading the file that had already answered it.

   What shipped instead is the nearest behaviour-neutral analogue:
   **`cascademax`**, the largest swallow count within a single top-level
   cascade (reset when nesting returns to depth 1). A mutual trigger pair
   re-enters the frontier without bound inside one cascade; a wide finite
   cascade is bounded by board size. That is the discriminator depth was
   supposed to give us.
2. **A reproducible case now exists.** Deck and seeds are in the log. Any
   repro runs off .206.
3. **Stall work is now targetable.** "Make the engine faster" was unbounded.
   "Make this cascade cheap, or detect the mutual-trigger pair and resolve it
   in closed form" is a bounded problem whose payoff is measurable against a
   known 35% of slow-game time.

## Also found, and closed

3,628 `aborting game at turn N — process heap NNNNMb exceeds the 6442MB
ceiling` aborts — a class I had never counted, because I had been grepping
only for the 22 `heap delta exceeds budget` ones. **All 3,628 are on
2026-10-16 and there are none before or since.** A one-day event, already
over. Noting it so the next person grepping `exceeds budget` knows there are
two different abort strings.
