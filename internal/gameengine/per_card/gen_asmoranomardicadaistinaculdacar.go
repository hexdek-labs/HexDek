package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAsmoranomardicadaistinaculdacar wires Asmoranomardicadaistinaculdacar.
//
// Oracle text:
//
//   As long as you've discarded a card this turn, you may pay {B/R} to cast this spell.
//   When Asmoranomardicadaistinaculdacar enters, you may search your library for a card named The Underworld Cookbook, reveal it, put it into your hand, then shuffle.
//   Sacrifice two Foods: Target creature deals 6 damage to itself.
//
// Auto-generated ETB handler.
func registerAsmoranomardicadaistinaculdacar(r *Registry) {
	r.OnETB("Asmoranomardicadaistinaculdacar", asmoranomardicadaistinaculdacarETB)
}

func asmoranomardicadaistinaculdacarETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "asmoranomardicadaistinaculdacar_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
