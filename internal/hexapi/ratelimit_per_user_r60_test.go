package hexapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ratelimit_per_user_r60_test.go — pins the per-user-account variant
// (enforceRateLimitByOwner + ownerOrIPKey + DeckMutationLimiter
// wiring). Per-IP keying is wrong for owner-authenticated endpoints:
// household IP sharing → cross-throttle; single user on phone +
// laptop → split budget. Per-owner keying follows the human.

// --- ownerOrIPKey unit tests ---------------------------------------------

func TestOwnerOrIPKey_HeaderPresentReturnsOwnerNamespace(t *testing.T) {
	r := httptest.NewRequest("DELETE", "/x", nil)
	r.Header.Set("X-HexDek-Owner", "Alice")
	if got := ownerOrIPKey(r); got != "owner:alice" {
		t.Errorf("owner key = %q, want %q (lowercased + namespaced)", got, "owner:alice")
	}
}

func TestOwnerOrIPKey_HeaderTrimmedAndLowered(t *testing.T) {
	r := httptest.NewRequest("DELETE", "/x", nil)
	r.Header.Set("X-HexDek-Owner", "  Bob  ")
	if got := ownerOrIPKey(r); got != "owner:bob" {
		t.Errorf("trimmed owner key = %q, want %q", got, "owner:bob")
	}
}

func TestOwnerOrIPKey_BlankHeaderFallsBackToIP(t *testing.T) {
	r := httptest.NewRequest("DELETE", "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	r.Header.Set("X-HexDek-Owner", "   ") // whitespace-only counts as blank
	if got := ownerOrIPKey(r); got != "ip:203.0.113.7" {
		t.Errorf("blank-owner fallback = %q, want %q", got, "ip:203.0.113.7")
	}
}

func TestOwnerOrIPKey_MissingHeaderFallsBackToIP(t *testing.T) {
	r := httptest.NewRequest("DELETE", "/x", nil)
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	if got := ownerOrIPKey(r); got != "ip:198.51.100.9" {
		t.Errorf("missing-owner fallback = %q, want %q", got, "ip:198.51.100.9")
	}
}

func TestOwnerOrIPKey_NamespacePreventsCrossPoolCollision(t *testing.T) {
	// Defense-in-depth: a request from IP literal "alice" and one
	// with owner header "alice" must NOT share a bucket. The "owner:"
	// / "ip:" prefixes guarantee disjoint keyspaces regardless of how
	// implausible the literal collision is.
	r1 := httptest.NewRequest("DELETE", "/x", nil)
	r1.Header.Set("X-HexDek-Owner", "alice")
	r2 := httptest.NewRequest("DELETE", "/x", nil)
	r2.Header.Set("X-Forwarded-For", "alice")
	if ownerOrIPKey(r1) == ownerOrIPKey(r2) {
		t.Errorf("namespace collision: owner-alice key %q == ip-alice key %q",
			ownerOrIPKey(r1), ownerOrIPKey(r2))
	}
}

// --- enforceRateLimitByOwner helper unit tests ---------------------------

func TestEnforceRateLimitByOwner_NilStorePassthrough(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/x", nil)
	if blocked := enforceRateLimitByOwner(nil, w, r, "deck update"); blocked {
		t.Fatal("nil limiter must not block")
	}
}

func TestEnforceRateLimitByOwner_KeyedByOwner(t *testing.T) {
	rl := NewRateLimiter(1, 1.0/60.0) // 1-token burst
	// Alice burns her budget.
	wa := httptest.NewRecorder()
	ra := httptest.NewRequest("DELETE", "/x", nil)
	ra.Header.Set("X-HexDek-Owner", "alice")
	ra.Header.Set("X-Forwarded-For", "10.0.0.50") // shared household IP
	if blocked := enforceRateLimitByOwner(rl, wa, ra, "deck update"); blocked {
		t.Fatal("alice's first call should pass")
	}
	wa = httptest.NewRecorder()
	if blocked := enforceRateLimitByOwner(rl, wa, ra, "deck update"); !blocked {
		t.Fatal("alice's second call should 429")
	}
	// Bob on the SAME IP must have a fresh budget — proves per-user keying.
	wb := httptest.NewRecorder()
	rb := httptest.NewRequest("DELETE", "/x", nil)
	rb.Header.Set("X-HexDek-Owner", "bob")
	rb.Header.Set("X-Forwarded-For", "10.0.0.50") // same IP
	if blocked := enforceRateLimitByOwner(rl, wb, rb, "deck update"); blocked {
		t.Fatal("bob on shared IP MUST get an independent per-user budget, but was rate-limited")
	}
}

func TestEnforceRateLimitByOwner_SameOwnerAcrossIPs(t *testing.T) {
	// Inverse of the household-sharing case: a single user moving
	// between phone (4G) and laptop (wifi) shows two different IPs
	// but ONE owner. The budget must follow the owner.
	rl := NewRateLimiter(1, 1.0/60.0)

	r1 := httptest.NewRequest("DELETE", "/x", nil)
	r1.Header.Set("X-HexDek-Owner", "alice")
	r1.Header.Set("X-Forwarded-For", "10.0.0.1") // wifi
	if blocked := enforceRateLimitByOwner(rl, httptest.NewRecorder(), r1, "deck update"); blocked {
		t.Fatal("first call should pass")
	}

	r2 := httptest.NewRequest("DELETE", "/x", nil)
	r2.Header.Set("X-HexDek-Owner", "alice")
	r2.Header.Set("X-Forwarded-For", "100.64.0.7") // 4G
	w2 := httptest.NewRecorder()
	if blocked := enforceRateLimitByOwner(rl, w2, r2, "deck update"); !blocked {
		t.Fatal("same owner from different IP MUST share the budget; was not rate-limited")
	}
}

func TestEnforceRateLimitByOwner_NoHeaderFallsBackToIPKeying(t *testing.T) {
	// A missing X-HexDek-Owner must NOT yield an unlimited rate.
	// Verifier falls back to per-IP bucketing — same key two requests
	// from the same IP drain the bucket.
	rl := NewRateLimiter(1, 1.0/60.0)
	r := httptest.NewRequest("DELETE", "/x", nil)
	r.Header.Set("X-Forwarded-For", "198.51.100.77")
	// No owner header.
	if blocked := enforceRateLimitByOwner(rl, httptest.NewRecorder(), r, "deck update"); blocked {
		t.Fatal("first call should pass")
	}
	if blocked := enforceRateLimitByOwner(rl, httptest.NewRecorder(), r, "deck update"); !blocked {
		t.Fatal("anonymous attacker without owner header must STILL be IP-bucketed; was unbounded")
	}
}

func TestEnforceRateLimitByOwner_OverLimitWritesLabel(t *testing.T) {
	rl := NewRateLimiter(1, 1.0/60.0)
	r := httptest.NewRequest("DELETE", "/x", nil)
	r.Header.Set("X-HexDek-Owner", "alice")
	_ = enforceRateLimitByOwner(rl, httptest.NewRecorder(), r, "deck delete") // drain
	w := httptest.NewRecorder()
	if !enforceRateLimitByOwner(rl, w, r, "deck delete") {
		t.Fatal("second call should 429")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After")
	}
	if !strings.Contains(w.Body.String(), "deck delete") {
		t.Errorf("body should mention the label, got %s", w.Body.String())
	}
}

// --- enforceRateLimit (per-IP) regression: untouched -----------------------

func TestEnforceRateLimit_StillKeysByIPAfterRefactor(t *testing.T) {
	// The refactor that introduced enforceRateLimitByKey must NOT
	// change the existing per-IP semantics for the old call shape.
	rl := NewRateLimiter(1, 1.0/60.0)
	r1 := httptest.NewRequest("POST", "/x", nil)
	r1.Header.Set("X-Forwarded-For", "10.0.0.99")
	if enforceRateLimit(rl, httptest.NewRecorder(), r1, "x") {
		t.Fatal("first call from 10.0.0.99 should pass")
	}
	if !enforceRateLimit(rl, httptest.NewRecorder(), r1, "x") {
		t.Fatal("second call from 10.0.0.99 should 429 — IP keying intact")
	}
	r2 := httptest.NewRequest("POST", "/x", nil)
	r2.Header.Set("X-Forwarded-For", "10.0.0.100")
	if enforceRateLimit(rl, httptest.NewRecorder(), r2, "x") {
		t.Fatal("different IP must have an independent bucket")
	}
}

// --- Integration: DeckMutationLimiter on PUT/PATCH/DELETE ----------------

func newDeckMutationTestHandler(t *testing.T, capacity float64) (*Handler, string) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "hexapi-rlpu-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(filepath.Join(decksDir, "alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(decksDir, "bob"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Plant deck files so PUT/PATCH/DELETE don't 404 before reaching
	// the body of the handler.
	for _, owner := range []string{"alice", "bob"} {
		path := filepath.Join(decksDir, owner, "korvold.txt")
		if err := os.WriteFile(path, []byte("COMMANDER: Korvold\n1 Sol Ring\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := &Handler{
		DecksDir:            decksDir,
		DeckMutationLimiter: NewRateLimiter(capacity, 1.0/60.0),
	}
	return h, decksDir
}

func deleteDeckRequest(owner, id, callerOwner, ip string) *http.Request {
	r := httptest.NewRequest("DELETE", "/api/decks/"+owner+"/"+id, nil)
	r.SetPathValue("owner", owner)
	r.SetPathValue("id", id)
	r.Header.Set("X-HexDek-Owner", callerOwner)
	r.Header.Set("X-Forwarded-For", ip)
	return r
}

func putDeckRequest(owner, id, callerOwner, ip string) *http.Request {
	body, _ := json.Marshal(map[string]string{"deck_list": "COMMANDER: Korvold\n1 Sol Ring\n"})
	r := httptest.NewRequest("PUT", "/api/decks/"+owner+"/"+id, bytes.NewReader(body))
	r.SetPathValue("owner", owner)
	r.SetPathValue("id", id)
	r.Header.Set("X-HexDek-Owner", callerOwner)
	r.Header.Set("X-Forwarded-For", ip)
	return r
}

func TestIntegration_DeleteDeck_PerUserBudgetIsolated(t *testing.T) {
	// Pin the headline behavior: alice and bob on the same IP have
	// independent budgets when the endpoint uses the per-user limiter.
	h, _ := newDeckMutationTestHandler(t, 1)

	// Alice drains her 1-token budget on her own deck.
	w := httptest.NewRecorder()
	h.handleDeleteDeck(w, deleteDeckRequest("alice", "korvold", "alice", "10.0.0.5"))
	if w.Code != http.StatusOK {
		t.Fatalf("alice first delete: status=%d body=%s", w.Code, w.Body.String())
	}
	// Re-plant the file (alice's first call actually deleted it) so
	// her second call's 429 doesn't get masked by a 404 — we need the
	// rate-limit gate to fire, not the file-not-found gate.
	_ = os.WriteFile(filepath.Join(h.DecksDir, "alice", "korvold.txt"), []byte("x"), 0o644)
	w = httptest.NewRecorder()
	h.handleDeleteDeck(w, deleteDeckRequest("alice", "korvold", "alice", "10.0.0.5"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("alice second delete: status=%d, want 429 (per-user budget drained)", w.Code)
	}

	// Bob on the SAME IP must still succeed — independent per-user bucket.
	w = httptest.NewRecorder()
	h.handleDeleteDeck(w, deleteDeckRequest("bob", "korvold", "bob", "10.0.0.5"))
	if w.Code != http.StatusOK {
		t.Fatalf("bob on shared IP: status=%d, want 200 (independent per-user budget); body=%s",
			w.Code, w.Body.String())
	}
}

func TestIntegration_DeleteDeck_NilLimiterPassesThrough(t *testing.T) {
	tmp, err := os.MkdirTemp("", "hexapi-rlpu-nil-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	decksDir := filepath.Join(tmp, "decks", "alice")
	_ = os.MkdirAll(decksDir, 0o755)
	_ = os.WriteFile(filepath.Join(decksDir, "korvold.txt"), []byte("x"), 0o644)
	h := &Handler{DecksDir: filepath.Join(tmp, "decks")} // DeckMutationLimiter nil
	w := httptest.NewRecorder()
	h.handleDeleteDeck(w, deleteDeckRequest("alice", "korvold", "alice", "1.2.3.4"))
	if w.Code != http.StatusOK {
		t.Fatalf("nil limiter must not block: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestIntegration_PutDeck_SharesBudgetWithDelete(t *testing.T) {
	// PUT + DELETE share the SAME limiter — proves the "one human's
	// edit session sums against one bucket" goal in the
	// DeckMutationLimiter docstring. (PATCH joins the same bucket
	// per its handler edit, but exercising PATCH end-to-end requires
	// deck_meta SQLite setup that's out of scope for this fixture;
	// the patchDeck call-site is unit-covered by inspection of
	// deckmeta.go's enforceRateLimitByOwner call.)
	h, _ := newDeckMutationTestHandler(t, 1) // 1-token burst

	// Alice burns her 1-token via PUT.
	w := httptest.NewRecorder()
	h.handleUpdateDeck(w, putDeckRequest("alice", "korvold", "alice", "10.0.0.5"))
	if w.Code != http.StatusOK {
		t.Fatalf("alice PUT: status=%d body=%s", w.Code, w.Body.String())
	}
	// DELETE from alice should now 429 — shared budget exhausted by
	// the upstream PUT. If PUT and DELETE used separate limiters this
	// would succeed.
	w = httptest.NewRecorder()
	h.handleDeleteDeck(w, deleteDeckRequest("alice", "korvold", "alice", "10.0.0.5"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("alice DELETE after PUT: status=%d, want 429 (PUT/DELETE share one bucket); body=%s",
			w.Code, w.Body.String())
	}
}

func TestIntegration_DeleteDeck_LimiterRunsBeforeOwnershipCheck(t *testing.T) {
	// Ordering pin: the rate-limit gate must fire BEFORE the
	// ownership check. An attacker sending mismatched-owner DELETEs
	// shouldn't get an effectively-unlimited rate of 403s (which can
	// be used to enumerate which owner/deck pairs exist via timing).
	h, _ := newDeckMutationTestHandler(t, 1)
	// Drain budget with one valid call (alice deletes her own).
	w := httptest.NewRecorder()
	h.handleDeleteDeck(w, deleteDeckRequest("alice", "korvold", "alice", "10.0.0.5"))
	if w.Code != http.StatusOK {
		t.Fatalf("setup: alice delete failed: %d %s", w.Code, w.Body.String())
	}
	// Second call from alice attacking bob's deck: would be 403 if
	// ownership ran first; must be 429 because limiter is in front.
	w = httptest.NewRecorder()
	h.handleDeleteDeck(w, deleteDeckRequest("bob", "korvold", "alice", "10.0.0.5"))
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("limiter must run before ownership check: status=%d, want 429 (got %s)",
			w.Code, w.Body.String())
	}
}
