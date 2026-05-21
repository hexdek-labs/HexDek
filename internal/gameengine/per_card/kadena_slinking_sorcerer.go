package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKadenaSlinkingSorcerer wires Kadena, Slinking Sorcerer.
//
// Oracle text:
//
//	The first face-down creature spell you cast each turn costs {3}
//	less to cast.
//	Whenever a face-down creature you control enters, draw a card.
//
// Implementation (R53 batch N port):
//   - First-cast cost reduction: wired in cost_modifiers.go under a
//     scan that checks seat.Flags["kadena_used_turn"] (sentinel
//     gs.Turn+1) and the cast Card carrying type tag "face_down".
//   - ETB draw: permanent_etb trigger fires when a face-down
//     creature (tagged via the "face_down" type) enters under
//     Kadena's controller. Draw 1 card.
//   - seat.Flags["kadena_used_turn"] is bumped on the controller's
//     spell_cast for any face_down creature spell so the discount
//     is single-use per turn.
func registerKadenaSlinkingSorcerer(r *Registry) {
	r.OnETB("Kadena, Slinking Sorcerer", kadenaETB)
	r.OnTrigger("Kadena, Slinking Sorcerer", "permanent_etb", kadenaFaceDownETB)
	r.OnTrigger("Kadena, Slinking Sorcerer", "spell_cast", kadenaMarkUsed)
}

func kadenaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kadena_etb"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func kadenaFaceDownETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kadena_face_down_etb_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	if entering.Controller != perm.Controller {
		return
	}
	if !entering.IsCreature() {
		return
	}
	if !cardHasType(entering.Card, "face_down") {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"face_down": entering.Card.DisplayName(),
	})
}

func kadenaMarkUsed(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || !cardHasType(card, "face_down") || !cardHasType(card, "creature") {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["kadena_used_turn"] = gs.Turn + 1
}
