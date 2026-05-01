package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerThrasios wires Thrasios, Triton Hero.
//
// Oracle text:
//
//	{4}: Scry 1, then reveal the top card of your library. If it's a
//	land card, put it onto the battlefield tapped. Otherwise, draw a card.
//	Partner
//
// The AST parses the scry + reveal but the conditional "if land → ETB
// tapped; else → draw" falls into conditional_static (no-op). This
// handler implements the full activation.
func registerThrasios(r *Registry) {
	r.OnActivated("Thrasios, Triton Hero", thrasiosActivate)
}

func thrasiosActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "thrasios_scry_draw_ramp"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	if s == nil || len(s.Library) == 0 {
		return
	}

	gameengine.Scry(gs, seat, 1)

	if len(s.Library) == 0 {
		return
	}

	top := s.Library[0]
	if cardHasType(top, "land") {
		s.Library = s.Library[1:]
		enterBattlefieldWithETB(gs, seat, top, true)
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"revealed":    top.DisplayName(),
			"destination": "battlefield_tapped",
		})
	} else {
		gameengine.MoveCard(gs, top, seat, "library", "hand", "draw")
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":        seat,
			"revealed":    top.DisplayName(),
			"destination": "hand",
		})
	}
}
