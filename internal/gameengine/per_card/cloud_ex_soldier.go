package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCloudExSoldier wires Cloud, Ex-SOLDIER.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Haste
//	When Cloud enters, attach up to one target Equipment you control to
//	it.
//	Whenever Cloud attacks, draw a card for each equipped attacking
//	creature you control. Then if Cloud has power 7 or greater, create
//	two Treasure tokens.
//
// Implementation:
//   - Haste — handled by the AST keyword pipeline.
//   - OnETB: walk Cloud's controller's battlefield for any Equipment.
//     Prefer an unattached Equipment (no current AttachedTo) over one
//     stuck on a sub-optimal host; otherwise re-attach the highest-MV
//     Equipment to Cloud. Equipment is only re-attached during the
//     equip-cost window in normal play, so this trigger is the cleanest
//     way to land a +/+ buff on Cloud immediately.
//   - "creature_attacks": when Cloud attacks, count attacking creatures
//     this controller controls that have at least one Equipment attached
//     (any Equipment whose AttachedTo points at the creature). Draw that
//     many cards. Then, if Cloud's current Power() is 7+, mint two
//     Treasure tokens.
func registerCloudExSoldier(r *Registry) {
	r.OnETB("Cloud, Ex-SOLDIER", cloudExSoldierETB)
	r.OnTrigger("Cloud, Ex-SOLDIER", "creature_attacks", cloudExSoldierAttacks)
}

func cloudExSoldierETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cloud_ex_soldier_etb_attach_equipment"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	var bestEq *gameengine.Permanent
	bestScore := -1
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cloudIsEquipment(p.Card) {
			continue
		}
		// Score: prefer unattached (already detached → no swap needed) and
		// higher mana value (better buff in expectation).
		score := cardCMC(p.Card) * 2
		if p.AttachedTo == nil {
			score += 100
		}
		// Don't yank an Equipment off a creature with higher power than Cloud.
		if p.AttachedTo != nil && p.AttachedTo.IsCreature() &&
			p.AttachedTo.Power() > perm.Power() {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestEq = p
		}
	}

	if bestEq == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_equipment_to_attach", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}

	prior := bestEq.AttachedTo
	bestEq.AttachedTo = perm
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"equipment": bestEq.Card.DisplayName(),
		"detached_from": cloudPermName(prior),
	})
}

func cloudExSoldierAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cloud_ex_soldier_attack_draw_treasure"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Count this controller's attacking creatures with an Equipment
	// attached. The set of attached Equipment lives on other permanents
	// pointing at the attacker, so we walk the battlefield.
	equippedSet := map[*gameengine.Permanent]bool{}
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cloudIsEquipment(p.Card) {
			continue
		}
		host := p.AttachedTo
		if host == nil || host.Controller != perm.Controller {
			continue
		}
		if !host.IsCreature() || !host.IsAttacking() {
			continue
		}
		equippedSet[host] = true
	}

	drawN := len(equippedSet)
	drawn := 0
	for i := 0; i < drawN && len(seat.Library) > 0; i++ {
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, perm.Controller, "library", "hand", "draw")
		drawn++
	}

	treasuresMinted := 0
	if perm.Power() >= 7 {
		gameengine.CreateTreasureToken(gs, perm.Controller)
		gameengine.CreateTreasureToken(gs, perm.Controller)
		treasuresMinted = 2
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"equipped_attackers": drawN,
		"drawn":             drawn,
		"cloud_power":       perm.Power(),
		"treasures":         treasuresMinted,
	})
}

func cloudIsEquipment(card *gameengine.Card) bool {
	if card == nil {
		return false
	}
	for _, t := range card.Types {
		if strings.EqualFold(t, "equipment") {
			return true
		}
	}
	return false
}

func cloudPermName(p *gameengine.Permanent) string {
	if p == nil || p.Card == nil {
		return ""
	}
	return p.Card.DisplayName()
}
