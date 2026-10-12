package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerThrakkusTheButcher wires Thrakkus the Butcher.
//
// Oracle text:
//
//	{3}{R}{G}
//	Legendary Creature — Dragon Peasant
//	Trample
//	Whenever Thrakkus attacks, double the power of each Dragon you
//	  control until end of turn.
//
// Implementation:
//   - Trample via AST keyword.
//   - "creature_attacks" trigger gated to attacker == perm: walk own
//     dragons; for each, set Flags["temp_power"] += effective_power
//     (so total = 2x power UEOT).
func registerThrakkusTheButcher(r *Registry) {
	r.OnTrigger("Thrakkus the Butcher", "creature_attacks", thrakkusAttack)
}

func thrakkusAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "thrakkus_double_dragons"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	doubled := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "dragon") {
			continue
		}
		// "Double its power" — doubling adds the Dragon's CURRENT power,
		// which means counters, until-EOT modifications and anthem-style
		// layer effects, not the printed BasePower plus a flag stamp.
		base := gs.PowerOf(p)
		if base < 0 {
			base = 0
		}
		pumpUntilEOT(gs, p, base, 0)
		doubled++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"doubled": doubled,
	})
}
