package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Test_SaveProfileJSON_EmitsExpectedScalars confirms saveProfileJSON
// writes a file readable by db.FreyaProfileFromJSON. The fields
// asserted here are exactly the ones the snapshot-schema follow-up
// (deck_freya_profile) reads back via the runFreya backfill —
// adding a new scalar to FreyaProfile means extending this assertion
// so the wire is verified end-to-end.
func Test_SaveProfileJSON_EmitsExpectedScalars(t *testing.T) {
	dir := t.TempDir()
	report := &FreyaReport{
		TotalCards: 100,
		AvgCMC:     2.6,
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{},
		},
		Profile: &DeckProfile{
			DeckName:           "edgar_markov_b2_alice",
			Commander:          "Edgar Markov",
			Bracket:            2,
			BracketLabel:       "Core",
			PrimaryArchetype:   "Aggro / Tribal",
			SecondaryArchetype: "Tokens",
			CommanderSynergy:   0.625,
			PowerPercentile:    47,
			PowerTierCounts: map[string]int{
				"S": 2, "A": 8, "B": 35, "C": 24, "D": 6,
			},
			TopRoles: []RoleCount{
				{Role: RoleTag("threat"), Count: 22},
				{Role: RoleTag("ramp"), Count: 11},
				{Role: RoleTag("removal"), Count: 7},
			},
		},
	}

	path := filepath.Join(dir, "edgar.profile.json")
	saveProfileJSON(path, report)

	blob, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("parse sidecar: %v", err)
	}

	// These keys are the contract with db.FreyaProfileFromJSON. If
	// you rename one, that downstream parser stops finding the
	// field and the snapshot column silently goes empty.
	wantKeys := []string{
		"primary_archetype", "secondary_archetype", "bracket",
		"commander_synergy", "power_percentile", "power_tier_counts",
		"top_roles",
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("sidecar missing key %q (full payload: %v)", k, got)
		}
	}

	// And the values round-trip — defends against a future PR that
	// keeps the keys but reshapes their content (e.g., switching
	// commander_synergy from 0-1 to 0-100 without telling the
	// downstream parser).
	if got["primary_archetype"] != "Aggro / Tribal" {
		t.Errorf("primary_archetype: got %v", got["primary_archetype"])
	}
	if got["bracket"].(float64) != 2 {
		t.Errorf("bracket: got %v", got["bracket"])
	}
	if got["commander_synergy"].(float64) != 0.625 {
		t.Errorf("commander_synergy: got %v want 0.625", got["commander_synergy"])
	}
	tiers, ok := got["power_tier_counts"].(map[string]any)
	if !ok {
		t.Fatalf("power_tier_counts not an object: %T", got["power_tier_counts"])
	}
	if tiers["S"].(float64) != 2 || tiers["B"].(float64) != 35 {
		t.Errorf("power_tier_counts mismatch: %v", tiers)
	}
	roles, ok := got["top_roles"].([]any)
	if !ok || len(roles) != 3 {
		t.Fatalf("top_roles not a 3-element array: %v", got["top_roles"])
	}
}

// Test_SaveProfileJSON_NilReport confirms the sidecar writer is safe
// against nil Profile / nil report (defensive paths the production
// callsite hits when Freya fails partway through analysis).
func Test_SaveProfileJSON_NilReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nil.profile.json")
	saveProfileJSON(path, nil)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("nil report should not create sidecar; stat err=%v", err)
	}

	path2 := filepath.Join(dir, "noprofile.profile.json")
	saveProfileJSON(path2, &FreyaReport{})
	if _, err := os.Stat(path2); !os.IsNotExist(err) {
		t.Errorf("nil Profile should not create sidecar; stat err=%v", err)
	}
}
