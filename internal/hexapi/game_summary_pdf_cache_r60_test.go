package hexapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// Cache wiring on /api/games/{id}/summary.pdf — mirrors the
// /summary (#202) and /replay (#212) coverage. /summary.pdf
// shares the same Handler-scoped snapshotCache, so priming via
// any of the three routes serves the others from memory.

func TestHandleGameSummaryPDF_CacheServesSecondRequestFromMemory(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 9, "combat")

	snap := heimdall.GameObservationSnapshot{
		Seed:            heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 9},
		CardFirstPlayed: map[string]int{"Sol Ring": 1},
		MVPCards: []heimdall.MVPCard{
			{Seat: 1, CardName: "Atraxa", CMC: 4, Score: 8, IsCommander: true},
		},
		FinalTurn: 9,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d (%s)", i, rec.Code, rec.Body.String())
		}
		if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-1.4")) {
			t.Fatalf("request %d: not a PDF; first bytes %q", i, rec.Body.Bytes()[:16])
		}
	}
	if h.snapshotCache == nil {
		t.Fatal("ensureSnapshotCache should have wired the cache on first /summary.pdf request")
	}
	if h.snapshotCache.len() != 1 {
		t.Errorf("cache should have exactly 1 entry after 2 requests; got %d",
			h.snapshotCache.len())
	}
}

// Upsert invalidation: the new snapshot's content must show up in
// the PDF on the next request after a snapshot upsert.
func TestHandleGameSummaryPDF_CacheInvalidatesOnSnapshotUpsert(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 9, "combat")
	ctx := context.Background()

	snap1 := heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{RNGSeed: 1, Winner: 1, Turns: 5},
		MVPCards: []heimdall.MVPCard{
			{Seat: 0, CardName: "OldMVPCard", CMC: 4, Score: 8},
		},
		FinalTurn: 5,
	}
	p1, _ := heimdall.MarshalSnapshot(snap1)
	if err := db.InsertGameObservation(ctx, h.db, gid, p1); err != nil {
		t.Fatalf("insert 1: %v", err)
	}

	req1 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first PDF: %d", rec1.Code)
	}
	if !bytes.Contains(rec1.Body.Bytes(), []byte("OldMVPCard")) {
		t.Fatalf("first PDF should carry OldMVPCard; got body of len %d", rec1.Body.Len())
	}

	// Sleep so the unixepoch-resolution created_at ticks.
	time.Sleep(time.Second + 100*time.Millisecond)

	snap2 := snap1
	snap2.MVPCards = []heimdall.MVPCard{
		{Seat: 1, CardName: "NewMVPCard", CMC: 6, Score: 12},
	}
	p2, _ := heimdall.MarshalSnapshot(snap2)
	if err := db.InsertGameObservation(ctx, h.db, gid, p2); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	body2 := rec2.Body.Bytes()
	if !bytes.Contains(body2, []byte("NewMVPCard")) {
		t.Errorf("second PDF should reflect upserted snapshot; missing NewMVPCard")
	}
	if bytes.Contains(body2, []byte("OldMVPCard")) {
		t.Errorf("second PDF should NOT carry cached OldMVPCard")
	}
}

// db_only fallback through the cache layer: missing snapshot
// (ErrNoRows) should still produce a valid PDF, matching the
// /summary contract.
func TestHandleGameSummaryPDF_DBOnlyFallbackThroughCache(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	// No snapshot inserted.

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200 (db_only fallback); got %d (%s)",
			rec.Code, rec.Body.String())
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-1.4")) {
		t.Errorf("db_only fallback should still be a valid PDF; got %q", rec.Body.Bytes()[:16])
	}
}

// Malformed payload through the cache layer: should also fall
// back to a valid db_only PDF (not 500), matching the /summary
// contract.
func TestHandleGameSummaryPDF_MalformedFallbackThroughCache(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	if err := db.InsertGameObservation(context.Background(), h.db, gid, "{{ broken"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200 (graceful malformed fallback); got %d",
			rec.Code)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-1.4")) {
		t.Errorf("malformed fallback should still be a valid PDF")
	}
}

// Cache-invariant: priming via /summary serves /summary.pdf from
// memory (and vice versa), so a user clicking through the three
// surfaces of the same game id only pays the snapshot parse cost
// once. Verifies the cache lives on the Handler, not per-route.
func TestHandleGameSummaryPDF_CacheSharedAcrossSummaryAndReplay(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 9, "combat")

	snap := heimdall.GameObservationSnapshot{
		Seed:            heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 9},
		CardFirstPlayed: map[string]int{"Sol Ring": 1},
		FinalTurn:       9,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Visit each route in succession. After the first, the cache
	// should be populated; the next two should hit it.
	routes := []string{
		"/api/games/" + intToStr(gid) + "/summary",
		"/api/games/" + intToStr(gid) + "/replay",
		"/api/games/" + intToStr(gid) + "/summary.pdf",
	}
	for i, url := range routes {
		req := httptest.NewRequest("GET", url, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("route %d (%s): status %d", i, url, rec.Code)
		}
	}
	if h.snapshotCache == nil {
		t.Fatal("cache should be wired after first route")
	}
	if h.snapshotCache.len() != 1 {
		t.Errorf("cache should stay at 1 entry across all three routes; got %d",
			h.snapshotCache.len())
	}
}

// Post-corruption + version-bump: cached parse-success must not
// leak past a corruption event. After a valid snapshot is cached
// and then deliberately corrupted with a version tick, the next
// PDF request should re-parse, hit the malformed-snapshot
// branch, and graceful-fall back to a db_only PDF.
func TestHandleGameSummaryPDF_CacheReheatsToMalformedOnVersionBump(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	ctx := context.Background()

	snap := heimdall.GameObservationSnapshot{
		Seed:            heimdall.GameSeed{RNGSeed: 1, Winner: 0, Turns: 5},
		CardFirstPlayed: map[string]int{"ValidCard": 1},
		FinalTurn:       5,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(ctx, h.db, gid, payload); err != nil {
		t.Fatalf("insert valid: %v", err)
	}

	// Prime the cache.
	req1 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("prime: %d", rec1.Code)
	}

	// Corrupt + wait for version tick.
	time.Sleep(time.Second + 100*time.Millisecond)
	if err := db.InsertGameObservation(ctx, h.db, gid, "{{ broken"); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary.pdf", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	// /summary.pdf's contract is graceful-fallback to db_only PDF,
	// not 500 — same as /summary.
	if rec2.Code != http.StatusOK {
		t.Errorf("post-corruption: want 200 (graceful), got %d", rec2.Code)
	}
	if !bytes.HasPrefix(rec2.Body.Bytes(), []byte("%PDF-1.4")) {
		t.Errorf("post-corruption PDF should still be valid")
	}
	// The fallback PDF should NOT reference the prior cached
	// "ValidCard" — the version check should have evicted it and
	// the malformed-fallback path doesn't have access to the
	// snapshot content.
	if strings.Contains(rec2.Body.String(), "ValidCard") {
		t.Errorf("post-corruption PDF should not leak the cached snapshot's card data")
	}
}
