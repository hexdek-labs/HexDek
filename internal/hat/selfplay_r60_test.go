package hat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// selfplay_r60_test pins SelfPlayManager construction + accessor +
// threshold-gate paths. The runTraining branch shells to Python and
// would be flaky in CI; tests deliberately keep RecordSamples calls
// BELOW the threshold so the goroutine never spawns. The covered
// paths are the construction/inspection surface the audit flagged at
// 0% — accessors, threshold gate, file rotation helper.

// TestDefaultSelfPlayConfig_PathsResolved verifies the defaults: paths
// are joined onto baseDir, threshold/epochs/python defaults are
// populated. baseDir prefix must appear in every path field.
func TestDefaultSelfPlayConfig_PathsResolved(t *testing.T) {
	cfg := DefaultSelfPlayConfig("/opt/hexdek")
	if !strings.HasPrefix(cfg.SamplesPath, "/opt/hexdek/") {
		t.Errorf("SamplesPath not under baseDir: %q", cfg.SamplesPath)
	}
	if !strings.HasPrefix(cfg.ModelPath, "/opt/hexdek/") {
		t.Errorf("ModelPath not under baseDir: %q", cfg.ModelPath)
	}
	if !strings.HasPrefix(cfg.CheckpointDir, "/opt/hexdek/") {
		t.Errorf("CheckpointDir not under baseDir: %q", cfg.CheckpointDir)
	}
	if !strings.HasPrefix(cfg.TrainScript, "/opt/hexdek/") {
		t.Errorf("TrainScript not under baseDir: %q", cfg.TrainScript)
	}
	if cfg.TrainThreshold != 10000 {
		t.Errorf("TrainThreshold default = %d, want 10000", cfg.TrainThreshold)
	}
	if cfg.Epochs != 200 {
		t.Errorf("Epochs default = %d, want 200", cfg.Epochs)
	}
	if cfg.PythonBin != "python3" {
		t.Errorf("PythonBin default = %q, want python3", cfg.PythonBin)
	}
}

// TestNewSelfPlayManager_StartsZeroed verifies a fresh manager starts
// at gen=0, samples=0, training=false. These are the accessor outputs
// callers depend on for telemetry/status rendering.
func TestNewSelfPlayManager_StartsZeroed(t *testing.T) {
	sp := NewSelfPlayManager(SelfPlayConfig{TrainThreshold: 1000})
	if g := sp.Generation(); g != 0 {
		t.Errorf("Generation initial = %d, want 0", g)
	}
	if s := sp.SampleCount(); s != 0 {
		t.Errorf("SampleCount initial = %d, want 0", s)
	}
	if sp.IsTraining() {
		t.Errorf("IsTraining initial = true, want false")
	}
}

// TestRecordSamples_BelowThresholdNoTraining pins the threshold gate:
// recording fewer samples than TrainThreshold must NOT trigger the
// training goroutine. We deliberately set a high threshold so even
// many calls accumulated together stay below it.
func TestRecordSamples_BelowThresholdNoTraining(t *testing.T) {
	sp := NewSelfPlayManager(SelfPlayConfig{TrainThreshold: 100000})
	// Accumulate 100 samples across 10 calls — well under the 100k
	// threshold, so the gate must NOT fire.
	for i := 0; i < 10; i++ {
		sp.RecordSamples(10)
	}
	if sp.SampleCount() != 100 {
		t.Errorf("SampleCount = %d after 100 sample records, want 100", sp.SampleCount())
	}
	if sp.IsTraining() {
		t.Errorf("training should NOT fire below threshold (100 < 100000)")
	}
	if sp.Generation() != 0 {
		t.Errorf("Generation should stay 0 below threshold, got %d", sp.Generation())
	}
}

// TestStatus_Format pins the human-readable status string format —
// downstream telemetry/log lines parse this in places, so the
// "gen=N samples=M training=BOOL" shape must stay stable.
func TestStatus_Format(t *testing.T) {
	sp := NewSelfPlayManager(SelfPlayConfig{TrainThreshold: 100000})
	sp.RecordSamples(42)
	got := sp.Status()
	for _, want := range []string{"gen=0", "samples=42", "training=false"} {
		if !strings.Contains(got, want) {
			t.Errorf("Status() = %q, missing %q", got, want)
		}
	}
}

// TestCleanOldRotations_KeepsNewest verifies the rotation cleanup:
// when more than `keep` rotated files exist, the oldest ones get
// removed; the newest `keep` survive. Cleanup uses lexicographic
// ordering on the rotation timestamp suffix (which is YYYYMMDD-HHMMSS
// — naturally lexicographic).
func TestCleanOldRotations_KeepsNewest(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "samples.jsonl")

	// Create 5 rotated files with sortable timestamps. Names like
	// samples.jsonl.20240101-000000, ..., 20240105-000000.
	rotations := []string{
		"samples.jsonl.20240101-000000",
		"samples.jsonl.20240102-000000",
		"samples.jsonl.20240103-000000",
		"samples.jsonl.20240104-000000",
		"samples.jsonl.20240105-000000",
	}
	for _, name := range rotations {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	cleanOldRotations(basePath, 2) // keep newest 2

	// Newest 2 (-04 and -05) survive; oldest 3 (-01, -02, -03) removed.
	for _, name := range []string{
		"samples.jsonl.20240101-000000",
		"samples.jsonl.20240102-000000",
		"samples.jsonl.20240103-000000",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("expected %s to be removed by cleanup", name)
		}
	}
	for _, name := range []string{
		"samples.jsonl.20240104-000000",
		"samples.jsonl.20240105-000000",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected %s to survive cleanup: %v", name, err)
		}
	}
}

// TestCleanOldRotations_NoopWhenUnderKeep verifies the no-op path:
// if there are already <= keep rotated files, cleanOldRotations is a
// silent no-op (doesn't remove anything).
func TestCleanOldRotations_NoopWhenUnderKeep(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "samples.jsonl")
	// Create 2 rotations; keep=3 → no-op expected.
	for _, n := range []string{"samples.jsonl.20240101-000000", "samples.jsonl.20240102-000000"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0644); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	cleanOldRotations(basePath, 3)
	for _, n := range []string{"samples.jsonl.20240101-000000", "samples.jsonl.20240102-000000"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("file removed when cleanup should have no-op'd: %s", n)
		}
	}
}

// TestCleanOldRotations_IgnoresNonMatchingFiles verifies that files
// not matching the basePath prefix pattern are ignored — only the
// rotation files for THIS base name get considered for cleanup.
// Without this guard, a sibling file like "other-thing.txt" in the
// same directory could be accidentally deleted.
func TestCleanOldRotations_IgnoresNonMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "samples.jsonl")
	// Mix: 4 matching rotations + 1 unrelated file.
	rotations := []string{
		"samples.jsonl.20240101-000000",
		"samples.jsonl.20240102-000000",
		"samples.jsonl.20240103-000000",
		"samples.jsonl.20240104-000000",
	}
	for _, n := range rotations {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0644)
	}
	unrelated := filepath.Join(dir, "other-thing.txt")
	os.WriteFile(unrelated, []byte("y"), 0644)

	cleanOldRotations(basePath, 2) // remove 2 oldest rotations

	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("unrelated file was deleted: %v", err)
	}
}
