package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLaviniaAzoriusRenegade wires Lavinia, Azorius Renegade.
//
// Oracle text:
//
//	Each opponent can't cast noncreature spells with mana value greater
//	than the number of lands that player controls.
//	Whenever an opponent casts a spell, if no mana was spent to cast it,
//	counter that spell.
//
// Implementation: both clauses are static replacement / cost-checking
// effects that the engine doesn't yet plumb through. emitPartial.
func registerLaviniaAzoriusRenegade(r *Registry) {
	r.OnTrigger("Lavinia, Azorius Renegade", "spell_cast_by_opponent", laviniaSpellCast)
}

func laviniaSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lavinia_free_spell_counter"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat == perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	// R53 batch N port: "if no mana was spent to cast it". The cast
	// pipeline doesn't surface per-cast mana-spent attribution; the
	// conservative approximation is CMC==0 (Moxen, Lotus Petal,
	// Memnite, Mishra's Bauble, etc.). Force-of-Will-style alt-cost
	// casts on >0-CMC spells still slip through — emitPartial flags
	// the gap.
	if gameengine.ManaCostOf(card) > 0 {
		return
	}
	// Find the matching StackItem (most recent push of this Card).
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		item := gs.Stack[i]
		if item == nil || item.Card != card {
			continue
		}
		if item.Countered {
			break
		}
		item.Countered = true
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":            perm.Controller,
			"countered_spell": card.DisplayName(),
			"caster_seat":     casterSeat,
		})
		break
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"mv_above_lands_noncreature_cast_restriction_engine_side_unimplemented")
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"free_cast_detection_approximated_as_cmc_zero_alt_cost_spells_slip_through")
}
