package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEzrimAgencyChief wires Ezrim, Agency Chief.
//
// Oracle text:
//
//   Flying
//   When Ezrim enters, investigate twice. (To investigate, create a Clue token. It's an artifact with "{2}, Sacrifice this token: Draw a card.")
//   {1}, Sacrifice an artifact: Ezrim gains your choice of vigilance, lifelink, or hexproof until end of turn.
//
// Auto-generated ETB handler.
func registerEzrimAgencyChief(r *Registry) {
	r.OnETB("Ezrim, Agency Chief", ezrimAgencyChiefETB)
}

func ezrimAgencyChiefETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ezrim_agency_chief_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, seat, perm.Card.DisplayName())
	gameengine.GainLife(gs, seat, 1, perm.Card.DisplayName())
	token := &gameengine.Card{
		Name:          "1/1 Creature Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "creature"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
