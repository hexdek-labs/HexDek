package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Wrexial, the Risen Deep — {3}{U}{B} 5/8 Legendary Creature — Kraken.
//
//   Islandwalk, swampwalk
//   Whenever Wrexial deals combat damage to a player, you may cast
//   target instant or sorcery card from that player's graveyard without
//   paying its mana cost. If that spell would be put into a graveyard,
//   exile it instead.
//
// Implementation: OnTrigger combat_damage_player with source_seat ==
// Wrexial.Controller AND source_card == "Wrexial, the Risen Deep"
// (combat.go fires with source as string, not perm). Pick the
// highest-CMC instant/sorcery in the defender's graveyard; register a
// ZoneCastGrant scoped to Wrexial's controller with ManaCost=0,
// ExileOnResolve=true, Duration="until_end_of_turn" so the
// ZoneCastGrantExpiry plumbing reclaims it at EOT (r60 lifecycle pin).
//
// Islandwalk/swampwalk are keyword-pipeline.

func init() {
	registerWrexialTheRisenDeep(Global())
	AddResetHook(registerWrexialTheRisenDeep)
}

func registerWrexialTheRisenDeep(r *Registry) {
	r.OnTrigger("Wrexial, the Risen Deep", "combat_damage_player", wrexialCombatDamage)
}

func wrexialCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wrexial_free_cast_from_opp_graveyard"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceCard, _ := ctx["source_card"].(string)
	if sourceCard != perm.Card.DisplayName() {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) || defenderSeat == perm.Controller {
		return
	}
	defender := gs.Seats[defenderSeat]
	if defender == nil {
		return
	}
	pick := pickHighestCMCInstantOrSorceryFromGraveyard(defender.Graveyard)
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"defender_seat": defenderSeat,
			"reason":        "no_instant_or_sorcery_in_opp_graveyard",
		})
		return
	}
	grant := &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneGraveyard,
		Keyword:           "wrexial_free_cast",
		ManaCost:          0,
		ExileOnResolve:    true,
		RequireController: perm.Controller,
		SourceName:        perm.Card.DisplayName(),
		Duration:          "until_end_of_turn",
		GrantTurn:         gs.Turn,
		SourceTimestamp:   perm.Timestamp,
	}
	gameengine.RegisterZoneCastGrant(gs, pick, grant)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"grant_card":    pick.DisplayName(),
	})
}

func pickHighestCMCInstantOrSorceryFromGraveyard(gy []*gameengine.Card) *gameengine.Card {
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range gy {
		if c == nil {
			continue
		}
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	return best
}
