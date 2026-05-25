package main

import (
	"strings"
	"testing"
)

// makeCompProfile builds a DeckProfile + matching FreyaReport with
// enough scaffolding for CompareDecks. Caller supplies nonland cards
// (which populate the FreyaReport.Profiles for fingerprinting) and
// the headline profile fields driving the diff.
type compProfileSpec struct {
	name             string
	commander        string
	primaryArchetype string
	secondaryArchetype string
	bracket          int
	bracketLabel     string
	manaGrade        string
	rampCount        int
	drawCount        int
	winLineCount     int
	primaryWinLine   string
	nonlandCards     []string
	stars            []string // names that should appear in StarCards
	solids           []string // names in SolidCards
	removal          int
	counterspells    int
	boardWipes       int
	tutors           int
	clusters         int
}

func makeCompProfile(spec compProfileSpec) (*DeckProfile, *FreyaReport) {
	dp := &DeckProfile{
		DeckName:           spec.name,
		Commander:          spec.commander,
		PrimaryArchetype:   spec.primaryArchetype,
		SecondaryArchetype: spec.secondaryArchetype,
		Bracket:            spec.bracket,
		BracketLabel:       spec.bracketLabel,
		ManaBaseGrade:      spec.manaGrade,
		RampCount:          spec.rampCount,
		DrawCount:          spec.drawCount,
		WinLineCount:       spec.winLineCount,
		PrimaryWinLine:     spec.primaryWinLine,
	}
	for _, n := range spec.stars {
		dp.StarCards = append(dp.StarCards, CardQuality{Name: n, Tier: "star"})
	}
	for _, n := range spec.solids {
		dp.SolidCards = append(dp.SolidCards, CardQuality{Name: n, Tier: "solid"})
	}
	for i := 0; i < spec.clusters; i++ {
		dp.SynergyClusters = append(dp.SynergyClusters, SynergyCluster{
			Name: "Cluster", Theme: "theme", Score: 5, MemberCount: 4,
		})
	}

	report := &FreyaReport{
		NonLandTutorCount: spec.tutors,
		RemovalCount:      spec.removal,
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: spec.counterspells,
				RoleBoardWipe:    spec.boardWipes,
			},
			TotalCards: len(spec.nonlandCards),
		},
	}
	for _, n := range spec.nonlandCards {
		report.Profiles = append(report.Profiles, CardProfile{Name: n, IsLand: false})
	}
	return dp, report
}

// TestCompareDecks_SameCommanderHeadline pins the headline use case:
// two builds of the same commander → SameCommander=true, the verdict
// opens with "Two {commander} builds." framing, and the comparison
// surfaces the bracket / density / overlap diff so a Decks-screen
// panel can render it directly.
func TestCompareDecks_SameCommanderHeadline(t *testing.T) {
	a, repA := makeCompProfile(compProfileSpec{
		name: "Iroh A", commander: "Iroh, Avatar of Fire",
		primaryArchetype: "Spellslinger", bracket: 4, bracketLabel: "Optimized",
		manaGrade: "A", rampCount: 12, drawCount: 11, winLineCount: 3,
		primaryWinLine: "Storm + Grapeshot",
		tutors:         8, removal: 7, counterspells: 5, boardWipes: 3,
		clusters:       2,
		nonlandCards: []string{
			"sol ring", "rhystic study", "demonic tutor", "vampiric tutor",
			"counterspell", "force of will", "grapeshot", "thoracle",
			"birgi", "underworld breach",
		},
		stars: []string{"thoracle", "underworld breach"},
	})
	b, repB := makeCompProfile(compProfileSpec{
		name: "Iroh B", commander: "Iroh, Avatar of Fire",
		primaryArchetype: "Spellslinger", bracket: 3, bracketLabel: "Upgraded",
		manaGrade: "C", rampCount: 8, drawCount: 9, winLineCount: 1,
		primaryWinLine: "Grapeshot",
		tutors:         3, removal: 6, counterspells: 4, boardWipes: 1,
		clusters:       1,
		nonlandCards: []string{
			"sol ring", "rhystic study", "counterspell", "grapeshot",
			"approach of the second sun", "thousand-year storm", "birgi",
			"goblin electromancer", "young pyromancer", "guttersnipe",
		},
		stars: []string{"approach of the second sun", "thousand-year storm"},
	})

	cmp := CompareDecks(a, b, repA, repB)
	if cmp == nil {
		t.Fatal("CompareDecks returned nil")
	}
	if !cmp.SameCommander {
		t.Errorf("expected SameCommander=true for two Iroh builds, got false")
	}
	if cmp.Commander != "Iroh, Avatar of Fire" {
		t.Errorf("Commander = %q, want 'Iroh, Avatar of Fire'", cmp.Commander)
	}
	if !strings.Contains(cmp.Verdict, "Iroh, Avatar of Fire") {
		t.Errorf("verdict should name the shared commander, got %q", cmp.Verdict)
	}
	if cmp.BracketDelta != -1 {
		t.Errorf("BracketDelta = %d, want -1 (B is bracket 3, A is bracket 4)", cmp.BracketDelta)
	}
	// A is the higher-power build.
	if !strings.Contains(cmp.Verdict, "Iroh A is the higher-power build") {
		t.Errorf("verdict should call out A as the tighter build, got %q", cmp.Verdict)
	}
}

// TestCompareDecks_CardOverlapPartitioning pins that shared / only-A
// / only-B card buckets are correct counts, and the sample lists
// surface meaningful uniques (star cards first).
func TestCompareDecks_CardOverlapPartitioning(t *testing.T) {
	a, repA := makeCompProfile(compProfileSpec{
		name: "A", primaryArchetype: "Combo", bracket: 4,
		nonlandCards: []string{"shared1", "shared2", "shared3", "only-a-1", "only-a-2"},
		stars:        []string{"only-a-1"},
	})
	b, repB := makeCompProfile(compProfileSpec{
		name: "B", primaryArchetype: "Combo", bracket: 4,
		nonlandCards: []string{"shared1", "shared2", "shared3", "only-b-1", "only-b-2", "only-b-3"},
		stars:        []string{"only-b-3"},
	})
	cmp := CompareDecks(a, b, repA, repB)
	if cmp.SharedNonlandCount != 3 {
		t.Errorf("SharedNonlandCount = %d, want 3", cmp.SharedNonlandCount)
	}
	if cmp.OnlyInACount != 2 {
		t.Errorf("OnlyInACount = %d, want 2", cmp.OnlyInACount)
	}
	if cmp.OnlyInBCount != 3 {
		t.Errorf("OnlyInBCount = %d, want 3", cmp.OnlyInBCount)
	}
	// Star uniques first.
	if len(cmp.UniqueToA) == 0 || cmp.UniqueToA[0] != "only-a-1" {
		t.Errorf("UniqueToA should lead with star card 'only-a-1', got %v", cmp.UniqueToA)
	}
	if len(cmp.UniqueToB) == 0 || cmp.UniqueToB[0] != "only-b-3" {
		t.Errorf("UniqueToB should lead with star card 'only-b-3', got %v", cmp.UniqueToB)
	}
}

// TestCompareDecks_DensityDiffSignedDelta pins that DensityDiff
// rows carry signed Delta = ValueB − ValueA. A has +ramp, B has
// +tutors → ramp.Delta is negative (B has fewer), tutors.Delta is
// positive (B has more).
func TestCompareDecks_DensityDiffSignedDelta(t *testing.T) {
	a, repA := makeCompProfile(compProfileSpec{
		name: "A", primaryArchetype: "Combo", bracket: 4,
		rampCount:    14,
		drawCount:    8,
		tutors:       2,
		removal:      6,
		counterspells: 4,
		boardWipes:   2,
		winLineCount: 2,
		clusters:     1,
		nonlandCards: []string{"a"},
	})
	b, repB := makeCompProfile(compProfileSpec{
		name: "B", primaryArchetype: "Combo", bracket: 4,
		rampCount:    9,
		drawCount:    11,
		tutors:       8,
		removal:      4,
		counterspells: 5,
		boardWipes:   2,
		winLineCount: 4,
		clusters:     3,
		nonlandCards: []string{"b"},
	})
	cmp := CompareDecks(a, b, repA, repB)

	rowByMetric := map[string]DensityRow{}
	for _, r := range cmp.DensityDiff {
		rowByMetric[r.Metric] = r
	}
	cases := []struct {
		metric    string
		wantDelta int
	}{
		{"ramp", -5}, // B has fewer
		{"draw", 3},
		{"tutors", 6}, // B has way more
		{"interaction", -1}, // B: removal 4+counters 5 = 9; A: 6+4=10
		{"board_wipes", 0},
		{"win_lines", 2},
		{"clusters", 2},
	}
	for _, tc := range cases {
		row, ok := rowByMetric[tc.metric]
		if !ok {
			t.Errorf("density row %q missing from output", tc.metric)
			continue
		}
		if row.Delta != tc.wantDelta {
			t.Errorf("density[%s].Delta = %d, want %d", tc.metric, row.Delta, tc.wantDelta)
		}
	}
	// Note framing: positive deltas describe B's edge; negative
	// deltas flip to describe A's edge.
	rampNote := rowByMetric["ramp"].Note
	if !strings.Contains(rampNote, "A has more ramp") {
		t.Errorf("ramp note should describe A's edge (delta < 0), got %q", rampNote)
	}
	tutorNote := rowByMetric["tutors"].Note
	if !strings.Contains(tutorNote, "B has more tutors") {
		t.Errorf("tutors note should describe B's edge (delta > 0), got %q", tutorNote)
	}
}

// TestCompareDecks_ArchetypeMatchLadder pins that the archetype-match
// score uses the same 1.0 / 0.5 / 0.0 ladder as similarity.go: same
// primary → 1.0, primary↔secondary crossover → 0.5, disjoint → 0.0.
func TestCompareDecks_ArchetypeMatchLadder(t *testing.T) {
	cases := []struct {
		primA, secA, primB, secB string
		want                      float64
	}{
		{"Combo", "", "Combo", "", 1.0},
		{"Combo", "Control", "Control", "Storm", 0.5},
		{"Combo", "", "Voltron", "", 0.0},
	}
	for _, tc := range cases {
		a, repA := makeCompProfile(compProfileSpec{
			name: "A", primaryArchetype: tc.primA, secondaryArchetype: tc.secA,
			bracket: 4, nonlandCards: []string{"x"},
		})
		b, repB := makeCompProfile(compProfileSpec{
			name: "B", primaryArchetype: tc.primB, secondaryArchetype: tc.secB,
			bracket: 4, nonlandCards: []string{"y"},
		})
		cmp := CompareDecks(a, b, repA, repB)
		if cmp.ArchetypeMatch != tc.want {
			t.Errorf("archetype %q↔%q vs %q↔%q: match=%.2f, want %.2f",
				tc.primA, tc.secA, tc.primB, tc.secB, cmp.ArchetypeMatch, tc.want)
		}
	}
}

// TestCompareDecks_CrossCommanderFraming pins that when commanders
// differ but archetypes match, the verdict opens with "Two X decks
// across different commanders" rather than the same-commander
// shorthand.
func TestCompareDecks_CrossCommanderFraming(t *testing.T) {
	a, repA := makeCompProfile(compProfileSpec{
		name: "Krenko", commander: "Krenko, Mob Boss",
		primaryArchetype: "Aggro / Go Wide", bracket: 3,
		nonlandCards: []string{"x"},
	})
	b, repB := makeCompProfile(compProfileSpec{
		name: "Krava", commander: "Krava the Spider",
		primaryArchetype: "Aggro / Go Wide", bracket: 3,
		nonlandCards: []string{"y"},
	})
	cmp := CompareDecks(a, b, repA, repB)
	if cmp.SameCommander {
		t.Errorf("different commanders should yield SameCommander=false, got true")
	}
	if !strings.Contains(cmp.Verdict, "different commanders") {
		t.Errorf("verdict should mention 'different commanders', got %q", cmp.Verdict)
	}
}

// TestCompareDecks_DisjointArchetypeVerdict pins the third opening:
// when archetypes differ entirely, the verdict says so up-front.
func TestCompareDecks_DisjointArchetypeVerdict(t *testing.T) {
	a, repA := makeCompProfile(compProfileSpec{
		name: "Combo", primaryArchetype: "Combo", bracket: 4,
		nonlandCards: []string{"x"},
	})
	b, repB := makeCompProfile(compProfileSpec{
		name: "Voltron", primaryArchetype: "Voltron", bracket: 4,
		nonlandCards: []string{"y"},
	})
	cmp := CompareDecks(a, b, repA, repB)
	if !strings.Contains(cmp.Verdict, "different archetypes") {
		t.Errorf("verdict should call out disjoint archetypes, got %q", cmp.Verdict)
	}
}

// TestCompareDecks_NilSafe pins that nil inputs don't panic and
// return nil cleanly so the Decks screen can short-circuit rendering.
func TestCompareDecks_NilSafe(t *testing.T) {
	dp := &DeckProfile{}
	if got := CompareDecks(nil, dp, nil, nil); got != nil {
		t.Errorf("nil A should return nil, got %+v", got)
	}
	if got := CompareDecks(dp, nil, nil, nil); got != nil {
		t.Errorf("nil B should return nil, got %+v", got)
	}
	if got := FormatDeckComparison(nil); got != "" {
		t.Errorf("FormatDeckComparison(nil) should return empty string, got %q", got)
	}
}

// TestCompareDecks_FormatRendersAllSections pins that the text
// renderer includes every section the Decks-screen panel needs:
// the verdict, the side-by-side archetype/bracket/mana/win-line
// block, the density table, and the card overlap summary.
func TestCompareDecks_FormatRendersAllSections(t *testing.T) {
	a, repA := makeCompProfile(compProfileSpec{
		name: "Atraxa A", commander: "Atraxa, Praetors' Voice",
		primaryArchetype: "Counters Matter", secondaryArchetype: "Superfriends",
		bracket: 4, bracketLabel: "Optimized", manaGrade: "A",
		rampCount: 12, drawCount: 10, winLineCount: 3,
		primaryWinLine: "Counters + Doubling Season",
		tutors: 5, removal: 6, counterspells: 4, boardWipes: 2, clusters: 2,
		nonlandCards: []string{"sol ring", "atraxa", "doubling season", "vorinclex"},
		stars:        []string{"doubling season"},
	})
	b, repB := makeCompProfile(compProfileSpec{
		name: "Atraxa B", commander: "Atraxa, Praetors' Voice",
		primaryArchetype: "Counters Matter", bracket: 3, bracketLabel: "Upgraded",
		manaGrade: "B",
		rampCount: 10, drawCount: 8, winLineCount: 2,
		primaryWinLine: "Inexorable Tide + planeswalkers",
		tutors: 3, removal: 5, counterspells: 2, boardWipes: 1, clusters: 1,
		nonlandCards: []string{"sol ring", "atraxa", "inexorable tide"},
		stars:        []string{"inexorable tide"},
	})
	cmp := CompareDecks(a, b, repA, repB)
	out := FormatDeckComparison(cmp)

	wantedSubstrings := []string{
		"Deck Comparison:",
		"Atraxa A",
		"Atraxa B",
		"Commander: Atraxa, Praetors' Voice (both)",
		"Archetype:",
		"Bracket:",
		"Mana Base:",
		"A  vs  B",
		"Win Lines:",
		"Density:",
		"ramp",
		"draw",
		"tutors",
		"interaction",
		"Card Overlap:",
		"Only in Atraxa A:",
		"Only in Atraxa B:",
		"doubling season",  // A's star unique
		"inexorable tide",  // B's star unique
	}
	for _, want := range wantedSubstrings {
		if !strings.Contains(out, want) {
			t.Errorf("FormatDeckComparison output missing %q. Got:\n%s", want, out)
		}
	}
}

// TestCompareDecks_SharedSampleAlphabeticalAndCapped pins that the
// SharedSample is sorted alphabetical (deterministic) and capped at
// 12 — guards against map-iter nondeterminism leaking through.
func TestCompareDecks_SharedSampleAlphabeticalAndCapped(t *testing.T) {
	shared := []string{
		"zebra", "apple", "mango", "banana", "kiwi", "fig",
		"grape", "honeydew", "lime", "orange", "pear", "quince",
		"raspberry", "strawberry", "tangerine",
	}
	a, repA := makeCompProfile(compProfileSpec{
		name: "A", primaryArchetype: "Combo", bracket: 4, nonlandCards: shared,
	})
	b, repB := makeCompProfile(compProfileSpec{
		name: "B", primaryArchetype: "Combo", bracket: 4, nonlandCards: shared,
	})
	cmp := CompareDecks(a, b, repA, repB)
	if len(cmp.SharedSample) != 12 {
		t.Errorf("SharedSample should cap at 12, got %d", len(cmp.SharedSample))
	}
	for i := 1; i < len(cmp.SharedSample); i++ {
		if cmp.SharedSample[i] < cmp.SharedSample[i-1] {
			t.Errorf("SharedSample not alphabetical: %q after %q",
				cmp.SharedSample[i], cmp.SharedSample[i-1])
		}
	}
	if cmp.SharedSample[0] != "apple" {
		t.Errorf("first shared = %q, want 'apple'", cmp.SharedSample[0])
	}
}

// TestCompareDecks_EqualBracketVerdict pins that when brackets match,
// the verdict says so rather than awkwardly emitting "A is higher
// than A" or similar nonsense.
func TestCompareDecks_EqualBracketVerdict(t *testing.T) {
	a, repA := makeCompProfile(compProfileSpec{
		name: "A", primaryArchetype: "Combo", bracket: 4,
		nonlandCards: []string{"x"},
	})
	b, repB := makeCompProfile(compProfileSpec{
		name: "B", primaryArchetype: "Combo", bracket: 4,
		nonlandCards: []string{"y"},
	})
	cmp := CompareDecks(a, b, repA, repB)
	if !strings.Contains(cmp.Verdict, "Both target bracket 4") {
		t.Errorf("equal brackets should produce 'Both target bracket N' framing, got %q", cmp.Verdict)
	}
}
