package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// game_search_test.go — handler tests for GET /api/games/search.
// Pins URL parsing, validation gates, and the example queries the
// endpoint is shaped around.

func newSearchHandler(t *testing.T) (*Handler, http.Handler) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	h := &Handler{}
	h.SetDB(d)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux
}

func seedSearchHandlerGame(t *testing.T, h *Handler, finishedAt int64, turns, winner int,
	commanders, owners, decks [4]string, endReason, winnerName string) int64 {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if owners[i] == "" {
			continue
		}
		dk := owners[i] + "/" + decks[i]
		_, err := h.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, updated_at)
			 VALUES (?, ?, ?, 1500, 0, 0, 0, 0, 0, 0, 0, unixepoch())`,
			dk, commanders[i], owners[i])
		if err != nil {
			t.Fatalf("elo: %v", err)
		}
	}
	g := db.GameRecord{
		StartedAt:  finishedAt - 600,
		FinishedAt: finishedAt,
		Turns:      turns,
		Winner:     winner,
		WinnerName: winnerName,
		EndReason:  endReason,
		Seed:       finishedAt,
	}
	seats := make([]db.GameSeatRecord, 0, 4)
	for i := 0; i < 4; i++ {
		seats = append(seats, db.GameSeatRecord{
			Seat:      i,
			Commander: commanders[i],
			DeckKey:   owners[i] + "/" + decks[i],
			Lost:      i != winner,
		})
	}
	gid, err := db.PersistGameTx(ctx, h.db, g, seats)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	return gid
}

// TestHandleGameSearch_HappyPath_ExampleQuery pins the example query
// from the task brief: "find all my games where I lost to a board
// wipe on turn 5."
func TestHandleGameSearch_HappyPath_ExampleQuery(t *testing.T) {
	h, mux := newSearchHandler(t)
	// Target match: alice lost to a wipe on turn 5.
	seedSearchHandlerGame(t, h, 100, 5, 1,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "wipe", "B")
	// Distractor: alice WON on turn 5.
	seedSearchHandlerGame(t, h, 200, 5, 0,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "wipe", "A")
	// Distractor: alice lost on turn 12.
	seedSearchHandlerGame(t, h, 300, 12, 1,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "wipe", "B")

	req := httptest.NewRequest("GET",
		"/api/games/search?owner=alice&won=false&min_turn=5&max_turn=5&end_reason=wipe", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out struct {
		Rows  []db.GameSearchRow `json:"rows"`
		Count int                `json:"count"`
		Owner string             `json:"owner"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Count != 1 {
		t.Errorf("count = %d, want 1 (only the lost-wipe-turn-5 game)", out.Count)
	}
	if out.Owner != "alice" {
		t.Errorf("owner echo = %q, want 'alice'", out.Owner)
	}
	if out.Rows[0].FinishedAt != 100 || out.Rows[0].OwnerWon ||
		out.Rows[0].Turns != 5 || out.Rows[0].EndReason != "wipe" {
		t.Errorf("matched game wrong: %+v", out.Rows[0])
	}
}

// TestHandleGameSearch_QParamThreeWayMatch pins that q hits
// winner_name OR commander OR end_reason.
func TestHandleGameSearch_QParamThreeWayMatch(t *testing.T) {
	h, mux := newSearchHandler(t)
	seedSearchHandlerGame(t, h, 100, 8, 0,
		[4]string{"Krenko", "Atraxa", "Z", "W"},
		[4]string{"a", "b", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "Krenko")
	seedSearchHandlerGame(t, h, 200, 8, 0,
		[4]string{"Other", "Foo", "Bar", "Baz"},
		[4]string{"a", "b", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "decking", "Other")

	for _, c := range []struct {
		q    string
		want int
	}{
		{"krenko", 1},  // winner_name hit
		{"atraxa", 1},  // commander hit
		{"decking", 1}, // end_reason hit
		{"zzzz", 0},    // no match
	} {
		req := httptest.NewRequest("GET", "/api/games/search?q="+c.q, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out struct {
			Count int `json:"count"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if out.Count != c.want {
			t.Errorf("q=%q → count=%d, want %d", c.q, out.Count, c.want)
		}
	}
}

// TestHandleGameSearch_WonRequiresOwner pins the 400 when ?won is
// supplied without ?owner.
func TestHandleGameSearch_WonRequiresOwner(t *testing.T) {
	_, mux := newSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/games/search?won=true", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (won without owner)", rec.Code)
	}
}

// TestHandleGameSearch_WonInvalidValueRejected pins the 400 on a
// non-boolean ?won value.
func TestHandleGameSearch_WonInvalidValueRejected(t *testing.T) {
	_, mux := newSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/games/search?owner=alice&won=maybe", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (won=maybe)", rec.Code)
	}
}

// TestHandleGameSearch_InvalidOwnerPathComponent pins the 400 on a
// non-path-component-safe ?owner.
func TestHandleGameSearch_InvalidOwnerPathComponent(t *testing.T) {
	_, mux := newSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/games/search?owner=..%2Fevil", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (path traversal)", rec.Code)
	}
}

// TestHandleGameSearch_DBUnavailable pins the 503 guard.
func TestHandleGameSearch_DBUnavailable(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/games/search", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

// TestHandleGameSearch_LimitOffsetEchoed pins that the response
// echoes the effective limit/offset back to the caller.
func TestHandleGameSearch_LimitOffsetEchoed(t *testing.T) {
	_, mux := newSearchHandler(t)
	req := httptest.NewRequest("GET", "/api/games/search?limit=7&offset=3", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Limit != 7 || out.Offset != 3 {
		t.Errorf("echo: limit=%d offset=%d, want 7/3", out.Limit, out.Offset)
	}
}
