package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGlissaSunslayer wires Glissa Sunslayer.
//
// Oracle text:
//
//	First strike, deathtouch
//	Whenever Glissa Sunslayer deals combat damage to a player, choose
//	one —
//	  • You draw a card and lose 1 life.
//	  • Destroy target enchantment.
//	  • Remove up to three counters from target permanent.
//
// AI policy: prefer the draw-card mode (most reliably useful).
// Counter-removal and enchantment-destruction modes need targeting that
// the engine doesn't expose for AI choice — emitPartial those branches.
func registerGlissaSunslayer(r *Registry) {
	r.OnTrigger("Glissa Sunslayer", "combat_damage_player", glissaSunslayerCombatDamage)
}

func glissaSunslayerCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "glissa_sunslayer_draw_lose1"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceName, _ := ctx["source_card"].(string)
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	gameengine.LoseLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"destroy_enchantment_and_remove_counters_modes_unimplemented")
}
