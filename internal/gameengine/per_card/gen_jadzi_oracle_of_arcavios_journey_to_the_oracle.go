package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJadziOracleOfArcaviosJourneyToTheOracle wires Jadzi, Oracle of Arcavios // Journey to the Oracle.
//
// Oracle text:
//
//   Discard a card: Return Jadzi to its owner's hand.
//   Magecraft — Whenever you cast or copy an instant or sorcery spell, reveal the top card of your library. If it's a nonland card, you may cast it by paying {1} rather than paying its mana cost. If it's a land card, put it onto the battlefield.
//   You may put any number of land cards from your hand onto the battlefield. Then if you control eight or more lands, you may discard a card. If you do, return Journey to the Oracle to its owner's hand.
//
// Auto-generated activated ability handler.
func registerJadziOracleOfArcaviosJourneyToTheOracle(r *Registry) {
	r.OnActivated("Jadzi, Oracle of Arcavios // Journey to the Oracle", jadziOracleOfArcaviosJourneyToTheOracleActivate)
}

func jadziOracleOfArcaviosJourneyToTheOracleActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jadzi_oracle_of_arcavios_journey_to_the_oracle_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(), "auto-gen: activated effect not parsed from oracle text")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
