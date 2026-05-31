package hat

import (
	"testing"
)

// TestStrategyProfile_ProtectionRatio pins the protection-ratio helper
// added in the wave-3 freya-hat integration audit (2026-05-30). The
// helper feeds protectionThreatScalar which adjusts ThreatExposure in
// rescaleWeights.
func TestStrategyProfile_ProtectionRatio(t *testing.T) {
	cases := []struct {
		name        string
		sp          *StrategyProfile
		want        float64
		description string
	}{
		{
			name:        "nil sp -> -1 sentinel",
			sp:          nil,
			want:        -1,
			description: "legacy / no-strategy hat — no signal",
		},
		{
			name:        "zero key pieces -> -1 sentinel (below min)",
			sp:          &StrategyProfile{ProtectedKeyPieces: 0, UnprotectedKeyPieces: 0},
			want:        -1,
			description: "non-combo deck with no key pieces — denominator zero",
		},
		{
			name:        "2 total key pieces -> -1 (below 3-piece min)",
			sp:          &StrategyProfile{ProtectedKeyPieces: 1, UnprotectedKeyPieces: 1},
			want:        -1,
			description: "2 pieces is too noisy — even 1 protected → 50%",
		},
		{
			name:        "3 total, 0 protected -> 0.0 (real signal)",
			sp:          &StrategyProfile{ProtectedKeyPieces: 0, UnprotectedKeyPieces: 3},
			want:        0.0,
			description: "3 unprotected key pieces — combo shell wide open",
		},
		{
			name:        "3 total, 3 protected -> 1.0",
			sp:          &StrategyProfile{ProtectedKeyPieces: 3, UnprotectedKeyPieces: 0},
			want:        1.0,
			description: "every key piece protected",
		},
		{
			name:        "4 total, 2 protected -> 0.5",
			sp:          &StrategyProfile{ProtectedKeyPieces: 2, UnprotectedKeyPieces: 2},
			want:        0.5,
			description: "half-and-half",
		},
		{
			name:        "10 total, 7 protected -> 0.7",
			sp:          &StrategyProfile{ProtectedKeyPieces: 7, UnprotectedKeyPieces: 3},
			want:        0.7,
			description: "heavily protected combo shell",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.sp.ProtectionRatio()
			if got != tc.want {
				t.Errorf("ProtectionRatio: want %v, got %v (%s)", tc.want, got, tc.description)
			}
		})
	}
}

// TestProtectionThreatScalar pins the bracket-tier mapping that
// protectionThreatScalar applies. Three regions: high protection ≥0.50
// → 0.85, low protection ≤0.25 → 1.15, middle / no-signal → 1.0.
func TestProtectionThreatScalar(t *testing.T) {
	cases := []struct {
		name   string
		sp     *StrategyProfile
		want   float64
		reason string
	}{
		{
			name:   "nil sp -> 1.0",
			sp:     nil,
			want:   1.0,
			reason: "no-strategy hat",
		},
		{
			name:   "non-combo deck (no key pieces) -> 1.0",
			sp:     &StrategyProfile{},
			want:   1.0,
			reason: "ratio=-1 sentinel passes through unscaled",
		},
		{
			name:   "fully protected (ratio 1.0) -> 0.85",
			sp:     &StrategyProfile{ProtectedKeyPieces: 5, UnprotectedKeyPieces: 0},
			want:   0.85,
			reason: "all 5 pieces wear protection — eval shouldn't panic",
		},
		{
			name:   "ratio exactly 0.50 -> 0.85 (high-protection boundary)",
			sp:     &StrategyProfile{ProtectedKeyPieces: 2, UnprotectedKeyPieces: 2},
			want:   0.85,
			reason: "0.50 is the inclusive high-protection threshold",
		},
		{
			name:   "ratio 0.40 (middle band) -> 1.0",
			sp:     &StrategyProfile{ProtectedKeyPieces: 2, UnprotectedKeyPieces: 3},
			want:   1.0,
			reason: "middle band 0.25 < r < 0.50 → no adjustment",
		},
		{
			name:   "ratio 0.30 (middle band) -> 1.0",
			sp:     &StrategyProfile{ProtectedKeyPieces: 3, UnprotectedKeyPieces: 7},
			want:   1.0,
			reason: "still in the middle band",
		},
		{
			name:   "ratio exactly 0.25 -> 1.15 (low-protection boundary)",
			sp:     &StrategyProfile{ProtectedKeyPieces: 1, UnprotectedKeyPieces: 3},
			want:   1.15,
			reason: "0.25 is the inclusive low-protection threshold",
		},
		{
			name:   "ratio 0.0 (no protection) -> 1.15",
			sp:     &StrategyProfile{ProtectedKeyPieces: 0, UnprotectedKeyPieces: 8},
			want:   1.15,
			reason: "wide-open combo shell — eval should worry about removal",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := protectionThreatScalar(tc.sp)
			if got != tc.want {
				t.Errorf("protectionThreatScalar: want %v, got %v (%s)", tc.want, got, tc.reason)
			}
		})
	}
}

// TestProtectionThreatScalar_ChangesDecisionDirection is the motivating
// invariant for the wave-3 integration: given the same baseline
// ThreatExposure weight, a deck with high protection density must end
// up with a LOWER scaled weight than a deck with low protection density.
// Pins that the signal actually shifts the decision-shaping output, not
// just that it computes a number.
func TestProtectionThreatScalar_ChangesDecisionDirection(t *testing.T) {
	baselineWeight := 1.0

	highProtection := &StrategyProfile{ProtectedKeyPieces: 6, UnprotectedKeyPieces: 2} // ratio 0.75
	lowProtection := &StrategyProfile{ProtectedKeyPieces: 1, UnprotectedKeyPieces: 7}  // ratio 0.125
	noSignal := &StrategyProfile{ProtectedKeyPieces: 1, UnprotectedKeyPieces: 1}       // ratio -1 (too few pieces)

	highScaled := baselineWeight * protectionThreatScalar(highProtection)
	lowScaled := baselineWeight * protectionThreatScalar(lowProtection)
	noSignalScaled := baselineWeight * protectionThreatScalar(noSignal)

	// Direction: low-protection deck weights threats HIGHER than high-
	// protection deck. The whole point of the signal — without this,
	// the wiring is dead.
	if !(lowScaled > noSignalScaled && noSignalScaled > highScaled) {
		t.Errorf("expected lowScaled > noSignalScaled > highScaled, got low=%v noSignal=%v high=%v",
			lowScaled, noSignalScaled, highScaled)
	}

	// Magnitudes: lowScaled should be ≥35% higher than highScaled
	// (1.15 / 0.85 = 1.35). Defends against accidentally compressing
	// the spread on a future refactor.
	spread := lowScaled / highScaled
	if spread < 1.35 {
		t.Errorf("low/high spread compressed to %.3fx (expected ≥1.35x)", spread)
	}

	// Symmetry around no-signal: high should be below 1.0, low should
	// be above 1.0, no-signal should be exactly 1.0. Pins the
	// "no-signal preserves baseline" contract.
	if highScaled >= 1.0 {
		t.Errorf("high-protection should reduce below baseline, got %v", highScaled)
	}
	if lowScaled <= 1.0 {
		t.Errorf("low-protection should raise above baseline, got %v", lowScaled)
	}
	if noSignalScaled != 1.0 {
		t.Errorf("no-signal should preserve baseline 1.0, got %v", noSignalScaled)
	}
}

// TestBuildFromStrategyJSON_ProtectionFieldsPlumbed pins that the new
// ProtectedKeyPieces / UnprotectedKeyPieces JSON fields flow through
// the load path onto StrategyProfile. Without this, Freya could emit
// the fields and the hat would silently drop them — a regression mode
// that wouldn't be caught by the unit tests above (which set the
// StrategyProfile directly).
func TestBuildFromStrategyJSON_ProtectionFieldsPlumbed(t *testing.T) {
	sj := &strategyFileJSON{
		Archetype:            "combo",
		ProtectedKeyPieces:   4,
		UnprotectedKeyPieces: 2,
	}
	sp := buildFromStrategyJSON(sj)
	if sp == nil {
		t.Fatal("expected StrategyProfile")
	}
	if sp.ProtectedKeyPieces != 4 {
		t.Errorf("ProtectedKeyPieces: want 4, got %d", sp.ProtectedKeyPieces)
	}
	if sp.UnprotectedKeyPieces != 2 {
		t.Errorf("UnprotectedKeyPieces: want 2, got %d", sp.UnprotectedKeyPieces)
	}
	// ProtectionRatio should produce 4/6 = 0.6667 from the loaded fields,
	// which puts the deck in the high-protection band (≥0.50) and the
	// scalar should resolve to 0.85x.
	want := 4.0 / 6.0
	if got := sp.ProtectionRatio(); got != want {
		t.Errorf("ProtectionRatio: want %v, got %v", want, got)
	}
	if got := protectionThreatScalar(sp); got != 0.85 {
		t.Errorf("protectionThreatScalar: want 0.85, got %v", got)
	}
}

// TestProtectionThreatScalar_AppliedInRescaleWeights verifies the
// helper is actually wired into rescaleWeights — protectionThreatScalar
// applies BEFORE stage/position scaling so those bumps ride on top of
// the protection-adjusted baseline. Direct end-to-end pin: a hat
// constructed with high-protection strategy must show a lower
// ThreatExposure weight than a hat with low-protection strategy when
// rescaleWeights runs on the same game state. Without this test, the
// helper could be defined and tested but silently disconnected from
// the integration site.
func TestProtectionThreatScalar_AppliedInRescaleWeights(t *testing.T) {
	highProtection := &StrategyProfile{
		Archetype:            "combo",
		ProtectedKeyPieces:   6,
		UnprotectedKeyPieces: 2, // ratio 0.75 → 0.85x
	}
	lowProtection := &StrategyProfile{
		Archetype:            "combo",
		ProtectedKeyPieces:   1,
		UnprotectedKeyPieces: 7, // ratio 0.125 → 1.15x
	}

	highEval := NewEvaluator(highProtection)
	lowEval := NewEvaluator(lowProtection)

	// Pin baseline weights match so the only delta is the scalar.
	if highEval.Weights.ThreatExposure != lowEval.Weights.ThreatExposure {
		t.Fatalf("baseline ThreatExposure should match across strategies: high=%v low=%v",
			highEval.Weights.ThreatExposure, lowEval.Weights.ThreatExposure)
	}
	baselineThreat := highEval.Weights.ThreatExposure

	// rescaleWeights needs a game state for stage/position scaling.
	// Use the same minimal 2-seat helper other evaluator-weight tests
	// use; we're comparing relative ThreatExposure between two
	// evaluators sharing the same stage, so stage scaling cancels out.
	gs := newTestGame(t, 2)
	gs.Turn = 5
	highWeights := highEval.rescaleWeights(gs, 0)
	lowWeights := lowEval.rescaleWeights(gs, 0)

	if highWeights.ThreatExposure >= lowWeights.ThreatExposure {
		t.Errorf("high-protection deck should have LOWER scaled ThreatExposure than low-protection: high=%v low=%v",
			highWeights.ThreatExposure, lowWeights.ThreatExposure)
	}

	// Pin the scalar applied is the expected one: high should be ~0.85x
	// baseline-adjusted, low should be ~1.15x baseline-adjusted, so
	// their ratio should be ~0.85/1.15 = 0.7391. Allowing a small
	// tolerance because stage/position scaling layers on top (same on
	// both sides, cancels in the ratio).
	if baselineThreat > 0 {
		actualRatio := highWeights.ThreatExposure / lowWeights.ThreatExposure
		expectedRatio := 0.85 / 1.15
		const tolerance = 0.001
		if actualRatio < expectedRatio-tolerance || actualRatio > expectedRatio+tolerance {
			t.Errorf("scaled ratio: want ~%.4f, got %.4f", expectedRatio, actualRatio)
		}
	}
}
