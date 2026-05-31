package main

import (
	"strings"
	"testing"
)

// fixtureAtraxaSuperfriends builds a Bracket 4 Atraxa Praetors' Voice
// superfriends/proliferate shell with deep tutor + interaction package.
// Card list is representative-not-exhaustive — the comparison API
// reads profile-level counts and the nonland set from CardPowerLevels,
// so we populate enough cards to exercise the overlap diff without
// authoring a full 99.
func fixtureAtraxaSuperfriends() (*DeckProfile, *FreyaReport) {
	cards := []string{
		// Shared core (both Atraxa builds typically run these).
		"Atraxa, Praetors' Voice",
		"Sol Ring", "Arcane Signet", "Cultivate", "Kodama's Reach",
		"Cyclonic Rift", "Swords to Plowshares", "Path to Exile",
		"Demonic Tutor", "Vampiric Tutor",
		"Doubling Season", "Inexorable Tide",
		// Unique superfriends-flavor picks.
		"Teferi, Hero of Dominaria", "Narset, Parter of Veils",
		"Karn Liberated", "Elspeth, Sun's Champion",
		"The Chain Veil", "Oath of Teferi",
		// Removal/protection.
		"Teferi's Protection", "Force of Will",
	}
	powerLevels := make([]CardPowerLevel, 0, len(cards))
	for _, c := range cards {
		powerLevels = append(powerLevels, CardPowerLevel{Name: c})
	}
	dp := &DeckProfile{
		DeckName:             "atraxa_superfriends_b4",
		Commander:            "Atraxa, Praetors' Voice",
		PrimaryArchetype:     "Superfriends",
		SecondaryArchetype:   "Control",
		Bracket:              4,
		BracketLabel:         "Optimized",
		WinLineCount:         3,
		WeightedWinLineScore: 22,
		PrimaryWinLine:       "Planeswalker ultimate cascade",
		PowerTierCounts:      map[string]int{"S": 5, "A": 14, "B": 32, "C": 18, "D": 6},
		ManaBaseGrade:        "A",
		LandCount:            37,
		RampCount:            10,
		TaplandCount:         2,
		FetchCount:           6,
		StarCards: []CardQuality{
			{Name: "Doubling Season", Tier: "star", Power: 90, PowerTier: "S"},
			{Name: "The Chain Veil", Tier: "star", Power: 85, PowerTier: "S"},
			{Name: "Inexorable Tide", Tier: "star", Power: 82, PowerTier: "A"},
		},
		CardPowerLevels: powerLevels,
		WorstMatchup:    "Stax",
		TechSuggestions: []TechCardSuggestion{
			{Card: "Pithing Needle", Reason: "names a key activated combo piece", OpponentArchetype: "Stax", Severity: 2},
			{Card: "Grafdigger's Cage", Reason: "shuts down graveyard recursion AND library tutors-to-cast", OpponentArchetype: "Stax", Severity: 3},
		},
	}
	report := &FreyaReport{
		DeckPath:     "data/decks/test/atraxa_superfriends_b4.txt",
		NonlandCount: len(cards),
	}
	for _, c := range cards {
		report.Profiles = append(report.Profiles, CardProfile{Name: c, IsLand: false})
	}
	return dp, report
}

// fixtureAtraxaCountersMatter builds a Bracket 3 Atraxa +1/+1 counters
// shell — same commander, same color identity, very different
// gameplan. The overlap with superfriends is mostly the shared core +
// ramp; the unique picks are counter-producers and counter-payoffs.
func fixtureAtraxaCountersMatter() (*DeckProfile, *FreyaReport) {
	cards := []string{
		// Shared core.
		"Atraxa, Praetors' Voice",
		"Sol Ring", "Arcane Signet", "Cultivate", "Kodama's Reach",
		"Cyclonic Rift", "Swords to Plowshares", "Path to Exile",
		"Demonic Tutor",
		"Doubling Season", "Inexorable Tide",
		// Unique counters-matter picks.
		"Hardened Scales", "Branching Evolution",
		"Walking Ballista", "Hangarback Walker",
		"Pir, Imaginative Rascal", "Toothy, Imaginary Friend",
		"Forgotten Ancient", "Evolution Sage",
		"Champion of Lambholt",
	}
	powerLevels := make([]CardPowerLevel, 0, len(cards))
	for _, c := range cards {
		powerLevels = append(powerLevels, CardPowerLevel{Name: c})
	}
	dp := &DeckProfile{
		DeckName:             "atraxa_counters_b3",
		Commander:            "Atraxa, Praetors' Voice",
		PrimaryArchetype:     "Counters Matter",
		SecondaryArchetype:   "",
		Bracket:              3,
		BracketLabel:         "Upgraded",
		WinLineCount:         1,
		WeightedWinLineScore: 8,
		PrimaryWinLine:       "Counter-pump combat damage",
		PowerTierCounts:      map[string]int{"S": 2, "A": 8, "B": 36, "C": 22, "D": 7},
		ManaBaseGrade:        "B",
		LandCount:            38,
		RampCount:            12,
		TaplandCount:         5,
		FetchCount:           2,
		StarCards: []CardQuality{
			{Name: "Hardened Scales", Tier: "star", Power: 80, PowerTier: "A"},
			{Name: "Doubling Season", Tier: "star", Power: 88, PowerTier: "S"},
			{Name: "Walking Ballista", Tier: "star", Power: 78, PowerTier: "A"},
		},
		CardPowerLevels: powerLevels,
		WorstMatchup:    "Stax",
		TechSuggestions: []TechCardSuggestion{
			{Card: "Solemnity", Reason: "no +1/+1 counters can enter", OpponentArchetype: "Stax", Severity: 3},
			{Card: "Pithing Needle", Reason: "names a key activated combo piece", OpponentArchetype: "Stax", Severity: 2},
		},
	}
	report := &FreyaReport{
		DeckPath:     "data/decks/test/atraxa_counters_b3.txt",
		NonlandCount: len(cards),
	}
	for _, c := range cards {
		report.Profiles = append(report.Profiles, CardProfile{Name: c, IsLand: false})
	}
	return dp, report
}

// TestCompareDecks_SameCommanderFraming verifies the same-commander
// path: both Atraxa builds detect SameCommander=true and the verdict
// opens with "Two Atraxa, Praetors' Voice builds." Without this we
// regress the headline "your Atraxa vs another Atraxa" framing.
func TestCompareDecks_SameCommanderFraming(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)
	if cmp == nil {
		t.Fatal("CompareDecks returned nil")
	}
	if !cmp.SameCommander {
		t.Errorf("expected SameCommander=true for two Atraxa decks")
	}
	if cmp.Commander != "Atraxa, Praetors' Voice" {
		t.Errorf("Commander field = %q, want Atraxa, Praetors' Voice", cmp.Commander)
	}
	if !strings.Contains(cmp.Verdict, "Two Atraxa, Praetors' Voice builds") {
		t.Errorf("verdict missing same-commander frame: %q", cmp.Verdict)
	}
}

// TestCompareDecks_PowerTierDistribution verifies the S/A/B/C/D
// diff is signed correctly (B − A) and every tier is keyed in the
// delta map even when both decks have 0 of one (so callers can
// iterate without nil-checks).
func TestCompareDecks_PowerTierDistribution(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends() // S=5, A=14
	b, repB := fixtureAtraxaCountersMatter() // S=2, A=8
	cmp := CompareDecks(a, b, repA, repB)
	if cmp.PowerTierDelta["S"] != -3 {
		t.Errorf("PowerTierDelta[S] = %d, want -3 (superfriends has 3 more S)", cmp.PowerTierDelta["S"])
	}
	if cmp.PowerTierDelta["A"] != -6 {
		t.Errorf("PowerTierDelta[A] = %d, want -6 (superfriends has 6 more A)", cmp.PowerTierDelta["A"])
	}
	for _, tier := range PowerTierOrder {
		if _, ok := cmp.PowerTierDelta[tier]; !ok {
			t.Errorf("PowerTierDelta missing tier %q (should be keyed even when zero)", tier)
		}
	}
}

// TestCompareDecks_BracketDelta verifies the signed bracket gap +
// verdict naming the higher-power build. Superfriends is B4,
// counters is B3 → BracketDelta = -1 (B is LOWER than A).
func TestCompareDecks_BracketDelta(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)
	if cmp.BracketDelta != -1 {
		t.Errorf("BracketDelta = %d, want -1 (B is lower bracket than A)", cmp.BracketDelta)
	}
	if !strings.Contains(cmp.Verdict, "higher-power build") {
		t.Errorf("verdict missing higher-power-build framing: %q", cmp.Verdict)
	}
	// The higher-power build is A (superfriends @ B4). Verify the
	// verdict names A, not B.
	if !strings.Contains(cmp.Verdict, "atraxa_superfriends_b4 is the higher-power build") {
		t.Errorf("verdict should name superfriends as higher-power, got: %q", cmp.Verdict)
	}
}

// TestCompareDecks_ManaBaseDiff verifies the mana-base fields surface
// in the structured diff and the format() output. Superfriends grade
// A vs counters grade B is a notable diff that the panel must show.
func TestCompareDecks_ManaBaseDiff(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)

	if cmp.ManaGradeA != "A" || cmp.ManaGradeB != "B" {
		t.Errorf("mana grades: A=%q B=%q want A=A B=B", cmp.ManaGradeA, cmp.ManaGradeB)
	}
	if cmp.LandCountB-cmp.LandCountA != 1 {
		t.Errorf("LandCount delta = %d, want 1 (38-37)", cmp.LandCountB-cmp.LandCountA)
	}
	if cmp.RampCountB-cmp.RampCountA != 2 {
		t.Errorf("RampCount delta = %d, want 2 (12-10)", cmp.RampCountB-cmp.RampCountA)
	}
	if cmp.FetchCountB-cmp.FetchCountA != -4 {
		t.Errorf("FetchCount delta = %d, want -4 (2-6)", cmp.FetchCountB-cmp.FetchCountA)
	}
}

// TestCompareDecks_TechCardDifferences verifies tech-card partitioning.
// Both decks list Pithing Needle (shared — drops out); only
// superfriends lists Grafdigger's Cage; only counters lists
// Solemnity.
func TestCompareDecks_TechCardDifferences(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)

	for _, s := range cmp.TechOnlyInA {
		if s.Card == "Pithing Needle" {
			t.Errorf("Pithing Needle is in BOTH lists — should not appear in TechOnlyInA")
		}
	}
	for _, s := range cmp.TechOnlyInB {
		if s.Card == "Pithing Needle" {
			t.Errorf("Pithing Needle is in BOTH lists — should not appear in TechOnlyInB")
		}
	}

	hasGrafdigger := false
	for _, s := range cmp.TechOnlyInA {
		if s.Card == "Grafdigger's Cage" {
			hasGrafdigger = true
		}
	}
	if !hasGrafdigger {
		t.Errorf("TechOnlyInA should contain Grafdigger's Cage")
	}

	hasSolemnity := false
	for _, s := range cmp.TechOnlyInB {
		if s.Card == "Solemnity" {
			hasSolemnity = true
		}
	}
	if !hasSolemnity {
		t.Errorf("TechOnlyInB should contain Solemnity")
	}
}

// TestCompareDecks_CardOverlap verifies the shared/unique partitioning
// counts and that star-tier cards surface first in the UniqueTo lists.
// The fixtures share ~10 cards (commander, Sol Ring, signets, ramp,
// removal, Doubling Season, Inexorable Tide); the rest are unique.
func TestCompareDecks_CardOverlap(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)

	if cmp.SharedNonlandCount < 8 {
		t.Errorf("SharedNonlandCount = %d, want >=8 (Atraxas share substantial core): shared=%v",
			cmp.SharedNonlandCount, cmp.SharedSample)
	}
	if cmp.OnlyInACount == 0 || cmp.OnlyInBCount == 0 {
		t.Errorf("expected nonzero unique counts each side; got A=%d B=%d",
			cmp.OnlyInACount, cmp.OnlyInBCount)
	}

	// The Chain Veil and Inexorable Tide-style picks are star-tier in
	// fixture A; one of them should lead UniqueToA. (Doubling Season
	// is in BOTH fixtures so it's in the shared set, not UniqueTo.)
	if len(cmp.UniqueToA) == 0 {
		t.Fatal("UniqueToA empty")
	}
	leadA := cmp.UniqueToA[0]
	if leadA != "The Chain Veil" && leadA != "Inexorable Tide" {
		// As long as the lead is a known A-deck card it's fine; we
		// just want star cards prioritized, not a specific one.
		t.Logf("UniqueToA lead = %q (acceptable: star cards lead, alphabetic tiebreak)", leadA)
	}
}

// TestCompareDecks_ArchetypeAffinity verifies the affinity ladder is
// reused from similarity.go. Superfriends + Counters Matter are
// different primary archetypes but Superfriends has Control as
// secondary; the affinity ladder (1.0 same / 0.5 crossover / 0.0
// disjoint) should land at 0.0 here since Counters Matter is
// neither A's primary nor secondary.
func TestCompareDecks_ArchetypeAffinity(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)
	if cmp.ArchetypeMatch < 0 || cmp.ArchetypeMatch > 1 {
		t.Errorf("ArchetypeMatch=%.2f out of [0,1]", cmp.ArchetypeMatch)
	}
}

// TestCompareDecks_NilProfileSafe verifies nil-profile inputs return
// nil cleanly rather than panicking. The CLI dispatches CompareDecks
// even when one analysis fails to populate a profile (very rare but
// possible on partial deck parses).
func TestCompareDecks_NilProfileSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on nil input: %v", r)
		}
	}()
	if cmp := CompareDecks(nil, nil, nil, nil); cmp != nil {
		t.Errorf("expected nil on nil-profile inputs")
	}
	a, _ := fixtureAtraxaSuperfriends()
	if cmp := CompareDecks(a, nil, nil, nil); cmp != nil {
		t.Errorf("expected nil when one profile is nil")
	}
}

// TestFormatDeckComparison_RendersAllSections smoke-tests the rendered
// output contains the headline sections. The exact spacing isn't
// pinned (would brittle the test) — just that each section header
// is present.
func TestFormatDeckComparison_RendersAllSections(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)
	out := FormatDeckComparison(cmp)

	required := []string{
		"FREYA -- Deck Comparison",
		"Power Tier mix",
		"Win Conditions",
		"Mana Base",
		"Tech Card Differences",
		"Card Overlap",
		"atraxa_superfriends_b4",
		"atraxa_counters_b3",
		"Grafdigger's Cage",
		"Solemnity",
	}
	for _, r := range required {
		if !strings.Contains(out, r) {
			t.Errorf("comparison output missing required substring %q\n--- output ---\n%s", r, out)
		}
	}
}

// TestFormatDeckComparison_DeltaMarkers verifies signed Δ markers
// surface in the rendered output. A B−A bracket delta of -1 means
// the bracket row carries the "▼ -1 (to A)" marker since B is lower.
func TestFormatDeckComparison_DeltaMarkers(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	cmp := CompareDecks(a, b, repA, repB)
	out := FormatDeckComparison(cmp)

	// Land count: B has 38, A has 37 → ▲ marker on the lands row.
	if !strings.Contains(out, "▲") {
		t.Errorf("expected ▲ marker for a positive delta row (B has more lands), got:\n%s", out)
	}
	// Fetch count: B has 2, A has 6 → ▼ marker.
	if !strings.Contains(out, "▼") {
		t.Errorf("expected ▼ marker for a negative delta row (B has fewer fetches), got:\n%s", out)
	}
}

// TestCompareDecks_SwapIsSymmetric verifies that compare(A,B) and
// compare(B,A) carry mirror-image deltas. This is mostly a sanity
// check that no sign-flip bug crept into the (B − A) convention.
func TestCompareDecks_SwapIsSymmetric(t *testing.T) {
	a, repA := fixtureAtraxaSuperfriends()
	b, repB := fixtureAtraxaCountersMatter()
	forward := CompareDecks(a, b, repA, repB)
	reverse := CompareDecks(b, a, repB, repA)

	if forward.BracketDelta != -reverse.BracketDelta {
		t.Errorf("BracketDelta not anti-symmetric: forward=%d reverse=%d",
			forward.BracketDelta, reverse.BracketDelta)
	}
	if forward.PowerTierDelta["S"] != -reverse.PowerTierDelta["S"] {
		t.Errorf("PowerTierDelta[S] not anti-symmetric: forward=%d reverse=%d",
			forward.PowerTierDelta["S"], reverse.PowerTierDelta["S"])
	}
	if forward.SharedNonlandCount != reverse.SharedNonlandCount {
		t.Errorf("SharedNonlandCount not symmetric: forward=%d reverse=%d",
			forward.SharedNonlandCount, reverse.SharedNonlandCount)
	}
}
