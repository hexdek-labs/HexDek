package hexapi

import (
	"encoding/json"
	"testing"
)

// TestReconcileStaleBans_UnbannedCardFlipsLegal pins the fix for the
// stale-served-legality bug (7174n1c: Sin/Spira's Punishment reading ILLEGAL
// for months on Gifts Ungiven, unbanned 2024-09).
func TestReconcileStaleBans_UnbannedCardFlipsLegal(t *testing.T) {
	raw := []byte(`{
		"legality": {
			"valid": false,
			"card_count": {"valid": true},
			"color_identity": {"valid": true},
			"singleton": {"valid": true},
			"banned_cards": {"valid": false, "banned_found": ["Gifts Ungiven"]},
			"commander": {"valid": true},
			"errors": ["Gifts Ungiven is banned in Commander"]
		}
	}`)
	out := reconcileStaleBans(raw)
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	leg := doc["legality"].(map[string]any)
	if leg["valid"] != true {
		t.Errorf("expected legality.valid=true after unbanning Gifts Ungiven, got %v", leg["valid"])
	}
	bc := leg["banned_cards"].(map[string]any)
	if bc["valid"] != true {
		t.Errorf("expected banned_cards.valid=true, got %v", bc["valid"])
	}
	if _, present := bc["banned_found"]; present {
		t.Errorf("expected banned_found dropped, still present: %v", bc["banned_found"])
	}
	if errs, ok := leg["errors"].([]any); ok && len(errs) != 0 {
		t.Errorf("expected the Gifts Ungiven error pruned, got %v", errs)
	}
}

// A genuinely banned card must stay flagged (never un-flag a real ban).
func TestReconcileStaleBans_GenuineBanUntouched(t *testing.T) {
	raw := []byte(`{"legality":{"valid":false,"banned_cards":{"valid":false,"banned_found":["Black Lotus"]},"card_count":{"valid":true},"color_identity":{"valid":true},"singleton":{"valid":true},"commander":{"valid":true}}}`)
	out := reconcileStaleBans(raw)
	var doc map[string]any
	json.Unmarshal(out, &doc)
	leg := doc["legality"].(map[string]any)
	if leg["valid"] == true {
		t.Errorf("a still-banned card must keep the deck illegal")
	}
}

// A deck illegal for a NON-banned reason must stay illegal even if it also had
// a stale ban (only the banned dimension is reconciled).
func TestReconcileStaleBans_OtherViolationKeepsIllegal(t *testing.T) {
	raw := []byte(`{"legality":{"valid":false,"banned_cards":{"valid":false,"banned_found":["Gifts Ungiven"]},"card_count":{"valid":false},"color_identity":{"valid":true},"singleton":{"valid":true},"commander":{"valid":true}}}`)
	out := reconcileStaleBans(raw)
	var doc map[string]any
	json.Unmarshal(out, &doc)
	leg := doc["legality"].(map[string]any)
	if leg["valid"] == true {
		t.Errorf("card_count invalid must keep the deck illegal after ban reconcile")
	}
}

// Malformed / ban-free inputs pass through unchanged.
func TestReconcileStaleBans_PassThrough(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`not json`),
		[]byte(`{"legality":{"valid":true,"banned_cards":{"valid":true}}}`),
		[]byte(`{}`),
	} {
		out := reconcileStaleBans(raw)
		if string(out) != string(raw) {
			t.Errorf("expected pass-through for %q, got %q", raw, out)
		}
	}
}

// TestReconcileStaleCardCount pins the stale-undercount heal (7174n1c: a deck
// analyzed before its list was completed froze "found 99, expected 100").
func TestReconcileStaleCardCount(t *testing.T) {
	base := `{"legality":{"valid":false,"card_count":{"valid":false,"expected":100,"actual":99,"message":"expected 100 cards, found 99"},"color_identity":{"valid":true},"singleton":{"valid":true},"banned_cards":{"valid":true},"commander":{"valid":true},"errors":["expected 100 cards, found 99"]}}`

	// live deck is now 100 -> heal to valid
	out := reconcileStaleCardCount([]byte(base), 100)
	var d map[string]any
	json.Unmarshal(out, &d)
	leg := d["legality"].(map[string]any)
	if leg["valid"] != true {
		t.Errorf("expected overall valid=true after count heal, got %v", leg["valid"])
	}
	cc := leg["card_count"].(map[string]any)
	if cc["valid"] != true || cc["actual"].(float64) != 100 {
		t.Errorf("expected card_count healed to 100/valid, got %v", cc)
	}
	if errs, ok := leg["errors"].([]any); ok && len(errs) != 0 {
		t.Errorf("expected stale count error pruned, got %v", errs)
	}

	// live deck still 99 -> do NOT heal (genuinely short stays flagged)
	out2 := reconcileStaleCardCount([]byte(base), 99)
	if string(out2) != base {
		t.Errorf("a genuinely-99 deck must stay flagged (no heal)")
	}

	// never flip a passing count, and never manufacture a fail:
	// actualTotal 0 (unreadable) -> pass-through
	if string(reconcileStaleCardCount([]byte(base), 0)) != base {
		t.Errorf("unreadable total must pass through unchanged")
	}
	// a deck the checker passed must be untouched even if live total differs
	passing := `{"legality":{"valid":true,"card_count":{"valid":true,"expected":100,"actual":100}}}`
	if string(reconcileStaleCardCount([]byte(passing), 42)) != passing {
		t.Errorf("a passing count must never be flipped to invalid")
	}
}
