package main

import (
	"strings"
	"testing"
)

// makeFlexTierReport builds the minimal FreyaReport scaffolding
// computeCardQualityTiers needs to surface FlexSlots: a Roles
// analysis with the role-tag assignments wired through, no win
// lines or value chains (so flex candidates aren't disqualified
// by those filters), and a non-nil WinLines/ValueChains for the
// pipeline to walk without nil deref.
func makeFlexTierReport(assignments []CardRoleAssignment, profiles []CardProfile) *FreyaReport {
	counts := map[RoleTag]int{}
	for _, a := range assignments {
		for _, r := range a.Roles {
			counts[r]++
		}
	}
	return &FreyaReport{
		TotalCards: 99,
		Profiles:   profiles,
		Roles: &RoleAnalysis{
			TotalCards:  99,
			RoleCounts:  counts,
			Assignments: assignments,
		},
		WinLines:    &WinLineAnalysis{},
		ValueChains: nil,
	}
}

// TestComputeCardQualityTiers_FlexCardSingleGenericRole pins that a
// card with exactly ONE generic utility role (Removal here) at low CMC
// lands in FlexSlots, not in SolidCards.
func TestComputeCardQualityTiers_FlexCardSingleGenericRole(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Swords to Plowshares", CMC: 1, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Swords to Plowshares", Roles: []RoleTag{RoleRemoval}},
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, makeFlexTierReport(assignments, profiles), nil)

	foundFlex := false
	for _, fs := range dp.FlexSlots {
		if fs.Name == "Swords to Plowshares" {
			foundFlex = true
			if fs.Tier != "flex" {
				t.Errorf("tier = %q, want flex", fs.Tier)
			}
			if !strings.Contains(fs.Reason, "swap for situational meta tech") {
				t.Errorf("reason should mention meta-tech swap: %q", fs.Reason)
			}
		}
	}
	if !foundFlex {
		t.Errorf("Swords to Plowshares (single Removal role at CMC 1) should be in FlexSlots, got: %+v", dp.FlexSlots)
	}
	// Also ensure it's NOT in SolidCards — disjoint tiers.
	for _, sc := range dp.SolidCards {
		if sc.Name == "Swords to Plowshares" {
			t.Errorf("Swords to Plowshares should be flex, not solid (saw in SolidCards too — disjoint invariant broken)")
		}
	}
}

// TestComputeCardQualityTiers_FlexExcludesWinLinePieces pins that a
// card that would otherwise qualify as flex (single Removal role) is
// NOT flagged flex if it's a win-line piece — the win-line membership
// signals the card is load-bearing for the gameplan.
func TestComputeCardQualityTiers_FlexExcludesWinLinePieces(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Swords to Plowshares", CMC: 1, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Swords to Plowshares", Roles: []RoleTag{RoleRemoval}},
	}
	report := makeFlexTierReport(assignments, profiles)
	report.WinLines = &WinLineAnalysis{
		WinLines: []WinLine{
			{Pieces: []string{"Swords to Plowshares", "Other Combo Piece"}},
		},
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)
	for _, fs := range dp.FlexSlots {
		if fs.Name == "Swords to Plowshares" {
			t.Errorf("win-line piece should NOT be flex, got %+v", fs)
		}
	}
}

// TestComputeCardQualityTiers_FlexExcludesMultiRole pins that a
// multi-role card (e.g. Removal + Draw) is NOT flex — it's pulling
// weight on two dimensions, which is the Solid (or Star) shape.
func TestComputeCardQualityTiers_FlexExcludesMultiRole(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Wear // Tear", CMC: 2, TypeLine: "Instant"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Wear // Tear", Roles: []RoleTag{RoleRemoval, RoleDraw}},
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, makeFlexTierReport(assignments, profiles), nil)
	for _, fs := range dp.FlexSlots {
		if fs.Name == "Wear // Tear" {
			t.Errorf("multi-role card should NOT be flex, got %+v", fs)
		}
	}
}

// TestComputeCardQualityTiers_FlexExcludesNonGenericRole pins that a
// role outside the generic utility set (Combo, Threat, Tutor, etc) is
// never flex — Combo pieces are not slot-swappable for meta tech.
func TestComputeCardQualityTiers_FlexExcludesNonGenericRole(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Demonic Tutor", CMC: 2, TypeLine: "Sorcery"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Demonic Tutor", Roles: []RoleTag{RoleTutor}},
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, makeFlexTierReport(assignments, profiles), nil)
	for _, fs := range dp.FlexSlots {
		if fs.Name == "Demonic Tutor" {
			t.Errorf("Tutor role is not a generic-flex role, must not be in FlexSlots, got %+v", fs)
		}
	}
}

// TestComputeCardQualityTiers_FlexCapAtFive pins the cap-at-5 ceiling
// on the FlexSlots list (same cap as Star/Solid/Cuttable). Eight
// candidate cards all qualifying as flex should produce exactly 5
// entries.
func TestComputeCardQualityTiers_FlexCapAtFive(t *testing.T) {
	names := []string{
		"Swords to Plowshares", "Path to Exile", "Generous Gift",
		"Beast Within", "Krosan Grip", "Anguished Unmaking",
		"Assassin's Trophy", "Vindicate",
	}
	var profiles []CardProfile
	var assignments []CardRoleAssignment
	for _, n := range names {
		profiles = append(profiles, CardProfile{Name: n, CMC: 2, TypeLine: "Instant"})
		assignments = append(assignments, CardRoleAssignment{Name: n, Roles: []RoleTag{RoleRemoval}})
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, makeFlexTierReport(assignments, profiles), nil)
	if len(dp.FlexSlots) != 5 {
		t.Errorf("FlexSlots cap is 5, got %d entries", len(dp.FlexSlots))
	}
}

// TestComputeCardQualityTiers_AllGenericFlexRolesAccepted pins that
// each of the five generic roles (Removal / Draw / Ramp / Protection
// / Utility) qualifies a card for flex, given the other filters pass.
func TestComputeCardQualityTiers_AllGenericFlexRolesAccepted(t *testing.T) {
	cases := []struct {
		name string
		role RoleTag
	}{
		{"Swords to Plowshares", RoleRemoval},
		{"Brainstorm", RoleDraw},
		{"Sol Ring", RoleRamp},
		{"Lightning Greaves", RoleProtection},
		{"Sensei's Divining Top", RoleUtility},
	}
	for _, tc := range cases {
		profiles := []CardProfile{{Name: tc.name, CMC: 2, TypeLine: "Instant"}}
		assignments := []CardRoleAssignment{{Name: tc.name, Roles: []RoleTag{tc.role}}}
		dp := &DeckProfile{}
		computeCardQualityTiers(dp, makeFlexTierReport(assignments, profiles), nil)
		found := false
		for _, fs := range dp.FlexSlots {
			if fs.Name == tc.name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s (single %s role) should qualify for flex, FlexSlots = %+v",
				tc.name, tc.role, dp.FlexSlots)
		}
	}
}

// TestComputeCardQualityTiers_FlexSolidDisjoint pins the structural
// invariant: no card appears in BOTH FlexSlots and SolidCards.
// Builds a deck with a mix of multi-role (→ solid) and single-
// generic-role (→ flex) cards, then asserts the two lists share no
// names.
func TestComputeCardQualityTiers_FlexSolidDisjoint(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Swords to Plowshares", CMC: 1, TypeLine: "Instant"},
		{Name: "Brainstorm", CMC: 1, TypeLine: "Instant"},
		{Name: "Wear // Tear", CMC: 2, TypeLine: "Instant"},
		{Name: "Mystic Remora", CMC: 1, TypeLine: "Enchantment"},
	}
	assignments := []CardRoleAssignment{
		{Name: "Swords to Plowshares", Roles: []RoleTag{RoleRemoval}},
		{Name: "Brainstorm", Roles: []RoleTag{RoleDraw}},
		// Multi-role — solid.
		{Name: "Wear // Tear", Roles: []RoleTag{RoleRemoval, RoleDraw}},
		// Multi-role + low-CMC efficiency bonus — likely star or solid.
		{Name: "Mystic Remora", Roles: []RoleTag{RoleDraw, RoleStax}},
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, makeFlexTierReport(assignments, profiles), nil)
	flexNames := map[string]bool{}
	for _, fs := range dp.FlexSlots {
		flexNames[fs.Name] = true
	}
	for _, sc := range dp.SolidCards {
		if flexNames[sc.Name] {
			t.Errorf("disjoint invariant violated: %q appears in both Solid and Flex", sc.Name)
		}
	}
}

// TestComputeCardQualityTiers_FlexReasonMentionsMetaTech pins the
// reason-string contract: the flex tier must explicitly tell builders
// these slots are swappable for meta tech, not just "okay-but-
// replaceable" like solid. This is the user-facing differentiation
// that justifies adding the tier.
func TestComputeCardQualityTiers_FlexReasonMentionsMetaTech(t *testing.T) {
	profiles := []CardProfile{{Name: "Sol Ring", CMC: 1, TypeLine: "Artifact"}}
	assignments := []CardRoleAssignment{{Name: "Sol Ring", Roles: []RoleTag{RoleRamp}}}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, makeFlexTierReport(assignments, profiles), nil)
	if len(dp.FlexSlots) == 0 {
		t.Fatal("expected at least one flex entry")
	}
	for _, want := range []string{"flex slot", "meta tech", "graveyard hate"} {
		if !strings.Contains(strings.ToLower(dp.FlexSlots[0].Reason), strings.ToLower(want)) {
			t.Errorf("flex reason missing %q: got %q", want, dp.FlexSlots[0].Reason)
		}
	}
}
