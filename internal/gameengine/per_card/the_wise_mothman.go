package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheWiseMothman wires The Wise Mothman.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Flying
//	Whenever The Wise Mothman enters or attacks, each player gets a rad
//	counter.
//	Whenever one or more nonland cards are milled, put a +1/+1 counter
//	on each of up to X target creatures, where X is the number of nonland
//	cards milled this way.
//
// Implementation:
//   - Flying — handled by the AST keyword pipeline.
//   - OnETB: each living seat gains one rad counter (Flags["rad_counters"]++).
//   - "creature_attacks" filtered to Mothman: same rad-handout when Mothman
//     herself is the declared attacker.
//   - The third clause (mill-driven +1/+1 spread) requires a "nonland card
//     milled" trigger that the engine does not emit, so we log it as
//     partial. Rad counters cause precombat mills that already drain a
//     significant fraction of opponents' libraries; the mill-payoff clause
//     would compound that, but cleanly modeling it requires engine-level
//     plumbing outside this batch's scope.
func registerTheWiseMothman(r *Registry) {
	r.OnETB("The Wise Mothman", theWiseMothmanETB)
	r.OnTrigger("The Wise Mothman", "creature_attacks", theWiseMothmanAttacks)
}

func theWiseMothmanETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_wise_mothman_etb_rad_each_player"
	if gs == nil || perm == nil {
		return
	}
	dealtRad := theWiseMothmanHandOutRad(gs, perm, "etb")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"radded":    dealtRad,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"mill_payoff_+1/+1_counters_unimplemented")
}

func theWiseMothmanAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_wise_mothman_attack_rad_each_player"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atkPerm, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atkPerm != perm {
		return
	}
	dealtRad := theWiseMothmanHandOutRad(gs, perm, "attack")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"radded": dealtRad,
	})
}

// theWiseMothmanHandOutRad gives each living seat one rad counter and
// returns the list of seats that were affected for logging.
func theWiseMothmanHandOutRad(gs *gameengine.GameState, perm *gameengine.Permanent, reason string) []int {
	out := make([]int, 0, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		s.Flags["rad_counters"]++
		gs.LogEvent(gameengine.Event{
			Kind:   "counter_mod",
			Seat:   perm.Controller,
			Target: i,
			Source: perm.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"counter_kind": "rad",
				"op":           "put",
				"on_player":    true,
				"reason":       "the_wise_mothman_" + reason,
			},
		})
		out = append(out, i)
	}
	return out
}
