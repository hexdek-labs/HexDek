package heimdall

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// composition_prior_log_test.go — regressions for the per-game
// effect persistence introduced by PR #422 to feed the
// hexdek-composition-replay debug CLI.

func TestCompositionPriorLog_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	seed := int64(12345)
	rec := CompositionPriorReplayRecord{
		GameSeed: GameSeed{RNGSeed: seed, Winner: 1, Turns: 30, KillMethod: "combat"},
		Effects: []CompositionPriorEffect{
			{Seat: 0, Archetype: "Mill", Offset: 1.2, Confidence: 0.7, ExpectedWinrate: 0.65, MuDeltaVsBaseline: -0.8},
			{Seat: 1, Archetype: "Voltron", Offset: -0.6, Confidence: 0.7, ExpectedWinrate: 0.05, MuDeltaVsBaseline: 1.3},
			{Seat: 2, Archetype: "Aggro", Offset: 0.1, Confidence: 0.5, ExpectedWinrate: 0.15, MuDeltaVsBaseline: -0.1},
			{Seat: 3, Archetype: "Combo", Offset: 0.0, Confidence: 0.4, ExpectedWinrate: 0.15, MuDeltaVsBaseline: 0.0},
		},
	}
	writeCompositionPriorRecord(dir, rec)

	got, err := LoadCompositionPriorRecord(dir, seed)
	if err != nil {
		t.Fatalf("load round-trip: %v", err)
	}
	if got.GameSeed.RNGSeed != seed {
		t.Errorf("RNGSeed = %d, want %d", got.GameSeed.RNGSeed, seed)
	}
	if got.GameSeed.Winner != 1 {
		t.Errorf("Winner = %d, want 1", got.GameSeed.Winner)
	}
	if len(got.Effects) != 4 {
		t.Fatalf("Effects len = %d, want 4", len(got.Effects))
	}
	for i, want := range rec.Effects {
		gotEff := got.Effects[i]
		if gotEff.Seat != want.Seat || gotEff.Archetype != want.Archetype {
			t.Errorf("effect[%d] seat/archetype mismatch: got %+v want %+v", i, gotEff, want)
		}
		if math.Abs(gotEff.Offset-want.Offset) > 1e-9 ||
			math.Abs(gotEff.Confidence-want.Confidence) > 1e-9 ||
			math.Abs(gotEff.ExpectedWinrate-want.ExpectedWinrate) > 1e-9 ||
			math.Abs(gotEff.MuDeltaVsBaseline-want.MuDeltaVsBaseline) > 1e-9 {
			t.Errorf("effect[%d] numeric mismatch: got %+v want %+v", i, gotEff, want)
		}
	}
}

func TestCompositionPriorLog_FileNamingByRNGSeed(t *testing.T) {
	dir := t.TempDir()
	seed := int64(99887766)
	writeCompositionPriorRecord(dir, CompositionPriorReplayRecord{
		GameSeed: GameSeed{RNGSeed: seed, Winner: 0},
		Effects:  []CompositionPriorEffect{{Seat: 0, Archetype: "Mill"}},
	})
	wantPath := filepath.Join(CompositionPriorLogDir(dir), "99887766.json")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("expected file at %s; got error: %v", wantPath, err)
	}
}

func TestCompositionPriorLog_EmptyEffectsNotWritten(t *testing.T) {
	dir := t.TempDir()
	writeCompositionPriorRecord(dir, CompositionPriorReplayRecord{
		GameSeed: GameSeed{RNGSeed: 5},
		Effects:  nil, // empty
	})
	entries, _ := os.ReadDir(CompositionPriorLogDir(dir))
	if len(entries) != 0 {
		t.Errorf("empty-effects record should not create any files; found %d entries", len(entries))
	}
}

func TestCompositionPriorLog_LoadMissingReturnsNotExist(t *testing.T) {
	dir := t.TempDir()
	// Don't write anything — load should error with os.ErrNotExist.
	_, err := LoadCompositionPriorRecord(dir, 42)
	if err == nil {
		t.Fatal("load of missing record should error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error should wrap os.ErrNotExist; got %v", err)
	}
}

func TestCompositionPriorLog_OverwriteOnRewrite(t *testing.T) {
	dir := t.TempDir()
	seed := int64(7)
	first := CompositionPriorReplayRecord{
		GameSeed: GameSeed{RNGSeed: seed, Winner: 0},
		Effects:  []CompositionPriorEffect{{Seat: 0, Archetype: "First", Offset: 1.0}},
	}
	second := CompositionPriorReplayRecord{
		GameSeed: GameSeed{RNGSeed: seed, Winner: 2},
		Effects:  []CompositionPriorEffect{{Seat: 0, Archetype: "Second", Offset: 9.9}},
	}
	writeCompositionPriorRecord(dir, first)
	writeCompositionPriorRecord(dir, second)
	got, err := LoadCompositionPriorRecord(dir, seed)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.GameSeed.Winner != 2 || got.Effects[0].Archetype != "Second" {
		t.Errorf("expected second write to overwrite first; got %+v", got)
	}
}

// -----------------------------------------------------------------------------
// Observer integration: RecordObservation writes when effects present
// -----------------------------------------------------------------------------

func TestObserver_RecordObservation_PersistsEffects(t *testing.T) {
	dir := t.TempDir()
	o := New(dir, nil, nil, nil)
	seed := int64(42)
	o.RecordObservation(Observation{
		Seed: GameSeed{RNGSeed: seed, Winner: 1},
		CompositionPriorEffects: []CompositionPriorEffect{
			{Seat: 0, Archetype: "Mill", Offset: 1.5, Confidence: 0.8},
			{Seat: 1, Archetype: "Voltron", Offset: -0.3, Confidence: 0.8, MuDeltaVsBaseline: 2.0},
		},
	})

	got, err := LoadCompositionPriorRecord(dir, seed)
	if err != nil {
		t.Fatalf("expected effect file to exist after RecordObservation: %v", err)
	}
	if len(got.Effects) != 2 {
		t.Errorf("Effects len = %d, want 2", len(got.Effects))
	}
}

func TestObserver_RecordObservation_NoEffectsNoFile(t *testing.T) {
	dir := t.TempDir()
	o := New(dir, nil, nil, nil)
	o.RecordObservation(Observation{
		Seed: GameSeed{RNGSeed: 100, Winner: 0},
		// No CompositionPriorEffects
		ParserGaps: []string{"some_card"}, // other fields populated
	})
	entries, _ := os.ReadDir(CompositionPriorLogDir(dir))
	if len(entries) != 0 {
		t.Errorf("observation without effects should not write any file; found %d", len(entries))
	}
}
