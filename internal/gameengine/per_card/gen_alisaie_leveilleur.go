package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlisaieLeveilleur wires Alisaie Leveilleur.
//
// Oracle text:
//
//   Partner with Alphinaud Leveilleur (When this creature enters, target player may put Alphinaud Leveilleur into their hand from their library, then shuffle.)
//   First strike
//   Dualcast — The second spell you cast each turn costs {2} less to cast.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerAlisaieLeveilleur(r *Registry) {
	r.OnETB("Alisaie Leveilleur", alisaieLeveilleurStaticETB)
}

func alisaieLeveilleurStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "alisaie_leveilleur_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
