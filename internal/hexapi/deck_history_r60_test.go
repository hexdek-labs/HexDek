package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// Pins GET /api/decks/{owner}/{id}/history — per-game performance
// chronologically + an aggregate-totals roll-up.

func newDeckHistoryEnv(t *testing.T) (*Handler, http.Handler) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	h := &Handler{Showmatch: &Showmatch{sqlDB: d}}
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux
}

// seedDeckHistoryGame persists one 4-seat game where seat 0 is
// owner/deckID. winnerSeat = -1 for draws, 0..3 for normal wins.
func seedDeckHistoryGame(t *testing.T, h *Handler, owner, deckID string, finishedAt int64, turns, winnerSeat int, endReason string) int64 {
	t.Helper()
	ctx := context.Background()
	deckKey := owner + "/" + deckID
	g := db.GameRecord{
		StartedAt:  finishedAt - 600,
		FinishedAt: finishedAt,
		Turns:      turns,
		Winner:     winnerSeat,
		WinnerName: "Test Commander",
		EndReason:  endReason,
		Seed:       finishedAt,
	}
	seats := []db.GameSeatRecord{
		{Seat: 0, Commander: "Test Commander", DeckKey: deckKey},
		{Seat: 1, Commander: "Opp 1", DeckKey: "opp1/deck"},
		{Seat: 2, Commander: "Opp 2", DeckKey: "opp2/deck"},
		{Seat: 3, Commander: "Opp 3", DeckKey: "opp3/deck"},
	}
	gid, err := db.PersistGameTx(ctx, h.Showmatch.sqlDB, g, seats)
	if err != nil {
		t.Fatalf("PersistGameTx: %v", err)
	}
	return gid
}

func getHistory(t *testing.T, mux http.Handler, owner, id, query string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	url := "/api/decks/" + owner + "/" + id + "/history"
	if query != "" {
		url += "?" + query
	}
	req := httptest.NewRequest("GET", url, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return rec, nil
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	return rec, body
}

// Happy path: 4 games (2W, 1L, 1D) → totals correct, games chronological-
// ascending, win_rate = 2/4 = 0.5, avg_turns averaged across all 4.
func TestHandleDeckHistory_AggregatesAndChronologicalOrder(t *testing.T) {
	h, mux := newDeckHistoryEnv(t)

	// Seeds NOT in chronological order so we can verify the handler
	// reverses the DB's newest-first into ascending order for plotting.
	seedDeckHistoryGame(t, h, "alice", "krenko", 2000, 12, 0, "combat")  // win
	seedDeckHistoryGame(t, h, "alice", "krenko", 1000, 8, 0, "combat")   // win
	seedDeckHistoryGame(t, h, "alice", "krenko", 3000, 15, 2, "commander") // loss
	seedDeckHistoryGame(t, h, "alice", "krenko", 4000, 20, -1, "timeout")  // draw

	rec, body := getHistory(t, mux, "alice", "krenko", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	if got := body["deck_key"]; got != "alice/krenko" {
		t.Errorf("deck_key: got %v", got)
	}

	totals, _ := body["totals"].(map[string]any)
	if totals == nil {
		t.Fatalf("totals missing")
	}
	if g, _ := totals["games"].(float64); g != 4 {
		t.Errorf("totals.games: want 4, got %v", totals["games"])
	}
	if wins, _ := totals["wins"].(float64); wins != 2 {
		t.Errorf("totals.wins: want 2, got %v", totals["wins"])
	}
	if losses, _ := totals["losses"].(float64); losses != 1 {
		t.Errorf("totals.losses: want 1, got %v", totals["losses"])
	}
	if draws, _ := totals["draws"].(float64); draws != 1 {
		t.Errorf("totals.draws: want 1, got %v", totals["draws"])
	}
	wr, _ := totals["win_rate"].(float64)
	if wr != 0.5 {
		t.Errorf("totals.win_rate: want 0.5 (ratio of 2/4), got %v", wr)
	}
	avgT, _ := totals["avg_turns"].(float64)
	if avgT != 13.75 { // (12+8+15+20)/4
		t.Errorf("totals.avg_turns: want 13.75, got %v", avgT)
	}

	games, _ := body["games"].([]any)
	if len(games) != 4 {
		t.Fatalf("games: want 4, got %d", len(games))
	}
	// Chronological-ascending by finished_at.
	wantFinish := []float64{1000, 2000, 3000, 4000}
	for i, raw := range games {
		row, _ := raw.(map[string]any)
		fin, _ := row["finished_at"].(float64)
		if fin != wantFinish[i] {
			t.Errorf("games[%d].finished_at: want %v, got %v", i, wantFinish[i], fin)
		}
	}
	// Last game is the draw.
	last, _ := games[3].(map[string]any)
	if d, _ := last["draw"].(bool); !d {
		t.Errorf("last game should be draw: %v", last)
	}
	if w, _ := last["won"].(bool); w {
		t.Errorf("draw game should not also be won: %v", last)
	}
	if er := last["end_reason"]; er != "timeout" {
		t.Errorf("last game end_reason: want timeout, got %v", er)
	}
}

// Empty path: deck with no games returns empty games + zero totals,
// win_rate / avg_turns omitted via omitempty.
func TestHandleDeckHistory_EmptyWindow(t *testing.T) {
	_, mux := newDeckHistoryEnv(t)

	rec, body := getHistory(t, mux, "nobody", "no-games", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	games, _ := body["games"].([]any)
	if len(games) != 0 {
		t.Errorf("games: want empty, got %d", len(games))
	}
	totals, _ := body["totals"].(map[string]any)
	if g, _ := totals["games"].(float64); g != 0 {
		t.Errorf("totals.games: want 0, got %v", totals["games"])
	}
	if _, ok := totals["win_rate"]; ok {
		t.Errorf("win_rate should be omitted for empty window; got %v", totals["win_rate"])
	}
	if _, ok := totals["avg_turns"]; ok {
		t.Errorf("avg_turns should be omitted for empty window; got %v", totals["avg_turns"])
	}
}

// Limit parameter caps the window. 5 games seeded, limit=3 → 3 games
// returned (the 3 most recent, then reversed to ascending), totals
// aggregate ONLY across those 3 (windowed view, not lifetime).
func TestHandleDeckHistory_LimitWindowsTotals(t *testing.T) {
	h, mux := newDeckHistoryEnv(t)
	// 5 wins.
	for i := int64(1); i <= 5; i++ {
		seedDeckHistoryGame(t, h, "bob", "atraxa", i*1000, 10, 0, "combat")
	}

	rec, body := getHistory(t, mux, "bob", "atraxa", "limit=3")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	games, _ := body["games"].([]any)
	if len(games) != 3 {
		t.Fatalf("games: want 3 (limit), got %d", len(games))
	}
	totals, _ := body["totals"].(map[string]any)
	if g, _ := totals["games"].(float64); g != 3 {
		t.Errorf("totals.games: want 3 (windowed), got %v", totals["games"])
	}
	if w, _ := totals["wins"].(float64); w != 3 {
		t.Errorf("totals.wins: want 3 (all 3 in window are wins), got %v", totals["wins"])
	}
	// Newest 3 games are finished_at 3000, 4000, 5000 → reversed
	// to 3000, 4000, 5000 ascending.
	first, _ := games[0].(map[string]any)
	if fin, _ := first["finished_at"].(float64); fin != 3000 {
		t.Errorf("first game after limit-window: want finished_at=3000, got %v", fin)
	}
}

// Limit > 500 silently clamps to 500 (no 400 — same UX as elo-history).
// Verify by passing limit=99999 with 0 games and checking 200.
func TestHandleDeckHistory_LimitClampedAtMax(t *testing.T) {
	_, mux := newDeckHistoryEnv(t)

	rec, _ := getHistory(t, mux, "alice", "krenko", "limit=99999")
	if rec.Code != http.StatusOK {
		t.Errorf("status: want 200 (clamp not 400), got %d", rec.Code)
	}
}

// nil Showmatch wiring: handler returns 200 with empty totals + games
// rather than 500. Same pattern as deck_snapshot's record block.
func TestHandleDeckHistory_NilShowmatchReturnsEmpty(t *testing.T) {
	h := &Handler{} // no Showmatch
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko/history", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200 with nil Showmatch, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	games, _ := body["games"].([]any)
	if len(games) != 0 {
		t.Errorf("games: want empty when Showmatch nil, got %d", len(games))
	}
	totals, _ := body["totals"].(map[string]any)
	if g, _ := totals["games"].(float64); g != 0 {
		t.Errorf("totals.games: want 0, got %v", totals["games"])
	}
}

// 400 on invalid path component (mirrors deck_snapshot's contract).
func TestHandleDeckHistory_400OnInvalidPathChar(t *testing.T) {
	_, mux := newDeckHistoryEnv(t)

	req := httptest.NewRequest("GET", "/api/decks/alice@bad/krenko/history", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}
}
