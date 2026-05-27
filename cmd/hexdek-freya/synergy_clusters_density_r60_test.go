package main

import (
	"strings"
	"testing"
)

// TestFinalizeClusters_HighDensityFiresAtFloor pins the canonical case:
// a cluster with MemberCount == HighDensityClusterFloor (5) is flagged
// HighDensity; a sibling cluster at 4 is not. Boundary-inclusive.
func TestFinalizeClusters_HighDensityFiresAtFloor(t *testing.T) {
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{
			{Name: "Token Engine", Theme: "tokens", MemberCount: 5, Score: 10},
			{Name: "Spellslinger Package", Theme: "spellcast", MemberCount: 4, Score: 6},
		},
	}
	finalizeClusters(dp)

	if len(dp.SynergyClusters) != 2 {
		t.Fatalf("expected both clusters retained, got %d", len(dp.SynergyClusters))
	}
	// Sorted by score desc — tokens (10) first.
	if dp.SynergyClusters[0].Theme != "tokens" {
		t.Fatalf("score-desc sort broke: %+v", dp.SynergyClusters)
	}
	if !dp.SynergyClusters[0].HighDensity {
		t.Errorf("MemberCount=5 cluster should be HighDensity")
	}
	if dp.SynergyClusters[1].HighDensity {
		t.Errorf("MemberCount=4 cluster should NOT be HighDensity (must be strictly >= floor 5)")
	}
}

// TestFinalizeClusters_HighDensityFiresAboveFloor verifies the flag
// keeps firing for MemberCount well above 5 (defends against a
// "== floor only" off-by-one regression).
func TestFinalizeClusters_HighDensityFiresAboveFloor(t *testing.T) {
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{
			{Name: "Token Engine", Theme: "tokens", MemberCount: 12, Score: 30},
		},
	}
	finalizeClusters(dp)
	if !dp.SynergyClusters[0].HighDensity {
		t.Errorf("MemberCount=12 cluster should be HighDensity")
	}
}

// TestFinalizeClusters_SubTwoPruned verifies the defensive prune at
// MinimumClusterMembers (2): a 1-member "cluster" is dropped entirely.
// Current compute paths floor at 4 so this is a no-op under default
// flow, but the prune protects against a future floor relaxation.
func TestFinalizeClusters_SubTwoPruned(t *testing.T) {
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{
			{Name: "Real Theme", Theme: "tokens", MemberCount: 4, Score: 6},
			{Name: "Lone Card", Theme: "lifegain", MemberCount: 1, Score: 0},
		},
	}
	finalizeClusters(dp)
	if len(dp.SynergyClusters) != 1 {
		t.Fatalf("sub-2 cluster should be pruned, got: %+v", dp.SynergyClusters)
	}
	if dp.SynergyClusters[0].Theme != "tokens" {
		t.Errorf("wrong cluster retained: %+v", dp.SynergyClusters[0])
	}
}

// TestFinalizeClusters_TwoMemberRetained pins the boundary on the
// minimum side: MemberCount == MinimumClusterMembers (2) is retained.
// The prune is strictly "< 2", not "<= 2".
func TestFinalizeClusters_TwoMemberRetained(t *testing.T) {
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{
			{Name: "Pair", Theme: "lifegain", MemberCount: 2, Score: 1},
		},
	}
	finalizeClusters(dp)
	if len(dp.SynergyClusters) != 1 {
		t.Errorf("MemberCount=2 cluster should be retained, got: %+v", dp.SynergyClusters)
	}
	if dp.SynergyClusters[0].HighDensity {
		t.Errorf("MemberCount=2 cluster should NOT be HighDensity")
	}
}

// TestFinalizeClusters_CapAtFive verifies the existing 5-cluster cap is
// preserved through the finalize refactor (regression guard — the cap
// previously lived inline in computeSynergyClusters).
func TestFinalizeClusters_CapAtFive(t *testing.T) {
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{
			{Name: "A", MemberCount: 4, Score: 10},
			{Name: "B", MemberCount: 4, Score: 9},
			{Name: "C", MemberCount: 4, Score: 8},
			{Name: "D", MemberCount: 4, Score: 7},
			{Name: "E", MemberCount: 4, Score: 6},
			{Name: "F", MemberCount: 4, Score: 5},
			{Name: "G", MemberCount: 4, Score: 4},
		},
	}
	finalizeClusters(dp)
	if len(dp.SynergyClusters) != 5 {
		t.Errorf("cap of 5 not enforced, got %d", len(dp.SynergyClusters))
	}
	if dp.SynergyClusters[4].Name != "E" {
		t.Errorf("bottom of cap wrong (should be E, score 6), got %q", dp.SynergyClusters[4].Name)
	}
}

// TestSynergyClustersJSON_HighDensityRoundTrips verifies the
// HighDensity flag is mirrored into the jsonCluster build path so
// downstream consumers (UI, deck-builder integrations) actually see
// the signal. Builds the full deck profile JSON and asserts the
// high_density field on the right cluster.
func TestSynergyClustersJSON_HighDensityRoundTrips(t *testing.T) {
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{
			{Name: "Big Theme", Theme: "tokens", MemberCount: 8, Score: 20, HighDensity: true},
			{Name: "Small Theme", Theme: "lifegain", MemberCount: 4, Score: 6},
		},
	}
	report := &FreyaReport{}
	js := buildJSONDeckProfile(dp, report)
	if js == nil {
		t.Fatalf("buildJSONDeckProfile returned nil")
	}
	if len(js.SynergyClusters) != 2 {
		t.Fatalf("want 2 json clusters, got %d", len(js.SynergyClusters))
	}
	var bigCluster, smallCluster *jsonCluster
	for i := range js.SynergyClusters {
		switch js.SynergyClusters[i].Theme {
		case "tokens":
			bigCluster = &js.SynergyClusters[i]
		case "lifegain":
			smallCluster = &js.SynergyClusters[i]
		}
	}
	if bigCluster == nil || smallCluster == nil {
		t.Fatalf("missing cluster in JSON: %+v", js.SynergyClusters)
	}
	if !bigCluster.HighDensity {
		t.Errorf("HighDensity not mirrored into JSON for big cluster: %+v", bigCluster)
	}
	if smallCluster.HighDensity {
		t.Errorf("low-density cluster should serialize HighDensity=false: %+v", smallCluster)
	}
}

// TestClusterExport_HighDensityRoundTrips verifies the rich structured
// export (cluster_export.go) also carries the HighDensity flag — the
// two JSON surfaces serve different consumers and must agree.
func TestClusterExport_HighDensityRoundTrips(t *testing.T) {
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{
			{Name: "Big Theme", Theme: "tokens", MemberCount: 6, Score: 14, HighDensity: true,
				AllMembers: []string{"A", "B", "C", "D", "E", "F"}},
		},
	}
	export := BuildClusterExport(dp, &FreyaReport{})
	if export == nil || len(export.Clusters) != 1 {
		t.Fatalf("export wrong: %+v", export)
	}
	if !export.Clusters[0].HighDensity {
		t.Errorf("HighDensity should be true in ClusterEntry, got: %+v", export.Clusters[0])
	}
}

// TestComputeSynergyClusters_HighDensityEndToEnd runs the full compute
// pipeline (not just finalize) on a token-heavy fixture, verifies that
// the resulting tokens cluster ends up flagged HighDensity. Defends
// against a future refactor that moves the flag-stamping back inline
// and forgets to fire on real decks.
func TestComputeSynergyClusters_HighDensityEndToEnd(t *testing.T) {
	report := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Empty the Warrens", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Dragon Fodder", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Krenko's Command", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Hordeling Outburst", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Purphoros, God of the Forge",
				[]string{"etb", "opponent_pain"}, nil),
		}, 10),
	}
	dp := &DeckProfile{}
	computeSynergyClusters(dp, report, stubOracle())

	var tokens *SynergyCluster
	for i := range dp.SynergyClusters {
		if dp.SynergyClusters[i].Theme == "tokens" {
			tokens = &dp.SynergyClusters[i]
			break
		}
	}
	if tokens == nil {
		t.Fatalf("expected tokens cluster, got: %+v", dp.SynergyClusters)
	}
	if tokens.MemberCount < HighDensityClusterFloor {
		t.Fatalf("fixture expected to seed >= %d members in tokens, got %d",
			HighDensityClusterFloor, tokens.MemberCount)
	}
	if !tokens.HighDensity {
		t.Errorf("tokens cluster with MemberCount=%d should be HighDensity", tokens.MemberCount)
	}
}

// TestSynergyClustersText_HighDensityPrefix verifies the text-report
// render carries the ★ high-density prefix so the human-facing report
// — not just the JSON — surfaces the flag.
func TestSynergyClustersText_HighDensityPrefix(t *testing.T) {
	dp := &DeckProfile{
		SynergyClusters: []SynergyCluster{
			{Name: "Token Engine", Theme: "tokens", MemberCount: 7, Score: 18,
				Cards: []string{"A", "B", "C"}, HighDensity: true},
			{Name: "Lifegain", Theme: "lifegain", MemberCount: 4, Score: 6,
				Cards: []string{"D", "E"}},
		},
	}
	var buf strings.Builder
	report := &FreyaReport{Profile: dp}
	printText(&buf, report)
	out := buf.String()
	if !strings.Contains(out, "★ high-density") {
		t.Errorf("text report missing high-density prefix:\n%s", out)
	}
	// Low-density cluster must NOT carry the prefix.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "[Lifegain]") && strings.Contains(line, "★ high-density") {
			t.Errorf("low-density cluster wrongly flagged: %q", line)
		}
	}
}
