package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIxidron wires up Ixidron.
//
// Oracle text:
//
//	As Ixidron enters the battlefield, turn all other nontoken creatures
//	face down. (They're 2/2 creatures.)
//	Ixidron's power and toughness are each equal to the number of
//	face-down creatures on the battlefield.
//
// Implementation: on ETB, iterate all creatures on the battlefield that
// are not Ixidron itself and not tokens, set them face-down (Card.FaceDown = true).
// The layer system in layers.go already handles face-down creatures as 2/2
// colorless nameless (layers.go:285-313).
func registerIxidron(r *Registry) {
	r.OnETB("Ixidron", ixidronETB)
}

func ixidronETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ixidron_mass_face_down"
	if gs == nil || perm == nil {
		return
	}

	flipped := 0
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			// Skip Ixidron itself.
			if p == perm {
				continue
			}
			// Skip tokens.
			if p.IsToken() {
				continue
			}
			// Only affect creatures.
			if !p.IsCreature() {
				continue
			}
			// Skip already face-down creatures.
			if p.Card.FaceDown {
				continue
			}
			// Turn face down.
			p.Card.FaceDown = true
			flipped++
			gs.LogEvent(gameengine.Event{
				Kind:   "turned_face_down",
				Seat:   p.Controller,
				Source: p.Card.DisplayName(),
				Details: map[string]interface{}{
					"reason": "ixidron_etb",
					"rule":   "702.36",
				},
			})
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"creatures_flipped": flipped,
	})
}
