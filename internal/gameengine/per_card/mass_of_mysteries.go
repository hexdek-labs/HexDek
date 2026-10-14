package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMassOfMysteries wires Mass of Mysteries.
//
// Oracle text:
//
//	First strike, vigilance, trample
//	At the beginning of combat on your turn, another target Elemental
//	you control gains myriad until end of turn.
//
// Implementation: myriad-token mechanic with attacking-targets is
// non-trivial. We log target selection but emitPartial for the actual
// myriad copies.
func registerMassOfMysteries(r *Registry) {
	r.OnTrigger("Mass of Mysteries", "combat_begin", massOfMysteriesCombatBegin)
}

func massOfMysteriesCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mass_of_mysteries_grant_myriad"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var target *gameengine.Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "elemental") {
			continue
		}
		if p.Card.BasePower > bestPower {
			bestPower = p.Card.BasePower
			target = p
		}
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_elemental_target", nil)
		return
	}
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	gameengine.SetKeywordFlags(gs, target, "myriad_until_eot")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.Card.DisplayName(),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"myriad_attack_token_copies_unimplemented")
}
