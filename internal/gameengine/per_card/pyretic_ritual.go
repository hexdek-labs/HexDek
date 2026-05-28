package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPyreticRitual wires Pyretic Ritual.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Pyretic%20Ritual):
//
//	Add {R}{R}{R}.
//
// {1}{R} Instant. The red Dark Ritual analog — 2 mana in for 3 mana
// out (net +1). Storm-pile mainstay alongside Desperate Ritual /
// Rite of Flame / Seething Song; chains into Grapeshot for the
// canonical storm kill. Identical body to Desperate Ritual modulo
// the Arcane subtype clause (which Pyretic doesn't carry).
//
// Implementation:
//   - OnResolve. ManaPool += 3, log add_mana with reason="pyretic_ritual".
//     Same shape as darkRitualResolve / cabalRitualResolve.
//
// Note: the +3 represents 3 generic mana in the test-fixture mana pool
// abstraction. The engine's pool doesn't track per-color stamps at
// this layer; downstream color-affordance checks fold this into the
// pip-spend pipeline.
func registerPyreticRitual(r *Registry) {
	r.OnResolve("Pyretic Ritual", pyreticRitualResolve)
}

func pyreticRitualResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "pyretic_ritual"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	s.ManaPool += 3
	gameengine.SyncManaAfterAdd(s, 3)
	gs.LogEvent(gameengine.Event{
		Kind:   "add_mana",
		Seat:   seat,
		Target: seat,
		Source: "Pyretic Ritual",
		Amount: 3,
		Details: map[string]interface{}{
			"reason": "pyretic_ritual",
			"pool":   "RRR",
		},
	})
	emit(gs, slug, "Pyretic Ritual", map[string]interface{}{
		"seat":     seat,
		"added":    3,
		"new_pool": s.ManaPool,
	})
}
