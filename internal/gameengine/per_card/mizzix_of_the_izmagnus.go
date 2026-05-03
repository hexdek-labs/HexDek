package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMizzixOfTheIzmagnus wires Mizzix of the Izmagnus.
//
// Oracle text:
//
//	Whenever you cast an instant or sorcery spell with mana value greater
//	than the number of experience counters you have, you get an experience
//	counter.
//	Instant and sorcery spells you cast cost {1} less to cast for each
//	experience counter you have.
//
// Experience counters live in seat.Flags["experience_counters"], matching
// the engine's existing experience-counter wiring (resolve_helpers.go,
// scaling.go) so proliferate and ScalingAmount references see the same value.
//
// Implementation:
//   - OnTrigger("spell_cast"): when Mizzix's controller casts an instant or
//     sorcery whose CMC > current experience-counter count, gain one
//     experience counter.
//   - Cost reduction: handled in cost_modifiers.go via a ScanCostModifiers
//     case keyed on "Mizzix of the Izmagnus" — the experience-counter count
//     on the caster's seat produces a CostModReduction for each instant or
//     sorcery spell cast by Mizzix's controller.
func registerMizzixOfTheIzmagnus(r *Registry) {
	r.OnTrigger("Mizzix of the Izmagnus", "spell_cast", mizzixSpellCast)
}

func mizzixSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mizzix_of_the_izmagnus_experience"
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
	// "an instant or sorcery spell" — only instants and sorceries trigger.
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}

	xp := seat.Flags["experience_counters"]
	cmc := gameengine.ManaCostOf(card)

	// "with mana value greater than the number of experience counters you have"
	if cmc <= xp {
		return
	}

	seat.Flags["experience_counters"]++
	gs.LogEvent(gameengine.Event{
		Kind:   "experience_counter",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason":     "mizzix_instant_sorcery_cast",
			"spell":      card.DisplayName(),
			"spell_cmc":  cmc,
			"xp_before":  xp,
			"total":      seat.Flags["experience_counters"],
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"spell":     card.DisplayName(),
		"spell_cmc": cmc,
		"xp_before": xp,
		"xp_after":  seat.Flags["experience_counters"],
	})
}
