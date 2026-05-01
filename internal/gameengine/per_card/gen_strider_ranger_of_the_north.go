package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStriderRangerOfTheNorth wires Strider, Ranger of the North.
//
// Oracle text:
//
//   Landfall — Whenever a land you control enters, target creature gets +1/+1 until end of turn. Then if that creature has power 4 or greater, it gains first strike until end of turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerStriderRangerOfTheNorth(r *Registry) {
	r.OnETB("Strider, Ranger of the North", striderRangerOfTheNorthStaticETB)
}

func striderRangerOfTheNorthStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "strider_ranger_of_the_north_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
