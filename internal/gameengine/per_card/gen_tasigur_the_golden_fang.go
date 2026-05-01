package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTasigurTheGoldenFang wires Tasigur, the Golden Fang.
//
// Oracle text:
//
//   Delve (Each card you exile from your graveyard while casting this spell pays for {1}.)
//   {2}{G/U}{G/U}: Mill two cards, then return a nonland card of an opponent's choice from your graveyard to your hand.
//
// Auto-generated activated ability handler.
func registerTasigurTheGoldenFang(r *Registry) {
	r.OnActivated("Tasigur, the Golden Fang", tasigurTheGoldenFangActivate)
}

func tasigurTheGoldenFangActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "tasigur_the_golden_fang_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[src.Controller]
	for i := 0; i < 2; i++ {
		if len(s.Library) == 0 { break }
		card := s.Library[0]
		gameengine.MoveCard(gs, card, src.Controller, "library", "graveyard", "mill")
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
