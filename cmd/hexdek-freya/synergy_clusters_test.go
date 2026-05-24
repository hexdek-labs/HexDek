package main

import (
	"testing"
)

// TestRolesPairScore covers the producer/payoff pair-weighting helper
// in isolation. The matrix:
//
//	producer × producer  → 1 (same-side pile)
//	producer × payoff    → 2 (complementary engine)
//	payoff × payoff      → 1 (same-side pile)
//	both × anything      → 2 (slots into either side)
//	unknown × anything   → 1 (fallback for themes without a dichotomy)
func TestRolesPairScore(t *testing.T) {
	cases := []struct {
		a, b clusterRole
		want int
	}{
		{clusterRoleProducer, clusterRoleProducer, 1},
		{clusterRolePayoff, clusterRolePayoff, 1},
		{clusterRoleProducer, clusterRolePayoff, 2},
		{clusterRolePayoff, clusterRoleProducer, 2},
		{clusterRoleBoth, clusterRoleProducer, 2},
		{clusterRoleBoth, clusterRolePayoff, 2},
		{clusterRoleBoth, clusterRoleBoth, 2},
		{clusterRoleBoth, clusterRoleUnknown, 2},
		{clusterRoleUnknown, clusterRoleProducer, 1},
		{clusterRoleUnknown, clusterRoleUnknown, 1},
	}
	for _, tc := range cases {
		if got := rolesPairScore(tc.a, tc.b); got != tc.want {
			t.Errorf("rolesPairScore(%v, %v) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestClassifyClusterRole_Tokens covers the role classifier for the
// tokens theme — the canonical refinement case. Producer = produces
// tokens (Krenko shape). Payoff = "whenever a creature ETBs, deal
// damage to each opponent" (Purphoros shape) or "whenever a token is
// created" trigger.
func TestClassifyClusterRole_Tokens(t *testing.T) {
	cases := []struct {
		name string
		p    CardProfile
		want clusterRole
	}{
		{
			"Krenko-shape token producer",
			CardProfile{Name: "Krenko, Mob Boss", Produces: []ResourceType{ResToken}},
			clusterRoleProducer,
		},
		{
			"Purphoros-shape payoff",
			CardProfile{Name: "Purphoros, God of the Forge", Triggers: []string{"etb", "opponent_pain"}},
			clusterRolePayoff,
		},
		{
			"Bare creature-ETB trigger is NOT a tokens payoff",
			CardProfile{Name: "Solemn Simulacrum", Triggers: []string{"etb"}},
			clusterRoleUnknown,
		},
		{
			"Token-created trigger — direct payoff signal",
			CardProfile{Name: "Cathars' Crusade", Triggers: []string{"token_created"}},
			clusterRolePayoff,
		},
		{
			"Both: produces tokens AND triggers on token creation",
			CardProfile{
				Name:     "Hypothetical Engine",
				Produces: []ResourceType{ResToken},
				Triggers: []string{"token_created"},
			},
			clusterRoleBoth,
		},
		{
			"Non-token card under tokens theme",
			CardProfile{Name: "Sol Ring", Produces: []ResourceType{ResMana}},
			clusterRoleUnknown,
		},
	}
	for _, tc := range cases {
		if got := classifyClusterRole(tc.p, "tokens"); got != tc.want {
			t.Errorf("classifyClusterRole(%q, tokens) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestClassifyClusterRole_DeathValue covers the producer/payoff split
// for the death_value theme. Producer is the engine that supplies death
// events (sac outlet or token-maker for bodies). Payoff is the "whenever
// a creature dies" trigger (Blood Artist, Zulaport).
func TestClassifyClusterRole_DeathValue(t *testing.T) {
	cases := []struct {
		name string
		p    CardProfile
		want clusterRole
	}{
		{
			"Goblin Bombardment — sac outlet producer",
			CardProfile{Name: "Goblin Bombardment", IsOutlet: true},
			clusterRoleProducer,
		},
		{
			"Blood Artist — dies-trigger payoff",
			CardProfile{Name: "Blood Artist", Triggers: []string{"dies"}},
			clusterRolePayoff,
		},
		{
			"Pawn of Ulamog — both (makes tokens to die AND triggers on dies)",
			CardProfile{
				Name:     "Pawn of Ulamog",
				Produces: []ResourceType{ResToken},
				Triggers: []string{"dies"},
			},
			clusterRoleBoth,
		},
	}
	for _, tc := range cases {
		if got := classifyClusterRole(tc.p, "death_value"); got != tc.want {
			t.Errorf("classifyClusterRole(%q, death_value) = %v, want %v",
				tc.name, got, tc.want)
		}
	}
}

// TestClassifyClusterRole_UnknownTheme verifies that themes without a
// clean producer/payoff dichotomy fall through to Unknown so the
// caller's flat-pair-count fallback kicks in.
func TestClassifyClusterRole_UnknownTheme(t *testing.T) {
	p := CardProfile{Name: "Tireless Tracker", Triggers: []string{"landfall"}}
	for _, theme := range []string{"landfall", "spellcast", "lifegain", "draw", "mana"} {
		if got := classifyClusterRole(p, theme); got != clusterRoleUnknown {
			t.Errorf("classifyClusterRole(%q) = %v, want Unknown (no dichotomy)", theme, got)
		}
	}
}

// stubOracle returns a non-nil oracleDB that passes the nil-guard in
// computeSynergyClusters. The function doesn't otherwise use the
// oracle field; the test only needs the pointer to be non-nil.
func stubOracle() *oracleDB { return &oracleDB{byName: map[string]*oracleEntry{}} }

// makeProfileWithTriggers is a fixture helper for cluster tests.
func makeProfileWithTriggers(name string, triggers []string, produces []ResourceType) CardProfile {
	return CardProfile{
		Name:     name,
		TypeLine: "Creature",
		Triggers: triggers,
		Produces: produces,
	}
}

// padProfiles fills out a deck to the 10-profile minimum
// computeSynergyClusters requires. The padding profiles are blank so
// they don't contaminate cluster membership.
func padProfiles(profiles []CardProfile, target int) []CardProfile {
	for i := len(profiles); i < target; i++ {
		profiles = append(profiles, CardProfile{Name: "filler-" + string(rune('A'+i)), TypeLine: "Sorcery"})
	}
	return profiles
}

// TestComputeSynergyClusters_TokensPayoffEntersCluster asserts that
// Purphoros-shape payoff cards now land in the tokens cluster. Before
// R60, they only landed in etb_value because they don't Produce
// ResToken, leaving the tokens cluster as just the producers and
// hiding the deck's actual wincon density.
func TestComputeSynergyClusters_TokensPayoffEntersCluster(t *testing.T) {
	report := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			// 3 token producers
			makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Empty the Warrens", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Dragon Fodder", nil, []ResourceType{ResToken}),
			// 1 token payoff (Purphoros shape)
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
		t.Fatalf("expected tokens cluster, got clusters: %v", dp.SynergyClusters)
	}
	hasPurphoros := false
	for _, c := range tokens.Cards {
		if c == "Purphoros, God of the Forge" {
			hasPurphoros = true
			break
		}
	}
	if !hasPurphoros {
		t.Errorf("Purphoros (token payoff) missing from tokens cluster: %v", tokens.Cards)
	}
}

// TestComputeSynergyClusters_BalancedScoresHigher asserts the core
// refinement: a 4-card cluster with 2 producers + 2 payoffs scores
// higher than a 4-card cluster of 4 producers. Pre-R60 they both
// scored 6 (C(4,2)); post-R60 the balanced cluster scores 10
// (4 mixed pairs × 2 + 2 same-side pairs × 1).
func TestComputeSynergyClusters_BalancedScoresHigher(t *testing.T) {
	balanced := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Empty the Warrens", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Purphoros, God of the Forge",
				[]string{"etb", "opponent_pain"}, nil),
			makeProfileWithTriggers("Impact Tremors",
				[]string{"etb", "opponent_pain"}, nil),
		}, 10),
	}
	skewed := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Empty the Warrens", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Dragon Fodder", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Goblin Rabblemaster", nil, []ResourceType{ResToken}),
		}, 10),
	}

	dpBalanced := &DeckProfile{}
	computeSynergyClusters(dpBalanced, balanced, stubOracle())
	dpSkewed := &DeckProfile{}
	computeSynergyClusters(dpSkewed, skewed, stubOracle())

	getTokens := func(dp *DeckProfile) *SynergyCluster {
		for i := range dp.SynergyClusters {
			if dp.SynergyClusters[i].Theme == "tokens" {
				return &dp.SynergyClusters[i]
			}
		}
		return nil
	}
	balCluster := getTokens(dpBalanced)
	skwCluster := getTokens(dpSkewed)
	if balCluster == nil || skwCluster == nil {
		t.Fatalf("missing tokens cluster (balanced=%v, skewed=%v)", balCluster, skwCluster)
	}
	if balCluster.Score <= skwCluster.Score {
		t.Errorf("balanced cluster score (%d) should exceed all-producer cluster score (%d)",
			balCluster.Score, skwCluster.Score)
	}
	// Concrete expectations from the rolesPairScore matrix:
	//   balanced 2P + 2Pf: 2×2 mixed=4 pairs ×2 + 2 same-side pairs ×1 = 10
	//   skewed   4P:       6 pairs ×1 = 6
	if balCluster.Score != 10 {
		t.Errorf("balanced tokens cluster: want score 10, got %d", balCluster.Score)
	}
	if skwCluster.Score != 6 {
		t.Errorf("all-producer tokens cluster: want score 6, got %d", skwCluster.Score)
	}
}

// TestComputeSynergyClusters_UnknownThemeFallsBackToFlatCount asserts
// that themes without a producer/payoff dichotomy (landfall here) keep
// the pre-R60 flat C(n, 2) score shape — the refinement should never
// regress these.
func TestComputeSynergyClusters_UnknownThemeFallsBackToFlatCount(t *testing.T) {
	report := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			makeProfileWithTriggers("Tireless Tracker", []string{"landfall"}, nil),
			makeProfileWithTriggers("Avenger of Zendikar", []string{"landfall"}, nil),
			makeProfileWithTriggers("Lotus Cobra", []string{"landfall"}, nil),
			makeProfileWithTriggers("Omnath, Locus of Rage", []string{"landfall"}, nil),
		}, 10),
	}
	dp := &DeckProfile{}
	computeSynergyClusters(dp, report, stubOracle())

	var landfall *SynergyCluster
	for i := range dp.SynergyClusters {
		if dp.SynergyClusters[i].Theme == "landfall" {
			landfall = &dp.SynergyClusters[i]
			break
		}
	}
	if landfall == nil {
		t.Fatalf("expected landfall cluster, got %v", dp.SynergyClusters)
	}
	// 4 cards, no dichotomy → all pairs score 1 → C(4, 2) = 6.
	if landfall.Score != 6 {
		t.Errorf("landfall (no producer/payoff dichotomy) want flat score 6, got %d", landfall.Score)
	}
}
