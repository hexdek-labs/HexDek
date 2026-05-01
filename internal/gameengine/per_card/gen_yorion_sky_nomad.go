package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYorionSkyNomad wires Yorion, Sky Nomad.
//
// Oracle text:
//
//   Companion — Your starting deck contains at least twenty cards more than the minimum deck size. (If this card is your chosen companion, you may put it into your hand from outside the game for {3} as a sorcery.)
//   Flying
//   When Yorion enters, exile any number of other nonland permanents you own and control. Return those cards to the battlefield at the beginning of the next end step.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerYorionSkyNomad(r *Registry) {
	r.OnETB("Yorion, Sky Nomad", yorionSkyNomadStaticETB)
}

func yorionSkyNomadStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "yorion_sky_nomad_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
