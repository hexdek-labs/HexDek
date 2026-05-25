package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupForkTest mirrors setupCloneTest: alice owns a krenko deck on
// disk, the schema is fresh, and the router is registered. The caller
// drives requests as bob (the would-be forker).
func setupForkTest(t *testing.T) (*Handler, http.Handler, string) {
	t.Helper()
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(filepath.Join(decksDir, "alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	deckBody := "COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n20 Mountain\n1 Goblin Chieftain\n"
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "krenko.txt"),
		[]byte(deckBody), 0o644); err != nil {
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

	h := &Handler{DecksDir: decksDir}
	h.SetDB(db)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, mux, decksDir
}

func TestForkDeck_HappyPath(t *testing.T) {
	h, mux, decksDir := setupForkTest(t)

	req := httptest.NewRequest("POST", "/api/decks/alice/krenko/fork", nil)
	req.Header.Set("X-HexDek-Owner", "bob")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("fork: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["owner"] != "bob" {
		t.Errorf("owner = %v, want bob", resp["owner"])
	}
	if resp["forked_from"] != "alice/krenko" {
		t.Errorf("forked_from = %v, want alice/krenko", resp["forked_from"])
	}
	// Name must carry the (FORK) suffix so the user can distinguish a
	// fork from the original without opening the deck page.
	if name, _ := resp["name"].(string); name == "" || name[len(name)-len("(FORK)"):] != "(FORK)" {
		t.Errorf("name = %q, want trailing \"(FORK)\"", name)
	}

	id, _ := resp["id"].(string)
	if id == "" {
		t.Fatal("response missing id")
	}
	// File landed under bob/, not alice/.
	wantDeck := filepath.Join(decksDir, "bob", id+".txt")
	if _, err := os.Stat(wantDeck); err != nil {
		t.Fatalf("fork file not at %s: %v", wantDeck, err)
	}

	// forked_from survives a round-trip through GET.
	getReq := httptest.NewRequest("GET", "/api/decks/bob/"+id, nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != 200 {
		t.Fatalf("get: code=%d body=%s", getRec.Code, getRec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("get decode: %v", err)
	}
	if got["forked_from"] != "alice/krenko" {
		t.Errorf("GET forked_from = %v, want alice/krenko", got["forked_from"])
	}
	// cloned_from must NOT leak the fork source — fork and clone are
	// separate fields and a fork shouldn't masquerade as a clone.
	if got["cloned_from"] != "" {
		t.Errorf("GET cloned_from = %v, want \"\" (forks set forked_from, not cloned_from)", got["cloned_from"])
	}

	// fork_log got the row; clone_log stayed empty (separate budgets).
	var nFork, nClone int
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM fork_log WHERE owner = ?`, "bob").Scan(&nFork); err != nil {
		t.Fatal(err)
	}
	if nFork != 1 {
		t.Errorf("fork_log rows = %d, want 1", nFork)
	}
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM clone_log WHERE owner = ?`, "bob").Scan(&nClone); err != nil {
		t.Fatal(err)
	}
	if nClone != 0 {
		t.Errorf("clone_log rows = %d, want 0 (forks must not consume the clone budget)", nClone)
	}
}

func TestForkDeck_Unauthenticated(t *testing.T) {
	_, mux, _ := setupForkTest(t)

	req := httptest.NewRequest("POST", "/api/decks/alice/krenko/fork", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauth fork: code=%d, want 401", rec.Code)
	}
}

func TestForkDeck_RejectsSelfFork(t *testing.T) {
	_, mux, _ := setupForkTest(t)

	req := httptest.NewRequest("POST", "/api/decks/alice/krenko/fork", nil)
	req.Header.Set("X-HexDek-Owner", "alice")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("self-fork: code=%d, want 400", rec.Code)
	}
}

func TestForkDeck_DeckNotFound(t *testing.T) {
	_, mux, _ := setupForkTest(t)

	req := httptest.NewRequest("POST", "/api/decks/alice/ghost/fork", nil)
	req.Header.Set("X-HexDek-Owner", "bob")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("missing src: code=%d, want 404", rec.Code)
	}
}

func TestForkDeck_RateLimit(t *testing.T) {
	h, mux, _ := setupForkTest(t)

	// Pre-load fork_log with ForkRateLimit recent rows for bob.
	now := time.Now().Unix()
	for i := 0; i < ForkRateLimit; i++ {
		if _, err := h.db.Exec(
			`INSERT INTO fork_log (owner, src_key, dst_key, forked_at) VALUES (?, ?, ?, ?)`,
			"bob", "alice/krenko", "bob/k_fork", now); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest("POST", "/api/decks/alice/krenko/fork", nil)
	req.Header.Set("X-HexDek-Owner", "bob")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("rate-limited: code=%d body=%s, want 429", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("rate-limited response missing Retry-After header")
	}
}

// Clone and fork share neither rate-limit budget nor metadata column.
// A user at the clone cap should still be able to fork (and vice versa).
func TestForkDeck_BudgetIsolatedFromClone(t *testing.T) {
	h, mux, _ := setupForkTest(t)

	now := time.Now().Unix()
	for i := 0; i < CloneRateLimit; i++ {
		if _, err := h.db.Exec(
			`INSERT INTO clone_log (owner, src_key, dst_key, cloned_at) VALUES (?, ?, ?, ?)`,
			"bob", "alice/krenko", "bob/k_clone", now); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest("POST", "/api/decks/alice/krenko/fork", nil)
	req.Header.Set("X-HexDek-Owner", "bob")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("fork while clone-capped: code=%d body=%s, want 200", rec.Code, rec.Body.String())
	}
}

// A second fork of the same source picks a non-colliding dst id rather
// than overwriting the first one.
func TestForkDeck_NameCollisionAppendsSuffix(t *testing.T) {
	_, mux, decksDir := setupForkTest(t)
	if err := os.MkdirAll(filepath.Join(decksDir, "bob"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-create the natural fork destination so the handler has to pick
	// krenko_fork2.
	if err := os.WriteFile(
		filepath.Join(decksDir, "bob", "krenko_fork.txt"),
		[]byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/decks/alice/krenko/fork", nil)
	req.Header.Set("X-HexDek-Owner", "bob")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("collision fork: code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] != "krenko_fork2" {
		t.Errorf("id = %v, want krenko_fork2", resp["id"])
	}
}
