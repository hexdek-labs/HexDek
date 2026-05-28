package hat

import (
	"math"
	"testing"
)

// baseWeightsForRouting returns a neutral baseline for the 7 dimensions
// the power-tier router touches. Every dimension starts at 1.0 so test
// assertions can read the multiplier directly off the post-routing
// weight (no need to math against an archetype-baked baseline).
func baseWeightsForRouting() *EvalWeights {
	return &EvalWeights{
		BoardPresence:     1.0,
		CardAdvantage:     1.0,
		ManaAdvantage:     1.0,
		LifeResource:      1.0,
		ComboProximity:    1.0,
		ThreatExposure:    1.0,
		CommanderProgress: 1.0,
		GraveyardValue:    1.0,
		DrainEngine:       1.0,
		ArtifactSynergy:   1.0,
		EnchantmentSynergy:      1.0,
		OpponentGraveyardThreat: 1.0,
		PartnerSynergy:    1.0,
		ActivationTempo:   1.0,
		ToolboxBreadth:    1.0,
		ThreatTrajectory:  1.0,
		StackInteraction:  1.0,
		PlaneswalkerProgress: 1.0,
		ExileZoneAssets:   1.0,
		StaxLockProgress:  1.0,
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// TestApplyPowerTierRouting_CEDHPrioritizesCombo verifies that at T5
// (cEDH), ComboProximity, StackInteraction, ActivationTempo, and
// ToolboxBreadth are multiplied UP while BoardPresence, Commander-
// Progress, and LifeResource are multiplied DOWN.
func TestApplyPowerTierRouting_CEDHPrioritizesCombo(t *testing.T) {
	sp := &StrategyProfile{
		PowerTier: 5,
		Weights:   baseWeightsForRouting(),
	}
	ApplyPowerTierRouting(sp)
	w := sp.Weights

	// race-shape dimensions should be amplified
	if !almostEqual(w.ComboProximity, 1.60) {
		t.Errorf("T5 ComboProximity: want 1.60, got %.4f", w.ComboProximity)
	}
	if !almostEqual(w.StackInteraction, 1.60) {
		t.Errorf("T5 StackInteraction: want 1.60, got %.4f", w.StackInteraction)
	}
	if !almostEqual(w.ActivationTempo, 1.40) {
		t.Errorf("T5 ActivationTempo: want 1.40, got %.4f", w.ActivationTempo)
	}
	if !almostEqual(w.ToolboxBreadth, 1.30) {
		t.Errorf("T5 ToolboxBreadth: want 1.30, got %.4f", w.ToolboxBreadth)
	}

	// board-shape dimensions should be reduced
	if !almostEqual(w.BoardPresence, 0.60) {
		t.Errorf("T5 BoardPresence: want 0.60, got %.4f", w.BoardPresence)
	}
	if !almostEqual(w.CommanderProgress, 0.65) {
		t.Errorf("T5 CommanderProgress: want 0.65, got %.4f", w.CommanderProgress)
	}
	if !almostEqual(w.LifeResource, 0.70) {
		t.Errorf("T5 LifeResource: want 0.70, got %.4f", w.LifeResource)
	}

	// untouched dimensions stay at 1.0
	if !almostEqual(w.CardAdvantage, 1.0) {
		t.Errorf("T5 CardAdvantage should be untouched, got %.4f", w.CardAdvantage)
	}
	if !almostEqual(w.GraveyardValue, 1.0) {
		t.Errorf("T5 GraveyardValue should be untouched, got %.4f", w.GraveyardValue)
	}
}

// TestApplyPowerTierRouting_CasualPrioritizesBoard verifies that at
// T1/T2 (Casual), BoardPresence and CommanderProgress are amplified
// while ComboProximity, StackInteraction, ActivationTempo, and
// ToolboxBreadth are reduced.
func TestApplyPowerTierRouting_CasualPrioritizesBoard(t *testing.T) {
	for _, tier := range []int{1, 2} {
		t.Run(tierName(tier), func(t *testing.T) {
			sp := &StrategyProfile{
				PowerTier: tier,
				Weights:   baseWeightsForRouting(),
			}
			ApplyPowerTierRouting(sp)
			w := sp.Weights

			if !almostEqual(w.BoardPresence, 1.40) {
				t.Errorf("T%d BoardPresence: want 1.40, got %.4f", tier, w.BoardPresence)
			}
			if !almostEqual(w.CommanderProgress, 1.40) {
				t.Errorf("T%d CommanderProgress: want 1.40, got %.4f", tier, w.CommanderProgress)
			}
			if !almostEqual(w.LifeResource, 1.20) {
				t.Errorf("T%d LifeResource: want 1.20, got %.4f", tier, w.LifeResource)
			}

			if !almostEqual(w.ComboProximity, 0.60) {
				t.Errorf("T%d ComboProximity: want 0.60, got %.4f", tier, w.ComboProximity)
			}
			if !almostEqual(w.StackInteraction, 0.50) {
				t.Errorf("T%d StackInteraction: want 0.50, got %.4f", tier, w.StackInteraction)
			}
			if !almostEqual(w.ActivationTempo, 0.70) {
				t.Errorf("T%d ActivationTempo: want 0.70, got %.4f", tier, w.ActivationTempo)
			}
			if !almostEqual(w.ToolboxBreadth, 0.70) {
				t.Errorf("T%d ToolboxBreadth: want 0.70, got %.4f", tier, w.ToolboxBreadth)
			}
		})
	}
}

// TestApplyPowerTierRouting_UpgradedPreconNeutral verifies that at T3
// (Upgraded Precon), no dimension is altered — T3 is the neutral
// midline where the hat applies its archetype-tuned weights without
// tier-based tilt.
func TestApplyPowerTierRouting_UpgradedPreconNeutral(t *testing.T) {
	sp := &StrategyProfile{
		PowerTier: 3,
		Weights:   baseWeightsForRouting(),
	}
	ApplyPowerTierRouting(sp)
	w := sp.Weights

	// Every touched dimension stays at 1.0
	for _, pair := range []struct {
		name  string
		value float64
	}{
		{"ComboProximity", w.ComboProximity},
		{"StackInteraction", w.StackInteraction},
		{"ActivationTempo", w.ActivationTempo},
		{"ToolboxBreadth", w.ToolboxBreadth},
		{"BoardPresence", w.BoardPresence},
		{"CommanderProgress", w.CommanderProgress},
		{"LifeResource", w.LifeResource},
	} {
		if !almostEqual(pair.value, 1.0) {
			t.Errorf("T3 %s: want 1.0 (neutral), got %.4f", pair.name, pair.value)
		}
	}
}

// TestApplyPowerTierRouting_HighPowerSoftTilt verifies that at T4
// (High Power), the tilt is in the cEDH direction but smaller in
// magnitude than the T5 tilt. Specifically: T4 multipliers should be
// strictly between 1.0 and the T5 multiplier on race-shape dimensions,
// and between the T5 multiplier and 1.0 on board-shape dimensions.
func TestApplyPowerTierRouting_HighPowerSoftTilt(t *testing.T) {
	sp := &StrategyProfile{
		PowerTier: 4,
		Weights:   baseWeightsForRouting(),
	}
	ApplyPowerTierRouting(sp)
	w := sp.Weights

	// Race-shape: T4 between 1.0 and T5 (which is e.g. 1.60 for combo)
	for _, c := range []struct {
		name    string
		got     float64
		t5Mult  float64
	}{
		{"ComboProximity", w.ComboProximity, 1.60},
		{"StackInteraction", w.StackInteraction, 1.60},
		{"ActivationTempo", w.ActivationTempo, 1.40},
		{"ToolboxBreadth", w.ToolboxBreadth, 1.30},
	} {
		if c.got <= 1.0 || c.got >= c.t5Mult {
			t.Errorf("T4 %s: want between 1.0 and %.2f (T5), got %.4f", c.name, c.t5Mult, c.got)
		}
	}

	// Board-shape: T4 between T5 (e.g. 0.60) and 1.0
	for _, c := range []struct {
		name   string
		got    float64
		t5Mult float64
	}{
		{"BoardPresence", w.BoardPresence, 0.60},
		{"CommanderProgress", w.CommanderProgress, 0.65},
		{"LifeResource", w.LifeResource, 0.70},
	} {
		if c.got >= 1.0 || c.got <= c.t5Mult {
			t.Errorf("T4 %s: want between %.2f (T5) and 1.0, got %.4f", c.name, c.t5Mult, c.got)
		}
	}
}

// TestApplyPowerTierRouting_NoOpCases verifies the no-op paths: nil
// sp, nil Weights, PowerTier 0 (unset), and out-of-band PowerTier
// values (negative, > 5).
func TestApplyPowerTierRouting_NoOpCases(t *testing.T) {
	// Nil sp shouldn't panic
	ApplyPowerTierRouting(nil)

	// nil Weights shouldn't panic
	sp := &StrategyProfile{PowerTier: 5}
	ApplyPowerTierRouting(sp)

	// PowerTier=0 (unset) should be a no-op
	sp2 := &StrategyProfile{PowerTier: 0, Weights: baseWeightsForRouting()}
	ApplyPowerTierRouting(sp2)
	if !almostEqual(sp2.Weights.ComboProximity, 1.0) {
		t.Errorf("PowerTier=0 should be no-op, ComboProximity=%.4f", sp2.Weights.ComboProximity)
	}
	if !almostEqual(sp2.Weights.BoardPresence, 1.0) {
		t.Errorf("PowerTier=0 should be no-op, BoardPresence=%.4f", sp2.Weights.BoardPresence)
	}

	// PowerTier=-1 should be a no-op
	sp3 := &StrategyProfile{PowerTier: -1, Weights: baseWeightsForRouting()}
	ApplyPowerTierRouting(sp3)
	if !almostEqual(sp3.Weights.ComboProximity, 1.0) {
		t.Errorf("PowerTier=-1 should be no-op, ComboProximity=%.4f", sp3.Weights.ComboProximity)
	}

	// PowerTier=99 should be a no-op
	sp4 := &StrategyProfile{PowerTier: 99, Weights: baseWeightsForRouting()}
	ApplyPowerTierRouting(sp4)
	if !almostEqual(sp4.Weights.BoardPresence, 1.0) {
		t.Errorf("PowerTier=99 should be no-op, BoardPresence=%.4f", sp4.Weights.BoardPresence)
	}
}

// TestApplyPowerTierRouting_LoaderWiring verifies that the loader
// integration actually invokes ApplyPowerTierRouting — round-trip a
// strategy JSON with cedh_power_tier=5 through buildFromStrategyJSON
// and assert the weights got the cEDH multipliers applied.
func TestApplyPowerTierRouting_LoaderWiring(t *testing.T) {
	sj := &strategyFileJSON{
		Archetype: "Combo",
		PowerTier: 5,
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
		t.Errorf("PowerTier not propagated through loader: want 5, got %d", sp.PowerTier)
	}
	if sp.Weights == nil {
		t.Fatal("Weights nil after loader")
	}
	// At T5 cEDH, ComboProximity should be 1.6× the base, allowing
	// for the Combo archetype baseline multiplied by the cEDH router.
	// The Combo archetype starts ComboProximity at 2.0; the wire value
	// 1.0 overrides that, then ×1.6 = 1.6. Same applies to other
	// dimensions.
	if !almostEqual(sp.Weights.ComboProximity, 1.6) {
		t.Errorf("loader+router: T5 ComboProximity want 1.6, got %.4f", sp.Weights.ComboProximity)
	}
	if !almostEqual(sp.Weights.BoardPresence, 0.6) {
		t.Errorf("loader+router: T5 BoardPresence want 0.6, got %.4f", sp.Weights.BoardPresence)
	}
}

func tierName(tier int) string {
	switch tier {
	case 1:
		return "T1_Casual"
	case 2:
		return "T2_Casual"
	case 3:
		return "T3_UpgradedPrecon"
	case 4:
		return "T4_HighPower"
	case 5:
		return "T5_cEDH"
	default:
		return "T?"
	}
}
