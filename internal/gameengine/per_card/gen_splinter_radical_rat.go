package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSplinterRadicalRat wires Splinter, Radical Rat.
//
// Oracle text:
//
//   If a triggered ability of a Ninja creature you control triggers, that ability triggers an additional time.
//   {1}{U}: Target Ninja can't be blocked this turn.
//
// Auto-generated activated ability handler.
func registerSplinterRadicalRat(r *Registry) {
	r.OnActivated("Splinter, Radical Rat", splinterRadicalRatActivate)
}

func splinterRadicalRatActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "splinter_radical_rat_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(), "auto-gen: activated effect not parsed from oracle text")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
