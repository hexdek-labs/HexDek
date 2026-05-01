package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerObekaBruteChronologist wires Obeka, Brute Chronologist.
//
// Oracle text:
//
//   {T}: The player whose turn it is may end the turn. (Exile all spells and abilities from the stack. The player whose turn it is discards down to their maximum hand size. Damage wears off, and "this turn" and "until end of turn" effects end.)
//
// Auto-generated activated ability handler.
func registerObekaBruteChronologist(r *Registry) {
	r.OnActivated("Obeka, Brute Chronologist", obekaBruteChronologistActivate)
}

func obekaBruteChronologistActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "obeka_brute_chronologist_activate"
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
