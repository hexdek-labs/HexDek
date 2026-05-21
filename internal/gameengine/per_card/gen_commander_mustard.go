package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCommanderMustard wires Commander Mustard.
//
// Oracle text:
//
//	Vigilance
//	Other Soldiers you control have vigilance, trample, and haste.
//	{2}{R}{W}: Until end of turn, Soldiers you control gain "Whenever this
//	creature attacks, it deals 1 damage to defending player."
//
// Implementation:
//   - Activated ability gates on `seat.ManaPool >= 4` ({2}{R}{W} = 4
//     generic for the engine's colorless pool). Sets a per-seat
//     `mustard_soldier_attack_ping` flag for the combat layer to read
//     and queues a delayed trigger to clear the flag at end of turn.
//   - The "this creature attacks → 1 damage" rider itself relies on the
//     combat layer reading the flag during attacker resolution; the
//     handler tracks both the cost gate and the duration with a partial
//     breadcrumb so audits can find the wiring boundary.
func registerCommanderMustard(r *Registry) {
	r.OnActivated("Commander Mustard", commanderMustardActivate)
	// R51 batch H port: when ANY creature attacks while the grant is
	// active, check if it's a Soldier controlled by Mustard's
	// controller. If so, deal 1 damage to the defending player. This
	// is the runtime realization of the granted "Whenever this
	// creature attacks, it deals 1 damage to defending player" ability.
	r.OnTrigger("Commander Mustard", "creature_attacks", commanderMustardSoldierAttackPing)
}

func commanderMustardSoldierAttackPing(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "commander_mustard_soldier_attack_ping_fire"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil || seat.Flags["mustard_soldier_attack_ping"] == 0 {
		return
	}
	attacker, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attacker == nil || attacker.Card == nil || attacker.Controller != perm.Controller {
		return
	}
	if !cardHasSubtype(attacker.Card, "soldier") {
		return
	}
	// Defender seat: prefer attacker.AttackingSeat / defending_seat in
	// ctx; fall back to the lowest-life living opponent.
	defender := -1
	if v, ok := ctx["defender_seat"].(int); ok {
		defender = v
	} else if v, ok := ctx["defending_seat"].(int); ok {
		defender = v
	}
	if defender < 0 {
		bestLife := 1 << 30
		for _, opp := range gs.Opponents(perm.Controller) {
			s := gs.Seats[opp]
			if s == nil || s.Lost {
				continue
			}
			if s.Life < bestLife {
				bestLife = s.Life
				defender = opp
			}
		}
	}
	if defender < 0 {
		return
	}
	gameengine.DealDamage(gs, defender, 1, "Commander Mustard (Soldier attack ping)")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"attacker":      attacker.Card.DisplayName(),
		"defender_seat": defender,
		"damage":        1,
	})
}

func commanderMustardActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "commander_mustard_soldier_attack_ping"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if !payManaFromPool(seat, 4) {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"required":  4,
			"mana_pool": seat.ManaPool,
		})
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["mustard_soldier_attack_ping"]++
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: seatIdx,
		SourceCardName: src.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			s := gs.Seats[seatIdx]
			if s == nil || s.Flags == nil {
				return
			}
			if s.Flags["mustard_soldier_attack_ping"] > 0 {
				s.Flags["mustard_soldier_attack_ping"]--
			}
			if s.Flags["mustard_soldier_attack_ping"] <= 0 {
				delete(s.Flags, "mustard_soldier_attack_ping")
			}
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  seatIdx,
		"stack": seat.Flags["mustard_soldier_attack_ping"],
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"soldier-attacks-damage rider relies on combat layer reading mustard_soldier_attack_ping flag")
}
