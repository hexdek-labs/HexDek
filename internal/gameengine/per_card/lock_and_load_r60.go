package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// lock_and_load_r60.go — per_card handler for Lock and Load.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Draw a card, then draw a card for each other instant and sorcery
//	spell you've cast this turn.
//	Plot {3}{U}
//
// {3}{U} Sorcery. A spellslinger draw payoff — in a storm-y turn it
// refills the hand. Parses to an inert `parsed_effect_residual` node and
// no structured Draw, so it drew ZERO cards (the text fallback has no
// "draw a card for each … you've cast this turn" shape). Plot is an
// alternative cast handled by the cost pipeline; this handler is the
// resolution body.
//
// Implementation:
//   - OnResolve. Counts the controller's instant/sorcery casts this turn
//     from Turn.Casts, subtracting one for Lock and Load itself (the
//     "other" qualifier — Lock and Load is already in the cast log by
//     resolution time), then draws 1 + that count via gameengine.DrawN.
func init() {
	registerLockAndLoadR60(Global())
	AddResetHook(registerLockAndLoadR60)
}

func registerLockAndLoadR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Lock and Load", lockAndLoadResolve)
}

func lockAndLoadResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "lock_and_load"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	instSorc := 0
	selfSeen := false
	for _, rec := range gs.Seats[seat].Turn.Casts {
		isIS := false
		for _, t := range rec.Types {
			if t == "instant" || t == "sorcery" {
				isIS = true
				break
			}
		}
		if !isIS {
			continue
		}
		// Exclude one copy of Lock and Load itself ("each OTHER instant
		// and sorcery spell").
		if !selfSeen && rec.CardName == "Lock and Load" {
			selfSeen = true
			continue
		}
		instSorc++
	}

	n := 1 + instSorc
	var src *gameengine.Permanent
	if item.Card != nil {
		src = &gameengine.Permanent{Card: item.Card, Controller: seat, Owner: item.Card.Owner}
	}
	drawn := gameengine.DrawN(gs, seat, n, src)

	emit(gs, slug, "Lock and Load", map[string]interface{}{
		"seat":            seat,
		"other_inst_sorc": instSorc,
		"drawn":           drawn,
	})
}
