package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// lilianas_standard_bearer_r60.go — per_card handler for Liliana's
// Standard Bearer.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Flash
//	When Liliana's Standard Bearer enters, draw X cards, where X is the
//	number of creatures that died under your control this turn.
//
// {3}{B} Creature — Zombie Knight. An aristocrats / post-wrath refill —
// flash it in after a board wipe or a big sacrifice turn to draw a fistful
// of cards. The ETB draw parses to an inert `parsed_effect_residual`
// node (no structured Draw), so it drew ZERO cards.
//
// Implementation: OnETB → DrawN(controller, creaturesDiedThisTurn). The
// per-seat Turn.CreaturesDied counter tracks creatures that went to the
// graveyard from the battlefield under that seat's control this turn,
// which is exactly X.
func init() {
	registerLilianasStandardBearerR60(Global())
	AddResetHook(registerLilianasStandardBearerR60)
}

func registerLilianasStandardBearerR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Liliana's Standard Bearer", lilianasStandardBearerETB)
}

func lilianasStandardBearerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lilianas_standard_bearer"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	x := gs.Seats[seat].Turn.CreaturesDied
	drawn := gameengine.DrawN(gs, seat, x, perm)
	emit(gs, slug, "Liliana's Standard Bearer", map[string]interface{}{
		"seat":           seat,
		"creatures_died": x,
		"drawn":          drawn,
	})
}
