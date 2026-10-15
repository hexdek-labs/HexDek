# r126 — the cast path now matches pip TYPE, not just total (CR §202.1a)

## The ruling

> **§202.1a** — "paying that mana cost requires **matching the type of any
> colored or colorless mana symbols** as well as paying the generic mana
> indicated in the cost."
> **§107.4a** — "Colored mana in costs can be paid only with the appropriate
> color of mana."
> **§601.2h** — "Partial payments are not allowed."

Until r126 `buildCastableList` compared one integer against another:
`CalculateTotalCost` against `seat.Mana.Total()`. A mono-green board could
cast a `{U}{U}` counterspell.

## Why this is a small change now and was not before

r117 filed this as large and said pip data "would have to go on the runtime
`Card`". **That was wrong** — `Card.ManaCostString` already existed, and
r124 found that no card builder ever assigned it. With r124 shipped, the
data, the parser (`ParseCostRequirements`, four existing callers) and the
typed pool were all present and simply never joined.

## Scope: colour only, never quantity

`CanSupplyColoredPips` answers the colour question and nothing else. The
caller's existing `cost > availableMana` check still owns magnitude, and
deliberately so:

**`CalculateTotalCost` returns a REDUCED cost; the printed string has not
been reduced.** Judging generic from the printed cost would over-demand
against every cost reducer and wrongly block legal casts. Colour pips are
not reduced by cost reduction, so they are safe to read off the print.

## The carve-outs are as load-bearing as the enforcement

A false negative blocks a legal cast, which is strictly worse than the bug
it replaces. So the check declines to judge wherever it cannot be sure:

| case | why |
|---|---|
| no printed cost | tokens, engine-minted cards — nothing to match |
| pool holds **restricted** mana (§106.4a) | Food Chain / Powerstone mana may legally pay *this* spell and this function cannot tell |
| **Phyrexian** `{W/P}` | payable with 2 life (§107.4f) — **40 corpus cards** |
| **twobrid** `{2/W}` | payable with 2 generic (§107.4e) — **22 corpus cards** |
| nil pool or card | never a licence to block |

**`ParseCostRequirements` folds Phyrexian and twobrid into `Pure` on
purpose** — it was written as a *permissive payability estimate for the AI*,
where assuming you need the colour is the safe direction. For enforcement
the safe direction is the opposite one. Using `Pure` directly would have
wrongly blocked all 62 of those cards. r126 adds `CostRequirements.Soft`,
a per-slot count of pips that can be paid another way; enforcement reads
`Pure[i] - Soft[i]` and **every existing consumer keeps reading `Pure` and
is unaffected**.

True hybrids (`{W/U}`, 591 corpus cards) were already modelled correctly as
masks and are satisfied most-constrained-first, so a two-colour pip cannot
consume the only source a one-colour pip needed.

## Verification

The r117 probe — the one that *measured* this bug — has been upgraded from
characterization to regression in place, and **its header carried a claim
that was false when written**:

> "The runtime gameengine.Card has NO mana-cost string … there is nowhere to
> put {2}{R} versus {2}{U}, so the cast path structurally cannot compare
> pips against a pool."

Its constructor discarded the `cost` argument on the strength of that claim,
which is why it could only ever log. It now populates the cost and asserts.
Pre-change, driving the real `buildCastableList`:

```
r117 Blue Spell   {2}{U}    on {R}{R}{R} -> offered, want colour-legal=false
r117 Triple Blue  {U}{U}{U} on {R}{R}{R} -> offered, want colour-legal=false
```

Plus unit coverage of each carve-out and of hybrid ordering.

Twelve packages green including paritycheck and deckparser; crcite green
after pinning the new §601.2h citation (0 drifted). hexapi showed a fourth
red, `TestArchidektImport_HappyPath` — green in isolation and absent from a
second full run, i.e. one appearance in two. It is a second member of the
same Archidekt-import family as the r120 intermittent, which is weak
evidence the family is flaky rather than either test.

## ELO

This changes what every deck can cast. **Ratings from before r126 are not
comparable with ratings after it.**
