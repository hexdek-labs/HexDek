package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerViviOrnitier wires Vivi Ornitier.
//
// Oracle text:
//
//	Whenever Vivi Ornitier deals combat damage to a player, you may cast
//	an instant or sorcery spell from your graveyard without paying its
//	mana cost. If a spell cast this way would be put into your graveyard,
//	exile it instead.
//
// Implementation:
//   - OnTrigger "combat_damage_player": gates on source == Vivi and
//     source_seat == controller (rules out copies / opponent-controlled
//     Vivi via control magic still requires the source filter).
//   - We grant a one-shot ZoneCastPermission for graveyard with
//     ManaCost=0 + ExileOnResolve=true on every instant/sorcery currently
//     in the controller's graveyard. The AI/Hat consults ZoneCastGrants
//     when scoring castable plays and may choose the highest-impact spell.
//   - A delayed end-of-turn trigger revokes any unused grants so the
//     opportunity stays one-shot per oracle text. Cast spells exit the
//     graveyard naturally (CastFromZone removes them, ExileOnResolve
//     handles post-resolution routing).
//   - De-dupe per (turn, source_seat) — "Whenever ~ deals combat damage
//     to A player" fires once per damage event per defender, but the
//     graveyard-grant scope is per-controller and we want one grant
//     window per turn so multi-defender first-strike + normal damage
//     does not double-grant.
func registerViviOrnitier(r *Registry) {
	r.OnTrigger("Vivi Ornitier", "combat_damage_player", viviOrnitierCombatDamage)
}

func viviOrnitierCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vivi_ornitier_free_cast_from_graveyard"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Collect candidate instants/sorceries in the controller's graveyard.
	type candidate struct {
		card *gameengine.Card
		cmc  int
	}
	var candidates []candidate
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "instant") && !cardHasType(c, "sorcery") {
			continue
		}
		candidates = append(candidates, candidate{card: c, cmc: cardCMC(c)})
	}

	if len(candidates) == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"defender_seat": ctx["defender_seat"],
			"candidates":    0,
		})
		return
	}

	// Best heuristic: highest CMC (free cast value).
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.cmc > best.cmc {
			best = c
		}
	}

	// Grant one-shot free-cast permission to every candidate so the AI
	// has the same flexibility a human player would. Track granted cards
	// in a delayed end-of-turn trigger for cleanup of unused grants.
	grantedNames := make([]string, 0, len(candidates))
	grantedCards := make([]*gameengine.Card, 0, len(candidates))
	for _, c := range candidates {
		if gameengine.GetZoneCastGrant(gs, c.card) != nil {
			// Don't override an existing grant (e.g. Underworld Breach escape).
			continue
		}
		zonePerm := &gameengine.ZoneCastPermission{
			Zone:              gameengine.ZoneGraveyard,
			Keyword:           "vivi_free_cast",
			ManaCost:          0,
			ExileOnResolve:    true,
			RequireController: perm.Controller,
			SourceName:        perm.Card.DisplayName(),
		}
		gameengine.RegisterZoneCastGrant(gs, c.card, zonePerm)
		grantedNames = append(grantedNames, c.card.DisplayName())
		grantedCards = append(grantedCards, c.card)
	}

	if len(grantedCards) > 0 {
		controller := perm.Controller
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "end_of_turn",
			ControllerSeat: controller,
			SourceCardName: perm.Card.DisplayName(),
			EffectFn: func(gs *gameengine.GameState) {
				for _, c := range grantedCards {
					if g := gameengine.GetZoneCastGrant(gs, c); g != nil &&
						g.Keyword == "vivi_free_cast" {
						gameengine.RemoveZoneCastGrant(gs, c)
					}
				}
			},
		})
	}

	dedupeKey := fmt.Sprintf("vivi_grant_t%d", gs.Turn+1)
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags[dedupeKey]++

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": ctx["defender_seat"],
		"candidates":    len(candidates),
		"granted":       grantedNames,
		"best":          best.card.DisplayName(),
		"best_cmc":      best.cmc,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"free_cast_relies_on_ai_consuming_zone_cast_grant_within_priority_window")
}
