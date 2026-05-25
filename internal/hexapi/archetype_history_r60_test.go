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
	"time"

	"github.com/hexdek/hexdek/internal/versioning"
)

// Setup mirrors setupFeedbackTest but seeds a versioning DAG so we
// can pin per-version archetype events through the timeline path.
func setupHistoryTest(t *testing.T) (*Handler, http.Handler, string) {
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

// seedVersion stamps a fresh version into the DAG at the given
// time + archetype so the test can reason about CreatedAt-ordered
// timelines without waiting on real wall-clock ticks. Wraps
// versioning.RegisterVersion + UpdateArchetype + a manual
// CreatedAt rewrite (RegisterVersion stamps time.Now; the test
// needs deterministic ordering).
func seedVersion(t *testing.T, decksDir, owner, id, archetype string, when time.Time, cards []string) {
	t.Helper()
	dagDir := filepath.Join(decksDir, ".versions")
	dag, err := versioning.LoadDAG(dagDir)
	if err != nil {
		t.Fatalf("load DAG: %v", err)
	}
	node := dag.RegisterVersion(owner, id, "Krenko, Mob Boss", cards)
	node.CreatedAt = when.UTC().Format(time.RFC3339)
	dag.UpdateArchetype(node.Hash, archetype)
	if err := versioning.SaveDAG(dagDir, dag); err != nil {
		t.Fatalf("save DAG: %v", err)
	}
}

// ───── pure builder coverage ─────

func TestBuildArchetypeHistory_VersionEventsCarryReclassification(t *testing.T) {
	h, _, decksDir := setupHistoryTest(t)

	t0 := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)

	seedVersion(t, decksDir, "alice", "krenko", "Combo / Infinite", t0,
		[]string{"Sol Ring", "Mountain", "Mountain"})
	seedVersion(t, decksDir, "alice", "krenko", "Combo / Infinite", t1,
		[]string{"Sol Ring", "Mountain", "Mountain", "Goblin Recruiter"})
	seedVersion(t, decksDir, "alice", "krenko", "Storm", t2,
		[]string{"Sol Ring", "Mountain", "Lightning Bolt", "Grapeshot"})

	events := buildArchetypeHistory(decksDir, "alice", "krenko", h.db, context.Background())

	if len(events) != 3 {
		t.Fatalf("want 3 version events, got %d (%+v)", len(events), events)
	}
	for i, kind := range []string{"version_archetype", "version_archetype", "version_archetype"} {
		if events[i].Kind != kind {
			t.Errorf("event %d: want kind %s, got %s", i, kind, events[i].Kind)
		}
	}
	// v1 → no change_from (anchor)
	if events[0].ChangedFrom != "" {
		t.Errorf("first version should anchor (no ChangedFrom); got %q", events[0].ChangedFrom)
	}
	// v2 → same archetype, no change_from
	if events[1].ChangedFrom != "" {
		t.Errorf("v2 with same archetype should not carry ChangedFrom; got %q", events[1].ChangedFrom)
	}
	// v3 → reclassified, ChangedFrom captures the prior
	if events[2].ChangedFrom != "Combo / Infinite" || events[2].Archetype != "Storm" {
		t.Errorf("v3 reclassification expected Combo → Storm; got from=%q to=%q",
			events[2].ChangedFrom, events[2].Archetype)
	}
}

func TestBuildArchetypeHistory_SkipsVersionsWithoutArchetype(t *testing.T) {
	h, _, decksDir := setupHistoryTest(t)

	t0 := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)

	// Register WITHOUT stamping archetype — simulates a fresh
	// version where runFreya hasn't completed.
	dag, _ := versioning.LoadDAG(filepath.Join(decksDir, ".versions"))
	node := dag.RegisterVersion("alice", "krenko", "Krenko, Mob Boss",
		[]string{"Sol Ring"})
	node.CreatedAt = t0.Format(time.RFC3339)
	versioning.SaveDAG(filepath.Join(decksDir, ".versions"), dag)

	events := buildArchetypeHistory(decksDir, "alice", "krenko", h.db, context.Background())
	for _, e := range events {
		if e.Kind == "version_archetype" {
			t.Errorf("version without archetype should be skipped; got %+v", e)
		}
	}
}

func TestBuildArchetypeHistory_MergesUserFeedbackChronologically(t *testing.T) {
	h, mux, decksDir := setupHistoryTest(t)

	// Strategy file so the feedback handler's snapshot has data.
	strategyDir := filepath.Join(decksDir, "alice", "freya")
	os.MkdirAll(strategyDir, 0o755)
	b, _ := json.Marshal(map[string]any{"primary_archetype": "Combo / Infinite"})
	os.WriteFile(filepath.Join(strategyDir, "krenko.strategy.json"), b, 0o644)

	t0 := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	seedVersion(t, decksDir, "alice", "krenko", "Combo / Infinite", t0,
		[]string{"Sol Ring"})

	// Fire two real feedback POSTs — these land in the log.
	for _, body := range []string{
		`{"action":"confirm"}`,
		`{"action":"correct","archetype":"Stax"}`,
	} {
		req := httptest.NewRequest("POST", "/api/decks/alice/krenko/archetype-feedback",
			strings.NewReader(body))
		req.Header.Set("X-HexDek-Owner", "alice")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("post %q: %d", body, rec.Code)
		}
	}

	events := buildArchetypeHistory(decksDir, "alice", "krenko", h.db, context.Background())

	// 1 version event + 2 user events = 3 total.
	if len(events) != 3 {
		t.Fatalf("want 3 events (1 version + 2 user); got %d (%+v)", len(events), events)
	}
	// Version event sorts first (created at t0; user events recorded later).
	if events[0].Kind != "version_archetype" {
		t.Errorf("position 0 should be version event; got %+v", events[0])
	}
	// User events follow in confirm-then-correct order.
	if events[1].Kind != "user_confirm" || events[2].Kind != "user_correct" {
		t.Errorf("user events out of order: got %s, %s", events[1].Kind, events[2].Kind)
	}
	// Correct event carries the user's override value.
	if events[2].UserArchetype != "stax" {
		t.Errorf("user_correct UserArchetype: want stax, got %q", events[2].UserArchetype)
	}
}

func TestBuildArchetypeHistory_NoVersionsReturnsEmpty(t *testing.T) {
	h, _, decksDir := setupHistoryTest(t)
	// Deck file exists but no DAG written.
	events := buildArchetypeHistory(decksDir, "alice", "krenko", h.db, context.Background())
	if len(events) != 0 {
		t.Errorf("no-DAG case should return empty; got %d events", len(events))
	}
}

// ───── handler contract ─────

func TestHandleArchetypeHistory_PublicReadSuccess(t *testing.T) {
	h, mux, decksDir := setupHistoryTest(t)
	seedVersion(t, decksDir, "alice", "krenko", "Combo / Infinite",
		time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC),
		[]string{"Sol Ring"})

	req := httptest.NewRequest("GET", "/api/decks/alice/krenko/archetype-history", nil)
	// No X-HexDek-Owner — public read.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Events []ArchetypeHistoryEvent `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Events) != 1 || resp.Events[0].Archetype != "Combo / Infinite" {
		t.Errorf("unexpected events: %+v", resp.Events)
	}
	if resp.Events[0].TimeRFC3339 == "" {
		t.Errorf("TimeRFC3339 should populate for non-zero timestamps")
	}
	_ = h
}

func TestHandleArchetypeHistory_404OnUnknownDeck(t *testing.T) {
	_, mux, _ := setupHistoryTest(t)
	req := httptest.NewRequest("GET", "/api/decks/alice/no-such-deck/archetype-history", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("unknown deck: want 404, got %d", rec.Code)
	}
}

func TestHandleArchetypeHistory_400OnPathTraversalAttempt(t *testing.T) {
	_, mux, _ := setupHistoryTest(t)
	// `..` is the canonical path-traversal token validatePathComponent
	// rejects; any other "weird-looking but valid" string falls
	// through to findDeckFile and ends up as 404 instead.
	req := httptest.NewRequest("GET", "/api/decks/../krenko/archetype-history", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	// Go's stdlib mux folds ".." path components, so the path may
	// reach validatePathComponent as "../krenko" or the mux may
	// redirect first. Either a 400 or a 301/3xx is acceptable as
	// long as we don't 200 + leak data.
	if rec.Code == 200 {
		t.Errorf("path traversal must not 200; got %d", rec.Code)
	}
}

// Empty DAG + empty log: empty events array (NOT null). Frontends
// iterate without nil-guarding.
func TestHandleArchetypeHistory_EmptyReturnsNonNilArray(t *testing.T) {
	_, mux, _ := setupHistoryTest(t)
	req := httptest.NewRequest("GET", "/api/decks/alice/krenko/archetype-history", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status: %d", rec.Code)
	}
	body := rec.Body.String()
	// Must contain `"events":[]` (possibly with whitespace from indent).
	if !strings.Contains(body, `"events": []`) && !strings.Contains(body, `"events":[]`) {
		t.Errorf("empty events should serialize as non-null array; got: %s", body)
	}
}

// ───── versioning.UpdateArchetype unit pin ─────

func TestUpdateArchetype_NoOpOnUnknownHash(t *testing.T) {
	dag := versioning.NewDeckDAG()
	// Should not panic and the empty DAG stays empty.
	dag.UpdateArchetype("nonexistent-hash", "Storm")
	if len(dag.Nodes) != 0 {
		t.Errorf("UpdateArchetype on unknown hash should be no-op; got %d nodes", len(dag.Nodes))
	}
}

func TestUpdateArchetype_StampsKnownHash(t *testing.T) {
	dag := versioning.NewDeckDAG()
	node := dag.RegisterVersion("alice", "krenko", "Krenko, Mob Boss",
		[]string{"Sol Ring"})
	if node.Archetype != "" {
		t.Errorf("fresh node should have empty archetype; got %q", node.Archetype)
	}
	dag.UpdateArchetype(node.Hash, "Combo / Infinite")
	if node.Archetype != "Combo / Infinite" {
		t.Errorf("UpdateArchetype should stamp the node; got %q", node.Archetype)
	}
}
