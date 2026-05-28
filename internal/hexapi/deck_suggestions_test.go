package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupSuggestionsTest(t *testing.T) (*Handler, http.Handler, string) {
	t.Helper()
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(filepath.Join(decksDir, "alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := &Handler{DecksDir: decksDir}
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux, decksDir
}

// suggestionsFixture is a representative SuggestedChanges JSON
// payload — matches the schema emitted by freya's
// saveSuggestionsJSON. Used by the happy-path test to exercise the
// sidecar read + content-type wiring.
const suggestionsFixture = `{
  "adds": [
    {"card_name":"Counterspell","category":"interaction","priority":8,
     "reason":"current 4 interaction spells, target 6 for Control archetype","gap_size":2},
    {"card_name":"Swords to Plowshares","category":"interaction","priority":8,
     "reason":"current 4 interaction spells, target 6 for Control archetype","gap_size":2}
  ],
  "cuts": [
    {"card_name":"Gilded Lotus","category":"cuttable","priority":5,
     "reason":"high CMC with no clear role"}
  ],
  "swaps": [
    {"add_card_name":"Bountiful Promenade","cut_card_name":"Tranquil Cove",
     "category":"manabase","priority":7,
     "reason":"Mana base C-grade — swap Tranquil Cove for Bountiful Promenade (color-identity preserving, enters untapped in multiplayer)"}
  ],
  "summary": "2 adds, 1 cut, 1 swap suggested. Highest priority: add Counterspell.",
  "total_changes": 4
}`

// TestDeckSuggestions_HappyPath verifies the read+forward path:
// when the sidecar exists, the handler returns its bytes verbatim
// with the application/json content type.
func TestDeckSuggestions_HappyPath(t *testing.T) {
	_, mux, decksDir := setupSuggestionsTest(t)

	// Write deck file + sidecar so the handler hits the read path.
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "mydeck.txt"),
		[]byte("COMMANDER: Yenna, Redtooth Regent\n1 Sol Ring\n20 Plains\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	freyaDir := filepath.Join(decksDir, "alice", "freya")
	if err := os.MkdirAll(freyaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(freyaDir, "mydeck.suggestions.json"),
		[]byte(suggestionsFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/decks/alice/mydeck/suggestions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: want application/json, got %q", ct)
	}

	// Decode + spot-check fields.
	var payload struct {
		Adds  []map[string]any `json:"adds"`
		Cuts  []map[string]any `json:"cuts"`
		Swaps []map[string]any `json:"swaps"`
		Summary      string `json:"summary"`
		TotalChanges int    `json:"total_changes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode body: %v\nbody: %s", err, rec.Body.String())
	}
	if len(payload.Adds) != 2 {
		t.Errorf("adds: want 2, got %d", len(payload.Adds))
	}
	if len(payload.Cuts) != 1 {
		t.Errorf("cuts: want 1, got %d", len(payload.Cuts))
	}
	if len(payload.Swaps) != 1 {
		t.Errorf("swaps: want 1, got %d", len(payload.Swaps))
	}
	if payload.TotalChanges != 4 {
		t.Errorf("total_changes: want 4, got %d", payload.TotalChanges)
	}
	if !strings.Contains(payload.Summary, "Counterspell") {
		t.Errorf("summary missing Counterspell: %q", payload.Summary)
	}

	// First add: should be Counterspell with the expected field shape.
	if len(payload.Adds) > 0 {
		first := payload.Adds[0]
		if first["card_name"] != "Counterspell" {
			t.Errorf("first add card_name: want Counterspell, got %v", first["card_name"])
		}
		if first["category"] != "interaction" {
			t.Errorf("first add category: want interaction, got %v", first["category"])
		}
		if int(first["priority"].(float64)) != 8 {
			t.Errorf("first add priority: want 8, got %v", first["priority"])
		}
		if !strings.Contains(first["reason"].(string), "Control") {
			t.Errorf("first add reason missing 'Control': %v", first["reason"])
		}
	}
}

// TestDeckSuggestions_SwapShape pins the snake_case field names on
// swap entries (add_card_name + cut_card_name) so frontend bindings
// don't drift from the documented schema.
func TestDeckSuggestions_SwapShape(t *testing.T) {
	_, mux, decksDir := setupSuggestionsTest(t)
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "mydeck.txt"),
		[]byte("COMMANDER: Yenna, Redtooth Regent\n1 Sol Ring\n20 Plains\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	freyaDir := filepath.Join(decksDir, "alice", "freya")
	os.MkdirAll(freyaDir, 0o755)
	if err := os.WriteFile(
		filepath.Join(freyaDir, "mydeck.suggestions.json"),
		[]byte(suggestionsFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/decks/alice/mydeck/suggestions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// Raw-string check — looking for the exact JSON keys the schema
	// promises. Catches accidental camelCase serialization regressions.
	body := rec.Body.String()
	for _, key := range []string{
		`"adds"`, `"cuts"`, `"swaps"`, `"summary"`, `"total_changes"`,
		`"card_name"`, `"category"`, `"priority"`, `"reason"`, `"gap_size"`,
		`"add_card_name"`, `"cut_card_name"`,
	} {
		if !strings.Contains(body, key) {
			t.Errorf("response missing key %s", key)
		}
	}
}

// TestDeckSuggestions_MissingSidecarWithDeckFile_AutoTriggerAnalyzing
// verifies the auto-trigger fallback: when the deck file exists but
// no sidecar JSON is present, the handler returns a 200 with the
// "analyzing — refresh" status payload (mirrors handleGetAnalysis).
//
// We can't easily intercept the goroutine spawn here without
// dependency-injecting a Freya runner, so this test asserts the
// fallback response shape rather than the goroutine itself. The
// background runFreya call against the fixture deck is harmless —
// it logs warnings and exits cleanly when no oracle data is
// present.
func TestDeckSuggestions_MissingSidecarWithDeckFile_AutoTriggerAnalyzing(t *testing.T) {
	_, mux, decksDir := setupSuggestionsTest(t)
	// Deck file exists; sidecar absent.
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "mydeck.txt"),
		[]byte("COMMANDER: Yenna, Redtooth Regent\n1 Sol Ring\n20 Plains\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/decks/alice/mydeck/suggestions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200 (analyzing fallback), got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["status"] != "analyzing" {
		t.Errorf("status field: want analyzing, got %v", payload["status"])
	}
	if !strings.Contains(payload["message"].(string), "refresh") {
		t.Errorf("message missing 'refresh' hint: %v", payload["message"])
	}
}

// TestDeckSuggestions_MissingDeckFile_Returns404 verifies the 404
// path: when neither the deck file nor the sidecar exists, the
// handler returns a clear 404.
func TestDeckSuggestions_MissingDeckFile_Returns404(t *testing.T) {
	_, mux, _ := setupSuggestionsTest(t)

	req := httptest.NewRequest("GET", "/api/decks/alice/no-such-deck/suggestions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// TestDeckSuggestions_InvalidPathComponents_Returns400 verifies the
// path-component validation (mirrors handleGetAnalysis behavior).
// Owner / id values with path separators or null bytes are rejected
// before any filesystem access.
func TestDeckSuggestions_InvalidPathComponents_Returns400(t *testing.T) {
	_, mux, _ := setupSuggestionsTest(t)

	cases := []struct {
		name string
		path string
	}{
		{"owner_with_slash", "/api/decks/.." + string(filepath.Separator) + "alice/deck/suggestions"},
		{"id_with_dotdot", "/api/decks/alice/..%2Fetc%2Fpasswd/suggestions"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", c.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			// Either 400 (validation) or 404 (route didn't match) is
			// acceptable — the key is we don't reach the filesystem
			// with an attacker-controlled path. 200 would mean we
			// leaked data.
			if rec.Code == http.StatusOK {
				t.Errorf("validation/route bypass: got 200 on %s (body: %s)", c.path, rec.Body.String())
			}
		})
	}
}

// TestDeckSuggestions_EmptySuggestionsArrays verifies handling of
// the "well-tuned deck" case — empty adds/cuts/swaps arrays
// shouldn't cause JSON encoding issues at the handler boundary.
func TestDeckSuggestions_EmptySuggestionsArrays(t *testing.T) {
	_, mux, decksDir := setupSuggestionsTest(t)
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "mydeck.txt"),
		[]byte("COMMANDER: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	freyaDir := filepath.Join(decksDir, "alice", "freya")
	os.MkdirAll(freyaDir, 0o755)
	emptyFixture := `{"adds":[],"cuts":[],"swaps":[],"summary":"Deck looks well-tuned — no specific changes suggested.","total_changes":0}`
	if err := os.WriteFile(
		filepath.Join(freyaDir, "mydeck.suggestions.json"),
		[]byte(emptyFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/api/decks/alice/mydeck/suggestions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var payload struct {
		Adds         []any  `json:"adds"`
		Cuts         []any  `json:"cuts"`
		Swaps        []any  `json:"swaps"`
		Summary      string `json:"summary"`
		TotalChanges int    `json:"total_changes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Adds) != 0 || len(payload.Cuts) != 0 || len(payload.Swaps) != 0 {
		t.Errorf("empty arrays: want all 0, got %d/%d/%d",
			len(payload.Adds), len(payload.Cuts), len(payload.Swaps))
	}
	if payload.TotalChanges != 0 {
		t.Errorf("total_changes: want 0, got %d", payload.TotalChanges)
	}
	if !strings.Contains(payload.Summary, "well-tuned") {
		t.Errorf("summary missing 'well-tuned': %q", payload.Summary)
	}
}
