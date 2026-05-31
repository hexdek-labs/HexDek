package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// HeimdallFeedback contract tests
//
// Pin the schema + serialization + clamp semantics. These contracts are
// load-bearing for dev-4 / dev-13 — when those PRs fill in the
// ComputeWeightDeltas body or add a tuning-loop harness, none of this
// PR's tests should need changing.
// ---------------------------------------------------------------------------

func TestHeimdallFeedback_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fb := &HeimdallFeedback{
		Provenance:    "test:roundtrip",
		GauntletGames: 200,
		Archetypes: []ArchetypeFeedback{
			{
				Archetype: "aggro",
				Deltas: []DimensionDelta{
					{Dimension: "board_presence", Delta: 0.4, Confidence: 0.8, Reason: "underplayed"},
					{Dimension: "life_resource", Delta: -0.2, Confidence: 0.6},
				},
				SampleSize: 80,
				AvgWinRate: 0.27,
			},
		},
	}
	if err := SaveHeimdallFeedback(dir, fb); err != nil {
		t.Fatalf("save: %v", err)
	}
	if fb.SchemaVersion != HeimdallFeedbackSchemaVersion {
		t.Errorf("Save should set SchemaVersion when zero; got %d", fb.SchemaVersion)
	}
	if fb.GeneratedAt == "" {
		t.Errorf("Save should set GeneratedAt when empty")
	}
	out, err := LoadHeimdallFeedback(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil feedback on round-trip")
	}
	if out.Provenance != "test:roundtrip" {
		t.Errorf("provenance lost: %q", out.Provenance)
	}
	if len(out.Archetypes) != 1 || len(out.Archetypes[0].Deltas) != 2 {
		t.Errorf("archetype/delta shape lost: %+v", out)
	}
	if out.Archetypes[0].Deltas[0].Confidence != 0.8 {
		t.Errorf("confidence lost: %v", out.Archetypes[0].Deltas[0].Confidence)
	}
}

func TestHeimdallFeedback_LoadAbsentReturnsNil(t *testing.T) {
	dir := t.TempDir()
	fb, err := LoadHeimdallFeedback(dir)
	if err != nil {
		t.Errorf("missing-file should NOT be an error, got %v", err)
	}
	if fb != nil {
		t.Errorf("missing-file should return nil feedback, got %+v", fb)
	}
}

func TestHeimdallFeedback_SchemaVersionMismatchIsError(t *testing.T) {
	dir := t.TempDir()
	bogus := map[string]interface{}{
		"schema_version":  HeimdallFeedbackSchemaVersion + 99,
		"generated_at":    "2099-01-01T00:00:00Z",
		"provenance":      "future",
		"gauntlet_games":  0,
		"archetypes":      []interface{}{},
	}
	data, _ := json.MarshalIndent(bogus, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, HeimdallFeedbackFile), data, 0o644); err != nil {
		t.Fatalf("write bogus: %v", err)
	}
	if _, err := LoadHeimdallFeedback(dir); err == nil {
		t.Errorf("expected schema-version-mismatch error, got nil")
	}
}

func TestHeimdallFeedback_LoadReclampsHandEditedDeltas(t *testing.T) {
	dir := t.TempDir()
	// Hand-edited file with deltas that exceed MaxDimensionDelta. Loader
	// must clamp these on read so consumers never see the unsafe values.
	tooBig := &HeimdallFeedback{
		SchemaVersion: HeimdallFeedbackSchemaVersion,
		GeneratedAt:   "2026-05-30T00:00:00Z",
		Provenance:    "test:hand-edit",
		Archetypes: []ArchetypeFeedback{
			{
				Archetype:  "aggro",
				SampleSize: 100,
				Deltas: []DimensionDelta{
					{Dimension: "board_presence", Delta: 99.0},
					{Dimension: "life_resource", Delta: -50.0},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(tooBig, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, HeimdallFeedbackFile), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := LoadHeimdallFeedback(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	got := out.Archetypes[0].Deltas
	if got[0].Delta != MaxDimensionDelta {
		t.Errorf("positive overflow not clamped: got %v, want %v", got[0].Delta, MaxDimensionDelta)
	}
	if got[1].Delta != -MaxDimensionDelta {
		t.Errorf("negative overflow not clamped: got %v, want %v", got[1].Delta, -MaxDimensionDelta)
	}
}

func TestClampDelta_BoundsCheck(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{1.5, 1.5},
		{MaxDimensionDelta, MaxDimensionDelta},
		{MaxDimensionDelta + 0.01, MaxDimensionDelta},
		{100, MaxDimensionDelta},
		{-MaxDimensionDelta, -MaxDimensionDelta},
		{-MaxDimensionDelta - 0.01, -MaxDimensionDelta},
		{-100, -MaxDimensionDelta},
	}
	for _, c := range cases {
		if got := ClampDelta(c.in); got != c.want {
			t.Errorf("ClampDelta(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestComputeWeightDeltas_StubReturnsEmptyFeedback(t *testing.T) {
	// Stub contract: returns non-nil feedback with provenance set and
	// empty archetype slice (apply path treats empty as no-op). Any
	// downstream PR that implements real delta computation must keep
	// this null-input safety (nil games slice → safe-empty result).
	fb := ComputeWeightDeltas(nil, "")
	if fb == nil {
		t.Fatal("ComputeWeightDeltas should never return nil")
	}
	if fb.SchemaVersion != HeimdallFeedbackSchemaVersion {
		t.Errorf("stub should set schema version, got %d", fb.SchemaVersion)
	}
	if fb.Provenance != "stub" {
		t.Errorf("empty provenance should default to 'stub', got %q", fb.Provenance)
	}
	if len(fb.Archetypes) != 0 {
		t.Errorf("stub should ship empty Archetypes, got %d", len(fb.Archetypes))
	}
	fbCustom := ComputeWeightDeltas(nil, "loki:42")
	if fbCustom.Provenance != "loki:42" {
		t.Errorf("explicit provenance lost: %q", fbCustom.Provenance)
	}
}

func TestHeimdallFeedback_AtomicWrite_TmpFileCleansUp(t *testing.T) {
	dir := t.TempDir()
	fb := &HeimdallFeedback{Provenance: "test"}
	if err := SaveHeimdallFeedback(dir, fb); err != nil {
		t.Fatalf("save: %v", err)
	}
	// No .tmp leftover; final file present.
	if _, err := os.Stat(filepath.Join(dir, HeimdallFeedbackFile+".tmp")); !os.IsNotExist(err) {
		t.Errorf(".tmp file leftover after successful write")
	}
	if _, err := os.Stat(filepath.Join(dir, HeimdallFeedbackFile)); err != nil {
		t.Errorf("final file missing after save: %v", err)
	}
}
