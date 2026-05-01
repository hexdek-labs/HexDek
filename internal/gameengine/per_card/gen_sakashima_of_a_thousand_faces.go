package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSakashimaOfAThousandFaces wires Sakashima of a Thousand Faces.
//
// Oracle text:
//
//   You may have Sakashima enter as a copy of another creature you control, except it has Sakashima's other abilities.
//   The "legend rule" doesn't apply to permanents you control.
//   Partner (You can have two commanders if both have partner.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerSakashimaOfAThousandFaces(r *Registry) {
	r.OnETB("Sakashima of a Thousand Faces", sakashimaOfAThousandFacesStaticETB)
}

func sakashimaOfAThousandFacesStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sakashima_of_a_thousand_faces_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
