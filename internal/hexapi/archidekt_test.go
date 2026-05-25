package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/hexdek/hexdek/internal/archidekt"
)

// setupArchidektTest spins up a Handler with an in-memory SQLite +
// import_log schema + a stub Archidekt API. Returns the handler, the
// registered router, the decks directory, and the stub URL so tests
// can assert against the underlying request stream when they care to.
func setupArchidektTest(t *testing.T, apiJSON string) (*Handler, http.Handler, string) {
	t.Helper()
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(decksDir, 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := EnsureDeckMetaSchema(context.Background(), db); err != nil {
		t.Fatalf("schema deck_meta: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS import_log (
		    id           INTEGER PRIMARY KEY AUTOINCREMENT,
		    owner        TEXT NOT NULL,
		    deck_key     TEXT NOT NULL,
		    deck_name    TEXT NOT NULL DEFAULT '',
		    commander    TEXT NOT NULL DEFAULT '',
		    source       TEXT NOT NULL,
		    source_url   TEXT NOT NULL DEFAULT '',
		    card_count   INTEGER NOT NULL DEFAULT 0,
		    imported_at  INTEGER NOT NULL
		);`); err != nil {
		t.Fatalf("schema import_log: %v", err)
	}

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, apiJSON)
	}))
	t.Cleanup(stub.Close)
	t.Cleanup(archidekt.SetAPIBaseForTest(stub.URL))

	h := &Handler{DecksDir: decksDir}
	h.SetDB(db)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux, decksDir
}

const validArchidektJSON = `{
	"name": "Krenko Goblin Storm",
	"categories": [
		{"name": "Commander", "includedInDeck": true},
		{"name": "Maybeboard", "includedInDeck": false}
	],
	"cards": [
		{"quantity": 1, "categories": ["Commander"], "card": {"oracleCard": {"name": "Krenko, Mob Boss"}}},
		{"quantity": 1, "categories": [], "card": {"oracleCard": {"name": "Sol Ring"}}},
		{"quantity": 32, "categories": [], "card": {"oracleCard": {"name": "Mountain"}}},
		{"quantity": 1, "categories": ["Maybeboard"], "card": {"oracleCard": {"name": "Sneak Attack"}}}
	]
}`

func TestArchidektImport_HappyPath(t *testing.T) {
	h, mux, decksDir := setupArchidektTest(t, validArchidektJSON)

	body := jsonBody(t, map[string]any{
		"url":   "https://archidekt.com/decks/12345/krenko-goblin-storm",
		"owner": "bob",
		"tags":  []string{"goblin", "tribal"},
	})
	req := httptest.NewRequest("POST", "/api/import/archidekt", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("import: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["owner"] != "bob" {
		t.Errorf("owner = %v, want bob", resp["owner"])
	}
	if resp["source"] != "archidekt" {
		t.Errorf("source = %v, want archidekt", resp["source"])
	}
	if resp["archidekt_id"] != "12345" {
		t.Errorf("archidekt_id = %v, want 12345", resp["archidekt_id"])
	}
	if resp["commander"] != "Krenko, Mob Boss" {
		t.Errorf("commander = %v, want Krenko, Mob Boss", resp["commander"])
	}

	id, _ := resp["id"].(string)
	if id == "" {
		t.Fatal("response missing id")
	}

	// File landed under bob/<id>.txt and the maybeboard card went to
	// the comment line (not the legal mainboard).
	deckPath := filepath.Join(decksDir, "bob", id+".txt")
	raw, err := os.ReadFile(deckPath)
	if err != nil {
		t.Fatalf("read deck file: %v", err)
	}
	body2 := string(raw)
	if !strings.Contains(body2, "COMMANDER: Krenko, Mob Boss\n") {
		t.Errorf("commander line missing:\n%s", body2)
	}
	if !strings.Contains(body2, "32 Mountain\n") {
		t.Errorf("Mountain count wrong:\n%s", body2)
	}
	if !strings.Contains(body2, "// Maybeboard: 1 Sneak Attack\n") {
		t.Errorf("Maybeboard card should be commented out:\n%s", body2)
	}
	if strings.Contains("\n"+body2, "\n1 Sneak Attack\n") {
		t.Errorf("Sneak Attack leaked into mainboard:\n%s", body2)
	}

	// import_log uses h.Showmatch.sqlDB (best-effort, optional) and we
	// don't wire a Showmatch into the test rig — same as the clone
	// test, which also skips this assertion. The handler still calls
	// logImport(); a missing Showmatch silently no-ops, matching prod
	// behavior when the DB isn't attached.

	// Tags persisted.
	tags := h.loadTags(context.Background(), "bob", id)
	if len(tags) != 2 {
		t.Errorf("tags = %v, want 2 entries", tags)
	}
}

func TestArchidektImport_RejectsNonArchidektHost(t *testing.T) {
	_, mux, _ := setupArchidektTest(t, validArchidektJSON)

	body := jsonBody(t, map[string]any{
		"url":   "https://www.moxfield.com/decks/abc123",
		"owner": "bob",
	})
	req := httptest.NewRequest("POST", "/api/import/archidekt", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("non-archidekt URL: code=%d, want 400", rec.Code)
	}
}

func TestArchidektImport_RejectsMalformedDeckID(t *testing.T) {
	_, mux, _ := setupArchidektTest(t, validArchidektJSON)

	// archidekt.com path that doesn't match the /decks/{numeric-id}
	// shape — Archidekt always uses numeric IDs.
	body := jsonBody(t, map[string]any{
		"url":   "https://archidekt.com/profile/bob",
		"owner": "bob",
	})
	req := httptest.NewRequest("POST", "/api/import/archidekt", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("malformed deck path: code=%d, want 400", rec.Code)
	}
}

func TestArchidektImport_PropagatesAPIError(t *testing.T) {
	// Stub returns 404 → handler returns 502 with the error surfaced.
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	os.MkdirAll(decksDir, 0o755)
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, `{"detail": "Not found."}`)
	}))
	t.Cleanup(stub.Close)
	t.Cleanup(archidekt.SetAPIBaseForTest(stub.URL))

	h := &Handler{DecksDir: decksDir}
	mux := http.NewServeMux()
	h.Register(mux)

	body := jsonBody(t, map[string]any{
		"url":   "https://archidekt.com/decks/99999",
		"owner": "bob",
	})
	req := httptest.NewRequest("POST", "/api/import/archidekt", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("API 404: code=%d, want 502", rec.Code)
	}
}

func TestArchidektImport_RejectsMalformedJSONBody(t *testing.T) {
	_, mux, _ := setupArchidektTest(t, validArchidektJSON)

	req := httptest.NewRequest("POST", "/api/import/archidekt",
		strings.NewReader(`not-json{{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad body: code=%d, want 400", rec.Code)
	}
}

func TestArchidektImport_PartnerPairBothLandAsCommanders(t *testing.T) {
	const partnerJSON = `{
		"name": "Tymna + Kraum",
		"categories": [{"name": "Commander", "includedInDeck": true}],
		"cards": [
			{"quantity": 1, "categories": ["Commander"], "card": {"oracleCard": {"name": "Tymna the Weaver"}}},
			{"quantity": 1, "categories": ["Commander", "Partner"], "card": {"oracleCard": {"name": "Kraum, Ludevic's Opus"}}},
			{"quantity": 1, "categories": [], "card": {"oracleCard": {"name": "Sol Ring"}}}
		]
	}`
	_, mux, decksDir := setupArchidektTest(t, partnerJSON)

	body := jsonBody(t, map[string]any{
		"url":   "https://archidekt.com/decks/42",
		"owner": "bob",
	})
	req := httptest.NewRequest("POST", "/api/import/archidekt", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("import: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	id, _ := resp["id"].(string)

	raw, err := os.ReadFile(filepath.Join(decksDir, "bob", id+".txt"))
	if err != nil {
		t.Fatalf("read deck file: %v", err)
	}
	body2 := string(raw)
	// Both partners should be COMMANDER: lines.
	if strings.Count(body2, "COMMANDER:") != 2 {
		t.Errorf("partner deck: want 2 COMMANDER lines, got\n%s", body2)
	}
	if !strings.Contains(body2, "COMMANDER: Tymna the Weaver\n") {
		t.Errorf("missing Tymna commander line:\n%s", body2)
	}
	if !strings.Contains(body2, "COMMANDER: Kraum, Ludevic's Opus\n") {
		t.Errorf("missing Kraum commander line:\n%s", body2)
	}
}

// jsonBody builds an io.Reader from a JSON body for httptest. Local
// to this file. Named to avoid the package-level `bytes` import.
func jsonBody(t *testing.T, v map[string]any) *strings.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return strings.NewReader(string(b))
}
