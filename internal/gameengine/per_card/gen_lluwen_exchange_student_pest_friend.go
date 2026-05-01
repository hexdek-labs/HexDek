package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLluwenExchangeStudentPestFriend wires Lluwen, Exchange Student // Pest Friend.
//
// Oracle text:
//
//   Lluwen enters prepared. (While it's prepared, you may cast a copy of its spell. Doing so unprepares it.)
//   Exile a creature card from your graveyard: Lluwen becomes prepared. Activate only as a sorcery.
//   Create a 1/1 black and green Pest creature token with "Whenever this token attacks, you gain 1 life."
//
// Auto-generated activated ability handler.
func registerLluwenExchangeStudentPestFriend(r *Registry) {
	r.OnActivated("Lluwen, Exchange Student // Pest Friend", lluwenExchangeStudentPestFriendActivate)
}

func lluwenExchangeStudentPestFriendActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "lluwen_exchange_student_pest_friend_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Token Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
