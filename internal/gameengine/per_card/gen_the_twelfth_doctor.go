package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheTwelfthDoctor wires The Twelfth Doctor.
//
// Oracle text:
//
//	The first spell you cast from anywhere other than your hand each
//	turn has demonstrate. (When you cast that spell, you may copy it.
//	If you do, choose an opponent to also copy it. A copy of a
//	permanent spell becomes a token.)
//	Whenever you copy a spell, put a +1/+1 counter on The Twelfth
//	Doctor.
//
// Implementation (R46 stub port):
//   - OnTrigger("spell_copied") — dispatched by resolve.go's spell-copy
//     resolution path. When the copying seat matches Twelfth Doctor's
//     controller, add a +1/+1 counter. Per-card-driven copies (Kalamax,
//     Alania, Riku) don't reach the resolve.go path and don't fire;
//     that's flagged via emitPartial on ETB.
//   - The demonstrate-grant clause (first non-hand cast per turn gets
//     demonstrate) is a cast-pipeline rider not exposed at the
//     per_card layer — remains partial.
func registerTheTwelfthDoctor(r *Registry) {
	r.OnETB("The Twelfth Doctor", theTwelfthDoctorETB)
	r.OnTrigger("The Twelfth Doctor", "spell_copied", theTwelfthDoctorCounter)
}

func theTwelfthDoctorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_twelfth_doctor_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"demonstrate_grant_on_first_non_hand_cast_per_turn_not_wired_in_cast_pipeline")
}

func theTwelfthDoctorCounter(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_twelfth_doctor_copy_counter"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	perm.AddCounter("+1/+1", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"counter":  "+1/+1",
		"new_pow":  perm.Power(),
	})
}
