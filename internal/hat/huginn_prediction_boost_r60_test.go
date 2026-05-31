package hat

import (
	"testing"
)

// TestHuginnPredictionBoost pins the cardHeuristic boost helper that
// consumes StrategyProfile.HuginnPredictions. Worker D — Huginn 2.0
// freya integration (2026-05-31).
func TestHuginnPredictionBoost(t *testing.T) {
	card := newTestCardMinimal("Walking Ballista", []string{"creature"}, 0, nil)

	cases := []struct {
		name string
		sp   *StrategyProfile
		want float64
	}{
		{
			name: "nil sp -> 0",
			sp:   nil,
			want: 0,
		},
		{
			name: "empty predictions -> 0",
			sp:   &StrategyProfile{},
			want: 0,
		},
		{
			name: "non-member card -> 0",
			sp: &StrategyProfile{HuginnPredictions: []HuginnPrediction{
				{InstanceID: "p-1", Cards: []string{"Heliod, Sun-Crowned"}, Confidence: 0.9},
			}},
			want: 0,
		},
		{
			name: "single prediction conf 0.5 -> +0.05",
			sp: &StrategyProfile{HuginnPredictions: []HuginnPrediction{
				{InstanceID: "p-1", Cards: []string{"Walking Ballista"}, Confidence: 0.5},
			}},
			want: 0.05,
		},
		{
			name: "single prediction conf 1.0 -> +0.10",
			sp: &StrategyProfile{HuginnPredictions: []HuginnPrediction{
				{InstanceID: "p-1", Cards: []string{"Walking Ballista"}, Confidence: 1.0},
			}},
			want: 0.10,
		},
		{
			name: "single prediction conf 0.0 -> 0 (no signal)",
			sp: &StrategyProfile{HuginnPredictions: []HuginnPrediction{
				{InstanceID: "p-1", Cards: []string{"Walking Ballista"}, Confidence: 0.0},
			}},
			want: 0,
		},
		{
			name: "card appears in 2 predictions -> sums",
			sp: &StrategyProfile{HuginnPredictions: []HuginnPrediction{
				{InstanceID: "p-1", Cards: []string{"Walking Ballista", "Heliod, Sun-Crowned"}, Confidence: 0.8},
				{InstanceID: "p-2", Cards: []string{"Walking Ballista", "Triskelion"}, Confidence: 0.6},
			}},
			want: 0.08 + 0.06,
		},
		{
			name: "case-insensitive match",
			sp: &StrategyProfile{HuginnPredictions: []HuginnPrediction{
				{InstanceID: "p-1", Cards: []string{"WALKING BALLISTA"}, Confidence: 0.8},
			}},
			want: 0.08,
		},
		{
			name: "negative Confidence -> 0 (defensive — normalizeStrategyProfile should clamp at load, but bypass-construction in tests is possible)",
			sp: &StrategyProfile{HuginnPredictions: []HuginnPrediction{
				{InstanceID: "p-1", Cards: []string{"Walking Ballista"}, Confidence: -0.5},
			}},
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := huginnPredictionBoost(tc.sp, card)
			// Use a tolerance comparison — 0.10 * Confidence
			// can produce trailing float-imprecision (0.8 * 0.1
			// = 0.08000000000000002), which the strict-equality
			// check would false-positive on. 1e-9 is well below
			// any cardHeuristic threshold that matters.
			if diff := got - tc.want; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestHuginnPredictionBoost_CapsAt020 pins the cumulative cap. A
// candidate appearing in 4 high-confidence predictions would
// theoretically score 4 * 0.10 = 0.40; the +0.20 cap keeps the
// signal proportionate to the synergy-cluster cohesion boost (+0.25
// cap) and star-card bonus (+0.15) it composes with in cardHeuristic.
func TestHuginnPredictionBoost_CapsAt020(t *testing.T) {
	card := newTestCardMinimal("Walking Ballista", []string{"creature"}, 0, nil)
	sp := &StrategyProfile{HuginnPredictions: []HuginnPrediction{
		{InstanceID: "p-1", Cards: []string{"Walking Ballista"}, Confidence: 1.0}, // +0.10
		{InstanceID: "p-2", Cards: []string{"Walking Ballista"}, Confidence: 1.0}, // +0.10 (sum 0.20)
		{InstanceID: "p-3", Cards: []string{"Walking Ballista"}, Confidence: 1.0}, // +0.10 (would push to 0.30)
		{InstanceID: "p-4", Cards: []string{"Walking Ballista"}, Confidence: 1.0}, // +0.10 (would push to 0.40)
	}}
	if got := huginnPredictionBoost(sp, card); got != 0.20 {
		t.Errorf("want cap +0.20, got %v", got)
	}
}

// TestBuildFromStrategyJSON_HuginnPredictionsPlumbed pins JSON wire
// round-trip — strategyFileJSON's huginn_predictions array populates
// StrategyProfile.HuginnPredictions with all fields preserved.
// Defends against the load-side silently dropping the new field.
func TestBuildFromStrategyJSON_HuginnPredictionsPlumbed(t *testing.T) {
	sj := &strategyFileJSON{
		Archetype: "combo",
		HuginnPredictions: []freyaHuginnPrediction{
			{
				InstanceID: "deck-heliod-001",
				Cards:      []string{"Heliod, Sun-Crowned", "Walking Ballista"},
				Pattern:    "infinite-damage-loop",
				Confidence: 0.85,
			},
			{
				// Empty InstanceID should be skipped at load.
				InstanceID: "",
				Cards:      []string{"X"},
				Confidence: 0.5,
			},
			{
				// Empty Cards should be skipped at load.
				InstanceID: "deck-empty-002",
				Cards:      nil,
				Confidence: 0.5,
			},
			{
				InstanceID: "deck-recur-003",
				Cards:      []string{"Sun Titan", "Land Tax"},
				Pattern:    "land-recursion",
				Confidence: 0.45,
			},
		},
	}
	sp := buildFromStrategyJSON(sj)
	if sp == nil {
		t.Fatal("expected StrategyProfile")
	}
	if len(sp.HuginnPredictions) != 2 {
		t.Fatalf("want 2 predictions (degenerate ones skipped), got %d", len(sp.HuginnPredictions))
	}
	if sp.HuginnPredictions[0].InstanceID != "deck-heliod-001" {
		t.Errorf("pred[0].InstanceID: want deck-heliod-001, got %q", sp.HuginnPredictions[0].InstanceID)
	}
	if sp.HuginnPredictions[0].Pattern != "infinite-damage-loop" {
		t.Errorf("pred[0].Pattern: want infinite-damage-loop, got %q", sp.HuginnPredictions[0].Pattern)
	}
	if sp.HuginnPredictions[0].Confidence != 0.85 {
		t.Errorf("pred[0].Confidence: want 0.85, got %v", sp.HuginnPredictions[0].Confidence)
	}
	if len(sp.HuginnPredictions[0].Cards) != 2 {
		t.Errorf("pred[0].Cards: want 2 names, got %d", len(sp.HuginnPredictions[0].Cards))
	}
	if sp.HuginnPredictions[1].InstanceID != "deck-recur-003" {
		t.Errorf("pred[1].InstanceID: want deck-recur-003, got %q", sp.HuginnPredictions[1].InstanceID)
	}
}
