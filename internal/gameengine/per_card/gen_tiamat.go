package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTiamat wires Tiamat.
//
// Oracle text:
//
//   Flying
//   When Tiamat enters, if you cast it, search your library for up to five Dragon cards not named Tiamat that each have different names, reveal them, put them into your hand, then shuffle.
//
// Auto-generated ETB handler.
func registerTiamat(r *Registry) {
	r.OnETB("Tiamat", tiamatETB)
}

func tiamatETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tiamat_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
