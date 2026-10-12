package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAkromaAngelOfFury wires Akroma, Angel of Fury.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{5}{R}{R}{R}
//	Legendary Creature — Angel
//	6/6
//	This spell can't be countered.
//	Flying, trample, protection from white and from blue
//	{R}: Akroma gets +1/+0 until end of turn.
//	Morph {3}{R}{R}{R}
//
// Most abilities are AST-handled keywords. The {R} pump activated
// ability is modeled as an OnActivated path that bumps temp_power.
// Morph is engine-side cast machinery — emitPartial.
func registerAkromaAngelOfFury(r *Registry) {
	r.OnActivated("Akroma, Angel of Fury", akromaAngelOfFuryActivate)
}

func akromaAngelOfFuryActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	pumpUntilEOT(gs, src, 1, 0)
	emit(gs, "akroma_angel_of_fury_pump", src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"new_power": src.Power(),
	})
}
