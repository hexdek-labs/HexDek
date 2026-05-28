package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChargingWarBoar wires Charging War Boar — the canonical
// Ward—Pay N life test card for the r60 unified WardCost primitive
// (see ward_alt_payment.go).
//
// Printed oracle (Scryfall, verified 2026-05-27):
//
//	Ward—Pay 3 life. (Whenever this creature becomes the target of a
//	spell or ability an opponent controls, counter it unless that
//	player pays 3 life.)
//	Trample
//
// Implementation: ETB stamps WardCost{Type: Life, Amount: 3}.
// CheckWardOnTargeting routes through payWardByLife when an opponent
// targets the Boar.
func registerChargingWarBoar(r *Registry) {
	r.OnETB("Charging War Boar", chargingWarBoarETB)
}

func chargingWarBoarETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "charging_war_boar_ward_pay_life"
	if gs == nil || perm == nil {
		return
	}
	gameengine.SetWardCost(perm, gameengine.WardCost{
		Type:   gameengine.WardCostLife,
		Amount: 3,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"ward_alt_kind": "pay_life",
		"life_cost":     3,
	})
}
