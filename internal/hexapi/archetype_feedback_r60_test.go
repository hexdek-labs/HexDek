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

func setupFeedbackTest(t *testing.T) (*Handler, http.Handler, string) {
	t.Helper()
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(filepath.Join(decksDir, "alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "krenko.txt"),
		[]byte("COMMANDER: Krenko, Mob Boss\n1 Sol Ring\n20 Mountain\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Strategy.json fixture so loadFreyaPrimaryArchetype has data.
	if err := os.MkdirAll(filepath.Join(decksDir, "alice", "freya"), 0o755); err != nil {
		t.Fatal(err)
	}
	strategyJSON, _ := json.Marshal(map[string]any{"primary_archetype": "Combo / Infinite"})
	if err := os.WriteFile(
		filepath.Join(decksDir, "alice", "freya", "krenko.strategy.json"),
		strategyJSON, 0o644); err != nil {
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

func postFeedback(t *testing.T, mux http.Handler, owner, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/decks/"+owner+"/"+id+"/archetype-feedback",
		strings.NewReader(body))
	req.Header.Set("X-HexDek-Owner", owner)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func countFeedbackLog(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM archetype_feedback_log`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

// ───── schema migration ─────

func TestEnsureDeckMetaSchema_IsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	d, err := sql.Open("sqlite", filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for i := 0; i < 3; i++ {
		if err := EnsureDeckMetaSchema(context.Background(), d); err != nil {
			t.Fatalf("ensure #%d: %v", i, err)
		}
	}
	// archetype_feedback column should exist.
	rows, _ := d.Query(`PRAGMA table_info(deck_meta)`)
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		if name == "archetype_feedback" {
			found = true
		}
	}
	if !found {
		t.Errorf("archetype_feedback column missing after EnsureDeckMetaSchema")
	}
	// archetype_feedback_log table should exist.
	var tableExists int
	d.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='archetype_feedback_log'`).Scan(&tableExists)
	if tableExists != 1 {
		t.Errorf("archetype_feedback_log table missing")
	}
}

// ───── POST /archetype-feedback contract ─────

func TestHandleArchetypeFeedback_ConfirmPath(t *testing.T) {
	h, mux, _ := setupFeedbackTest(t)

	rec := postFeedback(t, mux, "alice", "krenko", `{"action":"confirm"}`)
	if rec.Code != 200 {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["feedback"] != "confirmed" {
		t.Errorf("feedback: want confirmed, got %v", resp["feedback"])
	}
	if resp["freya_archetype"] != "Combo / Infinite" {
		t.Errorf("freya_archetype snapshot: want 'Combo / Infinite', got %v", resp["freya_archetype"])
	}
	// State landed in deck_meta.
	if got := h.loadArchetypeFeedback(context.Background(), "alice", "krenko"); got != "confirmed" {
		t.Errorf("loadArchetypeFeedback: want confirmed, got %q", got)
	}
	// Training log got a row.
	if n := countFeedbackLog(t, h.db); n != 1 {
		t.Errorf("training log: want 1 row, got %d", n)
	}
}

func TestHandleArchetypeFeedback_CorrectPath(t *testing.T) {
	h, mux, _ := setupFeedbackTest(t)

	rec := postFeedback(t, mux, "alice", "krenko", `{"action":"correct","archetype":"Stax"}`)
	if rec.Code != 200 {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["feedback"] != "stax" {
		t.Errorf("feedback: want stax (normalized), got %v", resp["feedback"])
	}
	if got := h.loadArchetypeFeedback(context.Background(), "alice", "krenko"); got != "stax" {
		t.Errorf("persisted: want stax, got %q", got)
	}

	// Log row carries the snapshot of what Freya said + what user
	// changed it to.
	var freya, userAct, userArch string
	h.db.QueryRow(`SELECT freya_archetype, user_action, user_archetype FROM archetype_feedback_log`).
		Scan(&freya, &userAct, &userArch)
	if freya != "Combo / Infinite" || userAct != "correct" || userArch != "stax" {
		t.Errorf("log row: got freya=%q action=%q user=%q", freya, userAct, userArch)
	}
}

func TestHandleArchetypeFeedback_CorrectNormalizesMultiWord(t *testing.T) {
	h, mux, _ := setupFeedbackTest(t)

	rec := postFeedback(t, mux, "alice", "krenko",
		`{"action":"correct","archetype":"Lands Matter"}`)
	if rec.Code != 200 {
		t.Fatalf("status: %d", rec.Code)
	}
	if got := h.loadArchetypeFeedback(context.Background(), "alice", "krenko"); got != "lands-matter" {
		t.Errorf("multi-word normalize: want lands-matter, got %q", got)
	}
}

func TestHandleArchetypeFeedback_ClearPath(t *testing.T) {
	h, mux, _ := setupFeedbackTest(t)
	// Prime with a confirm so clear has something to clear.
	postFeedback(t, mux, "alice", "krenko", `{"action":"confirm"}`)

	rec := postFeedback(t, mux, "alice", "krenko", `{"action":"clear"}`)
	if rec.Code != 200 {
		t.Fatalf("status: %d", rec.Code)
	}
	if got := h.loadArchetypeFeedback(context.Background(), "alice", "krenko"); got != "" {
		t.Errorf("post-clear: want empty, got %q", got)
	}
	// Training log captures BOTH actions (append-only).
	if n := countFeedbackLog(t, h.db); n != 2 {
		t.Errorf("log: want 2 rows after confirm+clear, got %d", n)
	}
}

func TestHandleArchetypeFeedback_CorrectRequiresArchetype(t *testing.T) {
	_, mux, _ := setupFeedbackTest(t)

	rec := postFeedback(t, mux, "alice", "krenko", `{"action":"correct"}`)
	if rec.Code != 400 {
		t.Errorf("correct-without-archetype: want 400, got %d (%s)", rec.Code, rec.Body.String())
	}
	rec2 := postFeedback(t, mux, "alice", "krenko", `{"action":"correct","archetype":"   "}`)
	if rec2.Code != 400 {
		t.Errorf("correct-with-whitespace: want 400, got %d", rec2.Code)
	}
}

func TestHandleArchetypeFeedback_BadAction(t *testing.T) {
	_, mux, _ := setupFeedbackTest(t)
	rec := postFeedback(t, mux, "alice", "krenko", `{"action":"delete"}`)
	if rec.Code != 400 {
		t.Errorf("unknown action: want 400, got %d", rec.Code)
	}
}

func TestHandleArchetypeFeedback_MalformedBody(t *testing.T) {
	_, mux, _ := setupFeedbackTest(t)
	rec := postFeedback(t, mux, "alice", "krenko", `{{ broken`)
	if rec.Code != 400 {
		t.Errorf("malformed body: want 400, got %d", rec.Code)
	}
}

func TestHandleArchetypeFeedback_RequiresAuth(t *testing.T) {
	_, mux, _ := setupFeedbackTest(t)

	req := httptest.NewRequest("POST", "/api/decks/alice/krenko/archetype-feedback",
		strings.NewReader(`{"action":"confirm"}`))
	req.Header.Set("Content-Type", "application/json")
	// No X-HexDek-Owner header.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("no auth: want 401, got %d", rec.Code)
	}
}

func TestHandleArchetypeFeedback_RejectsCrossOwnerCaller(t *testing.T) {
	_, mux, _ := setupFeedbackTest(t)
	// Bob attempts to confirm Alice's deck.
	req := httptest.NewRequest("POST", "/api/decks/alice/krenko/archetype-feedback",
		strings.NewReader(`{"action":"confirm"}`))
	req.Header.Set("X-HexDek-Owner", "bob")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("cross-owner: want 403, got %d", rec.Code)
	}
}

func TestHandleArchetypeFeedback_404OnUnknownDeck(t *testing.T) {
	_, mux, _ := setupFeedbackTest(t)
	rec := postFeedback(t, mux, "alice", "no-such-deck", `{"action":"confirm"}`)
	if rec.Code != 404 {
		t.Errorf("unknown deck: want 404, got %d", rec.Code)
	}
}

// GET surfaces the persisted feedback so the UI can render the
// correct chip state on page load.
func TestHandleGetDeck_SurfacesArchetypeFeedback(t *testing.T) {
	_, mux, _ := setupFeedbackTest(t)
	postFeedback(t, mux, "alice", "krenko", `{"action":"correct","archetype":"Stax"}`)

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["archetype_feedback"] != "stax" {
		t.Errorf("GET archetype_feedback: want stax, got %v", resp["archetype_feedback"])
	}
}

// Training log is append-only — repeated feedback from the same user
// stacks rows (so historical re-classifications are preserved).
func TestHandleArchetypeFeedback_LogIsAppendOnly(t *testing.T) {
	h, mux, _ := setupFeedbackTest(t)
	postFeedback(t, mux, "alice", "krenko", `{"action":"confirm"}`)
	postFeedback(t, mux, "alice", "krenko", `{"action":"correct","archetype":"Stax"}`)
	postFeedback(t, mux, "alice", "krenko", `{"action":"correct","archetype":"Combo"}`)
	postFeedback(t, mux, "alice", "krenko", `{"action":"clear"}`)

	if n := countFeedbackLog(t, h.db); n != 4 {
		t.Errorf("log: want 4 rows (one per action), got %d", n)
	}
	// Final persisted state is just the last action.
	if got := h.loadArchetypeFeedback(context.Background(), "alice", "krenko"); got != "" {
		t.Errorf("post-final-clear: want empty, got %q", got)
	}
}

// Snapshot fidelity: log captures the RAW Freya archetype text
// ("Combo / Infinite") not the normalized prefixed tag, so future
// re-training can join against archetypes.go canonical names.
func TestHandleArchetypeFeedback_LogCapturesRawFreyaArchetype(t *testing.T) {
	h, mux, _ := setupFeedbackTest(t)
	postFeedback(t, mux, "alice", "krenko", `{"action":"confirm"}`)

	var freya string
	h.db.QueryRow(`SELECT freya_archetype FROM archetype_feedback_log`).Scan(&freya)
	if freya != "Combo / Infinite" {
		t.Errorf("log snapshot should preserve raw Freya text; got %q", freya)
	}
}
