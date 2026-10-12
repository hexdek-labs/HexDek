package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAdrianaCaptainOfTheGuard wires Adriana, Captain of the Guard.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{3}{R}{W}
//	Legendary Creature — Human Knight
//	Melee (Whenever this creature attacks, it gets +1/+1 until end of
//	turn for each opponent you attacked this combat.)
//	Other creatures you control have melee.
//
// Implementation:
//   - declare_attackers gated to controller: count distinct opponent
//     defenders attacked this combat, grant melee bonus to every
//     attacking creature controlled by us. The bonus is +N/+N where N is
//     the count of distinct opponent-defenders. We use temp_power /
//     temp_toughness flags consumed by combat math.
func registerAdrianaCaptainOfTheGuard(r *Registry) {
	r.OnTrigger("Adriana, Captain of the Guard", "declare_attackers", adrianaMelee)
}

func adrianaMelee(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "adriana_melee_anthem"
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
	// Count distinct opponents attacked. Engine tracks attacker.AttackTarget
	// (a seat index for player target, -1 for planeswalker etc.). We can't
	// see planeswalker attacks here cleanly; count seat targets.
	defenderSeats := map[int]bool{}
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if !p.IsAttacking() {
			continue
		}
		if def, ok := gameengine.AttackerDefender(p); ok && def != perm.Controller {
			defenderSeats[def] = true
		}
	}
	bonus := len(defenderSeats)
	if bonus == 0 {
		return
	}
	pumped := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || !p.IsAttacking() {
			continue
		}
		if def, ok := gameengine.AttackerDefender(p); !ok || def == perm.Controller {
			continue
		}
		pumpUntilEOT(gs, p, bonus, bonus)
		pumped++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":               perm.Controller,
		"opponents_attacked": bonus,
		"creatures_pumped":   pumped,
	})
}
