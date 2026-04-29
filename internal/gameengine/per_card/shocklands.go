package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Shockland handler — shared helper for all 10 Ravnica shocklands
// -----------------------------------------------------------------------------
//
// Oracle text (all ten):
//
//   ({T}: Add {X} or {Y}.)
//   As ~ enters the battlefield, you may pay 2 life. If you don't,
//   it enters the battlefield tapped.
//
// The ETB handler simulates the choice: the AI/Hat decides whether to pay
// 2 life. In MVP, we always pay (cEDH optimizes for tempo; shocks are
// virtually always paid). The land's mana ability (tap for colored mana)
// is handled by the generic AST resolver via typed mana; this handler only
// covers the ETB "pay 2 or enter tapped" replacement.

// shocklandNames lists all 10 Ravnica shocklands.
var shocklandNames = []string{
	"Watery Grave",
	"Steam Vents",
	"Breeding Pool",
	"Blood Crypt",
	"Overgrown Tomb",
	"Sacred Foundry",
	"Stomping Ground",
	"Godless Shrine",
	"Hallowed Fountain",
	"Temple Garden",
}

func registerShocklands(r *Registry) {
	for _, name := range shocklandNames {
		n := name // capture
		r.OnETB(n, func(gs *gameengine.GameState, perm *gameengine.Permanent) {
			shocklandETB(gs, perm, n)
		})
	}
}

// shocklandETB handles the "pay 2 life or enter tapped" ETB replacement.
func shocklandETB(gs *gameengine.GameState, perm *gameengine.Permanent, cardName string) {
	const slug = "shockland_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// MVP heuristic: pay 2 life if life > 5. Otherwise enter tapped.
	// In cEDH most decks almost always pay the 2, but at very low life
	// totals the risk isn't worth it.
	payLife := s.Life > 5
	if payLife {
		s.Life -= 2
		gs.LogEvent(gameengine.Event{
			Kind:   "lose_life",
			Seat:   seat,
			Target: seat,
			Source: cardName,
			Amount: 2,
			Details: map[string]interface{}{
				"reason": "shockland_etb_payment",
			},
		})
		emit(gs, slug, cardName, map[string]interface{}{
			"seat":    seat,
			"paid":    true,
			"life":    s.Life,
			"tapped":  false,
		})
	} else {
		perm.Tapped = true
		emit(gs, slug, cardName, map[string]interface{}{
			"seat":    seat,
			"paid":    false,
			"life":    s.Life,
			"tapped":  true,
		})
	}
	_ = gs.CheckEnd()
}
