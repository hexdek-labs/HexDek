# CR §108.4 Spell-Controller Property Test — R60

**Date:** 2026-05-27
**Branch:** `dev/cr-108-4-controller-property-r60`
**Test file:** `internal/gameengine/cr_108_4_controller_property_r60_test.go`

## The invariant

> CR §108.4a — "A spell's controller is, by default, the player who put it
> on the stack."
> CR §601.2a — "the spell becomes the topmost object on the stack ... and
> its controller is the player who cast it."

Every cast pipeline in the engine must stamp `StackItem.Controller =
seatIdx` (the caster) when pushing onto `gs.Stack`. The Controller value
MUST diverge from `card.Owner` whenever a non-owner casts the card —
that's the entire point of the §108.4 distinction. The two are
load-bearing for downstream:

- **Triggered-ability scoping** — "whenever you cast a spell" reads
  `caster_seat == perm.Controller`. A misattributed Controller fires
  the wrong player's triggers (Aetherflux Reservoir gains life for the
  owner instead of the caster; Storm Crow ETB-effect routes wrong).
- **Mana payment** — the cast pipeline drains the caster's pool. If
  Controller defaulted to owner, the wrong mana pool would resolve
  spell effects (X-cost burn, Fireball-style damage).
- **Wincon attribution** — Thassa's Oracle wins the game for "you" =
  the controller. A bug here lets the owner win even though the
  caster paid life + targeted the wincon.
- **Targeting legality** — "spells your opponents control" gates
  Hostage Taker / Gilded Drake-style stealing chains. Wrong
  Controller silently misroutes the entire interaction.

## Methodology

The property test sets up a 2-seat game per case, runs each cast
pipeline, then reads `gs.EventLog` for the `stack_push` event whose
`Source` matches the card name. `PushStackItem`
(`internal/gameengine/stack.go:149`) stamps `Seat: item.Controller`
on every push event, so the post-resolution event log preserves the
push-time controller value even after `DrainStack` has cleared the
stack.

The inspection helper is `stackPushControllerForCard(gs, name)`,
which scans the event log and returns the seat of the matching
push event, or -1 if not found. This avoids the timing problem of
inspecting `gs.Stack` directly: most cast functions end with
`PriorityRound + DrainStack`, so the StackItem is gone by the time
the test returns.

## Cases covered

| # | Cast path                          | Caster ≠ Owner? | Engine fn         |
|---|------------------------------------|-----------------|-------------------|
| 1 | Hand (baseline)                    | no              | `CastSpell`       |
| 2 | Graveyard flashback                | no¹             | `CastFlashback`   |
| 3 | Graveyard escape (with exile cost) | no¹             | `CastWithEscape`  |
| 4 | Exile + ZoneCastPermission         | **yes**         | `CastFromZone`    |
| 5 | Library (Bolas's Citadel)          | no              | `CastFromZone`    |

¹ Flashback / escape are always self-cast from one's own graveyard
(zones are private). The Controller-vs-Owner distinction is exercised
explicitly by case #4 (the Hostage-Taker / Praetor's-Grasp / Etali-
exile cross-cast scenario).

### Case 4 — the discriminator

Card owned by seat 0 (printed `Owner=0`), sitting in seat 1's exile
via a `ZoneCastPermission` grant (`RequireController: 1`, free cast).
Seat 1 invokes `CastFromZone`. The property test verifies:

- `stack_push.Seat == 1` (the caster) — NOT 0 (the owner).
- `card.Owner` stays at 0 — the cross-cast does not mutate ownership.
- A regression where the cast pipeline stamped `Controller: card.Owner`
  instead of `Controller: seatIdx` would surface immediately as
  `stack_push.Seat == 0`.

The cross-check subtest (`TestCR108_4_PropertyCrossCheck_ControllerNeverDefaultsToOwner`)
parameterizes the same property across all four cast paths so any
future cast pipeline that lifts the wrong field gets caught by a
single table-driven assertion.

## Note on `CastFromZone` zone ownership

`CastFromZone` reads `seat := gs.Seats[seatIdx]` and calls
`removeFromZone(seat, card, zone)` — it expects the card to be in the
**caster's** zone, not the owner's. This is the Hostage-Taker /
Gonti-style lifecycle (exile-to-controller's-side), not the
Praetor's-Grasp lifecycle (where Praetor's Grasp puts the card in
the **owner's** exile per CR §400.7).

Praetor's Grasp's cross-cast path (`per_card/praetors_grasp.go`)
exiles to the owner's zone but registers a grant for the controller.
The cast-back flow currently leans on the controller's hat/AI to
synthesize a `CastFromZone` call — but if the card is in the owner's
exile, that call fails with `not_in_zone`. This is a real gap that
batch_p's `TestPraetorsGrasp_ExilesOppWinconAndGrantsCast` doesn't
exercise (it stops at "grant registered" without completing the
cross-cast). Flagged here for future stub-hunt follow-up — out of
scope for this PR, which is the property test, not the bugfix.

## Files

- `internal/gameengine/cr_108_4_controller_property_r60_test.go` —
  six functions: 5 per-path tests + 1 parametric cross-check.
- `docs/cr-108-4-controller-property-r60.md` (this file).

## Future work

- Wire the Praetor's Grasp completion gap above — would extend
  `CastFromZone` (or add a sibling) that scans BOTH caster AND owner
  zones, or the per_card layer that re-routes the card to the
  caster's exile before invoking.
- Extend the property test as new cast pipelines land: aftermath,
  jump-start, retrace, disturb, mayhem, warp, embalm, eternalize,
  adventure, omen — each builds a StackItem and each must satisfy
  §108.4.
