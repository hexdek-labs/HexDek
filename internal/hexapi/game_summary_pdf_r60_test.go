package hexapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// db_only path: no snapshot persisted → handler still serves a
// valid PDF built from the GameRecord metadata.
func TestHandleGameSummaryPDF_DBOnlyPath(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 12, "combat")

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("Content-Type: want application/pdf, got %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "hexdek-game-") {
		t.Errorf("Content-Disposition should trigger download with hexdek-game- filename; got %q", cd)
	}
	body := rec.Body.Bytes()
	if !bytes.HasPrefix(body, []byte("%PDF-1.4")) {
		t.Errorf("body should be a PDF; first bytes: %q", body[:16])
	}
	if !bytes.Contains(body, []byte("%%EOF")) {
		t.Errorf("body should contain PDF EOF marker")
	}
	// Content-Length header matches actual body length.
	if cl := rec.Header().Get("Content-Length"); cl != intToStr(int64(len(body))) {
		t.Errorf("Content-Length mismatch: header %q vs actual %d", cl, len(body))
	}
}

// Rich path: persisted snapshot drives the PDF content; signals
// like commander zone visits + MVP labels appear in the bytes.
func TestHandleGameSummaryPDF_RichPathRendersSignals(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 9, "combat")

	snap := heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 9},
		CommanderZoneVisits: []heimdall.CommanderZoneVisit{
			{Seat: 1, CommanderName: "AtraxaTest", CastCount: 2, InZoneAtEnd: true, Visits: 3},
		},
		RegretCards: []heimdall.RegretCard{
			{Seat: 0, CardName: "PlagueWind", CMC: 9, Reason: "stranded_in_hand"},
		},
		MVPCards: []heimdall.MVPCard{
			{Seat: 1, CardName: "AtraxaTest", CMC: 4, Score: 8, IsCommander: true, TurnPlayed: 5},
		},
		CommanderNamesBySeat: [][]string{nil, {"AtraxaTest"}, nil, nil},
		CardFirstPlayed:      map[string]int{"AtraxaTest": 5},
		FinalTurn:            9,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	wantStrings := []string{"AtraxaTest", "PlagueWind", "COMMANDER ZONE VISITS", "REGRET CARDS", "MVP CARDS", "TURNING POINTS"}
	for _, s := range wantStrings {
		if !strings.Contains(body, s) {
			t.Errorf("PDF body missing %q", s)
		}
	}
}

// Authoritative end-state overlay: snapshot says winner=2, but
// GameRecord says winner=1 → the PDF labels winner SEAT 1.
func TestHandleGameSummaryPDF_RecordWinsOverSnapshotForWinner(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 14, "combat")

	snap := heimdall.GameObservationSnapshot{
		Seed:                 heimdall.GameSeed{RNGSeed: 4242, Winner: 2, Turns: 10},
		CommanderNamesBySeat: [][]string{nil, nil, nil, nil},
		FinalTurn:            10,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "WINNER: SEAT 1") {
		t.Errorf("GameRecord winner (1) should overlay snapshot (2); got body:\n%s",
			extractTextSection(body, "WINNER"))
	}
}

// Malformed snapshot in DB: handler logs and falls back to
// db_only PDF (200, not 500) — same robustness contract as
// /summary.
func TestHandleGameSummaryPDF_MalformedSnapshotFallsBackToDBOnly(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 7, "combat")
	if err := db.InsertGameObservation(context.Background(), h.db, gid, "{{ not json"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200 (graceful), got %d (%s)", rec.Code, rec.Body.String())
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-1.4")) {
		t.Errorf("fallback body should still be a PDF; got %q", rec.Body.Bytes()[:16])
	}
}

func TestHandleGameSummaryPDF_404OnUnknownID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)
	req := httptest.NewRequest("GET", "/api/games/9999999/summary.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rec.Code)
	}
}

func TestHandleGameSummaryPDF_400OnInvalidID(t *testing.T) {
	_, mux := newSummaryTestHandler(t)
	for _, bad := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest("GET", "/api/games/"+bad+"/summary.pdf", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("id=%q: want 400, got %d", bad, rec.Code)
		}
	}
}

func TestHandleGameSummaryPDF_503WhenDBMissing(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/games/1/summary.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d", rec.Code)
	}
}

// extractTextSection returns a short slice around the first match
// of needle in haystack for error messages.
func extractTextSection(haystack, needle string) string {
	idx := strings.Index(haystack, needle)
	if idx < 0 {
		return "(not found)"
	}
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + 40
	if end > len(haystack) {
		end = len(haystack)
	}
	return "..." + haystack[start:end] + "..."
}
