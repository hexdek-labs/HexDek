package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRakdosPatronOfChaos wires Rakdos, Patron of Chaos.
//
// Oracle text:
//
//	Flying, trample
//	At the beginning of your end step, target opponent may sacrifice
//	two nonland, nontoken permanents of their choice. If they don't,
//	you draw two cards.
//
// Implementation (R53 batch N port):
//   - Flying / trample: AST keyword pipeline.
//   - End-step trigger gated on active_seat == controller. AI policy
//     for the targeted opponent: ALWAYS decline the sacrifice (a
//     "may" choice trading 2 permanents for the opponent's 2 cards
//     is value-negative for the opp). Controller draws 2 cards.
//   - The actual sacrifice-prompt routing would let the AI weigh the
//     trade; per-card layer can't drive that yet — emitPartial flags
//     the simplification.
func registerRakdosPatronOfChaos(r *Registry) {
	r.OnTrigger("Rakdos, Patron of Chaos", "end_step", rakdosPatronEndStep)
}

func rakdosPatronEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "rakdos_patron_end_step_draw_two"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Target opponent — highest-permanent-count opp.
	target := -1
	bestN := -1
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if len(s.Battlefield) > bestN {
			bestN = len(s.Battlefield)
			target = opp
		}
	}
	if target < 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"reason": "no_opponent",
		})
		return
	}
	drewA := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drewB := drawOne(gs, perm.Controller, perm.Card.DisplayName())
	drew := 0
	if drewA != nil {
		drew++
	}
	if drewB != nil {
		drew++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"target":   target,
		"drew":     drew,
		"declined": true,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"opp_sacrifice_choice_auto_declined_no_ai_routing_for_may_sac_yet")
}
