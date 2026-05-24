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
	token := &gameengine.Card{
		Name:  "Cragflame",
		Owner: seatIdx,
		Types: []string{"token", "legendary", "artifact", "equipment", "cragflame_equipment_grant"},
	}
	cragflame := enterBattlefieldWithETB(gs, seatIdx, token, false)
	target := pickBestMouseForCragflame(gs.Seats[seatIdx], perm)
	if cragflame != nil && target != nil {
		cragflame.AttachedTo = target
		if target.Flags == nil {
			target.Flags = map[string]int{}
		}
		target.Flags["kw:vigilance"] = 1
		target.Flags["kw:trample"] = 1
		target.Flags["kw:haste"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seatIdx,
		"token":    "Cragflame",
		"attached": target != nil,
	})
}

// pickBestMouseForCragflame returns the friendly Mouse the auto-equip
// planner should attach Cragflame to: highest base power, with non-
// commander mice preferred over Mabel herself on a tie. Returns nil when
// the controller has no Mouse on the battlefield.
func pickBestMouseForCragflame(seat *gameengine.Seat, mabel *gameengine.Permanent) *gameengine.Permanent {
	if seat == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "creature") {
			continue
		}
		if !cardHasSubtype(p.Card, "mouse") {
			continue
		}
		pwr := p.Card.BasePower
		if pwr > bestPower {
			best, bestPower = p, pwr
			continue
		}
		if pwr == bestPower && best == mabel && p != mabel {
			best = p
		}
	}
	return best
}
