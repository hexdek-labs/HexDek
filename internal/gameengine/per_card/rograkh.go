package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRograkh wires Rograkh, Son of Rohgahh.
//
// Oracle text:
//
//	First strike, menace, trample
//	Partner
//
// Rograkh has mana cost {0} and is 0/1. All gameplay-relevant pieces —
// keywords (first strike, menace, trample) and the printed power/toughness —
// are handled by the AST/engine. Partner is a deck-building rule.
//
// This stub exists only so the per-card registry has an entry for
// registration tracking and so the tooling that audits "every commander
// has a handler" doesn't flag Rograkh.
func registerRograkh(r *Registry) {
	r.OnETB("Rograkh, Son of Rohgahh", rograkhStaticETB)
}

func rograkhStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rograkh_son_of_rohgahh_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"keywords (first strike, menace, trample) handled by AST engine; per_card stub for registration tracking")
}
