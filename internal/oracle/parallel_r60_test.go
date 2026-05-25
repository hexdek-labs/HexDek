package oracle

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
)

// parallel_r60_test.go — pins the leaky-bucket limiter + parallel
// LookupMany behavior. The point is to make a 99-card first-import
// "missing-all" deck not take serial-RTT wall time, while still
// respecting Scryfall's documented rate ceiling.

// ---------------------------------------------------------------------------
// Limiter unit tests
// ---------------------------------------------------------------------------

// Wait returns immediately on a fresh limiter.
func TestLimiter_FirstWaitImmediate(t *testing.T) {
	l := newScryfallLimiter(50 * time.Millisecond)
	start := time.Now()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if d := time.Since(start); d > 5*time.Millisecond {
		t.Errorf("first Wait should be immediate; took %v", d)
	}
}

// Two sequential Waits must space by at least the interval.
func TestLimiter_SequentialWaitsHonorInterval(t *testing.T) {
	l := newScryfallLimiter(50 * time.Millisecond)
	if err := l.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := l.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	d := time.Since(start)
	if d < 45*time.Millisecond {
		t.Errorf("second Wait should sleep ~50ms; took %v", d)
	}
	if d > 100*time.Millisecond {
		t.Errorf("second Wait took too long (%v) — bucket may be over-throttling", d)
	}
}

// Concurrent Waits get unique slots spaced by the interval. The
// individual gap measurements are noisy under -race + macOS scheduling,
// so we test the cumulative span instead: 5 staggered Waits must span
// at least ~3 intervals after slack (the leaky bucket reserves slots
// 0, I, 2I, 3I, 4I — span = 4I, slack-tolerant lower bound 3I).
func TestLimiter_ConcurrentWaitsStaggerStarts(t *testing.T) {
	interval := 40 * time.Millisecond
	l := newScryfallLimiter(interval)
	const n = 5
	starts := make([]time.Time, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Wait(context.Background()); err != nil {
				t.Errorf("Wait: %v", err)
				return
			}
			starts[i] = time.Now()
		}()
	}
	wg.Wait()

	// Sort by time to extract dispatch order regardless of goroutine
	// scheduling.
	times := make([]time.Time, n)
	copy(times, starts)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && times[j].Before(times[j-1]); j-- {
			times[j], times[j-1] = times[j-1], times[j]
		}
	}
	totalSpan := times[n-1].Sub(times[0])
	// 4 gaps × interval = expected. Allow 25% slack downward for
	// scheduling jitter compressing measured wakeups.
	minSpan := 3 * interval // tolerant lower bound (≈ 0.75 × ideal)
	if totalSpan < minSpan {
		t.Errorf("staggered Waits span = %v < %v (ideal 4 × %v); limiter may be returning slots too fast",
			totalSpan, minSpan, interval)
	}
	// Upper bound: if Wait() inadvertently serialized end-to-end at
	// (interval + handler_work) the span would explode. Test handler
	// is just a time.Now(), so span should stay close to ideal.
	if totalSpan > 8*interval {
		t.Errorf("staggered Waits span = %v; far above ideal 4 × %v (regression: serialized?)",
			totalSpan, interval)
	}
}

// Cancelled context aborts a queued Wait.
func TestLimiter_ContextCancelAbortsQueuedWait(t *testing.T) {
	l := newScryfallLimiter(500 * time.Millisecond)
	// First Wait reserves the bucket so the second will queue.
	if err := l.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := l.Wait(ctx)
	d := time.Since(start)
	if err == nil {
		t.Fatal("expected ctx.Err() return")
	}
	if d > 100*time.Millisecond {
		t.Errorf("Wait should abort fast on cancel; took %v", d)
	}
}

// ---------------------------------------------------------------------------
// LookupMany parallelism — wall time + concurrency bounds
// ---------------------------------------------------------------------------

// scryfallStub spins up an httptest server emulating /cards/collection
// and /cards/named, tracking concurrent in-flight requests and total
// hits. Each request sleeps `latency` to simulate real RTT so a
// regression to serial dispatch shows up as wall time.
type scryfallStub struct {
	server      *httptest.Server
	totalHits   int32
	inflight    int32
	maxInflight int32

	// notFoundForCollection: names the bulk endpoint should return as
	// not_found so they fall through to the fuzzy /cards/named path.
	notFoundForCollection map[string]bool
}

func newScryfallStub(t *testing.T, latency time.Duration, notFound map[string]bool) *scryfallStub {
	t.Helper()
	s := &scryfallStub{notFoundForCollection: notFound}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Track concurrency.
		current := atomic.AddInt32(&s.inflight, 1)
		defer atomic.AddInt32(&s.inflight, -1)
		for {
			max := atomic.LoadInt32(&s.maxInflight)
			if current <= max || atomic.CompareAndSwapInt32(&s.maxInflight, max, current) {
				break
			}
		}
		atomic.AddInt32(&s.totalHits, 1)

		time.Sleep(latency)

		switch {
		case strings.Contains(r.URL.Path, "/cards/collection"):
			var req struct {
				Identifiers []struct {
					Name string `json:"name"`
				} `json:"identifiers"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			data := []scryfallNamedResp{}
			notFoundList := []map[string]string{}
			for _, ident := range req.Identifiers {
				if s.notFoundForCollection[strings.ToLower(ident.Name)] {
					notFoundList = append(notFoundList, map[string]string{"name": ident.Name})
					continue
				}
				data = append(data, scryfallNamedResp{
					Object: "card", ID: ident.Name + "-id", Name: ident.Name,
					TypeLine: "Creature — Test",
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data":      data,
				"not_found": notFoundList,
			})
		case strings.Contains(r.URL.Path, "/cards/named"):
			name := r.URL.Query().Get("fuzzy")
			_ = json.NewEncoder(w).Encode(scryfallNamedResp{
				Object: "card", ID: name + "-id", Name: name,
				TypeLine: "Sorcery",
			})
		default:
			w.WriteHeader(404)
		}
	}))
	return s
}

func (s *scryfallStub) close() { s.server.Close() }

// newOracleTestDB opens a tmp SQLite DB with the oracle cache schema.
// Each test gets its own so cache writes from one test don't leak into
// another's count.
func newOracleTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmp := t.TempDir()
	d, err := db.Open(tmp + "/cache.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// All names hit the bulk endpoint (single chunk for 75 names). The
// limiter is irrelevant for a single chunk — test that the bulk
// endpoint runs.
func TestLookupMany_SingleBulkChunk(t *testing.T) {
	stub := newScryfallStub(t, 5*time.Millisecond, nil)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(10 * time.Millisecond)()

	names := []string{"A", "B", "C", "D"}
	out := LookupMany(context.Background(), newOracleTestDB(t), names)
	if len(out) < 4 {
		t.Errorf("expected ≥4 results, got %d (%v)", len(out), out)
	}
	if atomic.LoadInt32(&stub.totalHits) != 1 {
		t.Errorf("single chunk should be 1 HTTP hit, got %d", stub.totalHits)
	}
}

// Multi-chunk: 160 names → 3 chunks. Primary assertion is structural
// (maxInflight ≥ 2 proves overlapping dispatch); wall-time bound is
// generous because the -race goroutine scheduler adds 3-4× overhead
// on every channel op and makes tight timing assertions flaky.
func TestLookupMany_BulkChunksRunInParallel(t *testing.T) {
	latency := 80 * time.Millisecond
	stub := newScryfallStub(t, latency, nil)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(10 * time.Millisecond)()
	defer SetScryfallParallelismForTest(4)()

	names := make([]string, 160) // 3 chunks of 75/75/10
	for i := range names {
		names[i] = fmt.Sprintf("card-%03d", i)
	}

	out := LookupMany(context.Background(), newOracleTestDB(t), names)
	if len(out) < len(names) {
		t.Errorf("expected ≥%d results, got %d", len(names), len(out))
	}
	if atomic.LoadInt32(&stub.totalHits) != 3 {
		t.Errorf("expected 3 chunk HTTP hits, got %d", stub.totalHits)
	}
	// Structural: 3 chunks should have been in flight simultaneously.
	// This proves parallel dispatch directly and is unaffected by the
	// goroutine scheduler overhead that the -race detector adds. Wall-
	// time assertions are intentionally not used here — race vs no-race
	// stretches them enough to be flaky without strengthening the
	// correctness claim that maxInflight already pins.
	if peak := atomic.LoadInt32(&stub.maxInflight); peak < 3 {
		t.Errorf("expected peak in-flight = 3 (parallel chunks); got %d", peak)
	}
}

// Fuzzy fallback parallel: 12 names that bulk endpoint can't resolve →
// 12 serial /cards/named in the old code; should parallelize now.
func TestLookupMany_FuzzyFallbackParallel(t *testing.T) {
	latency := 50 * time.Millisecond
	// Make every card not_found in collection → all fall through to fuzzy.
	notFound := map[string]bool{}
	for i := 0; i < 12; i++ {
		notFound[fmt.Sprintf("fuzz-%02d", i)] = true
	}
	stub := newScryfallStub(t, latency, notFound)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(10 * time.Millisecond)()
	defer SetScryfallParallelismForTest(6)()

	names := make([]string, 12)
	for i := range names {
		names[i] = fmt.Sprintf("fuzz-%02d", i)
	}

	out := LookupMany(context.Background(), newOracleTestDB(t), names)
	if len(out) < len(names) {
		t.Errorf("expected ≥%d results, got %d", len(names), len(out))
	}
	// 1 collection call + 12 fuzzy calls = 13 total hits.
	if got := atomic.LoadInt32(&stub.totalHits); got != 13 {
		t.Errorf("expected 13 HTTP hits (1 bulk + 12 fuzzy), got %d", got)
	}
	// Structural: parallel fuzzy workers should overlap. With latency
	// 50ms and limiter interval 10ms, steady-state concurrent fuzzy
	// requests = latency/interval = 5; pin ≥ 3 to leave slack for
	// scheduling jitter at the start/end.
	if peak := atomic.LoadInt32(&stub.maxInflight); peak < 3 {
		t.Errorf("expected peak in-flight ≥ 3 (parallel fallback); got %d", peak)
	}
}

// The parallelism cap must hold: with parallelism=2, we never see >2
// concurrent in-flight even with 20 cards.
func TestLookupMany_ParallelismCapEnforced(t *testing.T) {
	notFound := map[string]bool{}
	for i := 0; i < 20; i++ {
		notFound[fmt.Sprintf("capped-%02d", i)] = true
	}
	stub := newScryfallStub(t, 20*time.Millisecond, notFound)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(5 * time.Millisecond)()
	defer SetScryfallParallelismForTest(2)()

	names := make([]string, 20)
	for i := range names {
		names[i] = fmt.Sprintf("capped-%02d", i)
	}
	LookupMany(context.Background(), newOracleTestDB(t), names)

	if peak := atomic.LoadInt32(&stub.maxInflight); peak > 2 {
		t.Errorf("parallelism cap = 2 but peak in-flight was %d", peak)
	}
}

// Limiter is respected end-to-end: with interval=50ms and 5 fallbacks,
// dispatch span ≥ 4 × 50ms = 200ms even with high parallelism.
func TestLookupMany_LimiterEnforcesDispatchInterval(t *testing.T) {
	notFound := map[string]bool{
		"a": true, "b": true, "c": true, "d": true, "e": true,
	}
	stub := newScryfallStub(t, 1*time.Millisecond, notFound)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(50 * time.Millisecond)()
	defer SetScryfallParallelismForTest(8)()

	start := time.Now()
	LookupMany(context.Background(), newOracleTestDB(t), []string{"a", "b", "c", "d", "e"})
	wall := time.Since(start)
	// 1 bulk call (50ms slot) + 5 fuzzy calls (5 × 50ms slots) = 300ms
	// minimum. Allow 50ms slack for goroutine scheduling.
	if wall < 250*time.Millisecond {
		t.Errorf("LookupMany finished in %v — limiter may not be enforcing 50ms interval", wall)
	}
}

// Duplicate names in the input must be deduped — concurrent fetches
// for the same name would race and waste budget.
func TestLookupMany_DuplicateNamesDeduped(t *testing.T) {
	stub := newScryfallStub(t, 5*time.Millisecond, nil)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(10 * time.Millisecond)()

	names := []string{"Island", "Island", "Island", "Forest", "Island"}
	LookupMany(context.Background(), newOracleTestDB(t), names)
	if got := atomic.LoadInt32(&stub.totalHits); got != 1 {
		t.Errorf("duplicate names: expected 1 bulk hit (2 unique), got %d", got)
	}
}

// All-cached input takes zero HTTP calls.
func TestLookupMany_AllCachedSkipsHTTP(t *testing.T) {
	stub := newScryfallStub(t, 5*time.Millisecond, nil)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()

	tmp := t.TempDir()
	d, err := db.Open(tmp + "/cache.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	if err := saveToCache(ctx, d, "lightning bolt", &Card{Name: "Lightning Bolt"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := saveToCache(ctx, d, "swamp", &Card{Name: "Swamp"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out := LookupMany(ctx, d, []string{"Lightning Bolt", "Swamp"})
	if len(out) != 2 {
		t.Errorf("expected 2 cached results, got %d", len(out))
	}
	if got := atomic.LoadInt32(&stub.totalHits); got != 0 {
		t.Errorf("all-cached: expected 0 HTTP hits, got %d", got)
	}
}

// End-to-end realistic shape: 99-card "first import" with mixed bulk-
// hit and fuzzy-fallback. Pin only structural correctness — wall time
// is too sensitive to scheduler overhead to assert reliably.
func TestLookupMany_RealisticImportShape(t *testing.T) {
	// Use latency >> limiter interval so multiple fuzzy fetches are
	// steady-state in flight at once (steady-state inflight ≈
	// latency/interval = 60/10 = 6).
	const latency = 60 * time.Millisecond
	notFound := map[string]bool{}
	for i := 0; i < 8; i++ {
		notFound[fmt.Sprintf("dfc-%02d", i)] = true
	}
	stub := newScryfallStub(t, latency, notFound)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(10 * time.Millisecond)()
	defer SetScryfallParallelismForTest(8)()

	names := make([]string, 0, 99)
	for i := 0; i < 91; i++ {
		names = append(names, fmt.Sprintf("card-%03d", i))
	}
	for i := 0; i < 8; i++ {
		names = append(names, fmt.Sprintf("dfc-%02d", i))
	}

	out := LookupMany(context.Background(), newOracleTestDB(t), names)
	if len(out) < 99 {
		t.Errorf("expected ≥99 results, got %d", len(out))
	}
	// Structural: with 8 fuzzy fallbacks at parallelism=8 and latency/
	// interval = 6, peak inflight should comfortably exceed 3.
	if peak := atomic.LoadInt32(&stub.maxInflight); peak < 3 {
		t.Errorf("99-card import: expected peak in-flight ≥ 3 (parallel chunks + fuzzy); got %d", peak)
	}
}
