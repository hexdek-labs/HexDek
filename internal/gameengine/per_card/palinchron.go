package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPalinchron wires up Palinchron.
//
// Oracle text:
//
//	Flying
//	When Palinchron enters the battlefield, untap up to seven lands.
//	{5}{U}{U}: Return Palinchron to its owner's hand.
//
// The strictly-better Peregrine Drake: 7 lands untapped, so with any
// copy effect (Phantasmal Image, Sakashima the Impostor, Rite of
// Replication with kicker) it loops infinite mana. The return-to-hand
// activated ability makes it re-castable in the same turn: cast → ETB
// untap 7 → activate for {5}{U}{U} cost → Palinchron returns → cast
// again. Net mana positive if your 7 lands produce more than 7 {U}
// pips' worth of mana.
//
// Handlers:
//   - OnETB: untap up to 7 tapped lands.
//   - OnActivated(0, ...): return Palinchron from battlefield to hand.
//     Cost is assumed paid by the caller.
func registerPalinchron(r *Registry) {
	r.OnETB("Palinchron", palinchronETB)
	r.OnActivated("Palinchron", palinchronActivate)
}

func palinchronETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "palinchron_untap_seven"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	untapped := 0
	const limit = 7
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
				"reason":      "palinchron_etb",
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"untapped": untapped,
		"limit":    limit,
	})
}

func palinchronActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "palinchron_bounce_self"
	if gs == nil || src == nil {
		return
	}
	// Return self to owner's hand via BouncePermanent for proper zone-change
	// handling: replacement effects, LTB triggers, commander redirect.
	if !gameengine.BouncePermanent(gs, src, src, "hand") {
		emitFail(gs, slug, src.Card.DisplayName(), "not_on_battlefield", nil)
		return
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"owner": src.Owner,
	})
}
