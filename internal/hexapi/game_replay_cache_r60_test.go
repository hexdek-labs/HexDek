package hexapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// Cache wiring on /api/games/{id}/replay — mirrors the
// summary-side coverage from snapshot_cache_r60_test.go but
// enforces /replay's stricter error contract (404 on missing
// snapshot, 500 on malformed payload).

func TestHandleGameReplay_CacheServesSecondRequestFromMemory(t *testing.T) {
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

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, rec.Code)
		}
	}
	if h.snapshotCache == nil {
		t.Fatal("ensureSnapshotCache should have wired the cache on first /replay")
	}
	if h.snapshotCache.len() != 1 {
		t.Errorf("cache should have 1 entry after 2 requests; got %d",
			h.snapshotCache.len())
	}
}

func TestHandleGameReplay_CacheInvalidatesOnSnapshotUpsert(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 9, "combat")
	ctx := context.Background()

	snap1 := heimdall.GameObservationSnapshot{
		Seed:            heimdall.GameSeed{RNGSeed: 1, Winner: 0, Turns: 5},
		CardFirstPlayed: map[string]int{"OldCard": 2},
		FinalTurn:       5,
	}
	p1, _ := heimdall.MarshalSnapshot(snap1)
	if err := db.InsertGameObservation(ctx, h.db, gid, p1); err != nil {
		t.Fatalf("insert 1: %v", err)
	}

	req1 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first replay: %d", rec1.Code)
	}
	if !contains(rec1.Body.String(), "OldCard") {
		t.Fatalf("first response should reference OldCard; got %s", rec1.Body.String())
	}

	// Sleep so the unixepoch-resolution created_at ticks.
	time.Sleep(time.Second + 100*time.Millisecond)

	snap2 := heimdall.GameObservationSnapshot{
		Seed:            heimdall.GameSeed{RNGSeed: 1, Winner: 0, Turns: 5},
		CardFirstPlayed: map[string]int{"NewCard": 3},
		FinalTurn:       5,
	}
	p2, _ := heimdall.MarshalSnapshot(snap2)
	if err := db.InsertGameObservation(ctx, h.db, gid, p2); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	body2 := rec2.Body.String()
	if !contains(body2, "NewCard") {
		t.Errorf("second response should reflect upserted snapshot; got %s", body2)
	}
	if contains(body2, "OldCard") {
		t.Errorf("second response should NOT carry the cached OldCard; got %s", body2)
	}
}

// Stricter contract: missing snapshot still 404s through the
// cache layer (not a graceful db_only fallback like /summary).
func TestHandleGameReplay_404WhenSnapshotMissing_ThroughCache(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	// No snapshot inserted — cache lookup should return ErrNoRows.

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404 (snapshot missing); got %d (%s)",
			rec.Code, rec.Body.String())
	}
}

// Stricter contract: malformed snapshot 500s (vs /summary's
// graceful fallback). Verified through the cache layer.
func TestHandleGameReplay_500OnMalformedSnapshot_ThroughCache(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	if err := db.InsertGameObservation(context.Background(), h.db, gid, "{{ not json"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500 (malformed); got %d", rec.Code)
	}
}

// Cache hits don't poison subsequent malformed-snapshot paths —
// after a valid snapshot is cached and then deliberately
// corrupted in the DB, the next request after the version ticks
// should re-fetch and surface 500. Verifies the cache version
// check actually fires before the cached parse-success leaks
// past a corruption event.
func TestHandleGameReplay_CacheReheatsToMalformedOnVersionBump(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	ctx := context.Background()

	snap := heimdall.GameObservationSnapshot{
		Seed:            heimdall.GameSeed{RNGSeed: 1, Winner: 0, Turns: 5},
		CardFirstPlayed: map[string]int{"X": 1},
		FinalTurn:       5,
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(ctx, h.db, gid, payload); err != nil {
		t.Fatalf("insert valid: %v", err)
	}
	// Prime the cache.
	req1 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("prime: %d", rec1.Code)
	}

	// Corrupt the payload and wait for the version tick.
	time.Sleep(time.Second + 100*time.Millisecond)
	if err := db.InsertGameObservation(ctx, h.db, gid, "{{ broken"); err != nil {
		t.Fatalf("corrupt: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500 after corruption + version bump; got %d (%s)",
			rec2.Code, rec2.Body.String())
	}
}

// Cache is shared across /summary and /replay handlers — priming
// from one handler serves the other from memory.
func TestHandleGameReplay_CacheSharedWithSummary(t *testing.T) {
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

	// First request goes to /summary — primes the cache.
	req1 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("summary prime: %d", rec1.Code)
	}

	// Second request to /replay should hit the SAME cache entry
	// (the cache lives on the Handler, shared across routes).
	if h.snapshotCache == nil || h.snapshotCache.len() != 1 {
		t.Fatalf("cache should have 1 entry after summary prime")
	}
	req2 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("replay should hit shared cache; got %d", rec2.Code)
	}
	// Still 1 entry — /replay didn't double-cache.
	if h.snapshotCache.len() != 1 {
		t.Errorf("cache shouldn't grow on shared hit; got len=%d",
			h.snapshotCache.len())
	}
}
