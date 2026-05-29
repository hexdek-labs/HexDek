package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSkitterbeamBattalion wires Skitterbeam Battalion (Muninn parser-gap
// #84, 6.6K hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{9}
//	Artifact Creature — Construct
//	Prototype {3}{R}{R} — 2/2
//	Trample, haste
//	When this creature enters, if you cast it, create two tokens that
//	are copies of it.
//
// Implementation (Muninn #101-120 wave):
//   - Trample/haste handled by AST keyword pipeline.
//   - Prototype is a cast-time alternative cost & p/t override — engine
//     prototype support is incomplete; emitPartial.
//   - OnETB gate on perm.Flags["was_cast"]. Two token copies enter under
//     the controller. Each copy carries the "token" type so the ETB-gated
//     clause won't recursively fire (tokens enter without being cast).
//     Mirrors Phoenix Fleet Airship's deep-copy approach.
func registerSkitterbeamBattalion(r *Registry) {
	r.OnETB("Skitterbeam Battalion", skitterbeamBattalionETB)
}

func skitterbeamBattalionETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "skitterbeam_battalion_two_copies"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Flags == nil || perm.Flags["was_cast"] == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "not_cast", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	tokens := 0
	for i := 0; i < 2; i++ {
		// InstanceID Phase 5: token-as-copy chokepoint.
		card := gameengine.MintTokenAsCopyOf(gs, perm.Card, seat, "")
		if card == nil {
			continue
		}
		if enterBattlefieldWithETB(gs, seat, card, false) != nil {
			tokens++
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   seat,
		"tokens": tokens,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"prototype_alt_cost_and_size_override_not_fully_modelled")
}
