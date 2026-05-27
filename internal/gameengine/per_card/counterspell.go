package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCounterspell wires Counterspell.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Counterspell):
//
//	Counter target spell.
//
// {U}{U} instant. The OG counter. Counters any spell, no restrictions.
// Pattern mirrors registerForceOfWill in commander_staples.go — both
// counter any spell with no filter.
func registerCounterspell(r *Registry) {
	r.OnResolve("Counterspell", counterspellResolve)
}

func counterspellResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "counterspell"
	if gs == nil || item == nil {
		return
	}
	target := findCounterableSpell(gs, item.Controller, nil)
	if target == nil {
		emitFail(gs, slug, "Counterspell", "no_spell_on_stack", nil)
		return
	}
	target.Countered = true
	emitCounter(gs, slug, "Counterspell", item.Controller, target)
}
