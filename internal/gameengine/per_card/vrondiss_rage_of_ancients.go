package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVrondissRageOfAncients wires Vrondiss, Rage of Ancients.
//
// Oracle text:
//
//	Enrage — Whenever Vrondiss is dealt damage, you may create a 5/4 red
//	and green Dragon Spirit creature token with "When this token deals
//	damage, sacrifice it."
//	Whenever you roll one or more dice, you may have Vrondiss deal 1
//	damage to itself.
//
// Implementation:
//   - "damage_taken" trigger gated on perm == damaged: mint a 5/4 Dragon
//     Spirit. The token's "sacrifice when it deals damage" is not modeled.
//   - "roll_dice" trigger: deal 1 damage to Vrondiss (which then loops
//     the enrage trigger). AI policy: always opt yes — generates tokens.
func registerVrondissRageOfAncients(r *Registry) {
	r.OnTrigger("Vrondiss, Rage of Ancients", "damage_taken", vrondissEnrage)
	r.OnTrigger("Vrondiss, Rage of Ancients", "creature_dealt_damage", vrondissEnrage)
	r.OnTrigger("Vrondiss, Rage of Ancients", "roll_dice", vrondissRollDice)
}

func vrondissEnrage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vrondiss_enrage_dragon_spirit"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	tgt, _ := ctx["target_perm"].(*gameengine.Permanent)
	if tgt != nil && tgt != perm {
		return
	}
	token := &gameengine.Card{
		Name:          "Dragon Spirit Token",
		Owner:         perm.Controller,
		BasePower:     5,
		BaseToughness: 4,
		Types:         []string{"token", "creature", "dragon", "spirit"},
		Colors:        []string{"R", "G"},
		TypeLine:      "Token Creature — Dragon Spirit",
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "token_self_sac_on_damage_not_implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func vrondissRollDice(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vrondiss_self_damage_on_roll"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Engine fires roll_dice with "seat" (resolve_helpers.go:1216 + :3086).
	// Legacy "controller_seat" kept as a fallback.
	rollerSeat, ok := ctx["seat"].(int)
	if !ok {
		rollerSeat, _ = ctx["controller_seat"].(int)
	}
	if rollerSeat != perm.Controller {
		return
	}
	perm.MarkedDamage += 1
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Target: perm.Controller,
		Amount: 1,
		Details: map[string]interface{}{
			"slug":   slug,
			"reason": "vrondiss_self_damage_on_roll",
		},
	})
	gameengine.FireCardTrigger(gs, "damage_taken", map[string]interface{}{
		"target_perm": perm,
		"amount":      1,
		"source":      perm,
	})
	gs.LogEvent(gameengine.Event{
		Kind: "state_check",
		Seat: perm.Controller,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
