package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestConvictionEvent_ArchetypeRoundTrip verifies the new Archetype field
// added in R40 round-trips intact through the conviction telemetry ring.
// The post-game correlation pipeline depends on this — if we drop the
// archetype label on ingest, "did win-line-extinct fire more on combo
// decks?" becomes unanswerable without rejoining against the deck index.
func TestConvictionEvent_ArchetypeRoundTrip(t *testing.T) {
	ResetConvictionTelemetry()
	t.Cleanup(ResetConvictionTelemetry)

	TestingPushConvictionEvent(ConvictionEvent{
		Turn:      7,
		Archetype: "combo",
	})
	TestingPushConvictionEvent(ConvictionEvent{
		Turn:      8,
		Archetype: "control",
	})
	// One sample with no archetype to verify omitempty doesn't trip us up
	// (the ring stores the zero value either way, but consumers reading
	// the JSON-encoded form expect the field to be absent).
	TestingPushConvictionEvent(ConvictionEvent{Turn: 9})

	events, _ := SnapshotConvictionEvents(0, 0)
	if len(events) != 3 {
		t.Fatalf("len(events) = %d, want 3", len(events))
	}
	if events[0].Archetype != "combo" {
		t.Errorf("events[0].Archetype = %q, want combo", events[0].Archetype)
	}
	if events[1].Archetype != "control" {
		t.Errorf("events[1].Archetype = %q, want control", events[1].Archetype)
	}
	if events[2].Archetype != "" {
		t.Errorf("events[2].Archetype = %q, want empty", events[2].Archetype)
	}
}

// TestEmitDecisionEvent_StampsArchetypeAndTurn verifies the shared
// emitDecisionEvent helper stamps every persisted hat-decision event with
// the hat's archetype belief and the current turn, even when the caller
// doesn't supply them. This is the contract heimdall relies on — every
// row is self-describing without rejoining the deck index.
func TestEmitDecisionEvent_StampsArchetypeAndTurn(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Turn = 5

	h := NewYggdrasilHat(&StrategyProfile{Archetype: "combo"}, 0)

	before := len(gs.EventLog)
	h.emitDecisionEvent(gs, 0, "mulligan", map[string]interface{}{
		"hand_size":  7,
		"lands":      3,
		"mulliganed": false,
	})
	if len(gs.EventLog) != before+1 {
		t.Fatalf("EventLog grew by %d, want 1", len(gs.EventLog)-before)
	}
	ev := gs.EventLog[len(gs.EventLog)-1]
	if ev.Kind != "hat_decision_mulligan" {
		t.Errorf("Kind = %q, want hat_decision_mulligan", ev.Kind)
	}
	if ev.Seat != 0 {
		t.Errorf("Seat = %d, want 0", ev.Seat)
	}
	if got, _ := ev.Details["archetype"].(string); got != "combo" {
		t.Errorf("Details[archetype] = %q, want combo", got)
	}
	if got, _ := ev.Details["turn"].(int); got != 5 {
		t.Errorf("Details[turn] = %v, want 5", ev.Details["turn"])
	}
	if got, _ := ev.Details["hand_size"].(int); got != 7 {
		t.Errorf("Details[hand_size] = %v, want 7", ev.Details["hand_size"])
	}
}

// TestEmitDecisionEvent_NoStrategyEmitsEmptyArchetype verifies the helper
// is safe when no strategy is loaded — it should still emit the event,
// just with an empty archetype string. (The R37 audit confirmed the
// evaluator falls back to ArchetypeMidrange in that path, but the
// decision log honestly reports "no strategy belief" rather than
// fabricating a label.)
func TestEmitDecisionEvent_NoStrategyEmitsEmptyArchetype(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	h := NewYggdrasilHat(nil, 0)

	h.emitDecisionEvent(gs, 1, "block", map[string]interface{}{"attackers": 2})
	if len(gs.EventLog) == 0 {
		t.Fatal("expected one event")
	}
	ev := gs.EventLog[len(gs.EventLog)-1]
	if ev.Kind != "hat_decision_block" {
		t.Errorf("Kind = %q, want hat_decision_block", ev.Kind)
	}
	if got, ok := ev.Details["archetype"].(string); !ok || got != "" {
		t.Errorf("Details[archetype] = %v (ok=%v), want empty string", ev.Details["archetype"], ok)
	}
}

// TestEmitDecisionEvent_NilGameStateIsNoop verifies the defensive guard:
// passing a nil game state must not panic and must not attempt to write
// anywhere. Some test harnesses pass nil to exercise hat helpers in
// isolation.
func TestEmitDecisionEvent_NilGameStateIsNoop(t *testing.T) {
	// Explicit doesn't-panic assertion via recover() — Phase 2B audit.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("emitDecisionEvent(nil, ...) must not panic; got %v", r)
		}
	}()
	h := NewYggdrasilHat(&StrategyProfile{Archetype: "control"}, 0)
	h.emitDecisionEvent(nil, 0, "mode", map[string]interface{}{"x": 1})
}
