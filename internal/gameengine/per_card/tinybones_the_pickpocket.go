package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTinybonesThePickpocket wires Tinybones, the Pickpocket.
//
// Oracle text:
//
//	{B}
//	Legendary Creature — Skeleton Rogue
//	Deathtouch
//	Whenever Tinybones deals combat damage to a player, you may cast
//	  target nonland permanent card from that player's graveyard, and
//	  mana of any type can be spent to cast that spell.
//
// Implementation:
//   - Deathtouch via AST.
//   - "creature_combat_damage_to_player" trigger gated to attacker ==
//     Tinybones. Casting from opponent's graveyard with any-color mana
//     is an alt-cast path not surfaced cleanly to per_card; we
//     emitPartial and as a heuristic stand-in, exile the target
//     permanent card to perm.Flags so Heimdall can audit.
func registerTinybonesThePickpocket(r *Registry) {
	r.OnTrigger("Tinybones, the Pickpocket", "creature_combat_damage_to_player", tinybonesPickpocketDamage)
}

func tinybonesPickpocketDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tinybones_pickpocket_cast_from_opp_grave"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk != perm {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat < 0 || defenderSeat == perm.Controller || defenderSeat >= len(gs.Seats) {
		return
	}
	target := gs.Seats[defenderSeat]
	if target == nil {
		return
	}
	// Pick the highest-MV nonland permanent card in target's graveyard.
	var best *gameengine.Card
	bestMV := -1
	for _, c := range target.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "land") {
			continue
		}
		if !cardHasTypeAny(c, "creature", "artifact", "enchantment", "planeswalker", "battle") {
			continue
		}
		mv := cardCMC(c)
		if mv > bestMV {
			bestMV = mv
			best = c
		}
	}
	if best == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"defender_seat": defenderSeat,
			"reason":        "no_castable_target",
		})
		return
	}
	// R55: register a ZoneCastPolicy that lets Tinybones' controller
	// cast THAT SPECIFIC opp's graveyard card without paying mana,
	// with any-color mana eligibility for fixing. The policy is
	// scoped to one card via a closure predicate matching the picked
	// pointer; duration is until_end_of_turn (the cast offer expires
	// at EOT per CR §117.5b). Once cast (or the turn ends), the
	// policy is dropped by the captured perm's permanent_ltb hook or
	// by its own delayed trigger below.
	pickedCard := best
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:     perm,
		HandlerID:      "tinybones_pickpocket_cast_from_opp_grave",
		Zone:           gameengine.ZoneGraveyard,
		OwnerScope:     "opponents",
		CasterScope:    "controller",
		ControllerSeat: perm.Controller,
		Predicate: func(c *gameengine.Card) bool {
			return c == pickedCard
		},
		ManaCost:        0, // without paying its mana cost
		SpendAnyColor:   true,
		Duration:        "until_end_of_turn",
		SourceTimestamp: perm.Timestamp,
		GrantTurn:       gs.Turn,
	})
	captured := perm
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			gs.UnregisterZoneCastPoliciesForPermanent(captured)
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"target_spell":  best.DisplayName(),
		"target_mv":     bestMV,
	})
}
