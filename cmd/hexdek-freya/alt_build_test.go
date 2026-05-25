package main

import (
	"strings"
	"testing"
)

// makeTokenProducer is a fixture helper for alt-build tests — a vanilla
// token-producing creature.
func makeTokenProducer(name string) CardProfile {
	return CardProfile{
		Name:     name,
		TypeLine: "Creature",
		Produces: []ResourceType{ResToken},
	}
}

// makeRecursionCreature is a fixture helper — a reanimator-style
// creature flagged IsRecursion so it lands in the recursion cluster.
func makeRecursionCreature(name string) CardProfile {
	return CardProfile{
		Name:        name,
		TypeLine:    "Creature",
		IsRecursion: true,
	}
}

// TestAltBuildSuggestions_TokensVsReanimator pins the headline use
// case from the spec: a deck with ~12 token-cluster cards AND ~8
// recursion cards should surface "you could refocus around reanimator"
// as a suggestion alongside its primary token engine.
func TestAltBuildSuggestions_TokensVsReanimator(t *testing.T) {
	var profiles []CardProfile
	// 12 token cluster cards (mix of producers + Purphoros-shape payoffs
	// so it forms a high-score cluster).
	tokenProducers := []string{"Krenko", "Bitterblossom", "Empty the Warrens",
		"Dragon Fodder", "Goblin Rabblemaster", "Hordeling Outburst",
		"Krenko's Command", "Mardu Ascendancy", "Beetleback Chief"}
	for _, n := range tokenProducers {
		profiles = append(profiles, makeTokenProducer(n))
	}
	tokenPayoffs := []string{"Purphoros", "Impact Tremors", "Reckless Fireweaver"}
	for _, n := range tokenPayoffs {
		profiles = append(profiles, makeProfileWithTriggers(n,
			[]string{"etb", "opponent_pain"}, nil))
	}
	// 8 recursion cluster cards.
	recursionCards := []string{"Reanimate", "Animate Dead", "Unburial Rites",
		"Eternal Witness", "Sun Titan", "Karmic Guide", "Living Death",
		"Victimize"}
	for _, n := range recursionCards {
		profiles = append(profiles, makeRecursionCreature(n))
	}

	dp := &DeckProfile{}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	computeAltBuildSuggestions(dp)

	if len(dp.AltBuildSuggestions) == 0 {
		t.Fatalf("expected an alt-build suggestion for the 12/8 split, got none. Clusters: %v",
			summariseClusters(dp.SynergyClusters))
	}
	// The recursion cluster (8 cards) should appear as an alt-build —
	// it clears the 8-card pivot threshold and is the secondary engine.
	var recHit *AltBuildSuggestion
	for i := range dp.AltBuildSuggestions {
		if dp.AltBuildSuggestions[i].Cluster == "recursion" {
			recHit = &dp.AltBuildSuggestions[i]
			break
		}
	}
	if recHit == nil {
		t.Fatalf("expected recursion alt-build suggestion, got %v",
			altBuildNames(dp.AltBuildSuggestions))
	}
	if recHit.MemberCount < 8 {
		t.Errorf("recursion alt-build MemberCount=%d, want >=8", recHit.MemberCount)
	}
	if !strings.Contains(recHit.Pivot, "Reanimator") &&
		!strings.Contains(recHit.Pivot, "Recursion") {
		t.Errorf("recursion pivot text should name the package, got %q", recHit.Pivot)
	}
	if !strings.Contains(recHit.Trade, "Token") {
		t.Errorf("recursion trade text should reference the primary token engine, got %q", recHit.Trade)
	}
}

// TestAltBuildSuggestions_BelowThresholdSilences pins that a deck with
// one big cluster and one small (<8) cluster does NOT emit an alt-build
// suggestion — the small cluster isn't a real seed, just incidental
// overlap, and suggesting a pivot to it would be noise.
func TestAltBuildSuggestions_BelowThresholdSilences(t *testing.T) {
	var profiles []CardProfile
	// 12 token cards (well above threshold).
	for i := 0; i < 9; i++ {
		profiles = append(profiles, makeTokenProducer("Token Maker "+string(rune('A'+i))))
	}
	for i := 0; i < 3; i++ {
		profiles = append(profiles, makeProfileWithTriggers(
			"Token Payoff "+string(rune('A'+i)), []string{"etb", "opponent_pain"}, nil))
	}
	// Only 5 recursion cards (well below 8-card threshold).
	for i := 0; i < 5; i++ {
		profiles = append(profiles, makeRecursionCreature("Reanimator "+string(rune('A'+i))))
	}

	dp := &DeckProfile{}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	computeAltBuildSuggestions(dp)

	for _, a := range dp.AltBuildSuggestions {
		if a.Cluster == "recursion" {
			t.Errorf("recursion cluster only had 5 cards (below 8-card threshold) — should not surface as alt-build, got %+v", a)
		}
	}
}

// TestAltBuildSuggestions_NoPrimaryNoSuggestions pins that a deck whose
// LARGEST cluster doesn't even clear the threshold gets no
// alt-build suggestions — the framing requires a primary spine to
// pivot AWAY from.
func TestAltBuildSuggestions_NoPrimaryNoSuggestions(t *testing.T) {
	var profiles []CardProfile
	// 5 token cards + 5 recursion cards — neither clears 8.
	for i := 0; i < 5; i++ {
		profiles = append(profiles, makeTokenProducer("Token "+string(rune('A'+i))))
	}
	for i := 0; i < 5; i++ {
		profiles = append(profiles, makeRecursionCreature("Recursion "+string(rune('A'+i))))
	}

	dp := &DeckProfile{}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	computeAltBuildSuggestions(dp)

	if len(dp.AltBuildSuggestions) != 0 {
		t.Errorf("expected no suggestions when no cluster clears threshold, got %v",
			altBuildNames(dp.AltBuildSuggestions))
	}
}

// TestAltBuildSuggestions_SingleClusterNoSuggestion pins that a deck
// with exactly one substantial cluster gets no suggestions — there's
// nothing to pivot to.
func TestAltBuildSuggestions_SingleClusterNoSuggestion(t *testing.T) {
	var profiles []CardProfile
	for i := 0; i < 10; i++ {
		profiles = append(profiles, makeTokenProducer("Token "+string(rune('A'+i))))
	}

	dp := &DeckProfile{}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	computeAltBuildSuggestions(dp)

	if len(dp.AltBuildSuggestions) != 0 {
		t.Errorf("single-cluster deck should yield no alt-build suggestions, got %v",
			altBuildNames(dp.AltBuildSuggestions))
	}
}

// TestAltBuildSuggestions_TriplePivotCapsAtThree pins the display cap:
// a deck split across 4 themes (all above threshold) gets at most 3
// alt-build suggestions. Past 3 the deck is so unfocused the list
// itself becomes noise.
func TestAltBuildSuggestions_TriplePivotCapsAtThree(t *testing.T) {
	var profiles []CardProfile
	// 9 each of token / recursion / spellcast / landfall.
	for i := 0; i < 9; i++ {
		profiles = append(profiles, makeTokenProducer("Token "+string(rune('A'+i))))
		profiles = append(profiles, makeRecursionCreature("Reanimator "+string(rune('A'+i))))
		profiles = append(profiles, makeProfileWithTriggers(
			"Spellcaster "+string(rune('A'+i)), []string{"cast"}, nil))
		profiles = append(profiles, makeProfileWithTriggers(
			"Landfaller "+string(rune('A'+i)), []string{"landfall"}, nil))
	}

	dp := &DeckProfile{}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	computeAltBuildSuggestions(dp)

	if len(dp.AltBuildSuggestions) > 3 {
		t.Errorf("4-cluster split should cap suggestions at 3, got %d: %v",
			len(dp.AltBuildSuggestions), altBuildNames(dp.AltBuildSuggestions))
	}
	if len(dp.AltBuildSuggestions) < 2 {
		t.Errorf("4-cluster split should yield at least 2 alt-builds, got %d",
			len(dp.AltBuildSuggestions))
	}
}

// TestAltBuildSuggestions_MemberCountTracksUncapped pins the
// MemberCount field — it must be the full deduped count, not the
// display-capped Cards slice length. The pivot threshold reads
// MemberCount, so if a 12-card cluster only stored its 8-card display
// list, the threshold check would still pass but the suggestion text
// would lie about the cluster size.
func TestAltBuildSuggestions_MemberCountTracksUncapped(t *testing.T) {
	var profiles []CardProfile
	for i := 0; i < 12; i++ {
		profiles = append(profiles, makeTokenProducer("Token "+string(rune('A'+i))))
	}

	dp := &DeckProfile{}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())

	var tokens *SynergyCluster
	for i := range dp.SynergyClusters {
		if dp.SynergyClusters[i].Theme == "tokens" {
			tokens = &dp.SynergyClusters[i]
			break
		}
	}
	if tokens == nil {
		t.Fatalf("expected tokens cluster, got %v", summariseClusters(dp.SynergyClusters))
	}
	if tokens.MemberCount != 12 {
		t.Errorf("tokens cluster MemberCount=%d, want 12 (uncapped)", tokens.MemberCount)
	}
	if len(tokens.Cards) > 8 {
		t.Errorf("Cards display list should still cap at 8, got %d", len(tokens.Cards))
	}
}

// TestAltBuildSuggestions_PivotTextNamesClusters pins that the
// human-readable Pivot and Trade strings name BOTH the suggested
// cluster (to pivot toward) and the primary cluster (to trim from).
// This is what the Decks screen surfaces, so the test guards against
// the strings going silent if either argument is dropped.
func TestAltBuildSuggestions_PivotTextNamesClusters(t *testing.T) {
	var profiles []CardProfile
	for i := 0; i < 10; i++ {
		profiles = append(profiles, makeTokenProducer("Token "+string(rune('A'+i))))
	}
	for i := 0; i < 8; i++ {
		profiles = append(profiles, makeRecursionCreature("Reanimator "+string(rune('A'+i))))
	}

	dp := &DeckProfile{}
	report := &FreyaReport{Profiles: profiles}
	computeSynergyClusters(dp, report, stubOracle())
	computeAltBuildSuggestions(dp)

	if len(dp.AltBuildSuggestions) == 0 {
		t.Fatalf("expected an alt-build suggestion")
	}
	a := dp.AltBuildSuggestions[0]
	if !strings.Contains(a.Pivot, a.ClusterName) {
		t.Errorf("Pivot text should name the target cluster %q, got %q", a.ClusterName, a.Pivot)
	}
	if !strings.Contains(a.Trade, "Token") {
		t.Errorf("Trade text should reference the primary engine (Token), got %q", a.Trade)
	}
	if a.MemberCount == 0 {
		t.Errorf("MemberCount must be populated, got 0")
	}
}

// summariseClusters and altBuildNames are display helpers for test
// failure output — show enough to debug a regression without dumping
// the full profile dataset.
func summariseClusters(cs []SynergyCluster) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Theme+"("+itoa(c.MemberCount)+")")
	}
	return out
}

func altBuildNames(as []AltBuildSuggestion) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Cluster+"("+itoa(a.MemberCount)+")")
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return sign + string(digits)
}
