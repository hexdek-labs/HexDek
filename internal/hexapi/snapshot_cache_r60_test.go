package hexapi

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// snapshotCache primitives — exercise hit / miss / version
// invalidation / LRU eviction without touching the DB.

func TestSnapshotCache_HitReturnsSameSnapshot(t *testing.T) {
	c := newSnapshotCache(8)
	snap := heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{RNGSeed: 1, Winner: 0, Turns: 5},
	}
	c.put(42, 100, snap)

	got, ok := c.get(42, 100)
	if !ok {
		t.Fatal("expected cache hit on matching version")
	}
	if got.Seed.RNGSeed != 1 {
		t.Errorf("returned snapshot differs: %+v", got)
	}
}

func TestSnapshotCache_MissOnUnknownKey(t *testing.T) {
	c := newSnapshotCache(8)
	if _, ok := c.get(99, 1); ok {
		t.Errorf("expected miss on unknown game id")
	}
}

func TestSnapshotCache_VersionMismatchEvicts(t *testing.T) {
	c := newSnapshotCache(8)
	c.put(7, 100, heimdall.GameObservationSnapshot{})

	if _, ok := c.get(7, 200); ok {
		t.Errorf("expected miss on version mismatch")
	}
	// After the mismatched get, the stale entry should be removed
	// so memory doesn't accumulate.
	if c.len() != 0 {
		t.Errorf("stale entry should be evicted on version mismatch; cache size=%d", c.len())
	}
}

func TestSnapshotCache_PutOverwritesAndBumpsLRU(t *testing.T) {
	c := newSnapshotCache(8)
	c.put(1, 1, heimdall.GameObservationSnapshot{Seed: heimdall.GameSeed{RNGSeed: 1}})
	c.put(1, 2, heimdall.GameObservationSnapshot{Seed: heimdall.GameSeed{RNGSeed: 2}})

	if c.len() != 1 {
		t.Errorf("re-put should NOT grow the cache; got len=%d", c.len())
	}
	got, ok := c.get(1, 2)
	if !ok || got.Seed.RNGSeed != 2 {
		t.Errorf("re-put should overwrite the prior entry; got %+v ok=%v", got, ok)
	}
}

func TestSnapshotCache_LRUEvictionAtCap(t *testing.T) {
	c := newSnapshotCache(3)
	for i := int64(1); i <= 3; i++ {
		c.put(i, 100, heimdall.GameObservationSnapshot{})
	}
	// Touch entry 1 so it stays MRU.
	if _, ok := c.get(1, 100); !ok {
		t.Fatal("entry 1 should be cached")
	}
	// Insert a 4th — entry 2 (LRU) should evict.
	c.put(4, 100, heimdall.GameObservationSnapshot{})

	if c.len() != 3 {
		t.Errorf("cache should stay at cap=3; got %d", c.len())
	}
	if _, ok := c.get(2, 100); ok {
		t.Errorf("LRU entry 2 should have been evicted")
	}
	for _, id := range []int64{1, 3, 4} {
		if _, ok := c.get(id, 100); !ok {
			t.Errorf("entry %d should still be cached", id)
		}
	}
}

func TestSnapshotCache_InvalidateRemovesEntry(t *testing.T) {
	c := newSnapshotCache(8)
	c.put(5, 100, heimdall.GameObservationSnapshot{})

	c.invalidate(5)
	if _, ok := c.get(5, 100); ok {
		t.Errorf("invalidate should remove the entry")
	}
	c.invalidate(999) // no-op for unknown id — must not panic
}

func TestSnapshotCache_ZeroCapDefaultsTo1024(t *testing.T) {
	c := newSnapshotCache(0)
	if c.cap != 1024 {
		t.Errorf("zero cap should default to 1024; got %d", c.cap)
	}
	c2 := newSnapshotCache(-5)
	if c2.cap != 1024 {
		t.Errorf("negative cap should default to 1024; got %d", c2.cap)
	}
}

// db.LoadGameObservationVersion — cheap created_at probe.

func TestLoadGameObservationVersion_RoundTrip(t *testing.T) {
	h, _ := newSummaryTestHandler(t)
	ctx := context.Background()
	gid := seedSummaryGame(t, h, 0, 5, "combat")

	now := time.Now().Unix()
	if err := db.InsertGameObservation(ctx, h.db, gid, `{"v":1}`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	v, err := db.LoadGameObservationVersion(ctx, h.db, gid)
	if err != nil {
		t.Fatalf("version load: %v", err)
	}
	// created_at uses unixepoch() — should be within a few seconds
	// of test start.
	if v < now-5 || v > now+5 {
		t.Errorf("version %d should be near now=%d", v, now)
	}

	// Upsert bumps created_at — call again, version should be >=
	// the prior one.
	time.Sleep(time.Second) // make sure unixepoch() ticks
	if err := db.InsertGameObservation(ctx, h.db, gid, `{"v":2}`); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	v2, err := db.LoadGameObservationVersion(ctx, h.db, gid)
	if err != nil {
		t.Fatalf("version load 2: %v", err)
	}
	if v2 < v {
		t.Errorf("post-upsert version %d should be >= prior %d", v2, v)
	}
}

func TestLoadGameObservationVersion_ReturnsErrNoRows(t *testing.T) {
	h, _ := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 5, "combat")
	// No snapshot inserted.
	_, err := db.LoadGameObservationVersion(context.Background(), h.db, gid)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("want sql.ErrNoRows for missing snapshot; got %v", err)
	}
}

// End-to-end: cache hit on a second handler request.

func TestHandleGameSummary_CacheServesSecondRequestFromMemory(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 9, "combat")
	snap := heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 9},
		MVPCards: []heimdall.MVPCard{
			{Seat: 1, CardName: "Atraxa", CMC: 4, Score: 8, IsCommander: true},
		},
	}
	payload, _ := heimdall.MarshalSnapshot(snap)
	if err := db.InsertGameObservation(context.Background(), h.db, gid, payload); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// First request — cache miss, primes the entry.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, rec.Code)
		}
	}

	// Cache should have exactly one entry for this game.
	if h.snapshotCache == nil {
		t.Fatal("ensureSnapshotCache should have lazily wired the cache")
	}
	if h.snapshotCache.len() != 1 {
		t.Errorf("cache should have 1 entry after 2 requests; got %d",
			h.snapshotCache.len())
	}
}

// Cache invalidates correctly after a snapshot upsert — the second
// request should pick up the new payload, not the cached one.
func TestHandleGameSummary_CacheInvalidatesOnSnapshotUpsert(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 9, "combat")
	ctx := context.Background()

	snap1 := heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{RNGSeed: 1, Winner: 1, Turns: 5},
		MVPCards: []heimdall.MVPCard{
			{Seat: 0, CardName: "OldMVP", CMC: 4, Score: 8},
		},
	}
	p1, _ := heimdall.MarshalSnapshot(snap1)
	if err := db.InsertGameObservation(ctx, h.db, gid, p1); err != nil {
		t.Fatalf("insert 1: %v", err)
	}

	req1 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec1 := httptest.NewRecorder()
	mux.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("request 1: %d", rec1.Code)
	}
	body1 := rec1.Body.String()
	if !contains(body1, "OldMVP") {
		t.Fatalf("first response should carry OldMVP; got: %s", body1)
	}

	// Sleep so created_at definitely ticks (unixepoch is second-
	// resolution).
	time.Sleep(time.Second + 100*time.Millisecond)

	snap2 := snap1
	snap2.MVPCards = []heimdall.MVPCard{
		{Seat: 1, CardName: "NewMVP", CMC: 6, Score: 12},
	}
	p2, _ := heimdall.MarshalSnapshot(snap2)
	if err := db.InsertGameObservation(ctx, h.db, gid, p2); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	body2 := rec2.Body.String()
	if !contains(body2, "NewMVP") {
		t.Errorf("second response should reflect the upserted snapshot; got: %s", body2)
	}
	if contains(body2, "OldMVP") {
		t.Errorf("second response should NOT carry the cached OldMVP; got: %s", body2)
	}
}

func TestIsMalformedSnapshot_DiscriminatesErrorKind(t *testing.T) {
	if IsMalformedSnapshot(sql.ErrNoRows) {
		t.Errorf("sql.ErrNoRows should not be classified as malformed")
	}
	if !IsMalformedSnapshot(errMalformedSnapshot) {
		t.Errorf("the sentinel itself must classify as malformed")
	}
}
