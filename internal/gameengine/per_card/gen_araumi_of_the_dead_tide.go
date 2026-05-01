package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAraumiOfTheDeadTide wires Araumi of the Dead Tide.
//
// Oracle text:
//
//   {T}, Exile cards from your graveyard equal to the number of opponents you have: Target creature card in your graveyard gains encore until end of turn. The encore cost is equal to its mana cost. (Exile the creature card and pay its mana cost: For each opponent, create a token copy that attacks that opponent this turn if able. They gain haste. Sacrifice them at the beginning of the next end step. Activate only as a sorcery.)
//
// Auto-generated activated ability handler.
func registerAraumiOfTheDeadTide(r *Registry) {
	r.OnActivated("Araumi of the Dead Tide", araumiOfTheDeadTideActivate)
}

func araumiOfTheDeadTideActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "araumi_of_the_dead_tide_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Creature Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "creature"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
