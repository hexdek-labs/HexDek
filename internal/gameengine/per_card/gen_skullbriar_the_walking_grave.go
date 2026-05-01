package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSkullbriarTheWalkingGrave wires Skullbriar, the Walking Grave.
//
// Oracle text:
//
//   Haste
//   Whenever Skullbriar deals combat damage to a player, put a +1/+1 counter on it.
//   Counters remain on Skullbriar as it moves to any zone other than a player's hand or library.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerSkullbriarTheWalkingGrave(r *Registry) {
	r.OnETB("Skullbriar, the Walking Grave", skullbriarTheWalkingGraveStaticETB)
}

func skullbriarTheWalkingGraveStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "skullbriar_the_walking_grave_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
