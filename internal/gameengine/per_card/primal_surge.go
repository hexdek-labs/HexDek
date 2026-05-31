package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Primal Surge — {8}{G}{G} Sorcery. Exile the top card of your library.
// If it is a permanent card, you may put it onto the battlefield. If you
// do, repeat this process.
//
// Greedy policy: always elect "yes" on the may-put — the standard Surge
// line is "I want every permanent on top of my library." Stops on the
// first nonpermanent (which stays exiled per the printed text) or when
// library empties. Capped at 100 iterations as a defensive guard against
// pathologically large permanent-heavy libraries.

func init() {
	registerPrimalSurge(Global())
	AddResetHook(registerPrimalSurge)
}

func registerPrimalSurge(r *Registry) {
	r.OnResolve("Primal Surge", primalSurgeResolve)
}

func primalSurgeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "primal_surge"
	if gs == nil || item == nil || item.Card == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	const maxIter = 100
	exiledCount := 0
	playedCount := 0
	for i := 0; i < maxIter && len(s.Library) > 0; i++ {
		top := s.Library[0]
		if top == nil {
			break
		}
		moveCardBetweenZones(gs, seat, top, "library", "exile", "primal_surge_exile")
		exiledCount++
		if !isPermanentCard(top) {
			// Per oracle: stops the loop. Nonpermanent stays exiled.
			break
		}
		if !gameengine.CardCanEnterBattlefield(top) {
			// Defensive — shouldn't happen for permanent types but guard
			// the createPermanent contract.
			break
		}
		if enterBattlefieldWithETB(gs, seat, top, false) == nil {
			break
		}
		playedCount++
	}
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"exiled_count":  exiledCount,
		"played_count":  playedCount,
		"library_empty": len(s.Library) == 0,
	})
}
