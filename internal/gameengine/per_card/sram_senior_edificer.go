package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSramSeniorEdificer wires Sram, Senior Edificer.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Sram%2C%20Senior%20Edificer):
//
//	Whenever you cast an Aura, Equipment, or Vehicle spell, draw a card.
//
// {1}{W} 1/2 Legendary Creature — Dwarf Advisor. Voltron / Equipment
// staple — every Aura/Equipment/Vehicle cast cantrips. The 2-mana cost
// and pure-white identity makes him a popular Bruna Light of Alabaster
// / Sigarda's Aid commander combo enabler.
//
// Implementation:
//   - OnTrigger("spell_cast"). Filter chain: (1) caster_seat ==
//     perm.Controller, (2) cast card's Types include "aura" OR
//     "equipment" OR "vehicle".
//   - Action: draw 1.
//
// Subtype detection uses cardHasType, which forwards to the engine's
// CardHasTypeExact helper — already covers the canonical lowercase
// subtype tags used elsewhere in the codebase (see Edgar Markov's
// "vampire" check for the same pattern).
func registerSramSeniorEdificer(r *Registry) {
	r.OnTrigger("Sram, Senior Edificer", "spell_cast", sramOnCast)
}

func sramOnCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sram_senior_edificer"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	caster, _ := ctx["caster_seat"].(int)
	if caster != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "aura") && !cardHasType(card, "equipment") && !cardHasType(card, "vehicle") {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	drawOne(gs, perm.Controller, "Sram, Senior Edificer")

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"cast_spell": card.DisplayName(),
	})
}
