package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ramp_spells.go — sorcery-speed green ramp staples (R60 stub sweep
// batch 10).
//
// These cards may be handled by the AST tutor pipeline today via
// ResolveTutorGeneric, but explicit per_card OnResolve handlers pin
// the canonical resolution behavior with tests and override any
// future AST drift.
//
// Members covered:
//   - Rampant Growth     — search basic land → battlefield tapped
//   - Cultivate          — search 2 basics → 1 BF tapped + 1 to hand
//   - Kodama's Reach     — same as Cultivate
//   - Three Visits       — search Forest → battlefield (untapped)
//   - Nature's Lore      — same as Three Visits
//   - Farseek            — search non-Forest basic → battlefield tapped
//
// All members shuffle the library after the search per CR §701.18b.

// -----------------------------------------------------------------------------
// Filter predicates
// -----------------------------------------------------------------------------

// isForestCard reports whether the card has the Forest subtype (basic
// Forest, Snow-Covered Forest, dual lands like Taiga / Stomping Ground,
// utility lands with Forest type like Yavimaya Coast — no, those are
// hybrid types, only true Forest subtype matches Three Visits).
func isForestCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return cardHasSubtype(c, "forest")
}

// isNonForestBasic returns true for Plains / Island / Swamp / Mountain
// — Farseek's filter ("a Plains, Island, Swamp, or Mountain card").
// This includes basic Plains/etc AND non-basic lands with those
// subtypes (dual lands like Tundra, shocklands like Hallowed Fountain).
func isNonForestBasic(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, sub := range []string{"plains", "island", "swamp", "mountain"} {
		if cardHasSubtype(c, sub) {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Shared helper: tutor a land to a destination zone
// -----------------------------------------------------------------------------

// tutorLandToDest searches the controller's library for the first card
// matching filter, moves it to dest ("battlefield" or
// "battlefield_tapped" or "hand"), and shuffles. Returns the card name
// (empty if not found).
func tutorLandToDest(gs *gameengine.GameState, seat int, filter func(*gameengine.Card) bool, dest, source string) string {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return ""
	}
	s := gs.Seats[seat]
	if s == nil {
		return ""
	}
	foundIdx := -1
	for i, c := range s.Library {
		if c == nil {
			continue
		}
		if filter == nil || filter(c) {
			foundIdx = i
			break
		}
	}
	if foundIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		return ""
	}
	card := s.Library[foundIdx]
	gameengine.MoveCard(gs, card, seat, "library", dest, source)
	shuffleLibraryPerCard(gs, seat)
	return card.DisplayName()
}

// -----------------------------------------------------------------------------
// Rampant Growth
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for a basic land card, put that card onto the
//   battlefield tapped, then shuffle.

func registerRampantGrowth(r *Registry) {
	r.OnResolve("Rampant Growth", rampantGrowthResolve)
}

func rampantGrowthResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "rampant_growth"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	found := tutorLandToDest(gs, seat, isBasicLand, "battlefield_tapped", "Rampant Growth")
	emit(gs, slug, "Rampant Growth", map[string]interface{}{
		"seat":  seat,
		"found": found,
	})
}

// -----------------------------------------------------------------------------
// Cultivate / Kodama's Reach
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for up to two basic land cards, reveal those
//   cards, put one onto the battlefield tapped and the other into your
//   hand, then shuffle.

func registerCultivate(r *Registry) {
	r.OnResolve("Cultivate", cultivateResolve)
	r.OnResolve("Kodama's Reach", cultivateResolve)
}

func cultivateResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "cultivate_or_kodamas_reach"
	if gs == nil || item == nil || item.Card == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}
	// Pick up to two basic lands. First → battlefield_tapped, second → hand.
	var firstIdx, secondIdx int = -1, -1
	for i, c := range s.Library {
		if c == nil || !isBasicLand(c) {
			continue
		}
		if firstIdx < 0 {
			firstIdx = i
			continue
		}
		if secondIdx < 0 {
			secondIdx = i
			break
		}
	}
	toBF := ""
	toHand := ""
	if firstIdx >= 0 {
		card := s.Library[firstIdx]
		gameengine.MoveCard(gs, card, seat, "library", "battlefield_tapped", item.Card.DisplayName())
		toBF = card.DisplayName()
	}
	if secondIdx >= 0 {
		// secondIdx is the ORIGINAL index but the first MoveCard mutated
		// the library. Re-scan for the second basic to avoid an
		// off-by-one (the original second-found card is still present
		// because MoveCard removes-by-pointer, not by index, but its
		// index may have shifted). Just re-pick the first basic
		// remaining.
		for _, c := range s.Library {
			if c == nil || !isBasicLand(c) {
				continue
			}
			gameengine.MoveCard(gs, c, seat, "library", "hand", item.Card.DisplayName())
			toHand = c.DisplayName()
			break
		}
	}
	shuffleLibraryPerCard(gs, seat)
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"to_bf":     toBF,
		"to_hand":   toHand,
	})
}

// -----------------------------------------------------------------------------
// Three Visits / Nature's Lore
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for a Forest card and put that card onto the
//   battlefield, then shuffle.

func registerThreeVisits(r *Registry) {
	r.OnResolve("Three Visits", threeVisitsResolve)
	r.OnResolve("Nature's Lore", threeVisitsResolve)
}

func threeVisitsResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "three_visits_or_natures_lore"
	if gs == nil || item == nil || item.Card == nil {
		return
	}
	seat := item.Controller
	found := tutorLandToDest(gs, seat, isForestCard, "battlefield", item.Card.DisplayName())
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"found": found,
	})
}

// -----------------------------------------------------------------------------
// Farseek
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for a Plains, Island, Swamp, or Mountain card,
//   put it onto the battlefield tapped, then shuffle.

func registerFarseek(r *Registry) {
	r.OnResolve("Farseek", farseekResolve)
}

func farseekResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "farseek"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	found := tutorLandToDest(gs, seat, isNonForestBasic, "battlefield_tapped", "Farseek")
	emit(gs, slug, "Farseek", map[string]interface{}{
		"seat":  seat,
		"found": found,
	})
}
