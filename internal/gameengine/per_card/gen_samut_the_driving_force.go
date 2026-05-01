package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSamutTheDrivingForce wires Samut, the Driving Force.
//
// Oracle text:
//
//   First strike, vigilance, haste
//   Start your engines! (If you have no speed, it starts at 1. It increases once on each of your turns when an opponent loses life. Max speed is 4.)
//   Other creatures you control get +X/+0, where X is your speed.
//   Noncreature spells you cast cost {X} less to cast, where X is your speed.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerSamutTheDrivingForce(r *Registry) {
	r.OnETB("Samut, the Driving Force", samutTheDrivingForceStaticETB)
}

func samutTheDrivingForceStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "samut_the_driving_force_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
