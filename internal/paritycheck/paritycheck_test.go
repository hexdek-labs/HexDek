package paritycheck

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiff_IdenticalReplays verifies that identical input streams
// produce zero divergences.
func TestDiff_IdenticalReplays(t *testing.T) {
	events := []Event{
		{Seq: 0, Turn: 1, Kind: "turn_start", Seat: 0},
		{Seq: 1, Turn: 1, Kind: "draw", Seat: 0, Amount: 1},
		{Seq: 2, Turn: 1, Kind: "play_land", Seat: 0, Source: "Plains"},
		{Seq: 3, Turn: 1, Kind: "game_end", Seat: 0},
	}
	outcome := Outcome{Winner: 0, Turns: 1, EndReason: "test"}
	go_ := &ReplayData{GameIdx: 0, Events: events, Outcome: outcome}
	py := &ReplayData{GameIdx: 0, Events: events, Outcome: outcome}
	divs := Diff(0, go_, py)
	if len(divs) != 0 {
		t.Errorf("expected 0 divergences on identical streams, got %d: %+v",
			len(divs), divs)
	}
}

// TestDiff_OutcomeDivergence catches when the winners disagree.
func TestDiff_OutcomeDivergence(t *testing.T) {
	events := []Event{{Seq: 0, Kind: "turn_start"}}
	go_ := &ReplayData{Events: events, Outcome: Outcome{Winner: 0, Turns: 10}}
	py := &ReplayData{Events: events, Outcome: Outcome{Winner: 1, Turns: 10}}
	divs := Diff(0, go_, py)
	found := false
	for _, d := range divs {
		if d.Category == "outcome" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected outcome divergence, got %+v", divs)
	}
}

// TestDiff_EventCountDivergence detects when one side emits more events
// of a given kind than the other.
func TestDiff_EventCountDivergence(t *testing.T) {
	goEvents := []Event{
		{Seq: 0, Kind: "turn_start"},
		{Seq: 1, Kind: "draw"},
		{Seq: 2, Kind: "draw"},
	}
	pyEvents := []Event{
		{Seq: 0, Kind: "turn_start"},
		{Seq: 1, Kind: "draw"},
	}
	outcome := Outcome{Winner: 0, Turns: 1}
	go_ := &ReplayData{Events: goEvents, Outcome: outcome}
	py := &ReplayData{Events: pyEvents, Outcome: outcome}
	divs := Diff(0, go_, py)
	found := false
	for _, d := range divs {
		if d.Category == "event_count" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected event_count divergence, got %+v", divs)
	}
}

// TestNormalizeKind folds known synonyms.
func TestNormalizeKind(t *testing.T) {
	tests := []struct{ in, want string }{
		{"enter_battlefield", "enter_battlefield"},
		{"etb", "enter_battlefield"},
		{"ETBed", "enter_battlefield"},
		{"combat_damage_dealt", "combat_damage"},
		{"Stack Push", "stack_push"},
		{"game_over", "game_end"},
		{"player_lost", "seat_eliminated"},
	}
	for _, tc := range tests {
		got := normalizeKind(tc.in)
		if got != tc.want {
			t.Errorf("normalizeKind(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestWriteMarkdown emits a valid report file.
func TestWriteMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")
	report := &ParityReport{
		Games: 2, OutcomeMatches: 1, EventStreamMatches: 0,
		CategoryCounts: map[string]int{"outcome": 1, "event_count": 3},
		DeckPaths:      []string{"a.txt", "b.txt"},
		NSeats:         2,
		BaseSeed:       42,
		PythonAvailable: true,
	}
	if err := WriteMarkdown(report, path); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("report is empty")
	}
}

// TestParsePythonReplay_OutcomeLine verifies that the outcome line is
// extracted correctly from a JSONL file.
func TestParsePythonReplay_OutcomeLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	content := `{"seq": 0, "type": "turn_start", "seat": 0, "turn": 1, "phase_kind": "beginning"}
{"seq": 1, "type": "draw", "seat": 0, "turn": 1, "amount": 1}
{"_outcome": {"winner": 2, "turns": 19, "end_reason": "last_seat_standing", "life_totals": [0, 0, 14, 0], "lost_by_seat": [true, true, false, true]}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	replay, err := parsePythonReplay(path, 0, 42)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(replay.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(replay.Events))
	}
	if replay.Outcome.Winner != 2 {
		t.Errorf("expected winner=2, got %d", replay.Outcome.Winner)
	}
	if replay.Outcome.Turns != 19 {
		t.Errorf("expected turns=19, got %d", replay.Outcome.Turns)
	}
	if replay.Events[0].Kind != "turn_start" {
		t.Errorf("expected first kind=turn_start, got %q", replay.Events[0].Kind)
	}
}
