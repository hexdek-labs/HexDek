package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerErietteOfTheCharmedApple wires Eriette of the Charmed Apple.
//
// Oracle text:
//
//   Each creature that's enchanted by an Aura you control can't attack you or planeswalkers you control.
//   At the beginning of your end step, each opponent loses X life and you gain X life, where X is the number of Auras you control.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerErietteOfTheCharmedApple(r *Registry) {
	r.OnETB("Eriette of the Charmed Apple", erietteOfTheCharmedAppleStaticETB)
}

func erietteOfTheCharmedAppleStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "eriette_of_the_charmed_apple_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
