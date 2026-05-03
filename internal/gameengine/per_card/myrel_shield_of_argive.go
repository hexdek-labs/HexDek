package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMyrelShieldOfArgive wires Myrel, Shield of Argive.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	During your turn, your opponents can't cast spells or activate
//	abilities of artifacts, creatures, or enchantments.
//	Whenever Myrel, Shield of Argive attacks, create X 1/1 colorless
//	Soldier artifact creature tokens, where X is the number of Soldiers
//	you control.
//
// Implementation:
//   - OnETB: sets seat.Flags["myrel_silence_active"] = 1 so the engine's
//     CastSpell / ActivateAbility paths can enforce the static silencing
//     during the controller's turn. The flag marks "this seat has Myrel";
//     the engine must additionally check whose turn it is and whether the
//     acting player is an opponent of the flagged seat.
//   - OnTrigger("permanent_ltb"): when Myrel herself leaves the
//     battlefield, remove the silence flag from her controller's seat.
//   - OnTrigger("creature_attacks"): when the attacker is Myrel, count
//     the number of Soldiers on the controller's battlefield and create
//     that many 1/1 colorless Soldier artifact creature tokens.
//
// emitPartial gaps:
//   - Static silencing: the engine's CastSpell and ActivateAbility paths
//     do not yet check seat.Flags["myrel_silence_active"] to block
//     opponent spells/abilities during the controller's turn.
//
// Token spec:
//
//	Name="Soldier", Power=1, Toughness=1,
//	Types=["creature","artifact","soldier"], Colors=[] (colorless).
func registerMyrelShieldOfArgive(r *Registry) {
	r.OnETB("Myrel, Shield of Argive", myrelShieldETB)
	r.OnTrigger("Myrel, Shield of Argive", "permanent_ltb", myrelShieldLTB)
	r.OnTrigger("Myrel, Shield of Argive", "creature_attacks", myrelShieldAttackTrigger)
}

// myrelShieldETB sets the silence flag on the controller's seat so the
// engine knows to block opponent spells/abilities during this player's turn.
func myrelShieldETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "myrel_shield_of_argive_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	if s.Flags == nil {
		s.Flags = map[string]int{}
	}

	s.Flags["myrel_silence_active"] = 1

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"effect": "silence_flag_set",
	})

	emitPartial(gs, slug, perm.Card.DisplayName(),
		"static silencing: engine CastSpell/ActivateAbility paths do not yet check seat.Flags[\"myrel_silence_active\"] to block opponent spells and artifact/creature/enchantment abilities during controller's turn")
}

// myrelShieldLTB fires on "permanent_ltb" for every permanent that leaves
// the battlefield. We gate on the leaving permanent being Myrel herself
// (name match) to clean up the silence flag.
func myrelShieldLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "myrel_shield_of_argive_ltb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	leavingPerm, _ := ctx["perm"].(*gameengine.Permanent)
	if leavingPerm == nil || leavingPerm.Card == nil {
		return
	}
	if normalizeName(leavingPerm.Card.DisplayName()) != normalizeName("Myrel, Shield of Argive") {
		return
	}
	// Only clean up flags for OUR seat (the Myrel that left).
	if leavingPerm.Controller != perm.Controller {
		return
	}

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Flags == nil {
		return
	}

	delete(s.Flags, "myrel_silence_active")

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"effect": "silence_flag_removed",
	})
}

// myrelShieldAttackTrigger fires on "creature_attacks". We gate on the
// attacker being Myrel herself, then count all Soldiers on the controller's
// battlefield and create that many 1/1 colorless Soldier artifact creature
// tokens.
func myrelShieldAttackTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "myrel_shield_of_argive_attack_tokens"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}

	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Count Soldiers on the controller's battlefield.
	soldierCount := 0
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "soldier") {
				soldierCount++
				break
			}
		}
	}

	if soldierCount == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     seat,
			"soldiers": 0,
			"tokens":   0,
			"reason":   "no_soldiers_on_battlefield",
		})
		return
	}

	// Create X 1/1 colorless Soldier artifact creature tokens.
	for i := 0; i < soldierCount; i++ {
		gameengine.CreateCreatureToken(
			gs,
			seat,
			"Soldier",
			[]string{"creature", "artifact", "soldier"},
			1, 1,
		)
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"soldiers":   soldierCount,
		"tokens":     soldierCount,
		"token_name": "Soldier",
		"token_pt":   "1/1",
		"token_type": "colorless artifact creature — Soldier",
	})
}
