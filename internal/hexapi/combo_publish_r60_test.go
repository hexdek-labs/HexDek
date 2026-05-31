package hexapi

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// R60 spectator UX: regressions for the publishNewCombos helper
// that bridges engine `infinite_cycle` events into the
// `game.combo_fired` EventBus stream. Drives the frontend toast
// notification on the spectator screen.

// captureSubscriber drains an EventBus subscription into a slice
// of Events so tests can assert on what got published.
func captureSubscriber(t *testing.T, bus *EventBus) (drain func() []Event) {
	t.Helper()
	sub := bus.Subscribe(64)
	t.Cleanup(func() { bus.Unsubscribe(sub) })
	return func() []Event {
		out := []Event{}
		// Drain non-blocking — Publish is synchronous from the test
		// caller's perspective so anything pending is already buffered.
		for {
			select {
			case ev, ok := <-sub.Ch:
				if !ok {
					return out
				}
				out = append(out, ev)
			default:
				return out
			}
		}
	}
}

// withInfiniteCycle constructs an event log entry as the engine
// emits it. Matches the shape DetectEngineCycles consumes (kind
// "infinite_cycle" + Details with cycle_length / participating_*).
func withInfiniteCycle(kind string, details map[string]any, seat int) gameengine.Event {
	return gameengine.Event{
		Kind:    kind,
		Seat:    seat,
		Details: details,
	}
}

func TestPublishNewCombos_NoBus_NoOp(t *testing.T) {
	sm := &Showmatch{}
	gs := &gameengine.GameState{
		EventLog: []gameengine.Event{
			withInfiniteCycle("infinite_cycle", map[string]any{"cycle_length": 3}, 0),
		},
	}
	// Should not panic / not publish anywhere.
	sm.publishNewCombos(gs, 1, []string{"A"})
}

func TestPublishNewCombos_NilState_NoOp(t *testing.T) {
	sm := &Showmatch{Events: NewEventBus()}
	sm.publishNewCombos(nil, 1, []string{"A"})
	// Reaching here without panic is the assertion.
}

func TestPublishNewCombos_NoCyclesNoPublish(t *testing.T) {
	bus := NewEventBus()
	drain := captureSubscriber(t, bus)
	sm := &Showmatch{Events: bus}
	gs := &gameengine.GameState{
		EventLog: []gameengine.Event{
			{Kind: "card_drawn", Seat: 0},
			{Kind: "spell_cast", Seat: 1},
		},
	}
	sm.publishNewCombos(gs, 1, []string{"A", "B"})
	got := drain()
	if len(got) != 0 {
		t.Errorf("expected 0 published events, got %d: %+v", len(got), got)
	}
}

func TestPublishNewCombos_SingleCycle_PublishesOne(t *testing.T) {
	bus := NewEventBus()
	drain := captureSubscriber(t, bus)
	sm := &Showmatch{Events: bus}
	gs := &gameengine.GameState{
		Turn: 7,
		EventLog: []gameengine.Event{
			{Kind: "card_drawn", Seat: 0},
			withInfiniteCycle("infinite_cycle", map[string]any{
				"cycle_length":        3,
				"participating_names": []string{"Heliod, Sun-Crowned", "Walking Ballista"},
				"participating_iids":  []string{"iid-1", "iid-2"},
				"detected_by":         "engine_no_op_loop",
			}, 2),
		},
	}
	sm.publishNewCombos(gs, 42, []string{"A", "B", "Heliod, Sun-Crowned", "D"})
	got := drain()
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	ev := got[0]
	if ev.Type != EventComboFired {
		t.Errorf("type = %q, want %q", ev.Type, EventComboFired)
	}
	data, ok := ev.Data.(map[string]any)
	if !ok {
		t.Fatalf("Data not map: %T", ev.Data)
	}
	if data["game_id"] != 42 {
		t.Errorf("game_id = %v, want 42", data["game_id"])
	}
	if data["turn"] != 7 {
		t.Errorf("turn = %v, want 7", data["turn"])
	}
	if data["seat"] != 2 {
		t.Errorf("seat = %v, want 2", data["seat"])
	}
	if data["controller_commander"] != "Heliod, Sun-Crowned" {
		t.Errorf("controller_commander = %v, want Heliod", data["controller_commander"])
	}
	if data["cycle_length"] != 3 {
		t.Errorf("cycle_length = %v, want 3", data["cycle_length"])
	}
	parts, ok := data["participating_cards"].([]string)
	if !ok || len(parts) != 2 || parts[0] != "Heliod, Sun-Crowned" || parts[1] != "Walking Ballista" {
		t.Errorf("participating_cards = %v, want [Heliod, Walking Ballista]", data["participating_cards"])
	}
	if data["detected_by"] != "engine_no_op_loop" {
		t.Errorf("detected_by = %v, want engine_no_op_loop", data["detected_by"])
	}
}

func TestPublishNewCombos_RepeatTickDedupes(t *testing.T) {
	bus := NewEventBus()
	drain := captureSubscriber(t, bus)
	sm := &Showmatch{Events: bus}
	gs := &gameengine.GameState{
		Turn: 3,
		EventLog: []gameengine.Event{
			withInfiniteCycle("infinite_cycle", map[string]any{"cycle_length": 2}, 0),
		},
	}
	// First call publishes; second call (same log content) must NOT
	// re-publish — the cursor is now past the cycle event.
	sm.publishNewCombos(gs, 1, []string{"A"})
	sm.publishNewCombos(gs, 1, []string{"A"})
	got := drain()
	if len(got) != 1 {
		t.Errorf("expected exactly 1 event across two ticks, got %d", len(got))
	}
}

func TestPublishNewCombos_NewCycleAfterCursor(t *testing.T) {
	bus := NewEventBus()
	drain := captureSubscriber(t, bus)
	sm := &Showmatch{Events: bus}
	gs := &gameengine.GameState{
		EventLog: []gameengine.Event{
			withInfiniteCycle("infinite_cycle", map[string]any{"cycle_length": 2}, 0),
		},
	}
	sm.publishNewCombos(gs, 1, []string{"A"})
	// Engine emits a SECOND cycle later in the same game.
	gs.EventLog = append(gs.EventLog,
		gameengine.Event{Kind: "spell_cast", Seat: 1},
		withInfiniteCycle("infinite_cycle", map[string]any{"cycle_length": 4}, 1),
	)
	sm.publishNewCombos(gs, 1, []string{"A", "B"})
	got := drain()
	if len(got) != 2 {
		t.Fatalf("expected 2 events across two ticks, got %d", len(got))
	}
	// Second event must reflect the SECOND cycle's seat/cycle_length.
	data := got[1].Data.(map[string]any)
	if data["seat"] != 1 || data["cycle_length"] != 4 {
		t.Errorf("second event payload wrong: %+v", data)
	}
}

func TestPublishNewCombos_PerGameCursorIsolation(t *testing.T) {
	bus := NewEventBus()
	drain := captureSubscriber(t, bus)
	sm := &Showmatch{Events: bus}
	gsA := &gameengine.GameState{
		EventLog: []gameengine.Event{
			withInfiniteCycle("infinite_cycle", map[string]any{"cycle_length": 2}, 0),
		},
	}
	gsB := &gameengine.GameState{
		EventLog: []gameengine.Event{
			withInfiniteCycle("infinite_cycle", map[string]any{"cycle_length": 5}, 0),
		},
	}
	// Game 1 + Game 2 are independent — cursor doesn't bleed.
	sm.publishNewCombos(gsA, 1, []string{"A"})
	sm.publishNewCombos(gsB, 2, []string{"X"})
	got := drain()
	if len(got) != 2 {
		t.Fatalf("expected 2 events (one per game), got %d", len(got))
	}
	dataA := got[0].Data.(map[string]any)
	dataB := got[1].Data.(map[string]any)
	if dataA["game_id"] != 1 || dataB["game_id"] != 2 {
		t.Errorf("game_id isolation wrong: %v / %v", dataA["game_id"], dataB["game_id"])
	}
	if dataA["cycle_length"] != 2 || dataB["cycle_length"] != 5 {
		t.Errorf("cycle_length isolation wrong: %v / %v", dataA["cycle_length"], dataB["cycle_length"])
	}
}

func TestPublishNewCombos_FallsBackToAmountWhenDetailsMissing(t *testing.T) {
	bus := NewEventBus()
	drain := captureSubscriber(t, bus)
	sm := &Showmatch{Events: bus}
	gs := &gameengine.GameState{
		EventLog: []gameengine.Event{
			// No Details map — engine on legacy event-log shape.
			{Kind: "infinite_cycle", Seat: 0, Amount: 4},
		},
	}
	sm.publishNewCombos(gs, 1, []string{"A"})
	got := drain()
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	data := got[0].Data.(map[string]any)
	if data["cycle_length"] != 4 {
		t.Errorf("cycle_length fallback via Amount failed: %v", data["cycle_length"])
	}
}

func TestPublishNewCombos_SeatOutOfRangeSkipsCommander(t *testing.T) {
	bus := NewEventBus()
	drain := captureSubscriber(t, bus)
	sm := &Showmatch{Events: bus}
	gs := &gameengine.GameState{
		EventLog: []gameengine.Event{
			withInfiniteCycle("infinite_cycle", map[string]any{"cycle_length": 2}, 5),
		},
	}
	sm.publishNewCombos(gs, 1, []string{"A", "B"}) // only 2 commanders
	got := drain()
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	data := got[0].Data.(map[string]any)
	if data["seat"] != 5 {
		t.Errorf("seat should be carried even when out of range, got %v", data["seat"])
	}
	if _, has := data["controller_commander"]; has {
		t.Errorf("controller_commander should be absent when seat out of range, got %v", data["controller_commander"])
	}
}
