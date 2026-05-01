package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJuneBountyHunter wires June, Bounty Hunter.
//
// Oracle text:
//
//   June can't be blocked as long as you've drawn two or more cards this turn.
//   {1}, Sacrifice another creature: Create a Clue token. Activate only during your turn. (It's an artifact with "{2}, Sacrifice this token: Draw a card.")
//
// Auto-generated activated ability handler.
func registerJuneBountyHunter(r *Registry) {
	r.OnActivated("June, Bounty Hunter", juneBountyHunterActivate)
}

func juneBountyHunterActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "june_bounty_hunter_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, src.Controller, src.Card.DisplayName())
	token := &gameengine.Card{
		Name:          "1/1 Creature Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "creature"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
