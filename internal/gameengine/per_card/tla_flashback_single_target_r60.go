package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// tla_flashback_single_target_r60.go — R60 sweep continuation:
// remaining single-target flashback-grant cards from the systemic
// scan, all reusing the existing GrantFlashbackUntilEOT primitive.
//
// Cards covered:
//
//   - Snapcaster Mage — ETB grants flashback to target i/s in
//     controller's graveyard at printed mana cost. The canonical
//     trigger-grant card; the primitive `GrantFlashbackUntilEOT` is
//     literally named after Snapcaster's effect.
//
//   - Sphinx of Forgotten Lore — attack trigger, same shape.
//
//   - Slickshot Lockpicker — ETB trigger, same shape as Snapcaster.
//     (Plot is the AST keyword pipeline's surface; not touched here.)
//
//   - Katilda and Lier — Human-spell-cast trigger ("Whenever you
//     cast a Human spell"), same target shape. We gate on
//     caster_seat == perm.Controller AND ctx["card"] having the Human
//     subtype.
//
// All four use pickHighestCMCInstantOrSorcery from
// oneshot_flashback_grants.go for target selection (until full
// target-prompt plumbing lands).

func init() {
	registerTLAFlashbackSingleTargetR60(Global())
	AddResetHook(registerTLAFlashbackSingleTargetR60)
}

func registerTLAFlashbackSingleTargetR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Snapcaster Mage", snapcasterMageETB)
	r.OnTrigger("Sphinx of Forgotten Lore", "attacks", sphinxOfForgottenLoreAttacks)
	r.OnETB("Slickshot Lockpicker", slickshotLockpickerETB)
	r.OnTrigger("Katilda and Lier", "spell_cast", katildaAndLierSpellCast)
}

// -----------------------------------------------------------------------------
// Snapcaster Mage
// -----------------------------------------------------------------------------
//
// Oracle text ({1}{U} Creature — Human Wizard):
//
//	Flash
//	When this creature enters, target instant or sorcery card in your
//	graveyard gains flashback until end of turn. The flashback cost is
//	equal to its mana cost.
//
// Flash is the AST keyword pipeline's surface.

func snapcasterMageETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	grantFlashbackToBestGraveyardTarget(gs, perm, "snapcaster_mage_flashback_grant")
}

// -----------------------------------------------------------------------------
// Sphinx of Forgotten Lore
// -----------------------------------------------------------------------------
//
// Oracle text ({2}{U}{U} Creature — Sphinx):
//
//	Flash
//	Flying
//	Whenever this creature attacks, target instant or sorcery card in
//	your graveyard gains flashback until end of turn. The flashback
//	cost is equal to that card's mana cost.

func sphinxOfForgottenLoreAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	attacker, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attacker != perm {
		return
	}
	grantFlashbackToBestGraveyardTarget(gs, perm, "sphinx_of_forgotten_lore_flashback_grant")
}

// -----------------------------------------------------------------------------
// Slickshot Lockpicker
// -----------------------------------------------------------------------------
//
// Oracle text ({2}{U} Creature — Human Rogue):
//
//	When this creature enters, target instant or sorcery card in your
//	graveyard gains flashback until end of turn. The flashback cost is
//	equal to its mana cost.
//	Plot {2}{U}
//
// Plot is the AST keyword pipeline's surface.

func slickshotLockpickerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	grantFlashbackToBestGraveyardTarget(gs, perm, "slickshot_lockpicker_flashback_grant")
}

// -----------------------------------------------------------------------------
// Katilda and Lier
// -----------------------------------------------------------------------------
//
// Oracle text ({G}{W}{U} Legendary Creature — Human):
//
//	Whenever you cast a Human spell, target instant or sorcery card
//	in your graveyard gains flashback until end of turn. The flashback
//	cost is equal to its mana cost.
//
// (Katilda and Lier has other abilities — combat-driven scry,
// activated Spirit token — handled elsewhere or by the AST pipeline.)

func katildaAndLierSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "katilda_and_lier_flashback_grant"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	cast, _ := ctx["card"].(*gameengine.Card)
	if cast == nil {
		return
	}
	// "Human spell" — CR §700 type/subtype: any spell with the Human
	// creature subtype. A Human Wizard, Human Rogue, etc. all qualify.
	if !cardHasSubtype(cast, "human") {
		return
	}
	grantFlashbackToBestGraveyardTarget(gs, perm, slug)
}

// -----------------------------------------------------------------------------
// Shared target-pick + grant
// -----------------------------------------------------------------------------
//
// grantFlashbackToBestGraveyardTarget picks the highest-CMC i/s in the
// controller's graveyard and registers a printed-mana-cost flashback
// grant. Emits per_card_failed on empty/no-eligible-target graveyards
// so observability tracks the "trigger fired but did nothing" case.
//
// Centralized here so adding the next trigger-grant card (Backflash
// Nighteyes, Mind's Desire variant, etc.) is a one-line OnETB/OnTrigger
// pointing at this helper plus a name change.

func grantFlashbackToBestGraveyardTarget(gs *gameengine.GameState, perm *gameengine.Permanent, slug string) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	target := pickHighestCMCInstantOrSorcery(seat.Graveyard, nil)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_target_in_graveyard", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	gameengine.GrantFlashbackUntilEOT(gs, target, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.DisplayName(),
	})
}
