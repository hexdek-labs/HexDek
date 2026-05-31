package hat

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadHeimdallFeedback_RoundTrip pins the JSON load path. Writes
// a feedback file, reads it back, and verifies every field survives
// the marshal/unmarshal cycle.
func TestLoadHeimdallFeedback_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feedback.json")
	want := HeimdallFeedback{
		Source:      "tournament-2026-05-30-evening",
		GeneratedAt: "2026-05-30T22:00:00Z",
		SampleSize:  120,
		Attributions: map[string]float64{
			"combo_proximity":  0.10,
			"board_presence":   -0.05,
			"threat_exposure":  0.03,
		},
	}
	data, _ := json.Marshal(want)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := LoadHeimdallFeedback(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got == nil {
		t.Fatal("got nil feedback")
	}
	if got.Source != want.Source || got.SampleSize != want.SampleSize {
		t.Errorf("metadata mismatch: %+v", got)
	}
	if len(got.Attributions) != 3 || got.Attributions["combo_proximity"] != 0.10 {
		t.Errorf("attributions mismatch: %+v", got.Attributions)
	}
}

// TestLoadHeimdallFeedback_EmptyPathReturnsNilNil pins the
// flag-friendly behavior: empty path is "no feedback requested",
// not an error. Lets the tournament CLI pass flag.String value
// unconditionally without guarding.
func TestLoadHeimdallFeedback_EmptyPathReturnsNilNil(t *testing.T) {
	got, err := LoadHeimdallFeedback("")
	if got != nil || err != nil {
		t.Errorf("empty path: want (nil, nil), got (%+v, %v)", got, err)
	}
}

// TestLoadHeimdallFeedback_MissingFileReturnsError pins that a
// non-empty path that can't be read errors instead of silently
// no-op'ing. CLI caller can log + continue, but the explicit error
// is needed for "user passed a typo'd path" diagnostics.
func TestLoadHeimdallFeedback_MissingFileReturnsError(t *testing.T) {
	got, err := LoadHeimdallFeedback("/nonexistent/feedback.json")
	if got != nil || err == nil {
		t.Errorf("missing file: want (nil, err), got (%+v, %v)", got, err)
	}
}

// TestLoadHeimdallFeedback_MissingAttributionsErrors pins the
// minimum schema requirement: a feedback file without an
// Attributions map is structurally invalid (nothing to apply).
// Catches Heimdall-side bugs that would otherwise apply zero deltas
// silently.
func TestLoadHeimdallFeedback_MissingAttributionsErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`{"source":"x","sample_size":10}`), 0o644)
	_, err := LoadHeimdallFeedback(path)
	if err == nil {
		t.Error("expected error on missing attributions")
	}
}

// TestApplyHeimdallFeedback_NilSafety pins defensive zero-returns
// for all nil-input shapes. Returns the count of dimensions
// modified.
func TestApplyHeimdallFeedback_NilSafety(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	sp := &StrategyProfile{Weights: &w}
	fb := &HeimdallFeedback{Attributions: map[string]float64{"combo_proximity": 0.05}}

	cases := []struct {
		name string
		sp   *StrategyProfile
		fb   *HeimdallFeedback
	}{
		{"nil sp", nil, fb},
		{"nil fb", sp, nil},
		{"empty attributions", sp, &HeimdallFeedback{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ApplyHeimdallFeedback(tc.sp, tc.fb); got != 0 {
				t.Errorf("want 0 modifications, got %d", got)
			}
		})
	}
}

// TestApplyHeimdallFeedback_AppliesDelta is the happy-path pin —
// a feedback with one attribution bumps the named weight by the
// delta (assuming no cap / floor / scale interventions). Picks a
// midrange-baseline ComboProximity (0.4 in archetypeWeights) +
// 0.05 delta = 0.45.
func TestApplyHeimdallFeedback_AppliesDelta(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	originalCombo := w.ComboProximity
	sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}
	fb := &HeimdallFeedback{
		Attributions: map[string]float64{"combo_proximity": 0.05},
		// SampleSize 30 → exact 1x scale via sqrt(30/30)=1
		SampleSize: 30,
	}

	n := ApplyHeimdallFeedback(sp, fb)
	if n != 1 {
		t.Fatalf("want 1 dimension modified, got %d", n)
	}
	want := originalCombo + 0.05
	if diff := sp.Weights.ComboProximity - want; math.Abs(diff) > 1e-9 {
		t.Errorf("ComboProximity: want %v, got %v", want, sp.Weights.ComboProximity)
	}
}

// TestApplyHeimdallFeedback_PerDimDeltaCap pins the per-dimension
// cap (±0.15). A 0.50 attribution should produce a 0.15 effective
// delta, not 0.50.
func TestApplyHeimdallFeedback_PerDimDeltaCap(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	originalCombo := w.ComboProximity
	sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}
	fb := &HeimdallFeedback{
		Attributions: map[string]float64{"combo_proximity": 0.50}, // way over cap
		SampleSize:   30,
	}
	ApplyHeimdallFeedback(sp, fb)
	want := originalCombo + perDimDeltaCap // 0.15
	if diff := sp.Weights.ComboProximity - want; math.Abs(diff) > 1e-9 {
		t.Errorf("per-dim cap: want %v (orig + %.2f), got %v",
			want, perDimDeltaCap, sp.Weights.ComboProximity)
	}

	// Symmetric: negative side caps at -0.15.
	w2 := DefaultWeightsForArchetype(ArchetypeMidrange)
	originalBoard := w2.BoardPresence
	sp2 := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w2}
	fb2 := &HeimdallFeedback{
		Attributions: map[string]float64{"board_presence": -0.80},
		SampleSize:   30,
	}
	ApplyHeimdallFeedback(sp2, fb2)
	want2 := originalBoard - perDimDeltaCap // -0.15
	if diff := sp2.Weights.BoardPresence - want2; math.Abs(diff) > 1e-9 {
		t.Errorf("negative cap: want %v, got %v", want2, sp2.Weights.BoardPresence)
	}
}

// TestApplyHeimdallFeedback_TotalMagnitudeBudget pins the
// 0.5-total-magnitude rule. A feedback that nudges 8 dimensions
// at +0.10 each would total +0.80; scaling proportionally to fit
// the 0.5 budget produces 8 × 0.0625.
func TestApplyHeimdallFeedback_TotalMagnitudeBudget(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	original := w.AsArray()
	sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}

	fb := &HeimdallFeedback{
		Attributions: map[string]float64{
			"board_presence":         0.10,
			"card_advantage":         0.10,
			"mana_advantage":         0.10,
			"life_resource":          0.10,
			"combo_proximity":        0.10,
			"threat_exposure":        0.10,
			"commander_progress":     0.10,
			"graveyard_value":        0.10,
		},
		SampleSize: 30,
	}
	ApplyHeimdallFeedback(sp, fb)
	// Total raw magnitude was 8 × 0.10 = 0.80 (each survived the per-dim cap of 0.15).
	// Budget scale = 0.5 / 0.80 = 0.625.
	// Each effective delta = 0.10 × 0.625 = 0.0625.
	wantDelta := 0.0625
	postBoard := original[0] + wantDelta
	if diff := sp.Weights.BoardPresence - postBoard; math.Abs(diff) > 1e-9 {
		t.Errorf("budget-scaled BoardPresence: want %v, got %v",
			postBoard, sp.Weights.BoardPresence)
	}

	// Verify total |Δ| applied equals budget (within float tolerance).
	totalApplied := 0.0
	post := sp.Weights.AsArray()
	for i := range post {
		totalApplied += math.Abs(post[i] - original[i])
	}
	if math.Abs(totalApplied-totalMagnitudeBudget) > 1e-6 {
		t.Errorf("total applied magnitude: want %v, got %v",
			totalMagnitudeBudget, totalApplied)
	}
}

// TestApplyHeimdallFeedback_FloorFloors pins the regression-safety
// floor — no weight can be driven below 0.05. Catches the "noisy
// negative feedback zeros out the dimension" failure mode.
func TestApplyHeimdallFeedback_FloorFloors(t *testing.T) {
	w := EvalWeights{ComboProximity: 0.10} // close to the floor
	sp := &StrategyProfile{Weights: &w}
	fb := &HeimdallFeedback{
		Attributions: map[string]float64{"combo_proximity": -0.15}, // would push to -0.05
		SampleSize:   30,
	}
	ApplyHeimdallFeedback(sp, fb)
	if sp.Weights.ComboProximity != minimumWeightFloor {
		t.Errorf("floor: want %v, got %v", minimumWeightFloor, sp.Weights.ComboProximity)
	}
}

// TestApplyHeimdallFeedback_UnknownKeysIgnored pins the defensive
// schema-drift handling: an attribution for "fake_dimension" must
// not crash and must not be counted in the modification total.
func TestApplyHeimdallFeedback_UnknownKeysIgnored(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeMidrange)
	originalCombo := w.ComboProximity
	sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}
	fb := &HeimdallFeedback{
		Attributions: map[string]float64{
			"fake_dimension":   0.50,
			"another_fake":     -0.30,
			"combo_proximity":  0.05,
		},
		SampleSize: 30,
	}
	n := ApplyHeimdallFeedback(sp, fb)
	if n != 1 {
		t.Errorf("expected 1 modification (only combo_proximity), got %d", n)
	}
	if diff := sp.Weights.ComboProximity - (originalCombo + 0.05); math.Abs(diff) > 1e-9 {
		t.Errorf("ComboProximity expected nudge intact, got %v (delta %v)",
			sp.Weights.ComboProximity, sp.Weights.ComboProximity-originalCombo)
	}
}

// TestApplyHeimdallFeedback_SampleSizeScaling pins the
// sqrt(N/30) damping curve. SampleSize 8 should apply at
// sqrt(8/30) ≈ 0.516×; SampleSize 0 (unknown) bypasses scaling;
// SampleSize 120 caps at 1× (no over-trust).
func TestApplyHeimdallFeedback_SampleSizeScaling(t *testing.T) {
	cases := []struct {
		name       string
		sampleSize int
		wantScale  float64
	}{
		{"small N=8 → ~0.516x", 8, math.Sqrt(8.0 / 30.0)},
		{"reference N=30 → 1.0x", 30, 1.0},
		{"large N=120 → caps at 1.0x", 120, 1.0},
		{"unknown N=0 → 1.0x (bypass)", 0, 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := DefaultWeightsForArchetype(ArchetypeMidrange)
			originalCombo := w.ComboProximity
			sp := &StrategyProfile{Archetype: ArchetypeMidrange, Weights: &w}
			fb := &HeimdallFeedback{
				Attributions: map[string]float64{"combo_proximity": 0.10},
				SampleSize:   tc.sampleSize,
			}
			ApplyHeimdallFeedback(sp, fb)
			gotDelta := sp.Weights.ComboProximity - originalCombo
			wantDelta := 0.10 * tc.wantScale
			if diff := gotDelta - wantDelta; math.Abs(diff) > 1e-9 {
				t.Errorf("N=%d: want delta %v (scale %v), got %v",
					tc.sampleSize, wantDelta, tc.wantScale, gotDelta)
			}
		})
	}
}

// TestApplyHeimdallFeedback_MaterializesWeightsWhenNil pins that
// a StrategyProfile with nil Weights still gets feedback applied —
// the applier materializes a copy of the archetype defaults
// before nudging. Prevents the silent no-op when Freya didn't
// supply weights (a real case for legacy strategy.json reports).
func TestApplyHeimdallFeedback_MaterializesWeightsWhenNil(t *testing.T) {
	sp := &StrategyProfile{Archetype: ArchetypeCombo, Weights: nil}
	fb := &HeimdallFeedback{
		Attributions: map[string]float64{"combo_proximity": 0.05},
		SampleSize:   30,
	}
	n := ApplyHeimdallFeedback(sp, fb)
	if n != 1 {
		t.Errorf("want 1 modification, got %d", n)
	}
	if sp.Weights == nil {
		t.Fatal("Weights should have been materialized")
	}
	baseline := DefaultWeightsForArchetype(ArchetypeCombo)
	want := baseline.ComboProximity + 0.05
	if diff := sp.Weights.ComboProximity - want; math.Abs(diff) > 1e-9 {
		t.Errorf("nil-Weights materialize+apply: want %v, got %v", want, sp.Weights.ComboProximity)
	}
}

// TestHeimdallFeedback_AllDimensionsCovered iterates every JSON-tagged
// EvalWeights dimension and confirms the applyDeltaToWeight switch
// covers it (the delta is actually applied, not silently dropped).
// Pin against the "added a new EvalWeights field but forgot to extend
// the switch" drift mode. The reflective evalWeightKeySet is the
// source of truth; this test is the "actually-applied" cross-check.
func TestHeimdallFeedback_AllDimensionsCovered(t *testing.T) {
	for key := range evalWeightKeySet {
		t.Run(key, func(t *testing.T) {
			w := EvalWeights{
				BoardPresence: 1.0, CardAdvantage: 1.0, ManaAdvantage: 1.0,
				LifeResource: 1.0, ComboProximity: 1.0, ThreatExposure: 1.0,
				CommanderProgress: 1.0, GraveyardValue: 1.0, DrainEngine: 1.0,
				ArtifactSynergy: 1.0, EnchantmentSynergy: 1.0, OpponentGraveyardThreat: 1.0,
				PartnerSynergy: 1.0, ActivationTempo: 1.0, ToolboxBreadth: 1.0,
				ThreatTrajectory: 1.0, StackInteraction: 1.0, PlaneswalkerProgress: 1.0,
				ExileZoneAssets: 1.0, StaxLockProgress: 1.0,
			}
			before := w.AsArray()
			applyDeltaToWeight(&w, key, 0.10)
			after := w.AsArray()
			delta := 0.0
			for i := range after {
				delta += math.Abs(after[i] - before[i])
			}
			if delta == 0 {
				t.Errorf("dimension %q: delta dropped silently (applyDeltaToWeight switch missing arm)", key)
			}
		})
	}
}
