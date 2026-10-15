# r127 — flashback was unreachable: the driver was the missing piece, not the mechanic

## The measurement that set the target

**28 of 34 `Cast*` entry points have zero production callers.** The filed
issue said "twelve alt-cost entry points plus `CastFromZone`" — it is more
than double that, and I only know because I counted rather than trusted the
filing.

Ranked by **live-deck** exposure, not corpus size:

| keyword | corpus | decks (of 1,758) |
|---|---|---|
| **flashback** | 244 | **760** |
| overload | 29 | 530 |
| buyback | 42 | 263 |
| warp | 46 | 226 |
| escape | 41 | 194 |
| retrace | 22 | 116 |
| foretell | 62 | 113 |

Flashback is the largest single reachability gap in the engine.

## Why it was unreachable

`CastFlashback` is **fully built**: intrinsic keyword, Snapcaster-style
per-card grants, Lier-style graveyard-wide grants, cost resolution with -1
meaning "use the printed cost", payment, exile on resolve. Nothing about
the mechanic is missing.

Two lines of driver were:

1. `buildCastableList` returns early on `len(seat.Hand) == 0`. **An empty
   hand is not an empty set of castable spells.**
2. It then enumerates `seat.Hand` only, so a graveyard card is never a
   candidate under any circumstances.

And a third thing would have broken it even if the first two were fixed:
the driver's success test is `len(seat.Hand) == before`. **A flashback cast
leaves the graveyard, so a successful one reads as a failure**, gets marked
`lastFailed`, and the loop moves on.

## The change

- The early return now requires an empty hand **and** an empty graveyard.
- `flashbackCandidates` enumerates the seat's own graveyard for cards with
  the intrinsic keyword, affordable on total **and** on colour
  (`CanSupplyColoredPips` — §202.1a applies to an alternative cost too).
- Candidates carry the transient `Card.CastingFlashback`, the same shape as
  the existing `CastingBackFace`, so the driver routes to `CastFlashback`
  and measures the **graveyard** shrinking rather than the hand.
- The marker is cleared on every enumeration pass, so a card that stops
  qualifying is never left flagged for a later pass to misroute.

Grants (Snapcaster, Lier) are deliberately **not** offered here. They are
`CastFlashback`'s business, and keeping r127 to the intrinsic keyword makes
this one reachability fix rather than also changing what grants do.

## Verification

Five regressions driving the real `buildCastableList`. **Only one is red
pre-change** — the headline, a flashback card in the graveyard with an empty
hand, not offered. The other four passed pre-change **vacuously**, because
nothing from the graveyard was ever offered at all; they are guards against
the *new* code over-offering (unaffordable cost, wrong colour, a plain card
with no flashback, and the both-zones-empty short circuit). Saying so rather
than counting five reds I did not get.

Twelve packages green including paritycheck and deckparser; hexapi its three
known reds.

## ELO

This changes what decks can do — 760 of them. **Ratings from before r127 are
not comparable with ratings after it**, on top of the same caveat from r126.

## What is left

The other 27 entry points. The pattern is now established end to end:
enumerate the zone the alternative cost names, mark the candidate, route it,
and measure the zone it actually leaves. Next by exposure: **overload** (530
decks, cast from hand so no zone change), **buyback** (263), **warp** (226),
**escape** (194).
