package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFiresongAndSunspeaker wires Firesong and Sunspeaker.
//
// Oracle text:
//
//	Red instant and sorcery spells you control have lifelink.
//	Whenever a white instant or sorcery spell causes you to gain life,
//	Firesong and Sunspeaker deals 3 damage to target creature or player.
//
// Implementation:
//   - The "red instants/sorceries have lifelink" static is left to the
//     AST keyword pipeline (a per-card static-grant slot we can't reach
//     from a trigger handler). emitPartial flags the wiring boundary.
//   - "life_gained" trigger gated on (a) the gainer is Firesong's
//     controller and (b) the source of the gain is a white
//     instant/sorcery that the controller cast. Picks the lowest-life
//     living opponent as the deal-3 target — same heuristic as Silverquill.
func registerFiresongAndSunspeaker(r *Registry) {
	r.OnETB("Firesong and Sunspeaker", firesongAndSunspeakerETB)
	r.OnTrigger("Firesong and Sunspeaker", "life_gained", firesongAndSunspeakerLifeGained)
}

func firesongAndSunspeakerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "firesong_and_sunspeaker_etb"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	// Red I/S lifelink-grant is wired in custom_firesong_and_sunspeaker.go
	// (R49 batchD) — we listen on noncombat damage events and reflect
	// the damage as life gain when the resolving spell is a red I/S the
	// controller cast.
}

func firesongAndSunspeakerLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "firesong_white_instant_sorcery_burn"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller {
		return
	}
	// Source must be a white instant or sorcery the controller cast.
	srcCard, _ := ctx["source_card"].(*gameengine.Card)
	if srcCard == nil {
		// Fallback: try a stack-item carrier.
		if si, ok := ctx["stack_item"].(*gameengine.StackItem); ok && si != nil {
			srcCard = si.Card
		}
	}
	if srcCard == nil {
		return
	}
	if !cardHasType(srcCard, "instant") && !cardHasType(srcCard, "sorcery") {
		return
	}
	if !cardIsWhite(srcCard) {
		return
	}
	target := lowestLifeOpponent(gs, perm.Controller)
	if target < 0 {
		return
	}
	gameengine.DealDamage(gs, target, 3, perm.Card.DisplayName())
	_ = gs.CheckEnd()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"target_seat": target,
		"damage":      3,
	})
}

// cardIsWhite checks Card.Colors and pip:W for a white identity.
func cardIsWhite(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, col := range c.Colors {
		if col == "W" || col == "w" {
			return true
		}
	}
	for _, t := range c.Types {
		if t == "pip:W" {
			return true
		}
	}
	return false
}

// lowestLifeOpponent returns the seat index of the lowest-life living
// opponent, or -1 if no legal target.
func lowestLifeOpponent(gs *gameengine.GameState, controller int) int {
	target := -1
	bestLife := 1 << 30
	for _, opp := range gs.Opponents(controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life < bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	return target
}
