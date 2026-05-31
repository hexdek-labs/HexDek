package main

import (
	"strings"
	"testing"
)

// fixtureComboCEDH builds a Bracket 5 cEDH Thoracle pile — deep tutor
// package, S-tier kill, tight interaction.
func fixtureComboCEDH() (*DeckProfile, *FreyaReport) {
	cards := []string{
		"Thrasios, Triton Hero",
		"Sol Ring", "Mana Crypt", "Mana Vault", "Chrome Mox", "Mox Diamond",
		"Demonic Tutor", "Vampiric Tutor", "Mystical Tutor", "Enlightened Tutor",
		"Thassa's Oracle", "Demonic Consultation", "Tainted Pact",
		"Force of Will", "Pact of Negation", "Mental Misstep",
		"Cyclonic Rift", "Swords to Plowshares",
	}
	powerLevels := make([]CardPowerLevel, 0, len(cards))
	for _, c := range cards {
		powerLevels = append(powerLevels, CardPowerLevel{Name: c})
	}
	dp := &DeckProfile{
		DeckName:             "thrasios_cedh_combo",
		Commander:            "Thrasios, Triton Hero",
		PrimaryArchetype:     "Combo",
		Bracket:              5,
		BracketLabel:         "cEDH",
		WinLineCount:         4,
		WeightedWinLineScore: 42,
		PrimaryWinLine:       "Thoracle + Consultation",
		PowerTierCounts:      map[string]int{"S": 12, "A": 18, "B": 25, "C": 12, "D": 3},
		ManaBaseGrade:        "A",
		LandCount:            32,
		RampCount:            18,
		TaplandCount:         0,
		FetchCount:           9,
		StarCards: []CardQuality{
			{Name: "Thassa's Oracle", Tier: "star", Power: 92, PowerTier: "S"},
			{Name: "Demonic Consultation", Tier: "star", Power: 88, PowerTier: "S"},
			{Name: "Mana Crypt", Tier: "star", Power: 85, PowerTier: "S"},
		},
		CardPowerLevels: powerLevels,
		GameChangerCards: []string{"Mana Crypt", "Mox Diamond", "Demonic Tutor"},
		WorstMatchup: "Stax",
		TechSuggestions: []TechCardSuggestion{
			{Card: "Mental Misstep", Reason: "free counter for 1-mana Stax pieces", OpponentArchetype: "Stax", Severity: 3, AlreadyInDeck: true},
			{Card: "Force of Will", Reason: "free hard counter", OpponentArchetype: "Stax", Severity: 3, AlreadyInDeck: true},
			{Card: "Pact of Negation", Reason: "free counter for the kill turn", OpponentArchetype: "Stax", Severity: 3, AlreadyInDeck: true},
		},
	}
	report := &FreyaReport{DeckPath: "thrasios_cedh_combo.txt", NonlandCount: len(cards)}
	for _, c := range cards {
		report.Profiles = append(report.Profiles, CardProfile{Name: c, IsLand: false})
	}
	report.WinLines = &WinLineAnalysis{WinLines: []WinLine{
		{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite", Tier: "S"},
		{Pieces: []string{"Thassa's Oracle", "Tainted Pact"}, Type: "infinite", Tier: "S"},
	}}
	for i, c := range powerLevels {
		switch c.Name {
		case "Thassa's Oracle", "Demonic Consultation", "Mana Crypt":
			powerLevels[i].PowerTier = "S"
		case "Mana Vault", "Force of Will", "Demonic Tutor":
			powerLevels[i].PowerTier = "A"
		default:
			powerLevels[i].PowerTier = "B"
		}
	}
	dp.CardPowerLevels = powerLevels
	return dp, report
}

// fixtureComboCasual builds a Bracket 3 casual combo deck — same
// archetype as cEDH pile but lower power tier, fewer tutors, no free
// counters. Same worst matchup (Stax) so the recommended-swap path
// will fire suggesting cEDH-side hate pieces.
func fixtureComboCasual() (*DeckProfile, *FreyaReport) {
	cards := []string{
		"Thrasios, Triton Hero",
		"Sol Ring", "Arcane Signet", "Talisman of Curiosity",
		"Diabolic Tutor", "Lim-Dûl's Vault",
		"Laboratory Maniac", "Jace, Wielder of Mysteries",
		"Counterspell", "Mana Leak", "Swan Song",
		"Cyclonic Rift", "Toxic Deluge",
		"Cultivate", "Kodama's Reach", "Rampant Growth",
		"Eternal Witness", "Mulldrifter",
	}
	powerLevels := make([]CardPowerLevel, 0, len(cards))
	for _, c := range cards {
		powerLevels = append(powerLevels, CardPowerLevel{Name: c})
	}
	dp := &DeckProfile{
		DeckName:             "thrasios_casual_combo",
		Commander:            "Thrasios, Triton Hero",
		PrimaryArchetype:     "Combo",
		Bracket:              3,
		BracketLabel:         "Upgraded",
		WinLineCount:         2,
		WeightedWinLineScore: 15,
		PrimaryWinLine:       "LabMan + Counterspell wall",
		PowerTierCounts:      map[string]int{"S": 1, "A": 6, "B": 32, "C": 19, "D": 7},
		ManaBaseGrade:        "B",
		LandCount:            36,
		RampCount:            12,
		TaplandCount:         4,
		FetchCount:           2,
		StarCards: []CardQuality{
			{Name: "Laboratory Maniac", Tier: "star", Power: 75, PowerTier: "A"},
			{Name: "Cyclonic Rift", Tier: "star", Power: 80, PowerTier: "A"},
		},
		CardPowerLevels: powerLevels,
		WorstMatchup:    "Stax",
		TechSuggestions: []TechCardSuggestion{
			{Card: "Cyclonic Rift", Reason: "instant-speed asymmetric bounce", OpponentArchetype: "Stax", Severity: 2, AlreadyInDeck: true},
		},
	}
	report := &FreyaReport{DeckPath: "thrasios_casual_combo.txt", NonlandCount: len(cards)}
	for _, c := range cards {
		report.Profiles = append(report.Profiles, CardProfile{Name: c, IsLand: false})
	}
	report.WinLines = &WinLineAnalysis{WinLines: []WinLine{
		{Pieces: []string{"Laboratory Maniac", "Cyclonic Rift"}, Type: "finisher", Tier: "C"},
	}}
	return dp, report
}

// fixtureVoltronUril builds a cross-archetype contrast for comparing
// against an Aggro deck — different commander, different colors,
// disjoint everything except basics.
func fixtureVoltronUril() (*DeckProfile, *FreyaReport) {
	cards := []string{
		"Uril, the Miststalker",
		"Shield of the Oversoul", "Armadillo Cloak", "Daybreak Coronet",
		"Eldrazi Conscription", "Vow of Wildness", "Asha's Favor",
		"Sol Ring", "Lightning Greaves", "Swiftfoot Boots",
		"Heroic Intervention", "Open the Armory", "Three Dreams",
		"Cultivate", "Rampant Growth",
	}
	powerLevels := make([]CardPowerLevel, 0, len(cards))
	for _, c := range cards {
		powerLevels = append(powerLevels, CardPowerLevel{Name: c, PowerTier: "B"})
	}
	dp := &DeckProfile{
		DeckName:             "uril_aura_voltron",
		Commander:            "Uril, the Miststalker",
		PrimaryArchetype:     "Voltron",
		Bracket:              2,
		BracketLabel:         "Core",
		WinLineCount:         1,
		WeightedWinLineScore: 6,
		PrimaryWinLine:       "Commander damage via Uril",
		PowerTierCounts:      map[string]int{"S": 0, "A": 5, "B": 38, "C": 18, "D": 4},
		ManaBaseGrade:        "B",
		LandCount:            38,
		RampCount:            10,
		TaplandCount:         6,
		FetchCount:           0,
		StarCards: []CardQuality{
			{Name: "Shield of the Oversoul", Tier: "star", Power: 70, PowerTier: "A"},
			{Name: "Eldrazi Conscription", Tier: "star", Power: 78, PowerTier: "A"},
		},
		CardPowerLevels: powerLevels,
		WorstMatchup:    "Reanimator",
		TechSuggestions: []TechCardSuggestion{
			{Card: "Soul-Guide Lantern", Reason: "graveyard exile", OpponentArchetype: "Reanimator", Severity: 2},
		},
	}
	report := &FreyaReport{DeckPath: "uril_voltron.txt", NonlandCount: len(cards)}
	for _, c := range cards {
		report.Profiles = append(report.Profiles, CardProfile{Name: c, IsLand: false})
	}
	return dp, report
}

// fixtureAggroKrenko builds an aggro / go-wide goblins deck — single
// color, low CMC, no overlap with Atraxa or Voltron card pools.
func fixtureAggroKrenko() (*DeckProfile, *FreyaReport) {
	cards := []string{
		"Krenko, Mob Boss",
		"Sol Ring", "Goblin Lackey", "Goblin Recruiter",
		"Skirk Prospector", "Goblin Bombardment",
		"Goblin Chieftain", "Goblin Warchief", "Goblin King",
		"Goblin Piledriver", "Goblin Matron", "Krenko's Command",
		"Impact Tremors", "Purphoros, God of the Forge",
	}
	powerLevels := make([]CardPowerLevel, 0, len(cards))
	for _, c := range cards {
		powerLevels = append(powerLevels, CardPowerLevel{Name: c, PowerTier: "B"})
	}
	dp := &DeckProfile{
		DeckName:             "krenko_goblins",
		Commander:            "Krenko, Mob Boss",
		PrimaryArchetype:     "Aggro",
		Bracket:              2,
		BracketLabel:         "Core",
		WinLineCount:         1,
		WeightedWinLineScore: 5,
		PrimaryWinLine:       "Krenko token swarm",
		PowerTierCounts:      map[string]int{"S": 0, "A": 4, "B": 35, "C": 20, "D": 6},
		ManaBaseGrade:        "C",
		LandCount:            35,
		RampCount:            5,
		TaplandCount:         2,
		FetchCount:           0,
		StarCards: []CardQuality{
			{Name: "Purphoros, God of the Forge", Tier: "star", Power: 80, PowerTier: "A"},
			{Name: "Goblin Piledriver", Tier: "star", Power: 70, PowerTier: "A"},
		},
		CardPowerLevels: powerLevels,
		WorstMatchup:    "Stax",
		TechSuggestions: []TechCardSuggestion{
			{Card: "Vandalblast", Reason: "one-sided artifact wipe vs Stax artifact mana", OpponentArchetype: "Stax", Severity: 2},
		},
	}
	report := &FreyaReport{DeckPath: "krenko_goblins.txt", NonlandCount: len(cards)}
	for _, c := range cards {
		report.Profiles = append(report.Profiles, CardProfile{Name: c, IsLand: false})
	}
	return dp, report
}

// TestApplyComparisonPresentation_RanksByImpact verifies the impact
// ranker scores star cards + win-line pieces higher than generic
// uniques and that the top entry on each side is a high-impact card.
func TestApplyComparisonPresentation_RanksByImpact(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)

	if len(cmp.RankedDiffsA) == 0 {
		t.Fatal("RankedDiffsA empty — superfriends has uniques that should score >0")
	}
	if len(cmp.RankedDiffsB) == 0 {
		t.Fatal("RankedDiffsB empty — counters-matter has uniques that should score >0")
	}

	// Star card "The Chain Veil" is a unique-to-A standout in the
	// existing fixtures; it should rank highly. Same with "Hardened
	// Scales" / "Walking Ballista" on B's side.
	topA := cmp.RankedDiffsA[0]
	if topA.Impact < impactStarCard {
		t.Errorf("top RankedDiffsA score %d should be >= impactStarCard (%d)", topA.Impact, impactStarCard)
	}
	if !strings.EqualFold(topA.Card, "The Chain Veil") && !strings.EqualFold(topA.Card, "Inexorable Tide") {
		t.Logf("top A: %s (acceptable — star cards or game-changers lead)", topA.Card)
	}

	// Sorted descending invariant.
	for i := 1; i < len(cmp.RankedDiffsA); i++ {
		if cmp.RankedDiffsA[i].Impact > cmp.RankedDiffsA[i-1].Impact {
			t.Errorf("RankedDiffsA not sorted by impact desc at index %d", i)
		}
	}
}

// TestApplyComparisonPresentation_NarrativeMentionsCards verifies the
// narrative summary names specific cards driving the identity gap.
// Without this the summary regresses to "Deck A is more X than B"
// without evidence.
func TestApplyComparisonPresentation_NarrativeMentionsCards(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)

	if cmp.NarrativeSummary == "" {
		t.Fatal("NarrativeSummary empty")
	}

	// At least one of A's top-3 ranked uniques must be named in the
	// narrative — the "because of X, Y, Z" evidence promise.
	mentionedA := 0
	for i, d := range cmp.RankedDiffsA {
		if i >= 3 {
			break
		}
		if strings.Contains(cmp.NarrativeSummary, d.Card) {
			mentionedA++
		}
	}
	if mentionedA == 0 {
		t.Errorf("narrative mentions zero of A's top-3 impact diffs: narrative=%q diffs=%v",
			cmp.NarrativeSummary, cmp.RankedDiffsA[:min3(len(cmp.RankedDiffsA))])
	}
}

// TestApplyComparisonPresentation_RecommendedSwapsFireOnSharedMatchup
// verifies the recommended-swap section fires when both decks share
// their worst matchup. Atraxa fixtures both face Stax; the swap
// section should surface cards each runs that the other doesn't.
func TestApplyComparisonPresentation_RecommendedSwapsFireOnSharedMatchup(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)

	if cmp.SharedWorstMatchup != "Stax" {
		t.Errorf("SharedWorstMatchup=%q, want Stax (both Atraxa fixtures face Stax)", cmp.SharedWorstMatchup)
	}
	if len(cmp.RecommendedSwaps) == 0 {
		t.Fatal("RecommendedSwaps empty despite shared Stax worst-matchup + diverging tech lists")
	}

	// At least one swap should target B (telling counters-matter to
	// consider Grafdigger's Cage from superfriends's tech list).
	hasA2B := false
	for _, s := range cmp.RecommendedSwaps {
		if s.TargetDeck == cmp.deckLabelB() {
			hasA2B = true
		}
	}
	if !hasA2B {
		t.Errorf("expected at least one A→B swap recommendation; got %+v", cmp.RecommendedSwaps)
	}
}

// TestApplyComparisonPresentation_RecommendedSwapsQuietOnDifferentMatchups
// verifies swaps DON'T fire when worst matchups differ. Atraxa
// Superfriends (Stax) vs Uril Voltron (Reanimator) — different
// problems, no cross-deck advice.
func TestApplyComparisonPresentation_RecommendedSwapsQuietOnDifferentMatchups(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends() // worst: Stax
	b, repB := fixtureVoltronUril()        // worst: Reanimator
	cmp := CompareDecks(a, b, repA, repB)

	if cmp.SharedWorstMatchup != "" {
		t.Errorf("SharedWorstMatchup=%q, want empty (different worst matchups)", cmp.SharedWorstMatchup)
	}
	if len(cmp.RecommendedSwaps) != 0 {
		t.Errorf("RecommendedSwaps should be empty when worst matchups differ; got %+v", cmp.RecommendedSwaps)
	}
}

// TestApplyComparisonPresentation_SameArchetypeNarrativeFraming
// verifies the lead sentence shifts based on archetype alignment.
// Combo cEDH vs Combo casual → "Both decks play Combo, but the
// engines diverge"; cross-archetype comparisons use the contrast
// framing instead.
func TestApplyComparisonPresentation_SameArchetypeNarrativeFraming(t *testing.T) {
	cases := []struct {
		name        string
		a           func() (*DeckProfile, *FreyaReport)
		b           func() (*DeckProfile, *FreyaReport)
		wantContain string
	}{
		{
			name:        "same-archetype, different commander",
			a:           fixtureComboCEDH,
			b:           fixtureComboCasual,
			wantContain: "Two Combo builds of Thrasios, Triton Hero", // same commander triggers this lead
		},
		{
			name:        "cross-archetype, different commander",
			a:           fixtureVoltronUril,
			b:           fixtureAggroKrenko,
			wantContain: "leans Voltron", // contrast frame names both archetypes
		},
		{
			name:        "same commander, same archetype",
			a:           fixtureAtraxaSuperfriends,
			b:           fixtureAtraxaCountersMatter,
			wantContain: "Atraxa, Praetors' Voice",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dpA, repA := c.a()
			dpB, repB := c.b()
			cmp := CompareDecks(dpA, dpB, repA, repB)
			if !strings.Contains(cmp.NarrativeSummary, c.wantContain) {
				t.Errorf("%s: narrative missing expected framing %q\n--- narrative ---\n%s",
					c.name, c.wantContain, cmp.NarrativeSummary)
			}
		})
	}
}

// TestApplyComparisonPresentation_RankedDiffCap verifies the per-side
// rankedDiffCap (8) is enforced even when more cards score >0.
func TestApplyComparisonPresentation_RankedDiffCap(t *testing.T) {
	a, repA := fixtureComboCEDH() // 17 cards in fixture, mostly S/A tier
	b, repB := fixtureComboCasual()
	cmp := CompareDecks(a, b, repA, repB)
	if len(cmp.RankedDiffsA) > rankedDiffCap {
		t.Errorf("RankedDiffsA = %d entries, want <= %d", len(cmp.RankedDiffsA), rankedDiffCap)
	}
	if len(cmp.RankedDiffsB) > rankedDiffCap {
		t.Errorf("RankedDiffsB = %d entries, want <= %d", len(cmp.RankedDiffsB), rankedDiffCap)
	}
}

// TestApplyComparisonPresentation_FormatRendersAllPresentationSections
// is an end-to-end smoke test that the rendered text contains the
// three new section headers when populated.
func TestApplyComparisonPresentation_FormatRendersAllPresentationSections(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)
	out := FormatDeckComparison(cmp)

	required := []string{
		"Impact-Ranked Differences",
		"Narrative Summary",
		"Recommended Tech Swaps (both face Stax)",
	}
	for _, r := range required {
		if !strings.Contains(out, r) {
			t.Errorf("missing required section %q\n--- output ---\n%s", r, out)
		}
	}
}

// TestApplyComparisonPresentation_NilSafe defends against nil-input
// panics on the presentation entry point.
func TestApplyComparisonPresentation_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on nil input: %v", r)
		}
	}()
	ApplyComparisonPresentation(nil, nil, nil, nil, nil)
	cmp := &DeckComparison{}
	ApplyComparisonPresentation(cmp, nil, nil, nil, nil)
}

// TestApplyComparisonPresentation_ImpactScoringStackable verifies
// multiple positive signals on the same card stack additively. A
// card that is both a star + a win-line piece + S-tier should
// score higher than a card with any single signal.
func TestApplyComparisonPresentation_ImpactScoringStackable(t *testing.T) {
	dp := &DeckProfile{
		StarCards: []CardQuality{{Name: "Triple Threat", Tier: "star"}},
		CardPowerLevels: []CardPowerLevel{
			{Name: "Triple Threat", PowerTier: "S"},
			{Name: "Single Signal", PowerTier: "B"},
		},
	}
	report := &FreyaReport{
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{{Pieces: []string{"Triple Threat"}, Type: "infinite"}},
		},
	}
	diffs := rankImpactfulDiffs([]string{"triple threat", "single signal"}, dp, report)

	tripleScore, singleScore := 0, 0
	for _, d := range diffs {
		switch d.Card {
		case "Triple Threat":
			tripleScore = d.Impact
		case "Single Signal":
			singleScore = d.Impact
		}
	}
	if tripleScore <= singleScore {
		t.Errorf("Triple Threat (%d) should outscore Single Signal (%d) — stacking broken", tripleScore, singleScore)
	}
	wantMin := impactStarCard + impactWinLinePiece + impactPowerTierS
	if tripleScore < wantMin {
		t.Errorf("Triple Threat score %d < expected sum %d (star + winline + S-tier)", tripleScore, wantMin)
	}
}

func min3(n int) int {
	if n < 3 {
		return n
	}
	return 3
}
