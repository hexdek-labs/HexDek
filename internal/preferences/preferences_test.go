package preferences

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/auth"
	"github.com/hexdek/hexdek/internal/db"
)

// openTestDB mirrors the in-memory pattern used in internal/auth tests.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := EnsureSchema(context.Background(), d); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return d
}

// seedDevice + issueSession together produce an Authorization header
// the auth middleware accepts. Mirrors the auth test helpers.
func seedDevice(t *testing.T, d *sql.DB, id string) string {
	t.Helper()
	now := db.Now()
	if _, err := d.ExecContext(context.Background(),
		`INSERT INTO device (id, display_name, created_at, last_seen_at) VALUES (?, ?, ?, ?)`,
		id, id, now, now); err != nil {
		t.Fatalf("seed device %q: %v", id, err)
	}
	return id
}

func issueSession(t *testing.T, d *sql.DB, deviceID string) string {
	t.Helper()
	s, err := auth.IssueSession(context.Background(), d, deviceID, 3600)
	if err != nil {
		t.Fatalf("IssueSession: %v", err)
	}
	return s.Token
}

// buildMux returns an *http.ServeMux with the preferences endpoints
// wired against the test database. Reused across handler tests.
func buildMux(t *testing.T, d *sql.DB) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux, d)
	return mux
}

// ============================================================================
// Storage layer
// ============================================================================

func TestGet_EmptyRowReturnsZeroValues(t *testing.T) {
	d := openTestDB(t)
	p, err := Get(context.Background(), d, "device_brand_new")
	if err != nil {
		t.Fatalf("Get on missing row: %v", err)
	}
	if p.DisplayName != "" || p.OwnerName != "" || p.UpdatedAt != 0 {
		t.Errorf("expected zero-value Preferences for missing row, got %+v", p)
	}
	if p.DeviceID != "device_brand_new" {
		t.Errorf("DeviceID should be threaded through, got %q", p.DeviceID)
	}
}

func TestPatch_InsertsRowOnFirstWrite(t *testing.T) {
	d := openTestDB(t)
	dn, on := "Acolyte", "josh"
	p, err := Patch(context.Background(), d, "device_a",
		PatchInput{DisplayName: &dn, OwnerName: &on})
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if p.DisplayName != "Acolyte" || p.OwnerName != "josh" {
		t.Errorf("post-insert: got %+v", p)
	}
	if p.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set on first write")
	}
	// Round-trip via Get.
	got, err := Get(context.Background(), d, "device_a")
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Acolyte" || got.OwnerName != "josh" {
		t.Errorf("Get after Patch: got %+v", got)
	}
}

func TestPatch_PartialUpdatePreservesOtherField(t *testing.T) {
	d := openTestDB(t)
	first := "Acolyte"
	owner := "josh"
	if _, err := Patch(context.Background(), d, "device_a",
		PatchInput{DisplayName: &first, OwnerName: &owner}); err != nil {
		t.Fatal(err)
	}
	// Patch ONLY display_name — owner_name should survive.
	second := "Mage"
	p, err := Patch(context.Background(), d, "device_a",
		PatchInput{DisplayName: &second})
	if err != nil {
		t.Fatalf("partial Patch: %v", err)
	}
	if p.DisplayName != "Mage" {
		t.Errorf("display_name update: got %q want Mage", p.DisplayName)
	}
	if p.OwnerName != "josh" {
		t.Errorf("owner_name should be preserved: got %q want josh", p.OwnerName)
	}
}

func TestPatch_EmptyStringClearsField(t *testing.T) {
	d := openTestDB(t)
	val := "Acolyte"
	owner := "josh"
	if _, err := Patch(context.Background(), d, "device_a",
		PatchInput{DisplayName: &val, OwnerName: &owner}); err != nil {
		t.Fatal(err)
	}
	empty := ""
	p, err := Patch(context.Background(), d, "device_a",
		PatchInput{DisplayName: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if p.DisplayName != "" {
		t.Errorf("empty-string patch should clear field, got %q", p.DisplayName)
	}
	if p.OwnerName != "josh" {
		t.Errorf("owner_name preserved across explicit-clear of display_name, got %q", p.OwnerName)
	}
}

func TestPatch_TrimsWhitespace(t *testing.T) {
	d := openTestDB(t)
	val := "  Acolyte  "
	owner := "\t josh\n"
	p, err := Patch(context.Background(), d, "device_a",
		PatchInput{DisplayName: &val, OwnerName: &owner})
	if err != nil {
		t.Fatal(err)
	}
	if p.DisplayName != "Acolyte" {
		t.Errorf("display_name should be trimmed, got %q", p.DisplayName)
	}
	if p.OwnerName != "josh" {
		t.Errorf("owner_name should be trimmed, got %q", p.OwnerName)
	}
}

func TestPatch_BothFieldsNilIsNoop(t *testing.T) {
	d := openTestDB(t)
	dn, on := "Acolyte", "josh"
	_, _ = Patch(context.Background(), d, "device_a",
		PatchInput{DisplayName: &dn, OwnerName: &on})
	before, _ := Get(context.Background(), d, "device_a")
	p, err := Patch(context.Background(), d, "device_a", PatchInput{})
	if err != nil {
		t.Fatal(err)
	}
	if p.DisplayName != before.DisplayName || p.OwnerName != before.OwnerName {
		t.Errorf("nil-fields Patch should be no-op; before=%+v after=%+v", before, p)
	}
	// Updated_at should NOT advance on a no-op.
	if p.UpdatedAt != before.UpdatedAt {
		t.Errorf("nil-fields Patch should not bump updated_at; before=%d after=%d",
			before.UpdatedAt, p.UpdatedAt)
	}
}

// ============================================================================
// HTTP handlers
// ============================================================================

func TestGetEndpoint_UnauthenticatedReturns401(t *testing.T) {
	d := openTestDB(t)
	mux := buildMux(t, d)

	req := httptest.NewRequest("GET", "/api/me/preferences", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("unauth GET: want 401, got %d (body=%q)", rr.Code, rr.Body.String())
	}
}

func TestGetEndpoint_AuthenticatedEmptyReturnsZeroValues(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice")
	token := issueSession(t, d, alice)
	mux := buildMux(t, d)

	req := httptest.NewRequest("GET", "/api/me/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	var got Preferences
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.DeviceID != alice {
		t.Errorf("device_id: got %q want %q", got.DeviceID, alice)
	}
	if got.DisplayName != "" || got.OwnerName != "" {
		t.Errorf("new device should have zero-value fields, got %+v", got)
	}
}

func TestPatchEndpoint_UnauthenticatedReturns401(t *testing.T) {
	d := openTestDB(t)
	mux := buildMux(t, d)

	body := strings.NewReader(`{"display_name":"Acolyte"}`)
	req := httptest.NewRequest("PATCH", "/api/me/preferences", body)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("unauth PATCH: want 401, got %d", rr.Code)
	}
}

func TestPatchEndpoint_AuthenticatedRoundTrip(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice")
	token := issueSession(t, d, alice)
	mux := buildMux(t, d)

	// PATCH both fields.
	body := strings.NewReader(`{"display_name":"Acolyte","owner_name":"josh"}`)
	req := httptest.NewRequest("PATCH", "/api/me/preferences", body)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH status=%d body=%q", rr.Code, rr.Body.String())
	}

	// GET back — should reflect the upsert.
	req = httptest.NewRequest("GET", "/api/me/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%q", rr.Code, rr.Body.String())
	}
	var got Preferences
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Acolyte" || got.OwnerName != "josh" {
		t.Errorf("round-trip: got %+v", got)
	}
}

func TestPatchEndpoint_DevicesAreIsolated(t *testing.T) {
	// Multi-device guard: alice's PATCH must not visible to bob.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice")
	bob := seedDevice(t, d, "device_bob")
	aliceTok := issueSession(t, d, alice)
	bobTok := issueSession(t, d, bob)
	mux := buildMux(t, d)

	// Alice sets her display name.
	patch(t, mux, aliceTok, `{"display_name":"Alice's Name"}`)

	// Bob's GET returns zero values.
	got := get(t, mux, bobTok)
	if got.DisplayName != "" {
		t.Errorf("bob should NOT see alice's display_name, got %q", got.DisplayName)
	}

	// Alice's GET still has hers.
	got = get(t, mux, aliceTok)
	if got.DisplayName != "Alice's Name" {
		t.Errorf("alice's own GET lost her name, got %q", got.DisplayName)
	}
}

func TestPatchEndpoint_BadJSONReturns400(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice")
	token := issueSession(t, d, alice)
	mux := buildMux(t, d)

	req := httptest.NewRequest("PATCH", "/api/me/preferences",
		strings.NewReader(`{this is not json`))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("malformed body: want 400, got %d (body=%q)", rr.Code, rr.Body.String())
	}
}

// ============================================================================
// HTTP helpers — keep handler tests tight
// ============================================================================

func patch(t *testing.T, mux http.Handler, token, body string) Preferences {
	t.Helper()
	req := httptest.NewRequest("PATCH", "/api/me/preferences",
		bytes.NewReader([]byte(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PATCH: status=%d body=%q", rr.Code, rr.Body.String())
	}
	var p Preferences
	if err := json.NewDecoder(io.NopCloser(bytes.NewReader(rr.Body.Bytes()))).Decode(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func get(t *testing.T, mux http.Handler, token string) Preferences {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/me/preferences", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET: status=%d body=%q", rr.Code, rr.Body.String())
	}
	var p Preferences
	if err := json.NewDecoder(io.NopCloser(bytes.NewReader(rr.Body.Bytes()))).Decode(&p); err != nil {
		t.Fatal(err)
	}
	return p
}
