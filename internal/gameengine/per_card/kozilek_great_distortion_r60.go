package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// kozilek_great_distortion_r60.go — per_card handler for Kozilek, the
// Great Distortion.
//
// Oracle text (Scryfall / ast_dataset):
//
//	When you cast this spell, if you have fewer than seven cards in hand,
//	draw cards equal to the difference.
//	Menace
//	Discard a card with mana value X: Counter target spell with mana value X.
//
// {8}{C}{C} Creature — Eldrazi. A premier ramp finisher whose cast
// trigger refills your hand to seven (fueling the discard-to-counter
// activated ability). The cast-trigger draw parses to an inert
// `cast_trigger_tail` node (no structured Draw), so it drew ZERO cards.
//
// Implementation:
//   - OnCast. CR §601: the "when you cast" trigger fires as the spell
//     goes on the stack — at which point Kozilek is on the stack, not in
//     hand. Draw up to (7 - current hand size) via DrawN. Menace and the
//     discard-counter activated ability are handled elsewhere (keyword
//     layer / activation); only the cast-trigger draw was dead.
func init() {
	registerKozilekGreatDistortionR60(Global())
	AddResetHook(registerKozilekGreatDistortionR60)
}

func registerKozilekGreatDistortionR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnCast("Kozilek, the Great Distortion", kozilekGreatDistortionCast)
}

func kozilekGreatDistortionCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "kozilek_great_distortion"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}
	hand := len(gs.Seats[seat].Hand)
	if hand >= 7 {
		emit(gs, slug, "Kozilek, the Great Distortion", map[string]interface{}{
			"seat": seat, "hand": hand, "drawn": 0,
		})
		return
	}
	var src *gameengine.Permanent
	if item.Card != nil {
		src = &gameengine.Permanent{Card: item.Card, Controller: seat, Owner: item.Card.Owner}
	}
	drawn := gameengine.DrawN(gs, seat, 7-hand, src)
	emit(gs, slug, "Kozilek, the Great Distortion", map[string]interface{}{
		"seat": seat, "hand": hand, "drawn": drawn,
	})
}
