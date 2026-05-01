package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheJollyBalloonMan wires The Jolly Balloon Man.
//
// Oracle text:
//
//   Haste
//   {1}, {T}: Create a token that's a copy of another target creature you control, except it's a 1/1 red Balloon creature in addition to its other colors and types and it has flying and haste. Sacrifice it at the beginning of the next end step. Activate only as a sorcery.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheJollyBalloonMan(r *Registry) {
	r.OnETB("The Jolly Balloon Man", theJollyBalloonManStaticETB)
}

func theJollyBalloonManStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_jolly_balloon_man_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
