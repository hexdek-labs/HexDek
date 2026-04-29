package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEternalWitness wires up Eternal Witness.
//
// Oracle text:
//
//   When Eternal Witness enters the battlefield, you may return target
//   card from your graveyard to your hand.
//
// 1GG creature (2/1). cEDH staple for recursion loops with blink
// effects (Deadeye Navigator, Displacer Kitten, Cloudstone Curio).
func registerEternalWitness(r *Registry) {
	r.OnETB("Eternal Witness", eternalWitnessETB)
}

func eternalWitnessETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "eternal_witness_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if len(s.Graveyard) == 0 {
		emit(gs, slug, "Eternal Witness", map[string]interface{}{
			"seat":     seat,
			"returned": false,
			"reason":   "graveyard_empty",
		})
		return
	}

	// MVP heuristic: return the most recently-added card (top of graveyard).
	// In a real game the player chooses; the Hat could be consulted here.
	card := s.Graveyard[len(s.Graveyard)-1]
	gameengine.MoveCard(gs, card, seat, "graveyard", "hand", "return-from-graveyard")

	gs.LogEvent(gameengine.Event{
		Kind:   "return_to_hand",
		Seat:   seat,
		Target: seat,
		Source: "Eternal Witness",
		Details: map[string]interface{}{
			"card":   card.DisplayName(),
			"from":   "graveyard",
			"reason": "eternal_witness_etb",
		},
	})
	emit(gs, slug, "Eternal Witness", map[string]interface{}{
		"seat":          seat,
		"returned":      true,
		"returned_card": card.DisplayName(),
	})
}
