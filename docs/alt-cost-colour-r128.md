# r128 — §202.1a stops at the printed cost

## The gap

r126 put colour enforcement on the cast path: `buildCastableList` asks
`CanSupplyColoredPips` before offering a card from hand, so a mono-green
board can no longer cast `{U}{U}`.

It asks that question about the card's **printed** cost, and only that.

An alternative cost is a different cost string. CR §601.2f makes the
total cost "the mana cost **or** alternative cost (as determined in rule
601.2b), plus all additional costs … minus all cost reductions", and
§107.4a then applies to whichever one was chosen: "Colored mana in costs
can be paid only with the appropriate color of mana."

The five alternative costs taken inside `CastSpell` — overload, surge,
spectacle, blitz, prototype — each compared `EnsureTypedPool(seat).Total()`
against an integer and nothing else.

## Why passing the r126 check proves nothing about the alt cost

36 corpus cards demand **more** coloured mana of some colour in their
alternative cost than their printed cost demands:

| keyword   | cards | example                                      |
|-----------|-------|----------------------------------------------|
| prototype | 21    | Goring Warplow `{6}` → `{1}{B}`               |
| overload  | 12    | Mizzium Mortars `{1}{R}` → `{3}{R}{R}{R}`     |
| blitz     | 2     | Tenacious Underdog `{1}{B}` → `{2}{B}{B}`     |
| spectacle | 1     | Rix Maadi Reveler `{1}{R}` → `{2}{B}{R}`      |

Prototype is the extreme case and the reason the gap is not symmetrical:
**every one of the 21 prototype cards has a colourless printed cost**, so
the r126 check has no pips to object to at all, and the Hat specifically
*prefers* prototype when mana-constrained.

**274 of 1,758 live decks** hold at least one of the 36 (overload 268,
prototype 15, blitz 1).

## The fix

`CanSupplyColoredPips(pool, card)` is now a thin delegate over
`CanSupplyColoredPipsForCost(pool, costString)`, so the printed cost and
every alternative cost get one implementation and one carve-out list.

`keywordArgCostString` reads the keyword's **raw** cost string
(`"{3}{R}{R}{R}"`) where `keywordArgCostStrict` returns the 5 derived
from it. The pip detail the int form discards is the whole question.

The five gates in `CastSpell` now require colour as well as total.

### Direction of safety

These sit on an **optional** cost. Failing one declines the alternative
cost and the spell is cast normally — it never becomes a cast failure.
The cost of a false negative is "the AI did not overload", not "a legal
play was blocked".

The carve-outs are kept anyway, since declining is still a behaviour
change: an unparsed or numeric cost arg, an empty cost string, a pool
holding restricted mana (§106.4a), and Phyrexian / twobrid pips all read
as payable.

## The data does reach the code

Checked before trusting the gate, because a gate the data never reaches
is a no-op that looks like a fix. In `ast_dataset.jsonl` all five
keywords store the cost as a **string**, not a number:

```
overload  26 × ('str',)              e.g. Mizzium Skin   ['{1}{u}']
prototype 19 × ('str','int','int')   e.g. Goring Warplow ['{1}{b}', 1, 1]
blitz     14 × ('str',)              e.g. Mezzio Mugger  ['{2}{r}']
spectacle 11 × ('str',)              e.g. Drill Bit      ['{b}']
surge     11 × ('str',)              e.g. Boulder Salvo  ['{1}{r}']
```

They are **lowercase**. `ParseCostRequirements` upper-cases each symbol
before matching, so the pips are read — but that was one `strings.ToUpper`
away from being a silent no-op, and it is pinned by a test case now.

## Verification

Six regressions in `alt_cost_colour_r128_test.go`, driven through
`CastSpell` with a Hat that always says yes, so the pool's colour is the
only variable. Mana spent is the discriminator: an instant with no body
resolves inside `CastSpell`, so the stack is empty by the time it returns.

**One of the six is red pre-change** — the one that casts
`{3}{R}{R}{R}` on a pool holding one red and five colourless, which is
the full overload total so quantity can never be the reason.

Four of the remaining five are carve-out guards that pass pre-change
because pre-change nothing was ever declined; they constrain the new code
against over-blocking and prove nothing about the bug. The sixth pins the
new cost-string helper directly and is new API.

## Correction to r127's headline

r127 reported "28 of 34 `Cast*` entry points have zero production callers"
and ranked overload next on that basis. **That ranking was wrong for
overload**, and the measurement over-counts dead mechanics generally:
`CastOverload` really does have no callers, but overload is *reachable
anyway*, through an alt-cost branch inside `CastSpell` that asks
`Hat.ChooseOptionalCost`. The same is true of surge, spectacle, blitz and
prototype.

A dead named entry point is not a dead mechanic. The caller-count sweep
is a lead, not a contract — the live question is whether **any** path
reaches the mechanic, and for these five the answer was yes, with the
colour of the cost unchecked.

`CastOverload` / `CastWithOverload` remain a dead duplicate of the
`CastSpell` branch, with their own raw `seat.ManaPool -= cost` payment
that bypasses the §601.2f modifier pipeline. Left alone here; it belongs
in the dead-code ledger, not in a behaviour change.
