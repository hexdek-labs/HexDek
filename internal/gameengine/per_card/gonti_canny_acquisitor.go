package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGontiCannyAcquisitor wires Gonti, Canny Acquisitor (Aetherdrift /
// Foundations Jumpstart legends pool — Sultai Aetherborn Rogue, 5/5 for
// {2}{B}{G}{U}).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Spells you cast but don't own cost {1} less to cast.
//	Whenever one or more creatures you control deal combat damage to a
//	player, look at the top card of that player's library, then exile
//	it face down. You may play that card for as long as it remains
//	exiled, and mana of any type can be spent to cast that spell.
//
// Engine wiring:
//   - The cost reduction is implemented in cost_modifiers.go (case
//     "Gonti, Canny Acquisitor"); see that file. No runtime hook here.
//   - OnTrigger("combat_damage_player") fires per damage event; we
//     filter to creatures controlled by Gonti's controller and de-dupe
//     per (defender_seat, turn) so the trigger fires once per damaged
//     player per combat (matching the "one or more creatures" oracle
//     text).
//   - On a fresh hit, exile the top card of the defender's library face
//     down and grant Gonti's controller a ZoneCastPermission to play it
//     from exile.
//
// "Mana of any type" is recorded on the permission's Keyword for
// downstream consumers but is not separately enforced here — the engine
// treats ZoneCastPermission casts as cost-flexible enough for AI play.
func registerGontiCannyAcquisitor(r *Registry) {
	r.OnTrigger("Gonti, Canny Acquisitor", "combat_damage_player", gontiCombatDamage)
}

func gontiCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gonti_canny_acquisitor_combat_exile"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	defenderSeat, ok := ctx["defender_seat"].(int)
	if !ok || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	defender := gs.Seats[defenderSeat]
	if defender == nil || defender.Lost {
		return
	}

	// De-dupe per (defender, turn) — "one or more creatures... deal
	// combat damage to a player" is one trigger per damaged player per
	// combat damage step.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupe := fmt.Sprintf("gonti_dmg_d%d_t%d", defenderSeat, gs.Turn+1)
	if perm.Flags[dedupe] == 1 {
		return
	}
	perm.Flags[dedupe] = 1

	if len(defender.Library) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "library_empty", map[string]interface{}{
			"seat":          perm.Controller,
			"defender_seat": defenderSeat,
		})
		return
	}

	top := defender.Library[0]
	if top == nil {
		return
	}
	gameengine.MoveCard(gs, top, defenderSeat, "library", "exile", "gonti_canny_acquisitor_exile")
	top.FaceDown = true
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	gs.ZoneCastGrants[top] = &gameengine.ZoneCastPermission{
		Zone:              "exile",
		Keyword:           "gonti_canny_acquisitor_any_mana",
		ManaCost:          -1,
		RequireController: perm.Controller,
		SourceName:        perm.Card.DisplayName(),
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "exile_from_library",
		Seat:   defenderSeat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"card":      top.DisplayName(),
			"reason":    "gonti_canny_acquisitor_combat_damage",
			"face_down": true,
			"any_mana":  true,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"exiled_card":   top.DisplayName(),
	})
}
