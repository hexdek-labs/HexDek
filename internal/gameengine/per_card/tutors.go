package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Tutor family — shared helpers for library-search spells
// -----------------------------------------------------------------------------
//
// All tutors share the pattern: search library for a card matching some
// filter, put it into a destination zone, then shuffle (or shuffle then
// place on top for "put on top" tutors).
//
// tutorToHand: Demonic Tutor — any card → hand → shuffle.
// tutorToTop:  Vampiric/Mystical/Enlightened/Worldly — filtered → top → shuffle first then top.
//
// Card filter is a func(*Card) bool predicate.

// tutorToHand searches the controller's library for the first card matching
// filter, puts it into their hand, and shuffles. If filter is nil, finds
// any card (Demonic Tutor).
func tutorToHand(gs *gameengine.GameState, seat int, filter func(*gameengine.Card) bool, source string) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
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
		// Fail to find — still shuffle.
		shuffleLibraryPerCard(gs, seat)
		return false
	}
	card := s.Library[foundIdx]
	gameengine.MoveCard(gs, card, seat, "library", "hand", "tutor-to-hand")
	shuffleLibraryPerCard(gs, seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: source,
		Details: map[string]interface{}{
			"found_card":  card.DisplayName(),
			"destination": "hand",
			"reason":      "tutor",
		},
	})
	return true
}

// tutorToTop searches the controller's library for the first card matching
// filter, shuffles the library, then places the found card on top.
// This is the correct order for Vampiric Tutor et al: "Search your library
// for a card, then shuffle your library. Put that card on top of your library."
func tutorToTop(gs *gameengine.GameState, seat int, filter func(*gameengine.Card) bool, source string) bool {
	if gs == nil || seat < 0 || seat >= len(gs.Seats) {
		return false
	}
	s := gs.Seats[seat]
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
		return false
	}
	card := s.Library[foundIdx]
	s.Library = append(s.Library[:foundIdx], s.Library[foundIdx+1:]...)
	// Shuffle FIRST, then place on top.
	shuffleLibraryPerCard(gs, seat)
	s.Library = append([]*gameengine.Card{card}, s.Library...)
	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: source,
		Details: map[string]interface{}{
			"found_card":  card.DisplayName(),
			"destination": "library_top",
			"reason":      "tutor",
		},
	})
	return true
}

// -----------------------------------------------------------------------------
// Filter predicates
// -----------------------------------------------------------------------------

func isInstantOrSorcery(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		tl := strings.ToLower(t)
		if tl == "instant" || tl == "sorcery" {
			return true
		}
	}
	return false
}

func isArtifactOrEnchantment(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		tl := strings.ToLower(t)
		if tl == "artifact" || tl == "enchantment" {
			return true
		}
	}
	return false
}

func isCreatureCard(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.ToLower(t) == "creature" {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Demonic Tutor
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for a card, put that card into your hand, then
//   shuffle your library.
//
// 1B sorcery. The best unrestricted tutor in the game.

func registerDemonicTutor(r *Registry) {
	r.OnResolve("Demonic Tutor", demonicTutorResolve)
}

func demonicTutorResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "demonic_tutor"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	found := tutorToHand(gs, seat, nil, "Demonic Tutor")
	emit(gs, slug, "Demonic Tutor", map[string]interface{}{
		"seat":  seat,
		"found": found,
	})
}

// -----------------------------------------------------------------------------
// Vampiric Tutor
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for a card, then shuffle your library and put
//   that card on top of it. You lose 2 life.
//
// B instant. Finds any card, puts on top, pay 2 life.

func registerVampiricTutor(r *Registry) {
	r.OnResolve("Vampiric Tutor", vampiricTutorResolve)
}

func vampiricTutorResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "vampiric_tutor"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Pay 2 life.
	gs.Seats[seat].Life -= 2
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: "Vampiric Tutor",
		Amount: 2,
		Details: map[string]interface{}{
			"reason": "vampiric_tutor_cost",
		},
	})

	found := tutorToTop(gs, seat, nil, "Vampiric Tutor")
	emit(gs, slug, "Vampiric Tutor", map[string]interface{}{
		"seat":  seat,
		"found": found,
	})
	_ = gs.CheckEnd()
}

// -----------------------------------------------------------------------------
// Mystical Tutor
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for an instant or sorcery card, reveal it,
//   then shuffle your library and put that card on top of it.
//
// U instant.

func registerMysticalTutor(r *Registry) {
	r.OnResolve("Mystical Tutor", mysticalTutorResolve)
}

func mysticalTutorResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "mystical_tutor"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	found := tutorToTop(gs, seat, isInstantOrSorcery, "Mystical Tutor")
	emit(gs, slug, "Mystical Tutor", map[string]interface{}{
		"seat":   seat,
		"found":  found,
		"filter": "instant_or_sorcery",
	})
}

// -----------------------------------------------------------------------------
// Enlightened Tutor
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for an artifact or enchantment card, reveal it,
//   then shuffle your library and put that card on top of it.
//
// W instant.

func registerEnlightenedTutor(r *Registry) {
	r.OnResolve("Enlightened Tutor", enlightenedTutorResolve)
}

func enlightenedTutorResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "enlightened_tutor"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	found := tutorToTop(gs, seat, isArtifactOrEnchantment, "Enlightened Tutor")
	emit(gs, slug, "Enlightened Tutor", map[string]interface{}{
		"seat":   seat,
		"found":  found,
		"filter": "artifact_or_enchantment",
	})
}

// -----------------------------------------------------------------------------
// Worldly Tutor
// -----------------------------------------------------------------------------
//
// Oracle text:
//   Search your library for a creature card, reveal it, then shuffle
//   your library and put that card on top of it.
//
// G instant.

func registerWorldlyTutor(r *Registry) {
	r.OnResolve("Worldly Tutor", worldlyTutorResolve)
}

func worldlyTutorResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "worldly_tutor"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	found := tutorToTop(gs, seat, isCreatureCard, "Worldly Tutor")
	emit(gs, slug, "Worldly Tutor", map[string]interface{}{
		"seat":   seat,
		"found":  found,
		"filter": "creature",
	})
}
