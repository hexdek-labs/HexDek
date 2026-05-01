package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheSecondDoctor wires The Second Doctor.
//
// Oracle text:
//
//   Players have no maximum hand size.
//   How Civil of You — At the beginning of your end step, each player may draw a card. Each opponent who does can't attack you or permanents you control during their next turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheSecondDoctor(r *Registry) {
	r.OnETB("The Second Doctor", theSecondDoctorStaticETB)
}

func theSecondDoctorStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_second_doctor_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
