package main

import (
	"testing"
)

// TestRolesPairScore covers the role pair-weighting helper in isolation.
// The matrix (R60 round 2 adds Amplifier as the chain-middle role):
//
//	producer × producer    → 1 (same-side pile)
//	producer × payoff      → 2 (complementary engine)
//	payoff × payoff        → 1 (same-side pile)
//	amplifier × producer   → 2 (chain pair)
//	amplifier × payoff     → 2 (chain pair)
//	amplifier × amplifier  → 1 (same-side pile)
//	both × anything        → 2 (slots into either side)
//	unknown × anything     → 1 (Unknown takes precedence — fallback for themes
//	                            without a dichotomy; required so landfall-style
//	                            clusters keep their flat C(n,2) score shape)
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
		{clusterRoleBoth, clusterRoleUnknown, 1},
		{clusterRoleUnknown, clusterRoleProducer, 1},
		{clusterRoleUnknown, clusterRoleUnknown, 1},
		// R60 round 2 amplifier rows
		{clusterRoleAmplifier, clusterRoleProducer, 2},
		{clusterRoleAmplifier, clusterRolePayoff, 2},
		{clusterRoleAmplifier, clusterRoleBoth, 2},
		{clusterRoleAmplifier, clusterRoleAmplifier, 1},
		{clusterRoleAmplifier, clusterRoleUnknown, 1},
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

// TestClassifyClusterRole_DeathValue covers the 3-role split for the
// death_value theme (R60 round 2). The chain is:
//
//	token-maker (Producer: supplies bodies)
//	  → sac outlet (Amplifier: converts bodies to death events)
//	    → dies-trigger (Payoff: rewards the events)
//
// Sac outlets are the central chain-amplifier and short-circuit the
// producer/payoff resolution — a card that's both an outlet and a
// dies-trigger still classifies as Amplifier (the outlet role is
// central to the chain).
func TestClassifyClusterRole_DeathValue(t *testing.T) {
	cases := []struct {
		name string
		p    CardProfile
		want clusterRole
	}{
		{
			"Goblin Bombardment — sac outlet amplifier",
			CardProfile{Name: "Goblin Bombardment", IsOutlet: true},
			clusterRoleAmplifier,
		},
		{
			"Krenko, Mob Boss — token-maker producer",
			CardProfile{Name: "Krenko, Mob Boss", Produces: []ResourceType{ResToken}},
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

// TestClassifyClusterRole_ETBValue covers the 3-role split for the
// etb_value theme (R60 round 2). The chain is:
//
//	HasValueETB creature (Producer: its own ETB is worth re-triggering)
//	  → blinker (Amplifier: re-triggers the ETB)
//	    → triggers-on-other-ETB (Payoff: rewards each ETB event)
//
// Critical distinction: Triggers="etb" only fires for OTHER creatures'
// ETBs (see analysis.go:386 — keyed off "whenever a creature enters"
// phrasing). A card's own ETB ability is HasValueETB, never Triggers="etb".
func TestClassifyClusterRole_ETBValue(t *testing.T) {
	cases := []struct {
		name string
		p    CardProfile
		want clusterRole
	}{
		{
			"Mulldrifter — value-ETB producer",
			CardProfile{Name: "Mulldrifter", HasValueETB: true},
			clusterRoleProducer,
		},
		{
			"Conjurer's Closet — blinker amplifier",
			CardProfile{Name: "Conjurer's Closet", IsBlinker: true},
			clusterRoleAmplifier,
		},
		{
			"Soul Warden — etb payoff",
			CardProfile{Name: "Soul Warden", Triggers: []string{"etb"}},
			clusterRolePayoff,
		},
		{
			"Brago, King Eternal — blinker that's also a triggerer (amplifier wins)",
			CardProfile{Name: "Brago, King Eternal", IsBlinker: true, Triggers: []string{"etb"}},
			clusterRoleAmplifier,
		},
		{
			"Reclamation Sage — Both (value ETB AND etb-trigger payoff)",
			// A card with both signals lands in Both; chain math counts it
			// as one producer and one payoff (see chain-bonus tests).
			CardProfile{Name: "Reclamation Sage", HasValueETB: true, Triggers: []string{"etb"}},
			clusterRoleBoth,
		},
	}
	for _, tc := range cases {
		if got := classifyClusterRole(tc.p, "etb_value"); got != tc.want {
			t.Errorf("classifyClusterRole(%q, etb_value) = %v, want %v",
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

// TestComputeSynergyClusters_DeathValueChainBonus asserts the R60 round 2
// chain-depth bonus: a complete producer→amplifier→payoff triangle
// scores higher than the same cluster with the amplifier replaced by an
// extra payoff. The chain bonus is min(P,A,Pf)*3 and is 0 when any of
// the three roles is missing — so adding the first sac outlet to a
// token+drain pile creates a discontinuous score jump, matching how
// the deck actually starts winning when the chain closes.
//
// Test uses Pawn-of-Ulamog-shape "Both" cards (token-maker + dies-trigger)
// so the death_value cluster forms without the cross-cluster promotion
// gate kicking in.
func TestComputeSynergyClusters_DeathValueChainBonus(t *testing.T) {
	pawnA := CardProfile{
		Name: "Pawn of Ulamog A", TypeLine: "Creature",
		Produces: []ResourceType{ResToken}, Triggers: []string{"dies"},
	}
	pawnB := CardProfile{
		Name: "Pawn of Ulamog B", TypeLine: "Creature",
		Produces: []ResourceType{ResToken}, Triggers: []string{"dies"},
	}
	outlet := CardProfile{Name: "Goblin Bombardment", TypeLine: "Enchantment", IsOutlet: true}
	bloodArtist := makeProfileWithTriggers("Blood Artist", []string{"dies"}, nil)

	// 4 cards with chain: 2 Both + 1 amplifier + 1 payoff.
	// Roles: Both, Both, A, Pf. Pair scores (Both treats as mixed-with-anything = 2):
	//   Both×Both=2, Both×A=2, Both×Pf=2, Both×A=2, Both×Pf=2, A×Pf=2 → 12
	// Chain count: producers = 2 (Both), amps = 1, payoffs = 2 (Both)+1 = 3
	//   min(2,1,3) = 1 → +3 → total 15.
	withChain := &FreyaReport{
		Profiles: padProfiles([]CardProfile{pawnA, pawnB, outlet, bloodArtist}, 10),
	}
	// Same 4-card shape WITHOUT amplifier — swap the outlet for another
	// payoff. Roles: Both, Both, Pf, Pf. Pairs:
	//   Both×Both=2, 4×(Both×Pf)=8, Pf×Pf=1 → 11.
	// Chain count: amps=0 → bonus 0. Total = 11.
	noChain := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			pawnA, pawnB, bloodArtist,
			makeProfileWithTriggers("Zulaport Cutthroat", []string{"dies"}, nil),
		}, 10),
	}

	dpChain := &DeckProfile{}
	computeSynergyClusters(dpChain, withChain, stubOracle())
	dpNoChain := &DeckProfile{}
	computeSynergyClusters(dpNoChain, noChain, stubOracle())

	getDeath := func(dp *DeckProfile) *SynergyCluster {
		for i := range dp.SynergyClusters {
			if dp.SynergyClusters[i].Theme == "death_value" {
				return &dp.SynergyClusters[i]
			}
		}
		return nil
	}
	chainCluster := getDeath(dpChain)
	noChainCluster := getDeath(dpNoChain)
	if chainCluster == nil || noChainCluster == nil {
		t.Fatalf("missing death_value cluster (chain=%v, noChain=%v)",
			chainCluster, noChainCluster)
	}
	if chainCluster.Score <= noChainCluster.Score {
		t.Errorf("chain-complete cluster (%d) should exceed amplifier-missing cluster (%d)",
			chainCluster.Score, noChainCluster.Score)
	}
	if chainCluster.Score != 15 {
		t.Errorf("chain cluster: want 15, got %d", chainCluster.Score)
	}
	if noChainCluster.Score != 11 {
		t.Errorf("no-chain cluster: want 11, got %d", noChainCluster.Score)
	}
}

// TestComputeSynergyClusters_ChainScalesWithCompleteness asserts that
// adding a second complete chain instance to a cluster scales the
// chain bonus linearly (chainCount = min(P, A, Pf), bonus = chainCount*3),
// while an unbalanced cluster with extra producers but only one amplifier
// caps at one complete chain.
func TestComputeSynergyClusters_ChainScalesWithCompleteness(t *testing.T) {
	// 6 cards, 2/2/2 chain — two complete chains. Roles: P,P,A,A,Pf,Pf.
	// Pairs: P×P=1, A×A=1, Pf×Pf=1, 4×(P×A)=8, 4×(P×Pf)=8, 4×(A×Pf)=8
	//   → 3 + 24 = 27. Chain count = min(2,2,2) = 2 → +6 → total 33.
	twoChains := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Bitterblossom", nil, []ResourceType{ResToken}),
			{Name: "Goblin Bombardment", TypeLine: "Enchantment", IsOutlet: true},
			{Name: "Phyrexian Altar", TypeLine: "Artifact", IsOutlet: true},
			makeProfileWithTriggers("Blood Artist", []string{"dies"}, nil),
			makeProfileWithTriggers("Zulaport Cutthroat", []string{"dies"}, nil),
		}, 10),
	}
	// Same 6 cards with one of the amplifiers replaced by an extra
	// producer. Roles: P,P,P,A,Pf,Pf. Pairs same total: 27 (the swap
	// keeps pair counts identical — P×A vs P×P loses 1, but P×P→1
	// already, and the displaced amp's pairs balance out). Actually
	// recompute: 3×P → C(3,2)*1=3 same-side; 2×Pf → 1 same-side;
	// 3×(P×A)=6; 3×(P×Pf)=6; 2×(A×Pf)=4. Total 3+1+6+6+4 = 20.
	// Wait that's not 27. Let me recompute the 2/2/2:
	//   pairs total = C(6,2)=15. role counts:
	//     P×P: C(2,2)=1 → 1×1=1
	//     A×A: 1 → 1×1=1
	//     Pf×Pf: 1 → 1×1=1
	//     P×A: 2*2=4 → 4×2=8
	//     P×Pf: 2*2=4 → 4×2=8
	//     A×Pf: 2*2=4 → 4×2=8
	//   sum: 1+1+1+8+8+8 = 27 ✓
	// For 3P/1A/2Pf:
	//     P×P: C(3,2)=3 → 3×1=3
	//     Pf×Pf: 1 → 1×1=1
	//     P×A: 3*1=3 → 3×2=6
	//     P×Pf: 3*2=6 → 6×2=12
	//     A×Pf: 1*2=2 → 2×2=4
	//   sum: 3+1+6+12+4 = 26
	// Chain count = min(3,1,2) = 1 → +3 → total 29.
	oneChain := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			makeProfileWithTriggers("Krenko, Mob Boss", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Bitterblossom", nil, []ResourceType{ResToken}),
			makeProfileWithTriggers("Dragon Fodder", nil, []ResourceType{ResToken}),
			{Name: "Goblin Bombardment", TypeLine: "Enchantment", IsOutlet: true},
			makeProfileWithTriggers("Blood Artist", []string{"dies"}, nil),
			makeProfileWithTriggers("Zulaport Cutthroat", []string{"dies"}, nil),
		}, 10),
	}

	dpTwo := &DeckProfile{}
	computeSynergyClusters(dpTwo, twoChains, stubOracle())
	dpOne := &DeckProfile{}
	computeSynergyClusters(dpOne, oneChain, stubOracle())

	getDeath := func(dp *DeckProfile) *SynergyCluster {
		for i := range dp.SynergyClusters {
			if dp.SynergyClusters[i].Theme == "death_value" {
				return &dp.SynergyClusters[i]
			}
		}
		return nil
	}
	two := getDeath(dpTwo)
	one := getDeath(dpOne)
	if two == nil || one == nil {
		t.Fatalf("missing death_value cluster (two=%v, one=%v)", two, one)
	}
	if two.Score <= one.Score {
		t.Errorf("two-chain cluster (%d) should exceed one-chain cluster (%d)",
			two.Score, one.Score)
	}
	if two.Score != 33 {
		t.Errorf("two-chain cluster: want 33, got %d", two.Score)
	}
	if one.Score != 29 {
		t.Errorf("one-chain cluster: want 29, got %d", one.Score)
	}
}

// TestComputeSynergyClusters_NoChainBonusWithoutAmplifier verifies that
// a cluster missing the amplifier role earns 0 chain bonus regardless
// of how many producers and payoffs are present. Uses 6 dies-trigger
// cards (all payoffs, no outlet, no Both, no producer) so the cluster
// is unambiguously amplifier-free.
func TestComputeSynergyClusters_NoChainBonusWithoutAmplifier(t *testing.T) {
	// 6 payoffs only. Pairs: C(6,2)=15 × 1 = 15. Chain count = 0.
	report := &FreyaReport{
		Profiles: padProfiles([]CardProfile{
			makeProfileWithTriggers("Blood Artist", []string{"dies"}, nil),
			makeProfileWithTriggers("Zulaport Cutthroat", []string{"dies"}, nil),
			makeProfileWithTriggers("Falkenrath Noble", []string{"dies"}, nil),
			makeProfileWithTriggers("Cruel Celebrant", []string{"dies"}, nil),
			makeProfileWithTriggers("Vito, Thorn of the Dusk Rose", []string{"dies"}, nil),
			makeProfileWithTriggers("Bastion of Remembrance", []string{"dies"}, nil),
		}, 12),
	}
	dp := &DeckProfile{}
	computeSynergyClusters(dp, report, stubOracle())

	var death *SynergyCluster
	for i := range dp.SynergyClusters {
		if dp.SynergyClusters[i].Theme == "death_value" {
			death = &dp.SynergyClusters[i]
			break
		}
	}
	if death == nil {
		t.Fatalf("expected death_value cluster, got %v", dp.SynergyClusters)
	}
	if death.Score != 15 {
		t.Errorf("no-amplifier cluster: want score 15 (pair-only, chain bonus 0), got %d", death.Score)
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
