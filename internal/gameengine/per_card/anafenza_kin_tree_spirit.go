package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAnafenzaKinTreeSpirit wires Anafenza, Kin-Tree Spirit.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{W}{W}
//	Legendary Creature — Spirit Soldier
//	Whenever another nontoken creature you control enters, bolster 1.
//	(Choose a creature with the least toughness among creatures you
//	control and put a +1/+1 counter on it.)
//
// Implementation:
//   - permanent_etb: gate on controller_seat == perm.Controller, entering
//     permanent is a non-token creature, and not Anafenza herself.
//   - Bolster: pick the lowest-toughness creature on the battlefield
//     (ties broken by lowest power so the buff helps the runt, not the
//     finisher), add a +1/+1 counter.
func registerAnafenzaKinTreeSpirit(r *Registry) {
	r.OnTrigger("Anafenza, Kin-Tree Spirit", "permanent_etb", anafenzaKinTreeBolster)
}

func anafenzaKinTreeBolster(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "anafenza_kin_tree_bolster_1"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	enteringPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if enteringPerm == nil {
		enteringPerm, _ = ctx["permanent"].(*gameengine.Permanent)
	}
	if enteringPerm == nil || enteringPerm == perm || enteringPerm.Card == nil {
		return
	}
	if enteringPerm.Controller != perm.Controller {
		return
	}
	if !enteringPerm.IsCreature() || enteringPerm.IsToken() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var target *gameengine.Permanent
	bestT := 1 << 30
	bestP := 1 << 30
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		t := p.Toughness()
		if t < bestT || (t == bestT && p.Power() < bestP) {
			bestT = t
			bestP = p.Power()
			target = p
		}
	}
	if target == nil {
		return
	}
	target.AddCounter("+1/+1", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"trigger_etb": enteringPerm.Card.DisplayName(),
		"bolstered":  target.Card.DisplayName(),
	})
}
