package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerByrkeLongEarOfTheLaw wires Byrke, Long Ear of the Law.
//
// Oracle text:
//
//   Vigilance
//   When Byrke enters, put a +1/+1 counter on each of up to two target creatures.
//   Whenever a creature you control with a +1/+1 counter on it attacks, double the number of +1/+1 counters on it.
//
// Auto-generated ETB handler.
func registerByrkeLongEarOfTheLaw(r *Registry) {
	r.OnETB("Byrke, Long Ear of the Law", byrkeLongEarOfTheLawETB)
}

func byrkeLongEarOfTheLawETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "byrke_long_ear_of_the_law_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
