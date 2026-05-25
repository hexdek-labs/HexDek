package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

// tournament_stats_r60_test.go — HTTP-level integration test for
// GET /api/tournament/{id}/stats. Exercises the gauntlet_runs +
// window-scoped showmatch_game scan + heimdall computation pipeline.

func newTournamentStatsEnv(t *testing.T) *Handler {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	return &Handler{
		Showmatch: &Showmatch{
			sqlDB: d,
		},
	}
}

// seedTournamentRun inserts a gauntlet_runs row plus the given
// per-game showmatch_game rows. games is one (turns, winnerSeat,
// opps) triple per game; the deck under test always sits at seat 0.
func seedTournamentRun(t *testing.T, h *Handler, deckKey, commander string, startedAt, finishedAt time.Time, games []tournamentGameSeed) int64 {
	t.Helper()
	ctx := context.Background()

	for _, g := range games {
		insertWindowedGame(t, h, deckKey, commander, g)
	}

	wins := 0
	losses := 0
	for _, g := range games {
		if g.winnerSeat == 0 {
			wins++
		} else if g.winnerSeat >= 0 {
			losses++
		}
	}
	winRate := 0.0
	if len(games) > 0 {
		winRate = float64(wins) / float64(len(games))
	}
	rec := db.GauntletRunRecord{
		DeckKey:    deckKey,
		Commander:  commander,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Games:      len(games),
		Wins:       wins,
		Losses:     losses,
		WinRate:    winRate,
		AvgTurns:   8.0,
	}
	if err := db.InsertGauntletRun(ctx, h.Showmatch.sqlDB, rec); err != nil {
		t.Fatalf("insert gauntlet run: %v", err)
	}
	var runID int64
	if err := h.Showmatch.sqlDB.QueryRowContext(ctx,
		`SELECT run_id FROM gauntlet_runs ORDER BY run_id DESC LIMIT 1`).Scan(&runID); err != nil {
		t.Fatalf("scan run_id: %v", err)
	}
	return runID
}

type tournamentGameSeed struct {
	finishedAt int64
	turns      int
	winnerSeat int // 0 = deck under test wins; -1 = draw; >0 = opponent wins
	opponents  [3]string
}

func insertWindowedGame(t *testing.T, h *Handler, deckKey, commander string, g tournamentGameSeed) {
	t.Helper()
	ctx := context.Background()

	// Make sure showmatch_elo carries the deck so future joins succeed.
	_, err := h.Showmatch.sqlDB.ExecContext(ctx,
		`INSERT OR IGNORE INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, updated_at)
		 VALUES (?, ?, '', 1500, 0, 0, 0, 0, 0, 0, 0, unixepoch())`, deckKey, commander)
	if err != nil {
		t.Fatalf("insert elo: %v", err)
	}

	rec := db.GameRecord{
		StartedAt:  g.finishedAt - int64(g.turns)*60,
		FinishedAt: g.finishedAt,
		Turns:      g.turns,
		Winner:     g.winnerSeat,
		WinnerName: "x",
		EndReason:  "combat",
		Seed:       g.finishedAt,
	}
	seats := []db.GameSeatRecord{
		{Seat: 0, Commander: commander, DeckKey: deckKey, Lost: g.winnerSeat != 0},
		{Seat: 1, Commander: g.opponents[0], DeckKey: "other/" + g.opponents[0], Lost: g.winnerSeat != 1},
		{Seat: 2, Commander: g.opponents[1], DeckKey: "other/" + g.opponents[1], Lost: g.winnerSeat != 2},
		{Seat: 3, Commander: g.opponents[2], DeckKey: "other/" + g.opponents[2], Lost: g.winnerSeat != 3},
	}
	if _, err := db.PersistGameTx(ctx, h.Showmatch.sqlDB, rec, seats); err != nil {
		t.Fatalf("persist game: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Happy path: per-game aggregates surface end-to-end.
// -----------------------------------------------------------------------------

func TestHandleTournamentStats_HappyPath(t *testing.T) {
	h := newTournamentStatsEnv(t)

	started := time.Unix(1000, 0)
	finished := time.Unix(9000, 0)
	games := []tournamentGameSeed{
		{finishedAt: 1100, turns: 8, winnerSeat: 0, opponents: [3]string{"Atraxa", "Edgar", "Krenko"}},
		{finishedAt: 2200, turns: 22, winnerSeat: 1, opponents: [3]string{"Atraxa", "Yuriko", "Korvold"}},
		{finishedAt: 3300, turns: 4, winnerSeat: 0, opponents: [3]string{"Atraxa", "Tergrid", "Najeela"}},
		{finishedAt: 4400, turns: 11, winnerSeat: 2, opponents: [3]string{"Edgar", "Yuriko", "Atraxa"}},
	}
	runID := seedTournamentRun(t, h, "alice/iroh", "Iroh", started, finished, games)

	req := httptest.NewRequest(http.MethodGet,
		"/api/tournament/"+runIDString(runID)+"/stats", nil)
	req.SetPathValue("id", runIDString(runID))
	w := httptest.NewRecorder()
	h.handleTournamentStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var got struct {
		TournamentID int64  `json:"tournament_id"`
		DeckKey      string `json:"deck_key"`
		Games        int    `json:"games"`
		Wins         int    `json:"wins"`
		Losses       int    `json:"losses"`
		DeckDistribution []struct {
			Commander string `json:"commander"`
			Games     int    `json:"games"`
		} `json:"deck_distribution"`
		WinrateVariance struct {
			SampleSize int     `json:"sample_size"`
			Variance   float64 `json:"variance"`
		} `json:"winrate_variance"`
		LongestGame *struct {
			GameID int64 `json:"game_id"`
			Turns  int   `json:"turns"`
		} `json:"longest_game"`
		FastestGame *struct {
			GameID int64 `json:"game_id"`
			Turns  int   `json:"turns"`
		} `json:"fastest_game"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if got.TournamentID != runID {
		t.Errorf("TournamentID = %d, want %d", got.TournamentID, runID)
	}
	if got.DeckKey != "alice/iroh" {
		t.Errorf("DeckKey = %q, want alice/iroh", got.DeckKey)
	}
	if got.Games != 4 || got.Wins != 2 || got.Losses != 2 {
		t.Errorf("scalars = G/W/L %d/%d/%d, want 4/2/2", got.Games, got.Wins, got.Losses)
	}
	// Atraxa appears in all 4 games → top of deck_distribution.
	if len(got.DeckDistribution) == 0 || got.DeckDistribution[0].Commander != "Atraxa" || got.DeckDistribution[0].Games != 4 {
		t.Errorf("top opponent = %+v, want Atraxa/4", got.DeckDistribution[0])
	}
	if got.LongestGame == nil || got.LongestGame.Turns != 22 {
		t.Errorf("LongestGame = %+v, want Turns=22", got.LongestGame)
	}
	if got.FastestGame == nil || got.FastestGame.Turns != 4 {
		t.Errorf("FastestGame = %+v, want Turns=4", got.FastestGame)
	}
}

// -----------------------------------------------------------------------------
// Unknown run_id → 404.
// -----------------------------------------------------------------------------

func TestHandleTournamentStats_UnknownIDReturns404(t *testing.T) {
	h := newTournamentStatsEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/tournament/9999/stats", nil)
	req.SetPathValue("id", "9999")
	w := httptest.NewRecorder()
	h.handleTournamentStats(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for unknown run_id", w.Code)
	}
}

// -----------------------------------------------------------------------------
// Non-numeric id → 400.
// -----------------------------------------------------------------------------

func TestHandleTournamentStats_NonNumericIDReturns400(t *testing.T) {
	h := newTournamentStatsEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/tournament/abc/stats", nil)
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()
	h.handleTournamentStats(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for non-numeric id", w.Code)
	}
}

// -----------------------------------------------------------------------------
// Window filter — games outside the gauntlet window are excluded.
// -----------------------------------------------------------------------------

func TestHandleTournamentStats_WindowFilterExcludesOutsideGames(t *testing.T) {
	h := newTournamentStatsEnv(t)

	// Gauntlet window [1000, 2000].
	started := time.Unix(1000, 0)
	finished := time.Unix(2000, 0)
	games := []tournamentGameSeed{
		{finishedAt: 1200, turns: 8, winnerSeat: 0, opponents: [3]string{"Atraxa", "Edgar", "Krenko"}},
		{finishedAt: 1500, turns: 6, winnerSeat: 1, opponents: [3]string{"Atraxa", "Yuriko", "Korvold"}},
	}
	runID := seedTournamentRun(t, h, "alice/iroh", "Iroh", started, finished, games)

	// A later game by the same deck — outside the window. Must NOT
	// pollute this tournament's stats.
	insertWindowedGame(t, h, "alice/iroh", "Iroh",
		tournamentGameSeed{finishedAt: 5000, turns: 30, winnerSeat: 0,
			opponents: [3]string{"OutOfWindow1", "OutOfWindow2", "OutOfWindow3"}})

	req := httptest.NewRequest(http.MethodGet,
		"/api/tournament/"+runIDString(runID)+"/stats", nil)
	req.SetPathValue("id", runIDString(runID))
	w := httptest.NewRecorder()
	h.handleTournamentStats(w, req)

	var got struct {
		DeckDistribution []struct {
			Commander string `json:"commander"`
		} `json:"deck_distribution"`
		LongestGame *struct {
			Turns int `json:"turns"`
		} `json:"longest_game"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, d := range got.DeckDistribution {
		if d.Commander == "OutOfWindow1" || d.Commander == "OutOfWindow2" || d.Commander == "OutOfWindow3" {
			t.Errorf("out-of-window opponent %q bled into the stats", d.Commander)
		}
	}
	if got.LongestGame != nil && got.LongestGame.Turns == 30 {
		t.Errorf("LongestGame picked up the out-of-window 30-turn game")
	}
}

func runIDString(n int64) string {
	return strconv.FormatInt(n, 10)
}
