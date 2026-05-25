package main

import (
	"strings"
	"testing"
)

// TestBuildPowerExplanation_FormatShape verifies the canonical format:
// "TIER — signal1 + signal2 + ..." with the tier letter leading.
func TestBuildPowerExplanation_FormatShape(t *testing.T) {
	got := buildPowerExplanation(powerExplanationInputs{
		tier:             "S",
		cmc:              2,
		roles:            []RoleTag{RoleCombo, RoleTutor},
		primaryArchetype: "Combo",
		matchedFitRoles:  []string{"Combo", "Tutor"},
		multiLowCMC:      true,
		isWincon:         true,
	})
	if !strings.HasPrefix(got, "S — ") {
		t.Errorf("explanation should lead with tier letter and em-dash: got %q", got)
	}
	if !strings.Contains(got, " + ") {
		t.Errorf("multi-signal explanation should join with ' + ': got %q", got)
	}
	if !strings.Contains(got, "wincon piece") {
		t.Errorf("wincon signal missing: %q", got)
	}
	if !strings.Contains(got, "2-role at CMC 2") {
		t.Errorf("multi-role + CMC signal missing: %q", got)
	}
	if !strings.Contains(got, "Combo fit") {
		t.Errorf("archetype-fit signal missing: %q", got)
	}
}

// TestBuildPowerExplanation_TopSynergyOnly verifies that the synergy
// section emits only the SINGLE highest-priority membership, not all
// of them (keeps the line scannable). Wincon outranks bridge outranks
// finisher outranks step outranks cluster.
func TestBuildPowerExplanation_TopSynergyOnly(t *testing.T) {
	got := buildPowerExplanation(powerExplanationInputs{
		tier:       "S",
		cmc:        2,
		roles:      []RoleTag{RoleCombo},
		isWincon:   true,
		isBridge:   true,
		isFinisher: true,
		isCluster:  true,
	})
	// All four synergy bools set — only "wincon piece" should appear.
	if !strings.Contains(got, "wincon piece") {
		t.Errorf("wincon (highest priority) should win: %q", got)
	}
	for _, lower := range []string{"value-chain bridge", "finisher", "cluster member"} {
		if strings.Contains(got, lower) {
			t.Errorf("lower-priority synergy %q should not appear when wincon present: %q", lower, got)
		}
	}
}

// TestBuildPowerExplanation_OffArchetypeFraming verifies tagged cards
// whose roles don't match the deck's fingerprint surface as
// "off-archetype" (not "fit:none" or silence).
func TestBuildPowerExplanation_OffArchetypeFraming(t *testing.T) {
	got := buildPowerExplanation(powerExplanationInputs{
		tier:             "C",
		cmc:              3,
		roles:            []RoleTag{RoleRemoval},
		primaryArchetype: "Combo",
		matchedFitRoles:  nil, // Removal doesn't match Combo fingerprint
		fitFloorHit:      true,
	})
	if !strings.Contains(got, "off-archetype") {
		t.Errorf("tagged-but-no-fingerprint-match should surface 'off-archetype': %q", got)
	}
}

// TestBuildPowerExplanation_UntaggedFraming verifies that a card with
// zero role tags reads as "untagged" rather than blank.
func TestBuildPowerExplanation_UntaggedFraming(t *testing.T) {
	got := buildPowerExplanation(powerExplanationInputs{
		tier:  "D",
		cmc:   4,
		roles: nil,
	})
	if !strings.Contains(got, "untagged") {
		t.Errorf("zero-role card should read 'untagged': %q", got)
	}
}

// TestBuildPowerExplanation_PenaltiesAppendedAtTail verifies that
// dead-slot and redundant-tutor penalties are surfaced at the end of
// the line so the why-line explains a D-tier verdict.
func TestBuildPowerExplanation_PenaltiesAppendedAtTail(t *testing.T) {
	got := buildPowerExplanation(powerExplanationInputs{
		tier:       "D",
		cmc:        6,
		roles:      []RoleTag{RoleUtility},
		isDeadSlot: true,
	})
	if !strings.Contains(got, "dead slot") {
		t.Errorf("dead-slot penalty missing: %q", got)
	}
	// "dead slot" should appear AFTER the CMC/role signals.
	idxDead := strings.Index(got, "dead slot")
	idxCMC := strings.Index(got, "CMC 6")
	if idxCMC == -1 || idxDead < idxCMC {
		t.Errorf("dead-slot should appear after CMC signal, got: %q", got)
	}

	gotTutor := buildPowerExplanation(powerExplanationInputs{
		tier:             "C",
		cmc:              5,
		roles:            []RoleTag{RoleTutor},
		isRedundantTutor: true,
	})
	if !strings.Contains(gotTutor, "redundant tutor") {
		t.Errorf("redundant-tutor penalty missing: %q", gotTutor)
	}
}

// TestBuildPowerExplanation_NeverEmpty verifies the defensive fallback:
// even with all-false signals and no roles, the explanation is at
// minimum the tier letter (never an empty string).
func TestBuildPowerExplanation_NeverEmpty(t *testing.T) {
	got := buildPowerExplanation(powerExplanationInputs{tier: "D"})
	if got == "" {
		t.Errorf("explanation must never be empty even with all-zero signals")
	}
	if !strings.HasPrefix(got, "D") {
		t.Errorf("explanation must start with tier letter, got %q", got)
	}
}

// TestComputeCardPower_AttachesExplanation verifies that after
// computeCardPower runs, every CardPowerLevel has a non-empty
// Explanation and that the leading tier letter matches PowerTier.
func TestComputeCardPower_AttachesExplanation(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Star", CMC: 1},
		{Name: "Mid", CMC: 3},
		{Name: "Cut", CMC: 6},
	}
	assignments := []CardRoleAssignment{
		{Name: "Star", Roles: []RoleTag{RoleCombo, RoleTutor, RoleDraw, RoleRamp}},
		{Name: "Mid", Roles: []RoleTag{RoleRemoval}},
		{Name: "Cut", Roles: []RoleTag{RoleUtility}},
	}
	report := &FreyaReport{
		Profiles: profiles,
		Roles:    &RoleAnalysis{Assignments: assignments, TotalCards: len(profiles)},
		WinLines: &WinLineAnalysis{WinLines: []WinLine{{Pieces: []string{"Star"}}}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, report)

	for _, pl := range dp.CardPowerLevels {
		if pl.Explanation == "" {
			t.Errorf("%s missing Explanation", pl.Name)
		}
		// Tier letter should lead the explanation and match PowerTier.
		want := pl.PowerTier + " — "
		if !strings.HasPrefix(pl.Explanation, want) && pl.Explanation != pl.PowerTier {
			t.Errorf("%s: explanation prefix mismatch — got %q, want it to start with %q",
				pl.Name, pl.Explanation, want)
		}
	}
}

// TestComputeCardPower_ExplanationFlowsToCardQuality verifies the
// PowerExplanation field is populated on each star/solid/cuttable
// CardQuality entry, so the report can render the why-line without
// re-querying CardPowerLevels.
func TestComputeCardPower_ExplanationFlowsToCardQuality(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Star", CMC: 1},
		{Name: "Cut", CMC: 6},
	}
	assignments := []CardRoleAssignment{
		{Name: "Star", Roles: []RoleTag{RoleCombo, RoleTutor, RoleDraw, RoleRamp}},
		{Name: "Cut", Roles: []RoleTag{RoleUtility}},
	}
	report := &FreyaReport{
		Profiles: profiles,
		Roles:    &RoleAnalysis{Assignments: assignments, TotalCards: len(profiles)},
		WinLines: &WinLineAnalysis{WinLines: []WinLine{{Pieces: []string{"Star"}}}},
	}
	dp := &DeckProfile{PrimaryArchetype: "Combo"}
	computeCardPower(dp, report)
	computeCardQualityTiers(dp, report, nil)

	for _, c := range dp.StarCards {
		if c.PowerExplanation == "" {
			t.Errorf("StarCard %s missing PowerExplanation", c.Name)
		}
	}
	for _, c := range dp.CuttableCards {
		if c.PowerExplanation == "" {
			t.Errorf("CuttableCard %s missing PowerExplanation", c.Name)
		}
	}
}
