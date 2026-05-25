package heimdall

import (
	"math"
	"testing"
)

// tournament_stats_r60_test.go — pins ComputeTournamentStats as a
// pure aggregator over (run meta, per-game rows).

func newRunMeta() TournamentRunMeta {
	return TournamentRunMeta{
		RunID:      42,
		DeckKey:    "alice/iroh",
		Commander:  "Iroh",
		StartedAt:  1000,
		FinishedAt: 9000,
		Games:      6,
		Wins:       3,
		Losses:     3,
		WinRate:    0.5,
		AvgTurns:   8.0,
	}
}

// -----------------------------------------------------------------------------
// Empty / shape.
// -----------------------------------------------------------------------------

func TestComputeTournamentStats_EmptyGames(t *testing.T) {
	got := ComputeTournamentStats(newRunMeta(), nil)
	if got.TournamentID != 42 {
		t.Errorf("TournamentID = %d, want 42", got.TournamentID)
	}
	if got.LongestGame != nil || got.FastestGame != nil {
		t.Errorf("extremes should be nil with no games, got long=%v fast=%v", got.LongestGame, got.FastestGame)
	}
	if len(got.DeckDistribution) != 0 {
		t.Errorf("DeckDistribution should be empty, got %d", len(got.DeckDistribution))
	}
	if got.WinrateVariance.SampleSize != 0 {
		t.Errorf("WinrateVariance.SampleSize = %d, want 0", got.WinrateVariance.SampleSize)
	}
	if got.WinRate != 0.5 {
		t.Errorf("WinRate should echo run meta even with zero games processed, got %f", got.WinRate)
	}
}

// -----------------------------------------------------------------------------
// Deck distribution.
// -----------------------------------------------------------------------------

func TestComputeTournamentStats_DeckDistribution(t *testing.T) {
	games := []TournamentGameInput{
		{GameID: 1, Turns: 8, Won: true, Opponents: []string{"Atraxa", "Edgar", "Krenko"}},
		{GameID: 2, Turns: 6, Won: false, Opponents: []string{"Atraxa", "Yuriko", "Korvold"}},
		{GameID: 3, Turns: 12, Won: true, Opponents: []string{"Atraxa", "Tergrid", "Najeela"}},
	}
	got := ComputeTournamentStats(newRunMeta(), games)
	// Atraxa appears 3x; others 1x each. 9 total opp entries.
	if len(got.DeckDistribution) != 7 {
		t.Fatalf("expected 7 unique opponents, got %d (%+v)", len(got.DeckDistribution), got.DeckDistribution)
	}
	if got.DeckDistribution[0].Commander != "Atraxa" || got.DeckDistribution[0].Games != 3 {
		t.Errorf("first slot = %+v, want Atraxa/3", got.DeckDistribution[0])
	}
	wantShare := 3.0 / 9.0
	if math.Abs(got.DeckDistribution[0].Share-wantShare) > 1e-9 {
		t.Errorf("Atraxa share = %f, want %f", got.DeckDistribution[0].Share, wantShare)
	}
}

// -----------------------------------------------------------------------------
// Longest / fastest game.
// -----------------------------------------------------------------------------

func TestComputeTournamentStats_LongestAndFastestGame(t *testing.T) {
	games := []TournamentGameInput{
		{GameID: 1, Turns: 8, Won: true, FinishedAt: 1100, Opponents: []string{"X", "Y", "Z"}},
		{GameID: 2, Turns: 22, Won: false, FinishedAt: 1200, Opponents: []string{"X", "Y", "Z"}},
		{GameID: 3, Turns: 4, Won: true, FinishedAt: 1300, Opponents: []string{"X", "Y", "Z"}},
		{GameID: 4, Turns: 11, Won: false, FinishedAt: 1400, Opponents: []string{"X", "Y", "Z"}},
	}
	got := ComputeTournamentStats(newRunMeta(), games)
	if got.LongestGame == nil || got.LongestGame.GameID != 2 || got.LongestGame.Turns != 22 {
		t.Errorf("LongestGame = %+v, want GameID=2 Turns=22", got.LongestGame)
	}
	if got.FastestGame == nil || got.FastestGame.GameID != 3 || got.FastestGame.Turns != 4 {
		t.Errorf("FastestGame = %+v, want GameID=3 Turns=4", got.FastestGame)
	}
	if !got.FastestGame.Won {
		t.Errorf("FastestGame.Won = false, want true")
	}
	if got.LongestGame.Won {
		t.Errorf("LongestGame.Won = true, want false")
	}
}

func TestComputeTournamentStats_SingleGameIsBothLongestAndFastest(t *testing.T) {
	games := []TournamentGameInput{
		{GameID: 7, Turns: 9, Won: true, Opponents: []string{"X"}},
	}
	got := ComputeTournamentStats(newRunMeta(), games)
	if got.LongestGame == nil || got.FastestGame == nil {
		t.Fatalf("extremes should be populated, got long=%v fast=%v", got.LongestGame, got.FastestGame)
	}
	if got.LongestGame.GameID != 7 || got.FastestGame.GameID != 7 {
		t.Errorf("single-game tournament should have same game as both extremes; got long=%d fast=%d",
			got.LongestGame.GameID, got.FastestGame.GameID)
	}
}

// -----------------------------------------------------------------------------
// Winrate variance.
// -----------------------------------------------------------------------------

func TestComputeTournamentStats_WinrateVarianceSpread(t *testing.T) {
	// Deck is 100% vs Atraxa (2-0), 0% vs Krenko (0-2). 4 games total
	// in deck distribution opp entries, 2 sample opponents.
	// Per-opponent winrates: [1.0, 0.0]. Mean = 0.5, variance = 0.25,
	// stddev = 0.5.
	games := []TournamentGameInput{
		{GameID: 1, Turns: 8, Won: true, Opponents: []string{"Atraxa"}},
		{GameID: 2, Turns: 8, Won: true, Opponents: []string{"Atraxa"}},
		{GameID: 3, Turns: 8, Won: false, Opponents: []string{"Krenko"}},
		{GameID: 4, Turns: 8, Won: false, Opponents: []string{"Krenko"}},
	}
	got := ComputeTournamentStats(newRunMeta(), games)
	v := got.WinrateVariance
	if v.SampleSize != 2 {
		t.Errorf("SampleSize = %d, want 2", v.SampleSize)
	}
	if math.Abs(v.Variance-0.25) > 1e-9 {
		t.Errorf("Variance = %f, want 0.25", v.Variance)
	}
	if math.Abs(v.Stddev-0.5) > 1e-9 {
		t.Errorf("Stddev = %f, want 0.5", v.Stddev)
	}
	if v.MinWinRate != 0 || v.MaxWinRate != 1 {
		t.Errorf("Min/Max = %f/%f, want 0/1", v.MinWinRate, v.MaxWinRate)
	}
}

func TestComputeTournamentStats_WinrateVarianceFiltersSingletons(t *testing.T) {
	// FringeCommander appears once (deck won) — should be EXCLUDED
	// from the variance sample so a single-game 100% doesn't dominate.
	games := []TournamentGameInput{
		{GameID: 1, Won: true, Opponents: []string{"Atraxa"}},
		{GameID: 2, Won: false, Opponents: []string{"Atraxa"}},
		{GameID: 3, Won: true, Opponents: []string{"FringeCommander"}},
	}
	got := ComputeTournamentStats(newRunMeta(), games)
	if got.WinrateVariance.SampleSize != 1 {
		t.Errorf("SampleSize = %d, want 1 (Atraxa qualifies, fringe excluded)", got.WinrateVariance.SampleSize)
	}
}

func TestComputeTournamentStats_WinrateVarianceFlatProfile(t *testing.T) {
	// Same 50% winrate against everyone → variance = 0.
	games := []TournamentGameInput{
		{GameID: 1, Won: true, Opponents: []string{"A"}},
		{GameID: 2, Won: false, Opponents: []string{"A"}},
		{GameID: 3, Won: true, Opponents: []string{"B"}},
		{GameID: 4, Won: false, Opponents: []string{"B"}},
	}
	got := ComputeTournamentStats(newRunMeta(), games)
	if got.WinrateVariance.Variance != 0 {
		t.Errorf("Variance = %f, want 0 for flat 50%% profile", got.WinrateVariance.Variance)
	}
}

// -----------------------------------------------------------------------------
// Run meta echo: scalar columns come from gauntlet_runs verbatim,
// not recomputed from games (avoid drift on partial persistence).
// -----------------------------------------------------------------------------

func TestComputeTournamentStats_ScalarFieldsEchoRunMeta(t *testing.T) {
	run := newRunMeta()
	// Hand games that disagree with run meta to confirm we echo
	// the meta rather than recompute.
	games := []TournamentGameInput{
		{GameID: 1, Won: true, Opponents: []string{"X"}},
	}
	got := ComputeTournamentStats(run, games)
	if got.Wins != run.Wins || got.Losses != run.Losses || got.Games != run.Games {
		t.Errorf("scalars should mirror run meta, got W/L/G = %d/%d/%d want %d/%d/%d",
			got.Wins, got.Losses, got.Games, run.Wins, run.Losses, run.Games)
	}
	if got.WinRate != run.WinRate || got.AvgTurns != run.AvgTurns {
		t.Errorf("WinRate / AvgTurns should mirror run meta, got %f/%f", got.WinRate, got.AvgTurns)
	}
}

// -----------------------------------------------------------------------------
// Draws are counted separately.
// -----------------------------------------------------------------------------

func TestComputeTournamentStats_DrawsCounted(t *testing.T) {
	games := []TournamentGameInput{
		{GameID: 1, Draw: true, Opponents: []string{"X"}},
		{GameID: 2, Draw: true, Opponents: []string{"Y"}},
		{GameID: 3, Won: true, Opponents: []string{"Z"}},
	}
	got := ComputeTournamentStats(newRunMeta(), games)
	if got.Draws != 2 {
		t.Errorf("Draws = %d, want 2", got.Draws)
	}
}
