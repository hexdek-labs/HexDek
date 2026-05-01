package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTamObservantSequencerDeepSight wires Tam, Observant Sequencer // Deep Sight.
//
// Oracle text:
//
//   Landfall — Whenever a land you control enters, Tam becomes prepared. (While it's prepared, you may cast a copy of its spell. Doing so unprepares it.)
//   You draw a card and gain 1 life.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTamObservantSequencerDeepSight(r *Registry) {
	r.OnETB("Tam, Observant Sequencer // Deep Sight", tamObservantSequencerDeepSightStaticETB)
}

func tamObservantSequencerDeepSightStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tam_observant_sequencer_deep_sight_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
