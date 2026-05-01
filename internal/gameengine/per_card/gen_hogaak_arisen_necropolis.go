package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHogaakArisenNecropolis wires Hogaak, Arisen Necropolis.
//
// Oracle text:
//
//   You can't spend mana to cast this spell.
//   Convoke, delve (Each creature you tap while casting this spell pays for {1} or one mana of that creature's color. Each card you exile from your graveyard pays for {1}.)
//   You may cast this card from your graveyard.
//   Trample
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerHogaakArisenNecropolis(r *Registry) {
	r.OnETB("Hogaak, Arisen Necropolis", hogaakArisenNecropolisStaticETB)
}

func hogaakArisenNecropolisStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hogaak_arisen_necropolis_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
