package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWraithViciousVigilante wires Wraith, Vicious Vigilante.
//
// Oracle text (Scryfall, verified):
//
//	Double strike
//	Fear Gas — Wraith can't be blocked.
//
// Implementation (R42b stub port):
//   - Double strike: AST keyword pipeline (combat splits damage steps).
//   - Fear Gas (can't be blocked): static self-applied unblockable.
//     Stamp Flags["unblockable"] = 1 at ETB. combat.go's blocker
//     legality check reads this flag (same surface used by Bria
//     Riptide Rogue, Splinter Radical Rat, and the AST "can't be
//     blocked" parser).
//
// The flag is permanent on the perm and survives the cleanup step
// (cleanup wipes Flags["saddled"] and "until_end_of_turn"
// Modifications, but leaves static-self flags). When Wraith leaves
// the battlefield the perm itself is gone, so no explicit cleanup
// is required.
func registerWraithViciousVigilante(r *Registry) {
	r.OnETB("Wraith, Vicious Vigilante", wraithViciousVigilanteETB)
}

func wraithViciousVigilanteETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "wraith_vicious_vigilante_fear_gas"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["unblockable"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"unblockable": true,
	})
}
