package oracle

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestResponseCache_HitMissBasic pins the core contract: a Put followed
// by a Get returns the stored card, and an unwritten key misses.
func TestResponseCache_HitMissBasic(t *testing.T) {
	rc := NewResponseCache(10, time.Hour)
	card := &Card{Name: "Sol Ring"}

	if _, ok := rc.Get("Sol Ring"); ok {
		t.Fatal("empty cache should miss")
	}
	rc.Put("Sol Ring", card)
	got, ok := rc.Get("Sol Ring")
	if !ok {
		t.Fatal("Get after Put should hit")
	}
	if got != card {
		t.Fatal("Get must return the exact card pointer that was Put")
	}
}

// TestResponseCache_KeyCanonicalization confirms case + whitespace
// variants share an entry — matches Lookup's SQLite key normalization,
// so the in-memory cache doesn't grow N copies of "Sol Ring" / "sol
// ring" / " Sol Ring ".
func TestResponseCache_KeyCanonicalization(t *testing.T) {
	rc := NewResponseCache(10, time.Hour)
	rc.Put("Sol Ring", &Card{Name: "Sol Ring"})

	for _, variant := range []string{"sol ring", "SOL RING", "  Sol Ring  ", "Sol Ring"} {
		if _, ok := rc.Get(variant); !ok {
			t.Fatalf("variant %q should hit canonicalized entry", variant)
		}
	}
	if got := rc.Len(); got != 1 {
		t.Fatalf("4 case variants should map to 1 entry, got %d", got)
	}
}

// TestResponseCache_TTLExpiry pins the lazy-expiration contract:
// entries past their TTL miss AND are removed in the same Get call.
func TestResponseCache_TTLExpiry(t *testing.T) {
	rc := NewResponseCache(10, time.Minute)
	clock := time.Unix(1_700_000_000, 0)
	rc.now = func() time.Time { return clock }

	rc.Put("Island", &Card{Name: "Island"})
	if _, ok := rc.Get("Island"); !ok {
		t.Fatal("fresh entry should hit")
	}

	clock = clock.Add(59 * time.Second)
	if _, ok := rc.Get("Island"); !ok {
		t.Fatal("entry within TTL should still hit")
	}

	clock = clock.Add(2 * time.Second) // total +61s, past 1m TTL
	if _, ok := rc.Get("Island"); ok {
		t.Fatal("entry past TTL should miss")
	}
	if rc.Len() != 0 {
		t.Fatalf("stale entry should be removed on Get-miss, len=%d", rc.Len())
	}
}

// TestResponseCache_LRUEviction confirms over-capacity inserts evict
// the LEAST recently used entry, not an arbitrary one.
func TestResponseCache_LRUEviction(t *testing.T) {
	rc := NewResponseCache(3, time.Hour)
	rc.Put("a", &Card{Name: "a"})
	rc.Put("b", &Card{Name: "b"})
	rc.Put("c", &Card{Name: "c"})
	// Touch "a" so "b" becomes the LRU.
	if _, ok := rc.Get("a"); !ok {
		t.Fatal("a should hit before eviction")
	}
	rc.Put("d", &Card{Name: "d"})

	if _, ok := rc.Get("b"); ok {
		t.Fatal("b was LRU and should have been evicted")
	}
	for _, k := range []string{"a", "c", "d"} {
		if _, ok := rc.Get(k); !ok {
			t.Fatalf("%q should still be cached", k)
		}
	}
	if rc.Len() != 3 {
		t.Fatalf("capacity bound violated, len=%d", rc.Len())
	}
}

// TestResponseCache_PutRefreshesTTL confirms Put-on-existing-key
// extends the expiration AND moves the entry to most-recently-used.
func TestResponseCache_PutRefreshesTTL(t *testing.T) {
	rc := NewResponseCache(2, time.Minute)
	clock := time.Unix(1_700_000_000, 0)
	rc.now = func() time.Time { return clock }

	rc.Put("a", &Card{Name: "a-v1"})
	clock = clock.Add(45 * time.Second)
	rc.Put("a", &Card{Name: "a-v2"})

	clock = clock.Add(45 * time.Second) // 90s since v1, 45s since v2
	got, ok := rc.Get("a")
	if !ok {
		t.Fatal("refreshed entry should still hit (TTL reset by Put)")
	}
	if got.Name != "a-v2" {
		t.Fatalf("Get must return refreshed value, got %q", got.Name)
	}
}

// TestResponseCache_NilSafe pins the no-op contract for nil receivers
// and nil values so callers don't need per-call guards.
func TestResponseCache_NilSafe(t *testing.T) {
	var rc *ResponseCache
	if _, ok := rc.Get("x"); ok {
		t.Fatal("nil cache must miss")
	}
	rc.Put("x", &Card{Name: "x"}) // must not panic
	if rc.Len() != 0 {
		t.Fatal("nil cache Len should be 0")
	}

	rc = NewResponseCache(5, time.Hour)
	rc.Put("x", nil) // must not panic and must not insert
	if rc.Len() != 0 {
		t.Fatal("Put(nil) should not insert")
	}
}

// TestResponseCache_DegenerateCapacity pins the misconfiguration
// defense — capacity ≤ 0 clamps to 1 rather than panicking on the
// zero-length list.Back() path.
func TestResponseCache_DegenerateCapacity(t *testing.T) {
	rc := NewResponseCache(0, time.Hour)
	rc.Put("a", &Card{Name: "a"})
	rc.Put("b", &Card{Name: "b"})
	if rc.Len() != 1 {
		t.Fatalf("capacity clamped to 1, expected len=1, got %d", rc.Len())
	}
	if _, ok := rc.Get("a"); ok {
		t.Fatal("a should have been evicted by b")
	}
	if _, ok := rc.Get("b"); !ok {
		t.Fatal("b should be the lone entry")
	}
}

// TestResponseCache_ConcurrentAccess confirms the mutex actually
// serializes — race detector catches torn-write / map-grow races, the
// post-condition catches lost writes.
func TestResponseCache_ConcurrentAccess(t *testing.T) {
	rc := NewResponseCache(1000, time.Hour)
	var wg sync.WaitGroup
	const workers = 8
	const ops = 500
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				key := string(rune('a' + (i % 26)))
				rc.Put(key, &Card{Name: key})
				rc.Get(key)
			}
		}(w)
	}
	wg.Wait()
	// 26 unique keys, capacity 1000 → no eviction, all present.
	if rc.Len() != 26 {
		t.Fatalf("expected 26 distinct entries, got %d", rc.Len())
	}
}

// TestHandlerRegister_InstallsDefaultCache mirrors the limiter-default
// contract: a Handler built without an explicit Cache must NOT be
// deployed re-querying every request. Defends against future refactors
// that drop the Register default.
func TestHandlerRegister_InstallsDefaultCache(t *testing.T) {
	h := &Handler{}
	h.Register(http.NewServeMux())
	if h.Cache == nil {
		t.Fatal("Register must install a default cache when none was provided")
	}
	if h.Cache.capacity != 1000 {
		t.Fatalf("default capacity should be 1000, got %d", h.Cache.capacity)
	}
	if h.Cache.ttl != time.Hour {
		t.Fatalf("default TTL should be 1h, got %s", h.Cache.ttl)
	}
}

// TestHandlerRegister_PreservesCustomCache lets callers tune
// capacity/TTL without Register stomping them.
func TestHandlerRegister_PreservesCustomCache(t *testing.T) {
	custom := NewResponseCache(50, 5*time.Minute)
	h := &Handler{Cache: custom}
	h.Register(http.NewServeMux())
	if h.Cache != custom {
		t.Fatal("Register must not replace a caller-provided cache")
	}
}

// TestLookupCard_CacheHitSkipsDB is the load-bearing end-to-end test:
// a second lookup for the same card must NOT hit Scryfall — it must
// serve from the in-memory cache, and the response must carry
// X-Cache: HIT.
func TestLookupCard_CacheHitSkipsDB(t *testing.T) {
	stub := newScryfallStub(t, time.Millisecond, nil)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(time.Millisecond)()

	h := &Handler{
		DB:      newOracleTestDB(t),
		Limiter: NewInboundRateLimiter(1000, 100), // out of the way
		Cache:   NewResponseCache(100, time.Hour),
	}
	mux := http.NewServeMux()
	h.Register(mux)

	// First request: MISS, populates cache. Stub always returns a card
	// for /cards/named so the lookup succeeds end-to-end.
	req1 := httptest.NewRequest("GET", "/api/oracle/card/Sol%20Ring", nil)
	req1.RemoteAddr = "1.1.1.1:1"
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d (body=%q)", w1.Code, w1.Body.String())
	}
	if got := w1.Header().Get("X-Cache"); got != "MISS" {
		t.Fatalf("first request X-Cache: want MISS, got %q", got)
	}

	hitsBefore := atomic.LoadInt32(&stub.totalHits)

	// Second request, same name: HIT, no Scryfall call.
	req2 := httptest.NewRequest("GET", "/api/oracle/card/Sol%20Ring", nil)
	req2.RemoteAddr = "2.2.2.2:1"
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", w2.Code)
	}
	if got := w2.Header().Get("X-Cache"); got != "HIT" {
		t.Fatalf("second request X-Cache: want HIT, got %q", got)
	}
	hitsAfter := atomic.LoadInt32(&stub.totalHits)
	if hitsAfter != hitsBefore {
		t.Fatalf("cache HIT must not call Scryfall, hits %d → %d", hitsBefore, hitsAfter)
	}
	if !strings.Contains(w2.Body.String(), "Sol Ring") {
		t.Fatalf("HIT response should contain the card payload, got %q", w2.Body.String())
	}

	// Case-insensitive variant must also HIT — the canonicalization
	// guarantee is end-to-end, not just primitive-level.
	req3 := httptest.NewRequest("GET", "/api/oracle/card/sol%20ring", nil)
	req3.RemoteAddr = "3.3.3.3:1"
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	if got := w3.Header().Get("X-Cache"); got != "HIT" {
		t.Fatalf("lowercase variant X-Cache: want HIT, got %q", got)
	}
}

// TestLookupCard_NotFoundIsNotCached defends the cache-poisoning
// surface: an attacker hitting /api/oracle/card/asdfasdf must not be
// able to pin junk keys in the cache. Uses a local stub that returns
// 404 for /cards/named so the lookup path resolves to ErrNotFound.
func TestLookupCard_NotFoundIsNotCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 404 for all named lookups; collection endpoint
		// returns empty payload so the bulk path also resolves to "not
		// found" cleanly.
		if strings.Contains(r.URL.Path, "/cards/named") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"not_found":[]}`))
	}))
	defer srv.Close()
	defer SetScryfallBaseForTest(srv.URL)()
	defer SetScryfallLimitIntervalForTest(time.Millisecond)()

	cache := NewResponseCache(100, time.Hour)
	h := &Handler{
		DB:      newOracleTestDB(t),
		Limiter: NewInboundRateLimiter(1000, 100),
		Cache:   cache,
	}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/oracle/card/asdfasdf-not-a-real-card", nil)
	req.RemoteAddr = "1.1.1.1:1"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%q)", w.Code, w.Body.String())
	}
	if cache.Len() != 0 {
		t.Fatalf("NotFound responses must not be cached, len=%d", cache.Len())
	}
}
