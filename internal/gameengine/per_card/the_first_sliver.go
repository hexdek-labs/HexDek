package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheFirstSliver wires The First Sliver.
//
// Oracle text:
//
//	Cascade (When you cast this spell, exile cards from the top of your
//	library until you exile a nonland card that costs less. You may cast
//	it without paying its mana cost. Put the exiled cards on the bottom
//	of your library in a random order.)
//
//	Sliver spells you cast have cascade.
//
// Implementation:
//   - Cascade (keyword on TFS itself): handled by the AST keyword
//     pipeline in stack.go — HasCascadeKeyword returns true, so
//     CastSpell calls ApplyCascade automatically when TFS is cast.
//   - "Sliver spells you cast have cascade": OnTrigger("spell_cast")
//     gated on caster_seat == perm.Controller and the cast card being
//     a Sliver (cardHasType "sliver"). When a qualifying Sliver spell
//     is cast, we call ApplyCascade with the Sliver's CMC.
//
// Limitation (emitPartial):
//   - Cascade chains from cascaded Slivers: when ApplyCascade resolves
//     a cascaded Sliver, it does not re-enter the full CastSpell path,
//     so the "creature_spell_cast" trigger does not fire for cascaded
//     spells. This means a Sliver cascaded into another Sliver won't
//     recursively cascade. Fully modeling this requires the engine's
//     cascade path to call fireCastTriggers, which is a broader change.
func registerTheFirstSliver(r *Registry) {
	r.OnTrigger("The First Sliver", "spell_cast", theFirstSliverCascadeGrant)
}

// theFirstSliverCascadeGrant grants cascade to Sliver spells cast by
// The First Sliver's controller (CR §702.84 via static ability grant).
func theFirstSliverCascadeGrant(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_first_sliver_cascade_grant"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: only trigger for spells cast by TFS's controller.
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}

	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}

	// Gate: the cast spell must be a Sliver.
	if !cardHasType(card, "sliver") {
		return
	}

	// Don't double-cascade spells that already have cascade natively.
	// The First Sliver itself has cascade in its AST; the engine's
	// CastSpell path already calls ApplyCascade for those. Other Slivers
	// printed with cascade (e.g., future printings) would also be caught
	// by the AST check, so skip them here.
	if gameengine.HasCascadeKeyword(card) {
		return
	}

	// Resolve cascade for this Sliver spell.
	spellCMC := card.CMC
	spellName := card.DisplayName()

	hit := gameengine.ApplyCascade(gs, perm.Controller, spellCMC, spellName)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"sliver_spell": spellName,
		"sliver_cmc":   spellCMC,
		"cascade_hit":  hit,
		"rule":         "702.84 (granted by The First Sliver)",
	})

	// Partial: cascaded Slivers don't re-trigger cascade because
	// ApplyCascade doesn't go through the full CastSpell path (no
	// fireCastTriggers call), so the chain stops at one level of
	// granted cascade.
	emitPartial(gs, slug+"_chain", perm.Card.DisplayName(),
		"cascade chain from cascaded Slivers: ApplyCascade does not "+
			"fire spell_cast triggers, so Slivers cast via cascade "+
			"won't themselves cascade from this grant")
}
