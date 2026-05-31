package huginn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
)

// TestComputePredictionOutcomes_NilSafety pins zero-value returns on
// degenerate inputs. Defends against panics in heimdall integration
// when game-analysis data is partial or seat index is malformed.
func TestComputePredictionOutcomes_NilSafety(t *testing.T) {
	preds := []Prediction{{InstanceID: "x-y-1", Cards: []string{"A"}, Confidence: 0.5}}
	ga := &analytics.GameAnalysis{Players: []analytics.PlayerAnalysis{{Seat: 0}}}

	cases := []struct {
		name    string
		ga      *analytics.GameAnalysis
		seatIdx int
		preds   []Prediction
	}{
		{"nil ga", nil, 0, preds},
		{"negative seat", ga, -1, preds},
		{"seat out of range", ga, 5, preds},
		{"empty preds", ga, 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputePredictionOutcomes(tc.ga, tc.seatIdx, tc.preds); got != nil {
				t.Errorf("want nil, got %+v", got)
			}
		})
	}
}

// TestComputePredictionOutcomes_FiredWhenAllCardsCast pins the
// load-bearing semantic: a prediction fires when ALL its cards have
// TurnCast > 0 in the target seat's CardsPlayed. ConfidenceDelta
// gets the positive fired value (+0.05).
func TestComputePredictionOutcomes_FiredWhenAllCardsCast(t *testing.T) {
	ga := &analytics.GameAnalysis{
		Players: []analytics.PlayerAnalysis{{
			Seat: 0,
			CardsPlayed: []analytics.CardPerformance{
				{Name: "Heliod, Sun-Crowned", TurnCast: 4},
				{Name: "Walking Ballista", TurnCast: 5},
				{Name: "Mox Diamond", TurnCast: 1},
			},
		}},
	}
	preds := []Prediction{{
		InstanceID: "deck-heliod-ballista-001",
		Cards:      []string{"Heliod, Sun-Crowned", "Walking Ballista"},
		Confidence: 0.85,
	}}
	out := ComputePredictionOutcomes(ga, 0, preds)
	if len(out) != 1 {
		t.Fatalf("want 1 outcome, got %d", len(out))
	}
	if !out[0].Fired {
		t.Errorf("want Fired=true")
	}
	if out[0].ConfidenceDelta != outcomeFiredDelta {
		t.Errorf("want delta %v, got %v", outcomeFiredDelta, out[0].ConfidenceDelta)
	}
	if out[0].InstanceID != "deck-heliod-ballista-001" {
		t.Errorf("InstanceID mismatch: %v", out[0].InstanceID)
	}
}

// TestComputePredictionOutcomes_MissedWhenAnyCardNotCast pins the
// inverse: a prediction misses when ANY card is absent from the
// seat's CardsPlayed (TurnCast == 0 counts as "not cast").
// ConfidenceDelta gets the negative miss value (-0.02).
func TestComputePredictionOutcomes_MissedWhenAnyCardNotCast(t *testing.T) {
	ga := &analytics.GameAnalysis{
		Players: []analytics.PlayerAnalysis{{
			Seat: 0,
			CardsPlayed: []analytics.CardPerformance{
				{Name: "Heliod, Sun-Crowned", TurnCast: 4},
				// Walking Ballista absent — never drawn
				{Name: "Mox Diamond", TurnCast: 1},
				// TurnCast=0 should count as not-cast (drawn but never played)
				{Name: "Conjurer's Bauble", TurnCast: 0},
			},
		}},
	}
	preds := []Prediction{
		{
			InstanceID: "deck-heliod-ballista-001",
			Cards:      []string{"Heliod, Sun-Crowned", "Walking Ballista"},
			Confidence: 0.85,
		},
		{
			InstanceID: "deck-bauble-loop-002",
			Cards:      []string{"Conjurer's Bauble"}, // in CardsPlayed but TurnCast=0
			Confidence: 0.40,
		},
	}
	out := ComputePredictionOutcomes(ga, 0, preds)
	if len(out) != 2 {
		t.Fatalf("want 2 outcomes, got %d", len(out))
	}
	for i, o := range out {
		if o.Fired {
			t.Errorf("outcome[%d]: want Fired=false", i)
		}
		if o.ConfidenceDelta != outcomeMissDelta {
			t.Errorf("outcome[%d]: want delta %v, got %v", i, outcomeMissDelta, o.ConfidenceDelta)
		}
	}
}

// TestComputePredictionOutcomes_EmptyCardsListMisses pins that a
// prediction with zero cards never counts as "fired" — an empty
// prediction is degenerate input that should default to missed
// rather than vacuously true.
func TestComputePredictionOutcomes_EmptyCardsListMisses(t *testing.T) {
	ga := &analytics.GameAnalysis{
		Players: []analytics.PlayerAnalysis{{Seat: 0}},
	}
	preds := []Prediction{{InstanceID: "empty-001", Cards: nil, Confidence: 0.5}}
	out := ComputePredictionOutcomes(ga, 0, preds)
	if len(out) != 1 || out[0].Fired {
		t.Errorf("empty cards should NOT fire vacuously, got %+v", out)
	}
}

// TestRecordAndReadPredictionOutcomes_RoundTrip pins the disk
// persistence + read-back loop. Appends outcomes across two writes
// and verifies the read returns the full chronological log.
func TestRecordAndReadPredictionOutcomes_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	batch1 := []PredictionOutcome{
		{InstanceID: "deck-pat-001", Fired: true, ConfidenceDelta: 0.05},
		{InstanceID: "deck-pat-002", Fired: false, ConfidenceDelta: -0.02},
	}
	batch2 := []PredictionOutcome{
		{InstanceID: "deck-pat-003", Fired: true, ConfidenceDelta: 0.05},
	}
	if err := RecordPredictionOutcomes(dir, batch1, "game-001"); err != nil {
		t.Fatalf("record batch 1: %v", err)
	}
	if err := RecordPredictionOutcomes(dir, batch2, "game-002"); err != nil {
		t.Fatalf("record batch 2: %v", err)
	}
	got, err := ReadPredictionOutcomes(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 outcomes (batch1 + batch2), got %d", len(got))
	}
	// GameID stamping: outcomes without a per-record GameID should
	// pick up the write-time GameID parameter.
	if got[0].GameID != "game-001" || got[2].GameID != "game-002" {
		t.Errorf("GameID stamping: got [0]=%q [2]=%q, want game-001 / game-002",
			got[0].GameID, got[2].GameID)
	}
	// Order preserved (append-only).
	if got[0].InstanceID != "deck-pat-001" || got[2].InstanceID != "deck-pat-003" {
		t.Errorf("order: %v", []string{got[0].InstanceID, got[1].InstanceID, got[2].InstanceID})
	}
}

// TestRecordPredictionOutcomes_NoopOnEmpty pins that empty batches
// don't create the file. First-run callers that have no outcomes to
// record shouldn't pollute the huginn dir with an empty jsonl.
func TestRecordPredictionOutcomes_NoopOnEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := RecordPredictionOutcomes(dir, nil, "game-x"); err != nil {
		t.Fatalf("want no error on nil, got: %v", err)
	}
	if err := RecordPredictionOutcomes(dir, []PredictionOutcome{}, "game-x"); err != nil {
		t.Fatalf("want no error on empty, got: %v", err)
	}
	// File should not exist
	if _, err := os.Stat(filepath.Join(dir, predictionOutcomesFile)); !os.IsNotExist(err) {
		t.Errorf("file should not be created on empty batch, got err=%v", err)
	}
}

// TestReadPredictionOutcomes_MissingFileReturnsEmpty pins the first-
// run behavior: a missing prediction_outcomes.jsonl is "no feedback
// yet", not an error. dev-19's --predict CLI calls this on the
// pre-game pass to fold deltas in.
func TestReadPredictionOutcomes_MissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadPredictionOutcomes(dir)
	if err != nil {
		t.Fatalf("missing file should return nil error, got: %v", err)
	}
	if got != nil && len(got) != 0 {
		t.Errorf("want empty/nil, got %d outcomes", len(got))
	}
}

// TestAggregateConfidenceDeltas pins the pattern-key aggregation
// rollup. Multiple outcomes from the same pattern (different seq
// suffixes) sum into a single delta entry, so the next --predict
// round can apply per-pattern confidence updates without scanning
// the full log.
func TestAggregateConfidenceDeltas(t *testing.T) {
	outcomes := []PredictionOutcome{
		{InstanceID: "deck-heliod-loop-001", ConfidenceDelta: 0.05},
		{InstanceID: "deck-heliod-loop-002", ConfidenceDelta: 0.05},
		{InstanceID: "deck-heliod-loop-003", ConfidenceDelta: -0.02},
		{InstanceID: "deck-bauble-recur-001", ConfidenceDelta: -0.02},
	}
	deltas := AggregateConfidenceDeltas(outcomes)
	if len(deltas) != 2 {
		t.Fatalf("want 2 pattern keys, got %d (%+v)", len(deltas), deltas)
	}
	wantHeliod := 0.05 + 0.05 - 0.02
	if got := deltas["deck-heliod-loop"]; got != wantHeliod {
		t.Errorf("heliod aggregate: want %v, got %v", wantHeliod, got)
	}
	if got := deltas["deck-bauble-recur"]; got != -0.02 {
		t.Errorf("bauble aggregate: want -0.02, got %v", got)
	}
}

// TestAggregateConfidenceDeltas_MalformedIDFallsThrough pins the
// defensive handling of InstanceIDs without a "-" delimiter (hand-
// crafted test data, legacy log entries). Full ID becomes the key
// instead of panicking on slice arithmetic.
func TestAggregateConfidenceDeltas_MalformedIDFallsThrough(t *testing.T) {
	outcomes := []PredictionOutcome{
		{InstanceID: "noseparator", ConfidenceDelta: 0.05},
		{InstanceID: "", ConfidenceDelta: -0.02},
	}
	deltas := AggregateConfidenceDeltas(outcomes)
	if deltas["noseparator"] != 0.05 {
		t.Errorf("malformed ID: want 0.05 under full-ID key, got %v", deltas["noseparator"])
	}
	if deltas[""] != -0.02 {
		t.Errorf("empty ID: want -0.02 under empty-string key, got %v", deltas[""])
	}
}
