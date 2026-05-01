package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerScrivTheObligator wires Scriv, the Obligator (Aetherdrift).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	{2}{W}{B}, 2/3 Legendary Creature — Inkling Bird
//	Flying, deathtouch
//	Whenever Scriv enters or attacks, create a white Aura enchantment
//	token named Contract attached to target creature an opponent
//	controls. The token has enchant creature and "Whenever enchanted
//	creature attacks, it gets +2/+0 until end of turn if it's attacking
//	one of your opponents. Otherwise, its controller loses 2 life."
//
// Implementation:
//   - OnETB("Scriv, the Obligator") and OnTrigger("creature_attacks")
//     for Scriv herself both create a "Contract" Aura token attached
//     to the highest-power opponent creature.
//   - OnTrigger("Contract", "creature_attacks") fires each Contract
//     token's nested ability: when the creature it's attached to
//     attacks, either grant +2/+0 (if attacking a Contract-controller
//     opponent) or burn the attacker's controller for 2 life.
func registerScrivTheObligator(r *Registry) {
	r.OnETB("Scriv, the Obligator", scrivETB)
	r.OnTrigger("Scriv, the Obligator", "creature_attacks", scrivAttackTrigger)
	r.OnTrigger("Contract", "creature_attacks", contractAttackTrigger)
}

func scrivETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	scrivCreateContract(gs, perm, "etb")
}

func scrivAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	scrivCreateContract(gs, perm, "attack")
}

// scrivCreateContract attaches a fresh Contract Aura token to the
// highest-power creature controlled by an opponent of Scriv's
// controller.
func scrivCreateContract(gs *gameengine.GameState, perm *gameengine.Permanent, reason string) {
	const slug = "scriv_the_obligator_contract"
	if gs == nil || perm == nil {
		return
	}
	scrivSeat := perm.Controller
	target := pickContractTarget(gs, scrivSeat)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent_creature_target", map[string]interface{}{
			"seat":   scrivSeat,
			"reason": reason,
		})
		return
	}

	tokenCard := &gameengine.Card{
		Name:   "Contract",
		Owner:  scrivSeat,
		Types:  []string{"token", "enchantment", "aura"},
		Colors: []string{"W"},
	}
	contract := createPermanent(gs, scrivSeat, tokenCard, false)
	if contract == nil {
		return
	}
	contract.AttachedTo = target
	gameengine.RegisterReplacementsForPermanent(gs, contract)
	gameengine.FirePermanentETBTriggers(gs, contract)

	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   scrivSeat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":         "Contract",
			"reason":        "scriv_" + reason,
			"attached_to":   target.Card.DisplayName(),
			"attached_seat": target.Controller,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          scrivSeat,
		"reason":        reason,
		"attached_to":   target.Card.DisplayName(),
		"attached_seat": target.Controller,
	})
}

// pickContractTarget returns the highest-power creature controlled by
// any opponent of scrivSeat, or nil if none exists.
func pickContractTarget(gs *gameengine.GameState, scrivSeat int) *gameengine.Permanent {
	if gs == nil || scrivSeat < 0 || scrivSeat >= len(gs.Seats) {
		return nil
	}
	var best *gameengine.Permanent
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == scrivSeat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if best == nil || p.Power() > best.Power() {
				best = p
			}
		}
	}
	return best
}

// contractAttackTrigger is the Aura token's nested ability: when the
// enchanted creature attacks, +2/+0 if it's attacking one of the
// Contract controller's opponents, else its controller loses 2 life.
func contractAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "scriv_contract_attack"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || perm.AttachedTo != atk {
		return
	}
	contractCtrl := perm.Controller
	defenderSeat, ok := gameengine.AttackerDefender(atk)
	if !ok {
		return
	}
	// "Attacking one of your opponents" — opponents of the Contract's
	// controller (the player who created it via Scriv).
	attackingOpp := defenderSeat != contractCtrl &&
		defenderSeat >= 0 && defenderSeat < len(gs.Seats) &&
		gs.Seats[defenderSeat] != nil && !gs.Seats[defenderSeat].Lost

	// De-dupe per (attacker, turn) so the trigger fires once per attack
	// declaration rather than once per priority round.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupe := fmt.Sprintf("contract_attack_t%d", gs.Turn+1)
	if perm.Flags[dedupe] == 1 {
		return
	}
	perm.Flags[dedupe] = 1

	if attackingOpp {
		atk.Modifications = append(atk.Modifications, gameengine.Modification{
			Power:    2,
			Duration: "until_end_of_turn",
		})
		emit(gs, slug, "Contract", map[string]interface{}{
			"contract_seat": contractCtrl,
			"attacker":      atk.Card.DisplayName(),
			"defender_seat": defenderSeat,
			"effect":        "+2/+0",
		})
		return
	}
	// Otherwise — attacker's controller loses 2 life.
	atkSeat := atk.Controller
	if atkSeat >= 0 && atkSeat < len(gs.Seats) && gs.Seats[atkSeat] != nil {
		gs.Seats[atkSeat].Life -= 2
		gs.LogEvent(gameengine.Event{
			Kind:   "life_loss",
			Seat:   atkSeat,
			Target: atkSeat,
			Source: "Contract",
			Amount: 2,
			Details: map[string]interface{}{
				"reason": "contract_attacker_not_targeting_opponent",
			},
		})
	}
	emit(gs, slug, "Contract", map[string]interface{}{
		"contract_seat": contractCtrl,
		"attacker":      atk.Card.DisplayName(),
		"defender_seat": defenderSeat,
		"effect":        "life_loss_2",
	})
}
