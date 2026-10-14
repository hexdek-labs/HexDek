# r105 — The alternative-cost cast driver does not exist

**Filed, not fixed.** Date: 2026-10-14. Source: taking (N4), warp's "unimplemented half".

## The filed item was wrong, and wrong in the direction that costs more

(N4) read: *"Warp's unimplemented half (Tannuk). The delayed exile at the next end step
and the later cast-from-exile don't exist; a warped permanent just stays on the
battlefield."*

**Both of those exist.**

| piece | where | state |
|---|---|---|
| `HasWarp(card)` | `keywords_batch6.go:949` | implemented |
| `CastWarp(gs, seat, card, warpCost)` | `keywords_batch6.go:971` | implemented, routes cost through `EffectiveAlternativeCost` (r63) |
| resolve-time hook on `CostMeta["warped"]` | `stack.go:2400-2402` | implemented and wired |
| `RegisterWarpExileTrigger` — §513 end-step delayed exile | `keywords_batch6.go:1085` | implemented |
| `ExileWarpedPermanent` + cast-from-exile permission | `keywords_batch6.go` | implemented |
| `SpellWarpedThisTurn` (Axavar-style readers) | `keywords_batch6.go:1233` | implemented |

The whole §702.185a chain is built and wired. **16 tests pass, including
`TestWarp_CastResolveEndStepExile_EndToEnd`,** which drives cast → resolve → end
step → exile → cast-from-exile permission.

## What is actually missing: the front door

`CastWarp` has **zero production callers.** Measured with a go-source walk that
excludes comment lines and `_test.go` files.

It is not warp-specific. Every alternative-cost cast entry point in the engine has
zero production callers:

```
CastWarp            defined=yes  prod_calls=0
CastFlashback       defined=yes  prod_calls=0
CastWithEscape      defined=yes  prod_calls=0
CastOmen            defined=yes  prod_calls=0
CastAdventure       defined=yes  prod_calls=0
CastWithSpectacle   defined=yes  prod_calls=0
CastBuyback         defined=yes  prod_calls=0
CastOverload        defined=yes  prod_calls=0
CastWithSurge       defined=yes  prod_calls=0
CastWithCleave      defined=yes  prod_calls=0
CastWithStrive      defined=yes  prod_calls=0
CastMayhem          defined=yes  prod_calls=0
CastFromZone        defined=yes  prod_calls=0
```

(Madness, Foretell, Disturb, Jump-start, Retrace, Prowl, Freerunning, Bestow and
Mutate have no entry point defined at all.)

## The shared root is one function

Production casting is `gameengine.CastSpell(gs, seat, card, nil)` — the AI turn loop
at `tournament/turn.go:1000` and `:2016`, and `game.CastSpell` from `ws/handler.go`
and `ai/autopilot.go`. The candidate list comes from **`buildCastableList`
(`tournament/turn.go:1324`)**, which:

- iterates `seat.Hand` **only** — no graveyard, no exile;
- costs each card with `CalculateTotalCost`, i.e. **its normal mana cost**;
- has exactly one alternative branch, the MDFC back face.

No production code outside `internal/gameengine` reads `HasWarp` / `HasFlashback` /
`HasEscape` / `HasMadness` / `HasBuyback` / `HasOverload` / `HasSpectacle` /
`HasSurge` to offer a cast. Verified across `tournament`, `hat`, `hexapi`, `ws`,
`game` and `ai`.

So the alternative cost is never offered, and none of the twelve entry points can be
reached by a real game.

## Blast radius, measured

Scryfall `keywords[]` over the live corpus (`oracle-cards.json`, 38,754 cards) and
the live deck pool on .206:

```
                corpus cards       live decks containing >=1      slots
Flashback            214                   612                    1011
Overload              28                   439                     509
Warp                  37                   211                     252
Evoke                 37                   142                     239
Foretell              54                   112                     132
Eternalize            13                    96                      98
Escape                33                    84                      88
Buyback               41                    77                      88
Bestow                43                    77                      81
Miracle               19                    59                      68
Madness               63                    53                     117
... 11 more keywords
DISTINCT corpus cards with >=1 alt-cost keyword: 764
```

**1,245 of 1,758 live decks (71%) contain at least one card whose alternative cost
cannot be paid in production.**

## This is NOT the r103 shape — the data is present

r103's omen/adventure finding was a *data* gap: the AST exporter never emits the
`omen`/`adventure` keyword, so `HasOmen` and `HasAdventure` read a field that does
not exist (0 of 13 omen entries carry it).

Warp is different and the distinction decides sequencing. The AST **does** carry it:

```
scryfall keywords[] contains Warp:      37
AST entries carrying a warp Keyword:    31
  absent from the AST dataset entirely:  5  (Axavar Fate Thief, Network Marauder,
                                             Sliver Weftwinder, Spirited Simulacrum,
                                             Voidcalled Devotee)
  in the dataset but AST lacks warp:     1  (Timeline Culler)
AST warp entries not in the scryfall list: 0
```

`{"name": "warp", "args": ["{1}{w}"]}` is emitted as a real `Keyword` node with its
cost parsed. **Warp is a pure driver gap and is wirable today; omen is not, and
needs the r103 exporter round first.**

## Why this is filed and not fixed

1. **Standing rule: no production callers → stop and file.**
2. It is the r63 dead-primitive audit's "alt-cost cluster with a shared
   CastFromZone-no-driver root", which is **held for 7174n1c's greenlight**. This
   round measures it; it does not unilaterally start it.
3. Adding an alt-cost branch to `buildCastableList` changes what every deck plays —
   the same reason the r103 exporter round needs a human go/no-go. A deck with
   Flashback cards starts getting a second use out of its graveyard; 71% of the pool
   moves.
4. Building warp's "missing half" alone would have added *more* unreachable code to a
   chain that is already complete. The r103 lesson applies exactly: a green suite
   proves nothing about whether production can reach the function.

## Recurring finding, instance 27

r63 shipped commit `7cbc093b`, "Fixed all 19 alternative-cost spell-cast paths —
routed each through `EffectiveAlternativeCost`". That was a real cost-correctness
fix. **It was applied to twelve functions that nothing calls.** The cost math behind
the door was corrected while the door stayed shut.

And this is the strongest instance of the class so far: for omen (r103) the dead path
had unit tests. For warp the dead path has an **end-to-end** test, `TestWarp_Cast
ResolveEndStepExile_EndToEnd`, which passes, and which proves the mechanic works
when something calls it. Nothing does.

**Lesson 57: a green END-TO-END test is still not reachability.** An end-to-end test
reads as the strongest possible evidence that a mechanic works, and it is — it just
says nothing about whether the production entry point ever opens that path. Check the
caller count before trusting the depth of a suite.

## If greenlit, the shape of the work

- One enumeration change in `buildCastableList`: per candidate, offer each legal
  alternative cost alongside the normal one, from hand **and** from graveyard/exile
  for the zone-cast keywords.
- `ChooseCastFromHand` currently takes `[]*Card`; an alt-cost option is a
  (card, cost, entry-point) triple, so the hat interface widens.
- Sequence warp/overload/buyback (hand-only, no new zone) before
  flashback/escape/retrace (graveyard) before omen/adventure (blocked on r103).
- Expect throughput and win-rate movement across the pool; this is not a
  behaviour-neutral change.
