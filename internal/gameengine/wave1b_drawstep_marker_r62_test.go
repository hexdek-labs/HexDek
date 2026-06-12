package gameengine

import "testing"

// r62 — the CR 614.6 first-draw-step marker is consumed at the drawOne
// chokepoint (where card_drawn fires) and surfaced as
// ctx["is_draw_step_draw"]. Pre-r62 it was consumed in
// FireDrawTriggerObservers — a path turn-step draws never reach — so it
// leaked onto the seat's NEXT effect draw and suppressed the wrong
// trigger.
func TestWave1b_DrawStepMarker_ConsumedAtDrawOne(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Library = append(gs.Seats[0].Library,
		&Card{Name: "Top", Owner: 0, Types: []string{"sorcery"}},
		&Card{Name: "Next", Owner: 0, Types: []string{"sorcery"}})

	var captured []map[string]interface{}
	prev := TriggerHook
	TriggerHook = func(_ *GameState, event string, ctx map[string]interface{}) {
		if event == "card_drawn" {
			captured = append(captured, ctx)
		}
	}
	defer func() { TriggerHook = prev }()

	// Draw-step draw: marker set by the turn runner.
	gs.Flags["_suppress_first_draw_trigger_seat"] = 0 + 1
	gs.drawOne(0)
	if len(captured) != 1 {
		t.Fatalf("expected 1 card_drawn fire, got %d", len(captured))
	}
	if b, _ := captured[0]["is_draw_step_draw"].(bool); !b {
		t.Fatal("draw-step draw not marked is_draw_step_draw=true")
	}
	if _, leaked := gs.Flags["_suppress_first_draw_trigger_seat"]; leaked {
		t.Fatal("marker not consumed at drawOne — would leak onto the next effect draw (the pre-r62 bug)")
	}

	// Subsequent effect draw: unmarked.
	gs.drawOne(0)
	if len(captured) != 2 {
		t.Fatalf("expected 2 card_drawn fires, got %d", len(captured))
	}
	if b, _ := captured[1]["is_draw_step_draw"].(bool); b {
		t.Fatal("effect draw wrongly marked as draw-step draw")
	}
}
