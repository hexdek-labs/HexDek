package hat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// archetype_routing_voltron_r60_test.go — end-to-end audit of the
// Freya→hat archetype-tag routing path. CLAUDE.md documents that
// Freya provides per-deck MCTS weights to the hat and the hat ships
// 22 archetype profiles; this test verifies the wiring between them
// works for Voltron specifically (the loudest archetype in the
// corpus — CommanderProgress 2.0 vs midrange 0.7, the largest
// single-dim divergence in the archetypeWeights table).
//
// Routing path being tested:
//
//   1. Freya writes <deck>.strategy.json with `archetype: "voltron"`
//      (lowercased in cmd/hexdek-freya/main.go:saveStrategyJSON) +
//      `eval_weights: { ... 8 dims ... }`.
//   2. Hat's LoadStrategyFromFreya reads the file and calls
//      buildFromStrategyJSON.
//   3. buildFromStrategyJSON sets sp.Archetype = "voltron" and
//      calls DefaultWeightsForArchetype("voltron") for the
//      full 20-dim baseline, then overlays the 8 Freya-serialized
//      core dims on top of it.
//   4. NewEvaluator(sp) reads sp.Weights (non-nil after step 3) and
//      stamps the evaluator's e.Weights.
//   5. The hat's MCTS evaluator consults e.Weights at every Evaluate
//      call.
//
// The test cases pin each link in the chain plus the cross-archetype
// counterfactuals (midrange ≠ voltron; capital "Voltron" doesn't
// match because the routing requires lowercase normalization).

// voltronFreyaWeights mimics what Freya's ComputeEvalWeights emits
// for a Voltron-classified deck: the 8 core dims at Voltron-tuned
// values (matching cmd/hexdek-freya/deckprofile.go::defaultWeights
// ["voltron"]). The hat's overlay path will copy these onto the
// archetype baseline.
func voltronFreyaWeights() *freyaEvalWeights {
	return &freyaEvalWeights{
		BoardPresence:     0.8,
		CardAdvantage:     0.5,
		ManaAdvantage:     0.5,
		LifeResource:      0.6,
		ComboProximity:    0.2,
		ThreatExposure:    0.9,
		CommanderProgress: 2.0,
		GraveyardValue:    0.3,
	}
}

// -----------------------------------------------------------------------------
// 1. Direct archetype-string routing
// -----------------------------------------------------------------------------

func TestDefaultWeightsForArchetype_VoltronRoutesToVoltronProfile(t *testing.T) {
	got := DefaultWeightsForArchetype(ArchetypeVoltron)
	if got.CommanderProgress != 2.0 {
		t.Errorf("Voltron CommanderProgress: got %v, want 2.0 (the signature Voltron dim)",
			got.CommanderProgress)
	}
	if got.ArtifactSynergy < 1.0 {
		t.Errorf("Voltron ArtifactSynergy: got %v, want >= 1.0 (equipment IS the deck)",
			got.ArtifactSynergy)
	}
	if got.EnchantmentSynergy < 0.8 {
		t.Errorf("Voltron EnchantmentSynergy: got %v, want >= 0.8 (auras IS the deck)",
			got.EnchantmentSynergy)
	}
}

func TestDefaultWeightsForArchetype_LowercaseStringMatches(t *testing.T) {
	// Freya writes the archetype lowercase ("voltron" not "Voltron")
	// so the raw string MUST match the ArchetypeVoltron constant. If
	// either side ever changes case convention, this test fires.
	if ArchetypeVoltron != "voltron" {
		t.Fatalf("ArchetypeVoltron constant changed: %q != \"voltron\"", ArchetypeVoltron)
	}
	got := DefaultWeightsForArchetype("voltron")
	want := DefaultWeightsForArchetype(ArchetypeVoltron)
	if got != want {
		t.Errorf("\"voltron\" routes differently from ArchetypeVoltron constant — lowercase normalization broken")
	}
}

func TestDefaultWeightsForArchetype_CapitalVoltronFallsBack(t *testing.T) {
	// Counterfactual: if Freya ever stopped lowercasing the archetype
	// before writing strategy.json, hat would receive "Voltron" and
	// the map lookup would miss, falling back to midrange. This pins
	// that the case-sensitivity exists so the bug surface is visible.
	got := DefaultWeightsForArchetype("Voltron")
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if got != mid {
		t.Errorf("Capital \"Voltron\" must fall back to midrange (proves lowercase normalization is required upstream)")
	}
}

// -----------------------------------------------------------------------------
// 2. NewEvaluator routing — Voltron deck without sp.Weights gets
//    Voltron defaults (DefaultWeightsForArchetype fallback path)
// -----------------------------------------------------------------------------

func TestNewEvaluator_VoltronArchetypeNoFreyaWeightsUsesVoltronDefaults(t *testing.T) {
	// Voltron StrategyProfile with NO eval_weights from Freya (decks
	// without a Freya scan, or pre-r60 strategy.json without weights).
	sp := &StrategyProfile{Archetype: ArchetypeVoltron}
	e := NewEvaluator(sp)

	if e.Weights.CommanderProgress != 2.0 {
		t.Errorf("Voltron archetype + no Freya weights: CommanderProgress = %v, want 2.0 (DefaultWeightsForArchetype path broken)",
			e.Weights.CommanderProgress)
	}

	// Cross-check vs midrange: the same setup with Archetype: "midrange"
	// must produce a meaningfully lower CommanderProgress.
	midSP := &StrategyProfile{Archetype: ArchetypeMidrange}
	midE := NewEvaluator(midSP)
	if midE.Weights.CommanderProgress >= e.Weights.CommanderProgress {
		t.Errorf("midrange CommanderProgress (%v) should be LOWER than Voltron's (%v)",
			midE.Weights.CommanderProgress, e.Weights.CommanderProgress)
	}
	delta := e.Weights.CommanderProgress - midE.Weights.CommanderProgress
	if delta < 1.0 {
		t.Errorf("Voltron - midrange CommanderProgress delta = %v, want >= 1.0 (Voltron's signature divergence)",
			delta)
	}
}

func TestNewEvaluator_NilStrategyFallsBackToMidrange(t *testing.T) {
	// Pin the safety fallback: NewEvaluator(nil) must not panic and
	// must produce midrange weights so hat can run without strategy.
	e := NewEvaluator(nil)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if e.Weights != mid {
		t.Errorf("nil strategy: got non-midrange weights — graceful-degradation path broken")
	}
}

// -----------------------------------------------------------------------------
// 3. Freya-overlay path — sp.Weights non-nil overrides the 8 Freya dims
//    while preserving the archetype's 12 extra dims
// -----------------------------------------------------------------------------

func TestNewEvaluator_VoltronWithFreyaOverlayPreservesExtraDims(t *testing.T) {
	// Simulate what buildFromStrategyJSON does after parsing Freya's
	// strategy.json: start from the archetype baseline, overlay the 8
	// Freya core dims, ship the merged EvalWeights as sp.Weights.
	base := DefaultWeightsForArchetype(ArchetypeVoltron)
	fw := voltronFreyaWeights()
	base.BoardPresence = fw.BoardPresence
	base.CardAdvantage = fw.CardAdvantage
	base.ManaAdvantage = fw.ManaAdvantage
	base.LifeResource = fw.LifeResource
	base.ComboProximity = fw.ComboProximity
	base.ThreatExposure = fw.ThreatExposure
	base.CommanderProgress = fw.CommanderProgress
	base.GraveyardValue = fw.GraveyardValue

	sp := &StrategyProfile{Archetype: ArchetypeVoltron, Weights: &base}
	e := NewEvaluator(sp)

	// 8 Freya-overlay dims took: CommanderProgress comes from the
	// Freya value (which happens to match the Voltron baseline at 2.0).
	if e.Weights.CommanderProgress != 2.0 {
		t.Errorf("Freya CommanderProgress overlay: got %v, want 2.0", e.Weights.CommanderProgress)
	}

	// 12 extra dims preserved from the Voltron baseline (Freya
	// doesn't serialize these — they MUST come from
	// DefaultWeightsForArchetype, NOT be zeroed out).
	voltronBaseline := DefaultWeightsForArchetype(ArchetypeVoltron)
	if e.Weights.ArtifactSynergy != voltronBaseline.ArtifactSynergy {
		t.Errorf("ArtifactSynergy preservation broken: got %v, want %v (Voltron baseline)",
			e.Weights.ArtifactSynergy, voltronBaseline.ArtifactSynergy)
	}
	if e.Weights.EnchantmentSynergy != voltronBaseline.EnchantmentSynergy {
		t.Errorf("EnchantmentSynergy preservation broken: got %v, want %v",
			e.Weights.EnchantmentSynergy, voltronBaseline.EnchantmentSynergy)
	}
	if e.Weights.StackInteraction != voltronBaseline.StackInteraction {
		t.Errorf("StackInteraction preservation broken: got %v, want %v",
			e.Weights.StackInteraction, voltronBaseline.StackInteraction)
	}
}

// -----------------------------------------------------------------------------
// 4. End-to-end: write a fake voltron strategy.json, run
//    LoadStrategyFromFreya, build a hat, check the evaluator's weights
// -----------------------------------------------------------------------------

func TestLoadStrategyFromFreya_VoltronStrategyJSONRoutesEndToEnd(t *testing.T) {
	dir := t.TempDir()
	freyaDir := filepath.Join(dir, "freya")
	if err := os.MkdirAll(freyaDir, 0o755); err != nil {
		t.Fatalf("mkdir freya/: %v", err)
	}

	// Mock Freya's compact strategy.json output for a Voltron deck.
	sj := strategyFileJSON{
		Archetype:       "voltron", // lowercased, as Freya writes
		Bracket:         3,
		GameplanSummary: "Wyleth Voltron — commander damage with equipment + auras",
		Weights:         voltronFreyaWeights(),
		// Fill a few non-weights fields so the loader exercises the
		// realistic shape rather than a minimal stub.
		CommanderThemes: []string{"voltron", "equipment", "auras"},
		FinisherCards:   []string{"Embercleave", "Eldrazi Conscription"},
		StarCards:       []string{"Wyleth, Soul of Steel"},
	}
	raw, err := json.MarshalIndent(sj, "", "  ")
	if err != nil {
		t.Fatalf("marshal strategy.json fixture: %v", err)
	}
	// LoadStrategyFromFreya derives the strategy.json path from the
	// deck filename: <dir>/freya/<deck-basename>.strategy.json.
	deckPath := filepath.Join(dir, "wyleth_voltron.txt")
	if err := os.WriteFile(deckPath, []byte("dummy decklist\n"), 0o644); err != nil {
		t.Fatalf("write deck stub: %v", err)
	}
	stratPath := filepath.Join(freyaDir, "wyleth_voltron.strategy.json")
	if err := os.WriteFile(stratPath, raw, 0o644); err != nil {
		t.Fatalf("write strategy.json fixture: %v", err)
	}

	sp := LoadStrategyFromFreya(deckPath)
	if sp == nil {
		t.Fatal("LoadStrategyFromFreya returned nil — strategy.json discovery / parse broken")
	}
	if sp.Archetype != ArchetypeVoltron {
		t.Errorf("sp.Archetype = %q, want %q (Freya's lowercased value must survive the loader)",
			sp.Archetype, ArchetypeVoltron)
	}
	if sp.Weights == nil {
		t.Fatal("sp.Weights nil — Freya overlay path didn't fire (buildFromStrategyJSON: sj.Weights != nil block)")
	}
	if sp.Weights.CommanderProgress != 2.0 {
		t.Errorf("sp.Weights.CommanderProgress = %v, want 2.0 (Freya overlay didn't take)",
			sp.Weights.CommanderProgress)
	}

	// Now build a hat from this strategy and confirm the evaluator
	// inherits the Voltron weights. NewYggdrasilHat → NewEvaluator
	// (sp) → e.Weights = *sp.Weights.
	h := NewYggdrasilHat(sp, 0)
	if h == nil {
		t.Fatal("NewYggdrasilHat returned nil")
	}
	if h.Evaluator == nil {
		t.Fatal("hat.Evaluator nil — evaluator wiring broken")
	}
	if h.Evaluator.Weights.CommanderProgress != 2.0 {
		t.Errorf("hat evaluator CommanderProgress = %v, want 2.0 — Freya→hat routing broken end-to-end",
			h.Evaluator.Weights.CommanderProgress)
	}
	// And the 12 extra dims preserved from the Voltron archetype
	// baseline must survive too — they're Voltron-specific signals
	// the MCTS evaluator reads for equipment/aura decks.
	if h.Evaluator.Weights.ArtifactSynergy < 1.0 {
		t.Errorf("hat ArtifactSynergy = %v, want >= 1.0 (Voltron equipment signal lost across the wire)",
			h.Evaluator.Weights.ArtifactSynergy)
	}
}

// -----------------------------------------------------------------------------
// 5. Known gap: slashed Freya archetype names (e.g. "blink / flicker",
//    "combo / infinite", "ninjutsu / evasion", "discard / hand attack",
//    "theft / clone", "aggro / go wide", "pillowfort", "cycling",
//    "toxic", "vehicles", "damage redirect", "group slug") have no
//    hat archetypeWeights entry — they silently fall back to midrange
//    weights via the DefaultWeightsForArchetype default branch.
//
//    The 8 Freya-overlay dims STILL come through correctly when
//    sp.Weights != nil; only the 12 extra hat-specific dims
//    (ArtifactSynergy, EnchantmentSynergy, StackInteraction, ...)
//    use midrange values where they should use archetype-tuned ones.
//
//    This test PINS that gap so a future hat-side expansion (adding
//    ArchetypeBlinkFlicker, ArchetypeComboInfinite, etc.) flips this
//    test failing immediately and surfaces the routing work.
// -----------------------------------------------------------------------------

func TestDefaultWeightsForArchetype_FreyaSlashedNamesFallBackToMidrange_KnownGap(t *testing.T) {
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	gaps := []string{
		"combo / infinite",
		"aggro / go wide",
		"blink / flicker",
		"discard / hand attack",
		"theft / clone",
		"ninjutsu / evasion",
		// r60 Pillowfort eval-weight ENTRY in cmd/hexdek-freya
		// (PR #565) is NOT mirrored on the hat side — the Freya
		// overlay still ships the 8 core dims correctly, but the
		// 12 hat-extra dims use midrange instead of a pillowfort-
		// tuned set. Same gap applies to the other unmirrored
		// archetypes below.
		"pillowfort",
		"group slug",
		"cycling",
		"toxic",
		"vehicles",
		"damage redirect",
	}
	for _, arch := range gaps {
		t.Run(arch, func(t *testing.T) {
			got := DefaultWeightsForArchetype(arch)
			if got != mid {
				t.Errorf("Freya archetype %q now resolves to a non-midrange profile (%+v) — hat ArchetypeWeights gained an entry; remove %q from the known-gap list and add a routing test for the new profile",
					arch, got, arch)
			}
		})
	}
}
