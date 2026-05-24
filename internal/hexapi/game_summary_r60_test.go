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

func newSummaryTestHandler(t *testing.T) (*Handler, http.Handler) {
	t.Helper()
	d, err := db.Open(t.TempDir() + "/summary.db")
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

func seedSummaryGame(t *testing.T, d *Handler, winner, turns int, endReason string) int64 {
	t.Helper()
	ctx := context.Background()
	gid, err := db.PersistGameTx(ctx, d.db, db.GameRecord{
		StartedAt:  100,
		FinishedAt: 200,
		Turns:      turns,
		Winner:     winner,
		WinnerName: "Krenko, Mob Boss",
		EndReason:  endReason,
		Seed:       4242,
	}, []db.GameSeatRecord{
		{Seat: 0, Commander: "Krenko, Mob Boss", DeckKey: "alice/krenko"},
		{Seat: 1, Commander: "Atraxa, Praetors' Voice", DeckKey: "bob/atraxa"},
	})
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	return gid
}

func TestHandleGameSummary_ReturnsDBOnlySummaryForPersistedGame(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 17, "combat")

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var s heimdall.GameSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.GameID != intToStr(gid) {
		t.Errorf("GameID: want %s, got %s", intToStr(gid), s.GameID)
	}
	if s.Winner != 0 || s.Turns != 17 || s.EndReason != "combat" {
		t.Errorf("metadata mismatch: %+v", s)
	}
	if s.WinnerName != "Krenko, Mob Boss" {
		t.Errorf("WinnerName: want Krenko..., got %q", s.WinnerName)
	}
	if s.DataSource != "db_only" {
		t.Errorf("DataSource: want db_only, got %q", s.DataSource)
	}
	// Empty signal arrays should be non-nil so frontend code can iterate.
	if s.CommanderZoneVisits == nil || s.RegretCards == nil || s.MVPCards == nil {
		t.Errorf("R60 signal arrays should be non-nil empty slices; got %+v", s)
	}
	if len(s.TurningPoints) != 1 || s.TurningPoints[0].Kind != "game_end" {
		t.Errorf("expected single game_end turning point; got %+v", s.TurningPoints)
	}
	if s.TurningPoints[0].Turn != 17 || s.TurningPoints[0].Seat != 0 {
		t.Errorf("game_end turning point should match game metadata; got %+v", s.TurningPoints[0])
	}
}

func TestHandleGameSummary_404OnUnknownID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)

	req := httptest.NewRequest("GET", "/api/games/999999/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleGameSummary_400OnInvalidID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)

	for _, bad := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest("GET", "/api/games/"+bad+"/summary", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("id=%q: want 400, got %d", bad, rec.Code)
		}
	}
}

func TestHandleGameSummary_503WhenDBMissing(t *testing.T) {
	h := &Handler{} // no DB attached
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/games/1/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandleGameSummary_DrawGameSurfacesMinusOneWinner(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, -1, 80, "timeout")

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var s heimdall.GameSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Winner != -1 || s.EndReason != "timeout" {
		t.Errorf("draw metadata: %+v", s)
	}
	if len(s.TurningPoints) != 1 ||
		s.TurningPoints[0].Seat != -1 {
		t.Errorf("draw turning point should carry seat=-1; got %+v", s.TurningPoints)
	}
}

// buildDBOnlyGameSummary is exercised through the handler above; this
// unit-test pin lets it fail fast if the helper drifts.
func TestBuildDBOnlyGameSummary_FieldsAndNonNilSlices(t *testing.T) {
	g := db.GameRecord{
		GameID:     42,
		Turns:      11,
		Winner:     2,
		WinnerName: "Edgar Markov",
		EndReason:  "combat",
	}
	s := buildDBOnlyGameSummary(g)
	if s.GameID != "42" || s.Winner != 2 || s.Turns != 11 || s.EndReason != "combat" {
		t.Errorf("metadata mismatch: %+v", s)
	}
	if s.DataSource != "db_only" {
		t.Errorf("DataSource: want db_only, got %q", s.DataSource)
	}
	if s.CommanderZoneVisits == nil || s.RegretCards == nil || s.MVPCards == nil {
		t.Errorf("signal arrays should be initialized to empty (non-nil) slices")
	}
}

func intToStr(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var out []byte
	for n > 0 {
		out = append([]byte{digits[n%10]}, out...)
		n /= 10
	}
	if negative {
		out = append([]byte{'-'}, out...)
	}
	return string(out)
}
