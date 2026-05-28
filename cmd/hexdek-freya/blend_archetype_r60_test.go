package main

import (
	"math"
	"strings"
	"testing"
)

// TestApplyBlendDetection_CanonicalSplashFires pins the headline case:
// Aristocrats primary at 60% confidence with a Reanimator runner-up at
// distance 0.3 (fit 0.7) gets the "Aristocrats with Reanimator splash"
// blend label. Both gates clear — confidence < 0.7, fit > 0.5.
func TestApplyBlendDetection_CanonicalSplashFires(t *testing.T) {
	ac := &ArchetypeClassification{
		Primary:           "Aristocrats",
		PrimaryConfidence: 0.6,
		Secondary:         "Reanimator",
		SecondaryDistance: 0.3,
	}
	applyBlendDetection(ac)

	if !ac.IsBlend {
		t.Errorf("expected IsBlend=true for low conf + high fit (conf=0.6, fit=0.7)")
	}
	if ac.BlendLabel != "Aristocrats with Reanimator splash" {
		t.Errorf("BlendLabel = %q, want %q", ac.BlendLabel, "Aristocrats with Reanimator splash")
	}
	if math.Abs(ac.SecondaryFit-0.7) > 0.001 {
		t.Errorf("SecondaryFit = %.4f, want 0.7", ac.SecondaryFit)
	}
}

// TestApplyBlendDetection_HighConfidenceNoBlend verifies a strong
// primary call (confidence >= 0.7) does NOT fire the splash even when
// the secondary fits well. At high primary confidence the runner-up
// is just "the second-best fingerprint, which happened to be ok" —
// not a genuine blend.
func TestApplyBlendDetection_HighConfidenceNoBlend(t *testing.T) {
	ac := &ArchetypeClassification{
		Primary:           "Combo",
		PrimaryConfidence: 0.85, // strong primary
		Secondary:         "Storm",
		SecondaryDistance: 0.2, // fit = 0.8
	}
	applyBlendDetection(ac)

	if ac.IsBlend {
		t.Errorf("high primary confidence (0.85) should NOT fire blend even with strong secondary fit, got: %q", ac.BlendLabel)
	}
	// SecondaryFit must STILL be populated — the field is informational,
	// not gated by IsBlend.
	if math.Abs(ac.SecondaryFit-0.8) > 0.001 {
		t.Errorf("SecondaryFit should be computed regardless of blend, got %.4f", ac.SecondaryFit)
	}
}

// TestApplyBlendDetection_WeakSecondaryNoBlend verifies the "confused
// deck" guard: low primary confidence + weak secondary fit (fit <= 0.5)
// does NOT fire blend. This is the case where both archetypes scored
// badly and just happened to be close to each other — calling it a
// "splash" would be misleading.
func TestApplyBlendDetection_WeakSecondaryNoBlend(t *testing.T) {
	ac := &ArchetypeClassification{
		Primary:           "Midrange",
		PrimaryConfidence: 0.3,  // low — uncertain
		Secondary:         "Goodstuff",
		SecondaryDistance: 0.55, // fit = 0.45 (below floor)
	}
	applyBlendDetection(ac)

	if ac.IsBlend {
		t.Errorf("weak secondary fit (0.45 <= 0.5 floor) should NOT fire blend, got: %q", ac.BlendLabel)
	}
}

// TestApplyBlendDetection_NoSecondaryNoBlend verifies the nil-guard:
// when the upstream classifier didn't set a Secondary (runner-up was
// far outside the 25% margin), the blend detector is a no-op.
func TestApplyBlendDetection_NoSecondaryNoBlend(t *testing.T) {
	ac := &ArchetypeClassification{
		Primary:           "Stax",
		PrimaryConfidence: 0.4, // low conf but no secondary candidate
		Secondary:         "",
	}
	applyBlendDetection(ac)

	if ac.IsBlend {
		t.Errorf("empty Secondary must NOT fire blend")
	}
	if ac.SecondaryFit != 0 {
		t.Errorf("SecondaryFit should stay 0 when no secondary, got %.4f", ac.SecondaryFit)
	}
}

// TestApplyBlendDetection_ConfidenceCeilingBoundary pins the strict
// inequality at confidence < 0.7 (NOT <=). At exactly 0.7 the primary
// call is considered strong enough that splash framing is misleading.
func TestApplyBlendDetection_ConfidenceCeilingBoundary(t *testing.T) {
	// At exactly the ceiling — must NOT fire.
	ac := &ArchetypeClassification{
		Primary:           "Combo",
		PrimaryConfidence: 0.7,
		Secondary:         "Storm",
		SecondaryDistance: 0.2,
	}
	applyBlendDetection(ac)
	if ac.IsBlend {
		t.Errorf("confidence at exactly ceiling (0.7) must NOT fire blend")
	}

	// Just below the ceiling — must fire.
	ac2 := &ArchetypeClassification{
		Primary:           "Combo",
		PrimaryConfidence: 0.69,
		Secondary:         "Storm",
		SecondaryDistance: 0.2,
	}
	applyBlendDetection(ac2)
	if !ac2.IsBlend {
		t.Errorf("confidence just below ceiling (0.69) must fire blend")
	}
}

// TestApplyBlendDetection_FitFloorBoundary pins the strict inequality
// at fit > 0.5 (NOT >=). At exactly 0.5 the secondary is borderline
// and shouldn't qualify as a substantive blend.
func TestApplyBlendDetection_FitFloorBoundary(t *testing.T) {
	// At exactly the floor — must NOT fire.
	ac := &ArchetypeClassification{
		Primary:           "Aristocrats",
		PrimaryConfidence: 0.5,
		Secondary:         "Tokens",
		SecondaryDistance: 0.5, // fit = 0.5 exactly
	}
	applyBlendDetection(ac)
	if ac.IsBlend {
		t.Errorf("fit at exactly floor (0.5) must NOT fire blend")
	}

	// Just above the floor — must fire.
	ac2 := &ArchetypeClassification{
		Primary:           "Aristocrats",
		PrimaryConfidence: 0.5,
		Secondary:         "Tokens",
		SecondaryDistance: 0.49, // fit = 0.51
	}
	applyBlendDetection(ac2)
	if !ac2.IsBlend {
		t.Errorf("fit just above floor (0.51) must fire blend")
	}
}

// TestApplyBlendDetection_NegativeDistanceClamped verifies the
// defensive clamp: a (theoretically impossible) negative distance
// would compute fit > 1; the clamp pins it to [0, 1].
func TestApplyBlendDetection_FitClampToRange(t *testing.T) {
	// Distance > 1 → would yield negative fit; clamp to 0.
	ac := &ArchetypeClassification{
		Primary: "Combo", PrimaryConfidence: 0.5,
		Secondary: "Aggro", SecondaryDistance: 1.5,
	}
	applyBlendDetection(ac)
	if ac.SecondaryFit != 0 {
		t.Errorf("distance > 1 should clamp fit to 0, got %.4f", ac.SecondaryFit)
	}
	// Negative distance → would yield fit > 1; clamp to 1.
	ac2 := &ArchetypeClassification{
		Primary: "Combo", PrimaryConfidence: 0.5,
		Secondary: "Aggro", SecondaryDistance: -0.3,
	}
	applyBlendDetection(ac2)
	if ac2.SecondaryFit != 1 {
		t.Errorf("negative distance should clamp fit to 1, got %.4f", ac2.SecondaryFit)
	}
}

// TestApplyBlendDetection_LabelFormat pins the exact label format —
// the UI may rely on it for parsing or display, and a casual rewording
// would be a downstream-breaking change.
func TestApplyBlendDetection_LabelFormat(t *testing.T) {
	ac := &ArchetypeClassification{
		Primary:           "Lifegain",
		PrimaryConfidence: 0.4,
		Secondary:         "Tokens",
		SecondaryDistance: 0.3,
	}
	applyBlendDetection(ac)
	want := "Lifegain with Tokens splash"
	if ac.BlendLabel != want {
		t.Errorf("BlendLabel = %q, want %q", ac.BlendLabel, want)
	}
	if !strings.Contains(ac.BlendLabel, " with ") || !strings.HasSuffix(ac.BlendLabel, " splash") {
		t.Errorf("BlendLabel must use 'X with Y splash' format, got: %q", ac.BlendLabel)
	}
}

// TestBlendDetection_MirrorsToDeckProfile verifies the
// BlendLabel / IsBlend / SecondaryFit fields flow through to the
// DeckProfile via the archetype-mirror block in BuildDeckProfile. The
// UI reads from DeckProfile, not ArchetypeClassification directly, so
// the mirror is the load-bearing path.
func TestBlendDetection_MirrorsToDeckProfile(t *testing.T) {
	ac := &ArchetypeClassification{
		Primary:              "Aristocrats",
		PrimaryConfidence:    0.55,
		Secondary:            "Reanimator",
		SecondaryDistance:    0.3,
		MeasuredBracket:      3,
		MeasuredBracketLabel: "Upgraded",
	}
	applyBlendDetection(ac)
	if !ac.IsBlend {
		t.Fatalf("test fixture should produce a blend; got conf=%.2f fit=%.2f",
			ac.PrimaryConfidence, ac.SecondaryFit)
	}

	report := &FreyaReport{
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{},
			TotalCards: 99,
		},
		Archetype: ac,
	}
	dp := BuildDeckProfile(report, nil)
	if dp == nil {
		t.Fatalf("BuildDeckProfile returned nil")
	}
	if !dp.IsBlend {
		t.Errorf("dp.IsBlend not mirrored, got false")
	}
	if dp.BlendLabel != ac.BlendLabel {
		t.Errorf("dp.BlendLabel = %q, want %q", dp.BlendLabel, ac.BlendLabel)
	}
	if math.Abs(dp.SecondaryFit-ac.SecondaryFit) > 0.001 {
		t.Errorf("dp.SecondaryFit = %.4f, want %.4f", dp.SecondaryFit, ac.SecondaryFit)
	}
}

// TestBlendDetection_JSONRoundTrips verifies the JSON build path
// surfaces is_blend / blend_label / secondary_fit so downstream
// consumers (UI, integrations) actually see the signal.
func TestBlendDetection_JSONRoundTrips(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype:    "Aristocrats",
		SecondaryArchetype:  "Reanimator",
		ArchetypeConfidence: 0.55,
		SecondaryFit:        0.7,
		IsBlend:             true,
		BlendLabel:          "Aristocrats with Reanimator splash",
	}
	js := buildJSONDeckProfile(dp, &FreyaReport{})
	if js == nil {
		t.Fatalf("buildJSONDeckProfile returned nil")
	}
	if !js.IsBlend {
		t.Errorf("IsBlend not mirrored to JSON, got false")
	}
	if js.BlendLabel != dp.BlendLabel {
		t.Errorf("BlendLabel JSON mirror wrong: got %q want %q", js.BlendLabel, dp.BlendLabel)
	}
	if math.Abs(js.SecondaryFit-0.7) > 0.001 {
		t.Errorf("SecondaryFit JSON mirror wrong: got %.4f", js.SecondaryFit)
	}
}
