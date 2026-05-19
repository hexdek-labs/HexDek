package hexapi

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	hexdb "github.com/hexdek/hexdek/internal/db"
)

func newTestDeckCardStatsHandler(t *testing.T) (*Handler, *httptest.Server, func()) {
	t.Helper()
	tmp := t.TempDir()
	db, err := hexdb.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := hexdb.EnsureCardStatsSchema(context.Background(), db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(filepath.Join(decksDir, "josh"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	deck := `COMMANDER: Breya, Etherium Shaper
1 Sol Ring
1 Brainstorm
1 Counterspell
1 Mountain
1 Krark-Clan Ironworks
`
	if err := os.WriteFile(filepath.Join(decksDir, "josh", "breya.txt"), []byte(deck), 0o644); err != nil {
		t.Fatalf("write deck: %v", err)
	}

	h := &Handler{db: db, DecksDir: decksDir}
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	cleanup := func() {
		srv.Close()
		db.Close()
	}
	return h, srv, cleanup
}

// TestDeckCardStats_RanksByDeltaAndIntersectsDeck seeds a pool where
// Sol Ring wins big, Brainstorm wins moderately, Counterspell loses,
// and "Some Stranger" (not in the deck) wins everything. The endpoint
// must return only the three cards present in the deck, sorted by
// win-rate-above-baseline.
func TestDeckCardStats_RanksByDeltaAndIntersectsDeck(t *testing.T) {
	h, srv, cleanup := newTestDeckCardStatsHandler(t)
	defer cleanup()
	ctx := context.Background()

	// Build a pool large enough to trip deckCardStatsBaselineMin (50)
	// so the response reports a data-driven baseline.
	// Sol Ring: 30 games, 18 wins (60%)
	// Brainstorm: 20 games, 7 wins (35%)
	// Counterspell: 15 games, 1 win (~6.7%)
	// Some Stranger: 40 games, 30 wins — should NOT appear (not in deck).
	feeds := [][]hexdb.CardStatDelta{}
	push := func(name string, wins, losses int) {
		for i := 0; i < wins; i++ {
			feeds = append(feeds, []hexdb.CardStatDelta{{CardName: name, Win: 1}})
		}
		for i := 0; i < losses; i++ {
			feeds = append(feeds, []hexdb.CardStatDelta{{CardName: name, Loss: 1}})
		}
	}
	push("Sol Ring", 18, 12)
	push("Brainstorm", 7, 13)
	push("Counterspell", 1, 14)
	push("Some Stranger", 30, 10)
	// Mountain in the deck file but won't get any seeded data — proves
	// the handler skips below-min-games entries.
	push("Mountain", 0, 0)

	for _, f := range feeds {
		if err := hexdb.BatchUpsertCardStats(ctx, h.db, f); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	resp, err := http.Get(srv.URL + "/api/deck-card-stats/josh/breya")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, string(body))
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["owner"] != "josh" || body["id"] != "breya" {
		t.Errorf("owner/id mismatch: %v / %v", body["owner"], body["id"])
	}
	if body["commander"] != "Breya, Etherium Shaper" {
		t.Errorf("commander=%v", body["commander"])
	}

	rawCards, _ := body["cards"].([]any)
	if len(rawCards) != 3 {
		t.Fatalf("expected 3 cards (Sol Ring, Brainstorm, Counterspell), got %d: %v", len(rawCards), rawCards)
	}

	names := make([]string, len(rawCards))
	for i, raw := range rawCards {
		e, _ := raw.(map[string]any)
		names[i] = e["card_name"].(string)
	}
	if names[0] != "Sol Ring" || names[1] != "Brainstorm" || names[2] != "Counterspell" {
		t.Errorf("ranking mismatch, got %v", names)
	}

	first, _ := rawCards[0].(map[string]any)
	if g := first["games"].(float64); g != 30 {
		t.Errorf("Sol Ring games=%v want 30", g)
	}
	if w := first["wins"].(float64); w != 18 {
		t.Errorf("Sol Ring wins=%v want 18", w)
	}
	if losses := first["losses"].(float64); losses != 12 {
		t.Errorf("Sol Ring losses=%v want 12", losses)
	}
	if math.Abs(first["win_rate"].(float64)-0.6) > 1e-6 {
		t.Errorf("Sol Ring win_rate=%v want 0.6", first["win_rate"])
	}

	baseline := body["baseline_win_rate"].(float64)
	if baseline <= 0 || baseline >= 1 {
		t.Errorf("baseline out of (0,1): %v", baseline)
	}
	expectedDelta := first["win_rate"].(float64) - baseline
	if math.Abs(first["win_rate_delta"].(float64)-expectedDelta) > 1e-6 {
		t.Errorf("win_rate_delta=%v want %v", first["win_rate_delta"], expectedDelta)
	}

	// Counterspell sits below baseline — its delta must be negative.
	last, _ := rawCards[2].(map[string]any)
	if last["win_rate_delta"].(float64) >= 0 {
		t.Errorf("Counterspell expected negative delta, got %v", last["win_rate_delta"])
	}
}

// TestDeckCardStats_ExcludesCommander confirms the commander is filtered
// from the results even when it has card_stats coverage.
func TestDeckCardStats_ExcludesCommander(t *testing.T) {
	h, srv, cleanup := newTestDeckCardStatsHandler(t)
	defer cleanup()
	ctx := context.Background()

	// Seed enough Breya games to be eligible, plus a few other cards
	// so the pool clears the baseline-min threshold.
	deltas := []hexdb.CardStatDelta{}
	for i := 0; i < 20; i++ {
		deltas = append(deltas, hexdb.CardStatDelta{CardName: "Breya, Etherium Shaper", Win: 1})
	}
	for i := 0; i < 15; i++ {
		deltas = append(deltas, hexdb.CardStatDelta{CardName: "Sol Ring", Win: 1})
	}
	for i := 0; i < 15; i++ {
		deltas = append(deltas, hexdb.CardStatDelta{CardName: "Sol Ring", Loss: 1})
	}
	for i := 0; i < 20; i++ {
		deltas = append(deltas, hexdb.CardStatDelta{CardName: "Brainstorm", Loss: 1})
	}
	if err := hexdb.BatchUpsertCardStats(ctx, h.db, deltas); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp, err := http.Get(srv.URL + "/api/deck-card-stats/josh/breya")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)

	for _, raw := range body["cards"].([]any) {
		e, _ := raw.(map[string]any)
		if name := e["card_name"].(string); name == "Breya, Etherium Shaper" {
			t.Fatalf("commander leaked into results")
		}
	}
}

// TestDeckCardStats_NoStatsYet returns 200 with empty cards and the
// fallback 25% baseline when card_stats is empty.
func TestDeckCardStats_NoStatsYet(t *testing.T) {
	_, srv, cleanup := newTestDeckCardStatsHandler(t)
	defer cleanup()

	resp, err := http.Get(srv.URL + "/api/deck-card-stats/josh/breya")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)

	cards := body["cards"].([]any)
	if len(cards) != 0 {
		t.Errorf("expected empty cards on cold start, got %d", len(cards))
	}
	if math.Abs(body["baseline_win_rate"].(float64)-0.25) > 1e-6 {
		t.Errorf("baseline=%v want fallback 0.25", body["baseline_win_rate"])
	}
	if body["matched_cards"].(float64) != 0 {
		t.Errorf("matched_cards=%v want 0", body["matched_cards"])
	}
}

// TestDeckCardStats_DeckNotFound returns 404 for an unknown deck.
func TestDeckCardStats_DeckNotFound(t *testing.T) {
	_, srv, cleanup := newTestDeckCardStatsHandler(t)
	defer cleanup()

	resp, err := http.Get(srv.URL + "/api/deck-card-stats/josh/does-not-exist")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// TestDeckCardStats_InvalidPath rejects path-traversal / bad characters.
func TestDeckCardStats_InvalidPath(t *testing.T) {
	_, srv, cleanup := newTestDeckCardStatsHandler(t)
	defer cleanup()

	resp, err := http.Get(srv.URL + "/api/deck-card-stats/josh/..%2Fetc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 && resp.StatusCode != 404 {
		t.Fatalf("expected 400 or 404 for bad id, got %d", resp.StatusCode)
	}
}

// TestDeckCardStats_RespectsMinGames suppresses cards below the
// deckCardStatsMinGames floor so the widget never shows a 100%-win-rate
// card with 1 sample.
func TestDeckCardStats_RespectsMinGames(t *testing.T) {
	h, srv, cleanup := newTestDeckCardStatsHandler(t)
	defer cleanup()
	ctx := context.Background()

	// Sol Ring has just 2 wins — below the 5-game floor.
	// Brainstorm has 10 games — above the floor.
	deltas := []hexdb.CardStatDelta{
		{CardName: "Sol Ring", Win: 1},
		{CardName: "Sol Ring", Win: 1},
	}
	for i := 0; i < 5; i++ {
		deltas = append(deltas, hexdb.CardStatDelta{CardName: "Brainstorm", Win: 1})
	}
	for i := 0; i < 5; i++ {
		deltas = append(deltas, hexdb.CardStatDelta{CardName: "Brainstorm", Loss: 1})
	}
	if err := hexdb.BatchUpsertCardStats(ctx, h.db, deltas); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp, err := http.Get(srv.URL + "/api/deck-card-stats/josh/breya")
	if err != nil {
		t.Fatalf("GET deck-card-stats: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)

	for _, raw := range body["cards"].([]any) {
		e, _ := raw.(map[string]any)
		if e["card_name"].(string) == "Sol Ring" {
			t.Fatalf("Sol Ring with only 2 games should have been suppressed")
		}
	}
}
