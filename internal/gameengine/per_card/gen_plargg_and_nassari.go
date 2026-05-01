package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPlarggAndNassari wires Plargg and Nassari.
//
// Oracle text:
//
//   At the beginning of your upkeep, each player exiles cards from the top of their library until they exile a nonland card. An opponent chooses a nonland card exiled this way. You may cast up to two spells from among the other cards exiled this way without paying their mana costs.
//
// Auto-generated trigger handler.
func registerPlarggAndNassari(r *Registry) {
	r.OnTrigger("Plargg and Nassari", "upkeep_controller", plarggAndNassariTrigger)
}

func plarggAndNassariTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "plargg_and_nassari_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller { return }
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: trigger effect not parsed from oracle text")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
