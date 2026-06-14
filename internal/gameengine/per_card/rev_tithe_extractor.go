package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// rev_tithe_extractor.go — Rev, Tithe Extractor (Bloomburrow legends pool).
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Rev,%20Tithe%20Extractor):
//
//	Whenever you attack, target creature gains deathtouch until end of
//	turn.
//	Whenever one or more creatures you control deal combat damage to a
//	player, create a Treasure token, then look at the top card of that
//	player's library and exile it face down. You may cast that card for
//	as long as it remains exiled.
//
// SCOPE: this file wires ONLY the second ability — the §603.3c BATCHED
// "one or more creatures you control deal combat damage to a player"
// trigger (the combat-damage-batch firing-cardinality bucket). The first
// ability ("Whenever you attack ... deathtouch") is the separate
// you-attack bucket and is intentionally left to that dispatch path.
//
// Firing cardinality (CR §510.4): all combat damage is simultaneous, so
// this trigger fires EXACTLY ONCE per damaged player regardless of how
// many of your creatures connect. The engine fires combat_damage_player
// once per (attacker, defending player) pair (combat.go
// fireCombatDamageTriggers, called per damaging creature), so a naive
// handler would mint one Treasure + one impulse-exile PER attacker —
// three creatures into one player would wrongly yield three Treasures and
// three exiles. We dedupe per (defender, turn) so the batch resolves once
// per damaged player, mirroring Gonti, Canny Acquisitor's identical
// "one or more creatures ... deal combat damage" wording.

func init() {
	registerRevTitheExtractor(Global())
	AddResetHook(registerRevTitheExtractor)
}

func registerRevTitheExtractor(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Rev, Tithe Extractor", "combat_damage_player", revTitheExtractorCombatDamage)
}

func revTitheExtractorCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rev_tithe_extractor_treasure_impulse"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	defenderSeat, ok := ctx["defender_seat"].(int)
	if !ok {
		defenderSeat, _ = ctx["target_seat"].(int)
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	defender := gs.Seats[defenderSeat]
	if defender == nil || defender.Lost {
		return
	}

	// De-dupe per (defender, turn): "one or more creatures you control
	// deal combat damage to a player" fires once per damaged player per
	// combat damage step, not once per attacker (CR §510.4).
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupe := fmt.Sprintf("rev_dmg_d%d_t%d", defenderSeat, gs.Turn+1)
	if perm.Flags[dedupe] == 1 {
		return
	}
	perm.Flags[dedupe] = 1

	// "create a Treasure token" — one per damaged player per combat.
	gameengine.CreateTreasureToken(gs, perm.Controller)

	// "then look at the top card of that player's library and exile it
	// face down. You may cast that card for as long as it remains exiled."
	if len(defender.Library) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"defender_seat": defenderSeat,
			"treasure_made": 1,
			"exiled":        false,
			"reason":        "library_empty",
		})
		return
	}
	top := defender.Library[0]
	if top == nil {
		return
	}
	gameengine.MoveCard(gs, top, defenderSeat, "library", "exile", "rev_tithe_extractor_exile")
	top.FaceDown = true
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	// "for as long as it remains exiled" — permanent grant (no source
	// dependency, normal mana cost; unlike Gonti there is no "any type
	// of mana" clause).
	gs.ZoneCastGrants[top] = &gameengine.ZoneCastPermission{
		Zone:              "exile",
		Keyword:           "rev_tithe_extractor_impulse",
		ManaCost:          -1,
		RequireController: perm.Controller,
		SourceName:        perm.Card.DisplayName(),
		Duration:          "",
		SourceTimestamp:   perm.Timestamp,
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "exile_from_library",
		Seat:   defenderSeat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"card":      top.DisplayName(),
			"reason":    "rev_tithe_extractor_combat_damage",
			"face_down": true,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"treasure_made": 1,
		"exiled_card":   top.DisplayName(),
	})
}
