package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlpharaelStonechosen wires Alpharael, Stonechosen.
//
// Oracle text:
//
//   Ward—Discard a card at random.
//   Void — Whenever Alpharael attacks, if a nonland permanent left the battlefield this turn or a spell was warped this turn, defending player loses half their life, rounded up.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerAlpharaelStonechosen(r *Registry) {
	r.OnETB("Alpharael, Stonechosen", alpharaelStonechosenStaticETB)
}

func alpharaelStonechosenStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "alpharael_stonechosen_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
