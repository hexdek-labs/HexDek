package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNerivCracklingVanguard wires Neriv, Crackling Vanguard.
//
// Oracle text:
//
//   Flying, deathtouch
//   When Neriv enters, create two 1/1 red Goblin creature tokens.
//   Whenever Neriv attacks, exile a number of cards from the top of your library equal to the number of differently named tokens you control. During any turn you attacked with a commander, you may play those cards.
//
// Auto-generated ETB handler.
func registerNerivCracklingVanguard(r *Registry) {
	r.OnETB("Neriv, Crackling Vanguard", nerivCracklingVanguardETB)
}

func nerivCracklingVanguardETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "neriv_crackling_vanguard_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Goblin Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "goblin"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
