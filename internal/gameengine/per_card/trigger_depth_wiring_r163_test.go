package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r163 — the unit tests in internal/gameengine call NoteTriggerDepth
// directly, so they cannot tell whether fireTrigger actually calls it.
// This drives the REAL dispatcher under the REAL production event
// policy and asserts the curve moved. Without it, a control that
// deletes the production call site stays green and tests nothing.
func TestR163_FireTriggerFeedsTheDepthCurve(t *testing.T) {
	gameengine.ResetTriggerDepthForTest()
	t.Cleanup(gameengine.ResetTriggerDepthForTest)

	gs := newTestGS(2)
	gs.EventPolicy = gameengine.EventLogNone // what the grinder runs

	const name = "ZZ R163 Recursor"
	const evt = "creature_dies"

	r := Global()
	r.OnTrigger(name, evt, func(g *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fireTrigger(g, evt, map[string]interface{}{"seat": 0})
	})
	t.Cleanup(func() {
		r.mu.Lock()
		delete(r.onTrigger, normalizeName(name))
		r.mu.Unlock()
	})

	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &gameengine.Permanent{
		Card:       &gameengine.Card{Name: name, Types: []string{"creature"}},
		Controller: 0,
		Owner:      0,
	})

	fireTrigger(gs, evt, map[string]interface{}{"seat": 0})

	h := gameengine.TriggerDepthHistogram()
	if len(h) == 0 {
		t.Fatalf("fireTrigger recorded no depth at all — the production " +
			"call site is not wired")
	}
	// A recursive handler must climb, and the fireTrigger cap means the
	// deepest level reachable is exactly 9.
	if h[1] != 1 {
		t.Fatalf("depth 1 counted %d, want 1 (%v)", h[1], h)
	}
	if h[9] != 1 {
		t.Fatalf("an unbounded recursion must reach depth 9 exactly once (%v)", h)
	}
	if _, ok := h[10]; ok {
		t.Fatalf("depth 10 recorded; fireTrigger returns at >8 without "+
			"dispatching, so nothing can nest deeper (%v)", h)
	}
	for d := 1; d <= 9; d++ {
		if h[d] != 1 {
			t.Fatalf("curve must pass through every level once, depth %d = %d (%v)",
				d, h[d], h)
		}
	}
}
