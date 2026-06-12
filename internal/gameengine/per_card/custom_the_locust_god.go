package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheLocustGodCustom wires The Locust God's {2}{U}{R} loot
// activated ability ("Draw a card, then discard a card"). The
// auto-generated handler in gen_the_locust_god.go already covers
// flying (AST), the card_drawn trigger that mints 1/1 U/R flying-
// hasted Insect tokens, and the ETB breadcrumb.
//
// Implementation:
//   - OnActivated tap-pays the {2}{U}{R} cost ({4} mana-pool charge,
//     mirroring drivnodIndestructibleActivate's "pull total from
//     ManaPool, no color enforcement at the per_card layer" pattern).
//   - Draw one card. Discarding a card requires hand to be non-empty;
//     if empty we still log the draw but skip the discard step.
//   - AI discard heuristic: pick the LOWEST-MV nonland card if
//     available, falling back to the lowest-MV card overall — Locust
//     loot is a card-quality filter, not a hand-trimming Wheel.
func registerTheLocustGodCustom(r *Registry) {
	r.OnActivated("The Locust God", theLocustGodLootActivate)
}

func theLocustGodLootActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_locust_god_loot_activate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.ManaPool < 4 {
		emitFail(gs, slug, src.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"mana_pool": seat.ManaPool,
		})
		return
	}
	seat.ManaPool -= 4
	gameengine.SyncManaAfterSpend(seat)

	drewName := ""
	if drawn := drawOne(gs, src.Controller, src.Card.DisplayName()); drawn != nil {
		drewName = drawn.DisplayName()
	}

	discardedName := ""
	if len(seat.Hand) > 0 {
		choice := theLocustGodPickDiscard(seat.Hand)
		if choice != nil {
			discardedName = choice.DisplayName()
			gameengine.DiscardCard(gs, choice, src.Controller)
		}
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"drew":      drewName,
		"discarded": discardedName,
	})
}

func theLocustGodPickDiscard(hand []*gameengine.Card) *gameengine.Card {
	var best *gameengine.Card
	bestCMC := 1 << 30
	bestIsLand := false
	for _, c := range hand {
		if c == nil {
			continue
		}
		isLand := cardHasType(c, "land")
		cmc := cardCMC(c)
		if best == nil {
			best, bestCMC, bestIsLand = c, cmc, isLand
			continue
		}
		// Prefer nonland over land.
		if bestIsLand && !isLand {
			best, bestCMC, bestIsLand = c, cmc, isLand
			continue
		}
		if !bestIsLand && isLand {
			continue
		}
		// Same land-ness — pick lower CMC.
		if cmc < bestCMC {
			best, bestCMC = c, cmc
		}
	}
	return best
}
