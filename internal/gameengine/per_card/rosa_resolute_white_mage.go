package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRosaResoluteWhiteMage wires Rosa, Resolute White Mage.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Reach
//	At the beginning of combat on your turn, put a +1/+1 counter on
//	target creature you control. It gains lifelink until end of turn.
//
// Implementation:
//   - Reach handled by AST keyword pipeline.
//   - "combat_begin" trigger picks the highest-power friendly creature
//     (Rosa included), grants it a +1/+1 counter and sets a temporary
//     "lifelink_eot" flag the engine combat-damage step honors.
func registerRosaResoluteWhiteMage(r *Registry) {
	r.OnTrigger("Rosa, Resolute White Mage", "combat_begin", rosaCombatBegin)
}

func rosaCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rosa_combat_begin_buff"
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
	var best *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Power() > bestPow {
			bestPow = p.Power()
			best = p
		}
	}
	if best == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_creature_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	best.AddCounter("+1/+1", 1)
	grantKeywordUntilEOT(gs, best, "lifelink")
	best.Flags["lifelink_eot"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": best.Card.DisplayName(),
	})
}
