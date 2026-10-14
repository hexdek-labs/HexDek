package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVenserCorpsePuppet wires Venser, Corpse Puppet.
//
// Oracle text:
//
//	Lifelink, toxic 1
//	Whenever you proliferate, choose one —
//	• If you don't control a creature named The Hollow Sentinel, create
//	  The Hollow Sentinel, a legendary 3/3 colorless Phyrexian Golem
//	  artifact creature token.
//	• Target artifact creature you control gains flying and lifelink
//	  until end of turn.
//
// Implementation:
//   - Listen on "proliferate" trigger. When we proliferate and don't yet
//     control The Hollow Sentinel, create the token; otherwise grant
//     flying + lifelink until end of turn to the highest-power artifact
//     creature we control.
//   - Lifelink and toxic 1 are keywords (engine-handled).
func registerVenserCorpsePuppet(r *Registry) {
	r.OnTrigger("Venser, Corpse Puppet", "proliferate", venserCorpsePuppetProliferate)
}

func venserCorpsePuppetProliferate(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "venser_corpse_puppet_proliferate"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	proliferatorSeat, ok := ctx["controller_seat"].(int)
	if !ok {
		proliferatorSeat, _ = ctx["seat"].(int)
	}
	if proliferatorSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Mode 1: create The Hollow Sentinel if not already controlled.
	hasSentinel := false
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.Name == "The Hollow Sentinel" || p.Card.DisplayName() == "The Hollow Sentinel" {
			hasSentinel = true
			break
		}
	}
	if !hasSentinel {
		token := &gameengine.Card{
			Name:          "The Hollow Sentinel",
			Owner:         perm.Controller,
			BasePower:     3,
			BaseToughness: 3,
			Types:         []string{"token", "legendary", "artifact", "creature", "phyrexian", "golem"},
			TypeLine:      "Legendary Token Artifact Creature — Phyrexian Golem",
		}
		enterBattlefieldWithETB(gs, perm.Controller, token, false)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
			"mode": "make_sentinel",
		})
		return
	}

	// Mode 2: grant flying + lifelink to a target artifact creature we control.
	var best *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "artifact") {
			continue
		}
		if best == nil || p.Power() > best.Power() {
			best = p
		}
	}
	if best == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_artifact_creature_target", nil)
		return
	}
	if best.Flags == nil {
		best.Flags = map[string]int{}
	}
	best.Flags["kw:flying"] = 1
	best.Flags["kw:lifelink"] = 1
	captured := best
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			gameengine.ClearKeywordFlags(gs, captured, "flying")
			gameengine.ClearKeywordFlags(gs, captured, "lifelink")
		},
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"mode":   "fly_lifelink",
		"target": best.Card.DisplayName(),
	})
}
