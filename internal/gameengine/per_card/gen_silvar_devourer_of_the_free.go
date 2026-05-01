package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSilvarDevourerOfTheFree wires Silvar, Devourer of the Free.
//
// Oracle text:
//
//   Partner with Trynn, Champion of Freedom (When this creature enters, target player may put Trynn into their hand from their library, then shuffle.)
//   Menace
//   Sacrifice a Human: Put a +1/+1 counter on Silvar. It gains indestructible until end of turn.
//
// Auto-generated activated ability handler.
func registerSilvarDevourerOfTheFree(r *Registry) {
	r.OnActivated("Silvar, Devourer of the Free", silvarDevourerOfTheFreeActivate)
}

func silvarDevourerOfTheFreeActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "silvar_devourer_of_the_free_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	src.AddCounter("+1/+1", 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
