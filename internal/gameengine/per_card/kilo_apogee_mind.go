package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKiloApogeeMind wires Kilo, Apogee Mind.
//
// Implemented oracle text (per task spec):
//
//	When Kilo, Apogee Mind enters, draw cards equal to the number of
//	legendary creatures you control.
//	Whenever you cast a legendary spell, scry 2.
//
// Note: The current Scryfall printing reads "Haste / Whenever Kilo
// becomes tapped, proliferate." The simplified spec above is what this
// engine implements; if the printed version is later wired up, replace
// this handler.
//
// Implementation:
//   - OnETB: Kilo is already on the battlefield when the ETB hook fires
//     (see etb_dispatch.go), so the count includes Kilo himself.
//   - "Whenever you cast a legendary spell" listens on spell_cast and
//     gates on caster_seat == controller and "legendary" supertype on
//     the cast card. Kilo himself counts ("a legendary spell" — no
//     "another" qualifier in the user-supplied text), but in practice
//     Kilo is in the command zone or graveyard at that moment so the
//     trigger source isn't on the battlefield to fire anyway.
func registerKiloApogeeMind(r *Registry) {
	r.OnETB("Kilo, Apogee Mind", kiloApogeeMindETB)
	r.OnTrigger("Kilo, Apogee Mind", "spell_cast", kiloApogeeMindLegendaryCast)
}

func kiloApogeeMindETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kilo_apogee_mind_etb_draw"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "creature") {
			continue
		}
		if !cardHasType(p.Card, "legendary") {
			continue
		}
		count++
	}
	if count <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"drawn":  0,
			"reason": "no_legendary_creatures",
		})
		return
	}
	drawn := 0
	for i := 0; i < count; i++ {
		if drawOne(gs, perm.Controller, perm.Card.DisplayName()) != nil {
			drawn++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"count": count,
		"drawn": drawn,
	})
}

func kiloApogeeMindLegendaryCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kilo_apogee_mind_legendary_scry"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "legendary") {
		return
	}

	gameengine.Scry(gs, perm.Controller, 2)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_spell": card.DisplayName(),
	})
}
