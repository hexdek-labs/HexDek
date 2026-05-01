package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSauronTheNecromancer wires Sauron, the Necromancer.
//
// Oracle text:
//
//   Menace
//   Whenever Sauron attacks, exile target creature card from your graveyard. Create a tapped and attacking token that's a copy of that card, except it's a 3/3 black Wraith with menace. At the beginning of the next end step, exile that token unless Sauron is your Ring-bearer.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerSauronTheNecromancer(r *Registry) {
	r.OnETB("Sauron, the Necromancer", sauronTheNecromancerStaticETB)
}

func sauronTheNecromancerStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sauron_the_necromancer_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
