package per_card

import (
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJenovaAncientCalamity wires Jenova, Ancient Calamity (FF set).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary Creature — Alien (1/5, {2}{B}{G})
//	At the beginning of combat on your turn, put a number of +1/+1
//	counters equal to Jenova's power on up to one other target creature.
//	That creature becomes a Mutant in addition to its other types.
//	Whenever a Mutant you control dies during your turn, you draw cards
//	equal to its power.
//
// Implementation:
//   - "combat_begin" (gated to active_seat == controller): pick the best
//     other friendly creature and stack +1/+1 counters equal to Jenova's
//     current power on it. Append "mutant" to the target Card.Types if
//     not already present (this models the printed-type grant; the
//     actual layer-7 effect is too narrow for the engine's continuous-
//     effect framework, but tagging the card enables the death-trigger
//     branch below).
//   - "creature_dies" (gated to controller_seat == Jenova's controller
//     AND active player == Jenova's controller): if the dying creature
//     was a Mutant, draw cards equal to its power at time of death. The
//     dying perm carries its final power on its Permanent reference;
//     fall back to BasePower if zero/negative.
func registerJenovaAncientCalamity(r *Registry) {
	r.OnTrigger("Jenova, Ancient Calamity", "combat_begin", jenovaCombatBegin)
	r.OnTrigger("Jenova, Ancient Calamity", "creature_dies", jenovaMutantDies)
}

func jenovaCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jenova_ancient_calamity_combat_grow"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	power := perm.Power()
	if power <= 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "jenova_power_nonpositive", map[string]interface{}{
			"seat":  perm.Controller,
			"power": power,
		})
		return
	}
	target := jenovaPickCounterTarget(gs, perm)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_other_creature_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	target.AddCounter("+1/+1", power)
	addedMutant := jenovaTagMutant(target)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"target":       target.Card.DisplayName(),
		"counters":     power,
		"became_mutant": addedMutant,
	})
}

func jenovaPickCounterTarget(gs *gameengine.GameState, src *gameengine.Permanent) *gameengine.Permanent {
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == src || !p.IsCreature() {
			continue
		}
		pw := p.Power()
		if pw > bestPower {
			bestPower = pw
			best = p
			continue
		}
		if pw == bestPower && best != nil && p.Timestamp < best.Timestamp {
			best = p
		}
	}
	return best
}

func jenovaTagMutant(target *gameengine.Permanent) bool {
	if target == nil || target.Card == nil {
		return false
	}
	for _, t := range target.Card.Types {
		if strings.EqualFold(t, "mutant") {
			return false
		}
	}
	target.Card.Types = append(target.Card.Types, "mutant")
	return true
}

func jenovaMutantDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "jenova_mutant_dies_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if gs.Active != perm.Controller {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	dyingCard, _ := ctx["card"].(*gameengine.Card)
	if dying == nil && dyingCard == nil {
		return
	}
	if dying != nil && !jenovaIsMutant(dying.Card) {
		return
	}
	if dying == nil && !jenovaIsMutant(dyingCard) {
		return
	}
	power := 0
	if dying != nil {
		power = dying.Power()
	}
	if power <= 0 && dyingCard != nil {
		power = dyingCard.BasePower
	}
	if power <= 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "dying_mutant_zero_power", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	drawn := 0
	for i := 0; i < power; i++ {
		if c := drawOne(gs, perm.Controller, perm.Card.DisplayName()); c == nil {
			break
		}
		drawn++
	}
	dyingName := ""
	if dyingCard != nil {
		dyingName = dyingCard.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"mutant": dyingName,
		"power":  power,
		"drew":   drawn,
		"turn":   fmt.Sprintf("%d", gs.Turn),
	})
}

func jenovaIsMutant(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "mutant") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(c.TypeLine), "mutant")
}
