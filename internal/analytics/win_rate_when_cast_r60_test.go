package analytics

import (
	"strings"
	"testing"
)

// win_rate_when_cast_r60_test.go — pins the per-card
// WinRateWhenCast aggregation surfaced as part of the Heimdall
// analytics enhancement (PR opened from
// `dev/heimdall-analytics-enhance-r60`).
//
// The metric is distinct from WinContribution (fraction of WINS that
// involved this card) and from KillShotRate (fraction of wins where
// this card delivered the final blow):
//
//   WinRateWhenCast = gamesCastAndWon / gamesCast
//
// where `gamesCast` is the count of distinct games where the card was
// successfully cast at least once (TurnCast > 0; lands and tokens
// excluded) and `gamesCastAndWon` is the subset where the controller
// also won. Lossy interpretation: "when I drew and cast this card, did
// I win this game?"

// TestWinRateWhenCast_PerfectRate pins the simplest positive case:
// the same card cast in 3 distinct games, all three won → 100%.
func TestWinRateWhenCast_PerfectRate(t *testing.T) {
	mk := func(won bool) *GameAnalysis {
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: "Sol Ring", TurnCast: 1},
				},
			}},
		}
	}
	rankings := RankCards([]*GameAnalysis{mk(true), mk(true), mk(true)})
	cr := findRanking(rankings, "Sol Ring")
	if cr == nil {
		t.Fatal("Sol Ring missing")
	}
	if cr.GamesCast != 3 {
		t.Errorf("GamesCast = %d, want 3", cr.GamesCast)
	}
	if cr.GamesCastAndWon != 3 {
		t.Errorf("GamesCastAndWon = %d, want 3", cr.GamesCastAndWon)
	}
	if !rankCardsApprox(cr.WinRateWhenCast, 1.0) {
		t.Errorf("WinRateWhenCast = %f, want 1.0", cr.WinRateWhenCast)
	}
}

// TestWinRateWhenCast_PartialRate covers the mixed-win case: 4 games,
// cast in all 4, winner in 1 → 25%.
func TestWinRateWhenCast_PartialRate(t *testing.T) {
	mk := func(won bool) *GameAnalysis {
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{
					{Name: "Wheel of Fortune", TurnCast: 2},
				},
			}},
		}
	}
	rankings := RankCards([]*GameAnalysis{mk(true), mk(false), mk(false), mk(false)})
	cr := findRanking(rankings, "Wheel of Fortune")
	if cr == nil {
		t.Fatal("Wheel missing")
	}
	if cr.GamesCast != 4 {
		t.Errorf("GamesCast = %d, want 4", cr.GamesCast)
	}
	if cr.GamesCastAndWon != 1 {
		t.Errorf("GamesCastAndWon = %d, want 1", cr.GamesCastAndWon)
	}
	if !rankCardsApprox(cr.WinRateWhenCast, 0.25) {
		t.Errorf("WinRateWhenCast = %f, want 0.25", cr.WinRateWhenCast)
	}
}

// TestWinRateWhenCast_NeverCast_NoSignal pins the divide-by-zero
// guard: a card that was countered or stayed in hand in every game
// has zero cast samples → WinRateWhenCast = 0 (not NaN, not
// extrapolated from a single missed cast).
func TestWinRateWhenCast_NeverCast_NoSignal(t *testing.T) {
	games := []*GameAnalysis{
		{Players: []PlayerAnalysis{{
			Won: true,
			CardsPlayed: []CardPerformance{
				{Name: "Stuck in Hand", TurnCast: 0}, // dead in hand
			},
		}}},
		{Players: []PlayerAnalysis{{
			Won: false,
			CardsPlayed: []CardPerformance{
				{Name: "Stuck in Hand", TurnCast: 0, WasCountered: true},
			},
		}}},
	}
	rankings := RankCards(games)
	cr := findRanking(rankings, "Stuck in Hand")
	if cr == nil {
		t.Fatal("Stuck in Hand missing")
	}
	if cr.GamesCast != 0 {
		t.Errorf("GamesCast = %d, want 0 (countered + dead-in-hand should not bump)", cr.GamesCast)
	}
	if cr.WinRateWhenCast != 0 {
		t.Errorf("WinRateWhenCast = %f, want 0 (no signal when GamesCast == 0)", cr.WinRateWhenCast)
	}
}

// TestWinRateWhenCast_DedupesPerGame_MultiCastSameGame guards against
// a card cast multiple times in one game inflating gamesCast. We
// dedupe by (gameIdx, cardName) so a recast Eldritch Evolution that
// lands a creature twice still counts as one game.
//
// Two games: in game 0 the card is in CardsPlayed once but won; in
// game 1 the card appears in TWO seats (e.g. mirror) with the same
// gameIdx. The expected gamesCast is 2 (one per game), NOT 3 (per
// CardPlayed entry).
func TestWinRateWhenCast_DedupesPerGame_MultiCastSameGame(t *testing.T) {
	games := []*GameAnalysis{
		{
			Players: []PlayerAnalysis{{
				Won: true,
				CardsPlayed: []CardPerformance{
					{Name: "Demonic Tutor", TurnCast: 3},
				},
			}},
		},
		{
			Players: []PlayerAnalysis{
				{
					Won: true,
					CardsPlayed: []CardPerformance{
						{Name: "Demonic Tutor", TurnCast: 2},
					},
				},
				{
					Won: false,
					CardsPlayed: []CardPerformance{
						{Name: "Demonic Tutor", TurnCast: 4},
					},
				},
			},
		},
	}
	rankings := RankCards(games)
	cr := findRanking(rankings, "Demonic Tutor")
	if cr == nil {
		t.Fatal("Demonic Tutor missing")
	}
	if cr.GamesCast != 2 {
		t.Errorf("GamesCast = %d, want 2 (per-game dedupe across multi-seat mirrors)", cr.GamesCast)
	}
	// game 0 won, game 1 had at least one winning seat that cast the
	// card → both games count toward GamesCastAndWon.
	if cr.GamesCastAndWon != 2 {
		t.Errorf("GamesCastAndWon = %d, want 2", cr.GamesCastAndWon)
	}
	if !rankCardsApprox(cr.WinRateWhenCast, 1.0) {
		t.Errorf("WinRateWhenCast = %f, want 1.0", cr.WinRateWhenCast)
	}
}

// TestWinRateWhenCast_ExcludesLandsAndTokens defends the lands/tokens
// filter. Lands are "played" not "cast" and tokens are "created" not
// "drawn", so neither should contribute to the cast denominator. A
// land that "appears" in every won game shouldn't show WinRateWhenCast
// = 100% — that's the existing kill-shot/MVP path. WinRateWhenCast is
// specifically about CAST cards.
func TestWinRateWhenCast_ExcludesLandsAndTokens(t *testing.T) {
	games := []*GameAnalysis{
		{
			Players: []PlayerAnalysis{{
				Won: true,
				CardsPlayed: []CardPerformance{
					{Name: "Forest", TurnCast: 1, IsLand: true},
					{Name: "Squirrel Token", TurnCast: 2, IsToken: true},
				},
			}},
		},
	}
	rankings := RankCards(games)
	for _, name := range []string{"Forest", "Squirrel Token"} {
		cr := findRanking(rankings, name)
		if cr == nil {
			t.Fatalf("%s missing — should still appear in rankings as zero-cast", name)
		}
		if cr.GamesCast != 0 {
			t.Errorf("%s GamesCast = %d, want 0 (lands/tokens excluded from cast sample)",
				name, cr.GamesCast)
		}
		if cr.WinRateWhenCast != 0 {
			t.Errorf("%s WinRateWhenCast = %f, want 0", name, cr.WinRateWhenCast)
		}
	}
}

// TestWriteWinRateWhenCast_RenderShape pins the markdown surface: top
// rows ordered by WinRateWhenCast desc, sample size filter, header +
// per-row format. Stable against accidental column reordering in
// future report changes.
func TestWriteWinRateWhenCast_RenderShape(t *testing.T) {
	games := make([]*GameAnalysis, 12)
	for i := 0; i < 12; i++ {
		games[i] = &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: true,
				CardsPlayed: []CardPerformance{
					{Name: "Hot Card", TurnCast: 3},
				},
			}},
		}
	}
	rankings := RankCards(games)
	r := &AnalyticsReport{
		Analyses:     games,
		CardRankings: rankings,
		TotalGames:   12,
	}
	var b strings.Builder
	r.writeWinRateWhenCast(&b, 5)
	out := b.String()

	// Header presence.
	if !strings.Contains(out, "## Win Rate When Cast") {
		t.Errorf("missing section header: %q", out)
	}
	for _, want := range []string{
		"| Rank |", "Win % When Cast", "Games Cast", "Games Won When Cast", "Avg Turn Cast",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing column %q in:\n%s", want, out)
		}
	}
	// Hot Card on row 1.
	if !strings.Contains(out, "| 1 | Hot Card |") {
		t.Errorf("Hot Card not on row 1; got:\n%s", out)
	}
	if !strings.Contains(out, "100%") {
		t.Errorf("Hot Card should render at 100%%; got:\n%s", out)
	}
}

// TestWriteWinRateWhenCast_MinSampleFilter pins the minimum-sample
// gate. With only 2 games played, the filter should report no
// eligible cards rather than ranking a 1-game cast at 100%.
func TestWriteWinRateWhenCast_MinSampleFilter(t *testing.T) {
	games := []*GameAnalysis{
		{Players: []PlayerAnalysis{{
			Won: true,
			CardsPlayed: []CardPerformance{
				{Name: "Tiny Sample", TurnCast: 1},
			},
		}}},
	}
	rankings := RankCards(games)
	r := &AnalyticsReport{
		Analyses:     games,
		CardRankings: rankings,
		TotalGames:   1,
	}
	var b strings.Builder
	r.writeWinRateWhenCast(&b, 10)
	out := b.String()
	if !strings.Contains(out, "minimum sample") && !strings.Contains(out, "Minimum sample") {
		t.Errorf("filter should mention min sample; got:\n%s", out)
	}
	if strings.Contains(out, "Tiny Sample") {
		t.Errorf("Tiny Sample below threshold should NOT render; got:\n%s", out)
	}
}
