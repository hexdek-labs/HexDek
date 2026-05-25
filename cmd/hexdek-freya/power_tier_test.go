package main

import (
	"testing"
)

// TestPowerTierFor pins the S/A/B/C/D thresholds. Boundary points
// (70, 55, 38, 25) all land on the higher tier — boundaries are
// inclusive on the high side. Thresholds recalibrated from 75/60/40/25
// after the cross-deck aggregate (#341) showed the original bands
// produced only 3.3% S and 11.4% A — too thin on both elite tiers.
func TestPowerTierFor(t *testing.T) {
	cases := []struct {
		power int
		want  string
	}{
		// D band
		{0, "D"}, {10, "D"}, {24, "D"},
		// C band
		{25, "C"}, {30, "C"}, {37, "C"},
		// B band
		{38, "B"}, {45, "B"}, {54, "B"},
		// A band
		{55, "A"}, {60, "A"}, {69, "A"},
		// S band
		{70, "S"}, {85, "S"}, {100, "S"},
	}
	for _, tc := range cases {
		if got := PowerTierFor(tc.power); got != tc.want {
			t.Errorf("PowerTierFor(%d) = %q, want %q", tc.power, got, tc.want)
		}
	}
}

// TestPowerTierFor_BoundariesAreHighInclusive double-checks the exact
// boundary semantics so a refactor can't silently shift them off-by-one.
// 70 must be S (not A); 55 must be A; 38 must be B; 25 must be C; 24 D.
func TestPowerTierFor_BoundariesAreHighInclusive(t *testing.T) {
	pairs := [][2]any{
		{70, "S"}, {69, "A"},
		{55, "A"}, {54, "B"},
		{38, "B"}, {37, "C"},
		{25, "C"}, {24, "D"},
	}
	for _, p := range pairs {
		power, want := p[0].(int), p[1].(string)
		if got := PowerTierFor(power); got != want {
			t.Errorf("boundary check: PowerTierFor(%d) = %q, want %q", power, got, want)
		}
	}
}

// TestComputeCardPower_AttachesPowerTier verifies every CardPowerLevel
// gets its PowerTier set, and that the tier matches its Power.
func TestComputeCardPower_AttachesPowerTier(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Hi", CMC: 1},
		{Name: "Mid", CMC: 3},
		{Name: "Lo", CMC: 6},
	}
	assignments := []CardRoleAssignment{
		{Name: "Hi", Roles: []RoleTag{RoleCombo, RoleTutor, RoleDraw, RoleRamp}},
		{Name: "Mid", Roles: []RoleTag{RoleRemoval}},
		{Name: "Lo", Roles: []RoleTag{RoleUtility}},
	}
	report := &FreyaReport{
		Profiles: profiles,
		Roles:    &RoleAnalysis{Assignments: assignments, TotalCards: len(profiles)},
		WinLines: &WinLineAnalysis{WinLines: []WinLine{{Pieces: []string{"Hi"}}}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, report)

	for _, pl := range dp.CardPowerLevels {
		if pl.PowerTier == "" {
			t.Errorf("%s missing PowerTier", pl.Name)
		}
		if pl.PowerTier != PowerTierFor(pl.Power) {
			t.Errorf("%s tier=%q does not match power=%d (PowerTierFor=%q)",
				pl.Name, pl.PowerTier, pl.Power, PowerTierFor(pl.Power))
		}
	}
}

// TestComputeCardPower_PowerTierCountsSumToCards verifies the histogram
// totals match the non-land card count (no lost cards, no double-counts).
func TestComputeCardPower_PowerTierCountsSumToCards(t *testing.T) {
	profiles := []CardProfile{
		{Name: "A1", CMC: 1},
		{Name: "A2", CMC: 2},
		{Name: "A3", CMC: 3},
		{Name: "A4", CMC: 4},
		{Name: "A5", CMC: 5},
		{Name: "A6", CMC: 6},
		{Name: "Forest", CMC: 0, IsLand: true}, // excluded from power scoring
	}
	dp := &DeckProfile{}
	computeCardPower(dp, makePowerTestReport(profiles, nil))

	total := 0
	for _, t := range PowerTierOrder {
		total += dp.PowerTierCounts[t]
	}
	wantNonLand := 6
	if total != wantNonLand {
		t.Errorf("tier counts sum to %d, want %d (non-land card count)", total, wantNonLand)
	}
	if len(dp.CardPowerLevels) != wantNonLand {
		t.Errorf("CardPowerLevels len = %d, want %d", len(dp.CardPowerLevels), wantNonLand)
	}
}

// TestComputeCardPower_PowerTierCountsAllTiersSeeded verifies every
// tier key (S/A/B/C/D) is present in the map even when no cards land
// in that tier. Stable rendering needs all keys present.
func TestComputeCardPower_PowerTierCountsAllTiersSeeded(t *testing.T) {
	// One untagged CMC-3 card → ArchFit 0, CMCEff 14, Synergy 0 → power 14 → D tier
	profiles := []CardProfile{{Name: "Lone", CMC: 3}}
	dp := &DeckProfile{}
	computeCardPower(dp, makePowerTestReport(profiles, nil))

	for _, tier := range PowerTierOrder {
		if _, ok := dp.PowerTierCounts[tier]; !ok {
			t.Errorf("tier %q missing from PowerTierCounts (need pre-seeded for stable rendering)", tier)
		}
	}
	// Only one card → exactly one bucket non-zero, rest zero.
	nonZero := 0
	for _, t := range PowerTierOrder {
		if dp.PowerTierCounts[t] > 0 {
			nonZero++
		}
	}
	if nonZero != 1 {
		t.Errorf("1 card should populate exactly 1 tier, populated %d tiers: %v", nonZero, dp.PowerTierCounts)
	}
}

// TestComputeCardPower_TierAttachesToCardQuality verifies that the
// star/solid/cuttable lists carry PowerTier (not just Power) so the
// tier badge is renderable inline.
func TestComputeCardPower_TierAttachesToCardQuality(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Star Card", CMC: 1},
		{Name: "Cut Card", CMC: 6},
	}
	assignments := []CardRoleAssignment{
		{Name: "Star Card", Roles: []RoleTag{RoleCombo, RoleTutor, RoleDraw, RoleRamp}},
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

	for _, c := range dp.StarCards {
		if c.PowerTier == "" {
			t.Errorf("StarCard %s missing PowerTier", c.Name)
		}
		if c.PowerTier != PowerTierFor(c.Power) {
			t.Errorf("StarCard %s tier=%q mismatches power=%d", c.Name, c.PowerTier, c.Power)
		}
	}
	for _, c := range dp.CuttableCards {
		if c.PowerTier == "" {
			t.Errorf("CuttableCard %s missing PowerTier", c.Name)
		}
	}
}

// TestComputeCardPower_CasualDeckCanHaveZeroSTier verifies the
// "absolute bands, not percentile" design choice: a deck whose top
// cards score below 75 reports 0 S-tier cards (rather than promoting
// its top-scoring filler to S). This is the deck-buy-it pacing signal
// — "your top card is only A-tier, the deck is mid-power".
func TestComputeCardPower_CasualDeckCanHaveZeroSTier(t *testing.T) {
	// All cards untagged or single-role mid-CMC → no card reaches 75.
	profiles := []CardProfile{
		{Name: "Plain1", CMC: 3},
		{Name: "Plain2", CMC: 4},
		{Name: "Plain3", CMC: 5},
	}
	assignments := []CardRoleAssignment{
		{Name: "Plain1", Roles: []RoleTag{RoleRemoval}},
		{Name: "Plain2", Roles: []RoleTag{RoleRamp}},
		{Name: "Plain3", Roles: []RoleTag{RoleUtility}},
	}
	dp := &DeckProfile{}
	computeCardPower(dp, makePowerTestReport(profiles, assignments))

	if dp.PowerTierCounts["S"] != 0 {
		t.Errorf("casual deck with no 75+ cards should have 0 S, got %d", dp.PowerTierCounts["S"])
	}
}
