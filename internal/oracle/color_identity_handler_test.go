package oracle

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
)

// TestLookupCard_ReturnsColorIdentity pins the fix for the empty
// color_identity defect on GET /api/oracle/card/{name}: the endpoint
// previously never modeled color identity at all (missing from the
// Card struct, the scryfallNamedResp, and the card_oracle table), so
// every response came back with no color_identity. This drives the
// registered route via httptest against a SQLite-cache-seeded card and
// asserts a multicolour card round-trips its real WUBRG identity.
func TestLookupCard_ReturnsColorIdentity(t *testing.T) {
	tmp := t.TempDir()
	d, err := db.Open(tmp + "/cache.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	ctx := context.Background()

	// Seed a known Temur (green/blue/red) card and a colorless control
	// directly into the SQLite cache so the handler resolves them via
	// getCached with no Scryfall network round-trip.
	temur := &Card{
		Name:          "Animar, Soul of Elements",
		ManaCost:      "{2}{U}{R}{G}",
		CMC:           5,
		TypeLine:      "Legendary Creature — Elemental",
		ColorIdentity: []string{"G", "R", "U"},
	}
	if err := saveToCache(ctx, d, "animar, soul of elements", temur); err != nil {
		t.Fatalf("seed temur: %v", err)
	}
	// Control: a genuinely colorless card must round-trip an empty
	// identity, so an empty result is a real finding — not a lookup miss
	// masquerading as "no colors."
	colorless := &Card{
		Name:          "Sol Ring",
		ManaCost:      "{1}",
		CMC:           1,
		TypeLine:      "Artifact",
		ColorIdentity: nil,
	}
	if err := saveToCache(ctx, d, "sol ring", colorless); err != nil {
		t.Fatalf("seed colorless: %v", err)
	}

	h := &Handler{DB: d}
	mux := http.NewServeMux()
	h.Register(mux)

	get := func(name string) map[string]any {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/oracle/card/"+url.PathEscape(name), nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %q: status %d, body %s", name, w.Code, w.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %q: %v (body %s)", name, err, w.Body.String())
		}
		return body
	}

	// Multicolour card: color_identity present and correct.
	body := get("Animar, Soul of Elements")
	raw, ok := body["color_identity"]
	if !ok {
		t.Fatalf("response missing color_identity field: %v", body)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("color_identity is not an array: %T %v", raw, raw)
	}
	got := make([]string, 0, len(arr))
	for _, v := range arr {
		got = append(got, v.(string))
	}
	want := map[string]bool{"G": true, "R": true, "U": true}
	if len(got) != len(want) {
		t.Fatalf("color_identity = %v, want the 3 Temur colors G/R/U", got)
	}
	for _, c := range got {
		if !want[c] {
			t.Fatalf("color_identity = %v, contains unexpected %q (want G/R/U only)", got, c)
		}
	}

	// Colorless control: empty identity is a real, distinct result.
	cbody := get("Sol Ring")
	if craw, ok := cbody["color_identity"]; ok && craw != nil {
		if carr, ok := craw.([]any); ok && len(carr) != 0 {
			t.Fatalf("colorless card color_identity = %v, want empty", carr)
		}
	}
}
