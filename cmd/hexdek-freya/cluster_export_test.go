package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildClusterExport_SchemaVersionAndMetadata pins that every
// export — even from a deck with zero clusters — carries the schema
// version + deck metadata. Downstream parsers MUST be able to
// version-check before reading data.
func TestBuildClusterExport_SchemaVersionAndMetadata(t *testing.T) {
	dp := &DeckProfile{DeckName: "Empty Deck", Commander: "Bob"}
	export := BuildClusterExport(dp, &FreyaReport{})
	if export == nil {
		t.Fatal("expected non-nil export for empty-cluster deck (metadata still useful)")
	}
	if export.SchemaVersion != ClusterExportSchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", export.SchemaVersion, ClusterExportSchemaVersion)
	}
	if export.DeckName != "Empty Deck" {
		t.Errorf("DeckName = %q, want 'Empty Deck'", export.DeckName)
	}
	if export.Commander != "Bob" {
		t.Errorf("Commander = %q, want 'Bob'", export.Commander)
	}
	if export.GeneratedAt == "" {
		t.Errorf("GeneratedAt should be populated, got empty")
	}
	if export.Clusters == nil {
		t.Errorf("Clusters should be initialized to an empty slice (not nil), got nil")
	}
}

// TestBuildClusterExport_FullMembershipNotCapped pins the headline
// fix: the export carries the FULL deduped member list, not the
// 8-card display preview. A deck with 12 token producers must export
// all 12, otherwise downstream deck-builders see a truncated picture.
func TestBuildClusterExport_FullMembershipNotCapped(t *testing.T) {
	var profiles []CardProfile
	for i := 0; i < 12; i++ {
		profiles = append(profiles, makeTokenProducer("Token "+string(rune('A'+i))))
	}
	dp := &DeckProfile{DeckName: "Big Tokens"}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	export := BuildClusterExport(dp, report)

	var tokens *ClusterEntry
	for i := range export.Clusters {
		if export.Clusters[i].Theme == "tokens" {
			tokens = &export.Clusters[i]
			break
		}
	}
	if tokens == nil {
		t.Fatal("expected tokens cluster in export")
	}
	if tokens.MemberCount != 12 {
		t.Errorf("MemberCount = %d, want 12", tokens.MemberCount)
	}
	if len(tokens.Members) != 12 {
		t.Errorf("Members[] should carry all 12 cards (uncapped), got %d", len(tokens.Members))
	}
}

// TestBuildClusterExport_PerCardRoleClassification pins that each
// exported member carries its per-theme role classification — the
// information a deck-builder needs to know whether a cluster is
// producer-heavy ("add a payoff") or payoff-heavy ("add producers").
func TestBuildClusterExport_PerCardRoleClassification(t *testing.T) {
	profiles := padProfiles([]CardProfile{
		// 2 producers
		makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
		makeProfileWithTriggers("Bitterblossom", nil, []ResourceType{ResToken}),
		// 2 payoffs (Purphoros shape: etb+opponent_pain)
		makeProfileWithTriggers("Purphoros, God of the Forge",
			[]string{"etb", "opponent_pain"}, nil),
		makeProfileWithTriggers("Impact Tremors",
			[]string{"etb", "opponent_pain"}, nil),
	}, 10)
	dp := &DeckProfile{DeckName: "Tokens"}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	export := BuildClusterExport(dp, report)

	var tokens *ClusterEntry
	for i := range export.Clusters {
		if export.Clusters[i].Theme == "tokens" {
			tokens = &export.Clusters[i]
			break
		}
	}
	if tokens == nil {
		t.Fatal("expected tokens cluster")
	}
	rolesByName := map[string]string{}
	for _, m := range tokens.Members {
		rolesByName[m.Name] = m.Role
	}
	if rolesByName["Krenko, Mob Boss"] != "producer" {
		t.Errorf("Krenko role = %q, want producer", rolesByName["Krenko, Mob Boss"])
	}
	if rolesByName["Purphoros, God of the Forge"] != "payoff" {
		t.Errorf("Purphoros role = %q, want payoff", rolesByName["Purphoros, God of the Forge"])
	}
}

// TestBuildClusterExport_ScoreBreakdownDecomposes pins the
// score_breakdown field: the opaque integer Score must be decomposed
// into pair_score + chain_bonus + the underlying role tallies.
// pair_score + chain_bonus must equal Score.
func TestBuildClusterExport_ScoreBreakdownDecomposes(t *testing.T) {
	// 2 token producers + 2 token payoffs → no amplifier role for the
	// tokens theme (the role-classifier only knows producer/payoff
	// for tokens), so chain_count = 0, chain_bonus = 0, pair_score = 10.
	profiles := padProfiles([]CardProfile{
		makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
		makeProfileWithTriggers("Bitterblossom", nil, []ResourceType{ResToken}),
		makeProfileWithTriggers("Purphoros, God of the Forge",
			[]string{"etb", "opponent_pain"}, nil),
		makeProfileWithTriggers("Impact Tremors",
			[]string{"etb", "opponent_pain"}, nil),
	}, 10)
	dp := &DeckProfile{DeckName: "Tokens"}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	export := BuildClusterExport(dp, report)

	var tokens *ClusterEntry
	for i := range export.Clusters {
		if export.Clusters[i].Theme == "tokens" {
			tokens = &export.Clusters[i]
			break
		}
	}
	if tokens == nil {
		t.Fatal("expected tokens cluster")
	}
	bd := tokens.ScoreBreakdown
	if bd.PairScore+bd.ChainBonus != tokens.Score {
		t.Errorf("breakdown invariant violated: pair_score (%d) + chain_bonus (%d) != Score (%d)",
			bd.PairScore, bd.ChainBonus, tokens.Score)
	}
	if bd.ProducerCount != 2 {
		t.Errorf("producer_count = %d, want 2", bd.ProducerCount)
	}
	if bd.PayoffCount != 2 {
		t.Errorf("payoff_count = %d, want 2", bd.PayoffCount)
	}
	if bd.AmplifierCount != 0 {
		t.Errorf("amplifier_count = %d, want 0 (tokens theme has no amplifier role)", bd.AmplifierCount)
	}
	if bd.ChainBonus != 0 || bd.ChainCount != 0 {
		t.Errorf("no amplifier → chain_count + chain_bonus = 0, got %d / %d", bd.ChainCount, bd.ChainBonus)
	}
}

// TestBuildClusterExport_ChainCompleteFlag pins that ChainComplete
// fires when a P→A→Pf triangle exists. The death_value theme has all
// three roles defined, so a token-maker + sac outlet + dies-trigger
// cluster sets ChainComplete=true.
func TestBuildClusterExport_ChainCompleteFlag(t *testing.T) {
	profiles := padProfiles([]CardProfile{
		// Token-maker producer
		{Name: "Krenko, Mob Boss", TypeLine: "Creature",
			Produces: []ResourceType{ResToken}},
		{Name: "Bitterblossom", TypeLine: "Enchantment",
			Produces: []ResourceType{ResToken}},
		// Sac outlet amplifier
		{Name: "Goblin Bombardment", TypeLine: "Enchantment", IsOutlet: true},
		// Dies-trigger payoffs
		makeProfileWithTriggers("Blood Artist", []string{"dies"}, nil),
		makeProfileWithTriggers("Zulaport Cutthroat", []string{"dies"}, nil),
	}, 10)
	dp := &DeckProfile{DeckName: "Aristocrats"}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	export := BuildClusterExport(dp, report)

	var death *ClusterEntry
	for i := range export.Clusters {
		if export.Clusters[i].Theme == "death_value" {
			death = &export.Clusters[i]
			break
		}
	}
	if death == nil {
		t.Fatalf("expected death_value cluster, got clusters: %v", themesOf(export.Clusters))
	}
	if !death.ChainComplete {
		t.Errorf("ChainComplete should be true for P→A→Pf chain, got false")
	}
	if death.ScoreBreakdown.AmplifierCount < 1 {
		t.Errorf("expected ≥1 amplifier in death_value cluster, got %d", death.ScoreBreakdown.AmplifierCount)
	}
	if !death.HasRoleDichotomy {
		t.Errorf("death_value has producer/payoff/amplifier roles defined — HasRoleDichotomy should be true")
	}
}

// TestBuildClusterExport_FlatThemeHasRoleDichotomyFalse pins that
// themes without a producer/payoff dichotomy (landfall, spellcast,
// lifegain) export with has_role_dichotomy=false and all members
// rolled up as "unknown".
func TestBuildClusterExport_FlatThemeHasRoleDichotomyFalse(t *testing.T) {
	profiles := padProfiles([]CardProfile{
		makeProfileWithTriggers("Tireless Tracker", []string{"landfall"}, nil),
		makeProfileWithTriggers("Avenger of Zendikar", []string{"landfall"}, nil),
		makeProfileWithTriggers("Lotus Cobra", []string{"landfall"}, nil),
		makeProfileWithTriggers("Omnath, Locus of Rage", []string{"landfall"}, nil),
	}, 10)
	dp := &DeckProfile{DeckName: "Landfall"}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	export := BuildClusterExport(dp, report)

	var landfall *ClusterEntry
	for i := range export.Clusters {
		if export.Clusters[i].Theme == "landfall" {
			landfall = &export.Clusters[i]
			break
		}
	}
	if landfall == nil {
		t.Fatalf("expected landfall cluster, got %v", themesOf(export.Clusters))
	}
	if landfall.HasRoleDichotomy {
		t.Errorf("landfall is a flat theme — HasRoleDichotomy should be false")
	}
	if landfall.ChainComplete {
		t.Errorf("flat theme can't have a chain — ChainComplete should be false")
	}
	for _, m := range landfall.Members {
		if m.Role != "unknown" {
			t.Errorf("flat-theme member %s role = %q, want unknown", m.Name, m.Role)
		}
	}
}

// TestBuildClusterExport_MembersStableAlphabetical pins that the
// Members slice is sorted alphabetically. Without this, downstream
// consumers diffing two cluster exports across runs would see
// spurious noise from map-iter nondeterminism.
func TestBuildClusterExport_MembersStableAlphabetical(t *testing.T) {
	profiles := padProfiles([]CardProfile{
		makeProfileWithTriggers("Zebra Token Maker", nil, []ResourceType{ResToken}),
		makeProfileWithTriggers("Apple Token Maker", nil, []ResourceType{ResToken}),
		makeProfileWithTriggers("Mango Token Maker", nil, []ResourceType{ResToken}),
		makeProfileWithTriggers("Banana Token Maker", nil, []ResourceType{ResToken}),
	}, 10)
	dp := &DeckProfile{DeckName: "Tokens"}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	export := BuildClusterExport(dp, report)

	var tokens *ClusterEntry
	for i := range export.Clusters {
		if export.Clusters[i].Theme == "tokens" {
			tokens = &export.Clusters[i]
			break
		}
	}
	if tokens == nil {
		t.Fatal("expected tokens cluster")
	}
	for i := 1; i < len(tokens.Members); i++ {
		if tokens.Members[i].Name < tokens.Members[i-1].Name {
			t.Errorf("members not alphabetical: %q after %q",
				tokens.Members[i].Name, tokens.Members[i-1].Name)
		}
	}
	if tokens.Members[0].Name != "Apple Token Maker" {
		t.Errorf("first member should be alphabetical first, got %q", tokens.Members[0].Name)
	}
}

// TestWriteClusterExportJSON_RoundTrip pins the file-write path:
// the on-disk JSON must be parseable back into the same struct,
// preserving every field. This is the integration test for the
// --clusters-out CLI flag.
func TestWriteClusterExportJSON_RoundTrip(t *testing.T) {
	profiles := padProfiles([]CardProfile{
		makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
		makeProfileWithTriggers("Bitterblossom", nil, []ResourceType{ResToken}),
		makeProfileWithTriggers("Purphoros, God of the Forge",
			[]string{"etb", "opponent_pain"}, nil),
		makeProfileWithTriggers("Impact Tremors",
			[]string{"etb", "opponent_pain"}, nil),
	}, 10)
	dp := &DeckProfile{DeckName: "Tokens RT", Commander: "Krenko, Mob Boss"}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	export := BuildClusterExport(dp, report)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "clusters.json")
	if err := WriteClusterExportJSON(export, path); err != nil {
		t.Fatalf("WriteClusterExportJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var roundtripped SynergyClusterExport
	if err := json.Unmarshal(data, &roundtripped); err != nil {
		t.Fatalf("unmarshal roundtrip: %v", err)
	}
	if roundtripped.SchemaVersion != export.SchemaVersion {
		t.Errorf("SchemaVersion mismatch after roundtrip: %q vs %q",
			roundtripped.SchemaVersion, export.SchemaVersion)
	}
	if roundtripped.DeckName != export.DeckName {
		t.Errorf("DeckName mismatch: %q vs %q", roundtripped.DeckName, export.DeckName)
	}
	if len(roundtripped.Clusters) != len(export.Clusters) {
		t.Errorf("Clusters length mismatch: %d vs %d",
			len(roundtripped.Clusters), len(export.Clusters))
	}
	// Verify the JSON keys downstream tools actually parse against.
	wantedKeys := []string{
		`"schema_version"`, `"deck_name"`, `"clusters"`, `"theme"`,
		`"score_breakdown"`, `"pair_score"`, `"chain_bonus"`,
		`"chain_complete"`, `"has_role_dichotomy"`, `"members"`, `"role"`,
	}
	body := string(data)
	for _, key := range wantedKeys {
		if !strings.Contains(body, key) {
			t.Errorf("exported JSON missing expected key %s", key)
		}
	}
}

// TestBuildClusterExport_NilDpReturnsNil pins the safe-default
// contract: nil deck profile returns nil (callers can skip writing
// without nil-checking).
func TestBuildClusterExport_NilDpReturnsNil(t *testing.T) {
	if got := BuildClusterExport(nil, &FreyaReport{}); got != nil {
		t.Errorf("BuildClusterExport(nil, ...) should return nil, got %+v", got)
	}
}

// TestBuildClusterExport_FallsBackToCardsWhenAllMembersEmpty pins
// the back-compat clause: clusters built before AllMembers was added
// (or by external callers populating SynergyCluster manually) still
// produce a valid export by falling back to the capped Cards slice.
func TestBuildClusterExport_FallsBackToCardsWhenAllMembersEmpty(t *testing.T) {
	dp := &DeckProfile{
		DeckName: "Legacy",
		SynergyClusters: []SynergyCluster{
			{
				Theme: "tokens", Name: "Token Engine",
				Cards:       []string{"Card A", "Card B"},
				MemberCount: 2,
				Score:       5,
				// AllMembers intentionally nil — back-compat case.
			},
		},
	}
	export := BuildClusterExport(dp, &FreyaReport{})
	if len(export.Clusters) != 1 || len(export.Clusters[0].Members) != 2 {
		t.Errorf("fallback to Cards should yield 2 members, got %+v", export.Clusters)
	}
}

func themesOf(clusters []ClusterEntry) []string {
	out := make([]string, 0, len(clusters))
	for _, c := range clusters {
		out = append(out, c.Theme)
	}
	return out
}
