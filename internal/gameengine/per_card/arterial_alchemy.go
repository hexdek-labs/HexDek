package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Arterial Alchemy — {3}{B} Enchantment.
//
//	When this enchantment enters, create a Blood token for each opponent
//	you have.
//
// The ETB token creation parsed to a `custom` slug with no handler, so
// the enchantment entered and produced no Blood tokens. This handler
// creates one Blood token per living opponent.
func init() {
	registerArterialAlchemy(Global())
	AddResetHook(registerArterialAlchemy)
}

func registerArterialAlchemy(r *Registry) {
	r.OnETB("Arterial Alchemy", arterialAlchemyETB)
}

func arterialAlchemyETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "arterial_alchemy"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	opponents := 0
	for i, s := range gs.Seats {
		if i == seat || s == nil || s.Lost || s.LeftGame {
			continue
		}
		opponents++
	}
	for i := 0; i < opponents; i++ {
		gameengine.CreateBloodToken(gs, seat)
	}
	emit(gs, slug, "Arterial Alchemy", map[string]interface{}{
		"seat":         seat,
		"blood_tokens": opponents,
	})
}
