package hat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/analytics"
)

// ---------------------------------------------------------------------------
// Heimdall → hat self-tune end-to-end integration pin (R60)
//
// scripts/self-tune-loop.sh runs a 500-game gauntlet to validate the
// real end-to-end loop. This test pins the SAME data path against an
// in-memory feedback file so the integration is verified at unit-test
// speed (and on every PR), independent of the 500-game gauntlet
// shipping a feedback that actually shifts gameplay.
//
// Path under test (mirrors cmd/hexdek-tournament/main.go):
//
//   1. analytics.SaveHeimdallFeedback(dir, fb)
//      ← gauntlet A's --feedback-out side
//
//   2. analytics.LoadHeimdallFeedback(dir) returns fb
//      ← gauntlet B's --apply-feedback ingress
//
//   3. hat.SetActiveFeedback(fb, policy)
//      ← gauntlet B's hat installation
//
//   4. hat.DefaultWeightsForArchetype(archetype) returns OVERLAID weights
//      ← gauntlet B's MCTS dispatch path on every evaluator construction
//
// All four steps must succeed in order and produce overlaid weights
// that differ from the baseline.
// ---------------------------------------------------------------------------

func TestHeimdallSelfTuneLoop_EndToEndRoundTrip(t *testing.T) {
	t.Cleanup(ClearActiveFeedback)

	dir := t.TempDir()
	baselineAggro := archetypeWeights[ArchetypeAggro]

	// --- Step 1: gauntlet A's egress side — compute + save feedback.
	// Hand-build the feedback (real ComputeWeightDeltas is still a stub
	// returning empty; this test pins the WIRING, not the attribution
	// body). The deltas exercise both positive + negative arms.
	fb := &analytics.HeimdallFeedback{
		Provenance:    "test:self_tune_integration",
		GauntletGames: 500,
		Archetypes: []analytics.ArchetypeFeedback{
			{
				Archetype:  ArchetypeAggro,
				SampleSize: 200,
				AvgWinRate: 0.29,
				Deltas: []analytics.DimensionDelta{
					{Dimension: "board_presence", Delta: 0.4, Confidence: 0.8, Reason: "underplayed"},
					{Dimension: "life_resource", Delta: -0.2, Confidence: 0.7, Reason: "overplayed"},
				},
			},
		},
	}
	if err := analytics.SaveHeimdallFeedback(dir, fb); err != nil {
		t.Fatalf("Step 1 (SaveHeimdallFeedback): %v", err)
	}

	// Verify the file landed where the CLI's --feedback-out expects it.
	if _, err := os.Stat(filepath.Join(dir, analytics.HeimdallFeedbackFile)); err != nil {
		t.Fatalf("expected %s on disk: %v", analytics.HeimdallFeedbackFile, err)
	}

	// --- Step 2: gauntlet B's ingress side — load feedback from disk.
	loaded, err := analytics.LoadHeimdallFeedback(dir)
	if err != nil {
		t.Fatalf("Step 2 (LoadHeimdallFeedback): %v", err)
	}
	if loaded == nil {
		t.Fatal("Step 2: loaded feedback is nil; round-trip broken")
	}
	if loaded.GauntletGames != 500 {
		t.Errorf("Step 2: gauntlet game count lost in round-trip: %d", loaded.GauntletGames)
	}
	if len(loaded.Archetypes) != 1 {
		t.Fatalf("Step 2: expected 1 archetype, got %d", len(loaded.Archetypes))
	}

	// --- Step 3: gauntlet B installs the overlay via SetActiveFeedback.
	if err := SetActiveFeedback(loaded, PolicyDirect); err != nil {
		t.Fatalf("Step 3 (SetActiveFeedback): %v", err)
	}
	if af := ActiveFeedback(); af == nil {
		t.Fatal("Step 3: ActiveFeedback returned nil after SetActive")
	}

	// --- Step 4: dispatch path returns overlaid weights.
	got := DefaultWeightsForArchetype(ArchetypeAggro)
	if got.BoardPresence != baselineAggro.BoardPresence+0.4 {
		t.Errorf("Step 4: BoardPresence not overlaid: got %v, expected baseline %v + 0.4",
			got.BoardPresence, baselineAggro.BoardPresence)
	}
	if got.LifeResource != baselineAggro.LifeResource-0.2 {
		t.Errorf("Step 4: LifeResource not overlaid: got %v, expected baseline %v - 0.2",
			got.LifeResource, baselineAggro.LifeResource)
	}

	// Other archetypes untouched.
	control := DefaultWeightsForArchetype(ArchetypeControl)
	baselineControl := archetypeWeights[ArchetypeControl]
	if control.BoardPresence != baselineControl.BoardPresence {
		t.Errorf("Step 4: aggro overlay leaked into control: got %v, baseline %v",
			control.BoardPresence, baselineControl.BoardPresence)
	}
}

func TestHeimdallSelfTuneLoop_EmptyFeedbackIsNoOp(t *testing.T) {
	// Pins the stub-attribution behavior: when ComputeWeightDeltas
	// returns an empty feedback (current state, since PR #955 is
	// rebasing), the full round-trip through Save/Load/SetActive must
	// produce ZERO change to baseline weights. This is what makes the
	// self-tune-loop.sh "VERDICT: feedback file empty" branch safe to
	// run on master without poisoning gameplay.
	t.Cleanup(ClearActiveFeedback)

	dir := t.TempDir()
	baselineAggro := archetypeWeights[ArchetypeAggro]

	// Mirror what the CLI does post-gauntlet when ComputeWeightDeltas
	// is the stub: empty Archetypes, valid SchemaVersion.
	stub := analytics.ComputeWeightDeltas(nil, "tournament:test:games=500")
	if err := analytics.SaveHeimdallFeedback(dir, stub); err != nil {
		t.Fatalf("save stub: %v", err)
	}
	loaded, err := analytics.LoadHeimdallFeedback(dir)
	if err != nil {
		t.Fatalf("load stub: %v", err)
	}
	if err := SetActiveFeedback(loaded, PolicyDirect); err != nil {
		t.Fatalf("SetActive stub: %v", err)
	}

	got := DefaultWeightsForArchetype(ArchetypeAggro)
	if got != baselineAggro {
		t.Errorf("empty-stub feedback should produce zero-delta weights: got %+v, baseline %+v",
			got, baselineAggro)
	}
}

func TestHeimdallSelfTuneLoop_AdversarialFeedbackCannotInvertMCTS(t *testing.T) {
	// Hand-edited feedback with grotesquely large negative deltas. The
	// full pipeline (Load clamp + Apply clamp + zero floor) must not
	// let any dimension land negative — a negative weight inverts
	// MCTS sign and produces catastrophic play.
	t.Cleanup(ClearActiveFeedback)

	dir := t.TempDir()

	// Build a HEX-edited shape: every dimension a huge negative delta.
	// We need to bypass the producer-side clamp; SaveHeimdallFeedback
	// doesn't clamp on save (only re-clamps on load), so this writes
	// the unsafe values to disk and tests the LOAD-side clamp.
	dims := []string{
		"board_presence", "card_advantage", "mana_advantage", "life_resource",
		"combo_proximity", "threat_exposure", "commander_progress", "graveyard_value",
		"drain_engine", "artifact_synergy", "enchantment_synergy",
		"opponent_graveyard_threat", "partner_synergy", "activation_tempo",
		"toolbox_breadth", "threat_trajectory", "stack_interaction",
		"planeswalker_progress", "exile_zone_assets", "stax_lock_progress",
	}
	deltas := make([]analytics.DimensionDelta, len(dims))
	for i, d := range dims {
		deltas[i] = analytics.DimensionDelta{Dimension: d, Delta: -999.0}
	}
	fb := &analytics.HeimdallFeedback{
		Provenance: "test:adversarial",
		Archetypes: []analytics.ArchetypeFeedback{
			{Archetype: ArchetypeAggro, SampleSize: 100, Deltas: deltas},
		},
	}
	if err := analytics.SaveHeimdallFeedback(dir, fb); err != nil {
		t.Fatalf("save adversarial: %v", err)
	}
	loaded, err := analytics.LoadHeimdallFeedback(dir)
	if err != nil {
		t.Fatalf("load adversarial: %v", err)
	}
	if err := SetActiveFeedback(loaded, PolicyDirect); err != nil {
		t.Fatalf("SetActive adversarial: %v", err)
	}

	w := DefaultWeightsForArchetype(ArchetypeAggro)
	arr := w.AsArray()
	for i, v := range arr {
		if v < 0 {
			t.Errorf("dimension %d (%s) went negative after adversarial overlay: %v",
				i, dims[i], v)
		}
	}
}
