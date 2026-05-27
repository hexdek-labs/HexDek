package main

import (
	"strings"
	"testing"
)

// makeTierTestReport builds a FreyaReport stub for card-tier tests with
// custom card profiles, role assignments, and optional win-line / value-
// chain wiring. Score formula recap (see computeCardQualityTiers):
//
//	+1.0 per role tag
//	+3.0 if name is in a win line piece
//	+2.5 if name is a value-chain bridge card
//	+1.0 if name is in a chain step (non-bridge)
//	+1.5 if CMC<=2 AND >=2 roles
//	-2.0 if CMC>=5 AND single role is RoleUtility
//	-1.0 if CMC>=4 AND single role is RoleUtility (stacks with the
//	     CMC>=5 penalty above for a total of -3.0 at CMC>=5)
//
// Stars: score >= 3.0 (cap 5)
// Solid: 0.5 <= score < 3.0 AND (multi-role OR non-generic role OR
//
//	win-line/chain member) (cap 5)
//
// Flex: 0.0 <= score < 2.0 AND exactly one generic utility role
//
//	(Removal/Draw/Ramp/Protection/Utility) AND NOT win-line/chain
//	(cap 5). Disjoint from Solid — a single-generic-role card lands
//	here, not in Solid.
//
// Cuttable: bottom 5 with score <= 0
func makeTierTestReport(profiles []CardProfile, assignments []CardRoleAssignment) *FreyaReport {
	return &FreyaReport{
		Profiles: profiles,
		Roles: &RoleAnalysis{
			Assignments: assignments,
			TotalCards:  len(profiles),
		},
	}
}

// TestComputeCardQualityTiers_SolidTierWindow pins the score windows
// for each tier. Builds 7 cards spanning the score range so each tier
// has at least one representative, then asserts each lands in the
// correct bucket.
func TestComputeCardQualityTiers_SolidTierWindow(t *testing.T) {
	// Construct cards with predictable score outcomes:
	//   "Star Card"   CMC 2, 3 roles                  → 3.0 + 1.5 (low-CMC multi) = 4.5  → star
	//   "Solid Two"   CMC 3, 2 roles                  → 2.0                          → solid
	//   "Solid One"   CMC 3, 1 role                   → 1.0                          → solid
	//   "Filler"      CMC 3, 0 roles                  → 0.0                          → (neither: score==0 and not "score<=0 strict")
	//   "Cut Heavy"   CMC 5, [Utility] only           → 1.0 - 2.0 - 1.0 = -2.0       → cuttable
	//   "Cut Medium"  CMC 4, [Utility] only           → 1.0 - 1.0       = 0.0        → cuttable boundary (excluded since score>0 required is "score > 0 continue", so score==0 IS cuttable-eligible)
	//   "Solid Edge" CMC 3, 1 role                    → 1.0                          → solid
	profiles := []CardProfile{
		{Name: "Star Card", CMC: 2},
		{Name: "Solid Two", CMC: 3},
		{Name: "Solid One", CMC: 3},
		{Name: "Filler", CMC: 3},
		{Name: "Cut Heavy", CMC: 5},
		{Name: "Cut Medium", CMC: 4},
		{Name: "Solid Edge", CMC: 3},
	}
	assignments := []CardRoleAssignment{
		{Name: "Star Card", Roles: []RoleTag{RoleRamp, RoleDraw, RoleRemoval}},
		{Name: "Solid Two", Roles: []RoleTag{RoleRamp, RoleDraw}},
		{Name: "Solid One", Roles: []RoleTag{RoleRamp}},
		{Name: "Filler", Roles: []RoleTag{}},
		{Name: "Cut Heavy", Roles: []RoleTag{RoleUtility}},
		{Name: "Cut Medium", Roles: []RoleTag{RoleUtility}},
		{Name: "Solid Edge", Roles: []RoleTag{RoleRemoval}},
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	hasName := func(cards []CardQuality, name string) bool {
		for _, c := range cards {
			if c.Name == name {
				return true
			}
		}
		return false
	}

	if !hasName(dp.StarCards, "Star Card") {
		t.Errorf("Star Card missing from StarCards: %v", dp.StarCards)
	}
	if !hasName(dp.SolidCards, "Solid Two") {
		t.Errorf("Solid Two missing from SolidCards: %v", dp.SolidCards)
	}
	// R60: single-generic-role cards land in FlexSlots, not SolidCards.
	// "Solid One" (single RoleRamp) and "Solid Edge" (single RoleRemoval)
	// were pre-r60 in Solid; the disjoint flex/solid partition now
	// routes them to Flex.
	if !hasName(dp.FlexSlots, "Solid One") {
		t.Errorf("Solid One (single RoleRamp at CMC 3) should be in FlexSlots: solid=%v flex=%v", dp.SolidCards, dp.FlexSlots)
	}
	if !hasName(dp.FlexSlots, "Solid Edge") {
		t.Errorf("Solid Edge (single RoleRemoval at CMC 3) should be in FlexSlots: solid=%v flex=%v", dp.SolidCards, dp.FlexSlots)
	}
	if !hasName(dp.CuttableCards, "Cut Heavy") {
		t.Errorf("Cut Heavy missing from CuttableCards: %v", dp.CuttableCards)
	}

	// Cross-bucket exclusion: a card must never appear in two tiers at once.
	for _, sc := range dp.StarCards {
		if hasName(dp.SolidCards, sc.Name) {
			t.Errorf("%q appeared in both Star and Solid", sc.Name)
		}
		if hasName(dp.FlexSlots, sc.Name) {
			t.Errorf("%q appeared in both Star and Flex", sc.Name)
		}
		if hasName(dp.CuttableCards, sc.Name) {
			t.Errorf("%q appeared in both Star and Cuttable", sc.Name)
		}
	}
	for _, sc := range dp.SolidCards {
		if hasName(dp.FlexSlots, sc.Name) {
			t.Errorf("%q appeared in both Solid and Flex (disjoint invariant)", sc.Name)
		}
		if hasName(dp.CuttableCards, sc.Name) {
			t.Errorf("%q appeared in both Solid and Cuttable", sc.Name)
		}
	}
	for _, fs := range dp.FlexSlots {
		if hasName(dp.CuttableCards, fs.Name) {
			t.Errorf("%q appeared in both Flex and Cuttable", fs.Name)
		}
	}
}

// TestComputeCardQualityTiers_SolidTierTagging verifies every Solid
// card is emitted with Tier="solid" and a non-empty reason.
func TestComputeCardQualityTiers_SolidTierTagging(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Solid Two", CMC: 3},
		{Name: "Solid One", CMC: 3},
	}
	assignments := []CardRoleAssignment{
		{Name: "Solid Two", Roles: []RoleTag{RoleRamp, RoleDraw}},
		{Name: "Solid One", Roles: []RoleTag{RoleRamp}},
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	if len(dp.SolidCards) == 0 {
		t.Fatalf("expected SolidCards to be populated, got empty")
	}
	for _, c := range dp.SolidCards {
		if c.Tier != "solid" {
			t.Errorf("%q has Tier=%q, want %q", c.Name, c.Tier, "solid")
		}
		if c.Reason == "" {
			t.Errorf("%q has empty Reason — solid tier should always provide one", c.Name)
		}
	}
}

// TestComputeCardQualityTiers_SolidExcludesStars verifies the solid
// tier never duplicates a card already on the star list. Stars are
// chosen first and recorded in the starred set; solid scanning skips
// any starred name.
func TestComputeCardQualityTiers_SolidExcludesStars(t *testing.T) {
	// A win-line piece gets +3 win-line bonus, putting it well past
	// the star threshold. Without exclusion logic it would also fall
	// in the solid window (its base 0/1 score + 3.0 = 3+ which IS
	// star-only). This test is the regression guard against the
	// off-by-one where solid scanning forgets the starred set.
	profiles := []CardProfile{
		{Name: "Combo Piece A", CMC: 2},
		{Name: "Combo Piece B", CMC: 2},
	}
	assignments := []CardRoleAssignment{
		{Name: "Combo Piece A", Roles: []RoleTag{RoleCombo}},
		{Name: "Combo Piece B", Roles: []RoleTag{RoleCombo}},
	}
	report := &FreyaReport{
		Profiles: profiles,
		Roles: &RoleAnalysis{
			Assignments: assignments,
			TotalCards:  len(profiles),
		},
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Pieces: []string{"Combo Piece A", "Combo Piece B"}},
			},
		},
	}
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	// Both pieces should be stars (win-line bonus pushes them to 4+).
	if len(dp.StarCards) != 2 {
		t.Fatalf("expected 2 stars, got %d: %v", len(dp.StarCards), dp.StarCards)
	}
	for _, sc := range dp.StarCards {
		for _, mid := range dp.SolidCards {
			if sc.Name == mid.Name {
				t.Errorf("%q duplicated in both Star and Solid lists", sc.Name)
			}
		}
	}
}

// TestComputeCardQualityTiers_SolidCappedAt5 verifies the solid tier
// honors the same 5-card cap as stars and cuttables — keeps the section
// scannable in deck builder reports.
func TestComputeCardQualityTiers_SolidCappedAt5(t *testing.T) {
	// 8 single-role mid-CMC cards all score 1.0 → all eligible for solid.
	profiles := make([]CardProfile, 0, 8)
	assignments := make([]CardRoleAssignment, 0, 8)
	for i := 0; i < 8; i++ {
		name := "Solid Filler " + string(rune('A'+i))
		profiles = append(profiles, CardProfile{Name: name, CMC: 3})
		assignments = append(assignments, CardRoleAssignment{Name: name, Roles: []RoleTag{RoleRamp}})
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	if len(dp.SolidCards) > 5 {
		t.Errorf("SolidCards cap exceeded: %d emitted, max 5 allowed", len(dp.SolidCards))
	}
	if len(dp.SolidCards) == 0 {
		t.Errorf("expected non-empty SolidCards from 8 score-1.0 candidates")
	}
}

// TestComputeCardQualityTiers_SolidDefaultReason verifies that solid
// cards without a per-card reason field get a sensible default built
// from their role mix and CMC, so the report never shows a blank line.
func TestComputeCardQualityTiers_SolidDefaultReason(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Dual Role", CMC: 3},
		{Name: "Single Role", CMC: 3},
	}
	assignments := []CardRoleAssignment{
		{Name: "Dual Role", Roles: []RoleTag{RoleRamp, RoleDraw}},
		{Name: "Single Role", Roles: []RoleTag{RoleRamp}},
	}
	report := makeTierTestReport(profiles, assignments)
	dp := &DeckProfile{}
	computeCardQualityTiers(dp, report, nil)

	// Dual-role card stays in SolidCards (multi-role exits the flex
	// criteria), reason mentions both roles + CMC.
	solidReason := func(name string) string {
		for _, c := range dp.SolidCards {
			if c.Name == name {
				return c.Reason
			}
		}
		return ""
	}
	flexReason := func(name string) string {
		for _, c := range dp.FlexSlots {
			if c.Name == name {
				return c.Reason
			}
		}
		return ""
	}

	dualReason := solidReason("Dual Role")
	if !strings.Contains(dualReason, "Ramp") || !strings.Contains(dualReason, "Draw") {
		t.Errorf("Dual Role reason should mention both roles, got %q", dualReason)
	}
	if !strings.Contains(dualReason, "CMC 3") {
		t.Errorf("Dual Role reason should mention CMC, got %q", dualReason)
	}

	// Single-generic-role card now lands in FlexSlots (R60); its reason
	// must mention the role name and frame the slot as meta-tech-swap.
	singleReason := flexReason("Single Role")
	if !strings.Contains(strings.ToLower(singleReason), "ramp") {
		t.Errorf("Single Role flex reason should mention role, got %q", singleReason)
	}
	if !strings.Contains(singleReason, "flex slot") {
		t.Errorf("Single Role flex reason should flag flex-slot framing, got %q", singleReason)
	}
}
