package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// lord_of_change_r60.go — per_card handler for Lord of Change.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Flying, ward {3}
//	Architect of Deception — When Lord of Change enters, draw three cards.
//
// {3}{U}{U}{U} Creature — Demon. A blue beater with a three-card ETB
// refill. The ETB draw parses to an inert `parsed_tail` node (no
// structured Draw), so it drew ZERO cards. Flying/ward are keywords
// handled by the keyword layer; only the draw was dead.
//
// Implementation: OnETB → gameengine.DrawN(controller, 3).
func init() {
	registerLordOfChangeR60(Global())
	AddResetHook(registerLordOfChangeR60)
}

func registerLordOfChangeR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Lord of Change", lordOfChangeETB)
}

func lordOfChangeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lord_of_change"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawn := gameengine.DrawN(gs, seat, 3, perm)
	emit(gs, slug, "Lord of Change", map[string]interface{}{"seat": seat, "drawn": drawn})
}
