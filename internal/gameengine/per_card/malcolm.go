package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMalcolm wires Malcolm, Keen-Eyed Navigator.
//
// Oracle text:
//
//	Whenever a Pirate you control deals damage to a player, create a
//	Treasure token.
//	Partner
//
// Triggers on combat_damage_player. The source must be a Pirate
// controlled by Malcolm's controller. (Non-combat damage is rare in
// pirate decks; we cover combat which is the dominant case via the
// engine's combat_damage_player event.)
func registerMalcolm(r *Registry) {
	r.OnTrigger("Malcolm, Keen-Eyed Navigator", "combat_damage_player", malcolmTrigger)
}

func malcolmTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "malcolm_keen_eyed_navigator_treasure"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if sourceName == "" {
		return
	}

	isPirate := false
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !strings.EqualFold(p.Card.DisplayName(), sourceName) {
			continue
		}
		if cardHasType(p.Card, "pirate") {
			isPirate = true
		}
		break
	}
	if !isPirate {
		return
	}

	token := &gameengine.Card{
		Name:  "Treasure Token",
		Owner: perm.Controller,
		Types: []string{"token", "artifact", "treasure"},
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"token":  "Treasure Token",
			"reason": "malcolm_pirate_damage",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"source_card": sourceName,
	})
}
