package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBristlyBillSpineSower wires Bristly Bill, Spine Sower.
//
// Oracle text:
//
//   Landfall — Whenever a land you control enters, put a +1/+1 counter on target creature.
//   {3}{G}{G}: Double the number of +1/+1 counters on each creature you control.
//
// Auto-generated activated ability handler.
func registerBristlyBillSpineSower(r *Registry) {
	r.OnActivated("Bristly Bill, Spine Sower", bristlyBillSpineSowerActivate)
}

func bristlyBillSpineSowerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "bristly_bill_spine_sower_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for _, p := range gs.Seats[src.Controller].Battlefield {
		if p == nil || !p.IsCreature() || p == src { continue }
		p.AddCounter("+1/+1", 1)
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
