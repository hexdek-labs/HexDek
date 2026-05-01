package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIronManTitanOfInnovation wires Iron Man, Titan of Innovation.
//
// Oracle text:
//
//   Flying, haste
//   Genius Industrialist — Whenever Iron Man attacks, create a Treasure token, then you may sacrifice a noncreature artifact. If you do, search your library for an artifact card with mana value equal to 1 plus the sacrificed artifact's mana value, put it onto the battlefield tapped, then shuffle.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerIronManTitanOfInnovation(r *Registry) {
	r.OnETB("Iron Man, Titan of Innovation", ironManTitanOfInnovationStaticETB)
}

func ironManTitanOfInnovationStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "iron_man_titan_of_innovation_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
