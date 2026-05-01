package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJhoiraAgelessInnovator wires Jhoira, Ageless Innovator.
//
// Oracle text:
//
//   {T}: Put two ingenuity counters on Jhoira, then you may put an artifact card with mana value X or less from your hand onto the battlefield, where X is the number of ingenuity counters on Jhoira.
//
// Auto-generated activated ability handler.
func registerJhoiraAgelessInnovator(r *Registry) {
	r.OnActivated("Jhoira, Ageless Innovator", jhoiraAgelessInnovatorActivate)
}

func jhoiraAgelessInnovatorActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jhoira_ageless_innovator_activate"
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
