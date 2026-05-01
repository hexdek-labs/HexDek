package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIvyGleefulSpellthief wires Ivy, Gleeful Spellthief.
//
// Oracle text:
//
//   Flying
//   Whenever a player casts a spell that targets only a single creature other than Ivy, you may copy that spell. The copy targets Ivy. (A copy of an Aura spell becomes a token.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerIvyGleefulSpellthief(r *Registry) {
	r.OnETB("Ivy, Gleeful Spellthief", ivyGleefulSpellthiefStaticETB)
}

func ivyGleefulSpellthiefStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ivy_gleeful_spellthief_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
