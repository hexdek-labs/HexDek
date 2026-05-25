package main

import (
	"strings"
	"testing"
)

// makeFP is a fixture helper for recommendation tests that also sets
// BracketLabel — the similarity_test.go makeFingerprint helper predates
// the BracketLabel field.
func makeFP(name, commander, primary, secondary string, bracket int, bracketLabel string, colors, cards []string) *DeckFingerprint {
	fp := makeFingerprint(name, commander, primary, secondary, bracket, colors, cards)
	fp.BracketLabel = bracketLabel
	return fp
}

// staticLookup returns a PerformanceLookup that resolves a fixed
// map. Used to inject deterministic performance data into tests.
func staticLookup(stats map[string]*DeckPerformance) PerformanceLookup {
	return func(deckName string) *DeckPerformance {
		return stats[deckName]
	}
}

// TestRecommendSimilarDecks_TopFiveWithBracketAndWinrate pins the
// headline use case: target a deck, get back the top 5 most-similar
// corpus entries each annotated with their bracket label + winrate
// from the optional stats lookup. The Decks screen consumes this
// list directly.
func TestRecommendSimilarDecks_TopFiveWithBracketAndWinrate(t *testing.T) {
	target := makeFP("Atraxa Combo", "Atraxa", "Combo", "Control", 4, "Optimized",
		[]string{"W", "U", "B", "G"},
		[]string{"sol ring", "rhystic study", "demonic tutor", "swords to plowshares", "thoracle", "consultation"})

	corpus := []*DeckFingerprint{
		// Near-dupe (5 of 6 shared, same arch, same bracket).
		makeFP("Atraxa Stax", "Atraxa", "Combo", "Stax", 4, "Optimized",
			[]string{"W", "U", "B", "G"},
			[]string{"sol ring", "rhystic study", "demonic tutor", "swords to plowshares", "thoracle"}),
		// Very similar (3 shared, archetype crossover via secondary, 1 bracket gap).
		makeFP("Tymna Kraum", "Tymna", "Control", "Combo", 5, "cEDH",
			[]string{"W", "U", "B"},
			[]string{"sol ring", "rhystic study", "swords to plowshares", "force of will"}),
		// Loosely related (1 card, no archetype match, 2 bracket gap).
		makeFP("Krenko Goblins", "Krenko", "Aggro / Go Wide", "Tribal", 2, "Core",
			[]string{"R"},
			[]string{"sol ring", "krenko, mob boss", "goblin warchief"}),
		// Different (no cards, no arch, 3 bracket gap).
		makeFP("Yarok Blink", "Yarok", "Blink", "", 1, "Casual",
			[]string{"U", "B", "G"},
			[]string{"deadeye navigator", "panharmonicon", "yarok"}),
		// One more so limit=5 actually truncates from 6.
		makeFP("Najeela Storm", "Najeela", "Combo", "Extra Combats", 5, "cEDH",
			[]string{"W", "U", "B", "R", "G"},
			[]string{"sol ring", "demonic tutor", "thoracle"}),
		// The target itself — must be excluded.
		target,
	}
	stats := staticLookup(map[string]*DeckPerformance{
		"Atraxa Stax":   {Games: 50, Wins: 18, WinRate: 36.0},
		"Tymna Kraum":   {Games: 200, Wins: 64, WinRate: 32.0},
		"Krenko Goblins": {Games: 10, Wins: 2, WinRate: 20.0},
		// Yarok Blink + Najeela Storm intentionally absent — recs
		// should still render those rows with Performance=nil.
	})

	recs := RecommendSimilarDecks(target, corpus, 5, stats)
	if len(recs) != 5 {
		t.Fatalf("limit=5 with 6 corpus entries (1 = self) should return 5, got %d", len(recs))
	}

	// Self-exclusion.
	for _, r := range recs {
		if r.DeckName == "Atraxa Combo" {
			t.Errorf("target should be excluded, got it in results: %+v", r)
		}
	}

	// #1 must be the near-dupe.
	if recs[0].DeckName != "Atraxa Stax" {
		t.Errorf("top recommendation should be near-dupe 'Atraxa Stax', got %q", recs[0].DeckName)
	}

	// Bracket + BracketLabel propagated from fingerprint.
	if recs[0].Bracket != 4 || recs[0].BracketLabel != "Optimized" {
		t.Errorf("Atraxa Stax bracket=%d label=%q, want 4/Optimized", recs[0].Bracket, recs[0].BracketLabel)
	}

	// Performance carried through from lookup.
	if recs[0].Performance == nil || recs[0].Performance.Games != 50 || recs[0].Performance.WinRate != 36.0 {
		t.Errorf("Atraxa Stax performance: got %+v, want Games=50 WinRate=36.0", recs[0].Performance)
	}

	// Decks missing from stats lookup should have nil Performance but
	// still appear in the result list.
	for _, r := range recs {
		if r.DeckName == "Yarok Blink" || r.DeckName == "Najeela Storm" {
			if r.Performance != nil {
				t.Errorf("%s should have nil Performance (absent from lookup), got %+v",
					r.DeckName, r.Performance)
			}
		}
	}

	// Sort order descending by Score.Overall.
	for i := 1; i < len(recs); i++ {
		if recs[i].Score.Overall > recs[i-1].Score.Overall {
			t.Errorf("results not sorted desc: [%d] %.4f after [%d] %.4f",
				i, recs[i].Score.Overall, i-1, recs[i-1].Score.Overall)
		}
	}
}

// TestRecommendSimilarDecks_NilStatsLookupStillWorks pins that the
// stats provider is optional — callers in static-analysis mode (CLI
// without a DB) pass nil and recommendations render with
// Performance=nil on every row. FormatDeckRecommendation handles the
// nil case by printing "no game data".
func TestRecommendSimilarDecks_NilStatsLookupStillWorks(t *testing.T) {
	target := makeFP("T", "X", "Combo", "", 4, "Optimized", nil,
		[]string{"a", "b", "c"})
	corpus := []*DeckFingerprint{
		makeFP("Other", "Y", "Combo", "", 4, "Optimized", nil, []string{"a", "b"}),
	}
	recs := RecommendSimilarDecks(target, corpus, 5, nil)
	if len(recs) != 1 {
		t.Fatalf("expected 1 rec, got %d", len(recs))
	}
	if recs[0].Performance != nil {
		t.Errorf("nil stats lookup should leave Performance nil, got %+v", recs[0].Performance)
	}
	rendered := FormatDeckRecommendation(recs[0])
	if !strings.Contains(rendered, "no game data") {
		t.Errorf("FormatDeckRecommendation should render 'no game data' when Performance is nil, got %q", rendered)
	}
}

// TestRecommendSimilarDecks_RankingIndependentOfPerformance pins the
// design call: similarity rank is NOT a function of winrate. A
// near-clone with a 5% winrate outranks a loosely-related deck with
// 80% — the user explicitly asked for "most similar," not "best."
func TestRecommendSimilarDecks_RankingIndependentOfPerformance(t *testing.T) {
	target := makeFP("T", "X", "Combo", "", 4, "", nil,
		[]string{"a", "b", "c", "d", "e"})
	corpus := []*DeckFingerprint{
		// Near-clone, terrible winrate.
		makeFP("Clone", "X", "Combo", "", 4, "", nil,
			[]string{"a", "b", "c", "d", "e"}),
		// Different deck, great winrate.
		makeFP("Different", "Y", "Aggro", "", 2, "", nil,
			[]string{"x", "y", "z"}),
	}
	stats := staticLookup(map[string]*DeckPerformance{
		"Clone":     {Games: 100, Wins: 5, WinRate: 5.0},
		"Different": {Games: 100, Wins: 80, WinRate: 80.0},
	})
	recs := RecommendSimilarDecks(target, corpus, 5, stats)
	if len(recs) != 2 {
		t.Fatalf("expected 2 recs, got %d", len(recs))
	}
	if recs[0].DeckName != "Clone" {
		t.Errorf("rank #1 should be the near-clone regardless of winrate, got %q", recs[0].DeckName)
	}
}

// TestRecommendSimilarDecks_EmptyCorpusReturnsNil pins the
// short-circuit: nil/empty corpus returns nil, not a malformed empty
// slice. Callers can range over a nil result safely.
func TestRecommendSimilarDecks_EmptyCorpusReturnsNil(t *testing.T) {
	target := makeFP("T", "X", "Combo", "", 4, "", nil, []string{"a"})
	if got := RecommendSimilarDecks(target, nil, 5, nil); got != nil {
		t.Errorf("empty corpus should return nil, got %v", got)
	}
	if got := RecommendSimilarDecks(target, []*DeckFingerprint{}, 5, nil); got != nil {
		t.Errorf("empty []corpus should return nil, got %v", got)
	}
	if got := RecommendSimilarDecks(nil, []*DeckFingerprint{}, 5, nil); got != nil {
		t.Errorf("nil target should return nil, got %v", got)
	}
}

// TestComposeRecommendationReason pins the human-readable summary
// composition: names the dominant component drivers (shared cards,
// archetype match, bracket gap) so the user knows WHY this deck
// surfaced in the recommendations.
func TestComposeRecommendationReason(t *testing.T) {
	cases := []struct {
		name        string
		score       *SimilarityScore
		mustContain []string
	}{
		{
			"near-dupe summary names shared cards + same arch + same bracket",
			&SimilarityScore{
				Overall:        0.85,
				SharedCount:    18,
				ArchetypeMatch: 1.0,
				BracketDelta:   0,
				Verdict:        "nearly identical",
			},
			[]string{"nearly identical", "85%", "18 shared cards", "same archetype", "same bracket"},
		},
		{
			"crossover summary uses 'archetype crossover' label",
			&SimilarityScore{
				Overall:        0.65,
				SharedCount:    8,
				ArchetypeMatch: 0.5,
				BracketDelta:   1,
				Verdict:        "very similar",
			},
			[]string{"very similar", "65%", "8 shared cards", "archetype crossover", "1-bracket gap"},
		},
		{
			"disjoint summary still produces a non-empty line",
			&SimilarityScore{
				Overall:        0.10,
				SharedCount:    0,
				ArchetypeMatch: 0.0,
				BracketDelta:   3,
				Verdict:        "different",
			},
			[]string{"different", "10%", "3-bracket gap"},
		},
	}
	for _, tc := range cases {
		got := composeRecommendationReason(tc.score)
		for _, want := range tc.mustContain {
			if !strings.Contains(got, want) {
				t.Errorf("%s: reason %q missing %q", tc.name, got, want)
			}
		}
	}
}

// TestFormatDeckRecommendation_RendersBracketAndPerf pins the
// text-mode render: includes deck name, bracket (with label when
// available), winrate from Performance, and the composed reason.
func TestFormatDeckRecommendation_RendersBracketAndPerf(t *testing.T) {
	rec := &DeckRecommendation{
		DeckName:     "Atraxa Combo",
		Commander:    "Atraxa",
		Bracket:      4,
		BracketLabel: "Optimized",
		Score: &SimilarityScore{
			Overall: 0.73, SharedCount: 12, ArchetypeMatch: 1.0,
			BracketDelta: 0, Verdict: "very similar",
		},
		Performance: &DeckPerformance{Games: 200, Wins: 80, WinRate: 40.0},
		Reason:      "very similar (73%): 12 shared cards, same archetype, same bracket",
	}
	out := FormatDeckRecommendation(rec)
	for _, want := range []string{"Atraxa Combo", "B4 Optimized", "200 games", "40.0%", "very similar"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatDeckRecommendation output missing %q. Got:\n%s", want, out)
		}
	}
}

// TestFormatDeckRecommendation_ZeroGamesAndMissingLabel pins the
// fallback rendering for cases with no game data and no bracket
// label — the row still renders with sensible defaults rather than
// printing empty brackets or "0 games, 0.0% winrate" lies.
func TestFormatDeckRecommendation_ZeroGamesAndMissingLabel(t *testing.T) {
	rec := &DeckRecommendation{
		DeckName: "Untested Deck",
		Bracket:  3,
		Score:    &SimilarityScore{Overall: 0.45, Verdict: "similar"},
		// Performance nil — no DB row.
		Reason: "similar (45%)",
	}
	out := FormatDeckRecommendation(rec)
	if !strings.Contains(out, "B3") {
		t.Errorf("output should include 'B3' even without label, got %q", out)
	}
	if strings.Contains(out, "Optimized") {
		t.Errorf("output should NOT include a bracket label when none was supplied, got %q", out)
	}
	if !strings.Contains(out, "no game data") {
		t.Errorf("nil Performance should render 'no game data', got %q", out)
	}

	// Empty-games row (Performance non-nil but Games=0) renders "0 games"
	// instead of a misleading winrate.
	rec.Performance = &DeckPerformance{Games: 0, Wins: 0, WinRate: 0}
	out = FormatDeckRecommendation(rec)
	if !strings.Contains(out, "0 games") {
		t.Errorf("Games=0 should render '0 games', got %q", out)
	}
}

// TestRecommendSimilarDecks_LimitZeroReturnsAll pins the limit-zero
// convention inherited from FindSimilarDecks — a Decks-screen call
// that wants to render "all 47 similar decks" can pass 0.
func TestRecommendSimilarDecks_LimitZeroReturnsAll(t *testing.T) {
	target := makeFP("T", "X", "Combo", "", 4, "", nil, []string{"a"})
	corpus := []*DeckFingerprint{
		makeFP("a1", "X", "Combo", "", 4, "", nil, []string{"a"}),
		makeFP("a2", "X", "Combo", "", 4, "", nil, []string{"b"}),
		makeFP("a3", "X", "Combo", "", 4, "", nil, []string{"c"}),
	}
	recs := RecommendSimilarDecks(target, corpus, 0, nil)
	if len(recs) != 3 {
		t.Errorf("limit=0 should return all %d, got %d", len(corpus), len(recs))
	}
}

// TestBuildDeckFingerprint_CarriesBracketLabel pins the additive
// field on DeckFingerprint — BracketLabel must be populated from
// dp.BracketLabel so RecommendSimilarDecks can render it without a
// second DB call.
func TestBuildDeckFingerprint_CarriesBracketLabel(t *testing.T) {
	dp := &DeckProfile{
		DeckName: "Test", Bracket: 4, BracketLabel: "Optimized",
	}
	fp := BuildDeckFingerprint(dp, nil)
	if fp.BracketLabel != "Optimized" {
		t.Errorf("BracketLabel should propagate from DeckProfile, got %q", fp.BracketLabel)
	}
}
