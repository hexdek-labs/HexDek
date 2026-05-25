package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/heimdall"
)

// deck_archive_test.go — integration tests for
// GET /api/decks/{owner}/{id}/archive. Reuses newVersionTrendsEnv +
// registerDAGVersion + insertDeckGame from deck_version_trends_r60_test.go
// to seed the DAG and game history.

func writeStrategyFile(t *testing.T, decksDir, owner, deckID, archetype, commander string) {
	t.Helper()
	dir := filepath.Join(decksDir, owner, "freya")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir freya: %v", err)
	}
	body := map[string]any{"archetype": archetype, "commander_name": commander}
	data, _ := json.Marshal(body)
	if err := os.WriteFile(filepath.Join(dir, deckID+".strategy.json"), data, 0o644); err != nil {
		t.Fatalf("write strategy: %v", err)
	}
}

func writeVersionStrategySnapshot(t *testing.T, decksDir, owner, deckID string, version int, archetype string) {
	t.Helper()
	dir := filepath.Join(decksDir, owner, "versions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir versions: %v", err)
	}
	body := map[string]any{"archetype": archetype}
	data, _ := json.Marshal(body)
	path := filepath.Join(dir, deckID+"_v"+intToStr(int64(version))+".strategy.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write version strategy: %v", err)
	}
}

// TestHandleDeckArchive_HappyPath pins the end-to-end shape: identity,
// version rollup (matching version-trends numbers), aggregate game
// stats, recent-games newest-first, current archetype embedded.
func TestHandleDeckArchive_HappyPath(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)
	writeStrategyFile(t, decksDir, "alice", "krenko", "tribal", "Krenko, Mob Boss")

	// Two versions: v1 at t=1000, v2 at t=2000.
	registerDAGVersion(t, decksDir, "alice", "krenko", "Krenko, Mob Boss",
		[]string{"Mountain", "Goblin Piledriver"}, time.Unix(1000, 0))
	registerDAGVersion(t, decksDir, "alice", "krenko", "Krenko, Mob Boss",
		[]string{"Mountain", "Goblin Piledriver", "Goblin Chieftain"}, time.Unix(2000, 0))

	insertDeckGame(t, h, "alice", "krenko", 1100, 0)  // v1 win
	insertDeckGame(t, h, "alice", "krenko", 1500, 1)  // v1 loss
	insertDeckGame(t, h, "alice", "krenko", 2100, 0)  // v2 win
	insertDeckGame(t, h, "alice", "krenko", 2500, 0)  // v2 win
	insertDeckGame(t, h, "alice", "krenko", 2700, 2)  // v2 loss

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/krenko/archive", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var arc heimdall.DeckArchive
	if err := json.Unmarshal(rec.Body.Bytes(), &arc); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, rec.Body.String())
	}
	if arc.Owner != "alice" || arc.DeckID != "krenko" || arc.DeckKey != "alice/krenko" {
		t.Errorf("identity: %+v", arc)
	}
	if arc.CurrentCommander != "Krenko, Mob Boss" || arc.CurrentArchetype != "tribal" {
		t.Errorf("current fields: %+v", arc)
	}
	if arc.Games.Total != 5 || arc.Games.Wins != 3 || arc.Games.Losses != 2 {
		t.Errorf("aggregate: %+v, want 5/3/2", arc.Games)
	}
	// Recent games: newest first.
	if len(arc.Games.Recent) != 5 || arc.Games.Recent[0].FinishedAt != 2700 {
		t.Errorf("recent: %+v", arc.Games.Recent)
	}
	// Versions: 2 rows.
	if len(arc.Versions) != 2 {
		t.Fatalf("versions: got %d, want 2", len(arc.Versions))
	}
	v1, v2 := arc.Versions[0], arc.Versions[1]
	if v1.Version != 1 || v1.Games != 2 || v1.Wins != 1 {
		t.Errorf("v1: %+v, want games=2 wins=1", v1)
	}
	if v2.Version != 2 || v2.Games != 3 || v2.Wins != 2 {
		t.Errorf("v2: %+v, want games=3 wins=2", v2)
	}
	// Latest version's archetype gets backfilled from current.
	if v2.Archetype != "tribal" {
		t.Errorf("v2 archetype should backfill from current: %q", v2.Archetype)
	}
	// Archetype evolution should have one step (only known archetype is
	// "tribal" anchored at the latest version).
	if len(arc.ArchetypeEvolution) != 1 || arc.ArchetypeEvolution[0].Archetype != "tribal" {
		t.Errorf("archetype_evolution: %+v", arc.ArchetypeEvolution)
	}
}

// TestHandleDeckArchive_PerVersionSnapshotsLightUpEvolution pins that
// when per-version strategy snapshots ARE saved (forward-compatible
// path), the archetype evolution timeline reflects each transition.
func TestHandleDeckArchive_PerVersionSnapshotsLightUpEvolution(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)
	writeStrategyFile(t, decksDir, "alice", "deck", "midrange", "Iroh")

	registerDAGVersion(t, decksDir, "alice", "deck", "Iroh",
		[]string{"Island"}, time.Unix(1000, 0))
	registerDAGVersion(t, decksDir, "alice", "deck", "Iroh",
		[]string{"Island", "Mountain"}, time.Unix(2000, 0))
	registerDAGVersion(t, decksDir, "alice", "deck", "Iroh",
		[]string{"Island", "Mountain", "Forest"}, time.Unix(3000, 0))

	writeVersionStrategySnapshot(t, decksDir, "alice", "deck", 1, "control")
	writeVersionStrategySnapshot(t, decksDir, "alice", "deck", 2, "control")
	// v3 has no snapshot — will backfill from current (midrange)

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/alice/deck/archive", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var arc heimdall.DeckArchive
	if err := json.Unmarshal(rec.Body.Bytes(), &arc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arc.Versions) != 3 {
		t.Fatalf("versions: %d, want 3", len(arc.Versions))
	}
	if arc.Versions[0].Archetype != "control" || arc.Versions[1].Archetype != "control" {
		t.Errorf("v1/v2 archetype: got %q/%q, want control/control",
			arc.Versions[0].Archetype, arc.Versions[1].Archetype)
	}
	if arc.Versions[2].Archetype != "midrange" {
		t.Errorf("v3 archetype: got %q, want midrange (backfilled)", arc.Versions[2].Archetype)
	}
	// Evolution: control (v1) → midrange (v3), 2 steps total.
	if len(arc.ArchetypeEvolution) != 2 {
		t.Fatalf("archetype_evolution: %+v, want 2 steps", arc.ArchetypeEvolution)
	}
	if arc.ArchetypeEvolution[0].Archetype != "control" || arc.ArchetypeEvolution[0].SinceVersion != 1 {
		t.Errorf("step[0]: %+v", arc.ArchetypeEvolution[0])
	}
	if arc.ArchetypeEvolution[1].Archetype != "midrange" || arc.ArchetypeEvolution[1].SinceVersion != 3 {
		t.Errorf("step[1]: %+v", arc.ArchetypeEvolution[1])
	}
}

// TestHandleDeckArchive_EmptyDeckReturnsWellFormedPayload pins the
// no-history case: 200 with non-nil empty slices, no commander/
// archetype fields.
func TestHandleDeckArchive_EmptyDeckReturnsWellFormedPayload(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/decks/nobody/nothing/archive", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var arc heimdall.DeckArchive
	if err := json.Unmarshal(rec.Body.Bytes(), &arc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if arc.Owner != "nobody" || arc.DeckID != "nothing" {
		t.Errorf("identity: %+v", arc)
	}
	if arc.Versions == nil || arc.Games.Recent == nil || arc.ArchetypeEvolution == nil {
		t.Errorf("slices should be non-nil empty: %+v", arc)
	}
	if arc.Games.Total != 0 {
		t.Errorf("Games.Total = %d, want 0", arc.Games.Total)
	}
}

// TestHandleDeckArchive_RejectsInvalidPathComponents pins 400 on
// path-traversal / non-ASCII components.
func TestHandleDeckArchive_RejectsInvalidPathComponents(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)

	for _, url := range []string{
		"/api/decks/..%2Fevil/x/archive",
		"/api/decks/owner/..%2Fbad/archive",
	} {
		req := httptest.NewRequest("GET", url, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", url, rec.Code)
		}
	}
}

// TestHandleDeckArchive_RecentLimitQueryParam pins ?recent=N and the
// max=100 cap.
func TestHandleDeckArchive_RecentLimitQueryParam(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	for ts := int64(1); ts <= 50; ts++ {
		insertDeckGame(t, h, "alice", "deck", ts, 0)
	}
	mux := http.NewServeMux()
	h.Register(mux)

	// recent=5 → 5 games.
	req := httptest.NewRequest("GET", "/api/decks/alice/deck/archive?recent=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var arc heimdall.DeckArchive
	if err := json.Unmarshal(rec.Body.Bytes(), &arc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arc.Games.Recent) != 5 {
		t.Errorf("?recent=5: got %d, want 5", len(arc.Games.Recent))
	}
	if arc.Games.Total != 50 {
		t.Errorf("Total = %d, want 50 (recent param shouldn't affect aggregate)", arc.Games.Total)
	}

	// recent=9999 → clamps to 100 (we only have 50, so 50 returned).
	req = httptest.NewRequest("GET", "/api/decks/alice/deck/archive?recent=9999", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if err := json.Unmarshal(rec.Body.Bytes(), &arc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(arc.Games.Recent) != 50 {
		t.Errorf("?recent=9999 (capped to 100, 50 games available): got %d, want 50", len(arc.Games.Recent))
	}
}
