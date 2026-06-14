package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKataraWaterbendingMaster wires Katara, Waterbending Master.
//
// Oracle text:
//
//	Whenever you cast a spell during an opponent's turn, you get an
//	experience counter.
//	Whenever Katara attacks, you may draw a card for each experience
//	counter you have. If you do, discard a card.
//
// Implementation:
//   - spell_cast on an opponent's turn → +1 experience counter on
//     controller's seat (tracked via perm.Flags["katara_xp"]).
//   - attacks: draw N, then discard 1. AI policy: always take the deal
//     if we have at least 1 experience counter.
func registerKataraWaterbendingMaster(r *Registry) {
	r.OnTrigger("Katara, Waterbending Master", "spell_cast", kataraSpellCast)
	r.OnTrigger("Katara, Waterbending Master", "attacks", kataraAttacks)
}

func kataraSpellCast(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	casterSeat, _ := ctx["caster_seat"].(int)
	if casterSeat != perm.Controller {
		return
	}
	if gs.Active == perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["katara_xp"]++
	emit(gs, "katara_experience", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"xp":   perm.Flags["katara_xp"],
	})
}

func kataraAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "katara_attacks_draw_discard"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever ~ attacks" fires only when this creature
	// itself is the attacker. The creature_attacks event is dispatched once per
	// declared attacker, so the old seat check (ctx["seat"] is never populated by
	// the attack fire → it only matched seat 0) let the effect multiply per
	// attacker. Mirrors the canonical gate and the Aang and Katara fix (5254f9cf).
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	if perm.Flags == nil {
		return
	}
	xp := perm.Flags["katara_xp"]
	if xp <= 0 {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Wave 2 multi-step migration: route draws + discard through the
	// canonical helpers so §614 + observer triggers fire (no card_drawn,
	// no card_discarded otherwise — both are commander-deck-staple
	// trigger sources). Pre-r60 direct-spliced library/hand and manual
	// appended; the chokepoints handle source removal + ETB-equivalents.
	drawn := 0
	for i := 0; i < xp && len(seat.Library) > 0; i++ {
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, perm.Controller, "library", "hand", "katara_draw")
		drawn++
	}
	if drawn > 0 && len(seat.Hand) > 0 {
		discardIdx := len(seat.Hand) - 1
		card := seat.Hand[discardIdx]
		gameengine.DiscardCard(gs, card, perm.Controller)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"drawn": drawn,
	})
}
