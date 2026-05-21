package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlisaieLeveilleur wires Alisaie Leveilleur.
//
// Oracle text (Scryfall, verified):
//
//	Partner with Alphinaud Leveilleur
//	First strike
//	Dualcast — The second spell you cast each turn costs {2} less
//	to cast.
//
// Implementation (R49 stub port — batch A):
//   - First strike: AST keyword pipeline.
//   - Partner with Alphinaud Leveilleur: ETB tutor (CR §702.124a). Scan
//     the controller's library for Alphinaud, MoveCard to hand, shuffle.
//     If Alphinaud isn't in the library (already drawn, exiled, or
//     deck-built without him), the shuffle still happens per CR §701.19c.
//   - Dualcast cost reduction: engine-side via the
//     "Alisaie Leveilleur" case in cost_modifiers.go ScanCostModifiers.
//     Discount {2} when seat.Turn.SpellsCast == 1 (the spell being
//     cost-scanned is about to be the 2nd cast — SpellsCast increments
//     post-cast in cast_counts.go).
func registerAlisaieLeveilleur(r *Registry) {
	r.OnETB("Alisaie Leveilleur", alisaieLeveilleurETB)
}

func alisaieLeveilleurETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "alisaie_leveilleur_partner_tutor"
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
	var partner *gameengine.Card
	for _, c := range s.Library {
		if c != nil && normalizeName(c.DisplayName()) == normalizeName("Alphinaud Leveilleur") {
			partner = c
			break
		}
	}
	if partner == nil {
		shuffleLibraryPerCard(gs, seat)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   seat,
			"found":  false,
			"reason": "alphinaud_not_in_library",
		})
		return
	}
	gameengine.MoveCard(gs, partner, seat, "library", "hand", slug)
	shuffleLibraryPerCard(gs, seat)
	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"found":  []string{partner.DisplayName()},
			"reason": slug,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    seat,
		"found":   true,
		"partner": partner.DisplayName(),
	})
}
