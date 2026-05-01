package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJonIrenicusShatteredOne wires Jon Irenicus, Shattered One.
//
// Oracle text:
//
//   At the beginning of your end step, target opponent gains control of up to one target creature you control. Put two +1/+1 counters on it and tap it. It's goaded for the rest of the game and it gains "This creature can't be sacrificed." (It attacks each combat if able and attacks a player other than you if able.)
//   Whenever a creature you own but don't control attacks, you draw a card.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerJonIrenicusShatteredOne(r *Registry) {
	r.OnETB("Jon Irenicus, Shattered One", jonIrenicusShatteredOneStaticETB)
}

func jonIrenicusShatteredOneStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "jon_irenicus_shattered_one_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
