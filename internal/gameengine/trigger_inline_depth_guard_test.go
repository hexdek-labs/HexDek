package gameengine

import "testing"

// TestPerCardInlineResolveDepthGuard pins the recursion-depth guard in
// PushPerCardTrigger. A handler that re-entrantly pushes another per-card
// trigger from inside its own resolution models the sba704_5y crash
// family: StateBasedActions → destroy → FireCardTrigger →
// PushPerCardTrigger → ResolveStackTop → handler → (re-grant) →
// StateBasedActions → … Each cycle nests a fresh Go frame, so without a
// depth bound the process dies of stack overflow (fatal, not
// recover()-able) long before maxTriggerFiresPerTurn trips.
//
// The guard must end the game as a draw at maxPerCardInlineResolveDepth —
// a single degenerate game must never crash the server.
func TestPerCardInlineResolveDepthGuard(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"

	card := &Card{
		Name: "Synthetic Role Loop", Types: []string{"enchantment"},
		CMC: 1, Owner: 0,
	}
	perm := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	fires := 0
	var loop func(gs *GameState, p *Permanent, ctx map[string]interface{})
	loop = func(gs *GameState, p *Permanent, ctx map[string]interface{}) {
		fires++
		if fires > 10*maxPerCardInlineResolveDepth {
			t.Fatalf("re-entrant loop ran %d times without hitting the depth guard", fires)
		}
		PushPerCardTrigger(gs, p, loop, nil)
	}

	// Without the guard this recurses until the goroutine stack
	// overflows (or, two orders of magnitude later, trigger_loop_cap).
	PushPerCardTrigger(gs, perm, loop, nil)

	if gs.Flags["ended"] != 1 {
		t.Fatal("game did not end after depth guard should have tripped")
	}
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		if !s.Lost {
			t.Errorf("seat %d not marked Lost; depth-cap abort must draw the game", i)
		}
	}
	if len(gs.Stack) != 0 {
		t.Errorf("stack not cleared on depth-cap abort: %d items remain", len(gs.Stack))
	}

	// It must be the DEPTH guard that fired, not the per-turn COUNT cap:
	// the count cap (1000/turn) only trips after enough nested cycles to
	// overflow the real production stack.
	if gs.Flags["_trigger_fires_this_turn"] > maxPerCardInlineResolveDepth+5 {
		t.Errorf("trigger count reached %d — count cap territory; depth guard did not bound the nesting",
			gs.Flags["_trigger_fires_this_turn"])
	}
	sawAnomaly := false
	sawDepthDraw := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "loop_anomaly" {
			if r, _ := ev.Details["reason"].(string); r == "percard_inline_depth_cap" {
				sawAnomaly = true
			}
		}
		if ev.Kind == "game_draw" {
			if r, _ := ev.Details["reason"].(string); r == "percard_inline_depth_cap" {
				sawDepthDraw = true
			}
		}
	}
	if !sawAnomaly {
		t.Error("no loop_anomaly event with reason=percard_inline_depth_cap logged")
	}
	if !sawDepthDraw {
		t.Error("no game_draw event with reason=percard_inline_depth_cap logged")
	}

	// The defer-decrement chain must unwind cleanly so a (hypothetical)
	// later trigger on a fresh game state starts at depth 0.
	if d := gs.Flags["_percard_inline_depth"]; d != 0 {
		t.Errorf("_percard_inline_depth = %d after unwind; want 0", d)
	}
}

// TestPerCardInlineDepthGuardDoesNotAffectSequentialTriggers verifies the
// guard counts NESTED frames only: many sequential (sibling) per-card
// triggers at the same depth — the normal fireTrigger handler-loop shape —
// must all resolve without tripping the cap.
func TestPerCardInlineDepthGuardDoesNotAffectSequentialTriggers(t *testing.T) {
	gs := newTestGameState(4)
	gs.Active = 0
	gs.Phase = "precombat_main"

	card := &Card{
		Name: "Sequential Trigger Source", Types: []string{"creature"},
		CMC: 2, Owner: 0, BasePower: 1, BaseToughness: 1,
	}
	perm := &Permanent{
		Card: card, Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	fired := 0
	handler := func(gs *GameState, p *Permanent, ctx map[string]interface{}) {
		fired++
	}
	// 4x the depth cap, sequentially — well within the per-turn count cap.
	n := 4 * maxPerCardInlineResolveDepth
	for i := 0; i < n; i++ {
		PushPerCardTrigger(gs, perm, handler, nil)
	}

	if fired != n {
		t.Errorf("fired %d of %d sequential triggers; depth guard must not throttle siblings", fired, n)
	}
	if gs.Flags["ended"] == 1 {
		t.Error("game ended on sequential triggers; depth guard miscounted siblings as nesting")
	}
	if d := gs.Flags["_percard_inline_depth"]; d != 0 {
		t.Errorf("_percard_inline_depth = %d after sequential triggers; want 0", d)
	}
}
