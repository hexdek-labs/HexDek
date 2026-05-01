package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLyseHext wires Lyse Hext.
//
// Oracle text:
//
//   Prowess (Whenever you cast a noncreature spell, this creature gets +1/+1 until end of turn.)
//   Noncreature spells you cast cost {1} less to cast.
//   As long as you've cast two or more noncreature spells this turn, Lyse Hext has double strike.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerLyseHext(r *Registry) {
	r.OnETB("Lyse Hext", lyseHextStaticETB)
}

func lyseHextStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lyse_hext_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
