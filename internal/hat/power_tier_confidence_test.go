package hat

import "testing"

// TestApplyPowerTierRouting_ConfidenceBlend_FullCEDHAtHighConfidence
// verifies that at confidence=1.0 the full per-tier multiplier
// applies (regression of the pre-#717 behavior).
func TestApplyPowerTierRouting_ConfidenceBlend_FullCEDHAtHighConfidence(t *testing.T) {
	sp := &StrategyProfile{
		PowerTier:           5,
		PowerTierConfidence: 1.0,
		Weights:             baseWeightsForRouting(),
	}
	ApplyPowerTierRouting(sp)
	if !almostEqual(sp.Weights.ComboProximity, 1.60) {
		t.Errorf("T5 conf=1.0 ComboProximity: want 1.60, got %.4f", sp.Weights.ComboProximity)
	}
	if !almostEqual(sp.Weights.BoardPresence, 0.60) {
		t.Errorf("T5 conf=1.0 BoardPresence: want 0.60, got %.4f", sp.Weights.BoardPresence)
	}
}

// TestApplyPowerTierRouting_ConfidenceBlend_HalfwayBlend verifies that
// at confidence=0.5 the tilt is halved (effective multiplier = lerp
// between 1.0 and the raw multiplier).
func TestApplyPowerTierRouting_ConfidenceBlend_HalfwayBlend(t *testing.T) {
	sp := &StrategyProfile{
		PowerTier:           5,
		PowerTierConfidence: 0.5,
		Weights:             baseWeightsForRouting(),
	}
	ApplyPowerTierRouting(sp)
	// ComboProximity raw mult 1.60 at conf 0.5 → 1.0 + 0.5*(1.60-1.0)
	// = 1.0 + 0.30 = 1.30
	if !almostEqual(sp.Weights.ComboProximity, 1.30) {
		t.Errorf("T5 conf=0.5 ComboProximity: want 1.30, got %.4f", sp.Weights.ComboProximity)
	}
	// BoardPresence raw mult 0.60 at conf 0.5 → 1.0 + 0.5*(0.60-1.0)
	// = 1.0 - 0.20 = 0.80
	if !almostEqual(sp.Weights.BoardPresence, 0.80) {
		t.Errorf("T5 conf=0.5 BoardPresence: want 0.80, got %.4f", sp.Weights.BoardPresence)
	}
	// StackInteraction raw mult 1.60 at conf 0.5 → 1.30
	if !almostEqual(sp.Weights.StackInteraction, 1.30) {
		t.Errorf("T5 conf=0.5 StackInteraction: want 1.30, got %.4f", sp.Weights.StackInteraction)
	}
}

// TestApplyPowerTierRouting_ConfidenceBlend_ZeroConfidenceLegacy
// verifies that confidence=0.0 (Go zero — meaning "unset" on legacy
// reports) is treated as 1.0 to preserve pre-#717 routing behavior.
// This is the back-compat guarantee: legacy strategy.json files
// without the confidence field get the full tier tilt, not zero tilt.
func TestApplyPowerTierRouting_ConfidenceBlend_ZeroConfidenceLegacy(t *testing.T) {
	sp := &StrategyProfile{
		PowerTier:           5,
		PowerTierConfidence: 0.0, // unset / legacy
		Weights:             baseWeightsForRouting(),
	}
	ApplyPowerTierRouting(sp)
	if !almostEqual(sp.Weights.ComboProximity, 1.60) {
		t.Errorf("T5 conf=0 (legacy) ComboProximity: want 1.60 (full tilt), got %.4f", sp.Weights.ComboProximity)
	}
}

// TestApplyPowerTierRouting_ConfidenceBlend_BoundaryDeckSoftTilt
// validates the user's spec scenario: a deck at the T3/T4 boundary
// (low confidence) should get a softer tilt so MCTS samples both
// race and board nodes more evenly. Tier=4 confidence=0.55 →
// ComboProximity raw 1.25 → effective 1.0 + 0.55*0.25 = 1.1375.
func TestApplyPowerTierRouting_ConfidenceBlend_BoundaryDeckSoftTilt(t *testing.T) {
	sp := &StrategyProfile{
		PowerTier:           4,
		PowerTierConfidence: 0.55,
		Weights:             baseWeightsForRouting(),
	}
	ApplyPowerTierRouting(sp)
	want := 1.0 + 0.55*(1.25-1.0)
	if !almostEqual(sp.Weights.ComboProximity, want) {
		t.Errorf("T4 conf=0.55 boundary deck: want ComboProximity %.4f, got %.4f", want, sp.Weights.ComboProximity)
	}
	// BoardPresence raw 0.85 at conf 0.55 → 1.0 + 0.55*(0.85-1.0)
	// = 1.0 - 0.0825 = 0.9175 — much softer than the conf=1.0 value of 0.85.
	wantBP := 1.0 + 0.55*(0.85-1.0)
	if !almostEqual(sp.Weights.BoardPresence, wantBP) {
		t.Errorf("T4 conf=0.55 BoardPresence: want %.4f, got %.4f", wantBP, sp.Weights.BoardPresence)
	}
}

// TestApplyPowerTierRouting_ConfidenceBlend_LowConfidenceCEDHBlends
// tests the user's explicit scenario from the spec — a deck right at
// the B3/B4 boundary returning tier=B3 confidence=0.55 — applied to
// the routing layer. Low confidence at T1/T2 (casual) should soften
// the board-shape tilt so the deck isn't fully treated as casual.
func TestApplyPowerTierRouting_ConfidenceBlend_LowConfidenceCEDHBlends(t *testing.T) {
	sp := &StrategyProfile{
		PowerTier:           2,
		PowerTierConfidence: 0.55,
		Weights:             baseWeightsForRouting(),
	}
	ApplyPowerTierRouting(sp)
	// Casual T2 BoardPresence raw 1.40 at conf 0.55 → 1.0 + 0.55*0.40
	// = 1.22 (softer than full 1.40)
	want := 1.0 + 0.55*(1.40-1.0)
	if !almostEqual(sp.Weights.BoardPresence, want) {
		t.Errorf("T2 conf=0.55 BoardPresence: want %.4f, got %.4f", want, sp.Weights.BoardPresence)
	}
	// ComboProximity raw 0.60 at conf 0.55 → 1.0 + 0.55*(0.60-1.0)
	// = 1.0 - 0.22 = 0.78 (softer than full 0.60)
	wantCP := 1.0 + 0.55*(0.60-1.0)
	if !almostEqual(sp.Weights.ComboProximity, wantCP) {
		t.Errorf("T2 conf=0.55 ComboProximity: want %.4f, got %.4f", wantCP, sp.Weights.ComboProximity)
	}
}

// TestApplyPowerTierRouting_ConfidenceBlend_T3NeutralRegardlessOfConfidence
// verifies that T3 (Upgraded Precon) stays neutral regardless of
// confidence — the multiplier table returns nil for T3, so the
// confidence blend doesn't apply.
func TestApplyPowerTierRouting_ConfidenceBlend_T3NeutralRegardlessOfConfidence(t *testing.T) {
	for _, conf := range []float64{0.10, 0.50, 0.95, 1.0} {
		sp := &StrategyProfile{
			PowerTier:           3,
			PowerTierConfidence: conf,
			Weights:             baseWeightsForRouting(),
		}
		ApplyPowerTierRouting(sp)
		if !almostEqual(sp.Weights.ComboProximity, 1.0) ||
			!almostEqual(sp.Weights.BoardPresence, 1.0) {
			t.Errorf("T3 conf=%.2f: want all neutral, got Combo=%.4f Board=%.4f",
				conf, sp.Weights.ComboProximity, sp.Weights.BoardPresence)
		}
	}
}

// TestEffectiveConfidence verifies the [0,1] clamp + zero-legacy
// override.
func TestEffectiveConfidence(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0.0, 1.0},  // legacy: zero means "unset" → full tilt
		{-0.5, 1.0}, // negative also treated as unset
		{0.5, 0.5},
		{0.99, 0.99},
		{1.0, 1.0},
		{1.5, 1.0}, // clamp ceiling
		{99.0, 1.0},
	}
	for _, c := range cases {
		got := effectiveConfidence(c.in)
		if !almostEqual(got, c.want) {
			t.Errorf("effectiveConfidence(%.2f): want %.2f, got %.4f", c.in, c.want, got)
		}
	}
}

// TestBlendMultiplier_Direction verifies the lerp behavior on both
// directions (race-shape mult > 1, board-shape mult < 1).
func TestBlendMultiplier_Direction(t *testing.T) {
	cases := []struct {
		raw, conf, want float64
	}{
		{1.60, 1.0, 1.60},   // full conf — raw value
		{1.60, 0.0, 1.0},    // zero conf — collapse to neutral (effectiveConfidence override doesn't apply here)
		{1.60, 0.5, 1.30},   // halfway blend
		{0.60, 1.0, 0.60},   // full conf — raw value
		{0.60, 0.0, 1.0},    // zero conf — collapse to neutral
		{0.60, 0.5, 0.80},   // halfway blend
		{1.00, 0.5, 1.00},   // raw mult already neutral — blend is identity
	}
	for _, c := range cases {
		got := blendMultiplier(c.raw, c.conf)
		if !almostEqual(got, c.want) {
			t.Errorf("blendMultiplier(raw=%.2f conf=%.2f): want %.4f, got %.4f", c.raw, c.conf, c.want, got)
		}
	}
}

// TestApplyPowerTierRouting_ConfidenceLoaderWiring_BothFields
// verifies that buildFromStrategyJSON populates both PowerTier and
// PowerTierConfidence from the JSON, and that the loader-time
// routing applies the confidence-blended multiplier.
func TestApplyPowerTierRouting_ConfidenceLoaderWiring_BothFields(t *testing.T) {
	sj := &strategyFileJSON{
		Archetype:           "Combo",
		PowerTier:           5,
		PowerTierConfidence: 0.5,
		Weights: &freyaEvalWeights{
			BoardPresence:     1.0,
			CardAdvantage:     1.0,
			ManaAdvantage:     1.0,
			LifeResource:      1.0,
			ComboProximity:    1.0,
			ThreatExposure:    1.0,
			CommanderProgress: 1.0,
			GraveyardValue:    1.0,
		},
	}
	sp := buildFromStrategyJSON(sj)
	if sp.PowerTier != 5 {
		t.Errorf("PowerTier not propagated: want 5, got %d", sp.PowerTier)
	}
	if !almostEqual(sp.PowerTierConfidence, 0.5) {
		t.Errorf("PowerTierConfidence not propagated: want 0.5, got %.4f", sp.PowerTierConfidence)
	}
	// ComboProximity 1.0 base × blend(1.60, 0.5) = 1.0 × 1.30 = 1.30
	if !almostEqual(sp.Weights.ComboProximity, 1.30) {
		t.Errorf("loader+blend: ComboProximity at conf=0.5 want 1.30, got %.4f", sp.Weights.ComboProximity)
	}
}
