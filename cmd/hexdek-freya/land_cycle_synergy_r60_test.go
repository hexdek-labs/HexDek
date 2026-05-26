package main

import (
	"strings"
	"testing"
)

// makeCycleLand returns a CardProfile shaped like a dual-cycle land
// (Scattered Groves, Irrigated Farmland, Sheltered Thicket, Canyon
// Slough, Fetid Pools): IsLand+HasCycling, with the cycling-shaped
// Produces/Consumes pair that triggers the determined-loop detector.
func makeCycleLand(name string) CardProfile {
	return CardProfile{
		Name:       name,
		TypeLine:   "Land — Forest Plains",
		IsLand:     true,
		HasCycling: true,
		Produces:   []ResourceType{ResCard},
		Consumes:   []ResourceType{ResCard},
	}
}

// TestExtractLandCyclePairs_DualCycleLandsReclassified verifies the
// canonical Scattered Groves + Irrigated Farmland pair is pulled out
// of Determined into LandCycleSynergies with class=land_cycle_synergy
// and LoopType=land_cycle_synergy. This is the headline regression
// pin from 7174n1c's slow-fetch report.
func TestExtractLandCyclePairs_DualCycleLandsReclassified(t *testing.T) {
	groves := makeCycleLand("Scattered Groves")
	farmland := makeCycleLand("Irrigated Farmland")
	profiles := map[string]CardProfile{
		groves.Name:   groves,
		farmland.Name: farmland,
	}

	determined := []ComboResult{
		{
			Cards:    []string{"Scattered Groves", "Irrigated Farmland"},
			LoopType: "determined",
		},
	}

	kept, extracted := extractLandCyclePairs(determined, profiles)
	if len(kept) != 0 {
		t.Errorf("kept: want 0, got %d (%v)", len(kept), kept)
	}
	if len(extracted) != 1 {
		t.Fatalf("extracted: want 1, got %d (%v)", len(extracted), extracted)
	}
	got := extracted[0]
	if got.LoopType != "land_cycle_synergy" {
		t.Errorf("LoopType: want %q, got %q", "land_cycle_synergy", got.LoopType)
	}
	if got.Class != ComboClassLandCycleSynergy {
		t.Errorf("Class: want %q, got %q", ComboClassLandCycleSynergy, got.Class)
	}
	if !strings.Contains(got.Description, "Lands Matter") {
		t.Errorf("description should mention the gated archetypes, got %q", got.Description)
	}
}

// TestExtractLandCyclePairs_NonLandPairUntouched verifies that a
// pairing where only ONE side is a cycle land (e.g. Scattered Groves
// + Jo Grant — the cycle land closes a card↔card loop with a
// nonland draw creature) stays in Determined. The reclassification
// is strictly all-land-cycle.
func TestExtractLandCyclePairs_NonLandPairUntouched(t *testing.T) {
	groves := makeCycleLand("Scattered Groves")
	joGrant := CardProfile{
		Name:     "Jo Grant",
		TypeLine: "Legendary Creature — Human Companion",
		Produces: []ResourceType{ResCard},
		Consumes: []ResourceType{ResCard},
	}
	profiles := map[string]CardProfile{
		groves.Name:  groves,
		joGrant.Name: joGrant,
	}

	determined := []ComboResult{
		{
			Cards:    []string{"Scattered Groves", "Jo Grant"},
			LoopType: "determined",
		},
	}

	kept, extracted := extractLandCyclePairs(determined, profiles)
	if len(kept) != 1 {
		t.Errorf("kept: want 1, got %d", len(kept))
	}
	if len(extracted) != 0 {
		t.Errorf("extracted: want 0 (mixed-type pair is not all-land-cycle), got %d (%v)", len(extracted), extracted)
	}
}

// TestExtractLandCyclePairs_NonCyclingLandPairUntouched verifies that
// two plain dual lands (no cycling) that somehow close a resource
// cycle do NOT get reclassified — only lands with the Cycling keyword
// produce the discard-cost-draw-effect signature that this bucket
// exists to suppress.
func TestExtractLandCyclePairs_NonCyclingLandPairUntouched(t *testing.T) {
	plainsLand1 := CardProfile{Name: "Hallowed Fountain", TypeLine: "Land", IsLand: true, HasCycling: false}
	plainsLand2 := CardProfile{Name: "Tundra", TypeLine: "Land", IsLand: true, HasCycling: false}
	profiles := map[string]CardProfile{
		plainsLand1.Name: plainsLand1,
		plainsLand2.Name: plainsLand2,
	}
	determined := []ComboResult{
		{Cards: []string{"Hallowed Fountain", "Tundra"}, LoopType: "determined"},
	}
	kept, extracted := extractLandCyclePairs(determined, profiles)
	if len(kept) != 1 || len(extracted) != 0 {
		t.Errorf("non-cycling land pair must stay in determined: kept=%d extracted=%d", len(kept), len(extracted))
	}
}

// TestExtractLandCyclePairs_HasCyclingFlagSetByClassifier verifies
// the ClassifyCard helper sets HasCycling when the oracle text
// includes the canonical cycling-cost anchor. This is the upstream
// guarantee the reclassification relies on.
func TestExtractLandCyclePairs_HasCyclingFlagSetByClassifier(t *testing.T) {
	cases := []struct {
		name     string
		oracle   string
		typeLine string
		want     bool
	}{
		{
			name:     "Scattered Groves",
			oracle:   "({T}: Add {G} or {W}.) This land enters tapped. Cycling {2} ({2}, Discard this card: Draw a card.)",
			typeLine: "Land — Forest Plains",
			want:     true,
		},
		{
			name:     "Krosan Tusker",
			oracle:   "Cycling {2}{G} ({2}{G}, Discard this card: Draw a card.) When you cycle this card, you may search your library for a basic land card.",
			typeLine: "Creature — Beast",
			want:     true,
		},
		{
			name:     "Lonely Sandbar",
			oracle:   "This land enters tapped. {T}: Add {U}. Basic landcycling {2}",
			typeLine: "Land",
			want:     true,
		},
		{
			name:     "Plains",
			oracle:   "({T}: Add {W}.)",
			typeLine: "Basic Land — Plains",
			want:     false,
		},
		{
			name:     "Bountiful Promenade",
			oracle:   "Bountiful Promenade enters tapped unless two or more opponents are in the game.",
			typeLine: "Land",
			want:     false,
		},
	}
	for _, tc := range cases {
		p := ClassifyCard(tc.name, tc.oracle, tc.typeLine, "", 0, "")
		if p.HasCycling != tc.want {
			t.Errorf("%s: HasCycling = %v, want %v", tc.name, p.HasCycling, tc.want)
		}
	}
}

// TestEstimateBracket_LandCycleSynergyExcludedInNonLandsArchetype is
// the bracket-gating regression pin: a deck with land-cycle synergies
// in a Combo / Blink / Midrange archetype does NOT get the combo-line
// bracket lift. Mirrors the Blast from the Past Doctor Who precon —
// Scattered Groves + Irrigated Farmland flagged by the heuristic
// pair detector but the deck is fundamentally a flicker / value pile
// where the cycle is incidental fixing.
func TestEstimateBracket_LandCycleSynergyExcludedInNonLandsArchetype(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{RoleCounts: map[RoleTag]int{}},
		LandCycleSynergies: []ComboResult{
			{Cards: []string{"Scattered Groves", "Irrigated Farmland"}, Class: ComboClassLandCycleSynergy},
		},
	}
	// Zero true infinites and zero determined → ctx.comboCount = 0.
	ctx := &classifyContext{
		roleRatios: map[RoleTag]float64{},
		avgCMC:     3.4,
	}
	_, _, br := estimateMeasuredBracket(ctx, report, "Combo")
	var comboSig *BracketSignal
	var lcsSig *BracketSignal
	for i := range br.Signals {
		switch br.Signals[i].Name {
		case "Combo lines":
			comboSig = &br.Signals[i]
		case "Land-cycle synergy":
			lcsSig = &br.Signals[i]
		}
	}
	if comboSig != nil {
		t.Errorf("Combo lines should not score when only source is a land-cycle pair in Combo archetype, got %+v", comboSig)
	}
	if lcsSig == nil {
		t.Fatalf("expected Land-cycle synergy note when LandCycleSynergies present")
	}
	if !strings.Contains(lcsSig.Note, "excluded") {
		t.Errorf("LCS note should say excluded in non-lands archetype, got %q", lcsSig.Note)
	}
}

// TestEstimateBracket_LandCycleSynergyCountedInLandsMatter verifies
// the inverse: a Lands Matter / Reanimator / Selfmill deck DOES get
// the bracket lift from land-cycle synergies because in those shells
// the cycle is a deliberate wincon component (graveyard pipeline for
// Crucible / Excavator / Splendid Reclamation; targeted-discard outlet
// for reanimate targets; self-mill enabler for graveyard-size payoffs).
func TestEstimateBracket_LandCycleSynergyCountedInLandsMatter(t *testing.T) {
	for _, archetype := range []string{"Lands Matter", "Reanimator", "Selfmill"} {
		report := &FreyaReport{
			Roles: &RoleAnalysis{RoleCounts: map[RoleTag]int{}},
			LandCycleSynergies: []ComboResult{
				{Cards: []string{"Scattered Groves", "Irrigated Farmland"}, Class: ComboClassLandCycleSynergy},
				{Cards: []string{"Sheltered Thicket", "Canyon Slough"}, Class: ComboClassLandCycleSynergy},
			},
		}
		ctx := &classifyContext{
			roleRatios: map[RoleTag]float64{},
			avgCMC:     3.0,
		}
		_, _, br := estimateMeasuredBracket(ctx, report, archetype)
		var comboSig *BracketSignal
		var lcsSig *BracketSignal
		for i := range br.Signals {
			switch br.Signals[i].Name {
			case "Combo lines":
				comboSig = &br.Signals[i]
			case "Land-cycle synergy":
				lcsSig = &br.Signals[i]
			}
		}
		if comboSig == nil {
			t.Errorf("%s: expected Combo lines signal when 2 LCS pairs counted, got nil", archetype)
			continue
		}
		if comboSig.Contribution < 2 {
			t.Errorf("%s: expected combo-lines contribution >= 2 (2 LCS pairs ≥ 2-4 tier), got %+d", archetype, comboSig.Contribution)
		}
		if lcsSig == nil {
			t.Fatalf("%s: expected Land-cycle synergy note", archetype)
		}
		if !strings.Contains(lcsSig.Note, "counted toward combo lines") {
			t.Errorf("%s: LCS note should say counted, got %q", archetype, lcsSig.Note)
		}
	}
}

// TestComputeLandValueCluster_CrucibleExcavatorDetected pins the
// land_value synergy cluster. A Tatyova-style Lands Matter deck with
// the producer + amplifier + payoff chain (cycle-lands → Crucible /
// Excavator → landfall payoffs) should surface as a Land Value
// Package cluster with score scaling on chain completeness.
func TestComputeLandValueCluster_CrucibleExcavatorDetected(t *testing.T) {
	// 10 profiles minimum gate
	profiles := []CardProfile{
		// 3 cycle-lands → producers
		makeCycleLand("Scattered Groves"),
		makeCycleLand("Irrigated Farmland"),
		makeCycleLand("Sheltered Thicket"),
		// 2 amplifiers
		{Name: "Crucible of Worlds", TypeLine: "Artifact"},
		{Name: "Ramunap Excavator", TypeLine: "Creature — Naga"},
		// 2 payoffs (curated name match)
		{Name: "Tatyova, Benthic Druid", TypeLine: "Legendary Creature — Merfolk Druid"},
		{Name: "Aesi, Tyrant of Gyre Strait", TypeLine: "Legendary Creature — Serpent"},
		// Filler to clear the 10-profile gate
		{Name: "Plains", TypeLine: "Basic Land — Plains", IsLand: true},
		{Name: "Forest", TypeLine: "Basic Land — Forest", IsLand: true},
		{Name: "Island", TypeLine: "Basic Land — Island", IsLand: true},
	}
	report := &FreyaReport{Profiles: profiles}
	cluster := computeLandValueCluster(report)
	if cluster == nil {
		t.Fatalf("expected Land Value Package cluster, got nil")
	}
	if cluster.Theme != "land_value" {
		t.Errorf("Theme: want %q, got %q", "land_value", cluster.Theme)
	}
	if cluster.MemberCount < 7 {
		t.Errorf("MemberCount: want at least 7 (3 producers + 2 amps + 2 payoffs), got %d", cluster.MemberCount)
	}
	// All seven role-cards should appear in AllMembers.
	want := []string{"Crucible of Worlds", "Ramunap Excavator", "Tatyova, Benthic Druid", "Aesi, Tyrant of Gyre Strait"}
	got := strings.Join(cluster.AllMembers, "|")
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("AllMembers should contain %q, got %s", w, got)
		}
	}
	// Chain bonus: min(3 producers, 2 amps, 2 payoffs) = 2 → +6 from
	// chain alone, plus pair bonuses, must be well above the floor.
	if cluster.Score < 6 {
		t.Errorf("Score: want >= 6 from chain bonus alone, got %d", cluster.Score)
	}
}

// TestComputeLandValueCluster_NoPayoffSkipped verifies the "no payoff
// = no cluster" gate. A pile of cycle-lands and a Crucible without
// any landfall triggers / lands-in-graveyard payoffs is just fixing,
// not a wincon package — the bracket-gating note in archetype.go
// depends on this: if there's no payoff, no cluster surfaces.
func TestComputeLandValueCluster_NoPayoffSkipped(t *testing.T) {
	profiles := []CardProfile{
		makeCycleLand("Scattered Groves"),
		makeCycleLand("Irrigated Farmland"),
		makeCycleLand("Sheltered Thicket"),
		makeCycleLand("Canyon Slough"),
		{Name: "Crucible of Worlds", TypeLine: "Artifact"},
		{Name: "Plains", TypeLine: "Basic Land — Plains", IsLand: true},
		{Name: "Forest", TypeLine: "Basic Land — Forest", IsLand: true},
		{Name: "Island", TypeLine: "Basic Land — Island", IsLand: true},
		{Name: "Sol Ring", TypeLine: "Artifact"},
		{Name: "Arcane Signet", TypeLine: "Artifact"},
	}
	report := &FreyaReport{Profiles: profiles}
	cluster := computeLandValueCluster(report)
	if cluster != nil {
		t.Errorf("expected nil cluster when no payoff present, got %+v", cluster)
	}
}

// TestComputeLandValueCluster_TooSmallSkipped verifies the 4-member
// floor — a single cycle-land + Crucible + Tatyova is 3 cards, under
// the floor, and shouldn't surface.
func TestComputeLandValueCluster_TooSmallSkipped(t *testing.T) {
	profiles := []CardProfile{
		makeCycleLand("Scattered Groves"),
		{Name: "Crucible of Worlds", TypeLine: "Artifact"},
		{Name: "Tatyova, Benthic Druid", TypeLine: "Legendary Creature — Merfolk Druid"},
		// 10-profile floor for the report.Profiles size gate
		{Name: "Plains", TypeLine: "Basic Land — Plains", IsLand: true},
		{Name: "Forest", TypeLine: "Basic Land — Forest", IsLand: true},
		{Name: "Island", TypeLine: "Basic Land — Island", IsLand: true},
		{Name: "Sol Ring", TypeLine: "Artifact"},
		{Name: "Arcane Signet", TypeLine: "Artifact"},
		{Name: "Mountain", TypeLine: "Basic Land — Mountain", IsLand: true},
		{Name: "Swamp", TypeLine: "Basic Land — Swamp", IsLand: true},
	}
	report := &FreyaReport{Profiles: profiles}
	cluster := computeLandValueCluster(report)
	if cluster != nil {
		t.Errorf("expected nil cluster when under 4-member floor, got %+v", cluster)
	}
}
