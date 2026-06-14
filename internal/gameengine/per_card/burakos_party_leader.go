package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBurakosPartyLeader wires Burakos, Party Leader.
//
// Oracle text:
//
//	Burakos is also a Cleric, Rogue, Warrior, and Wizard.
//	Whenever Burakos attacks, defending player loses X life and you
//	create X Treasure tokens, where X is the number of creatures in
//	your party.
//
// Party = up to one Cleric, Rogue, Warrior, and Wizard you control. Max
// X = 4.
func registerBurakosPartyLeader(r *Registry) {
	r.OnTrigger("Burakos, Party Leader", "attacks", burakosAttacks)
}

func burakosAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "burakos_party_attack"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever Burakos, Party Leader attacks" fires only
	// when Burakos itself attacks. creature_attacks is dispatched once per
	// declared attacker, so without this Burakos drained life + minted Treasure
	// for EVERY creature the controller attacked with. See the Aang and Katara
	// fix (5254f9cf). (The attacker_seat check below is now subsumed.)
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	// Engine fires creature_attacks with attacker_perm + attacker_seat;
	// defender is stored on the attacker via setAttackerDefender. Legacy
	// "seat" / "defender_seat" reads kept as fallbacks for callers that
	// thread them explicitly.
	attackerSeat, ok := ctx["attacker_seat"].(int)
	if !ok {
		attackerSeat, _ = ctx["seat"].(int)
	}
	if attackerSeat != perm.Controller {
		return
	}
	defenderSeat := -1
	if atk, ok := ctx["attacker_perm"].(*gameengine.Permanent); ok && atk != nil {
		if d, found := gameengine.AttackerDefender(atk); found {
			defenderSeat = d
		}
	}
	if defenderSeat < 0 {
		defenderSeat, _ = ctx["defender_seat"].(int)
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	hasCleric, hasRogue, hasWarrior, hasWizard := false, false, false, false
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "cleric") {
			hasCleric = true
		}
		if cardHasType(p.Card, "rogue") {
			hasRogue = true
		}
		if cardHasType(p.Card, "warrior") {
			hasWarrior = true
		}
		if cardHasType(p.Card, "wizard") {
			hasWizard = true
		}
	}
	x := 0
	for _, b := range []bool{hasCleric, hasRogue, hasWarrior, hasWizard} {
		if b {
			x++
		}
	}
	if x <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"party":   0,
			"life":    0,
			"tokens":  0,
		})
		return
	}
	gameengine.LoseLife(gs, defenderSeat, x, perm.Card.DisplayName())
	for i := 0; i < x; i++ {
		gameengine.CreateTreasureToken(gs, perm.Controller)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"party":  x,
		"life":   x,
		"tokens": x,
	})
}
