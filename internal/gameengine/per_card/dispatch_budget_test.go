package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r63 liveness firehose regression: a Rebel token storm burned the
// whole 2000/turn per_card dispatch budget with ZERO-handler
// token_created/permanent_etb dispatches (seed 80808 game 672 —
// 6,804 cap fires), after which every REAL per_card trigger in the
// game was silently swallowed for the rest of the turn. The budget now
// counts only dispatches that actually found handlers: zero-handler
// events can't loop through handlers and must cost nothing.
func TestFireTrigger_ZeroHandlerDispatchesDontBurnBudget(t *testing.T) {
	gs := newGame(t, 2)
	// A bystander with NO trigger handlers (storms fire events at a
	// board, not into a vacuum).
	addPerm(gs, 0, "Rebel Bystander", "creature")

	// The storm: 3000 zero-handler dispatches (1.5x the budget).
	for i := 0; i < 3000; i++ {
		gameengine.FireCardTrigger(gs, "token_created", map[string]interface{}{"seat": 0})
	}
	if got := gs.Flags["trigger_total"]; got != 0 {
		t.Fatalf("zero-handler dispatches burned the budget: trigger_total=%d, want 0", got)
	}

	// A REAL handler after the storm must still fire (pre-fix it was
	// silently swallowed for the rest of the turn).
	fired := 0
	Global().OnTrigger("Budget Probe", "budget_probe_event", func(gs *gameengine.GameState, p *gameengine.Permanent, ctx map[string]interface{}) {
		fired++
	})
	t.Cleanup(Reset)
	addPerm(gs, 0, "Budget Probe", "artifact")
	gameengine.FireCardTrigger(gs, "budget_probe_event", nil)
	if fired != 1 {
		t.Fatalf("real handler swallowed after zero-handler storm: fired=%d, want 1", fired)
	}
	if got := gs.Flags["trigger_total"]; got != 1 {
		t.Errorf("handler-hit dispatch must cost exactly 1 budget unit, got %d", got)
	}
}
