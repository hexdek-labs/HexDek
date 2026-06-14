package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Brimaz, King of Oreskos — {1}{W}{W} 3/4 Legendary Creature — Cat Soldier.
//
//   Vigilance
//   Whenever Brimaz attacks, create a 1/1 white Cat Soldier creature
//   token with vigilance that's attacking.
//   Whenever Brimaz blocks a creature, create a 1/1 white Cat Soldier
//   creature token with vigilance that's blocking that creature.
//
// Implementation covers the attack trigger — the primary archetype payoff.
// The block trigger is emitPartial: there is currently no canonical
// "blocks" per_card event (no OnTrigger registrations for "blocks" or
// "creature_blocks" anywhere in per_card/*.go), so wiring it would require
// engine-side fan-out from combat.go. Vigilance is keyword-pipeline.

func init() {
	registerBrimazKingOfOreskos(Global())
	AddResetHook(registerBrimazKingOfOreskos)
}

func registerBrimazKingOfOreskos(r *Registry) {
	r.OnETB("Brimaz, King of Oreskos", brimazETB)
	r.OnTrigger("Brimaz, King of Oreskos", "creature_attacks", brimazAttack)
}

func brimazETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "brimaz_block_trigger_deferred"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"block_trigger_needs_engine_side_creature_blocks_event")
}

func brimazAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "brimaz_attack_cat_soldier_token"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	// §614 token doublers (Doubling Season, Parallel Lives, …) double this
	// creation event — route through CreateDoubledTokens so EACH resulting
	// cat (originals + doubled copies) gets the attacking setup below.
	defenderSeat, hasDef := ctx["defender_seat"].(int)
	tokens := gameengine.CreateDoubledTokens(gs, perm.Controller, 1, perm, func() *gameengine.Permanent {
		return gameengine.CreateCreatureToken(gs, perm.Controller, "Cat Soldier Token",
			[]string{"creature", "cat", "soldier", "pip:W", "vigilance"}, 1, 1)
	})
	for _, token := range tokens {
		// CR §508.1g / §509.1 — the token enters tapped and attacking the same
		// defender Brimaz is attacking. MarkEnteredAttacking stamps both
		// flagAttacking and the §508.1g carve-out tag.
		gameengine.MarkEnteredAttacking(token)
		token.SummoningSick = false
		if hasDef {
			token.Flags["attacking_defender_seat"] = defenderSeat + 1
		}
	}
	if len(tokens) == 0 {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "Cat Soldier",
		"count": len(tokens),
	})
}
