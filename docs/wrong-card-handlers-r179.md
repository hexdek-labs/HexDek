# r179 — handlers implementing the wrong card: six candidates read by hand

**Status:** measurement. **No production code changed.** This is a
card-by-card code read of the six suspects left open by
`docs/wrong-card-handlers-r178.md`, plus `High Perfect Morcant` (not on
the r178 list — it was added to this audit by the owner).

r178 established the bug class with **one** confirmed case (Golbez) and
explicitly warned that its detector's remaining flags were *suspects
whose handler bodies have NOT been read*. This document reads them.

## Summary

| # | Card | Handler file | Verdict | One-line divergence |
|---|------|--------------|---------|---------------------|
| 0 | Golbez, Crystal Collector | `golbez.go` | **WRONG CARD** (calibration, already known) | Real: artifact-ETB surveil + end-step reanimate. Handler: graveyard/exile-recast Treasures + `{2}{B}` graveyard exile-and-cast. |
| 1 | Teval, the Balanced Scale | `teval.go` | **WRONG CARD** | Real: flying, attack-mill-3 + land return, cards-leave-graveyard → Zombie Druid. Handler: begin-combat creature-count Beast token / team anthem. |
| 2 | Rootha, Mastering the Moment | `rootha.go` | **WRONG CARD** | Real: begin-combat X/X Elemental token. Handler: `{1}{U}{R}` copy an instant/sorcery and bounce itself — **that is a different real card, `Rootha, Mercurial Artist`.** |
| 3 | Berta, Wise Extrapolator | `berta_wise_extrapolator.go` | **WRONG CARD** | Real: Increment, mana on +1/+1 counters, `{X},{T}` Fractal. Handler: end-step lifegain draw + `{2}{W}{B}` lifelink Cleric. Colour identity is wrong too (GU card, WB handler). |
| 4 | Jace, Wielder of Mysteries | `jace_wielder_of_mysteries.go` | **PARTIAL — right card** | Static alt-win is implemented and **correct**. Both loyalty abilities are unimplemented (and the `// Oracle text:` comment misstates the `−8`). |
| 5 | Ms. Bumbleflower | `ms_bumbleflower.go` | **WRONG CARD** | Real: vigilance + cast-trigger (opponent draws, +1/+1 counter, flying, second-resolution draw two). Handler: second-draw-each-turn Rabbit token + `{2}{G}{W}` draw-per-token. |
| 6 | High Perfect Morcant | `morcant.go` | **WRONG CARD** | Real: Elf-ETB "each opponent blights 1" + tap-three-Elves proliferate. Handler: ETB draw-per-opponent + discard drain. |

**Five of the six assigned cards are genuinely the wrong card. One
(Jace) is the right card, incompletely implemented.**

All seven handlers are **live**: `registry.go` lines 1109, 1559, 1560,
1561, 1567, 1586, 1587 call every one of them from the global
registration path. None of this is dead code.

### Did the method catch Golbez?

Yes. Golbez was read first, before any of the six, specifically as a
calibration case, and the read identifies it as WRONG CARD without
reference to the r178 conclusion. Its fetched oracle text ("Whenever an
artifact you control enters, surveil 1…") shares no mechanic with its
handler body (treasure-on-recast-from-graveyard, `{2}{B}` exile-from-
graveyard-and-cast). Method validated.

## Method

Differences from r178, which was a text-similarity detector:

1. **Oracle text fetched live**, per card, from
   `https://hexdek.dev/api/oracle/card/{urlencoded name}`. All seven
   resolved; **zero UNRESOLVED**. Repo comments were treated as
   *claims under test*, never as a source.
2. **Handler bodies read in full** (873 lines across the seven files).
   The verdict is on what the code does, not on what the comment says.
3. CR numbers quoted below were each grepped out of
   `data/rules/MagicCompRules-20260819.txt` in-session.

A consequence worth stating plainly: r178's coverage score ranked Jace
(0.33) as a worse suspect than Ms. Bumbleflower (0.38), and Jace is the
one that turns out to be the *right* card. **Comment-vs-oracle coverage
does not order handlers by actual wrongness**, because it measures the
comment, and a comment can be wrong about a handler that is right.

---

## 0. Golbez, Crystal Collector — WRONG CARD (calibration)

**Oracle (fetched):** `{U}{B}` Legendary Creature — Human Wizard, 1/4

> Whenever an artifact you control enters, surveil 1.
> At the beginning of your end step, if you control four or more
> artifacts, return target creature card from your graveyard to your
> hand. Then if you control eight or more artifacts, each opponent
> loses life equal to that card's power.

**What the handler does** (`golbez.go`):

- `OnTrigger("spell_cast")` → if the caster is Golbez's controller and
  `ctx["cast_zone"]` is graveyard or exile, mint a Treasure token.
- `OnActivated(0)` → pick the highest-CMC nonland card from any
  graveyard (preferring opponents'), `MoveCard` it to exile, and
  register a `ZoneCastPermission` letting the controller cast it from
  exile.

**Divergence:** total. No artifact-ETB trigger, no surveil, no end-step
ability, no reanimation, no artifact count, no opponent life loss.
Instead: two abilities the card does not have, one of which is an
activated ability the AI can use. Confirms r178.

---

## 1. Teval, the Balanced Scale — WRONG CARD

**Oracle (fetched):** `{1}{B}{G}{U}` Legendary Creature — Spirit Dragon, 4/4

> Flying
> Whenever Teval attacks, mill three cards. Then you may return a land
> card from your graveyard to the battlefield tapped.
> Whenever one or more cards leave your graveyard, create a 2/2 black
> Zombie Druid creature token.

**What the handler does** (`teval.go`): a single
`OnTrigger("combat_begin")`. On the controller's own begin-combat,
de-duped once per `gs.Turn`, it counts creatures per seat and then:

- if the highest opponent creature count **strictly exceeds** the
  controller's → `CreateCreatureToken(... "Beast Token", 3/3, green)`;
- if the controller's count strictly exceeds every opponent's → append
  a `+1/+1 until_end_of_turn` `Modification` to every creature the
  controller has;
- on a tie → nothing.

**Divergence:** every element is wrong. Wrong trigger (begin-combat vs
attack / cards-leave-graveyard), wrong condition (creature-count
comparison appears nowhere on the card), wrong token (3/3 green Beast
vs 2/2 black Zombie Druid), and a team anthem the card does not have.
No mill, no land recursion, no flying grant.

Note the direction of the error: `combat_begin` is an event Teval has
**no** ability on, so this is a fabricated ability *added* rather than a
real one replaced — the real attack/graveyard triggers would still have
to come from the AST. The anthem branch is the damaging half: it is a
free, repeatable, table-wide pump the card does not print.

---

## 2. Rootha, Mastering the Moment — WRONG CARD

**Oracle (fetched):** `{2}{U}{R}` Legendary Creature — Orc Sorcerer, 3/4

> At the beginning of combat on your turn, if you've cast an instant or
> sorcery spell this turn, create an X/X blue and red Elemental
> creature token with flying and haste, where X is the greatest mana
> value among instant and sorcery spells you've cast this turn.

**What the handler does** (`rootha.go`): one `OnActivated(0)`. It finds
a target instant/sorcery on the stack (from `ctx`, else the topmost one
the controller controls), `MintSpellCopy`s it, pushes a `StackItem` with
`IsCopy: true` copying the original's targets, then calls
`BouncePermanent(gs, src, src, "hand")` to return Rootha to its owner's
hand.

**Divergence:** this is not a near-miss — **it is the text of a
different, real card.** `Rootha, Mercurial Artist` (VOW) is the
`{1}{U}{R}: Copy target instant or sorcery spell you control… Return
Rootha to its owner's hand` card. The handler is registered under the
*other* Rootha's name. The real `Mastering the Moment` has **no
activated ability at all** — it is a single begin-combat triggered
ability producing a token, none of which exists here.

This is the clearest instance of the r178 bug class after Golbez: a
correct implementation of a real card, filed under the wrong name.

---

## 3. Berta, Wise Extrapolator — WRONG CARD

**Oracle (fetched):** `{2}{G}{U}` Legendary Creature — Frog Druid, 1/4

> Increment (Whenever you cast a spell, if the amount of mana you spent
> is greater than this creature's power or toughness, put a +1/+1
> counter on this creature.)
> Whenever one or more +1/+1 counters are put on Berta, add one mana of
> any color.
> {X}, {T}: Create a 0/0 green and blue Fractal creature token and put
> X +1/+1 counters on it.

**What the handler does** (`berta_wise_extrapolator.go`):

- `OnTrigger("life_gained")` → tally the controller's lifegain into
  `seat.Flags["berta_lg_t<turn>"]`.
- `OnTrigger("end_step")` → on the controller's own end step, if the
  tally ≥ 3, draw one card; clear the tally.
- `OnActivated` → hard-deduct `4` from `seat.ManaPool`, then create a
  1/1 white Cleric token with a hand-built AST carrying `lifelink`.

**Divergence:** total, and across colour identity. Berta is a GU card;
the handler implements a WB lifegain-matters card (`{2}{W}{B}`, white
Cleric, lifelink). None of Increment, counters-to-mana, or the Fractal
is present.

Two mechanical knock-ons worth separating out:

- **Increment is unimplemented engine-wide.** `702.191a` (grepped):
  *"Increment is a triggered ability. 'Increment' means 'Whenever you
  cast a spell, if this permanent is a creature and the amount of mana
  spent to cast that spell is greater than this creature's power or
  this creature's toughness, put a +1/+1 counter on this creature.'"*
  A repo-wide grep for `Increment` in `internal/gameengine` returns
  only `IncrementDrawCount` / `IncrementCastCount` — unrelated
  counters. So even a corrected Berta handler would need the keyword.
- **Probable double mana payment.** `activation.go:782` has the engine
  deduct `ab.Cost.Mana.CMC()` from the pool at activation time, from
  the *real* AST ability. Berta's real cost is `{X}, {T}`, whose CMC is
  0 — and then `bertaActivate` deducts a hardcoded `4` at resolution.
  The handler therefore charges a cost the card does not have, on top
  of whatever the pipeline charged. (Code-evident; not runtime-proven
  here.)

---

## 4. Jace, Wielder of Mysteries — PARTIAL (right card)

**This one contradicts the r178 suspect list, and is the reason this
audit was worth running as a code read rather than a text diff.**

**Oracle (fetched):** `{1}{U}{U}{U}` Legendary Planeswalker — Jace

> If you would draw a card while your library has no cards in it, you
> win the game instead.
> +1: Target player mills two cards. Draw a card.
> −8: Draw seven cards. Then if your library has no cards in it, you
> win the game.

**What the handler does** (`jace_wielder_of_mysteries.go`): a single
`OnETB` that calls
`gameengine.RegisterJaceWielderOfMysteries(gs, perm)` and then emits a
`partial` marker naming the missing loyalty abilities.

`replacement.go:671` installs a `would_draw` replacement that applies
when the drawing seat is Jace's controller **and that seat's library is
empty**, and on apply sets `ev.Cancelled = true` and
`gs.Seats[controller].Won = true`.

That is exactly the printed static ability. CR `614.1` (grepped):
*"Some continuous effects are replacement effects… Such effects watch
for a particular event that would happen and completely or partially
replace that event with a different event."* Correct shape, correct
condition, correct outcome.

**Verdict: the handler is registered under the right card and the one
ability it implements is right.** The divergence is:

1. **Both loyalty abilities are missing.** The handler says so itself
   (`emitPartial(... "plusone_and_minuseight_loyalty_abilities_not_
   implemented_in_batch2")`), attributing it to planeswalker activation
   being engine-side work. So this is an **incompleteness**, the class
   r178 explicitly says it does not find — not a wrong card.
2. **The `// Oracle text:` comment is wrong**, which is why r178 flagged
   it. The comment claims `−8: Exile the top ten cards of your
   library. You may cast any number of nonland cards from among them
   without paying their mana costs` — that is not Jace, Wielder of
   Mysteries' `−8` under any printing. The real `−8` is *Draw seven
   cards. Then if your library has no cards in it, you win the game.*
   The comment's `+1: Draw a card` also drops the mill-two clause.

So: a false positive for *wrong card*, a true positive for *the comment
is untrustworthy*. Because the fabricated text was never implemented, it
changes no game outcome today — but it is a live trap for whoever
implements the loyalty abilities from the comment.

---

## 5. Ms. Bumbleflower — WRONG CARD

**Oracle (fetched):** `{1}{G}{W}{U}` Legendary Creature — Rabbit Citizen, 1/5

> Vigilance
> Whenever you cast a spell, target opponent draws a card. Put a +1/+1
> counter on target creature. It gains flying until end of turn. If
> this is the second time this ability has resolved this turn, you draw
> two cards.

**What the handler does** (`ms_bumbleflower.go`):

- `OnTrigger("player_would_draw")` → tally the controller's draws into
  `seat.Flags["bumble_drawn_t<turn>"]`; on the tally crossing from <2 to
  ≥2, create a 1/1 white Rabbit token.
- `OnActivated` → deduct `4` from the mana pool, count the controller's
  tokens on the battlefield, and draw that many cards.

**Divergence:** total. The real card's engine is a **cast** trigger with
a three-part effect (opponent draws, +1/+1 counter, flying) and a
second-resolution-this-turn rider; the handler has a **draw** trigger
making a token. The real card has **no activated ability whatsoever**;
the handler invents `{2}{G}{W}: draw per token`. Vigilance is not
granted.

Note the "second time" framing was preserved but reattached to the wrong
event: the real card counts *resolutions of its own ability*, the
handler counts *cards drawn*. That is the signature of a handler written
against a half-remembered card.

Mitigating, slightly: because the real Ms. Bumbleflower has no AST
`Activated` node, the fabricated activated ability is probably
unreachable via the normal `ActivateAbility` path (there is no ability
index to activate). The fabricated **trigger** is reachable and does
fire. This should be confirmed at runtime before anyone relies on it.

---

## 6. High Perfect Morcant — WRONG CARD

Not on the r178 list; added by the owner for this audit. It is wrong.

**Oracle (fetched):** `{2}{B}{G}` Legendary Creature — Elf Noble, 4/4

> Whenever High Perfect Morcant or another Elf you control enters, each
> opponent blights 1. (They each put a -1/-1 counter on a creature they
> control.)
> Tap three untapped Elves you control: Proliferate. Activate only as a
> sorcery. (Choose any number of permanents and/or players, then give
> each another counter of each kind already there.)

**What the handler does** (`morcant.go`):

- `OnETB` → draw one card per living opponent.
- `OnTrigger("card_discarded")` → if the discarder is Morcant's
  controller, `LoseLife(..., 1, ...)` on each opponent.

**Divergence:** total, and this one has a sharper edge than the others
because of the ETB suppression seam.

`etb_dispatch.go:217` skips the generic AST ETB push when
`HasETBHook(name)` is true, with the comment *"per_card owns this
card's self-ETB … skip the generic AST push or the printed ability
resolves twice."* Morcant registers `OnETB`. So on Morcant's own ETB the
**real** printed ability (each opponent blights 1) is suppressed and the
**fabricated** one (draw N) runs in its place. This is the one card in
this set where the wrong ability demonstrably *displaces* the right one
rather than merely sitting alongside it.

Both real abilities are also absent in every other respect:

- No tribal clause. The real trigger fires on *another Elf you control*
  entering, not just on Morcant; the handler has no Elf observer at all.
- No blight. CR `701.68a` (grepped): *"To 'blight N' means to put N
  -1/-1 counters on a creature you control."* `701.68b` (grepped): *"If
  a player is given the choice to blight but is unable to put N -1/-1
  counters on a creature they control (usually because they control no
  creatures), they can't choose to blight."* A grep for `blight` in
  `internal/gameengine` finds only an unrelated `blight` counter
  mechanic in `keywords_misc_test.go` / a string in a list at
  `resolve_helpers.go:4524` — **not** CR 701.68 blight-N. Unimplemented.
- No proliferate activated ability, and so no sorcery-speed
  restriction. (Proliferate itself does exist in the engine — CR
  `701.34a` grepped: *"To proliferate means to choose any number of
  permanents and/or players that have a counter, then give each one
  additional counter of each kind that permanent or player already
  has."*)
- The discard drain is pure fabrication.

---

## Cross-cutting observations

**1. The wrong ability does not always replace the right one.** Three
different seams behave three different ways, which matters for
estimating the damage:

- **ETB** (`etb_dispatch.go:217`): per_card **wins**, AST is skipped.
  Morcant's real blight trigger is actively suppressed.
- **Activated** (`activation.go:1107`): the per_card hook fires **and
  then** `ResolveEffect` runs the AST effect too — *additive*. So
  Golbez / Rootha / Berta / Bumbleflower's fabricated activated
  abilities stack on top of whatever the AST parsed, rather than
  replacing it.
- **Triggers**: `PerCardOwnsTrigger` is consulted only at specific
  sites (`combat.go:1029`, `combat.go:2658`, `keywords_misc.go:2167`,
  the ETB family). A handler registering a trigger the real card does
  not have — Teval on `combat_begin`, Bumbleflower on
  `player_would_draw` — simply **adds** an ability.

So the aggregate effect is mostly *extra* abilities, not substituted
ones, which is worse for balance than a straight swap: these cards are
strictly stronger than printed, and the AI will use what it is offered.

**2. Hardcoded mana deductions in handlers are a second bug.** Berta and
Bumbleflower both do `seat.ManaPool -= 4` inside the handler while
`activation.go:782` has the pipeline already deducting the real AST
ability's CMC. Golbez and Rootha explicitly do *not* do this (their
comments say the engine pays). The inconsistency alone says the
convention is not written down anywhere.

**3. `// Oracle text:` comments are not merely stale — some are
fabricated.** Jace's comment describes a `−8` that has never existed on
that card. Rootha's comment is a *different real card's* text. These are
not notes that drifted; they are the specifications the handlers were
written from. r178's proposal — gate `// Oracle text:` blocks against
the corpus in CI — would have caught all six of the wrong cards here,
and would have caught Jace's bad comment before it misleads the next
implementer.

## Proposals (NOT applied — owner call)

Listed in the order I would do them. None of this was implemented; per
the project rule, behaviour changes go through the owners.

1. **Highest value, lowest risk: unregister, don't rewrite.** For
   Teval, Rootha, Berta, Bumbleflower and Morcant, the handlers are
   implementing abilities the cards do not have. Removing the
   registration lines from `registry.go` immediately stops five cards
   from having fabricated abilities, and returns Morcant's ETB to the
   AST path. This is strictly closer to correct than today even if the
   AST coverage of these cards is poor, because "does less than
   printed" is a known, bounded failure mode and "does something else
   entirely" is not.
2. **Rootha is a free win for a *different* card.** `roothaActivated`
   looks like a correct implementation of `Rootha, Mercurial Artist`.
   Re-registering it under that name (after verifying that card's
   oracle text, which this audit did **not** fetch) would turn a bug
   into a supported card. Do not do this without the fetch.
3. **Jace:** fix the comment now (it costs nothing and defuses the
   trap), and file the two loyalty abilities as ordinary missing
   content, not as a wrong-card bug.
4. **Missing primitives surfaced here**, each independent of the
   handler question: CR 702.191 Increment (no implementation), CR
   701.68 blight N (no implementation; do not confuse with the existing
   unrelated `blight` counter), and an Elf-tribal "or another [type]
   you control enters" observer for Morcant.
5. **The CI gate from r178** is now better motivated: this audit found
   a 5-of-6 true-positive rate on its flag list, which is high enough
   that the remaining unexamined population is worth the fetch step.

## What this does NOT establish

- **Seven cards.** This is not a census. r178's detector covered ~a
  third of the handler corpus and this reads six of its eight flags
  plus one owner-supplied card.
- **No runtime verification.** Every claim here is from reading source
  and fetched oracle text. The suppression/additivity analysis in
  "Cross-cutting observations" is code-evident but was not confirmed by
  running a game. The Bumbleflower "activated ability probably
  unreachable" claim in particular is an inference, flagged as such.
- **`Rootha, Mercurial Artist`'s oracle text was not fetched**, so the
  claim that `rootha.go` implements it correctly is from memory and is
  explicitly NOT to be relied on — it is a lead, not a finding.
- **Coverage ≥ 0.50 handlers remain uninspected**, and handlers with no
  `// Oracle text:` block at all remain invisible to both r178 and this
  follow-up.
