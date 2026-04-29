package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAdNauseam wires up Ad Nauseam.
//
// Oracle text:
//
//	Reveal the top card of your library and put that card into your
//	hand. You lose life equal to its mana value. You may repeat this
//	process any number of times.
//
// THE cEDH win-engine. Paired with Angel's Grace (prevents the life
// loss from killing you) or a sufficient life cushion + low-curve deck,
// the spell assembles a lethal storm pile (Tendrils / Grapeshot /
// Thoracle-Consultation) in a single resolution.
//
// Policy for "any number of times":
//   - Greedy: keep flipping until one of these triggers fires:
//       1. Library is empty → §704.5b draw-loss next SBA pass, stop.
//       2. Controller is at or below 1 life and no Angel's Grace is
//          active → stop (self-preservation heuristic).
//       3. We've flipped a hard-cap of 30 cards → stop (defensive
//          against infinite libraries in pathological tests).
//   - Ad Nauseam's target is SELF; CR §609.3 — the spell resolves
//     exactly once and repeats the reveal in a loop. Each card's mana
//     value is read from Card.CMC (primary) or parsed out of Card.Types
//     ("cmc:N" marker; test-friendly fallback).
//
// The "Angel's Grace active" check reads seat.Flags["angels_grace_eot"]
// which is set by the Angel's Grace resolve handler (see angels_grace.go
// in this batch). When that flag is true, we remove the life-floor stop
// condition — matching the canonical cEDH line "Angel's Grace → Ad
// Nauseam to 0 life → Tendrils/Grapeshot storm kill".
func registerAdNauseam(r *Registry) {
	r.OnResolve("Ad Nauseam", adNauseamResolve)
}

func adNauseamResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "ad_nauseam"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	graceActive := gs.Flags != nil && gs.Flags["angels_grace_eot_seat_"+intToStr(seat)] > 0
	// Hard iteration cap — defensive guard against pathologically large
	// libraries. cEDH Ad Nauseam decks flip ~20 cards typical.
	const maxIter = 30
	revealed := 0
	totalLifeLoss := 0
	for revealed < maxIter && len(s.Library) > 0 {
		c := s.Library[0]
		gameengine.MoveCard(gs, c, seat, "library", "hand", "effect")
		cmc := cardCMC(c)
		s.Life -= cmc
		totalLifeLoss += cmc
		revealed++
		gs.LogEvent(gameengine.Event{
			Kind:   "reveal_and_draw",
			Seat:   seat,
			Target: seat,
			Source: item.Card.DisplayName(),
			Amount: cmc,
			Details: map[string]interface{}{
				"revealed_card": c.DisplayName(),
				"cmc":           cmc,
				"life_after":    s.Life,
			},
		})
		// Stop conditions.
		if s.Life <= 1 && !graceActive {
			// Self-preservation: don't flip the one that kills us unless
			// Angel's Grace keeps us at 1.
			break
		}
		if graceActive && s.Life <= -30 {
			// Extreme floor — even with Grace, don't flip into infinite
			// negative life (some tests set up impossible scenarios).
			break
		}
	}
	// SBA-visible lose-life event for §704.5a bookkeeping.
	if totalLifeLoss > 0 {
		gs.LogEvent(gameengine.Event{
			Kind:   "lose_life",
			Seat:   seat,
			Target: seat,
			Source: item.Card.DisplayName(),
			Amount: totalLifeLoss,
			Details: map[string]interface{}{
				"reason": "ad_nauseam_reveals",
			},
		})
	}
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat":             seat,
		"cards_revealed":   revealed,
		"total_life_loss":  totalLifeLoss,
		"final_life":       s.Life,
		"angels_grace":     graceActive,
		"library_empty":    len(s.Library) == 0,
	})
	// If we bottomed out below 0 life and Grace isn't up, §704.5a will
	// end the game next SBA pass — which is correct behavior.
	_ = gs.CheckEnd()
}
