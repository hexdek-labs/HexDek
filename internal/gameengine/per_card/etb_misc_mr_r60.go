package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// etb_misc_mr_r60.go — shard M-R creatures whose ETB clauses parsed to
// inert parsed_tail nodes (no structured effect) and did NOTHING:
//
//   - Owlbear: ETB draw a card
//   - Red Dragon: ETB deal 4 damage to each opponent
//   - Myconid Spore Tender: ETB destroy up to one target artifact or
//     enchantment (opponent's)
//
// (Flying/trample etc. are keywords handled elsewhere; only the ETB
// payload was dead.) One new self-registering file (init() +
// AddResetHook); no shared registry edits.
func init() {
	registerETBMiscMRR60(Global())
	AddResetHook(registerETBMiscMRR60)
}

func registerETBMiscMRR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnETB("Owlbear", owlbearETB)
	r.OnETB("Red Dragon", redDragonETB)
	r.OnETB("Myconid Spore Tender", myconidSporeTenderETB)
}

func owlbearETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	drawn := gameengine.DrawN(gs, perm.Controller, 1, perm)
	emit(gs, "owlbear", "Owlbear", map[string]interface{}{"seat": perm.Controller, "drawn": drawn})
}

func redDragonETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		gameengine.DealDamage(gs, opp, 4, "Red Dragon")
	}
	gameengine.StateBasedActions(gs)
	emit(gs, "red_dragon", "Red Dragon", map[string]interface{}{"seat": seat})
}

func myconidSporeTenderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "myconid_spore_tender"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	// "up to one" — pick the first opponent artifact or enchantment; decline
	// if none (no downside to declining).
	var target *gameengine.Permanent
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && (p.IsArtifact() || p.IsEnchantment()) {
				target = p
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		emit(gs, slug, "Myconid Spore Tender", map[string]interface{}{"seat": seat, "destroyed": 0})
		return
	}
	ok := gameengine.DestroyPermanent(gs, target, perm)
	gameengine.StateBasedActions(gs)
	emit(gs, slug, "Myconid Spore Tender", map[string]interface{}{"seat": seat, "destroyed": map[bool]int{true: 1, false: 0}[ok]})
}
