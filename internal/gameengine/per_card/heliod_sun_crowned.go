package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHeliodSunCrowned wires Heliod, Sun-Crowned.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Indestructible.
//	As long as your devotion to white is less than five, Heliod isn't
//	  a creature. (Each {W} in the mana costs of permanents you control
//	  counts toward your devotion to white.)
//	Whenever you gain life, put a +1/+1 counter on target creature or
//	  enchantment you control.
//	{1}{W}: Another target creature gains lifelink until end of turn.
//
// Implementation:
//   - OnETB: emitPartial for indestructible (handled by keyword pipeline) and
//     the devotion-gate creature/enchantment toggle (static ability, not
//     modelled by the trigger system; Heliod enters as an enchantment in
//     low-devotion states).
//   - OnTrigger("life_gained", ...): when Heliod's controller gains life, find
//     the best creature or enchantment they control (highest-power creature
//     first, or first enchantment if no creatures exist) and place a +1/+1
//     counter on it via p.AddCounter. This fires for every life-gain event
//     regardless of source, matching the oracle "whenever you gain life".
//   - OnActivated(0): {1}{W} lifelink-grant is noted via emitPartial; full
//     until-end-of-turn keyword tracking is not yet implemented.
//
// Coverage gaps (emitPartial):
//   - Devotion-gated creature/enchantment duality: Heliod is treated as
//     always-on for trigger purposes; the static type toggle requires a
//     continuous-effect layer that is not modelled here.
//   - Activated ability lifelink grant: UEOT keyword grants are not yet
//     tracked in the layers pipeline.
func registerHeliodSunCrowned(r *Registry) {
	r.OnETB("Heliod, Sun-Crowned", heliodSunCrownedETB)
	r.OnTrigger("Heliod, Sun-Crowned", "life_gained", heliodSunCrownedLifeGained)
	r.OnActivated("Heliod, Sun-Crowned", heliodSunCrownedActivate)
}

func heliodSunCrownedETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, "heliod_sun_crowned_devotion_gate", perm.Card.DisplayName(),
		"devotion_gated_creature_enchantment_toggle_not_modelled")
}

func heliodSunCrownedLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "heliod_sun_crowned_counter_on_life_gain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Only trigger when Heliod's controller gained the life.
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	// Find the best target: highest-power creature you control, falling back
	// to the first enchantment you control (mirrors how the oracle text
	// "target creature or enchantment you control" is used optimally — buff
	// the biggest attacker/blocker first).
	target := heliodPickCounterTarget(seat)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_valid_target", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}

	target.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"target":          target.Card.DisplayName(),
		"target_counters": target.Counters["+1/+1"],
	})
}

// heliodPickCounterTarget returns the best creature or enchantment controlled
// by seat to receive a +1/+1 counter.
//
// Priority:
//  1. Highest-power creature you control (ties broken by first-seen order).
//  2. First enchantment you control (if no creatures available).
//
// Returns nil if the seat controls no creatures or enchantments.
func heliodPickCounterTarget(seat *gameengine.Seat) *gameengine.Permanent {
	if seat == nil {
		return nil
	}

	var bestCreature *gameengine.Permanent
	bestPower := -1

	var firstEnchantment *gameengine.Permanent

	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsCreature() {
			pw := p.Card.BasePower
			if pw > bestPower {
				bestPower = pw
				bestCreature = p
			}
		} else if p.IsEnchantment() && firstEnchantment == nil {
			firstEnchantment = p
		}
	}

	if bestCreature != nil {
		return bestCreature
	}
	return firstEnchantment
}

func heliodSunCrownedActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	// {1}{W}: Another target creature gains lifelink until end of turn.
	// UEOT keyword grants require the layers pipeline — not yet modelled.
	emitPartial(gs, "heliod_sun_crowned_lifelink_grant", src.Card.DisplayName(),
		"ueot_lifelink_grant_not_modelled_by_layers_pipeline")
}
