package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheCapitolineTriad wires The Capitoline Triad.
//
// Oracle text:
//
//   Those Who Came Before — This spell costs {1} less to cast for each historic card in your graveyard. (Artifacts, legendaries, and Sagas are historic.)
//   Exile any number of historic cards from your graveyard with total mana value 30 or greater: You get an emblem with "Creatures you control have base power and toughness 9/9."
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheCapitolineTriad(r *Registry) {
	r.OnETB("The Capitoline Triad", theCapitolineTriadStaticETB)
}

func theCapitolineTriadStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_capitoline_triad_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
