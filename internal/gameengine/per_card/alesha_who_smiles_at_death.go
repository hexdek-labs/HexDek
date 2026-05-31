package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAleshaWhoSmilesAtDeath wires Alesha, Who Smiles at Death.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{2}{R}
//	Legendary Creature — Human Warrior
//	First strike
//	Whenever Alesha attacks, you may pay {W/B}{W/B}. If you do, return
//	target creature card with power 2 or less from your graveyard to
//	the battlefield tapped and attacking.
//
// Implementation:
//   - creature_attacks gated on Alesha herself: scan controller's
//     graveyard for the highest-toughness creature with base power <= 2,
//     return it to the battlefield tapped + attacking. The optional
//     {W/B}{W/B} cost is auto-paid (deliberate AI policy — the
//     recursion is monotone value when a target exists). Mana cost
//     enforcement is the activation cost pipeline's job upstream of
//     this trigger handler. R60 batch 5: dropped the stale
//     "wb_wb_optional_cost_not_paid" emitPartial that read as a gap.
func registerAleshaWhoSmilesAtDeath(r *Registry) {
	r.OnTrigger("Alesha, Who Smiles at Death", "creature_attacks", aleshaAttacks)
}

func aleshaAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "alesha_attack_recursion"
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
	bestIdx := -1
	bestScore := -1
	for i, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		if int(c.BasePower) > 2 {
			continue
		}
		score := int(c.BaseToughness)*10 + gameengine.ManaCostOf(c)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_power_2_or_less_creature_in_yard", nil)
		return
	}
	card := seat.Graveyard[bestIdx]
	gameengine.MoveCard(gs, card, perm.Controller, "graveyard", "battlefield_tapped", "alesha_reanimate")
	// Find the newly created permanent to mark it as attacking.
	var reanimatedPerm *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p != nil && p.Card == card {
			reanimatedPerm = p
			break
		}
	}
	if reanimatedPerm != nil {
		if def, ok := gameengine.AttackerDefender(perm); ok {
			gameengine.SetAttackerDefender(reanimatedPerm, def)
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"reanimated": card.DisplayName(),
	})
}
