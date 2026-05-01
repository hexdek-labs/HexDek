package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZimoneInfiniteAnalyst wires Zimone, Infinite Analyst.
//
// Oracle text:
//
//	The first spell you cast with {X} in its mana cost each turn costs
//	{1} less to cast for each +1/+1 counter on Zimone.
//	Whenever you cast your first spell with {X} in its mana cost each
//	turn, put two +1/+1 counters on Zimone.
//
// Implementation:
//   - Cost reduction lives in cost_modifiers.go (case
//     "Zimone, Infinite Analyst") so it slots into CR §601.2f cost
//     calculation. The reduction is gated to the first X-cost spell
//     each turn via seat.Flags["zimone_x_spell_turn"].
//   - OnCast trigger: detects spells with {X} in mana cost, gates to
//     first-per-turn via the same flag, and adds two +1/+1 counters.
//     Cost reduction reads counters BEFORE the trigger fires (cost is
//     calculated when announcing the spell; triggers go on the stack
//     after — CR §601.2f vs §603.3), so the reduction reflects the
//     pre-trigger counter count, not the post-trigger one. Subsequent
//     X-spells in the turn get neither the reduction nor counters.
//   - end_step: clears the per-turn flag.
const zimoneXSpellTurnKey = "zimone_x_spell_turn"

func registerZimoneInfiniteAnalyst(r *Registry) {
	r.OnTrigger("Zimone, Infinite Analyst", "spell_cast", zimoneOnSpellCast)
}

func zimoneOnSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zimone_first_x_spell"
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
	if !gameengine.ManaCostContainsX(card) {
		return
	}
	// Skip Zimone casting itself.
	if card == perm.Card {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	if seat.Flags[zimoneXSpellTurnKey] == gs.Turn {
		return
	}
	seat.Flags[zimoneXSpellTurnKey] = gs.Turn

	perm.AddCounter("+1/+1", 2)
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"x_spell":         card.DisplayName(),
		"counters_added":  2,
		"plus_one_total":  perm.Counters["+1/+1"],
	})
}
