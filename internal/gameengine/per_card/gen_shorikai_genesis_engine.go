package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShorikaiGenesisEngine wires Shorikai, Genesis Engine.
//
// Oracle text:
//
//   {1}, {T}: Draw two cards, then discard a card. Create a 1/1 colorless Pilot creature token with "This token crews Vehicles as though its power were 2 greater."
//   Crew 8 (Tap any number of creatures you control with total power 8 or more: This Vehicle becomes an artifact creature until end of turn.)
//
// Auto-generated activated ability handler.
func registerShorikaiGenesisEngine(r *Registry) {
	r.OnActivated("Shorikai, Genesis Engine", shorikaiGenesisEngineActivate)
}

func shorikaiGenesisEngineActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "shorikai_genesis_engine_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for i := 0; i < 2; i++ {
		drawOne(gs, src.Controller, src.Card.DisplayName())
	}
	token := &gameengine.Card{
		Name:          "1/1 Token Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
