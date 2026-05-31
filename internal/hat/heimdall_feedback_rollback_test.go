package hat

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestApplyHeimdallFeedbackWithSnapshot pins the snapshot-capable
// variant. The snapshot must be a VALUE-COPY of the pre-application
// weights so subsequent mutations to sp.Weights don't corrupt the
// rollback state.
func TestApplyHeimdallFeedbackWithSnapshot(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	originalCombo := w.ComboProximity
	sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}
	fb := &HeimdallFeedback{
		Source:       "test-source",
		Attributions: map[string]float64{"combo_proximity": 0.05},
		SampleSize:   30,
	}

	n, snap := ApplyHeimdallFeedbackWithSnapshot(sp, fb)
	if n != 1 {
		t.Errorf("want 1 modification, got %d", n)
	}
	if snap == nil {
		t.Fatal("expected snapshot")
	}
	// Snapshot preserves PRE-application value.
	if snap.PreApplicationWeights.ComboProximity != originalCombo {
		t.Errorf("snapshot pre-apply: want %v, got %v",
			originalCombo, snap.PreApplicationWeights.ComboProximity)
	}
	// sp.Weights reflects POST-application value.
	if diff := sp.Weights.ComboProximity - (originalCombo + 0.05); math.Abs(diff) > 1e-9 {
		t.Errorf("post-apply: want %v, got %v",
			originalCombo+0.05, sp.Weights.ComboProximity)
	}
	// Metadata fields populated.
	if snap.FeedbackSource != "test-source" || snap.DimensionsModified != 1 {
		t.Errorf("snapshot metadata: %+v", snap)
	}

	// Mutate sp.Weights AFTER capture; snapshot must NOT mirror the
	// new value (proves value-copy semantics).
	sp.Weights.ComboProximity = 99.0
	if snap.PreApplicationWeights.ComboProximity == 99.0 {
		t.Error("snapshot is sharing memory with sp.Weights — must be value-copy")
	}
}

// TestApplyHeimdallFeedbackWithSnapshot_NilSP returns (0, nil) on nil
// strategy. Caller can't roll back what doesn't exist.
func TestApplyHeimdallFeedbackWithSnapshot_NilSP(t *testing.T) {
	n, snap := ApplyHeimdallFeedbackWithSnapshot(nil, &HeimdallFeedback{
		Attributions: map[string]float64{"combo_proximity": 0.05},
	})
	if n != 0 || snap != nil {
		t.Errorf("nil sp: want (0, nil), got (%d, %+v)", n, snap)
	}
}

// TestApplyHeimdallFeedbackWithSnapshot_NilWeightsMaterialized pins
// that a profile with nil Weights gets the archetype defaults
// materialized BEFORE the snapshot is captured — so a rollback
// restores to the materialized baseline, not nil.
func TestApplyHeimdallFeedbackWithSnapshot_NilWeightsMaterialized(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeCombo, Weights: nil}
	fb := &HeimdallFeedback{
		Attributions: map[string]float64{"combo_proximity": 0.10},
		SampleSize:   30,
	}
	n, snap := ApplyHeimdallFeedbackWithSnapshot(sp, fb)
	if n != 1 {
		t.Errorf("want 1 modification, got %d", n)
	}
	if snap == nil {
		t.Fatal("expected snapshot")
	}
	baseline := DefaultWeightsForArchetype(ArchetypeCombo)
	if snap.PreApplicationWeights.ComboProximity != baseline.ComboProximity {
		t.Errorf("materialized baseline: want %v, got %v",
			baseline.ComboProximity, snap.PreApplicationWeights.ComboProximity)
	}
}

// TestRollbackFromSnapshot_RestoresWeights pins the round trip:
// apply → snapshot → restore returns Weights to byte-identical pre-
// application state.
func TestRollbackFromSnapshot_RestoresWeights(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	originalArr := w.AsArray()
	sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}
	fb := &HeimdallFeedback{
		Attributions: map[string]float64{
			"combo_proximity": 0.05,
			"board_presence":  -0.03,
			"life_resource":   0.04,
		},
		SampleSize: 30,
	}
	_, snap := ApplyHeimdallFeedbackWithSnapshot(sp, fb)
	// Confirm SOMETHING changed (apply actually mutated weights).
	postArr := sp.Weights.AsArray()
	changed := 0
	for i := range postArr {
		if postArr[i] != originalArr[i] {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("apply did not mutate weights — test setup broken")
	}
	// Roll back.
	RollbackFromSnapshot(sp, snap)
	restoredArr := sp.Weights.AsArray()
	for i := range restoredArr {
		if restoredArr[i] != originalArr[i] {
			t.Errorf("dim %d not restored: original=%v restored=%v",
				i, originalArr[i], restoredArr[i])
		}
	}
}

// TestRollbackFromSnapshot_Idempotent pins that two consecutive
// rollbacks produce the same result as one. The applier could be
// called multiple times in pathological code paths; rollback must
// be safe to retry.
func TestRollbackFromSnapshot_Idempotent(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	originalCombo := w.ComboProximity
	sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}
	fb := &HeimdallFeedback{
		Attributions: map[string]float64{"combo_proximity": 0.05},
		SampleSize:   30,
	}
	_, snap := ApplyHeimdallFeedbackWithSnapshot(sp, fb)
	RollbackFromSnapshot(sp, snap)
	first := sp.Weights.ComboProximity
	RollbackFromSnapshot(sp, snap)
	second := sp.Weights.ComboProximity
	if first != second || first != originalCombo {
		t.Errorf("not idempotent: first=%v second=%v original=%v",
			first, second, originalCombo)
	}
}

// TestRollbackFromSnapshot_NilSafe pins the defensive no-op
// behavior. Callers may invoke rollback with degenerate inputs (no
// apply happened, snapshot lost, etc.) and shouldn't panic.
func TestRollbackFromSnapshot_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil rollback panicked: %v", r)
		}
	}()
	RollbackFromSnapshot(nil, nil)
	RollbackFromSnapshot(&StrategyProfile{}, nil)
	RollbackFromSnapshot(nil, &FeedbackSnapshot{})
}

// TestShouldRollback pins the threshold decision matrix. The "drop"
// is baseline minus current; rollback fires when drop > threshold
// (strict inequality — exactly-at-threshold does NOT trip).
func TestShouldRollback(t *testing.T) {
	cases := []struct {
		name      string
		baseline  float64
		current   float64
		threshold float64
		want      bool
	}{
		{"clean run within threshold", 0.30, 0.28, 0.05, false},
		{"drop equals threshold (no trip)", 0.30, 0.25, 0.05, false},
		{"drop exceeds threshold (trip)", 0.30, 0.24, 0.05, true},
		{"current higher than baseline (improvement)", 0.30, 0.40, 0.05, false},
		{"baseline 0 (disabled gate)", 0, 0.10, 0.05, false},
		{"threshold 0 (disabled gate)", 0.30, 0.10, 0, false},
		{"negative baseline (defensive disabled)", -0.10, 0.05, 0.05, false},
		{"large drop with large threshold (no trip)", 0.50, 0.30, 0.25, false},
		{"large drop with large threshold (trip)", 0.50, 0.20, 0.25, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldRollback(tc.baseline, tc.current, tc.threshold)
			if got != tc.want {
				t.Errorf("baseline=%v current=%v threshold=%v: want %v, got %v",
					tc.baseline, tc.current, tc.threshold, tc.want, got)
			}
		})
	}
}

// TestFeedbackRejectionSidecar_RoundTrip pins the disk write/read
// cycle for the rejection marker. Caller writes via
// MarkFeedbackRejected, next-run caller checks via IsFeedbackRejected
// + LoadFeedbackRejection to see why.
func TestFeedbackRejectionSidecar_RoundTrip(t *testing.T) {
	// Pin time so the RejectedAt field is deterministic.
	pinned := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	origTimeNow := timeNow
	timeNow = func() time.Time { return pinned }
	t.Cleanup(func() { timeNow = origTimeNow })

	dir := t.TempDir()
	fbPath := filepath.Join(dir, "feedback.json")

	// Pre-state: no rejection sidecar.
	if IsFeedbackRejected(fbPath) {
		t.Fatal("expected no rejection sidecar pre-mark")
	}
	if rej, err := LoadFeedbackRejection(fbPath); rej != nil || err != nil {
		t.Errorf("pre-mark load: want (nil, nil), got (%+v, %v)", rej, err)
	}

	// Mark rejected.
	if err := MarkFeedbackRejected(fbPath, 0.28, 0.20, 0.05, "test-gauntlet"); err != nil {
		t.Fatalf("mark: %v", err)
	}

	// Post-mark checks.
	if !IsFeedbackRejected(fbPath) {
		t.Error("expected IsFeedbackRejected=true post-mark")
	}
	rej, err := LoadFeedbackRejection(fbPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rej == nil {
		t.Fatal("expected rejection record")
	}
	if rej.FeedbackPath != fbPath {
		t.Errorf("FeedbackPath: want %v, got %v", fbPath, rej.FeedbackPath)
	}
	if rej.BaselineWinRate != 0.28 || rej.CurrentWinRate != 0.20 || rej.Threshold != 0.05 {
		t.Errorf("rate fields: %+v", rej)
	}
	if rej.Note != "test-gauntlet" {
		t.Errorf("Note: got %q", rej.Note)
	}
	if rej.RejectedAt != pinned.Format("2006-01-02T15:04:05Z07:00") {
		t.Errorf("RejectedAt: got %q (pin %q)", rej.RejectedAt, pinned.Format("2006-01-02T15:04:05Z07:00"))
	}

	// Confirm raw file contents are valid JSON (readable for humans).
	raw, _ := os.ReadFile(fbPath + ".rejected")
	var parsed FeedbackRejection
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Errorf("sidecar file not valid JSON: %v", err)
	}
}

// TestClearFeedbackRejection pins the manual-clear path. Operator
// deletes the sidecar to re-enable an auto-rejected feedback file
// after investigation.
func TestClearFeedbackRejection(t *testing.T) {
	dir := t.TempDir()
	fbPath := filepath.Join(dir, "feedback.json")

	// Mark + verify present.
	if err := MarkFeedbackRejected(fbPath, 0.28, 0.20, 0.05, ""); err != nil {
		t.Fatalf("mark: %v", err)
	}
	if !IsFeedbackRejected(fbPath) {
		t.Fatal("setup: marker should be present")
	}

	// Clear + verify absent.
	if err := ClearFeedbackRejection(fbPath); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if IsFeedbackRejected(fbPath) {
		t.Error("expected marker cleared")
	}

	// Clearing a non-existent marker is a no-op (not an error).
	if err := ClearFeedbackRejection(fbPath); err != nil {
		t.Errorf("clear (already absent): want nil, got %v", err)
	}
}

// TestMarkFeedbackRejected_EmptyPathNoOp pins the defensive empty-
// path handling. Caller may pass an empty string when the feedback
// was loaded from a non-file source (programmatic feedback in tests,
// future inline-feedback flag).
func TestMarkFeedbackRejected_EmptyPathNoOp(t *testing.T) {
	if err := MarkFeedbackRejected("", 0.30, 0.20, 0.05, "x"); err != nil {
		t.Errorf("empty path: want nil, got %v", err)
	}
	if IsFeedbackRejected("") {
		t.Error("empty path should not register as rejected")
	}
	if rej, err := LoadFeedbackRejection(""); rej != nil || err != nil {
		t.Errorf("empty path load: want (nil, nil), got (%+v, %v)", rej, err)
	}
	if err := ClearFeedbackRejection(""); err != nil {
		t.Errorf("empty path clear: want nil, got %v", err)
	}
}

// TestLoadFeedbackRejection_MissingFileReturnsNilNil pins the
// "no sidecar yet" first-run behavior. Distinguishes "file not yet
// rejected" (nil, nil) from "file rejected but unreadable" (nil, err).
func TestLoadFeedbackRejection_MissingFileReturnsNilNil(t *testing.T) {
	rej, err := LoadFeedbackRejection("/nonexistent/feedback.json")
	if rej != nil || err != nil {
		t.Errorf("want (nil, nil), got (%+v, %v)", rej, err)
	}
}

// TestRollbackEndToEnd is the integration scenario: apply feedback
// to a deck, observe a "bad gauntlet" result, run the full rollback
// pipeline (decision → mark rejected → restore weights), and verify
// the next load skips the marked-rejected feedback.
func TestRollbackEndToEnd(t *testing.T) {
	dir := t.TempDir()
	fbPath := filepath.Join(dir, "feedback.json")

	// Stage a real feedback file on disk (LoadHeimdallFeedback reads it).
	fbData, _ := json.Marshal(HeimdallFeedback{
		Source:       "test-source",
		SampleSize:   30,
		Attributions: map[string]float64{"combo_proximity": 0.10},
	})
	os.WriteFile(fbPath, fbData, 0o644)

	// Round 1: load + apply with snapshot capture.
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	originalCombo := w.ComboProximity
	sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}
	fb, err := LoadHeimdallFeedback(fbPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, snap := ApplyHeimdallFeedbackWithSnapshot(sp, fb)

	// Simulate post-gauntlet: observed win rate 0.20 vs baseline 0.30
	// (drop 0.10, threshold 0.05 → ShouldRollback=true).
	if !ShouldRollback(0.30, 0.20, 0.05) {
		t.Fatal("setup: ShouldRollback should fire")
	}

	// Execute rollback pipeline.
	RollbackFromSnapshot(sp, snap)
	if err := MarkFeedbackRejected(fbPath, 0.30, 0.20, 0.05, "end-to-end test"); err != nil {
		t.Fatalf("mark: %v", err)
	}

	// Verify weights restored.
	if sp.Weights.ComboProximity != originalCombo {
		t.Errorf("weights not restored: want %v, got %v", originalCombo, sp.Weights.ComboProximity)
	}

	// Verify next run would skip the feedback.
	if !IsFeedbackRejected(fbPath) {
		t.Error("expected feedback marked as rejected after rollback pipeline")
	}
}
