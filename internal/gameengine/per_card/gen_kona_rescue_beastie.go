package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKonaRescueBeastie wires Kona, Rescue Beastie.
//
// Oracle text:
//
//   Survival — At the beginning of your second main phase, if Kona is tapped, you may put a permanent card from your hand onto the battlefield.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerKonaRescueBeastie(r *Registry) {
	r.OnETB("Kona, Rescue Beastie", konaRescueBeastieStaticETB)
}

func konaRescueBeastieStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kona_rescue_beastie_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
