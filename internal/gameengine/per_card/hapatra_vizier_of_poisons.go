package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Hapatra, Vizier of Poisons — {B}{G} 1/2 Legendary Creature — Human Cleric.
//
//   Whenever Hapatra deals combat damage to a player, you may put a
//   -1/-1 counter on target creature.
//   Whenever you put one or more -1/-1 counters on a creature, create a
//   1/1 green Snake creature token with deathtouch.
//
// Implementation covers the second (token-spawn) trigger — the primary
// payoff for the Hapatra archetype. Scoped to source_seat == Hapatra's
// controller per the "you put" wording (mirrors the Auntie Ool reference
// pattern in auntie_ool.go). Per-EVENT, not per-counter — one trigger
// fires per AST-driven counter application regardless of count, matching
// CR §603.1 single-trigger-per-event semantics.
//
// The combat-damage-to-player trigger is left to AST handling; it parses
// as a standard targeted -1/-1 placement and routes through the engine's
// trigger pipeline.

func init() {
	registerHapatraVizierOfPoisons(Global())
	AddResetHook(registerHapatraVizierOfPoisons)
}

func registerHapatraVizierOfPoisons(r *Registry) {
	r.OnTrigger("Hapatra, Vizier of Poisons", "counter_placed", hapatraCounterPlaced)
}

func hapatraCounterPlaced(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hapatra_minus_counter_snake"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	kind, _ := ctx["counter_kind"].(string)
	if kind != "-1/-1" {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || !target.IsCreature() {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	gameengine.CreateCreatureToken(gs, perm.Controller, "Snake Token",
		[]string{"creature", "snake", "pip:G", "deathtouch"}, 1, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"target_card": target.Card.DisplayName(),
	})
}
