package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// seedArchiveGame seeds a game + a single placeholder seat + a
// snapshot row so the INNER JOIN inside SearchGameSummaries
// finds it. Returns the inserted game id.
func seedArchiveGameWithSnap(t *testing.T, h *Handler, finishedAt int64, winner int, winnerName, commander, deckKey string) int64 {
	t.Helper()
	ctx := context.Background()
	gid, err := db.PersistGameTx(ctx, h.db, db.GameRecord{
		StartedAt:  finishedAt - 60,
		FinishedAt: finishedAt,
		Turns:      10,
		Winner:     winner,
		WinnerName: winnerName,
		EndReason:  "combat",
		Seed:       int64(4242),
	}, []db.GameSeatRecord{
		{Seat: 0, Commander: commander, DeckKey: deckKey},
	})
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	if err := db.InsertGameObservation(ctx, h.db, gid, `{"seed":{"rng_seed":4242}}`); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	return gid
}

type archiveResp struct {
	Rows   []db.SummaryArchiveRow `json:"rows"`
	Count  int                    `json:"count"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

func decodeArchive(t *testing.T, body []byte) archiveResp {
	t.Helper()
	var r archiveResp
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return r
}

func TestHandleGameSummaryArchive_ReturnsRowsForSnapshottedGames(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	seedArchiveGameWithSnap(t, h, 1000, 0, "Krenko", "Krenko, Mob Boss", "alice/krenko")
	seedArchiveGameWithSnap(t, h, 2000, 1, "Atraxa", "Atraxa, Praetors' Voice", "bob/atraxa")

	req := httptest.NewRequest("GET", "/api/games/summaries", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	resp := decodeArchive(t, rec.Body.Bytes())
	if resp.Count != 2 || len(resp.Rows) != 2 {
		t.Errorf("want 2 rows, got count=%d len=%d", resp.Count, len(resp.Rows))
	}
	// Newest first by finished_at.
	if resp.Rows[0].WinnerName != "Atraxa" {
		t.Errorf("position 0 should be Atraxa (newest); got %q", resp.Rows[0].WinnerName)
	}
	if resp.Limit != 50 || resp.Offset != 0 {
		t.Errorf("default limit/offset: want 50/0, got %d/%d", resp.Limit, resp.Offset)
	}
}

func TestHandleGameSummaryArchive_FiltersByCommander(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	seedArchiveGameWithSnap(t, h, 1000, 0, "Krenko", "Krenko, Mob Boss", "alice/krenko")
	seedArchiveGameWithSnap(t, h, 2000, 1, "Atraxa", "Atraxa, Praetors' Voice", "bob/atraxa")

	req := httptest.NewRequest("GET", "/api/games/summaries?commander=Atraxa", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	resp := decodeArchive(t, rec.Body.Bytes())
	if resp.Count != 1 || resp.Rows[0].WinnerName != "Atraxa" {
		t.Errorf("commander filter should match only Atraxa; got %+v", resp)
	}
}

func TestHandleGameSummaryArchive_FiltersByDeck(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	seedArchiveGameWithSnap(t, h, 1000, 0, "K", "Krenko, Mob Boss", "alice/krenko")
	seedArchiveGameWithSnap(t, h, 2000, 1, "A", "Atraxa", "bob/atraxa")

	req := httptest.NewRequest("GET", "/api/games/summaries?deck=alice/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	resp := decodeArchive(t, rec.Body.Bytes())
	if resp.Count != 1 || resp.Rows[0].DeckKeys[0] != "alice/krenko" {
		t.Errorf("deck filter should match alice/krenko; got %+v", resp)
	}
}

func TestHandleGameSummaryArchive_FiltersByWinnerAndDateRange(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	seedArchiveGameWithSnap(t, h, 1000, 0, "Krenko", "Krenko", "alice/k")
	gMatch := seedArchiveGameWithSnap(t, h, 2000, 0, "Krenko", "Krenko", "bob/k")
	seedArchiveGameWithSnap(t, h, 3000, 0, "Atraxa", "Atraxa", "carol/a")

	req := httptest.NewRequest("GET", "/api/games/summaries?winner=Krenko&since=1500&until=2500", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	resp := decodeArchive(t, rec.Body.Bytes())
	if resp.Count != 1 || resp.Rows[0].GameID != gMatch {
		t.Errorf("multi-filter should match only gMatch=%d; got %+v", gMatch, resp)
	}
}

func TestHandleGameSummaryArchive_LimitAndOffsetPaging(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	for i := 0; i < 8; i++ {
		seedArchiveGameWithSnap(t, h, int64(1000+i*100), 0, "W", "C", "d")
	}

	req := httptest.NewRequest("GET", "/api/games/summaries?limit=3", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	page1 := decodeArchive(t, rec.Body.Bytes())
	if page1.Count != 3 || page1.Limit != 3 {
		t.Errorf("page 1: want count=3 limit=3; got %+v", page1)
	}

	req2 := httptest.NewRequest("GET", "/api/games/summaries?limit=3&offset=3", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	page2 := decodeArchive(t, rec2.Body.Bytes())
	if page2.Count != 3 || page2.Offset != 3 {
		t.Errorf("page 2: want count=3 offset=3; got %+v", page2)
	}
	if page1.Rows[2].GameID == page2.Rows[0].GameID {
		t.Errorf("page 2 first should differ from page 1 last; both %d", page1.Rows[2].GameID)
	}
}

func TestHandleGameSummaryArchive_EmptyResultReturnsNonNilRowsArray(t *testing.T) {
	_, mux := newSummaryTestHandler(t)
	req := httptest.NewRequest("GET", "/api/games/summaries", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Body must contain `"rows":[]` (not null) so frontends can
	// iterate without nil-guarding.
	body := rec.Body.String()
	if !contains(body, `"rows": []`) && !contains(body, `"rows":[]`) {
		t.Errorf("empty result should produce non-nil rows array; got %s", body)
	}
}

func TestHandleGameSummaryArchive_503WhenDBMissing(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/games/summaries", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d", rec.Code)
	}
}

func TestHandleGameSummaryArchive_InvalidLimitFallsBackToDefault(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	seedArchiveGameWithSnap(t, h, 100, 0, "W", "C", "d")

	// Non-numeric limit shouldn't 400 — silently fall back to the
	// default 50.
	req := httptest.NewRequest("GET", "/api/games/summaries?limit=abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("invalid limit should not 4xx; got %d", rec.Code)
	}
	resp := decodeArchive(t, rec.Body.Bytes())
	if resp.Limit != 50 {
		t.Errorf("invalid limit should fall back to 50; got %d", resp.Limit)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > 0 && (indexOf(s, substr) >= 0)))
}
func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
