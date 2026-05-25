package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// player_trends_r60_test.go — HTTP integration test for the new
// GET /api/players/{id}/trends endpoint. Pins the route surface, the
// DB → heimdall translation, the filesystem-backed archetype
// lookup, and the empty-state shape.

func newTrendsTestEnv(t *testing.T) (*Handler, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	decksDir := t.TempDir()

	h := &Handler{
		DecksDir: decksDir,
		Showmatch: &Showmatch{
			sqlDB: d,
		},
	}
	return h, decksDir
}

// seedGame inserts one finished game with 4 seats. ownerSeats maps
// seat index → owner string. The deck_key for seat i is
// owners[i] + "/" + deckIDs[i].
func seedGame(t *testing.T, h *Handler, finishedAt int64, winnerSeat int, commanders [4]string, owners [4]string, deckIDs [4]string) {
	t.Helper()
	ctx := context.Background()
	// Register the deck_key in showmatch_elo so the LoadOwnerGames JOIN
	// matches.
	for i := 0; i < 4; i++ {
		if owners[i] == "" {
			continue
		}
		deckKey := owners[i] + "/" + deckIDs[i]
		_, err := h.Showmatch.sqlDB.ExecContext(ctx,
			`INSERT OR IGNORE INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, updated_at)
			 VALUES (?, ?, ?, 1500, 0, 0, 0, 0, 0, 0, 0, unixepoch())`,
			deckKey, commanders[i], owners[i])
		if err != nil {
			t.Fatalf("insert elo seat %d: %v", i, err)
		}
	}
	g := db.GameRecord{
		StartedAt:  finishedAt - 600,
		FinishedAt: finishedAt,
		Turns:      8,
		Winner:     winnerSeat,
		WinnerName: commanders[winnerSeat],
		EndReason:  "combat",
		Seed:       finishedAt, // unique per game
	}
	seats := make([]db.GameSeatRecord, 0, 4)
	for i := 0; i < 4; i++ {
		seats = append(seats, db.GameSeatRecord{
			Seat:      i,
			Commander: commanders[i],
			DeckKey:   owners[i] + "/" + deckIDs[i],
			Life:      0,
			Lost:      i != winnerSeat,
		})
	}
	if _, err := db.PersistGameTx(ctx, h.Showmatch.sqlDB, g, seats); err != nil {
		t.Fatalf("persist game: %v", err)
	}
}

func writeArchetypeStrategy(t *testing.T, decksDir, owner, deckID, archetype string) {
	t.Helper()
	writeStrategyJSON(t, decksDir, owner, deckID, map[string]any{"archetype": archetype})
}

// -----------------------------------------------------------------------------
// Happy path: 5 games, 2 wins, 3 archetypes, repeat opponent.
// -----------------------------------------------------------------------------

func TestHandlePlayerTrends_HappyPath(t *testing.T) {
	h, decksDir := newTrendsTestEnv(t)

	// alice runs voltron in games 1-3, combo in games 4-5.
	writeArchetypeStrategy(t, decksDir, "alice", "voltron-deck", "voltron")
	writeArchetypeStrategy(t, decksDir, "alice", "combo-deck", "combo")

	// Game 1: alice (voltron) wins; opps Atraxa, Edgar, Krenko.
	seedGame(t, h, 1000,
		0,
		[4]string{"Sram", "Atraxa", "Edgar Markov", "Krenko"},
		[4]string{"alice", "bob", "carol", "dave"},
		[4]string{"voltron-deck", "atraxa-deck", "edgar-deck", "krenko-deck"})
	// Game 2: alice (voltron) wins; Atraxa repeats.
	seedGame(t, h, 1100,
		0,
		[4]string{"Sram", "Atraxa", "Yuriko", "Korvold"},
		[4]string{"alice", "bob", "eve", "frank"},
		[4]string{"voltron-deck", "atraxa-deck", "yuriko-deck", "korvold-deck"})
	// Game 3: alice (voltron) loses.
	seedGame(t, h, 1200,
		1,
		[4]string{"Sram", "Atraxa", "Tergrid", "Najeela"},
		[4]string{"alice", "bob", "grace", "henry"},
		[4]string{"voltron-deck", "atraxa-deck", "tergrid-deck", "najeela-deck"})
	// Game 4: alice (combo) loses; draw end_reason but here a winner.
	seedGame(t, h, 1300,
		1,
		[4]string{"Thrasios", "Tymna", "Sisay", "Urza"},
		[4]string{"alice", "bob", "iris", "jake"},
		[4]string{"combo-deck", "tymna-deck", "sisay-deck", "urza-deck"})
	// Game 5: alice (combo) — DRAW (winner=-1).
	seedGame(t, h, 1400,
		0, // ignored when we override below
		[4]string{"Thrasios", "Atraxa", "Animar", "Ghave"},
		[4]string{"alice", "bob", "kara", "liam"},
		[4]string{"combo-deck", "atraxa-deck", "animar-deck", "ghave-deck"})
	// Force the last game's winner to -1 (draw) to exercise that path.
	_, _ = h.Showmatch.sqlDB.ExecContext(context.Background(),
		`UPDATE showmatch_game SET winner = -1 WHERE finished_at = 1400`)

	req := httptest.NewRequest(http.MethodGet, "/api/players/alice/trends", nil)
	req.SetPathValue("id", "alice")
	w := httptest.NewRecorder()
	h.handlePlayerTrends(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		PlayerID    string `json:"player_id"`
		WindowSize  int    `json:"window_size"`
		GamesPlayed int    `json:"games_played"`
		Wins        int    `json:"wins"`
		Losses      int    `json:"losses"`
		Draws       int    `json:"draws"`
		WinRate     float64
		Archetypes  []struct {
			Archetype string `json:"archetype"`
			Games     int    `json:"games"`
			Wins      int    `json:"wins"`
		} `json:"archetype_distribution"`
		OpponentDiversity struct {
			UniqueCommanders int `json:"unique_commanders"`
			TotalEncounters  int `json:"total_encounters"`
			TopRepeats       []struct {
				Commander  string `json:"commander"`
				Encounters int    `json:"encounters"`
			} `json:"top_repeat_opponents"`
		} `json:"opponent_diversity"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if got.PlayerID != "alice" {
		t.Errorf("PlayerID = %q, want alice", got.PlayerID)
	}
	if got.WindowSize != 30 {
		t.Errorf("WindowSize = %d, want 30 (default)", got.WindowSize)
	}
	if got.GamesPlayed != 5 {
		t.Errorf("GamesPlayed = %d, want 5", got.GamesPlayed)
	}
	if got.Wins != 2 || got.Losses != 2 || got.Draws != 1 {
		t.Errorf("W/L/D = %d/%d/%d, want 2/2/1", got.Wins, got.Losses, got.Draws)
	}
	// Archetypes: voltron(3 games, 2 wins), combo(2 games, 0 wins).
	if len(got.Archetypes) != 2 {
		t.Fatalf("expected 2 archetypes, got %d (%+v)", len(got.Archetypes), got.Archetypes)
	}
	if got.Archetypes[0].Archetype != "voltron" || got.Archetypes[0].Games != 3 || got.Archetypes[0].Wins != 2 {
		t.Errorf("first archetype = %+v, want voltron 3/2", got.Archetypes[0])
	}
	if got.Archetypes[1].Archetype != "combo" || got.Archetypes[1].Games != 2 {
		t.Errorf("second archetype = %+v, want combo 2", got.Archetypes[1])
	}
	// Opponent diversity: Atraxa shows up 4 times (games 1, 2, 3, 5).
	if got.OpponentDiversity.TotalEncounters != 15 {
		t.Errorf("TotalEncounters = %d, want 15", got.OpponentDiversity.TotalEncounters)
	}
	foundAtraxa := false
	for _, r := range got.OpponentDiversity.TopRepeats {
		if r.Commander == "Atraxa" {
			foundAtraxa = true
			if r.Encounters != 4 {
				t.Errorf("Atraxa encounters = %d, want 4", r.Encounters)
			}
		}
	}
	if !foundAtraxa {
		t.Errorf("expected Atraxa in top repeats, got %+v", got.OpponentDiversity.TopRepeats)
	}
}

// -----------------------------------------------------------------------------
// Empty state: unknown player still gets a well-formed payload.
// -----------------------------------------------------------------------------

func TestHandlePlayerTrends_UnknownPlayerReturnsEmpty(t *testing.T) {
	h, _ := newTrendsTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/players/nobody/trends", nil)
	req.SetPathValue("id", "nobody")
	w := httptest.NewRecorder()
	h.handlePlayerTrends(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got struct {
		GamesPlayed int `json:"games_played"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.GamesPlayed != 0 {
		t.Errorf("GamesPlayed = %d, want 0", got.GamesPlayed)
	}
}

// -----------------------------------------------------------------------------
// Window cap.
// -----------------------------------------------------------------------------

func TestHandlePlayerTrends_WindowCappedAtMax(t *testing.T) {
	h, _ := newTrendsTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/players/alice/trends?window=99999", nil)
	req.SetPathValue("id", "alice")
	w := httptest.NewRecorder()
	h.handlePlayerTrends(w, req)

	var got struct {
		WindowSize int `json:"window_size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.WindowSize != playerTrendsMaxWindow {
		t.Errorf("WindowSize = %d, want %d (capped)", got.WindowSize, playerTrendsMaxWindow)
	}
}

// -----------------------------------------------------------------------------
// Invalid id.
// -----------------------------------------------------------------------------

func TestHandlePlayerTrends_InvalidIDRejected(t *testing.T) {
	h, _ := newTrendsTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/players/.bad/trends", nil)
	req.SetPathValue("id", "../etc/passwd")
	w := httptest.NewRecorder()
	h.handlePlayerTrends(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for invalid id", w.Code)
	}
}
