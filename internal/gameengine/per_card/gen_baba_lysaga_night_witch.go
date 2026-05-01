package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBabaLysagaNightWitch wires Baba Lysaga, Night Witch.
//
// Oracle text:
//
//   {T}, Sacrifice up to three permanents: If there were three or more card types among the sacrificed permanents, each opponent loses 3 life, you gain 3 life, and you draw three cards.
//
// Auto-generated activated ability handler.
func registerBabaLysagaNightWitch(r *Registry) {
	r.OnActivated("Baba Lysaga, Night Witch", babaLysagaNightWitchActivate)
}

func babaLysagaNightWitchActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "baba_lysaga_night_witch_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for i := 0; i < 3; i++ {
		drawOne(gs, src.Controller, src.Card.DisplayName())
	}
	gameengine.GainLife(gs, src.Controller, 3, src.Card.DisplayName())
	for _, opp := range gs.Opponents(src.Controller) {
		if gs.Seats[opp] != nil && !gs.Seats[opp].Lost {
			gs.Seats[opp].Life -= 3
		}
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
