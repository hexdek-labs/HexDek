package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShikoAndNarsetUnified wires Shiko and Narset, Unified.
//
// Oracle text:
//
//   Flying, vigilance
//   Flurry — Whenever you cast your second spell each turn, copy that spell if it targets a permanent or player, and you may choose new targets for the copy. If you don't copy a spell this way, draw a card.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerShikoAndNarsetUnified(r *Registry) {
	r.OnETB("Shiko and Narset, Unified", shikoAndNarsetUnifiedStaticETB)
}

func shikoAndNarsetUnifiedStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "shiko_and_narset_unified_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
