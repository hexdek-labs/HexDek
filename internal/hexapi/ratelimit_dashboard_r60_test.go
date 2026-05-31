package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ratelimit_dashboard_r60_test.go — pins GET /api/admin/ratelimit:
//   - admin-only gate (HEXDEK_ADMIN_OWNER + matching X-HexDek-Owner)
//   - 503 when dashboard is nil (explicit "not configured" signal)
//   - response shape: limiters[] sorted by name, each with snapshot
//     {capacity, refill_per_sec, denials, bucket_count, buckets[]}
//   - denial counter ticks on each over-limit Allow call
//   - bucket snapshot reflects current token state per registered key
//   - nil-limiter Register is a silent no-op (no zero-named row)
//   - maxDashboardBuckets cap trims output deterministically
//   - SnapshotState on nil receiver returns zero value (no panic)

// --- Auth gate -----------------------------------------------------------

func TestRateLimitDashboard_403WhenNotAdmin(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "ops-admin")
	d := &RateLimitDashboard{}
	h := HandleRateLimitDashboard(d)

	cases := []struct {
		name   string
		setup  func(*http.Request)
	}{
		{"no owner header", func(r *http.Request) {}},
		{"wrong owner", func(r *http.Request) {
			r.Header.Set("X-HexDek-Owner", "imposter")
		}},
		{"empty owner header", func(r *http.Request) {
			r.Header.Set("X-HexDek-Owner", "")
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/ratelimit", nil)
			tc.setup(req)
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d, want 403 (body=%q)", rec.Code, rec.Body.String())
			}
			var body ErrorResponse
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, rec.Body.String())
			}
			if body.Error.Code != "forbidden" {
				t.Errorf("error.code=%q, want forbidden", body.Error.Code)
			}
		})
	}
}

// HEXDEK_ADMIN_OWNER unset = admin gate disabled entirely. Pin so an
// unset env doesn't accidentally promote anyone with an empty X-HexDek-
// Owner header (the empty == empty trap).
func TestRateLimitDashboard_EmptyAdminEnvRejectsEverything(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "")
	d := &RateLimitDashboard{}
	h := HandleRateLimitDashboard(d)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/ratelimit", nil)
	req.Header.Set("X-HexDek-Owner", "") // empty header + empty env
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 (empty env must NOT match empty header)", rec.Code)
	}
}

// --- Happy path ---------------------------------------------------------

// adminReq returns a request with the canonical admin headers set.
func adminReq(t *testing.T) *http.Request {
	t.Helper()
	t.Setenv("HEXDEK_ADMIN_OWNER", "ops-admin")
	req := httptest.NewRequest(http.MethodGet, "/api/admin/ratelimit", nil)
	req.Header.Set("X-HexDek-Owner", "ops-admin")
	return req
}

func TestRateLimitDashboard_ResponseShape(t *testing.T) {
	d := &RateLimitDashboard{}
	limA := NewRateLimiter(5, 1.0/60.0)
	limB := NewRateLimiter(10, 1.0/30.0)
	// Drive some traffic so the snapshot has interesting state.
	_, _ = limA.Allow("203.0.113.7")
	_, _ = limA.Allow("198.51.100.9")
	_, _ = limB.Allow("10.0.0.1")
	d.Register("deck import", limA)
	d.Register("gauntlet start", limB)

	h := HandleRateLimitDashboard(d)
	req := adminReq(t)
	req.Header.Set("X-HexDek-Owner", "ops-admin")
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (body=%q)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type=%q, want JSON", ct)
	}

	var resp struct {
		Limiters       []LimiterEntry `json:"limiters"`
		CapturedAtUnix int64          `json:"captured_at_unix"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (body=%q)", err, rec.Body.String())
	}

	if len(resp.Limiters) != 2 {
		t.Fatalf("limiters count=%d, want 2", len(resp.Limiters))
	}
	// Alphabetical name ordering (deck import before gauntlet start).
	if resp.Limiters[0].Name != "deck import" {
		t.Errorf("limiters[0].Name=%q, want deck import (alphabetical)", resp.Limiters[0].Name)
	}
	if resp.Limiters[1].Name != "gauntlet start" {
		t.Errorf("limiters[1].Name=%q, want gauntlet start", resp.Limiters[1].Name)
	}

	// Snapshot fields populated for limA.
	snapA := resp.Limiters[0].Snapshot
	if snapA.Capacity != 5 {
		t.Errorf("limA capacity=%v, want 5", snapA.Capacity)
	}
	if snapA.BucketCount != 2 {
		t.Errorf("limA bucket_count=%d, want 2 (two distinct IPs allowed)", snapA.BucketCount)
	}
	if len(snapA.Buckets) != 2 {
		t.Errorf("limA buckets returned=%d, want 2", len(snapA.Buckets))
	}
	// Captured timestamp is present and roughly current.
	if resp.CapturedAtUnix == 0 {
		t.Error("captured_at_unix missing")
	}
}

// Denial counter must tick on each over-limit Allow. Pin so a refactor
// that drops the rl.denials++ inside Allow's over-budget branch is
// caught at review.
func TestRateLimitDashboard_DenialsCounterTicksOnOverLimit(t *testing.T) {
	d := &RateLimitDashboard{}
	lim := NewRateLimiter(2, 1.0/60.0) // 2-burst
	d.Register("test", lim)

	// 2 allowed.
	for i := 0; i < 2; i++ {
		if ok, _ := lim.Allow("ip-1"); !ok {
			t.Fatalf("burst %d should pass", i+1)
		}
	}
	// 3 denied.
	for i := 0; i < 3; i++ {
		if ok, _ := lim.Allow("ip-1"); ok {
			t.Fatalf("over-limit %d must deny", i+1)
		}
	}

	h := HandleRateLimitDashboard(d)
	req := adminReq(t)
	rec := httptest.NewRecorder()
	h(rec, req)

	var resp struct {
		Limiters []LimiterEntry `json:"limiters"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Limiters) != 1 {
		t.Fatalf("limiters count=%d, want 1", len(resp.Limiters))
	}
	if got := resp.Limiters[0].Snapshot.Denials; got != 3 {
		t.Errorf("denials=%d, want 3 (one per over-limit Allow)", got)
	}
}

// Successful Allow calls do NOT bump denials. Defends against the
// inverse regression (counter ticks on every call instead of only
// on over-limit).
func TestRateLimitDashboard_DenialsCounterStaysAtZeroOnSuccess(t *testing.T) {
	d := &RateLimitDashboard{}
	lim := NewRateLimiter(100, 100.0) // huge — never denies
	d.Register("test", lim)
	for i := 0; i < 50; i++ {
		if ok, _ := lim.Allow("ip"); !ok {
			t.Fatal("setup: must not deny")
		}
	}
	h := HandleRateLimitDashboard(d)
	rec := httptest.NewRecorder()
	h(rec, adminReq(t))
	var resp struct {
		Limiters []LimiterEntry `json:"limiters"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Limiters[0].Snapshot.Denials != 0 {
		t.Errorf("denials=%d, want 0 (success path must not tick counter)", resp.Limiters[0].Snapshot.Denials)
	}
}

// Buckets carry per-key token state — pin that an Allow on key X
// surfaces in the snapshot's buckets entry for X.
func TestRateLimitDashboard_BucketReflectsTokenState(t *testing.T) {
	d := &RateLimitDashboard{}
	lim := NewRateLimiter(5, 1.0/60.0)
	d.Register("test", lim)
	// Burn 3 of the 5 tokens for alice.
	for i := 0; i < 3; i++ {
		_, _ = lim.Allow("ip:alice")
	}
	h := HandleRateLimitDashboard(d)
	rec := httptest.NewRecorder()
	h(rec, adminReq(t))

	var resp struct {
		Limiters []LimiterEntry `json:"limiters"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Limiters[0].Snapshot.Buckets) != 1 {
		t.Fatalf("want 1 bucket, got %d", len(resp.Limiters[0].Snapshot.Buckets))
	}
	b := resp.Limiters[0].Snapshot.Buckets[0]
	if b.Key != "ip:alice" {
		t.Errorf("bucket key=%q, want ip:alice", b.Key)
	}
	// After 3 of 5 burned: tokens ~= 2 (refill is negligible over <1s test).
	if b.Tokens < 1.5 || b.Tokens > 2.5 {
		t.Errorf("bucket tokens=%v, want ~2 (5-burst minus 3 Allows)", b.Tokens)
	}
}

// --- Register edge cases ------------------------------------------------

// nil limiter is silently dropped — Register can be called
// unconditionally with optional fields without producing zero rows.
func TestRateLimitDashboard_RegisterNilIsSilent(t *testing.T) {
	d := &RateLimitDashboard{}
	d.Register("not-wired", nil)
	d.Register("real", NewRateLimiter(1, 1.0))

	h := HandleRateLimitDashboard(d)
	rec := httptest.NewRecorder()
	h(rec, adminReq(t))

	var resp struct {
		Limiters []LimiterEntry `json:"limiters"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Limiters) != 1 {
		t.Fatalf("nil limiter should not register; got %d rows: %+v", len(resp.Limiters), resp.Limiters)
	}
	if resp.Limiters[0].Name != "real" {
		t.Errorf("got %q, want real", resp.Limiters[0].Name)
	}
}

// --- maxDashboardBuckets cap --------------------------------------------

// SnapshotState trims to maxBuckets keys (the lexicographically-first
// ones) so a limiter with thousands of unique IPs doesn't blow up the
// response.
func TestSnapshotState_TrimsToMaxBuckets(t *testing.T) {
	lim := NewRateLimiter(1000, 100.0)
	// 50 unique keys.
	for i := 0; i < 50; i++ {
		key := "ip:" + string(rune('a'+i%26)) + string(rune('a'+(i/26)))
		_, _ = lim.Allow(key)
	}
	snap := lim.SnapshotState(10)
	if snap.BucketCount != 50 {
		t.Errorf("BucketCount=%d, want 50 (full count even when trimmed)", snap.BucketCount)
	}
	if len(snap.Buckets) != 10 {
		t.Errorf("len(Buckets)=%d, want 10 (trimmed to max)", len(snap.Buckets))
	}
	// Trim window is alphabetically first — first key should sort
	// before the last returned key.
	for i := 1; i < len(snap.Buckets); i++ {
		if snap.Buckets[i-1].Key >= snap.Buckets[i].Key {
			t.Errorf("buckets not sorted: %q before %q", snap.Buckets[i-1].Key, snap.Buckets[i].Key)
		}
	}
}

// SnapshotState(-1) and SnapshotState(0) both mean "no trim" — pin
// so a caller that doesn't care about the cap doesn't accidentally
// truncate to zero rows.
func TestSnapshotState_ZeroOrNegativeMeansNoTrim(t *testing.T) {
	lim := NewRateLimiter(100, 100.0)
	for i := 0; i < 25; i++ {
		_, _ = lim.Allow("k" + string(rune('a'+i)))
	}
	for _, max := range []int{0, -1, -100} {
		snap := lim.SnapshotState(max)
		if len(snap.Buckets) != 25 {
			t.Errorf("SnapshotState(%d) returned %d buckets, want 25 (no trim)",
				max, len(snap.Buckets))
		}
	}
}

// Nil receiver returns zero value cleanly (no panic). Defends a
// caller that builds a dashboard with a never-constructed limiter.
func TestSnapshotState_NilReceiverIsZero(t *testing.T) {
	var rl *RateLimiter
	snap := rl.SnapshotState(10)
	if snap.Capacity != 0 || snap.Denials != 0 || snap.BucketCount != 0 || len(snap.Buckets) != 0 {
		t.Errorf("nil receiver should return zero value, got %+v", snap)
	}
}

// --- nil dashboard 503 -------------------------------------------------

func TestRateLimitDashboard_503WhenDashboardNil(t *testing.T) {
	h := HandleRateLimitDashboard(nil)
	req := adminReq(t)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
	var body ErrorResponse
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Error.Code != "service_unavailable" {
		t.Errorf("error.code=%q, want service_unavailable", body.Error.Code)
	}
}

// --- End-to-end route registration --------------------------------------

func TestRegisterRateLimitDashboard_MountsRoute(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "ops-admin")
	mux := http.NewServeMux()
	d := &RateLimitDashboard{}
	d.Register("test", NewRateLimiter(1, 1.0))
	RegisterRateLimitDashboard(mux, d)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/ratelimit", nil)
	req.Header.Set("X-HexDek-Owner", "ops-admin")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (route not mounted?)", rec.Code)
	}
}
