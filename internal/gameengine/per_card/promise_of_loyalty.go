package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Promise of Loyalty — {4}{W} Sorcery.
//
//	Each player puts a vow counter on a creature they control and
//	sacrifices the rest. Each of those creatures can't attack you or
//	planeswalkers you control for as long as it has a vow counter on it.
//
// A symmetric one-creature-each board sweeper (38 decks). Both halves
// parsed to inert `custom` slugs, so it resolved to a no-op. This handler
// makes each player keep ONE creature (their strongest — "of their choice")
// with a vow counter and sacrifice the rest.
//
// Scope: the "can't attack you" restriction the vow counter confers is a
// continuous combat-restriction (no per_card static hook); it is not modeled
// here. The dominant board effect — each player down to one creature — is.
func init() {
	registerPromiseOfLoyalty(Global())
	AddResetHook(registerPromiseOfLoyalty)
}

func registerPromiseOfLoyalty(r *Registry) {
	r.OnResolve("Promise of Loyalty", promiseOfLoyaltyResolve)
}

func promiseOfLoyaltyResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "promise_of_loyalty"
	if gs == nil || item == nil {
		return
	}
	kept := 0
	sacked := 0
	for _, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		var creatures []*gameengine.Permanent
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.IsCreature() {
				creatures = append(creatures, p)
			}
		}
		if len(creatures) == 0 {
			continue
		}
		// Keep the strongest (by power, tie-broken by toughness).
		keep := creatures[0]
		for _, p := range creatures[1:] {
			if p.Power() > keep.Power() ||
				(p.Power() == keep.Power() && p.Toughness() > keep.Toughness()) {
				keep = p
			}
		}
		keep.AddCounter("vow", 1)
		kept++
		for _, p := range creatures {
			if p != keep {
				gameengine.SacrificePermanent(gs, p, "promise_of_loyalty")
				sacked++
			}
		}
	}
	emit(gs, slug, "Promise of Loyalty", map[string]interface{}{
		"kept":       kept,
		"sacrificed": sacked,
	})
}
