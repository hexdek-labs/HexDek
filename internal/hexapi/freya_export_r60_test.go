package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func setupFreyaExportTest(t *testing.T) (*Handler, http.Handler, string) {
	t.Helper()
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(filepath.Join(decksDir, "alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	deckBody := "COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n20 Mountain\n4 Goblin Chieftain\n"
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "krenko_b3_R.txt"),
		[]byte(deckBody), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := sql.Open("sqlite", filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	if err := EnsureDeckMetaSchema(context.Background(), d); err != nil {
		t.Fatalf("schema: %v", err)
	}

	h := &Handler{DecksDir: decksDir}
	h.SetDB(d)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux, decksDir
}

func writeStrategy(t *testing.T, decksDir, owner, deckID string, payload map[string]any) {
	t.Helper()
	freyaDir := filepath.Join(decksDir, owner, "freya")
	if err := os.MkdirAll(freyaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(payload)
	if err := os.WriteFile(
		filepath.Join(freyaDir, deckID+".strategy.json"),
		b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// ───── happy path ─────

func TestHandleFreyaExport_FullEnvelopeShape(t *testing.T) {
	_, mux, decksDir := setupFreyaExportTest(t)
	writeStrategy(t, decksDir, "alice", "krenko_b3_R", map[string]any{
		"primary_archetype": "Aggro / Go Wide",
		"bracket":           3,
		"win_lines": []map[string]any{
			{"name": "Goblin swarm + Skullclamp", "type": "combat"},
		},
		"power_tier_counts": map[string]int{"S": 2, "A": 5, "B": 18, "C": 10, "D": 3},
	})

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko_b3_R/freya-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}

	var env FreyaExportEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.SchemaVersion != FreyaExportSchemaVersion {
		t.Errorf("schema_version: want %q, got %q", FreyaExportSchemaVersion, env.SchemaVersion)
	}
	if env.Source != "hexdek.dev/freya" {
		t.Errorf("source: got %q", env.Source)
	}
	if env.GeneratedAt == "" {
		t.Errorf("generated_at should populate")
	}
	if env.Deck.Owner != "alice" || env.Deck.ID != "krenko_b3_R" {
		t.Errorf("deck identity: got %+v", env.Deck)
	}
	if env.Deck.CommanderCard != "Krenko, Mob Boss" {
		t.Errorf("commander_card: got %q", env.Deck.CommanderCard)
	}
	// color_identity comes from parseDeckFilename which currently
	// hardcodes "?" — assert non-empty so we'd catch a real-color
	// regression but don't pin a value the parser doesn't compute.
	if env.Deck.ColorIdentity == "" {
		t.Errorf("color_identity should populate (even as '?'); got empty")
	}
	// parseDeckList auto-appends the commander when it's not
	// already in the body, so the 3 listed cards become 4 entries
	// + Krenko adds a qty-1 row → total 26, length 4.
	if env.Deck.CardCount != 26 {
		t.Errorf("card_count: want 26 (1+20+4+commander), got %d", env.Deck.CardCount)
	}
	if len(env.Deck.Cards) != 4 {
		t.Errorf("cards array: want 4 (3 listed + commander auto-append), got %d", len(env.Deck.Cards))
	}

	// Analysis blob passes through verbatim — verify by re-parsing.
	var analysis map[string]any
	if err := json.Unmarshal(env.Analysis, &analysis); err != nil {
		t.Fatalf("analysis re-parse: %v", err)
	}
	if analysis["primary_archetype"] != "Aggro / Go Wide" {
		t.Errorf("analysis pass-through lost: %v", analysis["primary_archetype"])
	}
	if _, ok := analysis["win_lines"]; !ok {
		t.Errorf("analysis pass-through dropped win_lines")
	}
}

// ───── extras passthrough ─────

func TestHandleFreyaExport_SurfacesR60Extras(t *testing.T) {
	h, mux, decksDir := setupFreyaExportTest(t)
	writeStrategy(t, decksDir, "alice", "krenko_b3_R", map[string]any{
		"primary_archetype": "Aggro / Go Wide",
	})

	// User feedback row from #421's path.
	if _, err := h.db.Exec(`INSERT INTO deck_meta (owner, id, archetype_feedback, updated_at)
		VALUES ('alice', 'krenko_b3_R', 'stax', strftime('%s', 'now'))`); err != nil {
		t.Fatalf("seed feedback: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko_b3_R/freya-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var env FreyaExportEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)

	// system_tags derived from primary_archetype via #416's helper.
	if len(env.Extras.SystemTags) != 1 || env.Extras.SystemTags[0] != "archetype:aggro-go-wide" {
		t.Errorf("system_tags: want [archetype:aggro-go-wide], got %v", env.Extras.SystemTags)
	}
	// User feedback from deck_meta.
	if env.Extras.UserArchetypeFeedback != "stax" {
		t.Errorf("user_archetype_feedback: want stax, got %q", env.Extras.UserArchetypeFeedback)
	}
}

// ───── error contracts ─────

func TestHandleFreyaExport_404OnUnknownDeck(t *testing.T) {
	_, mux, _ := setupFreyaExportTest(t)

	req := httptest.NewRequest("GET", "/api/decks/alice/nonexistent/freya-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("unknown deck: want 404, got %d", rec.Code)
	}
}

// Unlike /analysis (which auto-kicks-off Freya), /freya-export
// is READ-ONLY and 404s deterministically when no analysis exists.
// External integrators get a clear failure rather than "come back
// in 30 seconds" — crucial for batch-scraping corpus use cases.
func TestHandleFreyaExport_404WhenNoAnalysis_NoAutoKickoff(t *testing.T) {
	_, mux, _ := setupFreyaExportTest(t)
	// Deck file exists; no strategy.json.

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko_b3_R/freya-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("missing analysis: want 404, got %d", rec.Code)
	}
	// Error message points the consumer at the analyze endpoint —
	// they have to opt in, this endpoint stays read-only.
	if !strings.Contains(rec.Body.String(), "/analyze") {
		t.Errorf("404 body should hint at /analyze; got: %s", rec.Body.String())
	}
}

func TestHandleFreyaExport_400OnInvalidPath(t *testing.T) {
	_, mux, _ := setupFreyaExportTest(t)

	// Path-traversal-shaped owner.
	req := httptest.NewRequest("GET", "/api/decks/..bad/krenko/freya-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	// Either 400 (validatePathComponent rejects) OR 404 (the path
	// is technically valid but no such deck). The wrong outcome is
	// 200 + leaking a different deck's data.
	if rec.Code == 200 {
		t.Errorf("traversal-shaped path must not 200; got %d", rec.Code)
	}
}

func TestHandleFreyaExport_500OnMalformedStrategy(t *testing.T) {
	_, mux, decksDir := setupFreyaExportTest(t)
	freyaDir := filepath.Join(decksDir, "alice", "freya")
	os.MkdirAll(freyaDir, 0o755)
	// Deliberately broken JSON.
	os.WriteFile(filepath.Join(freyaDir, "krenko_b3_R.strategy.json"),
		[]byte("{{ broken"), 0o644)

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko_b3_R/freya-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 500 {
		t.Errorf("malformed strategy: want 500, got %d", rec.Code)
	}
}

// ───── version metadata ─────

func TestHandleFreyaExport_VersionHashFromDAG(t *testing.T) {
	_, mux, decksDir := setupFreyaExportTest(t)
	writeStrategy(t, decksDir, "alice", "krenko_b3_R", map[string]any{
		"primary_archetype": "Aggro / Go Wide",
	})

	// Seed a DAG entry so version_hash + version_number populate.
	h := &Handler{DecksDir: decksDir}
	dagDir := filepath.Join(decksDir, ".versions")
	h.registerDeckVersion("alice", "krenko_b3_R", "Krenko, Mob Boss",
		[]string{"Sol Ring", "Mountain", "Goblin Chieftain"})
	_ = dagDir

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko_b3_R/freya-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var env FreyaExportEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Extras.VersionNumber < 1 {
		t.Errorf("version_number should be >= 1 after registerDeckVersion; got %d",
			env.Extras.VersionNumber)
	}
	if env.Extras.VersionHash == "" {
		t.Errorf("version_hash should populate from the DAG; got empty")
	}
}

// ───── no-DB path ─────

// Handler can serve the export even with no DB (just no
// user_archetype_feedback in Extras). This keeps the endpoint
// usable in lightweight standalone configurations.
func TestHandleFreyaExport_WorksWithoutDB(t *testing.T) {
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	os.MkdirAll(filepath.Join(decksDir, "alice"), 0o755)
	os.WriteFile(filepath.Join(decksDir, "alice", "krenko.txt"),
		[]byte("COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n"), 0o644)
	writeStrategy(t, decksDir, "alice", "krenko", map[string]any{
		"primary_archetype": "Aggro / Go Wide",
	})

	h := &Handler{DecksDir: decksDir} // no SetDB
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko/freya-export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("no-DB path: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var env FreyaExportEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Extras.UserArchetypeFeedback != "" {
		t.Errorf("no-DB path should have empty feedback; got %q", env.Extras.UserArchetypeFeedback)
	}
	// system_tags should still populate (FS-based read).
	if len(env.Extras.SystemTags) == 0 {
		t.Errorf("system_tags should populate without DB; got empty")
	}
}

// Schema-version contract: external integrators pin against
// FreyaExportSchemaVersion. The constant value MUST be stable
// across patches — any breaking change to envelope shape bumps
// the major version.
func TestFreyaExportSchemaVersion_HasValue(t *testing.T) {
	if FreyaExportSchemaVersion == "" {
		t.Errorf("FreyaExportSchemaVersion must not be empty")
	}
	// Must follow N.M format — checked loosely (any non-zero
	// dotted number).
	if !strings.Contains(FreyaExportSchemaVersion, ".") {
		t.Errorf("FreyaExportSchemaVersion should look like a SemVer (N.M); got %q",
			FreyaExportSchemaVersion)
	}
}

// deckCardCount totals quantities; the parsed deck list uses
// "quantity" int keys.
func TestDeckCardCount_QuantityWeighted(t *testing.T) {
	cards := []map[string]any{
		{"name": "Sol Ring", "quantity": 1},
		{"name": "Mountain", "quantity": 20},
		{"name": "Goblin Chieftain", "quantity": 4},
		{"name": "NoQuantitySet"}, // defaults to 1
	}
	got := deckCardCount(cards)
	if got != 26 {
		t.Errorf("want 1+20+4+1=26, got %d", got)
	}
}
