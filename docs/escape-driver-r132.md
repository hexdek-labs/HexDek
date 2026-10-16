# r132 — escape, and the audit that stopped two false positives

## The audit came first, because r128 proved the filing wrong

r128's lesson was that **a dead named entry point is not a dead
mechanic**: `CastOverload` has no callers, but overload is reachable
anyway through a branch inside `CastSpell` that asks the Hat. So before
touching anything, every remaining alt-cost candidate was checked for
*both* an inline `CastSpell` branch and a Hat hook.

`CastSpell` turns out to carry **six** of them:

```
stack.go:518  HasOverload      stack.go:591  HasBlitz
stack.go:551  HasSurge         stack.go:613  HasPrototype
stack.go:568  HasSpectacle     stack.go:736  HasBuyback
```

and `YggdrasilHat.ChooseOptionalCost` answers for `replicate`,
`overload`, `buyback`, `casualty`/`bargain` and `prototype`.

**That removes buyback — 263 decks — from the dead list.** It was ranked
second by exposure and it is fully reachable. Two rounds ago it would
have been "fixed" into a duplicate path.

What survives the audit, with zero production callers and no inline
branch:

| entry point | decks |
|---|---|
| `CastWarp` | 226 |
| `CastWithEscape` | **194** |
| `CastWithRetrace` | 116 |

Escape was taken over warp despite the smaller number because it is the
exact shape r127 already solved — cast from the graveyard — so the
driver pattern transfers whole, and because it is the one with a real
rules subtlety (below). Warp and retrace are next.

## The rule

CR **§702.138a**, read in-session rather than recalled:

> Escape represents a static ability that functions while the card with
> escape is in a player's graveyard. "Escape [cost]" means "You may cast
> this card from your graveyard by paying [cost] rather than paying its
> mana cost." Casting a spell using its escape ability follows the rules
> for paying alternative costs in rules 601.2b and 601.2f–h.

## What escape has that flashback did not

The `[cost]` includes **"exile N other cards from your graveyard"** — an
additional cost paid *out of the very zone the spell is cast from*. So a
candidate needs **N+1** cards there: itself, plus N to exile. The
enumerator checks that up front rather than letting the cast discover it
as a failure, because a failed cast is marked `lastFailed` and skipped.

Three affordability questions, not one:

1. total mana ≥ escape cost
2. **colour** — §202.1a governs the alternative cost too (r128), via
   `CanSupplyColoredPipsForCost` on the escape cost string
3. **cards** — the graveyard can pay the exile

## The exile-target policy, stated rather than hidden

`escapeExileTargets` prefers cards that are **not** themselves escape or
flashback, falling back to recursion only when nothing inert is left.
Exiling your own escape fodder to pay for an escape is strictly worse
than exiling an inert card, and declining the cast entirely would be
worse still.

Anything cleverer than that is Hat policy and belongs in its own round.

## Verification

Seven regressions driving the real enumerator (`buildCastableList`),
reusing r127's `r127Game` / `r127Offered`.

**One of the seven is red pre-change** — the one that asserts an
affordable escape card is offered at all. Four are must-never-offer
guards that pass beforehand *vacuously*, because pre-change nothing from
the graveyard was ever offered: they constrain the new code against
over-offering (no fodder, wrong colour, not enough mana, no escape
keyword) and prove nothing about the bug. Two pin the new
exile-target helper, which is new API.

Same shape as r127, and worth stating the same way rather than counting
seven.

## Still open

`CastWarp` (226 decks) and `CastWithRetrace` (116) remain unreachable,
and foretell is a different shape again — it goes through
`ForetellExile` / `NewForetellCastPermission` and a `ZoneCastPermission`
rather than a `Cast*` entry point, so it needs its own look rather than
this pattern.
