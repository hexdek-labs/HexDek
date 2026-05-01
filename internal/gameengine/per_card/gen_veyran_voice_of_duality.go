package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVeyranVoiceOfDuality wires Veyran, Voice of Duality.
//
// Oracle text:
//
//   Magecraft — Whenever you cast or copy an instant or sorcery spell, Veyran gets +1/+1 until end of turn.
//   If you casting or copying an instant or sorcery spell causes a triggered ability of a permanent you control to trigger, that ability triggers an additional time.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerVeyranVoiceOfDuality(r *Registry) {
	r.OnETB("Veyran, Voice of Duality", veyranVoiceOfDualityStaticETB)
}

func veyranVoiceOfDualityStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "veyran_voice_of_duality_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
