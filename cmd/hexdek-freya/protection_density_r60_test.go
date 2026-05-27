package main

import (
	"fmt"
	"strings"
	"testing"
)

// vulnComboFixture builds the minimal report+oracle pair so
// computeProtectionDensity has Roles.Assignments, Profiles (for CMC), and
// an oracleDB to read OracleText from.
func vulnComboFixture(assignments []CardRoleAssignment, profiles []CardProfile, oracleText map[string]string) (*DeckProfile, *FreyaReport, *oracleDB) {
	report := &FreyaReport{
		Profiles: profiles,
		Roles: &RoleAnalysis{
			Assignments: assignments,
			RoleCounts:  map[RoleTag]int{},
			TotalCards:  len(profiles),
		},
	}
	dp := &DeckProfile{}
	return dp, report, stubOracleWithText(oracleText)
}

// TestVulnerableComboPieces_UnprotectedComboFlagged pins the canonical
// case: a RoleCombo card whose oracle text has no protection keywords
// surfaces as a vulnerable combo piece.
func TestVulnerableComboPieces_UnprotectedComboFlagged(t *testing.T) {
	dp, report, oracle := vulnComboFixture(
		[]CardRoleAssignment{
			{Name: "Thassa's Oracle", Roles: []RoleTag{RoleCombo}},
		},
		[]CardProfile{{Name: "Thassa's Oracle", CMC: 2, TypeLine: "Creature — Merfolk Wizard"}},
		map[string]string{
			"thassa's oracle": "When this creature enters, look at the top X cards of your library, where X is your devotion to blue. Put up to one of them on top of your library and the rest on the bottom in a random order. If X is greater than or equal to the number of cards in your library, you win the game.",
		},
	)
	computeProtectionDensity(dp, report, oracle)

	if len(dp.VulnerableComboPieces) != 1 {
		t.Fatalf("want 1 vulnerable combo piece, got %d: %+v", len(dp.VulnerableComboPieces), dp.VulnerableComboPieces)
	}
	v := dp.VulnerableComboPieces[0]
	if v.Name != "Thassa's Oracle" {
		t.Errorf("wrong name: %q", v.Name)
	}
	if v.CMC != 2 {
		t.Errorf("CMC = %d, want 2", v.CMC)
	}
	if v.Reason == "" {
		t.Errorf("reason should be populated")
	}
	if dp.UnprotectedKeyPieces != 1 || dp.ProtectedKeyPieces != 0 {
		t.Errorf("aggregate counts wrong: protected=%d unprotected=%d", dp.ProtectedKeyPieces, dp.UnprotectedKeyPieces)
	}
}

// TestVulnerableComboPieces_ProtectedComboNotFlagged verifies a combo
// piece WITH built-in protection (hexproof / ward / indestructible) is
// NOT flagged — the whole point of the callout is "no protection".
func TestVulnerableComboPieces_ProtectedComboNotFlagged(t *testing.T) {
	dp, report, oracle := vulnComboFixture(
		[]CardRoleAssignment{
			{Name: "Hexproof Combo", Roles: []RoleTag{RoleCombo}},
			{Name: "Indestructible Combo", Roles: []RoleTag{RoleCombo}},
			{Name: "Ward Combo", Roles: []RoleTag{RoleCombo}},
		},
		[]CardProfile{
			{Name: "Hexproof Combo", CMC: 3, TypeLine: "Creature"},
			{Name: "Indestructible Combo", CMC: 4, TypeLine: "Creature"},
			{Name: "Ward Combo", CMC: 5, TypeLine: "Creature"},
		},
		map[string]string{
			"hexproof combo":       "Hexproof. Whenever this attacks, you win.",
			"indestructible combo": "Indestructible. Sacrifice three: win.",
			"ward combo":           "Ward {3}. Combo enabler.",
		},
	)
	computeProtectionDensity(dp, report, oracle)

	if len(dp.VulnerableComboPieces) != 0 {
		t.Errorf("protected combo pieces should NOT be flagged, got: %+v", dp.VulnerableComboPieces)
	}
	if dp.ProtectedKeyPieces != 3 || dp.UnprotectedKeyPieces != 0 {
		t.Errorf("aggregate counts wrong: protected=%d unprotected=%d", dp.ProtectedKeyPieces, dp.UnprotectedKeyPieces)
	}
}

// TestVulnerableComboPieces_ThreatNotFlagged verifies a RoleThreat card
// (not RoleCombo) without protection counts toward UnprotectedKeyPieces
// but is NOT surfaced in the vulnerable-combo callout. Threat losses are
// recoverable; combo line losses are catastrophic — the callout is
// deliberately combo-only.
func TestVulnerableComboPieces_ThreatNotFlagged(t *testing.T) {
	dp, report, oracle := vulnComboFixture(
		[]CardRoleAssignment{
			{Name: "Beater", Roles: []RoleTag{RoleThreat}},
		},
		[]CardProfile{{Name: "Beater", CMC: 4, TypeLine: "Creature — Beast"}},
		map[string]string{"beater": "Trample. 5/5."},
	)
	computeProtectionDensity(dp, report, oracle)

	if len(dp.VulnerableComboPieces) != 0 {
		t.Errorf("RoleThreat without RoleCombo should NOT be flagged, got: %+v", dp.VulnerableComboPieces)
	}
	if dp.UnprotectedKeyPieces != 1 {
		t.Errorf("threat should still count toward UnprotectedKeyPieces, got %d", dp.UnprotectedKeyPieces)
	}
}

// TestVulnerableComboPieces_NonKeyRoleNotFlagged verifies cards with no
// combo/threat role (e.g. ramp, draw) are skipped entirely.
func TestVulnerableComboPieces_NonKeyRoleNotFlagged(t *testing.T) {
	dp, report, oracle := vulnComboFixture(
		[]CardRoleAssignment{
			{Name: "Sol Ring", Roles: []RoleTag{RoleRamp}},
			{Name: "Brainstorm", Roles: []RoleTag{RoleDraw}},
		},
		[]CardProfile{
			{Name: "Sol Ring", CMC: 1, TypeLine: "Artifact"},
			{Name: "Brainstorm", CMC: 1, TypeLine: "Instant"},
		},
		map[string]string{
			"sol ring":   "{T}: Add {C}{C}.",
			"brainstorm": "Draw three cards, then put two cards from your hand on top of your library in any order.",
		},
	)
	computeProtectionDensity(dp, report, oracle)

	if len(dp.VulnerableComboPieces) != 0 {
		t.Errorf("non-key roles should NOT be flagged, got: %+v", dp.VulnerableComboPieces)
	}
	if dp.ProtectedKeyPieces != 0 || dp.UnprotectedKeyPieces != 0 {
		t.Errorf("non-key roles should not affect aggregates: protected=%d unprotected=%d",
			dp.ProtectedKeyPieces, dp.UnprotectedKeyPieces)
	}
}

// TestVulnerableComboPieces_SortedCMCDesc verifies the output ordering:
// higher CMC first (bigger tempo loss to recast), tie-broken by name
// ascending for stable rendering.
func TestVulnerableComboPieces_SortedCMCDesc(t *testing.T) {
	dp, report, oracle := vulnComboFixture(
		[]CardRoleAssignment{
			{Name: "Cheap Combo", Roles: []RoleTag{RoleCombo}},
			{Name: "Mid Combo B", Roles: []RoleTag{RoleCombo}},
			{Name: "Mid Combo A", Roles: []RoleTag{RoleCombo}},
			{Name: "Big Combo", Roles: []RoleTag{RoleCombo}},
		},
		[]CardProfile{
			{Name: "Cheap Combo", CMC: 1, TypeLine: "Creature"},
			{Name: "Mid Combo A", CMC: 3, TypeLine: "Creature"},
			{Name: "Mid Combo B", CMC: 3, TypeLine: "Creature"},
			{Name: "Big Combo", CMC: 6, TypeLine: "Creature"},
		},
		map[string]string{
			"cheap combo": "Win the game.",
			"mid combo a": "Win the game.",
			"mid combo b": "Win the game.",
			"big combo":   "Win the game.",
		},
	)
	computeProtectionDensity(dp, report, oracle)

	if len(dp.VulnerableComboPieces) != 4 {
		t.Fatalf("want 4, got %d", len(dp.VulnerableComboPieces))
	}
	got := []string{
		dp.VulnerableComboPieces[0].Name,
		dp.VulnerableComboPieces[1].Name,
		dp.VulnerableComboPieces[2].Name,
		dp.VulnerableComboPieces[3].Name,
	}
	want := []string{"Big Combo", "Mid Combo A", "Mid Combo B", "Cheap Combo"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestVulnerableComboPieces_CappedAtLimit verifies the per-callout cap
// at vulnerableComboPieceCap entries. A combo-heavy deck with 12
// unprotected pieces should still only surface the top 8 (by CMC desc).
func TestVulnerableComboPieces_CappedAtLimit(t *testing.T) {
	assignments := make([]CardRoleAssignment, 0, 12)
	profiles := make([]CardProfile, 0, 12)
	oracle := map[string]string{}
	for i := 0; i < 12; i++ {
		name := fmt.Sprintf("Combo %02d", i)
		assignments = append(assignments, CardRoleAssignment{Name: name, Roles: []RoleTag{RoleCombo}})
		profiles = append(profiles, CardProfile{Name: name, CMC: i + 1, TypeLine: "Creature"})
		oracle[strings.ToLower(name)] = "Win the game."
	}
	dp, report, oraDB := vulnComboFixture(assignments, profiles, oracle)
	computeProtectionDensity(dp, report, oraDB)

	if len(dp.VulnerableComboPieces) != vulnerableComboPieceCap {
		t.Errorf("len = %d, want cap %d", len(dp.VulnerableComboPieces), vulnerableComboPieceCap)
	}
	// Top entry should be the highest-CMC item (Combo 11, CMC 12).
	if dp.VulnerableComboPieces[0].Name != "Combo 11" {
		t.Errorf("top entry = %q, want Combo 11 (highest CMC)", dp.VulnerableComboPieces[0].Name)
	}
	// Aggregate UnprotectedKeyPieces is NOT capped — it counts all 12.
	if dp.UnprotectedKeyPieces != 12 {
		t.Errorf("UnprotectedKeyPieces = %d, want 12 (cap applies only to callout list)", dp.UnprotectedKeyPieces)
	}
}

// TestVulnerableComboPieces_ComboAndThreatBothRoles verifies a card with
// BOTH RoleCombo and RoleThreat is flagged (the combo role still
// qualifies it for the vulnerability callout) — defends against an
// "isCombo only when sole role" misreading.
func TestVulnerableComboPieces_ComboAndThreatBothRoles(t *testing.T) {
	dp, report, oracle := vulnComboFixture(
		[]CardRoleAssignment{
			{Name: "Dual Role", Roles: []RoleTag{RoleThreat, RoleCombo}},
		},
		[]CardProfile{{Name: "Dual Role", CMC: 4, TypeLine: "Creature"}},
		map[string]string{"dual role": "When this attacks, win the game."},
	)
	computeProtectionDensity(dp, report, oracle)

	if len(dp.VulnerableComboPieces) != 1 {
		t.Fatalf("dual-role combo should be flagged, got %d: %+v", len(dp.VulnerableComboPieces), dp.VulnerableComboPieces)
	}
}

// TestVulnerableComboPieces_OracleMissingTreatedAsUnprotected verifies
// the safe-default path: when oracle.lookup returns nil (DFC corner case,
// custom card, or cache miss), a RoleCombo card is still surfaced —
// "unknown protection" should fail loud, not silently pass as protected.
func TestVulnerableComboPieces_OracleMissingTreatedAsUnprotected(t *testing.T) {
	dp, report, oracle := vulnComboFixture(
		[]CardRoleAssignment{
			{Name: "Missing Combo", Roles: []RoleTag{RoleCombo}},
		},
		[]CardProfile{{Name: "Missing Combo", CMC: 5, TypeLine: "Creature"}},
		map[string]string{}, // intentionally empty — lookup returns nil
	)
	computeProtectionDensity(dp, report, oracle)

	if len(dp.VulnerableComboPieces) != 1 {
		t.Fatalf("oracle-missing combo should still be flagged, got %d: %+v",
			len(dp.VulnerableComboPieces), dp.VulnerableComboPieces)
	}
	if dp.VulnerableComboPieces[0].CMC != 5 {
		t.Errorf("CMC fallback to Profiles map failed, got %d", dp.VulnerableComboPieces[0].CMC)
	}
}
