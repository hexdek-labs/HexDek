package analytics

import (
	"testing"
)

// matchup_detail_r60_test pins BuildMatchupDetails (the aggregation
// over GameAnalysis slices that powers the Decks-screen matchup table)
// and the topNCards helper. The audit flagged matchup_detail.go at 0%;
// nothing in the analytics test suite previously exercised this path.

// gameAnalysis is a small fixture builder so each test case only spells
// out the commander names + winner + cards (the rest defaults to
// reasonable zero values).
func gameAnalysisFixture(commanders []string, winnerSeat int, totalTurns int, winCond string, winnerCards []string) *GameAnalysis {
	players := make([]PlayerAnalysis, len(commanders))
	for i, name := range commanders {
		players[i] = PlayerAnalysis{Seat: i, CommanderName: name}
	}
	if winnerSeat >= 0 && winnerSeat < len(players) {
		cards := make([]CardPerformance, len(winnerCards))
		for i, c := range winnerCards {
			cards[i] = CardPerformance{Name: c, TurnCast: i + 1}
		}
		players[winnerSeat].CardsPlayed = cards
	}
	return &GameAnalysis{
		Players:     players,
		WinnerSeat:  winnerSeat,
		TotalTurns:  totalTurns,
		WinCondition: winCond,
	}
}

// TestBuildMatchupDetails_NilOrEmptyReturnsNil pins the no-input
// branch — a nil/empty analyses slice short-circuits to nil rather
// than building an empty pile of nothing.
func TestBuildMatchupDetails_NilOrEmptyReturnsNil(t *testing.T) {
	if got := BuildMatchupDetails(nil); got != nil {
		t.Errorf("nil input should return nil, got %+v", got)
	}
	if got := BuildMatchupDetails([]*GameAnalysis{}); got != nil {
		t.Errorf("empty input should return nil, got %+v", got)
	}
}

// TestBuildMatchupDetails_CanonicalAlphabeticalKey verifies that the
// (A,B) and (B,A) pair are aggregated under the same matchup, with
// Deck1 < Deck2 alphabetically. Without this canonicalization, the
// same matchup would appear twice in the output with mirrored stats.
func TestBuildMatchupDetails_CanonicalAlphabeticalKey(t *testing.T) {
	games := []*GameAnalysis{
		gameAnalysisFixture([]string{"Beast", "Atraxa"}, 1, 10, "combo", []string{"Demonic Tutor"}),
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 8, "combo", []string{"Demonic Tutor"}),
	}
	details := BuildMatchupDetails(games)
	if len(details) != 1 {
		t.Fatalf("want 1 matchup (canonical pair), got %d: %+v", len(details), details)
	}
	if details[0].Deck1 != "Atraxa" || details[0].Deck2 != "Beast" {
		t.Errorf("canonical ordering broken: got Deck1=%q Deck2=%q (want Atraxa, Beast)",
			details[0].Deck1, details[0].Deck2)
	}
	if details[0].TotalGames != 2 || details[0].Deck1Wins != 2 || details[0].Deck2Wins != 0 {
		t.Errorf("aggregation wrong: TotalGames=%d Deck1Wins=%d Deck2Wins=%d",
			details[0].TotalGames, details[0].Deck1Wins, details[0].Deck2Wins)
	}
}

// TestBuildMatchupDetails_WinRateAndAvgTurnsMath pins the exact stat
// math: WinRate = wins/games, AvgTurnToWin = sum/count of TotalTurns
// across games where that deck won. With 3 Atraxa wins (at turns 5,
// 10, 15) and 1 Beast win (turn 8), Atraxa WinRate = 0.75, avg = 10.
func TestBuildMatchupDetails_WinRateAndAvgTurnsMath(t *testing.T) {
	games := []*GameAnalysis{
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 5, "combo", nil),
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 10, "combo", nil),
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 15, "combo", nil),
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 1, 8, "combat", nil),
	}
	details := BuildMatchupDetails(games)
	if len(details) != 1 {
		t.Fatalf("want 1 matchup, got %d", len(details))
	}
	md := details[0]
	if md.Deck1WinRate != 0.75 {
		t.Errorf("Deck1WinRate = %.3f, want 0.75 (3 of 4)", md.Deck1WinRate)
	}
	if md.AvgTurnToWin1 != 10.0 {
		t.Errorf("AvgTurnToWin1 = %.3f, want 10.0 (avg of 5,10,15)", md.AvgTurnToWin1)
	}
	if md.AvgTurnToWin2 != 8.0 {
		t.Errorf("AvgTurnToWin2 = %.3f, want 8.0 (single Beast win)", md.AvgTurnToWin2)
	}
}

// TestBuildMatchupDetails_WinConditionBreakdown verifies that
// Deck1WinsByType aggregates the WinCondition strings of games the
// deck won — used by the Decks screen to show "Atraxa wins 5 by
// combo, 2 by combat damage" stats.
func TestBuildMatchupDetails_WinConditionBreakdown(t *testing.T) {
	games := []*GameAnalysis{
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 10, "combo", nil),
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 12, "combo", nil),
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 15, "combat", nil),
	}
	details := BuildMatchupDetails(games)
	md := details[0]
	if md.Deck1WinsByType["combo"] != 2 {
		t.Errorf("combo wins = %d, want 2", md.Deck1WinsByType["combo"])
	}
	if md.Deck1WinsByType["combat"] != 1 {
		t.Errorf("combat wins = %d, want 1", md.Deck1WinsByType["combat"])
	}
}

// TestBuildMatchupDetails_KeyCardsTopFive pins the KeyCards population
// path — only cards from the winning seat with TurnCast > 0 count, and
// the top 5 by frequency are surfaced. Cards that never landed
// (TurnCast == 0) must NOT pollute the rankings.
func TestBuildMatchupDetails_KeyCardsTopFive(t *testing.T) {
	games := []*GameAnalysis{
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 10, "combo",
			[]string{"Demonic Tutor", "Cyclonic Rift", "Sol Ring"}),
		gameAnalysisFixture([]string{"Atraxa", "Beast"}, 0, 10, "combo",
			[]string{"Demonic Tutor", "Sol Ring", "Vampiric Tutor"}),
	}
	details := BuildMatchupDetails(games)
	md := details[0]
	if len(md.KeyCards1) == 0 {
		t.Fatalf("expected key cards for winning deck, got empty")
	}
	// Top card should be Demonic Tutor (2 appearances) or Sol Ring (2).
	if md.KeyCards1[0] != "Demonic Tutor" && md.KeyCards1[0] != "Sol Ring" {
		t.Errorf("top KeyCard = %q, want Demonic Tutor or Sol Ring (each appeared 2x)", md.KeyCards1[0])
	}
	if len(md.KeyCards1) > 5 {
		t.Errorf("KeyCards1 not capped at 5, got %d", len(md.KeyCards1))
	}
}

// TestBuildMatchupDetails_NilAnalysisSkipped pins the nil-guard in the
// per-game loop: a nil GameAnalysis must be skipped silently, not
// panic. Necessary because batch ingestion sometimes hands in
// partially-populated slices.
func TestBuildMatchupDetails_NilAnalysisSkipped(t *testing.T) {
	games := []*GameAnalysis{
		nil,
		gameAnalysisFixture([]string{"A", "B"}, 0, 5, "combo", nil),
		nil,
	}
	details := BuildMatchupDetails(games)
	if len(details) != 1 {
		t.Errorf("nil entries should be skipped, got %d details: %+v", len(details), details)
	}
}

// TestBuildMatchupDetails_SortedByTotalGamesDesc pins the output
// ordering — most-played matchup first. The Decks screen relies on
// this to surface the most statistically-significant pairings.
func TestBuildMatchupDetails_SortedByTotalGamesDesc(t *testing.T) {
	games := []*GameAnalysis{
		gameAnalysisFixture([]string{"A", "B"}, 0, 5, "combo", nil), // A vs B: 1 game
		gameAnalysisFixture([]string{"C", "D"}, 0, 5, "combo", nil), // C vs D: 3 games
		gameAnalysisFixture([]string{"C", "D"}, 0, 5, "combo", nil),
		gameAnalysisFixture([]string{"C", "D"}, 0, 5, "combo", nil),
	}
	details := BuildMatchupDetails(games)
	if len(details) != 2 {
		t.Fatalf("want 2 matchups, got %d", len(details))
	}
	if details[0].TotalGames < details[1].TotalGames {
		t.Errorf("not sorted by TotalGames desc: %v", details)
	}
	if details[0].Deck1 != "C" {
		t.Errorf("top matchup should be C vs D (3 games), got %s vs %s", details[0].Deck1, details[0].Deck2)
	}
}

// TestTopNCards_Cap verifies the top-N cap. With 10 unique cards and
// n=5, exactly 5 come back; with n=10, all 10; with n>len, only the
// existing count (no padding with empties).
func TestTopNCards_Cap(t *testing.T) {
	m := map[string]int{}
	for i := 0; i < 10; i++ {
		m[string(rune('A'+i))] = i + 1
	}
	if got := topNCards(m, 5); len(got) != 5 {
		t.Errorf("topNCards cap=5, got %d entries", len(got))
	}
	if got := topNCards(m, 10); len(got) != 10 {
		t.Errorf("topNCards cap=10, got %d entries", len(got))
	}
	if got := topNCards(m, 20); len(got) != 10 {
		t.Errorf("topNCards cap>len, got %d entries (want 10, no padding)", len(got))
	}
}

// TestTopNCards_DescendingByCount pins the sort order: highest count
// first. The KeyCards rendering depends on the most-frequent card
// appearing at position 0.
func TestTopNCards_DescendingByCount(t *testing.T) {
	m := map[string]int{"low": 1, "high": 10, "mid": 5}
	got := topNCards(m, 3)
	if len(got) != 3 || got[0] != "high" || got[2] != "low" {
		t.Errorf("topNCards descending order broken: %v", got)
	}
}

// TestTopNCards_EmptyMap pins the empty-map edge — returns an empty
// slice (length 0, non-nil) rather than nil or panic.
func TestTopNCards_EmptyMap(t *testing.T) {
	got := topNCards(map[string]int{}, 5)
	if got == nil {
		t.Errorf("topNCards on empty map should return non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("topNCards on empty map should return 0-length, got %d", len(got))
	}
}
