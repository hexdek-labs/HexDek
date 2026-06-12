package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerViconiaDrowApostate wires Viconia, Drow Apostate (Muninn parser-gap
// #81, ~7.1K hits).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	At the beginning of your upkeep, if there are four or more creature
//	cards in your graveyard, return a creature card at random from your
//	graveyard to your hand.
//	Choose a Background (You can have a Background as a second commander.)
//
// Implementation:
//   - "upkeep_controller" listener gated on active_seat == controller.
//   - Count creature cards in our graveyard. If <4, do nothing.
//   - Otherwise pick one uniformly at random and return to hand.
//   - "Choose a Background" is a deckbuilding clause (CR §702.158) — no
//     in-game effect, no need for an emitPartial.
func registerViconiaDrowApostate(r *Registry) {
	r.OnTrigger("Viconia, Drow Apostate", "upkeep_controller", viconiaDrowUpkeep)
}

func viconiaDrowUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "viconia_drow_upkeep_random_creature"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	creatures := []*gameengine.Card{}
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "creature") {
			creatures = append(creatures, c)
		}
	}
	if len(creatures) < 4 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
			"count":     len(creatures),
			"reason":    "fewer_than_four_creatures",
		})
		return
	}
	pick := creatures[rngIntn(gs, len(creatures))]
	gameengine.MoveCard(gs, pick, perm.Controller, "graveyard", "hand", slug)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"triggered": true,
		"count":     len(creatures),
		"returned":  pick.DisplayName(),
	})
}
