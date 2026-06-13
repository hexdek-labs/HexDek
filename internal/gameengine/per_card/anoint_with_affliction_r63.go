package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// anoint_with_affliction_r63.go — per_card handler for Anoint with Affliction.
//
// Oracle text (Instant, {1}{B}):
//
//	Exile target creature if it has mana value 3 or less.
//	Corrupted — Exile that creature instead if its controller has three or
//	more poison counters.
//
// Replacement-orphan tail (r63 census): the base "exile if MV ≤ 3" parses
// to a structured Exile node but the Corrupted upgrade ("exile regardless
// of MV if the controller has 3+ poison") dumped to an inert parsed_tail.
// So the spell could never exile a big creature even against a poisoned
// opponent. A bespoke OnResolve handler evaluates Corrupted and exiles the
// chosen target when EITHER gate is met. Pure zone change (no damage
// pipeline) — safe to reimplement base + conditional.
//
// AI targeting: removal — the biggest opponent creature that is currently
// EXILE-eligible (MV ≤ 3, or any MV when the defending player is Corrupted).
func init() {
	registerAnointWithAfflictionR63(Global())
	AddResetHook(registerAnointWithAfflictionR63)
}

func registerAnointWithAfflictionR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Anoint with Affliction", anointWithAfflictionResolve)
}

func anointWithAfflictionResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "anoint_with_affliction"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller

	var best *gameengine.Permanent
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		// Corrupted — defending player has 3+ poison: any mana value is
		// exile-eligible. Otherwise only mana value 3 or less.
		corrupted := s.PoisonCounters >= 3
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			eligible := corrupted || p.Card.CMC <= 3
			if !eligible {
				continue
			}
			if best == nil || p.Power() > best.Power() {
				best = p
			}
		}
	}
	if best == nil {
		emitFail(gs, slug, "Anoint with Affliction", "no_eligible_target", nil)
		return
	}
	name := best.Card.DisplayName()
	corrupted := gs.Seats[best.Controller] != nil && gs.Seats[best.Controller].PoisonCounters >= 3
	gameengine.ExilePermanent(gs, best, nil)
	emit(gs, slug, "Anoint with Affliction", map[string]interface{}{
		"seat": seat, "target": name, "corrupted": corrupted, "rule": "614",
	})
	_ = gs.CheckEnd()
}
