package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPrimeSpeakerZegana wires Prime Speaker Zegana.
//
// Oracle text:
//
//   Prime Speaker Zegana enters with X +1/+1 counters on it, where X is the greatest power among other creatures you control.
//   When Prime Speaker Zegana enters, draw cards equal to its power.
//
// Auto-generated ETB handler.
func registerPrimeSpeakerZegana(r *Registry) {
	r.OnETB("Prime Speaker Zegana", primeSpeakerZeganaETB)
}

func primeSpeakerZeganaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "prime_speaker_zegana_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
