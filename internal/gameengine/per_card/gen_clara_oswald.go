package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerClaraOswald wires Clara Oswald.
//
// Oracle text:
//
//   Impossible Girl — If Clara Oswald is your commander, choose a color before the game begins. Clara Oswald is the chosen color.
//   If a triggered ability of a Doctor you control triggers, that ability triggers an additional time.
//   Doctor's companion (You can have two commanders if the other is the Doctor.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerClaraOswald(r *Registry) {
	r.OnETB("Clara Oswald", claraOswaldStaticETB)
}

func claraOswaldStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "clara_oswald_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
