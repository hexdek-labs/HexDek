package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPageLooseLeaf wires Page, Loose Leaf.
//
// Oracle text:
//
//	{T}: Add {C}.
//	Grandeur — Discard another card named Page, Loose Leaf: Reveal
//	cards from the top of your library until you reveal an instant or
//	sorcery card. Put that card into your hand and the rest on the
//	bottom of your library in a random order.
//
// Implementation (R53 batch N port):
//   - {T}: Add {C} — generic land/mana parser handles it; not wired
//     here (would conflict with the generic mana dispatcher).
//   - Grandeur OnActivated (abilityIdx 1): pay discard-another-Page
//     cost (require a Page in hand other than the activating one;
//     move it to graveyard via DiscardCard), then walk the
//     controller's library top-down: take the first instant/sorcery
//     card found and put it in hand; dump preceding non-matches to
//     the bottom of the library in order (engine doesn't yet random-
//     shuffle the bottom subset, partial documents the gap).
func registerPageLooseLeaf(r *Registry) {
	r.OnETB("Page, Loose Leaf", pageLooseLeafETB)
	r.OnActivated("Page, Loose Leaf", pageLooseLeafGrandeur)
}

func pageLooseLeafETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "page_loose_leaf_etb"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func pageLooseLeafGrandeur(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "page_loose_leaf_grandeur"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if abilityIdx != 1 {
		// Ability 0 ({T}: Add {C}) is handled by the generic mana dispatcher.
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}
	// Cost: discard another Page from hand.
	var discardVictim *gameengine.Card
	for _, c := range seat.Hand {
		if c == nil {
			continue
		}
		if normalizeName(c.DisplayName()) == normalizeName("Page, Loose Leaf") {
			discardVictim = c
			break
		}
	}
	if discardVictim == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_other_page_in_hand", nil)
		return
	}
	gameengine.DiscardCard(gs, discardVictim, src.Controller)

	// Effect: walk library top-down, take first inst/sorc, move rest
	// up to that point to the bottom.
	var found *gameengine.Card
	bottomPile := []*gameengine.Card{}
	newLibrary := []*gameengine.Card{}
	for _, c := range seat.Library {
		if c == nil {
			continue
		}
		if found == nil && (cardHasType(c, "instant") || cardHasType(c, "sorcery")) {
			found = c
			continue // skip — goes to hand
		}
		if found == nil {
			// Pre-find non-matches go to bottom.
			bottomPile = append(bottomPile, c)
			continue
		}
		// After the found card, the rest stays on top.
		newLibrary = append(newLibrary, c)
	}
	// R60 batch 9: shuffle the bottom pile per oracle ("put the rest
	// on the bottom of your library in a random order"). Closes the
	// "bottom_pile_random_order_not_shuffled_per_oracle" partial.
	if gs.Rng != nil && len(bottomPile) > 1 {
		gs.Rng.Shuffle(len(bottomPile), func(i, j int) {
			bottomPile[i], bottomPile[j] = bottomPile[j], bottomPile[i]
		})
	}
	// Wave 2 multi-step migration: rebuild the library WITH `found`
	// still at the top, then route the library→hand transition through
	// MoveCard so §614 / §903.9b / card_drawn-equivalent zone_change
	// observers all fire. Pre-r60 pulled `found` out during the rebuild
	// and manually appended to hand, bypassing every one of the above.
	rebuilt := newLibrary
	if found != nil {
		rebuilt = append([]*gameengine.Card{found}, rebuilt...)
	}
	seat.Library = append(rebuilt, bottomPile...)
	if found != nil {
		gameengine.MoveCard(gs, found, src.Controller, "library", "hand", "page_loose_leaf_tutor")
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"discarded": discardVictim.DisplayName(),
		"tutored":   foundOrEmpty(found),
		"to_bottom": len(bottomPile),
	})
}

func foundOrEmpty(c *gameengine.Card) string {
	if c == nil {
		return ""
	}
	return c.DisplayName()
}
