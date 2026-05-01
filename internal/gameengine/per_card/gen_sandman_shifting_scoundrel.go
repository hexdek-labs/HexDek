package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSandmanShiftingScoundrel wires Sandman, Shifting Scoundrel.
//
// Oracle text:
//
//   Sandman's power and toughness are each equal to the number of lands you control.
//   Sandman can't be blocked by creatures with power 2 or less.
//   {3}{G}{G}: Return this card and target land card from your graveyard to the battlefield tapped.
//
// Auto-generated activated ability handler.
func registerSandmanShiftingScoundrel(r *Registry) {
	r.OnActivated("Sandman, Shifting Scoundrel", sandmanShiftingScoundrelActivate)
}

func sandmanShiftingScoundrelActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sandman_shifting_scoundrel_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(), "auto-gen: activated effect not parsed from oracle text")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
