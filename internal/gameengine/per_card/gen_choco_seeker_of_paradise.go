package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChocoSeekerOfParadise wires Choco, Seeker of Paradise.
//
// Oracle text:
//
//   Whenever one or more Birds you control attack, look at that many cards from the top of your library. You may put one of them into your hand. Then put any number of land cards from among them onto the battlefield tapped and the rest into your graveyard.
//   Landfall — Whenever a land you control enters, Choco gets +1/+0 until end of turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerChocoSeekerOfParadise(r *Registry) {
	r.OnETB("Choco, Seeker of Paradise", chocoSeekerOfParadiseStaticETB)
}

func chocoSeekerOfParadiseStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "choco_seeker_of_paradise_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
