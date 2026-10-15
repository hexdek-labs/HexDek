# r124 — the printed mana cost never reached the runtime Card, and five consumers were inert because of it

## The instruction

Josh: *"research what the correct ruling on those would be then focus
entirely on rule / process accuracy, no stubs, missing routes / features for
all cards."*

## The ruling, grepped from `MagicCompRules-20260819.txt` in-session

**Coloured costs (the held item r117 filed):**

> **§202.1a** — "The mana cost of an object represents what a player must
> spend from their mana pool to cast that card. Unless an object's mana cost
> includes Phyrexian mana symbols, **paying that mana cost requires matching
> the type of any colored or colorless mana symbols** as well as paying the
> generic mana indicated in the cost."

> **§107.4a** — "Colored mana in costs **can be paid only with the
> appropriate color of mana**."

> **§601.2h** — "Partial payments are not allowed. Unpayable costs can't be
> paid."

So the engine pricing a cast by mana value alone is flatly contrary to
§202.1a. There is no ambiguity to resolve — the ruling is explicit.

**Alternative costs (the held item r105 filed):**

> **§118.9** — "An alternative cost is a cost listed in a spell's text, or
> applied to it from another effect, that its controller **may pay rather
> than paying the spell's mana cost**."

> **§601.2b** — "If the spell has alternative or additional costs that will
> be paid as it's being cast … the player **announces their intentions** to
> pay any or all of those costs. **A player can't apply two alternative
> methods of casting or two alternative costs to a single spell.**"

> **§601.2f** — "The total cost is the mana cost **or alternative cost (as
> determined in rule 601.2b)**, plus all additional costs and cost increases,
> and minus all cost reductions."

So an alternative cost is chosen at announcement and *replaces* the mana cost
in the total. `buildCastableList` never announces one, which is why twelve
alt-cost entry points have no production caller.

## What the research found on the way, and it is a bug, not a plan

My own filing said pip data "would have to go on the runtime `Card` and
thread through cost calculation and payment". **That was wrong in its first
clause.** The field already exists:

```go
// ManaCostString is the raw printed mana cost ("{2}{W}{W}{U}"). The
// corpus loader populates this when available.
ManaCostString string
```

**The corpus loader does not populate it.** Every assignment of
`ManaCostString` in the entire tree is inside a `_test.go`. There are exactly
two production card builders — `deckparser.buildCard` and
`deckparser.BuildCardFromName` — and both copy Name, Types, Power, Toughness,
Colors, CMC, TypeLine and the whole MDFC back face from `CardMeta` while
dropping `CardMeta.ManaCost` on the floor.

Driven through the real loader and the real builder:

```
Lightning Bolt           CardMeta.ManaCost="{R}"     ->  Card.ManaCostString=""
Thassa, God of the Sea   CardMeta.ManaCost="{2}{U}"  ->  Card.ManaCostString=""
```

The data is there and plentiful: **32,791 of 38,754 oracle cards carry a
printed `mana_cost`**, and it survives all the way into `CardMeta`.

## Five consumers, all guarding on `!= ""`, all inert in production

| consumer | what it did instead |
|---|---|
| `countPipsOnCard` → `DevotionPipsFromManaCost` | devotion counted **0** for every real card; fell through to a `"pip:U"` Type marker that only hand-built fixtures have |
| `manaCostHasRepeatColoredPip` (Jegantha companion, "no card has more than one of the same mana symbol") | returned false for every card, so the check **passed vacuously for every deck** |
| `cost_modifiers.go:185` | early-returned; colour-specific cost modifiers never applied |
| `hat.go:301` → `ParseCostRequirements` | the AI's colour-requirement reasoning was blind |
| `hat/evaluator.go:966`, `mulligan_bottom_signals_r60.go:64` | evaluation and mulligan colour signals blind |

`ParseCostRequirements` — the pip parser the colour-enforcement work would
need — **already exists and already has four callers.** It has simply never
been handed a non-empty string outside tests.

## The fix

One assignment in each of the two builders. That is the whole change.

## Verification

Two regressions, both red pre-change, driving the real loader and the real
builder rather than asserting on the struct — because the bug was precisely
that nothing assigned the field on the path between them. The second pins
that what arrives is the **pip string and not the mana value**: a builder
writing `strconv.Itoa(md.CMC)` would satisfy "non-empty" and be useless to
every consumer, all of which parse pips. Lands must stay empty, and are
asserted to.

Twelve packages green including crcite, paritycheck and deckparser; hexapi
its three known reds.

## What this changes in live games

Four behaviours switch from silently-wrong to correct at once: devotion
starts counting, the Jegantha companion restriction starts being enforced,
colour-specific cost modifiers start applying, and the AI can see what
colours a card actually demands. All four were wrong in the same direction —
inert — so every one of them moves toward the rules.

## Where this leaves the two held items

**Colour enforcement is now much smaller than filed.** The data exists, the
parser exists, and as of r124 the data reaches the object. What remains is
the cast path itself: `buildCastableList` and payment must consult
`ParseCostRequirements` instead of comparing mana values. That is still a
change that alters what every deck can cast and makes ELO before/after
incomparable — the reason it was held stands, but "we'd have to add pip data
to the Card first" was never true.

**The alt-cost driver is unaffected by this** and remains the larger of the
two: 764 corpus cards, 1,245 of 1,758 live decks.

## Lesson

**A field's doc comment is a claim about a producer that may not exist.**
This one asserted "the corpus loader populates this when available" and
nothing ever had. The tell was available cheaply: every assignment in the
tree was in a test file. **When a field has consumers that all guard on
empty, grep for its writers before trusting any of them.**
