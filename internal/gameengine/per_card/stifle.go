package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStifle wires Stifle.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Stifle):
//
//	Counter target activated or triggered ability. (Mana abilities
//	can't be targeted.)
//
// {U} Instant. Critical legacy/cEDH staple — counters fetchland
// triggers, planeswalker abilities, sac-outlet activations, sagas,
// and storm triggers. The "mana abilities can't be targeted" clause
// means we filter to non-mana ability stack items.
//
// Implementation:
//   - OnResolve: scan gs.Stack top-down for an item with Kind ==
//     "activated" or Kind == "triggered" that does NOT belong to
//     this Stifle resolver and is not a mana ability. Mark Countered.
//   - The engine's mana ability execution path resolves IMMEDIATELY
//     without going through the stack (CR §605.1b), so any item we
//     see on the stack is by definition NOT a mana ability — no
//     additional filtering needed.
//   - Opponent-only is NOT required (Stifle can target your own
//     ability), but the cEDH usage pattern is always opponent's
//     activation. The findCounterableSpell helper takes a filter,
//     but here we need to scan ABILITIES not SPELLS — implement a
//     bespoke scan.
func registerStifle(r *Registry) {
	r.OnResolve("Stifle", stifleResolve)
	// Trickbind shares the same text plus "Split second"; the
	// split-second timing is unmodeled (engine doesn't gate response
	// windows on this keyword yet), so the counter-ability effect is
	// the same.
	r.OnResolve("Trickbind", stifleResolve)
}

func stifleResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	slug := "stifle"
	if item != nil && item.Card != nil && item.Card.DisplayName() == "Trickbind" {
		slug = "trickbind"
	}
	if gs == nil || item == nil {
		return
	}
	// Scan top-down for the most recently pushed activated/triggered
	// ability (excluding our own Stifle stack item).
	target := findCounterableAbility(gs, item)
	if target == nil {
		emitFail(gs, slug, item.Card.DisplayName(), "no_ability_on_stack", nil)
		return
	}
	target.Countered = true
	emitCounter(gs, slug, item.Card.DisplayName(), item.Controller, target)
}

func findCounterableAbility(gs *gameengine.GameState, self *gameengine.StackItem) *gameengine.StackItem {
	if gs == nil {
		return nil
	}
	for i := len(gs.Stack) - 1; i >= 0; i-- {
		si := gs.Stack[i]
		if si == nil || si == self || si.Countered {
			continue
		}
		if si.Kind != "activated" && si.Kind != "triggered" {
			continue
		}
		return si
	}
	return nil
}
