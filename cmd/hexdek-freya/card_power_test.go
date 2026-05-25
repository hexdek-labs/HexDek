package main

import (
	"sort"
	"testing"
)

// makePowerTestReport builds a FreyaReport stub for computeCardPower
// tests. WinLines / ValueChains / Finishers default empty; pass
// non-empty values to exercise the synergy-contribution component.
func makePowerTestReport(profiles []CardProfile, assignments []CardRoleAssignment) *FreyaReport {
	return &FreyaReport{
		Profiles: profiles,
		Roles: &RoleAnalysis{
			Assignments: assignments,
			TotalCards:  len(profiles),
		},
	}
}

// TestComputeCardPower_RangeAndSort verifies Power values stay in [0, 100]
// and the output slice is sorted high → low.
func TestComputeCardPower_RangeAndSort(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Cheap Multi", CMC: 1},
		{Name: "Mid Utility", CMC: 3},
		{Name: "Heavy Filler", CMC: 6},
	}
	assignments := []CardRoleAssignment{
		{Name: "Cheap Multi", Roles: []RoleTag{RoleRamp, RoleDraw}},
		{Name: "Mid Utility", Roles: []RoleTag{RoleUtility}},
		{Name: "Heavy Filler", Roles: []RoleTag{RoleUtility}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, makePowerTestReport(profiles, assignments))

	if len(dp.CardPowerLevels) != 3 {
		t.Fatalf("expected 3 power entries, got %d", len(dp.CardPowerLevels))
	}
	for _, pl := range dp.CardPowerLevels {
		if pl.Power < 0 || pl.Power > 100 {
			t.Errorf("%s: power %d outside [0, 100]", pl.Name, pl.Power)
		}
	}
	if !sort.SliceIsSorted(dp.CardPowerLevels, func(i, j int) bool {
		return dp.CardPowerLevels[i].Power >= dp.CardPowerLevels[j].Power
	}) {
		t.Errorf("CardPowerLevels not sorted descending: %+v", dp.CardPowerLevels)
	}
}

// TestComputeCardPower_ComponentsSum verifies the headline Power equals
// the sum of its three components (subject to [0, 100] clamp).
func TestComputeCardPower_ComponentsSum(t *testing.T) {
	profiles := []CardProfile{{Name: "Test Card", CMC: 2}}
	assignments := []CardRoleAssignment{
		{Name: "Test Card", Roles: []RoleTag{RoleRamp, RoleDraw}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, makePowerTestReport(profiles, assignments))

	if len(dp.CardPowerLevels) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(dp.CardPowerLevels))
	}
	pl := dp.CardPowerLevels[0]
	sum := pl.ArchetypeFit + pl.CMCEfficiency + pl.SynergyContribution
	if sum > 100 {
		sum = 100
	}
	if pl.Power != sum {
		t.Errorf("Power=%d does not match component sum (%d + %d + %d = %d)",
			pl.Power, pl.ArchetypeFit, pl.CMCEfficiency, pl.SynergyContribution, sum)
	}
}

// TestComputeCardPower_ArchetypeFit verifies archetype-fit recognizes
// fingerprint-aligned roles: a card with roles matching the Combo
// fingerprint's ratio map scores higher fit than the same card under
// a Stax fingerprint.
func TestComputeCardPower_ArchetypeFit(t *testing.T) {
	profiles := []CardProfile{{Name: "Tutor + Draw", CMC: 2}}
	assignments := []CardRoleAssignment{
		{Name: "Tutor + Draw", Roles: []RoleTag{RoleTutor, RoleDraw}},
	}

	dpCombo := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dpCombo, makePowerTestReport(profiles, assignments))
	dpStax := &DeckProfile{PrimaryArchetype: "Stax"}
	computeCardPower(dpStax, makePowerTestReport(profiles, assignments))

	if dpCombo.CardPowerLevels[0].ArchetypeFit <= dpStax.CardPowerLevels[0].ArchetypeFit {
		t.Errorf("Combo fit (%d) should exceed Stax fit (%d) for a Tutor+Draw card — Combo ratios weight Tutor 0.10 / Draw 0.12; Stax has neither",
			dpCombo.CardPowerLevels[0].ArchetypeFit, dpStax.CardPowerLevels[0].ArchetypeFit)
	}
}

// TestComputeCardPower_TaggedCardFloor verifies the archetype-fit floor:
// a tagged card whose role doesn't match the fingerprint still gets
// 10 fit points, NOT zero (it's playing some role).
func TestComputeCardPower_TaggedCardFloor(t *testing.T) {
	profiles := []CardProfile{{Name: "Off-Archetype Tagged", CMC: 3}}
	assignments := []CardRoleAssignment{
		// RoleStax not in the Combo fingerprint
		{Name: "Off-Archetype Tagged", Roles: []RoleTag{RoleStax}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, makePowerTestReport(profiles, assignments))

	if got := dp.CardPowerLevels[0].ArchetypeFit; got != 10 {
		t.Errorf("tagged off-archetype card should hit fit floor 10, got %d", got)
	}

	// Untagged card → 0 fit.
	profilesUntagged := []CardProfile{{Name: "Untagged", CMC: 3}}
	dpUntagged := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dpUntagged, makePowerTestReport(profilesUntagged, nil))
	if got := dpUntagged.CardPowerLevels[0].ArchetypeFit; got != 0 {
		t.Errorf("untagged card should score 0 fit, got %d", got)
	}
}

// TestComputeCardPower_CMCEfficiency verifies the CMC band curve.
func TestComputeCardPower_CMCEfficiency(t *testing.T) {
	cases := []struct {
		cmc      int
		minScore int // expected lower bound (no multi-role bonus)
	}{
		{0, 20},
		{1, 20},
		{2, 18},
		{3, 14},
		{4, 10},
		{5, 6},
		{6, 2},
		{8, 2},
	}
	for _, tc := range cases {
		profiles := []CardProfile{{Name: "Test", CMC: tc.cmc}}
		dp := &DeckProfile{}
		computeCardPower(dp, makePowerTestReport(profiles, nil))
		got := dp.CardPowerLevels[0].CMCEfficiency
		if got != tc.minScore {
			t.Errorf("CMC %d efficiency: want %d, got %d", tc.cmc, tc.minScore, got)
		}
	}
}

// TestComputeCardPower_CMCMultiRoleBonus verifies the +2 efficiency
// bonus when a CMC<=2 card carries 2+ role tags.
func TestComputeCardPower_CMCMultiRoleBonus(t *testing.T) {
	single := []CardProfile{{Name: "Single", CMC: 2}}
	singleAssign := []CardRoleAssignment{{Name: "Single", Roles: []RoleTag{RoleRamp}}}
	dpSingle := &DeckProfile{}
	computeCardPower(dpSingle, makePowerTestReport(single, singleAssign))

	multi := []CardProfile{{Name: "Multi", CMC: 2}}
	multiAssign := []CardRoleAssignment{{Name: "Multi", Roles: []RoleTag{RoleRamp, RoleDraw}}}
	dpMulti := &DeckProfile{}
	computeCardPower(dpMulti, makePowerTestReport(multi, multiAssign))

	if dpMulti.CardPowerLevels[0].CMCEfficiency != dpSingle.CardPowerLevels[0].CMCEfficiency+2 {
		t.Errorf("multi-role low-CMC bonus: want %d+2, got %d",
			dpSingle.CardPowerLevels[0].CMCEfficiency,
			dpMulti.CardPowerLevels[0].CMCEfficiency)
	}
}

// TestComputeCardPower_SynergyContribution verifies the synergy
// component recognizes wincon pieces, value-chain bridges, finishers,
// and cluster membership with the expected point values.
func TestComputeCardPower_SynergyContribution(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Wincon", CMC: 3},
		{Name: "Bridge", CMC: 3},
		{Name: "Step", CMC: 3},
		{Name: "Finisher", CMC: 3},
		{Name: "Cluster", CMC: 3},
		{Name: "Plain", CMC: 3},
	}
	report := &FreyaReport{
		Profiles: profiles,
		Roles:    &RoleAnalysis{TotalCards: len(profiles)},
		WinLines: &WinLineAnalysis{WinLines: []WinLine{{Pieces: []string{"Wincon"}}}},
		ValueChains: []ValueChain{{
			Steps:       []ValueChainStep{{Cards: []string{"Step"}}},
			BridgeCards: []string{"Bridge"},
		}},
		Finishers: []ComboResult{{Cards: []string{"Finisher"}}},
	}
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{{Cards: []string{"Cluster"}}},
	}
	computeCardPower(dp, report)

	byName := map[string]CardPowerLevel{}
	for _, pl := range dp.CardPowerLevels {
		byName[pl.Name] = pl
	}

	// Each card has 0 roles, so synergy = signal points only (no per-role floor).
	wantSyn := map[string]int{
		"Wincon":   25,
		"Bridge":   20,
		"Step":     10,
		"Finisher": 12,
		"Cluster":  6,
		"Plain":    0,
	}
	for name, want := range wantSyn {
		if got := byName[name].SynergyContribution; got != want {
			t.Errorf("%s synergy: want %d, got %d", name, want, got)
		}
	}
}

// TestComputeCardPower_DeadSlotPenalty verifies the CMC 5+ Utility-only
// penalty: a high-CMC card with no synergy and only a Utility tag
// loses 10 points from its synergy component (clamped to 0).
func TestComputeCardPower_DeadSlotPenalty(t *testing.T) {
	profiles := []CardProfile{{Name: "Dead Slot", CMC: 6}}
	assignments := []CardRoleAssignment{
		{Name: "Dead Slot", Roles: []RoleTag{RoleUtility}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, makePowerTestReport(profiles, assignments))

	pl := dp.CardPowerLevels[0]
	// Per-role floor would normally give 1 role × 2 = 2; the -10 penalty
	// drives synergy negative, clamped to 0.
	if pl.SynergyContribution != 0 {
		t.Errorf("CMC 6 utility-only dead slot: want synergy 0 (penalty clamped), got %d",
			pl.SynergyContribution)
	}
}

// TestComputeCardPower_AttachesToTiers verifies that running
// computeCardPower upstream of computeCardQualityTiers populates the
// Power field on each emitted star / solid / cuttable entry.
func TestComputeCardPower_AttachesToTiers(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Star Card", CMC: 2},
		{Name: "Solid Card", CMC: 3},
		{Name: "Cut Card", CMC: 6},
	}
	assignments := []CardRoleAssignment{
		{Name: "Star Card", Roles: []RoleTag{RoleTutor, RoleDraw, RoleCombo}},
		{Name: "Solid Card", Roles: []RoleTag{RoleRamp}},
		{Name: "Cut Card", Roles: []RoleTag{RoleUtility}},
	}
	report := &FreyaReport{
		Profiles: profiles,
		Roles:    &RoleAnalysis{Assignments: assignments, TotalCards: len(profiles)},
		WinLines: &WinLineAnalysis{WinLines: []WinLine{{Pieces: []string{"Star Card"}}}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, report)
	computeCardQualityTiers(dp, report, nil)

	if len(dp.StarCards) == 0 || dp.StarCards[0].Power == 0 {
		t.Errorf("expected Power attached to StarCards[0], got %+v", dp.StarCards)
	}
	for _, c := range dp.StarCards {
		if c.Power < 1 || c.Power > 100 {
			t.Errorf("StarCard %s Power %d outside [1, 100]", c.Name, c.Power)
		}
	}
	for _, c := range dp.SolidCards {
		if c.Power < 1 || c.Power > 100 {
			t.Errorf("SolidCard %s Power %d outside [1, 100]", c.Name, c.Power)
		}
	}
	for _, c := range dp.CuttableCards {
		if c.Power < 0 || c.Power > 100 {
			t.Errorf("CuttableCard %s Power %d outside [0, 100]", c.Name, c.Power)
		}
	}
}

// TestComputeCardPower_PowerOrderingMatchesIntuition spot-checks the
// final power ordering against intuition: an efficient wincon piece
// should outscore a CMC 6 single-utility filler.
func TestComputeCardPower_PowerOrderingMatchesIntuition(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Cheap Wincon", CMC: 1},
		{Name: "Heavy Filler", CMC: 6},
	}
	assignments := []CardRoleAssignment{
		{Name: "Cheap Wincon", Roles: []RoleTag{RoleCombo, RoleTutor}},
		{Name: "Heavy Filler", Roles: []RoleTag{RoleUtility}},
	}
	report := &FreyaReport{
		Profiles: profiles,
		Roles:    &RoleAnalysis{Assignments: assignments, TotalCards: 2},
		WinLines: &WinLineAnalysis{WinLines: []WinLine{{Pieces: []string{"Cheap Wincon"}}}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, report)

	if dp.CardPowerLevels[0].Name != "Cheap Wincon" {
		t.Errorf("Cheap Wincon should rank #1, got order: %v", powerOrder(dp.CardPowerLevels))
	}
	if dp.CardPowerLevels[0].Power-dp.CardPowerLevels[1].Power < 40 {
		t.Errorf("Wincon should outscore filler by 40+, gap = %d",
			dp.CardPowerLevels[0].Power-dp.CardPowerLevels[1].Power)
	}
}

func powerOrder(pls []CardPowerLevel) []string {
	out := make([]string, len(pls))
	for i, pl := range pls {
		out[i] = pl.Name
	}
	return out
}
