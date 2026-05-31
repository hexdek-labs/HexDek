package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
)

// ---------------------------------------------------------------------------
// HeimdallFeedback apply-path contract tests
//
// Pin: overlay applies + clamps + zero-floors; nil/empty/below-threshold
// feedbacks no-op; concurrent SetActive doesn't race the dispatch path;
// known-archetype + unknown-archetype paths both honored.
// ---------------------------------------------------------------------------

// resetFeedback ensures tests don't leak overlay state into one another.
// All tests in this file MUST end with a defer call.
func resetFeedback(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		ClearActiveFeedback()
	})
}

func TestApplyFeedbackOverlay_NoActiveFeedbackReturnsBaseUnchanged(t *testing.T) {
	resetFeedback(t)
	ClearActiveFeedback() // belt + suspenders

	w := DefaultWeightsForArchetype(ArchetypeAggro)
	base := archetypeWeights[ArchetypeAggro]
	if w.BoardPresence != base.BoardPresence {
		t.Errorf("no-active path should return base weights unchanged; got BoardPresence=%v want %v",
			w.BoardPresence, base.BoardPresence)
	}
}

func TestApplyFeedbackOverlay_DirectPolicyAddsDeltaToBase(t *testing.T) {
	resetFeedback(t)
	baseAggro := archetypeWeights[ArchetypeAggro]

	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Provenance:    "test:direct",
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: 200,
				Deltas: []analytics.DimensionDelta{
					{Dimension: "board_presence", Delta: 0.5},
					{Dimension: "life_resource", Delta: -0.3},
				},
			},
		},
	}
	if err := SetActiveFeedback(fb, PolicyDirect); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	w := DefaultWeightsForArchetype(ArchetypeAggro)
	if w.BoardPresence != baseAggro.BoardPresence+0.5 {
		t.Errorf("BoardPresence overlay: got %v, want %v", w.BoardPresence, baseAggro.BoardPresence+0.5)
	}
	if w.LifeResource != baseAggro.LifeResource-0.3 {
		t.Errorf("LifeResource overlay: got %v, want %v", w.LifeResource, baseAggro.LifeResource-0.3)
	}
	// Other dimensions should be untouched.
	if w.CardAdvantage != baseAggro.CardAdvantage {
		t.Errorf("CardAdvantage should be unchanged; got %v want %v", w.CardAdvantage, baseAggro.CardAdvantage)
	}
}

func TestApplyFeedbackOverlay_OnlyAffectsTargetArchetype(t *testing.T) {
	resetFeedback(t)
	baseControl := archetypeWeights[ArchetypeControl]

	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: 100,
				Deltas:     []analytics.DimensionDelta{{Dimension: "board_presence", Delta: 1.0}},
			},
		},
	}
	if err := SetActiveFeedback(fb, PolicyDirect); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	wControl := DefaultWeightsForArchetype(ArchetypeControl)
	if wControl.BoardPresence != baseControl.BoardPresence {
		t.Errorf("Aggro overlay leaked into Control: got %v want %v", wControl.BoardPresence, baseControl.BoardPresence)
	}
}

func TestApplyFeedbackOverlay_ConfidenceWeightedScalesDelta(t *testing.T) {
	resetFeedback(t)
	baseAggro := archetypeWeights[ArchetypeAggro]

	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: 100,
				Deltas: []analytics.DimensionDelta{
					{Dimension: "board_presence", Delta: 1.0, Confidence: 0.5},
				},
			},
		},
	}
	if err := SetActiveFeedback(fb, PolicyConfidenceWeighted); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	w := DefaultWeightsForArchetype(ArchetypeAggro)
	want := baseAggro.BoardPresence + 0.5 // delta 1.0 × confidence 0.5
	if w.BoardPresence != want {
		t.Errorf("confidence-weighted: got BoardPresence=%v want %v", w.BoardPresence, want)
	}
}

func TestApplyFeedbackOverlay_AdvisoryOnlyIgnoresDeltas(t *testing.T) {
	resetFeedback(t)
	baseAggro := archetypeWeights[ArchetypeAggro]

	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: 200,
				Deltas:     []analytics.DimensionDelta{{Dimension: "board_presence", Delta: 1.0}},
			},
		},
	}
	if err := SetActiveFeedback(fb, PolicyAdvisoryOnly); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	w := DefaultWeightsForArchetype(ArchetypeAggro)
	if w.BoardPresence != baseAggro.BoardPresence {
		t.Errorf("advisory-only should not affect weights; got %v want %v", w.BoardPresence, baseAggro.BoardPresence)
	}
	// But ActiveFeedback should still expose the loaded feedback for reports.
	if ActiveFeedback() == nil {
		t.Errorf("advisory-only should still expose ActiveFeedback for reports")
	}
}

func TestApplyFeedbackOverlay_BelowMinSampleSizeIsSkipped(t *testing.T) {
	resetFeedback(t)
	baseAggro := archetypeWeights[ArchetypeAggro]

	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: analytics.MinSampleSize - 1, // below threshold
				Deltas:     []analytics.DimensionDelta{{Dimension: "board_presence", Delta: 1.0}},
			},
		},
	}
	if err := SetActiveFeedback(fb, PolicyDirect); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	w := DefaultWeightsForArchetype(ArchetypeAggro)
	if w.BoardPresence != baseAggro.BoardPresence {
		t.Errorf("below-MinSampleSize archetype should not be applied; got %v want %v",
			w.BoardPresence, baseAggro.BoardPresence)
	}
}

func TestApplyFeedbackOverlay_HandEditedOverflowsAreClamped(t *testing.T) {
	resetFeedback(t)
	baseAggro := archetypeWeights[ArchetypeAggro]

	// Bypass producer-side clamp by hand-constructing a feedback with
	// an out-of-bounds delta. The apply path must re-clamp.
	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: 100,
				Deltas:     []analytics.DimensionDelta{{Dimension: "board_presence", Delta: 99.0}},
			},
		},
	}
	if err := SetActiveFeedback(fb, PolicyDirect); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	w := DefaultWeightsForArchetype(ArchetypeAggro)
	wantMax := baseAggro.BoardPresence + analytics.MaxDimensionDelta
	if w.BoardPresence > wantMax+1e-9 {
		t.Errorf("99.0 delta not clamped: got BoardPresence=%v, max-allowed %v", w.BoardPresence, wantMax)
	}
}

func TestApplyFeedbackOverlay_NegativeOverlayFloorsAtZero(t *testing.T) {
	resetFeedback(t)
	baseAggro := archetypeWeights[ArchetypeAggro]
	// ComboProximity baseline on aggro is 0.1. A clamped negative delta
	// of −2.0 would land at −1.9; the floor must zero it.
	if baseAggro.ComboProximity >= 0.5 {
		t.Skipf("aggro ComboProximity baseline changed (%v); test assumption stale", baseAggro.ComboProximity)
	}
	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: 100,
				Deltas:     []analytics.DimensionDelta{{Dimension: "combo_proximity", Delta: -10.0}},
			},
		},
	}
	if err := SetActiveFeedback(fb, PolicyDirect); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	w := DefaultWeightsForArchetype(ArchetypeAggro)
	if w.ComboProximity < 0 {
		t.Errorf("negative weight produced — floor failed; got %v", w.ComboProximity)
	}
	if w.ComboProximity != 0 {
		t.Errorf("expected zero-floor after overlay drove below 0; got %v", w.ComboProximity)
	}
}

func TestApplyFeedbackOverlay_UnknownDimensionNoOps(t *testing.T) {
	resetFeedback(t)
	baseAggro := archetypeWeights[ArchetypeAggro]

	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: 100,
				Deltas: []analytics.DimensionDelta{
					{Dimension: "future_dimension_that_doesnt_exist_yet", Delta: 1.0},
					{Dimension: "board_presence", Delta: 0.2},
				},
			},
		},
	}
	if err := SetActiveFeedback(fb, PolicyDirect); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	w := DefaultWeightsForArchetype(ArchetypeAggro)
	// Unknown dimension silently ignored — known dimension still applied.
	if w.BoardPresence != baseAggro.BoardPresence+0.2 {
		t.Errorf("known dimension not applied; got %v want %v", w.BoardPresence, baseAggro.BoardPresence+0.2)
	}
}

func TestSetActiveFeedback_NilClears(t *testing.T) {
	resetFeedback(t)
	baseAggro := archetypeWeights[ArchetypeAggro]

	// First install a feedback.
	_ = SetActiveFeedback(&analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion,
		Archetypes: []analytics.ArchetypeFeedback{{
			Archetype: ArchetypeAggro, SampleSize: 100,
			Deltas: []analytics.DimensionDelta{{Dimension: "board_presence", Delta: 0.5}},
		}},
	}, PolicyDirect)
	if w := DefaultWeightsForArchetype(ArchetypeAggro); w.BoardPresence == baseAggro.BoardPresence {
		t.Fatal("first SetActive didn't take effect; test setup broken")
	}
	// Now clear.
	if err := SetActiveFeedback(nil, PolicyDirect); err != nil {
		t.Errorf("nil SetActive: %v", err)
	}
	if w := DefaultWeightsForArchetype(ArchetypeAggro); w.BoardPresence != baseAggro.BoardPresence {
		t.Errorf("nil SetActive should clear overlay; still got overlaid %v", w.BoardPresence)
	}
	if ActiveFeedback() != nil {
		t.Errorf("nil SetActive should clear ActiveFeedback, got %+v", ActiveFeedback())
	}
}

func TestSetActiveFeedback_SchemaVersionMismatchRejects(t *testing.T) {
	resetFeedback(t)
	fb := &analytics.HeimdallFeedback{
		SchemaVersion: analytics.HeimdallFeedbackSchemaVersion + 99,
	}
	if err := SetActiveFeedback(fb, PolicyDirect); err == nil {
		t.Errorf("expected schema-version-mismatch error, got nil")
	}
}
