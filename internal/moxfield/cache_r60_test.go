package moxfield

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// cache_r60_test.go — pins the two-layer Moxfield deck cache that
// fixes the importer's "two HTTP calls per import + zero cross-import
// dedup" perf bug. Each test isolates cache state via t.TempDir().

const fixtureJSON = `{
	"name": "Test Deck",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"a": {"quantity": 1, "card": {"name": "Sen Triplets"}}}},
		"mainboard":  {"count": 1, "cards": {"b": {"quantity": 1, "card": {"name": "Sol Ring"}}}}
	}
}`

// newCacheTestEnv wires httptest.Server + an isolated cache dir for one
// test, resets all cache state, and returns a hit counter + cleanup.
func newCacheTestEnv(t *testing.T) (counter *int32, cleanup func()) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureJSON))
	}))

	origBase := apiBaseVar
	apiBaseVar = srv.URL

	tmp := t.TempDir()
	origDir := os.Getenv(cacheEnvDir)
	origDisable := os.Getenv(cacheEnvDisable)
	origTTL := os.Getenv(cacheEnvTTL)
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(cacheEnvTTL, "")
	// Snapshots are written on every API hit (not cache hit); isolate
	// to tmp so test runs don't pollute the user's real ~/.local/share.
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))

	resetCachesForTest()

	return &hits, func() {
		srv.Close()
		apiBaseVar = origBase
		_ = origDir
		_ = origDisable
		_ = origTTL
	}
}

// FetchDeckName + FetchDeck back-to-back must share a single HTTP call.
// This is the original importer perf bug.
func TestCache_FetchDeckNameAndFetchDeckShareSingleHTTPCall(t *testing.T) {
	hits, done := newCacheTestEnv(t)
	defer done()

	name, err := FetchDeckName("https://moxfield.com/decks/abc123")
	if err != nil {
		t.Fatalf("FetchDeckName: %v", err)
	}
	if name != "Test Deck" {
		t.Errorf("FetchDeckName name = %q, want %q", name, "Test Deck")
	}

	text, err := FetchDeck("https://moxfield.com/decks/abc123")
	if err != nil {
		t.Fatalf("FetchDeck: %v", err)
	}
	if !containsLine(text, "COMMANDER: Sen Triplets") {
		t.Errorf("FetchDeck missing commander line:\n%s", text)
	}

	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("expected exactly 1 HTTP hit (in-proc cache should dedupe), got %d", got)
	}
}

// Disk cache survives a "fresh process" — simulated by clearing the
// in-process map but leaving the temp cache dir intact.
func TestCache_DiskHitAfterInProcReset(t *testing.T) {
	hits, done := newCacheTestEnv(t)
	defer done()

	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("first fetch should be 1 hit, got %d", got)
	}

	// Drop only the in-process layer to simulate a fresh CLI invocation.
	inProcCache.Range(func(k, _ any) bool { inProcCache.Delete(k); return true })

	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 1 {
		t.Fatalf("disk cache should serve second fetch (0 new hits), got total %d", got)
	}
}

// HEXDEK_MOXFIELD_CACHE=0 disables the disk cache entirely. Each fetch
// hits the API (in-process cache still applies within one process).
func TestCache_DiskDisabledViaEnv(t *testing.T) {
	hits, done := newCacheTestEnv(t)
	defer done()
	t.Setenv(cacheEnvDisable, "0")

	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	// Drop in-proc cache; if disk cache were active, no new HTTP.
	inProcCache.Range(func(k, _ any) bool { inProcCache.Delete(k); return true })
	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 2 {
		t.Fatalf("disk-cache-disabled: expected 2 HTTP hits, got %d", got)
	}
	// And the file must NOT be on disk.
	if _, err := os.Stat(cachePath("abc123")); !os.IsNotExist(err) {
		t.Fatalf("disk cache file should not exist when HEXDEK_MOXFIELD_CACHE=0; stat err=%v", err)
	}
}

// Expired (mtime older than TTL) entries fall through to the API.
func TestCache_DiskExpiredFallsThroughToAPI(t *testing.T) {
	hits, done := newCacheTestEnv(t)
	defer done()
	t.Setenv(cacheEnvTTL, "1") // 1 second TTL

	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	// Age the file past TTL by rewriting its mtime.
	path := cachePath("abc123")
	old := time.Now().Add(-10 * time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	inProcCache.Range(func(k, _ any) bool { inProcCache.Delete(k); return true })

	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("expired fetch: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 2 {
		t.Fatalf("expired-cache: expected 2 HTTP hits, got %d", got)
	}
}

// Refresh(id) invalidates both layers so the next fetch hits the API.
func TestCache_RefreshInvalidatesBothLayers(t *testing.T) {
	hits, done := newCacheTestEnv(t)
	defer done()

	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("first fetch: %v", err)
	}

	Refresh("abc123")

	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("post-refresh fetch: %v", err)
	}
	if got := atomic.LoadInt32(hits); got != 2 {
		t.Fatalf("after Refresh: expected 2 HTTP hits, got %d", got)
	}
	if _, err := os.Stat(cachePath("abc123")); err != nil && !os.IsNotExist(err) {
		// File should exist again after the second fetch repopulated it.
		t.Logf("cache file after refresh+refetch stat: %v", err)
	}
}

// Different deck IDs do not share cache entries.
func TestCache_DifferentDeckIDsIsolated(t *testing.T) {
	hits, done := newCacheTestEnv(t)
	defer done()

	for _, id := range []string{"abc123", "def456", "ghi789"} {
		if _, err := FetchDeckByID(id); err != nil {
			t.Fatalf("fetch %s: %v", id, err)
		}
	}
	if got := atomic.LoadInt32(hits); got != 3 {
		t.Fatalf("expected 3 HTTP hits for 3 distinct IDs, got %d", got)
	}
	// All three files must be on disk.
	for _, id := range []string{"abc123", "def456", "ghi789"} {
		if _, err := os.Stat(cachePath(id)); err != nil {
			t.Errorf("missing cache file for %s: %v", id, err)
		}
	}
}

// Concurrent FetchDeckByID calls for the same ID are safe (sync.Map
// guarantees we don't corrupt state). The race detector + a stress
// loop catch any regression to plain map[string]*apiResponse.
func TestCache_ConcurrentFetchesSafe(t *testing.T) {
	_, done := newCacheTestEnv(t)
	defer done()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := FetchDeckByID("abc123"); err != nil {
				t.Errorf("concurrent fetch: %v", err)
			}
		}()
	}
	wg.Wait()
}

// cachePath sanitizes weird deck IDs so the importer never writes
// outside the cache dir.
func TestCache_PathSanitization(t *testing.T) {
	_, done := newCacheTestEnv(t)
	defer done()
	dir := cacheDir()
	for _, evil := range []string{"../etc/passwd", "../../escape", "abc/def", "abc\\def"} {
		got := cachePath(evil)
		if got == "" {
			t.Errorf("cachePath(%q) returned empty", evil)
			continue
		}
		rel, err := filepath.Rel(dir, got)
		if err != nil {
			t.Errorf("Rel: %v", err)
			continue
		}
		if rel == ".." || (len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)) {
			t.Errorf("cachePath(%q) escapes cache dir: %s", evil, got)
		}
	}
}

// Sanity: a fetch failure (non-200) must not poison the cache with a
// half-baked entry that prevents recovery on the next attempt.
func TestCache_NoPoisonOnAPIError(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.WriteHeader(500)
			fmt.Fprintln(w, "boom")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureJSON))
	}))
	defer srv.Close()

	origBase := apiBaseVar
	apiBaseVar = srv.URL
	defer func() { apiBaseVar = origBase }()

	tmp := t.TempDir()
	t.Setenv(cacheEnvDir, tmp)
	t.Setenv(cacheEnvDisable, "")
	t.Setenv(cacheEnvTTL, "")
	t.Setenv(snapshotEnvDir, filepath.Join(tmp, "snapshots"))
	resetCachesForTest()

	if _, err := FetchDeckByID("abc123"); err == nil {
		t.Fatalf("expected error on first (500) fetch")
	}
	if _, err := FetchDeckByID("abc123"); err != nil {
		t.Fatalf("second fetch should succeed: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("expected 2 HTTP hits (1 fail, 1 success); got %d", got)
	}
}
