package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMabelHeirToCragflame wires Mabel, Heir to Cragflame.
//
// Oracle text (BLB, {1}{R}{W}, 3/3):
//
//	Other Mice you control get +1/+1.
//	When Mabel enters, create Cragflame, a legendary colorless Equipment
//	artifact token with "Equipped creature gets +1/+1 and has vigilance,
//	trample, and haste" and equip {2}.
//
// Implementation:
//   - Static "Other Mice +1/+1" handled by AST.
//   - ETB creates Cragflame as a legendary Equipment artifact token.
//     The token's grant text isn't applied until it's actually equipped
//     (engine has no auto-equip planner); we record the token's static
//     payload via a handler-readable Types tag so a future equip planner
//     can recognize it. emitPartial flags the equip-AI gap.
func registerMabelHeirToCragflame(r *Registry) {
	r.OnETB("Mabel, Heir to Cragflame", mabelHeirToCragflameETB)
}

func mabelHeirToCragflameETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mabel_cragflame_equipment_token"
	if gs == nil || perm == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	token := &gameengine.Card{
		Name:  "Cragflame",
		Owner: seatIdx,
		Types: []string{"token", "legendary", "artifact", "equipment", "cragflame_equipment_grant"},
	}
	tokenPerm := enterBattlefieldWithETB(gs, seatIdx, token, false)

	// R51 batch H port: auto-attach Cragflame to the best friendly Mouse
	// (or, if none, the highest-power friendly creature including Mabel
	// herself). The printed equip {2} cost is a Hat-driven decision the
	// engine doesn't yet plan; firing the attach at ETB matches the
	// strictly-positive value swing that Mabel decks reach for and gives
	// the per-token grant something to bite on for combat math.
	var target *gameengine.Permanent
	bestPow := -1 << 30
	for _, p := range seat.Battlefield {
		if p == nil || p == tokenPerm || p.Card == nil || !p.IsCreature() {
			continue
		}
		if cardHasSubtype(p.Card, "mouse") {
			pow := p.Power()
			if pow > bestPow {
				bestPow = pow
				target = p
			}
		}
	}
	if target == nil {
		for _, p := range seat.Battlefield {
			if p == nil || p == tokenPerm || p.Card == nil || !p.IsCreature() {
				continue
			}
			pow := p.Power()
			if pow > bestPow {
				bestPow = pow
				target = p
			}
		}
	}
	if tokenPerm != nil && target != nil {
		tokenPerm.AttachedTo = target
		if target.Flags == nil {
			target.Flags = map[string]int{}
		}
		target.Flags["kw:vigilance"] = 1
		target.Flags["kw:trample"] = 1
		target.Flags["kw:haste"] = 1
		target.Flags["cragflame_equipped"] = 1
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seatIdx,
		"token":    "Cragflame",
		"attached": target != nil,
	})
}
