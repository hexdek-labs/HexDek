package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRuneScarredDemon wires Rune-Scarred Demon (Muninn parser-gap,
// recurring reanimator / Birthing-Ritual chain target).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{5}{B}{B}
//	Creature — Demon
//	Flying
//	When this creature enters, search your library for a card, put it
//	into your hand, then shuffle.
//
// Implementation:
//   - Flying via AST keyword pipeline.
//   - OnETB: unrestricted tutor to hand. Hat picks the highest mana-value
//     card in the library — Rune-Scarred is almost always cast for the
//     biggest finisher in the deck (Craterhoof, Torment of Hailfire, a
//     second copy of a combo piece). Falls back to highest-CMC creature
//     if no high-CMC nonland is available, else any nonland.
//   - Shuffles the library after the search.
func registerRuneScarredDemon(r *Registry) {
	r.OnETB("Rune-Scarred Demon", runeScarredDemonETB)
	// FULLY implements the printed self-ETB tutor — declare ownership so the
	// engine's generic AST push is suppressed (Judge r63 double-fire gate).
	// Without it the AST "search your library for a card …" trigger ALSO
	// resolves, tutoring two cards (PROGRESSION r63).
	r.OwnsETBTrigger("Rune-Scarred Demon")
}

func runeScarredDemonETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rune_scarred_demon_tutor"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	foundIdx := -1
	bestCMC := -1
	for i, c := range s.Library {
		if c == nil {
			continue
		}
		if cardHasType(c, "land") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > bestCMC {
			bestCMC = cmc
			foundIdx = i
		}
	}
	if foundIdx < 0 {
		shuffleLibraryPerCard(gs, seat)
		emitFail(gs, slug, perm.Card.DisplayName(), "empty_or_lands_only_library", map[string]interface{}{
			"seat": seat,
		})
		return
	}
	tutored := s.Library[foundIdx]
	gameengine.MoveCard(gs, tutored, seat, "library", "hand", "rune_scarred_demon_tutor")
	shuffleLibraryPerCard(gs, seat)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"tutored":     tutored.DisplayName(),
		"tutored_cmc": bestCMC,
	})
}
