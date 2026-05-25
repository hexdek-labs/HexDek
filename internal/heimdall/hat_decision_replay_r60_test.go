package heimdall

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// hat_decision_replay_r60_test.go — R60 audit. The hat already emits
// per-decision events to gs.EventLog via emitDecisionEvent (Kind:
// "hat_decision_*"), but those events were never surfaced in
// GameSummary — a spectator asking "why did the hat cast this?" had
// no answer without scraping the raw event log. The new
// ExtractHatDecisions + GameSummary.HatDecisions surface projects
// every hat_decision_* event into a typed row keyed on turn / seat /
// kind, with the per-event Details preserved for the UI layer.

func TestExtractHatDecisions_ProjectsAllHatEvents(t *testing.T) {
	gs := &gameengine.GameState{Turn: 5, EventLog: []gameengine.Event{
		{Kind: "hat_decision_cast", Seat: 0, Amount: 5,
			Details: map[string]interface{}{
				"card":      "Sol Ring",
				"ucb":       0.42,
				"archetype": "midrange",
				"turn":      5,
			}},
		{Kind: "hat_decision_attack_target", Seat: 0, Amount: 5,
			Details: map[string]interface{}{
				"attacker":     "Goblin Guide",
				"target_seat":  1,
				"archetype":    "aggro",
				"turn":         5,
			}},
		// Non-hat event — must be skipped.
		{Kind: "cast", Seat: 1, Source: "Lightning Bolt"},
	}}

	got := ExtractHatDecisions(gs)
	if len(got) != 2 {
		t.Fatalf("expected 2 hat decisions, got %d", len(got))
	}
	if got[0].Kind != "cast" {
		t.Errorf("first row should be cast (prefix stripped); got %q", got[0].Kind)
	}
	if got[0].Archetype != "midrange" {
		t.Errorf("archetype should be lifted to its own field; got %q", got[0].Archetype)
	}
	if _, ok := got[0].Details["archetype"]; ok {
		t.Errorf("archetype should NOT remain in Details after lift")
	}
	if _, ok := got[0].Details["turn"]; ok {
		t.Errorf("turn should NOT remain in Details — it's a typed field")
	}
	if got[0].Turn != 5 {
		t.Errorf("turn should be projected from event; got %d", got[0].Turn)
	}
	if got[1].Kind != "attack_target" {
		t.Errorf("second row should be attack_target; got %q", got[1].Kind)
	}
	if got[1].Archetype != "aggro" {
		t.Errorf("second row archetype should be aggro; got %q", got[1].Archetype)
	}
}

func TestExtractHatDecisions_NilGameState(t *testing.T) {
	if got := ExtractHatDecisions(nil); got != nil {
		t.Errorf("nil game state must return nil; got %v", got)
	}
}

func TestExtractHatDecisions_EmptyEventLog(t *testing.T) {
	gs := &gameengine.GameState{Turn: 1}
	if got := ExtractHatDecisions(gs); got != nil {
		t.Errorf("empty event log should return nil; got %v", got)
	}
}

func TestExtractHatDecisions_PreservesNonReservedDetails(t *testing.T) {
	gs := &gameengine.GameState{Turn: 3, EventLog: []gameengine.Event{
		{Kind: "hat_decision_cast", Seat: 2, Amount: 3,
			Details: map[string]interface{}{
				"card":       "Cyclonic Rift",
				"ucb":        0.8,
				"pool_size":  7,
				"tier":       "rollout",
				"archetype":  "control",
				"turn":       3,
			}},
	}}
	got := ExtractHatDecisions(gs)
	if len(got) != 1 {
		t.Fatalf("expected 1 row; got %d", len(got))
	}
	for _, key := range []string{"card", "ucb", "pool_size", "tier"} {
		if _, ok := got[0].Details[key]; !ok {
			t.Errorf("Details should preserve %q; got %+v", key, got[0].Details)
		}
	}
}

func TestExtractHatDecisions_DefensiveCopy(t *testing.T) {
	gs := &gameengine.GameState{Turn: 1, EventLog: []gameengine.Event{
		{Kind: "hat_decision_cast", Seat: 0, Amount: 1,
			Details: map[string]interface{}{
				"card": "Sol Ring",
				"turn": 1,
			}},
	}}
	got := ExtractHatDecisions(gs)
	got[0].Details["card"] = "Mutated"
	// The original event's Details must NOT have been clobbered.
	if gs.EventLog[0].Details["card"] != "Sol Ring" {
		t.Errorf("ExtractHatDecisions must return a defensive copy; original mutated to %v",
			gs.EventLog[0].Details["card"])
	}
}

func TestExtractHatDecisions_RejectsPrefixOnlyKind(t *testing.T) {
	// Defensive: an event whose Kind is exactly "hat_decision_" (no
	// suffix) should not produce a phantom row with empty kind.
	gs := &gameengine.GameState{Turn: 1, EventLog: []gameengine.Event{
		{Kind: "hat_decision_", Seat: 0},
	}}
	got := ExtractHatDecisions(gs)
	for _, d := range got {
		if d.Kind == "" {
			t.Errorf("must not project an event with empty post-prefix kind: %+v", d)
		}
	}
}

func TestBuildGameSummary_IncludesHatDecisions(t *testing.T) {
	obs := Observation{
		Seed: GameSeed{
			RNGSeed: 42,
			Winner:  0,
			Turns:   8,
		},
	}
	gs := &gameengine.GameState{Turn: 8, EventLog: []gameengine.Event{
		{Kind: "hat_decision_cast", Seat: 0, Amount: 4,
			Details: map[string]interface{}{
				"card":      "Demonic Tutor",
				"ucb":       0.91,
				"archetype": "combo",
				"turn":      4,
			}},
		{Kind: "hat_decision_response_counter", Seat: 0, Amount: 6,
			Details: map[string]interface{}{
				"counter":     "Counterspell",
				"target":      "Cyclonic Rift",
				"must":        true,
				"must_reason": "destroy_all",
				"stack_depth": 2,
				"archetype":   "combo",
				"turn":        6,
			}},
	}}

	summary := BuildGameSummary(obs, gs, "")
	if len(summary.HatDecisions) != 2 {
		t.Fatalf("expected 2 decisions on GameSummary; got %d", len(summary.HatDecisions))
	}
	if summary.HatDecisions[0].Kind != "cast" {
		t.Errorf("first decision should be cast; got %q", summary.HatDecisions[0].Kind)
	}
	if summary.HatDecisions[1].Kind != "response_counter" {
		t.Errorf("second decision should be response_counter; got %q", summary.HatDecisions[1].Kind)
	}
	if v, ok := summary.HatDecisions[1].Details["target"]; !ok || !strings.EqualFold(v.(string), "cyclonic rift") {
		t.Errorf("counter decision should preserve target detail; got %+v",
			summary.HatDecisions[1].Details)
	}
}

func TestBuildGameSummary_EmptyHatDecisionsOmittedFromJSON(t *testing.T) {
	// When no hat decisions exist, the field is empty and the
	// json:"omitempty" tag keeps it out of marshaled output — so we
	// assert the slice is nil-or-empty (zero length).
	obs := Observation{Seed: GameSeed{RNGSeed: 1, Winner: 0, Turns: 1}}
	gs := &gameengine.GameState{Turn: 1}
	summary := BuildGameSummary(obs, gs, "")
	if len(summary.HatDecisions) != 0 {
		t.Errorf("no hat decisions → HatDecisions should be empty; got %v",
			summary.HatDecisions)
	}
}
