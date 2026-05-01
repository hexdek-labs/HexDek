package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSaheeliRadiantCreator wires Saheeli, Radiant Creator.
//
// Oracle text:
//
//   Whenever you cast an Artificer or artifact spell, you get {E} (an energy counter).
//   At the beginning of combat on your turn, you may pay {E}{E}{E}. When you do, create a token that's a copy of target permanent you control, except it's a 5/5 artifact creature in addition to its other types and has haste. Sacrifice it at the beginning of the next end step.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerSaheeliRadiantCreator(r *Registry) {
	r.OnETB("Saheeli, Radiant Creator", saheeliRadiantCreatorStaticETB)
}

func saheeliRadiantCreatorStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "saheeli_radiant_creator_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
