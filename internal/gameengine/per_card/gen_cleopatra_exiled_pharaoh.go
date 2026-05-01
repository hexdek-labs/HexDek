package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCleopatraExiledPharaoh wires Cleopatra, Exiled Pharaoh.
//
// Oracle text:
//
//   Allies — At the beginning of your end step, put a +1/+1 counter on each of up to two other target legendary creatures.
//   Betrayal — Whenever a legendary creature with counters on it dies, draw a card for each counter on it. You lose 2 life.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerCleopatraExiledPharaoh(r *Registry) {
	r.OnETB("Cleopatra, Exiled Pharaoh", cleopatraExiledPharaohStaticETB)
}

func cleopatraExiledPharaohStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cleopatra_exiled_pharaoh_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
