package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheTwelfthDoctor wires The Twelfth Doctor.
//
// Oracle text:
//
//   The first spell you cast from anywhere other than your hand each turn has demonstrate. (When you cast that spell, you may copy it. If you do, choose an opponent to also copy it. A copy of a permanent spell becomes a token.)
//   Whenever you copy a spell, put a +1/+1 counter on The Twelfth Doctor.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheTwelfthDoctor(r *Registry) {
	r.OnETB("The Twelfth Doctor", theTwelfthDoctorStaticETB)
}

func theTwelfthDoctorStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_twelfth_doctor_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
