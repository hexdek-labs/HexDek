package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/versioning"
)

// deck_version_trends_r60_test.go — HTTP-level integration test for
// GET /api/decks/{owner}/{id}/version-trends. Exercises the DAG file
// load + DB outcome query + heimdall computation pipeline end to end.

func newVersionTrendsEnv(t *testing.T) (*Handler, string) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	decksDir := t.TempDir()

	h := &Handler{
		DecksDir: decksDir,
		Showmatch: &Showmatch{
			sqlDB: d,
		},
	}
	return h, decksDir
}

// registerDAGVersion injects a node into the deck DAG with a specific
// CreatedAt time. The real RegisterVersion stamps time.Now(); for
// trend testing we want deterministic boundaries.
func registerDAGVersion(t *testing.T, decksDir, owner, deckID, commander string, cards []string, createdAt time.Time) {
	t.Helper()
	dagDir := filepath.Join(decksDir, ".versions")
	dag, err := versioning.LoadDAG(dagDir)
	if err != nil {
		t.Fatalf("load dag: %v", err)
	}
	node := dag.RegisterVersion(owner, deckID, commander, cards)
	node.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	if err := versioning.SaveDAG(dagDir, dag); err != nil {
		t.Fatalf("save dag: %v", err)
	}
}

// insertDeckGame stamps one game where seat 0 is owner/deckID and
// the other seats are placeholder commanders. winnerSeat=0 means our
// deck won; -1 = draw.
func insertDeckGame(t *testing.T, h *Handler, owner, deckID string, finishedAt int64, winnerSeat int) {
	t.Helper()
	ctx := context.Background()
	deckKey := owner + "/" + deckID

	// Make sure the deck_key is registered in showmatch_elo for any
	// joins that need it.
	_, err := h.Showmatch.sqlDB.ExecContext(ctx,
		`INSERT OR IGNORE INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, updated_at)
		 VALUES (?, ?, ?, 1500, 0, 0, 0, 0, 0, 0, 0, unixepoch())`,
		deckKey, "Iroh", owner)
	if err != nil {
		t.Fatalf("insert elo: %v", err)
	}

	g := db.GameRecord{
		StartedAt:  finishedAt - 600,
		FinishedAt: finishedAt,
		Turns:      8,
		Winner:     winnerSeat,
		WinnerName: "Iroh",
		EndReason:  "combat",
		Seed:       finishedAt,
	}
	seats := []db.GameSeatRecord{
		{Seat: 0, Commander: "Iroh", DeckKey: deckKey, Lost: winnerSeat != 0},
		{Seat: 1, Commander: "Atraxa", DeckKey: "other/atraxa", Lost: winnerSeat != 1},
		{Seat: 2, Commander: "Edgar", DeckKey: "other/edgar", Lost: winnerSeat != 2},
		{Seat: 3, Commander: "Krenko", DeckKey: "other/krenko", Lost: winnerSeat != 3},
	}
	if _, err := db.PersistGameTx(ctx, h.Showmatch.sqlDB, g, seats); err != nil {
		t.Fatalf("persist game: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Happy path: 3 versions, games split across eras, winrate per version.
// -----------------------------------------------------------------------------

func TestHandleDeckVersionTrends_BucketsByEra(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)

	// v1 created at t=1000 (10 cards), v2 at t=2000 (delta-2), v3 at
	// t=3000 (delta-3). Card lists differ so each registers as a new
	// version in the DAG.
	t1 := time.Unix(1000, 0)
	t2 := time.Unix(2000, 0)
	t3 := time.Unix(3000, 0)
	registerDAGVersion(t, decksDir, "alice", "iroh", "Iroh",
		[]string{"Iroh", "Plains", "Plains", "Sol Ring", "Swords to Plowshares"}, t1)
	registerDAGVersion(t, decksDir, "alice", "iroh", "Iroh",
		[]string{"Iroh", "Plains", "Plains", "Sol Ring", "Path to Exile"}, t2)
	registerDAGVersion(t, decksDir, "alice", "iroh", "Iroh",
		[]string{"Iroh", "Plains", "Plains", "Arcane Signet", "Path to Exile"}, t3)

	// v1 era: 1W 1L
	insertDeckGame(t, h, "alice", "iroh", 1200, 0)
	insertDeckGame(t, h, "alice", "iroh", 1500, 1)
	// v2 era: 2W 1L
	insertDeckGame(t, h, "alice", "iroh", 2100, 0)
	insertDeckGame(t, h, "alice", "iroh", 2300, 0)
	insertDeckGame(t, h, "alice", "iroh", 2700, 2)
	// v3 era: 1W 2L
	insertDeckGame(t, h, "alice", "iroh", 3050, 3)
	insertDeckGame(t, h, "alice", "iroh", 3200, 0)
	insertDeckGame(t, h, "alice", "iroh", 3400, 1)

	req := httptest.NewRequest(http.MethodGet, "/api/decks/alice/iroh/version-trends", nil)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "iroh")
	w := httptest.NewRecorder()
	h.handleDeckVersionTrends(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Owner    string `json:"owner"`
		DeckID   string `json:"deck_id"`
		Versions []struct {
			Version int     `json:"version"`
			Games   int     `json:"games"`
			Wins    int     `json:"wins"`
			Losses  int     `json:"losses"`
			Draws   int     `json:"draws"`
			WinRate float64 `json:"win_rate"`
		} `json:"versions"`
		UntrackedGames int `json:"untracked_games"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if got.Owner != "alice" || got.DeckID != "iroh" {
		t.Errorf("owner/deck = %q/%q, want alice/iroh", got.Owner, got.DeckID)
	}
	if len(got.Versions) != 3 {
		t.Fatalf("expected 3 versions, got %d (%+v)", len(got.Versions), got.Versions)
	}
	checks := []struct {
		ver, games, wins, losses int
	}{
		{1, 2, 1, 1},
		{2, 3, 2, 1},
		{3, 3, 1, 2},
	}
	for i, c := range checks {
		v := got.Versions[i]
		if v.Version != c.ver || v.Games != c.games || v.Wins != c.wins || v.Losses != c.losses {
			t.Errorf("Versions[%d] = %+v, want v%d games=%d wins=%d losses=%d",
				i, v, c.ver, c.games, c.wins, c.losses)
		}
	}
	if got.UntrackedGames != 0 {
		t.Errorf("UntrackedGames = %d, want 0", got.UntrackedGames)
	}
}

// -----------------------------------------------------------------------------
// No version history yet → empty payload, no 404.
// -----------------------------------------------------------------------------

func TestHandleDeckVersionTrends_NoVersionsYet(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/decks/alice/iroh/version-trends", nil)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "iroh")
	w := httptest.NewRecorder()
	h.handleDeckVersionTrends(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (empty state, not 404)", w.Code)
	}
	var got struct {
		Versions []any `json:"versions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Versions) != 0 {
		t.Errorf("expected empty versions list, got %d", len(got.Versions))
	}
}

// -----------------------------------------------------------------------------
// Games predating the first version land in untracked_games.
// -----------------------------------------------------------------------------

func TestHandleDeckVersionTrends_PreVersionGamesUntracked(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)

	registerDAGVersion(t, decksDir, "alice", "iroh", "Iroh",
		[]string{"Iroh", "Plains", "Sol Ring"}, time.Unix(5000, 0))

	insertDeckGame(t, h, "alice", "iroh", 100, 0)   // way before v1
	insertDeckGame(t, h, "alice", "iroh", 4999, 1)  // just before v1
	insertDeckGame(t, h, "alice", "iroh", 6000, 0)  // in v1 era

	req := httptest.NewRequest(http.MethodGet, "/api/decks/alice/iroh/version-trends", nil)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "iroh")
	w := httptest.NewRecorder()
	h.handleDeckVersionTrends(w, req)

	var got struct {
		Versions []struct {
			Games int `json:"games"`
			Wins  int `json:"wins"`
		} `json:"versions"`
		UntrackedGames int `json:"untracked_games"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if got.UntrackedGames != 2 {
		t.Errorf("UntrackedGames = %d, want 2", got.UntrackedGames)
	}
	if got.Versions[0].Games != 1 || got.Versions[0].Wins != 1 {
		t.Errorf("v1 should have 1W, got %+v", got.Versions[0])
	}
}

// -----------------------------------------------------------------------------
// Invalid path components rejected.
// -----------------------------------------------------------------------------

func TestHandleDeckVersionTrends_InvalidPathRejected(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/decks/../etc/iroh/version-trends", nil)
	req.SetPathValue("owner", "../etc")
	req.SetPathValue("id", "iroh")
	w := httptest.NewRecorder()
	h.handleDeckVersionTrends(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 on invalid path", w.Code)
	}
}
