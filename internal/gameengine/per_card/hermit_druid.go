package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHermitDruid wires up Hermit Druid.
//
// Oracle text:
//
//	{G}, {T}: Reveal cards from the top of your library until you
//	reveal a basic land card. Put that card into your hand and all
//	other cards revealed this way into your graveyard.
//
// Muldrotha wincon: a no-basics deck mills its entire library in one
// activation. Then a reanimator payload in the graveyard wins next
// turn.
//
// Implementation:
//   - OnActivated(0, ...) — reveal from top of library; mill (put into
//     graveyard) every non-basic-land; on first basic land, put it into
//     hand and stop.
//   - A deck with zero basics mills the entire library. Combined with
//     Laboratory Maniac or Thassa's Oracle, this wins.
//
// Basic-land detection: scans Card.Types for "basic" + "land", or for
// the canonical basic names ("Plains", "Island", "Swamp", "Mountain",
// "Forest", "Wastes"). Tests can mark cards with the `Types` slice.
func registerHermitDruid(r *Registry) {
	r.OnActivated("Hermit Druid", hermitDruidActivate)
}

func hermitDruidActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "hermit_druid_mill_to_basic"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	milled := 0
	foundBasic := false
	for len(s.Library) > 0 {
		c := s.Library[0]
		if isBasicLand(c) {
			gameengine.MoveCard(gs, c, seat, "library", "hand", "effect")
			foundBasic = true
			break
		}
		gameengine.MoveCard(gs, c, seat, "library", "graveyard", "mill")
		milled++
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"milled_count": milled,
		"found_basic":  foundBasic,
		"library_remaining": len(s.Library),
	})
	gs.LogEvent(gameengine.Event{
		Kind:   "mill",
		Seat:   seat,
		Target: seat,
		Source: src.Card.DisplayName(),
		Amount: milled,
	})
}

func isBasicLand(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	// Basic subtype marker.
	for _, t := range c.Types {
		if t == "basic" {
			// Paired with "land" in the same slice.
			for _, u := range c.Types {
				if u == "land" {
					return true
				}
			}
		}
	}
	// Name-based fallback.
	switch c.DisplayName() {
	case "Plains", "Island", "Swamp", "Mountain", "Forest", "Wastes",
		"Snow-Covered Plains", "Snow-Covered Island", "Snow-Covered Swamp",
		"Snow-Covered Mountain", "Snow-Covered Forest":
		return true
	}
	return false
}
