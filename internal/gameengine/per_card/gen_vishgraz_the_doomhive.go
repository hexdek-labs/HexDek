package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVishgrazTheDoomhive wires Vishgraz, the Doomhive.
//
// Oracle text:
//
//   Menace, toxic 1 (Players dealt combat damage by this creature also get a poison counter.)
//   When Vishgraz enters, create three 1/1 colorless Phyrexian Mite artifact creature tokens with toxic 1 and "This token can't block."
//   Vishgraz gets +1/+1 for each poison counter your opponents have.
//
// Auto-generated ETB handler.
func registerVishgrazTheDoomhive(r *Registry) {
	r.OnETB("Vishgraz, the Doomhive", vishgrazTheDoomhiveETB)
}

func vishgrazTheDoomhiveETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "vishgraz_the_doomhive_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Token Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
