package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

func seedCompareGame(t *testing.T, h *Handler, winner, turns int, end, winnerName string) int64 {
	t.Helper()
	gid, err := db.PersistGameTx(context.Background(), h.db, db.GameRecord{
		StartedAt:  1, FinishedAt: 2, Turns: turns, Winner: winner,
		WinnerName: winnerName, EndReason: end, Seed: 1,
	}, []db.GameSeatRecord{
		{Seat: 0, Commander: "Krenko, Mob Boss", DeckKey: "alice/krenko"},
		{Seat: 1, Commander: "Atraxa, Praetors' Voice", DeckKey: "bob/atraxa"},
	})
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	return gid
}

// TestHandleGameCompare_HappyPath pins the 200 + JSON shape with two
// real db-only games.
func TestHandleGameCompare_HappyPath(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gidA := seedCompareGame(t, h, 0, 12, "combat", "Krenko, Mob Boss")
	gidB := seedCompareGame(t, h, 1, 18, "decking", "Atraxa, Praetors' Voice")

	req := httptest.NewRequest("GET",
		"/api/games/compare?a="+intToStr(gidA)+"&b="+intToStr(gidB), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var cmp heimdall.GameComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &cmp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
	if cmp.GameA.GameID != intToStr(gidA) || cmp.GameB.GameID != intToStr(gidB) {
		t.Errorf("embedded ids: got a=%s b=%s, want %s/%s",
			cmp.GameA.GameID, cmp.GameB.GameID, intToStr(gidA), intToStr(gidB))
	}
	if cmp.Outcome.SameWinnerSeat || cmp.Outcome.SameWinnerName || cmp.Outcome.SameEndReason {
		t.Errorf("outcome should diff on all axes: %+v", cmp.Outcome)
	}
	if cmp.Turns.Delta != -6 {
		t.Errorf("turns delta: got %d, want -6 (12 - 18)", cmp.Turns.Delta)
	}
	// db_only games have no MVPs / no rich snapshot — diff slices
	// should be non-nil empty, not null.
	if cmp.MVPDiff.OnlyInA == nil || cmp.MVPDiff.OnlyInB == nil || cmp.MVPDiff.Shared == nil {
		t.Errorf("MVPDiff slices should be non-nil: %+v", cmp.MVPDiff)
	}
	// db_only summaries always emit a single game_end turning point
	// → divergence rows for the two game-end turns.
	if len(cmp.TurningPointDivergence) == 0 {
		t.Errorf("expected at least one turning-point divergence row")
	}
}

// TestHandleGameCompare_MissingParam returns 400 when either id is
// blank.
func TestHandleGameCompare_MissingParam(t *testing.T) {
	_, mux := newSummaryTestHandler(t)

	for _, url := range []string{
		"/api/games/compare",
		"/api/games/compare?a=5",
		"/api/games/compare?b=7",
	} {
		req := httptest.NewRequest("GET", url, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got status %d, want 400", url, rec.Code)
		}
	}
}

// TestHandleGameCompare_InvalidParam returns 400 for non-numeric, ≤0,
// or otherwise unparseable ids.
func TestHandleGameCompare_InvalidParam(t *testing.T) {
	_, mux := newSummaryTestHandler(t)
	for _, url := range []string{
		"/api/games/compare?a=abc&b=5",
		"/api/games/compare?a=5&b=xyz",
		"/api/games/compare?a=0&b=5",
		"/api/games/compare?a=5&b=-1",
	} {
		req := httptest.NewRequest("GET", url, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got status %d, want 400 (body: %s)", url, rec.Code, rec.Body.String())
		}
	}
}

// TestHandleGameCompare_RejectsEqualIDs returns 400 when a == b — the
// caller almost certainly meant two different games and a self-
// comparison would be uninteresting noise.
func TestHandleGameCompare_RejectsEqualIDs(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedCompareGame(t, h, 0, 10, "combat", "X")

	req := httptest.NewRequest("GET",
		"/api/games/compare?a="+intToStr(gid)+"&b="+intToStr(gid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestHandleGameCompare_NotFound returns 404 with a clear "game a:"
// or "game b:" prefix so the caller can tell which id was bad.
func TestHandleGameCompare_NotFound(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedCompareGame(t, h, 0, 10, "combat", "X")

	// a unknown
	req := httptest.NewRequest("GET",
		"/api/games/compare?a=999999&b="+intToStr(gid), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown a: got %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}

	// b unknown
	req = httptest.NewRequest("GET",
		"/api/games/compare?a="+intToStr(gid)+"&b=999999", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown b: got %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestHandleGameCompare_DBUnavailable pins the 503 guard for the
// no-DB boot edge.
func TestHandleGameCompare_DBUnavailable(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/games/compare?a=1&b=2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", rec.Code)
	}
}
