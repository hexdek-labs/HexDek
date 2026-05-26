package main

import (
	"strings"
	"testing"
)

// Pins the R60r2 hoser expansion: 10 new entries across 6 existing
// conditions plus 1 new condition (extra_turns) + Stranglehold.

// enchantmentHeavyReport builds a deck with 10+ enchantments so the
// existing enchantment_heavy threshold fires.
func enchantmentHeavyReport() *FreyaReport {
	r := &FreyaReport{}
	for i := 0; i < 12; i++ {
		r.Profiles = append(r.Profiles,
			CardProfile{Name: "Aura " + string(rune('A'+i)), TypeLine: "Enchantment"})
	}
	return r
}

// TestComputeThreatAssessment_EnchantmentHeavy_NewHosers asserts the
// R60r2 enchantress-expansion entries (Force of Vigor + Pernicious
// Deed) surface alongside the existing pair without displacing them.
func TestComputeThreatAssessment_EnchantmentHeavy_NewHosers(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Enchantress"}
	computeThreatAssessment(dp, enchantmentHeavyReport())
	for _, want := range []string{"Force of Vigor", "Pernicious Deed", "Aura Shards", "Back to Nature"} {
		if !vulnerableToContains(dp, want) {
			t.Errorf("enchantment_heavy missing %q — got %v", want, dp.VulnerableTo)
		}
	}
}

// artifactHeavyReport: 12 artifacts trips the artifact_heavy threshold.
func artifactHeavyReport() *FreyaReport {
	r := &FreyaReport{}
	for i := 0; i < 14; i++ {
		r.Profiles = append(r.Profiles,
			CardProfile{Name: "Artifact " + string(rune('A'+i)), TypeLine: "Artifact"})
	}
	return r
}

// TestComputeThreatAssessment_ArtifactHeavy_NewHosers asserts the
// R60r2 artifact-expansion entries surface alongside the existing trio.
func TestComputeThreatAssessment_ArtifactHeavy_NewHosers(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Artifacts"}
	computeThreatAssessment(dp, artifactHeavyReport())
	for _, want := range []string{"Null Rod", "Karn, the Great Creator", "Bane of Progress", "Collector Ouphe", "Stony Silence"} {
		if !vulnerableToContains(dp, want) {
			t.Errorf("artifact_heavy missing %q — got %v", want, dp.VulnerableTo)
		}
	}
}

// TestComputeThreatAssessment_Lifegain_TaintedRemedy asserts the
// lifegain-specific anti-engine hoser shows up for lifegain decks.
func TestComputeThreatAssessment_Lifegain_TaintedRemedy(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Lifegain"}
	computeThreatAssessment(dp, &FreyaReport{})
	if !vulnerableToContains(dp, "Tainted Remedy") {
		t.Errorf("lifegain missing Tainted Remedy — got %v", dp.VulnerableTo)
	}
	// Tainted Remedy is critical — must carry the inline tag.
	for _, v := range dp.VulnerableTo {
		if strings.HasPrefix(v, "Tainted Remedy") && !strings.Contains(v, "(critical)") {
			t.Errorf("Tainted Remedy should be tagged critical: %q", v)
		}
	}
}

// tokenHeavyReport: 8 token producers trips the token_heavy threshold.
func tokenHeavyReport() *FreyaReport {
	r := &FreyaReport{}
	for i := 0; i < 10; i++ {
		r.Profiles = append(r.Profiles, CardProfile{
			Name:     "Token Maker " + string(rune('A'+i)),
			TypeLine: "Creature",
			Produces: []ResourceType{ResToken},
		})
	}
	return r
}

// TestComputeThreatAssessment_TokenHeavy_Pyroclasm asserts the cheap
// sweep added by R60r2 surfaces for token decks.
func TestComputeThreatAssessment_TokenHeavy_Pyroclasm(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Tokens"}
	computeThreatAssessment(dp, tokenHeavyReport())
	if !vulnerableToContains(dp, "Pyroclasm") {
		t.Errorf("token_heavy missing Pyroclasm — got %v", dp.VulnerableTo)
	}
}

// TestComputeThreatAssessment_ComboHeavy_NewHosers asserts both new
// combo entries (Grafdigger's Cage, Pithing Needle) surface for a
// combo-tagged deck.
func TestComputeThreatAssessment_ComboHeavy_NewHosers(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	// 3+ infinites tip the combo_heavy detector via the second branch.
	// Roles must be non-nil for the detector to run; an empty
	// RoleAnalysis is enough (rolePct returns 0, falls through to the
	// TrueInfinites + Determined count branch).
	report := &FreyaReport{
		Roles: &RoleAnalysis{},
		TrueInfinites: []ComboResult{
			{Cards: []string{"A", "B"}},
			{Cards: []string{"C", "D"}},
			{Cards: []string{"E", "F"}},
		},
	}
	computeThreatAssessment(dp, report)
	for _, want := range []string{"Grafdigger's Cage", "Pithing Needle", "Rule of Law", "Drannith Magistrate"} {
		if !vulnerableToContains(dp, want) {
			t.Errorf("combo_heavy missing %q — got %v", want, dp.VulnerableTo)
		}
	}
}

// TestComputeThreatAssessment_Wheels_NarsetParter asserts the third
// wheels hoser (Narset, opposite-angle limit) surfaces alongside
// the existing Notion Thief / Hullbreacher pair.
func TestComputeThreatAssessment_Wheels_NarsetParter(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Wheels"}
	computeThreatAssessment(dp, &FreyaReport{})
	for _, want := range []string{"Narset, Parter of Veils", "Notion Thief", "Hullbreacher"} {
		if !vulnerableToContains(dp, want) {
			t.Errorf("wheels missing %q — got %v", want, dp.VulnerableTo)
		}
	}
}

// TestComputeThreatAssessment_ExtraTurns_ArchetypePath asserts the
// new extra_turns condition fires when the archetype string contains
// the phrase, and Stranglehold surfaces as the hoser.
func TestComputeThreatAssessment_ExtraTurns_ArchetypePath(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Extra Turns"}
	computeThreatAssessment(dp, &FreyaReport{})
	if !vulnerableToContains(dp, "Stranglehold") {
		t.Errorf("extra_turns (archetype path) missing Stranglehold — got %v", dp.VulnerableTo)
	}
}

// TestComputeThreatAssessment_ExtraTurns_CardCountPath asserts the
// 2+ extra-turn-spell threshold path also fires the condition for
// decks that don't have "extra turn" in the archetype name.
func TestComputeThreatAssessment_ExtraTurns_CardCountPath(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Control"} // no "extra turn" in name
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Time Warp", TypeLine: "Sorcery"},
			{Name: "Temporal Manipulation", TypeLine: "Sorcery"},
		},
	}
	computeThreatAssessment(dp, report)
	if !vulnerableToContains(dp, "Stranglehold") {
		t.Errorf("extra_turns (card-count path) missing Stranglehold — got %v", dp.VulnerableTo)
	}
}

// TestComputeThreatAssessment_ExtraTurns_BelowThreshold asserts a
// single extra-turn spell in an otherwise-non-extra-turns deck does
// NOT trigger the condition — incidental Time Warp inclusions are
// noise, not engine pieces.
func TestComputeThreatAssessment_ExtraTurns_BelowThreshold(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Control"}
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Time Warp", TypeLine: "Sorcery"},
		},
	}
	computeThreatAssessment(dp, report)
	if vulnerableToContains(dp, "Stranglehold") {
		t.Errorf("single-turn-spell deck should not trigger extra_turns — got %v",
			dp.VulnerableTo)
	}
}

// TestHoserDB_R60r2_NewEntriesPresent guards the explicit additions —
// if any future cleanup pass drops these by name, this test surfaces
// the regression at the unit level rather than as a downstream coverage
// gap.
func TestHoserDB_R60r2_NewEntriesPresent(t *testing.T) {
	want := []string{
		"Force of Vigor",
		"Pernicious Deed",
		"Null Rod",
		"Karn, the Great Creator",
		"Bane of Progress",
		"Tainted Remedy",
		"Pyroclasm",
		"Grafdigger's Cage",
		"Pithing Needle",
		"Narset, Parter of Veils",
		"Stranglehold",
	}
	got := map[string]bool{}
	for _, h := range hoserDB {
		got[h.Hoser] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("hoserDB missing R60r2 entry %q", w)
		}
	}
}
