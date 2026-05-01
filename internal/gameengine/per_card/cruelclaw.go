package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerInfamousCruelclaw wires The Infamous Cruelclaw (Bloomburrow).
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary Creature — Weasel Mercenary. {1}{B}{R}. 3/3. Menace.
//	Whenever The Infamous Cruelclaw deals combat damage to a player,
//	exile cards from the top of your library until you exile a nonland
//	card. You may cast that card by discarding a card rather than paying
//	its mana cost.
//
// Implementation:
//   - Menace is wired through the AST keyword pipeline.
//   - "combat_damage_player" trigger: when Cruelclaw is the source and the
//     defender is a player, exile cards from the controller's library
//     one-by-one until a nonland card is exiled (or the library empties).
//     All exiled lands are recorded in the slug emit for replay clarity.
//   - "may cast that card by discarding a card rather than paying its
//     mana cost": modeled as a ZoneCastPermission with ManaCost=0 plus a
//     custom AdditionalCost whose PayFn forces a discard. The permission
//     is controller-restricted so only Cruelclaw's controller may use it,
//     and the discard predicate (CanPayFn) requires at least one card
//     left in hand AFTER setting aside the to-be-cast card. This is the
//     same dispatch pattern Prosper, Tome-Bound uses for its impulse
//     exile (see prosper_tome_bound.go), with a free mana cost and an
//     extra discard hop instead of normal cost.
func registerInfamousCruelclaw(r *Registry) {
	r.OnTrigger("The Infamous Cruelclaw", "combat_damage_player", cruelclawCombatDamage)
}

func cruelclawCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "cruelclaw_combat_damage_exile_cast"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if perm.Card == nil || sourceName != perm.Card.DisplayName() {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}

	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	var exiledLands []string
	var nonland *gameengine.Card
	for len(s.Library) > 0 {
		card := s.Library[0]
		gameengine.MoveCard(gs, card, seat, "library", "exile", "cruelclaw_exile_until_nonland")
		gs.LogEvent(gameengine.Event{
			Kind:   "exile_from_library",
			Seat:   seat,
			Source: perm.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"card":   card.DisplayName(),
				"reason": "cruelclaw_until_nonland",
			},
		})
		if cardHasType(card, "land") {
			exiledLands = append(exiledLands, card.DisplayName())
			continue
		}
		nonland = card
		break
	}

	if nonland == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "library_empty_no_nonland", map[string]interface{}{
			"seat":          seat,
			"defender_seat": defenderSeat,
			"exiled_lands":  exiledLands,
		})
		return
	}

	// Grant a controller-restricted, free-mana, discard-an-other-card
	// alternative cost permission. Caller exercises through the engine's
	// CastFromZone path; CanPayAdditionalCost / PayAdditionalCost route
	// through the closures below.
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	cardToCast := nonland
	gs.ZoneCastGrants[cardToCast] = &gameengine.ZoneCastPermission{
		Zone:              gameengine.ZoneExile,
		Keyword:           "cruelclaw_discard_alt_cost",
		ManaCost:          0,
		RequireController: seat,
		SourceName:        perm.Card.DisplayName(),
		AdditionalCosts: []*gameengine.AdditionalCost{
			{
				Kind:  "discard",
				Label: "discard a card",
				CanPayFn: func(g *gameengine.GameState, idx int) bool {
					if g == nil || idx < 0 || idx >= len(g.Seats) {
						return false
					}
					ss := g.Seats[idx]
					if ss == nil {
						return false
					}
					// Need ≥1 card in hand other than the to-be-cast card.
					// (The card is exiled, not in hand, so any hand card works.)
					return len(ss.Hand) >= 1
				},
				PayFn: func(g *gameengine.GameState, idx int) bool {
					if g == nil || idx < 0 || idx >= len(g.Seats) {
						return false
					}
					ss := g.Seats[idx]
					if ss == nil || len(ss.Hand) == 0 {
						return false
					}
					// Discard the lowest-CMC card (heuristic: cheapest card
					// is usually the most expendable for a free spell).
					worst := 0
					worstCMC := 1 << 30
					for i, c := range ss.Hand {
						if c == nil {
							continue
						}
						cmc := cardCMC(c)
						if cmc < worstCMC {
							worst = i
							worstCMC = cmc
						}
					}
					discarded := ss.Hand[worst]
					gameengine.DiscardCard(g, discarded, idx)
					return true
				},
			},
		},
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"defender_seat": defenderSeat,
		"exiled_lands":  exiledLands,
		"exiled_card":   cardToCast.DisplayName(),
	})
}
