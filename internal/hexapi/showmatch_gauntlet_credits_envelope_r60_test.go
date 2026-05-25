package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/credits"
)

// r60 round 2 — pins the gauntlet credits gate's 402 responses to
// the unified ErrorResponse shape (originally PR #93, evolved by the
// r60 error-shape sweep). Pre-r60, handleStartGauntlet hand-rolled a
// JSON envelope without the `status` field and without
// X-Content-Type-Options: nosniff. The r60 sweep further moved the
// extras (`balance` / `needed` / `quota`) from top-level fields into
// the nested `error.details` map so the envelope shape stays uniform
// regardless of which fields a handler ships.

// TestStartGauntlet_FreeQuotaExhausted_UsesUnifiedEnvelope drives the
// handler into the "out of free runs, balance too low to upgrade"
// branch and asserts (1) the unified Error.Code + Error.Message +
// Status decode succeeds, (2) the `balance`, `needed`, and `quota`
// extras are nested under `error.details` for the frontend's
// earn/wait prompt, and (3) the nosniff header is set so middleware
// proxies can't sniff the body into HTML.
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

	// Unified envelope fields must populate — this is what frontend
	// decoders bind to.
	var base ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &base); err != nil {
		t.Fatalf("ErrorResponse decode: %v (raw=%q)", err, rr.Body.String())
	}
	if base.Error.Message != "free_quota_exhausted" {
		t.Errorf("body.error.message: want %q, got %q", "free_quota_exhausted", base.Error.Message)
	}
	if base.Status != http.StatusPaymentRequired {
		t.Errorf("body.status: want 402, got %d", base.Status)
	}

	// Extras live nested under error.details so the envelope shape
	// stays stable. The React "earn or wait" prompt reads the same
	// three fields, now via body.error.details.{balance,needed,quota}.
	details, ok := base.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("body.error.details: want map[string]any, got %T (%v)", base.Error.Details, base.Error.Details)
	}
	if _, ok := details["balance"]; !ok {
		t.Errorf("balance missing from error.details: %+v", details)
	}
	if got, _ := details["needed"].(float64); int64(got) != credits.CreditsPerGauntlet {
		t.Errorf("needed: want %d, got %v", credits.CreditsPerGauntlet, details["needed"])
	}
	if _, ok := details["quota"]; !ok {
		t.Errorf("quota missing from error.details: %+v", details)
	}

	// Sanity: the failure path must not have debited credits (already
	// covered by sibling tests, but a one-liner re-pin here keeps the
	// envelope-shape test honest about not silently changing
	// behaviour).
	if got := f.currentBalance(t); got != 0 {
		t.Errorf("balance changed despite 402: want 0, got %d", got)
	}
}
