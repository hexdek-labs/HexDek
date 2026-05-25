package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/credits"
)

// r60 round 2 — pin the gauntlet credits gate's 402 responses to the
// unified ErrorResponse shape (PR #93). Pre-fix, handleStartGauntlet
// hand-rolled a JSON envelope without the `status` field and without
// the `X-Content-Type-Options: nosniff` header that writeError
// guarantees, so frontends that bound the response into ErrorResponse
// saw `status: 0` even though the HTTP code was 402. The two 402
// paths now route through writeErrorWithDetails, which merges the
// extra `balance` / `needed` / `quota` fields on top of the unified
// `error` + `status` envelope.

// TestStartGauntlet_FreeQuotaExhausted_UsesUnifiedEnvelope drives the
// handler into the "out of free runs, balance too low to upgrade"
// branch and asserts (1) the unified Error + Status decode succeeds,
// (2) the `balance`, `needed`, and `quota` extras are still present
// at the top level for the frontend's earn/wait prompt, and (3) the
// nosniff header is set so middleware proxies can't sniff the body
// into HTML.
func TestStartGauntlet_FreeQuotaExhausted_UsesUnifiedEnvelope(t *testing.T) {
	f := newGauntletRefundFixture(t)
	// Exhaust the free quota for the day but leave the balance at 0
	// so CanRunPaid is false and the handler takes the
	// free_quota_exhausted branch.
	f.putUserInPaidTier(t, 0)
	f.addDeckToPool("alice", "korvold", "Korvold")

	req := f.makeStartReq("alice", "korvold")
	rr := httptest.NewRecorder()
	f.sm.handleStartGauntlet(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("status: want 402, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: want nosniff, got %q", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", got)
	}

	// Base ErrorResponse fields must populate — this is what frontend
	// decoders bind to.
	var base ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &base); err != nil {
		t.Fatalf("ErrorResponse decode: %v (raw=%q)", err, rr.Body.String())
	}
	if base.Error != "free_quota_exhausted" {
		t.Errorf("body.error: want %q, got %q", "free_quota_exhausted", base.Error)
	}
	if base.Status != http.StatusPaymentRequired {
		t.Errorf("body.status: want 402, got %d (PRE-FIX BUG: status field was missing)", base.Status)
	}

	// Extras must still be on the top level so the React "earn or
	// wait" prompt can render the actionable values.
	var full map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &full); err != nil {
		t.Fatalf("full body decode: %v", err)
	}
	if _, ok := full["balance"]; !ok {
		t.Errorf("balance missing from body: %+v", full)
	}
	if got, _ := full["needed"].(float64); int64(got) != credits.CreditsPerGauntlet {
		t.Errorf("needed: want %d, got %v", credits.CreditsPerGauntlet, full["needed"])
	}
	if _, ok := full["quota"]; !ok {
		t.Errorf("quota missing from body: %+v", full)
	}

	// Sanity: the failure path must not have debited credits (already
	// covered by sibling tests, but a one-liner re-pin here keeps the
	// envelope-shape test honest about not silently changing
	// behaviour).
	if got := f.currentBalance(t); got != 0 {
		t.Errorf("balance changed despite 402: want 0, got %d", got)
	}
}
