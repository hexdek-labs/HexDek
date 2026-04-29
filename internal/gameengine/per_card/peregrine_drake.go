package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPeregrineDrake wires up Peregrine Drake.
//
// Oracle text:
//
//	Flying
//	When Peregrine Drake enters the battlefield, untap up to five
//	lands.
//
// The 5-mana ETB refund makes Drake net mana-positive when combined
// with anything that flickers it (Deadeye Navigator, Conjurer's
// Closet, Ephemerate, Cloudshift, Displacer Kitten). With Deadeye
// Navigator specifically, the soulbond-activated flicker costs {1}{U}
// and Drake untaps 5 lands — net +2 to +3 mana per activation,
// infinite mana.
//
// Batch #2 scope:
//   - OnETB: untap up to 5 tapped lands you control. "Up to five"
//     policy: untap ALL tapped lands with pip count ≤ 5 (greedy).
//     If more than 5 tapped lands exist, untap the 5 highest-value
//     (heuristic: prefer multi-pip lands like Cavern of Souls → then
//     duals → then basics). MVP heuristic is simpler: take first 5.
func registerPeregrineDrake(r *Registry) {
	r.OnETB("Peregrine Drake", peregrineDrakeETB)
}

func peregrineDrakeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "peregrine_drake_untap_five"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	untapped := 0
	const limit = 5
	for _, p := range s.Battlefield {
		if untapped >= limit {
			break
		}
		if p == nil || !p.IsLand() {
			continue
		}
		if !p.Tapped {
			continue
		}
		p.Tapped = false
		untapped++
		gs.LogEvent(gameengine.Event{
			Kind:   "untap",
			Seat:   seat,
			Target: seat,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"target_card": p.Card.DisplayName(),
				"reason":      "peregrine_drake_etb",
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"untapped": untapped,
		"limit":    limit,
	})
}
