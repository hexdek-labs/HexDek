package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerThrunBreakerOfSilence wires Thrun, Breaker of Silence.
//
// Oracle text:
//
//   This spell can't be countered.
//   Trample
//   Thrun can't be the target of nongreen spells your opponents control or abilities from nongreen sources your opponents control.
//   During your turn, Thrun has indestructible.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerThrunBreakerOfSilence(r *Registry) {
	r.OnETB("Thrun, Breaker of Silence", thrunBreakerOfSilenceStaticETB)
}

func thrunBreakerOfSilenceStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "thrun_breaker_of_silence_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
