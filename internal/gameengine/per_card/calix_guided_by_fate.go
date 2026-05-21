package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCalixGuidedByFate wires Calix, Guided by Fate.
//
// Oracle text:
//
//	Constellation — Whenever Calix or another enchantment you control
//	enters, put a +1/+1 counter on target creature.
//	Whenever Calix or an enchanted creature you control deals combat
//	damage to a player, you may create a token that's a copy of a
//	nonlegendary enchantment you control. Do this only once each turn.
//
// Constellation: pick best other creature to bulk up. Combat copy is
// non-trivial — emitPartial.
func registerCalixGuidedByFate(r *Registry) {
	r.OnETB("Calix, Guided by Fate", calixSelfETB)
	r.OnTrigger("Calix, Guided by Fate", "permanent_etb", calixConstellation)
	r.OnTrigger("Calix, Guided by Fate", "combat_damage_player", calixCombatCopy)
}

func calixSelfETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	calixAddCounter(gs, perm)
}

func calixConstellation(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered == perm || entered.Card == nil {
		return
	}
	if !cardHasType(entered.Card, "enchantment") {
		return
	}
	calixAddCounter(gs, perm)
}

func calixAddCounter(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "calix_constellation"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var best *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if pow := p.Power(); pow > bestPow {
			best = p
			bestPow = pow
		}
	}
	if best == nil {
		return
	}
	best.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": best.Card.DisplayName(),
	})
}

// calixCombatCopy implements the once-per-turn "create a token copy of
// a nonlegendary enchantment you control" rider. Triggers on combat
// damage to a player by Calix or by any enchanted creature you control.
// R52 batchM port.
func calixCombatCopy(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "calix_combat_enchantment_copy"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// Once-per-turn gate.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["calix_combat_copy_turn"] == gs.Turn+1 {
		return
	}
	// Filter: source must be Calix herself OR an enchanted creature the
	// controller controls.
	srcPerm, _ := ctx["source_perm"].(*gameengine.Permanent)
	if srcPerm == nil {
		srcPerm, _ = ctx["attacker_perm"].(*gameengine.Permanent)
	}
	if srcPerm != nil && srcPerm != perm {
		if srcPerm.Controller != perm.Controller {
			return
		}
		if !srcPerm.IsCreature() {
			return
		}
		// "Enchanted" = has an aura attached. Approximate by checking
		// AttachedTo on any of the controller's auras pointing at srcPerm.
		seat := gs.Seats[perm.Controller]
		if seat == nil {
			return
		}
		enchanted := false
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !cardHasSubtype(p.Card, "aura") {
				continue
			}
			if p.AttachedTo == srcPerm {
				enchanted = true
				break
			}
		}
		if !enchanted {
			return
		}
	}

	// Pick the highest-CMC nonlegendary enchantment we control as the
	// copy target.
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var target *gameengine.Permanent
	bestCMC := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "enchantment") {
			continue
		}
		if cardHasType(p.Card, "legendary") {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc > bestCMC {
			bestCMC = cmc
			target = p
		}
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_nonlegendary_enchantment_to_copy", nil)
		return
	}
	copyCard := target.Card.DeepCopy()
	copyCard.IsCopy = true
	copyCard.Owner = perm.Controller
	// Drop the legendary supertype on the copy if any leaked in (shouldn't,
	// since we filtered) — defense in depth.
	filtered := copyCard.Types[:0]
	for _, t := range copyCard.Types {
		if t == "legendary" || t == "Legendary" {
			continue
		}
		filtered = append(filtered, t)
	}
	copyCard.Types = append(filtered, "token")
	enterBattlefieldWithETB(gs, perm.Controller, copyCard, false)
	perm.Flags["calix_combat_copy_turn"] = gs.Turn + 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"copied": target.Card.DisplayName(),
	})
}
