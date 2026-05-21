package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSandmanShiftingScoundrelCustom wires Sandman's
// "power and toughness are each equal to the number of lands you
// control" CDA. The auto-generated handler in
// gen_sandman_shifting_scoundrel.go covers the graveyard-recur
// activated ability; the partial breadcrumb it left was the CDA + the
// "can't be blocked by power 2 or less" combat restriction.
//
// CDA implementation: stamp Sandman's base P/T equal to the controller's
// land count on every relevant event:
//   - ETB (initial value).
//   - permanent_etb / permanent_ltb (lands changing the count).
//   - upkeep_controller (defensive refresh in case any event was missed).
//
// The block-restriction ("can't be blocked by creatures with power 2 or
// less") is a combat-layer hook we can't cleanly intercept from
// per_card; the gen_*.go partial documents that gap. We do NOT remove
// that partial — the CDA piece is what this file owns.
func registerSandmanShiftingScoundrelCustom(r *Registry) {
	r.OnETB("Sandman, Shifting Scoundrel", sandmanRefreshPTOnETB)
	r.OnTrigger("Sandman, Shifting Scoundrel", "permanent_etb", sandmanRefreshPTOnEvent)
	r.OnTrigger("Sandman, Shifting Scoundrel", "permanent_ltb", sandmanRefreshPTOnEvent)
	r.OnTrigger("Sandman, Shifting Scoundrel", "upkeep_controller", sandmanRefreshPTOnEvent)
}

func sandmanRefreshPTOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	sandmanRefreshPT(gs, perm)
}

func sandmanRefreshPTOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	sandmanRefreshPT(gs, perm)
}

func sandmanRefreshPT(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sandman_cda_pt_refresh"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	lands := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "land") {
			lands++
		}
	}
	perm.Card.BasePower = lands
	perm.Card.BaseToughness = lands
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"lands": lands,
		"power": lands,
	})
}
