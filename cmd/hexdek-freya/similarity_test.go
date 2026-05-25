package main

import (
	"math"
	"testing"
)

// makeFingerprint is a fixture helper for similarity tests.
func makeFingerprint(name, commander, primary, secondary string, bracket int, colors []string, nonlandCards []string) *DeckFingerprint {
	fp := &DeckFingerprint{
		Name:               name,
		Commander:          commander,
		PrimaryArchetype:   primary,
		SecondaryArchetype: secondary,
		Bracket:            bracket,
		ColorIdentity:      colors,
		NonlandCards:       map[string]bool{},
	}
	for _, c := range nonlandCards {
		fp.NonlandCards[c] = true
	}
	return fp
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// TestSimilarity_IdenticalDecks pins the upper-bound: scoring a deck
// against an identical-card-list, same-archetype, same-bracket, same-
// color copy yields Overall = 1.0 (or very close — the verdict must
// say "nearly identical").
func TestSimilarity_IdenticalDecks(t *testing.T) {
	cards := []string{"sol ring", "rhystic study", "demonic tutor", "swords to plowshares"}
	a := makeFingerprint("A", "Atraxa", "Combo", "Control", 4, []string{"W", "U", "B", "G"}, cards)
	b := makeFingerprint("B", "Atraxa", "Combo", "Control", 4, []string{"W", "U", "B", "G"}, cards)
	s := SimilarityScoreOf(a, b)
	if !approxEqual(s.Overall, 1.0) {
		t.Errorf("identical decks should score 1.0, got %.4f", s.Overall)
	}
	if s.CardJaccard != 1.0 {
		t.Errorf("identical cardlists: CardJaccard=%.4f, want 1.0", s.CardJaccard)
	}
	if s.Verdict != "nearly identical" {
		t.Errorf("verdict for identical decks = %q, want 'nearly identical'", s.Verdict)
	}
	if s.SharedCount != len(cards) {
		t.Errorf("SharedCount=%d, want %d", s.SharedCount, len(cards))
	}
}

// TestSimilarity_DisjointDecks pins the lower-bound: zero card overlap,
// different archetypes, mono- vs different mono-color, brackets 1 vs 5
// should score very low and verdict "different".
func TestSimilarity_DisjointDecks(t *testing.T) {
	a := makeFingerprint("A", "Krenko", "Aggro / Go Wide", "", 1, []string{"R"},
		[]string{"goblin warchief", "goblin king", "goblin matron"})
	b := makeFingerprint("B", "Thrasios", "Combo / Infinite", "", 5, []string{"U", "G"},
		[]string{"thassa's oracle", "demonic consultation", "tainted pact"})
	s := SimilarityScoreOf(a, b)
	if s.Overall > 0.10 {
		t.Errorf("fully disjoint decks should score <0.10, got %.4f", s.Overall)
	}
	if s.Verdict != "different" {
		t.Errorf("verdict for disjoint decks = %q, want 'different'", s.Verdict)
	}
	if s.SharedCount != 0 {
		t.Errorf("SharedCount=%d, want 0", s.SharedCount)
	}
	if s.BracketDelta != 4 {
		t.Errorf("BracketDelta=%d, want 4", s.BracketDelta)
	}
}

// TestSimilarity_LandsExcludedFromJaccard pins the design decision
// that LANDS are excluded from card Jaccard. Two decks that share
// only basics + Sol Ring should NOT score as similar — basics dominate
// the cardlist and would inflate every cross-archetype pairing.
// The fingerprint builder enforces this: IsLand profiles are dropped
// before being added to NonlandCards. This test confirms the scorer
// only sees the nonlands.
func TestSimilarity_LandsExcludedFromJaccard(t *testing.T) {
	// Both fingerprints' NonlandCards intentionally lack lands —
	// simulating what BuildDeckFingerprint would produce.
	a := makeFingerprint("A", "Krenko", "Aggro", "", 2, []string{"R"},
		[]string{"goblin warchief", "krenko, mob boss", "shared animosity"})
	b := makeFingerprint("B", "Atraxa", "Counters Matter", "", 4, []string{"W", "U", "B", "G"},
		[]string{"doubling season", "deepglow skate", "vorinclex, monstrous raider"})
	s := SimilarityScoreOf(a, b)
	if s.CardJaccard != 0 {
		t.Errorf("no shared nonlands → CardJaccard=%.4f, want 0", s.CardJaccard)
	}
	// Confirm the BuildDeckFingerprint path: lands in the report
	// must NOT survive into NonlandCards.
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Sol Ring", IsLand: false},
			{Name: "Plains", IsLand: true},
			{Name: "Island", IsLand: true},
			{Name: "Krenko, Mob Boss", IsLand: false},
		},
	}
	dp := &DeckProfile{DeckName: "test"}
	fp := BuildDeckFingerprint(dp, report)
	if fp.NonlandCards["plains"] || fp.NonlandCards["island"] {
		t.Errorf("BuildDeckFingerprint must exclude lands, got NonlandCards=%v", fp.NonlandCards)
	}
	if !fp.NonlandCards["sol ring"] || !fp.NonlandCards["krenko, mob boss"] {
		t.Errorf("BuildDeckFingerprint dropped a nonland, got NonlandCards=%v", fp.NonlandCards)
	}
}

// TestSimilarity_ArchetypeAffinityLadder pins the 1.0 / 0.5 / 0.0
// ladder: same primary = 1.0, primary↔secondary crossover = 0.5,
// disjoint = 0.0.
func TestSimilarity_ArchetypeAffinityLadder(t *testing.T) {
	base := makeFingerprint("base", "X", "Combo", "Control", 4, nil, nil)
	cases := []struct {
		name      string
		other     *DeckFingerprint
		wantMatch float64
	}{
		{
			"same primary",
			makeFingerprint("o1", "X", "Combo", "Aggro", 4, nil, nil),
			1.0,
		},
		{
			"other's primary == our secondary",
			makeFingerprint("o2", "X", "Control", "Storm", 4, nil, nil),
			0.5,
		},
		{
			"our primary == other's secondary",
			makeFingerprint("o3", "X", "Stax", "Combo", 4, nil, nil),
			0.5,
		},
		{
			"matching secondaries only",
			makeFingerprint("o4", "X", "Stax", "Control", 4, nil, nil),
			0.5,
		},
		{
			"disjoint",
			makeFingerprint("o5", "X", "Voltron", "Aggro", 4, nil, nil),
			0.0,
		},
		{
			"empty primary on other",
			makeFingerprint("o6", "X", "", "", 4, nil, nil),
			0.0,
		},
	}
	for _, tc := range cases {
		got := archetypeAffinity(base, tc.other)
		if got != tc.wantMatch {
			t.Errorf("%s: archetypeAffinity = %.1f, want %.1f", tc.name, got, tc.wantMatch)
		}
	}
}

// TestSimilarity_BracketAffinityLadder pins the bracket-delta formula:
// same bracket = 1.0, 1 apart = 0.75, 2 apart = 0.5, 3 apart = 0.25,
// 4 apart = 0.0. Clamped at 0 — no negative affinity even for
// out-of-range deltas.
func TestSimilarity_BracketAffinityLadder(t *testing.T) {
	cases := []struct {
		delta int
		want  float64
	}{
		{0, 1.0},
		{1, 0.75},
		{2, 0.5},
		{3, 0.25},
		{4, 0.0},
		{5, 0.0}, // clamped
	}
	for _, tc := range cases {
		a := makeFingerprint("a", "", "Combo", "", 4, nil, nil)
		b := makeFingerprint("b", "", "Combo", "", 4-tc.delta, nil, nil)
		s := SimilarityScoreOf(a, b)
		if !approxEqual(s.BracketAffin, tc.want) {
			t.Errorf("Δbracket=%d: BracketAffin=%.4f, want %.4f", tc.delta, s.BracketAffin, tc.want)
		}
		if s.BracketDelta != tc.delta && tc.delta <= 4 {
			t.Errorf("Δbracket=%d: BracketDelta=%d, want %d", tc.delta, s.BracketDelta, tc.delta)
		}
	}
}

// TestSimilarity_ColorOverlapJaccard pins that color identity uses
// Jaccard, not strict equality. Two colorless decks share the
// colorless mana base → 1.0; partial overlap returns intersection
// over union.
func TestSimilarity_ColorOverlapJaccard(t *testing.T) {
	cases := []struct {
		name    string
		a, b    []string
		want    float64
	}{
		{"identical 4-color", []string{"W", "U", "B", "G"}, []string{"W", "U", "B", "G"}, 1.0},
		{"3-of-4 overlap", []string{"W", "U", "B", "G"}, []string{"W", "U", "B"}, 3.0 / 4.0},
		{"single color match in 5", []string{"R", "G"}, []string{"R"}, 1.0 / 2.0},
		{"disjoint colors", []string{"W"}, []string{"B"}, 0.0},
		{"both colorless", nil, nil, 1.0},
		{"case-insensitive", []string{"w", "u"}, []string{"W", "U"}, 1.0},
	}
	for _, tc := range cases {
		got := setJaccard(tc.a, tc.b)
		if !approxEqual(got, tc.want) {
			t.Errorf("%s: setJaccard(%v, %v) = %.4f, want %.4f", tc.name, tc.a, tc.b, got, tc.want)
		}
	}
}

// TestSimilarity_CompositeWeighting pins the composite formula:
// Overall = 0.55*card + 0.20*archetype + 0.15*color + 0.10*bracket.
// A hand-built deck with known component scores should produce the
// expected composite.
func TestSimilarity_CompositeWeighting(t *testing.T) {
	// Construct decks where every component is exactly 0.5:
	//   - card jaccard: 2/4 = 0.5 (2 shared, 4 union)
	//   - archetype: 0.5 (primary↔secondary crossover)
	//   - color: 0.5 (1 of 2 union)
	//   - bracket: Δ=2 → affin=0.5
	a := makeFingerprint("A", "", "Combo", "Control", 4, []string{"U", "B"},
		[]string{"card1", "card2", "card3"})
	b := makeFingerprint("B", "", "Control", "Storm", 2, []string{"U", "R"},
		[]string{"card1", "card2", "card4"})
	// Cards: shared = {card1, card2} = 2; union = {card1, card2, card3, card4} = 4 → 0.5
	// Archetype: a.primary=Combo, b.primary=Control. a.secondary=Control matches b.primary → 0.5
	// Color: shared {U} = 1; union {U, B, R} = 3 → 1/3 ≈ 0.333
	// Bracket: Δ=2 → 0.5
	// Expected: 0.55*0.5 + 0.20*0.5 + 0.15*(1/3) + 0.10*0.5 = 0.275 + 0.10 + 0.05 + 0.05 = 0.475
	s := SimilarityScoreOf(a, b)
	want := 0.55*0.5 + 0.20*0.5 + 0.15*(1.0/3.0) + 0.10*0.5
	if !approxEqual(s.Overall, want) {
		t.Errorf("composite mismatch: got %.6f, want %.6f (components: card=%.4f arch=%.4f color=%.4f bracket=%.4f)",
			s.Overall, want, s.CardJaccard, s.ArchetypeMatch, s.ColorOverlap, s.BracketAffin)
	}
}

// TestSimilarity_NilSafe pins that nil inputs don't panic — the
// Decks screen will sometimes call this with partial fingerprints
// during corpus loading. A nil pair returns a zero-Overall result.
func TestSimilarity_NilSafe(t *testing.T) {
	if s := SimilarityScoreOf(nil, nil); s == nil || s.Overall != 0 {
		t.Errorf("SimilarityScoreOf(nil, nil) should return zero score, got %+v", s)
	}
	if s := SimilarityScoreOf(makeFingerprint("a", "", "Combo", "", 3, nil, nil), nil); s == nil || s.Overall != 0 {
		t.Errorf("SimilarityScoreOf(a, nil) should return zero score, got %+v", s)
	}
	if got := FindSimilarDecks(nil, []*DeckFingerprint{}, 5); got != nil {
		t.Errorf("FindSimilarDecks(nil, ...) should return nil, got %v", got)
	}
}

// TestFindSimilarDecks_SortsAndCaps pins the corpus-scan helper:
// results sorted descending by Overall, capped at limit, target deck
// excluded from its own results.
func TestFindSimilarDecks_SortsAndCaps(t *testing.T) {
	target := makeFingerprint("target", "", "Combo", "Control", 4, []string{"U", "B"},
		[]string{"c1", "c2", "c3", "c4"})
	corpus := []*DeckFingerprint{
		// near-duplicate (should rank #1)
		makeFingerprint("near-dup", "", "Combo", "Control", 4, []string{"U", "B"},
			[]string{"c1", "c2", "c3"}),
		// the target itself (should be excluded)
		target,
		// loosely related
		makeFingerprint("loose", "", "Storm", "", 4, []string{"U"},
			[]string{"c1"}),
		// fully disjoint
		makeFingerprint("disjoint", "", "Aggro", "", 1, []string{"R"},
			[]string{"x1", "x2"}),
	}
	got := FindSimilarDecks(target, corpus, 2)
	if len(got) != 2 {
		t.Fatalf("limit=2: got %d results, want 2", len(got))
	}
	if got[0].OtherDeck != "near-dup" {
		t.Errorf("rank #1 should be 'near-dup', got %q", got[0].OtherDeck)
	}
	// Target must NOT appear in its own match list.
	for _, s := range got {
		if s.OtherDeck == "target" {
			t.Errorf("target should be excluded from its own corpus match list, got %v", s)
		}
	}
	// Sorted descending.
	for i := 1; i < len(got); i++ {
		if got[i].Overall > got[i-1].Overall {
			t.Errorf("results not sorted desc: [%d]=%.4f after [%d]=%.4f",
				i, got[i].Overall, i-1, got[i-1].Overall)
		}
	}
}

// TestFindSimilarDecks_LimitZeroReturnsAll pins the convention:
// limit≤0 means "return everything," so callers can pull the full
// ranked list when they need it.
func TestFindSimilarDecks_LimitZeroReturnsAll(t *testing.T) {
	target := makeFingerprint("target", "", "Combo", "", 4, nil, []string{"c1"})
	corpus := []*DeckFingerprint{
		makeFingerprint("a", "", "Combo", "", 4, nil, []string{"c1"}),
		makeFingerprint("b", "", "Combo", "", 4, nil, []string{"c2"}),
		makeFingerprint("c", "", "Aggro", "", 2, nil, []string{"c3"}),
	}
	got := FindSimilarDecks(target, corpus, 0)
	if len(got) != 3 {
		t.Errorf("limit=0 should return all %d, got %d", len(corpus), len(got))
	}
	got = FindSimilarDecks(target, corpus, -1)
	if len(got) != 3 {
		t.Errorf("limit=-1 should also return all, got %d", len(got))
	}
}

// TestSimilarity_SharedSampleSorted pins that the SharedSample is
// alphabetized + capped at 8. Without that, sample contents would
// drift run-to-run due to map iteration order.
func TestSimilarity_SharedSampleSorted(t *testing.T) {
	cards := []string{"zebra", "apple", "mango", "banana", "kiwi", "fig", "grape", "honeydew", "lime"}
	a := makeFingerprint("a", "", "Combo", "", 4, nil, cards)
	b := makeFingerprint("b", "", "Combo", "", 4, nil, cards)
	s := SimilarityScoreOf(a, b)
	if len(s.SharedSample) != 8 {
		t.Errorf("sample should cap at 8, got %d", len(s.SharedSample))
	}
	for i := 1; i < len(s.SharedSample); i++ {
		if s.SharedSample[i] < s.SharedSample[i-1] {
			t.Errorf("sample not sorted: %q after %q", s.SharedSample[i], s.SharedSample[i-1])
		}
	}
	// First entry should be the alphabetical first (not a map-iter
	// artifact).
	if s.SharedSample[0] != "apple" {
		t.Errorf("first sample entry = %q, want 'apple' (alphabetical)", s.SharedSample[0])
	}
}

// TestSimilarity_VerdictLadder pins the qualitative-label thresholds
// so the Decks screen renders the same label for the same Overall
// score across releases.
func TestSimilarity_VerdictLadder(t *testing.T) {
	cases := []struct {
		overall float64
		want    string
	}{
		{0.95, "nearly identical"},
		{0.80, "nearly identical"},
		{0.65, "very similar"},
		{0.60, "very similar"},
		{0.45, "similar"},
		{0.40, "similar"},
		{0.25, "loosely related"},
		{0.20, "loosely related"},
		{0.10, "different"},
		{0.00, "different"},
	}
	for _, tc := range cases {
		got := similarityVerdict(tc.overall)
		if got != tc.want {
			t.Errorf("similarityVerdict(%.2f) = %q, want %q", tc.overall, got, tc.want)
		}
	}
}
