package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheRani wires The Rani.
//
// Oracle text:
//
//	{1}{U}{B}{R}
//	Legendary Creature — Time Lord Scientist
//	Whenever The Rani enters or attacks, create a red Aura enchantment
//	  token named Mark of the Rani attached to another target creature.
//	  That token has enchant creature and "Enchanted creature gets +2/+2
//	  and is goaded."
//	Whenever a goaded creature deals combat damage to one of your
//	  opponents, investigate.
//
// Implementation:
//   - ETB / "creature_attacks" (gated to The Rani): pick another target
//     creature on any battlefield (heuristic: largest opponent creature
//     to goad it back at its owner; falls back to any other creature).
//     Create a Mark of the Rani aura token attached to it; the static
//     pump (+2/+2) is recorded on the host's Flags["temp_power"] and
//     Flags["temp_toughness"] and the goaded marker on Flags["goaded"]
//     (these flags are not strictly UEOT — they persist while the aura
//     is attached; emitPartial covers the layers gap).
//   - "creature_combat_damage_to_player" trigger gated to defender being
//     the controller's opponent and the attacker being goaded: create a
//     Clue token (investigate).
func registerTheRani(r *Registry) {
	r.OnETB("The Rani", theRaniMakeMark)
	r.OnTrigger("The Rani", "creature_attacks", theRaniAttackMakeMark)
	r.OnTrigger("The Rani", "creature_combat_damage_to_player", theRaniInvestigate)
}

func theRaniMakeMark(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	theRaniCreateMark(gs, perm, "the_rani_etb_mark")
}

func theRaniAttackMakeMark(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm == nil || atkPerm != perm {
		return
	}
	theRaniCreateMark(gs, perm, "the_rani_attack_mark")
}

func theRaniCreateMark(gs *gameengine.GameState, perm *gameengine.Permanent, slug string) {
	target := theRaniPickTarget(gs, perm)
	if target == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"reason": "no_target_creature",
		})
		return
	}
	auraToken := &gameengine.Card{
		Name:     "Mark of the Rani",
		Owner:    perm.Controller,
		Types:    []string{"token", "enchantment", "aura"},
		Colors:   []string{"R"},
		TypeLine: "Token Enchantment — Aura",
	}
	aura := createPermanent(gs, perm.Controller, auraToken, false)
	if aura != nil {
		aura.AttachedTo = target
	}
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["temp_power"] += 2
	target.Flags["temp_toughness"] += 2
	target.Flags["goaded"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.Card.DisplayName(),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"aura_pump_and_goad_persistence_uses_flags_partial")
}

func theRaniPickTarget(gs *gameengine.GameState, perm *gameengine.Permanent) *gameengine.Permanent {
	// Prefer largest opponent creature.
	var best *gameengine.Permanent
	bestPow := -1
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if p.Card.BasePower > bestPow {
				bestPow = p.Card.BasePower
				best = p
			}
		}
	}
	if best != nil {
		return best
	}
	// Fall back to any non-self friendly creature.
	seat := gs.Seats[perm.Controller]
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if p.IsCreature() {
			return p
		}
	}
	return nil
}

func theRaniInvestigate(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_rani_investigate"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm == nil || atkPerm.Flags == nil || atkPerm.Flags["goaded"] != 1 {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat == perm.Controller {
		return
	}
	gameengine.CreateClueToken(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
