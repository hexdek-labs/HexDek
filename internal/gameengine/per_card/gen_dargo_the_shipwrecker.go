package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDargoTheShipwrecker wires Dargo, the Shipwrecker.
//
// Oracle text (Scryfall, verified):
//
//	As an additional cost to cast this spell, you may sacrifice any
//	number of artifacts and/or creatures. This spell costs {2} less
//	to cast for each permanent sacrificed this way and {2} less to
//	cast for each other artifact or creature you've sacrificed this
//	turn.
//	Trample
//	Partner
//
// Implementation (R46 stub port):
//   - Trample: AST keyword pipeline.
//   - Self-cast cost reduction (second clause): handled engine-side
//     via the new Dargo self-cast branch in cost_modifiers.go
//     ScanCostModifiers — discounts 2 × seat.Turn.Sacrificed when
//     Dargo is the card being cast. Turn.Sacrificed counts all
//     permanent sacrifices this turn; we treat it as a fuzzy
//     artifact-or-creature approximation since most sac outlets in
//     practice target those types.
//   - First-clause sac-as-additional-cost rider: alt-cost cast
//     pipeline territory; emitPartial breadcrumb on ETB. The cast
//     pipeline doesn't yet accept arbitrary at-cast permanent
//     sacrifice as an alternative cost from the per_card layer.
//   - Partner is a deck-construction tag; no runtime behavior.
func registerDargoTheShipwrecker(r *Registry) {
	r.OnETB("Dargo, the Shipwrecker", dargoTheShipwreckerETB)
}

func dargoTheShipwreckerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dargo_the_shipwrecker_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"sac_as_additional_cost_alt_cost_cast_pipeline_not_wired_at_per_card_layer")
}
