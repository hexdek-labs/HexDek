package huginn

import (
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
)

// TestPersistRawCycles_AppendsAndPreservesExisting confirms the
// Huginn cycle-ingestion path: a tournament-run analyses slice with
// CycleObservations is persisted to raw_cycles.json; subsequent calls
// append rather than overwrite.
func TestPersistRawCycles_AppendsAndPreservesExisting(t *testing.T) {
	dir := t.TempDir()

	// First batch — 2 cycles, 1 game.
	ga1 := &analytics.GameAnalysis{
		CycleObservations: []analytics.CycleObservation{
			{
				CycleLength:        2,
				ParticipatingIIDs:  []string{"h0OGVR700008", "h0OGVB400012"},
				ParticipatingCards: []string{"Worldgorger Dragon", "Animate Dead"},
				TurnWindow:         47,
				DetectedBy:         "engine_no_op_loop",
				GameID:             "game-411",
			},
			{
				CycleLength:        3,
				ParticipatingIIDs:  []string{"h0OGVR100007", "h0OGVR200015", "h0OGVR300022"},
				ParticipatingCards: []string{"Heliod, Sun-Crowned", "Walking Ballista", "Heliod trigger"},
				TurnWindow:         12,
				DetectedBy:         "engine_cr_727",
				GameID:             "game-411",
			},
		},
	}
	commanders1 := []string{"Eruth, Tormented Prophet", "Atraxa", "Krenko", "Lathril"}
	if err := PersistRawCycles(dir, []*analytics.GameAnalysis{ga1}, commanders1); err != nil {
		t.Fatalf("PersistRawCycles (1st): %v", err)
	}

	got, err := ReadRawCycles(dir)
	if err != nil {
		t.Fatalf("ReadRawCycles: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("after first persist: got %d cycles, want 2", len(got))
	}
	if got[0].DetectedBy != "engine_no_op_loop" {
		t.Errorf("[0].DetectedBy: got %q", got[0].DetectedBy)
	}
	if got[0].CycleLength != 2 || len(got[0].ParticipatingIIDs) != 2 {
		t.Errorf("[0].CycleLength/IIDs: got len=%d iids=%v", got[0].CycleLength, got[0].ParticipatingIIDs)
	}
	if got[0].DeckNames[0] != "Eruth, Tormented Prophet" {
		t.Errorf("DeckNames: got %v", got[0].DeckNames)
	}
	if got[0].Timestamp == "" {
		t.Errorf("Timestamp must be populated")
	}

	// Second batch — append 1 cycle, separate game.
	ga2 := &analytics.GameAnalysis{
		CycleObservations: []analytics.CycleObservation{
			{
				CycleLength:        2,
				ParticipatingIIDs:  []string{"h1OGVU200055", "h1OGVU300077"},
				ParticipatingCards: []string{"Kiki-Jiki, Mirror Breaker", "Felidar Guardian"},
				TurnWindow:         9,
				DetectedBy:         "graph_walker",
				GameID:             "game-2762",
			},
		},
	}
	commanders2 := []string{"Bruvac", "Codie", "Phelia", "Slimefoot"}
	if err := PersistRawCycles(dir, []*analytics.GameAnalysis{ga2}, commanders2); err != nil {
		t.Fatalf("PersistRawCycles (2nd): %v", err)
	}

	got, err = ReadRawCycles(dir)
	if err != nil {
		t.Fatalf("ReadRawCycles (after append): %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("after append: got %d cycles, want 3 (append-only)", len(got))
	}
	if got[2].DetectedBy != "graph_walker" {
		t.Errorf("[2].DetectedBy: got %q, want graph_walker", got[2].DetectedBy)
	}
	if got[2].DeckNames[0] != "Bruvac" {
		t.Errorf("[2].DeckNames: got %v", got[2].DeckNames)
	}
}

// TestPersistRawCycles_NoOpOnEmptyAnalyses guards against
// accidentally writing a 0-entry file (the function should short-
// circuit when there's nothing to persist).
func TestPersistRawCycles_NoOpOnEmptyAnalyses(t *testing.T) {
	dir := t.TempDir()
	if err := PersistRawCycles(dir, nil, nil); err != nil {
		t.Fatalf("nil analyses: %v", err)
	}
	if err := PersistRawCycles(dir, []*analytics.GameAnalysis{{}}, nil); err != nil {
		t.Fatalf("empty analyses: %v", err)
	}
	// File should not exist.
	got, err := ReadRawCycles(dir)
	if err != nil {
		t.Fatalf("ReadRawCycles empty: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty input should not create file; got %d entries", len(got))
	}
	_ = filepath.Join(dir, rawCyclesFile)
}

// TestReadRawCycles_MissingFile returns empty slice (not error) so
// first-run consumers don't panic.
func TestReadRawCycles_MissingFile(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadRawCycles(dir)
	if err != nil {
		t.Fatalf("missing file should not error; got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("missing file should return empty slice; got %d", len(got))
	}
}
