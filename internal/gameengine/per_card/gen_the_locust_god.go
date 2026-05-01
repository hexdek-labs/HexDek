package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheLocustGod wires The Locust God.
//
// Oracle text:
//
//   Flying
//   Whenever you draw a card, create a 1/1 blue and red Insect creature token with flying and haste.
//   {2}{U}{R}: Draw a card, then discard a card.
//   When The Locust God dies, return it to its owner's hand at the beginning of the next end step.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheLocustGod(r *Registry) {
	r.OnETB("The Locust God", theLocustGodStaticETB)
}

func theLocustGodStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_locust_god_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
