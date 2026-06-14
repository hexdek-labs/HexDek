package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerExcavaTheRisenPast wires Excava, the Risen Past.
//
// Oracle text:
//
//	Flying, haste
//	Whenever Excava attacks, return up to one target artifact, creature,
//	or non-Aura enchantment card with mana value 3 or less from your
//	graveyard to the battlefield with a finality counter on it. It's a
//	1/1 Spirit creature with flying in addition to its other types.
func registerExcavaTheRisenPast(r *Registry) {
	r.OnTrigger("Excava, the Risen Past", "attacks", excavaAttacks)
}

func excavaAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "excava_attack_recur_spirit"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever ~ attacks" fires only when this creature
	// itself is the attacker. The creature_attacks event is dispatched once per
	// declared attacker, so the old seat check (ctx["seat"] is never populated by
	// the attack fire → it only matched seat 0) let the effect multiply per
	// attacker. Mirrors the canonical gate and the Aang and Katara fix (5254f9cf).
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	bestIdx := -1
	bestCMC := -1
	for i, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > 3 {
			continue
		}
		if cardHasType(c, "land") || cardHasType(c, "planeswalker") {
			continue
		}
		isArt := cardHasType(c, "artifact")
		isCreat := cardHasType(c, "creature")
		isEnch := cardHasType(c, "enchantment") && !cardHasType(c, "aura")
		if !isArt && !isCreat && !isEnch {
			continue
		}
		if cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return
	}
	card := seat.Graveyard[bestIdx]
	gameengine.MoveCard(gs, card, perm.Controller, "graveyard", "battlefield", "excava_attack")
	// Find the permanent on the battlefield.
	var ent *gameengine.Permanent
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p != nil && p.Card == card {
			ent = p
			break
		}
	}
	if ent != nil {
		ent.AddCounter("finality", 1)
		if ent.Flags == nil {
			ent.Flags = map[string]int{}
		}
		ent.Flags["kw:flying"] = 1
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"returned": card.DisplayName(),
		"cmc":      bestCMC,
	})
}
