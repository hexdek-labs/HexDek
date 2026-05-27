package hat

import "testing"

// strategy.json max_recursion_depth must flow onto StrategyProfile so
// the hat can see the chain-depth signal at runtime (Yggdrasil reads
// h.Strategy.MaxRecursionDepth for any downstream gating).
func TestBuildFromStrategyJSON_MaxRecursionDepthPlumbed(t *testing.T) {
	sj := &strategyFileJSON{
		Archetype:         ArchetypeReanimator,
		MaxRecursionDepth: "infinite",
	}
	sp := buildFromStrategyJSON(sj)
	if sp == nil {
		t.Fatal("expected StrategyProfile")
	}
	if sp.MaxRecursionDepth != "infinite" {
		t.Errorf("MaxRecursionDepth: got %q, want %q", sp.MaxRecursionDepth, "infinite")
	}
}

// strategy.json path: when Freya supplies Weights, the hat must NOT
// double-apply the recursion-depth boost (Freya already folded it in
// during ComputeEvalWeights). The Weights pointer goes through verbatim.
func TestBuildFromStrategyJSON_WeightsNotDoubleBoosted(t *testing.T) {
	sj := &strategyFileJSON{
		Archetype:         ArchetypeReanimator,
		MaxRecursionDepth: "infinite",
		Weights: &freyaEvalWeights{
			BoardPresence:     0.8,
			CardAdvantage:     0.9,
			ManaAdvantage:     0.7,
			LifeResource:      0.4,
			ComboProximity:    0.5,
			ThreatExposure:    0.6,
			CommanderProgress: 0.5,
			GraveyardValue:    1.7, // Freya already baked +0.5 here
		},
	}
	sp := buildFromStrategyJSON(sj)
	if sp == nil || sp.Weights == nil {
		t.Fatal("expected weights populated")
	}
	if got := sp.Weights.GraveyardValue; got < 1.69 || got > 1.71 {
		t.Errorf("GraveyardValue must pass through unchanged when Weights provided: got %.3f, want 1.7", got)
	}
}

// _freya.json fallback path: when Weights are absent, the hat must
// derive MaxRecursionDepth from the embedded ValueChains array AND
// apply the equivalent GraveyardValue boost via DefaultWeightsForArchetype.
// Strategy.json builder embeds the boost; fallback must match.
func TestBuildStrategyProfile_FallbackAppliesRecursionDepthBoost(t *testing.T) {
	fj := &freyaJSON{
		Archetype: &freyaArchetype{Primary: "Reanimator", Bracket: 4},
		ValueChains: []freyaValueChain{
			{Name: "shallow gy", RecursionDepth: "shallow"},
			{Name: "loop", RecursionDepth: "infinite"},
		},
	}
	sp := buildStrategyProfile(fj)
	if sp == nil {
		t.Fatal("expected StrategyProfile")
	}
	if sp.MaxRecursionDepth != "infinite" {
		t.Errorf("MaxRecursionDepth: got %q, want infinite (strongest tag wins)", sp.MaxRecursionDepth)
	}
	if sp.Weights == nil {
		t.Fatal("expected Weights populated by recursion-depth boost")
	}
	base := DefaultWeightsForArchetype("reanimator")
	if got := sp.Weights.GraveyardValue - base.GraveyardValue; got < 0.49 || got > 0.51 {
		t.Errorf("infinite-chain boost: got +%.3f, want +0.5 above %s baseline (%.2f → %.2f)",
			got, "reanimator", base.GraveyardValue, sp.Weights.GraveyardValue)
	}
}

// Fallback path with NO chains: MaxRecursionDepth stays empty and no
// Weights are synthesized (matches pre-r60 behavior — Aggro decks
// without value chains should not gain a graveyard boost).
func TestBuildStrategyProfile_FallbackNoChainsNoBoost(t *testing.T) {
	fj := &freyaJSON{
		Archetype: &freyaArchetype{Primary: "Aggro", Bracket: 2},
	}
	sp := buildStrategyProfile(fj)
	if sp == nil {
		t.Fatal("expected StrategyProfile")
	}
	if sp.MaxRecursionDepth != "" {
		t.Errorf("MaxRecursionDepth must stay empty when no chains: got %q", sp.MaxRecursionDepth)
	}
	if sp.Weights != nil {
		t.Errorf("Weights must stay nil when no recursion signal: got %+v", sp.Weights)
	}
}

// applyRecursionDepthBoost is a no-op when Weights are already set —
// the strategy.json path supplies Weights, and we must not stomp the
// already-correct value. Guard against accidental re-application.
func TestApplyRecursionDepthBoost_NoOpWhenWeightsAlreadySet(t *testing.T) {
	preset := DefaultWeightsForArchetype("midrange")
	preset.GraveyardValue = 99.0 // sentinel
	sp := &StrategyProfile{
		Archetype:         "midrange",
		MaxRecursionDepth: "infinite",
		Weights:           &preset,
	}
	applyRecursionDepthBoost(sp)
	if sp.Weights.GraveyardValue != 99.0 {
		t.Errorf("must not stomp pre-set Weights: got GraveyardValue=%.2f, want 99.0", sp.Weights.GraveyardValue)
	}
}

// Per-tier boost table: shallow/deep/infinite map to the same lift the
// freya-side ComputeEvalWeights applies. Pins the inter-package
// contract.
func TestApplyRecursionDepthBoost_BoostTable(t *testing.T) {
	cases := []struct {
		depth string
		want  float64
	}{
		{"infinite", 0.5},
		{"deep", 0.25},
		{"shallow", 0.1},
	}
	for _, tc := range cases {
		sp := &StrategyProfile{
			Archetype:         "midrange",
			MaxRecursionDepth: tc.depth,
		}
		applyRecursionDepthBoost(sp)
		if sp.Weights == nil {
			t.Errorf("%s: expected Weights synthesized", tc.depth)
			continue
		}
		base := DefaultWeightsForArchetype("midrange")
		got := sp.Weights.GraveyardValue - base.GraveyardValue
		if got < tc.want-0.01 || got > tc.want+0.01 {
			t.Errorf("%s: got +%.3f, want +%.2f", tc.depth, got, tc.want)
		}
	}
}
