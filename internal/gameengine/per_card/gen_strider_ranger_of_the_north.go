package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStriderRangerOfTheNorth wires Strider, Ranger of the North.
//
// Oracle text:
//
//	Landfall — Whenever a land you control enters, target creature gets
//	+1/+1 until end of turn. Then if that creature has power 4 or
//	greater, it gains first strike until end of turn.
//
// Implementation:
//   - "permanent_etb" trigger filtered to lands controlled by Strider's
//     controller. Picks the highest-power controlled creature as the
//     +1/+1 target (snowballs the strongest threat).
//   - +1/+1 modeled via plus_power/plus_toughness_until_eot flags the
//     engine cleanup pass zeroes at end of turn.
//   - If post-bump power >= 4, also stamp kw:first_strike (cleared at EOT).
func registerStriderRangerOfTheNorth(r *Registry) {
	r.OnTrigger("Strider, Ranger of the North", "permanent_etb", striderLandfall)
}

func striderLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "strider_landfall_pump"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || !entering.IsLand() {
		return
	}
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var pick *gameengine.Permanent
	bestPower := -1 << 30
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Power() > bestPower {
			bestPower = p.Power()
			pick = p
		}
	}
	if pick == nil {
		return
	}
	if pick.Flags == nil {
		pick.Flags = map[string]int{}
	}
	pumpUntilEOT(gs, pick, 1, 1)
	pick.Flags["plus_power_until_eot"]++
	pick.Flags["plus_toughness_until_eot"]++

	postPower := gs.PowerOf(pick)
	if postPower >= 4 {
		grantKeywordUntilEOT(gs, pick, "first strike")
		gameengine.SetKeywordFlags(gs, pick, "first_strike")
		pick.Flags["kw_eot:first_strike"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"land":         entering.Card.DisplayName(),
		"target":       pick.Card.DisplayName(),
		"post_power":   postPower,
		"first_strike": postPower >= 4,
	})
}
