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

// deck_versions_get_r60_test.go — HTTP-level integration tests for
// GET /api/decks/{owner}/{id}/versions/{version}. Covers the happy
// path (returns deck_list + parsed cards + saved_at), the .json
// extension fallback (some imports archive .json instead of .txt),
// and the not-found / invalid-arg edges that gate the new
// DECK HISTORY view's per-row diff fetch.

func writeVersionTextFile(t *testing.T, decksDir, owner, deckID string, version int, body string) string {
	t.Helper()
	dir := filepath.Join(decksDir, owner, "versions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir versions: %v", err)
	}
	path := filepath.Join(dir, deckID+"_v"+intToStr(int64(version))+".txt")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write version: %v", err)
	}
	return path
}

func TestHandleGetVersion_HappyPath(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)
	body := "COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n30 Mountain\n"
	writeVersionTextFile(t, decksDir, "alice", "krenko", 1, body)

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/krenko/versions/1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
	if v, _ := resp["version"].(float64); int(v) != 1 {
		t.Errorf("version: want 1, got %v", resp["version"])
	}
	if got, _ := resp["deck_list"].(string); got != body {
		t.Errorf("deck_list mismatch: want %q got %q", body, got)
	}
	fn, _ := resp["filename"].(string)
	if !strings.HasSuffix(fn, "_v1.txt") || !strings.HasPrefix(fn, "krenko") {
		t.Errorf("filename: want krenko_v1.txt-style, got %q", fn)
	}
	cards, ok := resp["cards"].([]any)
	if !ok || len(cards) == 0 {
		t.Errorf("cards: want non-empty parsed list, got %v", resp["cards"])
	}
	if _, ok := resp["saved_at"]; !ok {
		t.Error("saved_at missing")
	}
}

func TestHandleGetVersion_JSONExtensionFallback(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)
	// Archive a version with .json extension (matches the
	// handleUpdateDeck path that preserves the source file's ext).
	dir := filepath.Join(decksDir, "alice", "versions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "1 Sol Ring\n"
	if err := os.WriteFile(filepath.Join(dir, "krenko_v2.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/krenko/versions/2", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if got, _ := resp["deck_list"].(string); got != body {
		t.Errorf("deck_list mismatch: want %q got %q", body, got)
	}
	if fn, _ := resp["filename"].(string); !strings.HasSuffix(fn, "_v2.json") {
		t.Errorf("filename: want _v2.json suffix, got %q", fn)
	}
}

func TestHandleGetVersion_NotFound(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)
	// No version files exist — every numeric request 404s, but the
	// path itself is valid.
	for _, ver := range []string{"1", "5", "42"} {
		req := httptest.NewRequest("GET", "/api/decks/alice/krenko/versions/"+ver, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("v%s status: want 404, got %d (%s)", ver, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleGetVersion_InvalidVersion(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)
	// Out-of-range / non-numeric versions must hit the 400 path, not
	// fall through to the file-system probe.
	for _, ver := range []string{"0", "501", "1000"} {
		req := httptest.NewRequest("GET", "/api/decks/alice/krenko/versions/"+ver, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("v%s status: want 400, got %d (%s)", ver, rec.Code, rec.Body.String())
		}
	}
}

func TestHandleGetVersion_InvalidOwnerOrID(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)
	// Path-component validation should reject traversal attempts before
	// touching disk.
	for _, p := range []string{
		"/api/decks/..%2Fetc/krenko/versions/1",
		"/api/decks/alice/..versions/versions/1",
	} {
		req := httptest.NewRequest("GET", p, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("%s should not 200 (got %d)", p, rec.Code)
		}
	}
}

// Sanity: requesting an existing v1 .txt while a v1 .json also exists
// should resolve deterministically (.txt wins per the dispatch order).
func TestHandleGetVersion_TxtPreferredOverJSON(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)
	dir := filepath.Join(decksDir, "alice", "versions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "krenko_v1.txt"), []byte("from txt"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "krenko_v1.json"), []byte("from json"), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/krenko/versions/1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if got, _ := resp["deck_list"].(string); got != "from txt" {
		t.Errorf("want .txt to win, got %q", got)
	}
}
