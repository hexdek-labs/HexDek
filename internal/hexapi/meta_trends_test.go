package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/heimdall"
)

// meta_trends_test.go — integration test for GET /api/meta/trends.
// Seeds games via insertDeckGame (from deck_version_trends_r60_test.go)
// and Freya strategy files via writeStrategyFile (from
// deck_archive_test.go), then asserts the assembled MetaTrends shape.

func mustWriteArchetype(t *testing.T, decksDir, owner, deckID, archetype string) {
	t.Helper()
	writeStrategyFile(t, decksDir, owner, deckID, archetype, "Some Commander")
}

// TestHandleMetaTrends_NoDB returns a well-formed empty envelope when
// the showmatch DB isn't wired (test handler without h.Showmatch).
func TestHandleMetaTrends_NoDB(t *testing.T) {
	h := &Handler{DecksDir: t.TempDir()}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/meta/trends?weeks=4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.MetaTrends
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Weeks != 4 || out.TotalGames != 0 {
		t.Errorf("envelope: %+v", out)
	}
	if out.Archetypes == nil || out.BiggestGainers == nil || out.BiggestLosers == nil {
		t.Errorf("nil slices: %+v", out)
	}
}

// TestHandleMetaTrends_HappyPath seeds a handful of games across two
// archetypes (control, combo) with one trending up and one trending
// down, then asserts the response surfaces both via the gainers/
// losers lists.
func TestHandleMetaTrends_HappyPath(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)

	// Register two decks with archetypes.
	mustWriteArchetype(t, decksDir, "alice", "control_deck", "control")
	mustWriteArchetype(t, decksDir, "bob", "combo_deck", "combo")

	// 4-week window backward from now. Place games into specific
	// weeks by subtracting (week_idx_back * 7d) from the current
	// time. We want enough samples per side for the 10-game floor.
	now := time.Now().Unix()
	weekSec := int64(7 * 24 * 3600)

	// "control" — trends DOWN: 10 prior wins, 0 prior losses
	// (high WR in prior half), then 0 recent wins, 10 recent losses
	// (low WR in recent half).
	// Prior half = weeks 0+1 (oldest), Recent half = weeks 2+3.
	priorTs := now - 3*weekSec + 100 // bucket "old" (week 0)
	recentTs := now - 1*weekSec - 100 // bucket "recent" (week 3)
	for i := 0; i < 10; i++ {
		insertSeededGame(t, h, "alice", "control_deck", priorTs+int64(i), 0) // win
	}
	for i := 0; i < 10; i++ {
		insertSeededGame(t, h, "alice", "control_deck", recentTs+int64(i), 1) // loss
	}

	// "combo" — trends UP: 0 prior wins, 10 prior losses; 10 recent
	// wins, 0 recent losses.
	for i := 0; i < 10; i++ {
		insertSeededGame(t, h, "bob", "combo_deck", priorTs+int64(100+i), 1) // loss
	}
	for i := 0; i < 10; i++ {
		insertSeededGame(t, h, "bob", "combo_deck", recentTs+int64(100+i), 0) // win
	}

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/meta/trends?weeks=4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.MetaTrends
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// 40 inserted games × 4 seats each = 160 seat-outcome rows.
	// MetaTrends counts seat outcomes (winrate is a per-seat metric).
	if out.TotalGames != 160 {
		t.Errorf("TotalGames (seat outcomes) = %d, want 160", out.TotalGames)
	}
	// Of those 160: control gets 20 (10+10), combo gets 20 (10+10),
	// the 3 placeholder seats per game (other/atraxa, other/edgar,
	// other/krenko) account for the remaining 120 — they have
	// deck_keys but no strategy file → bucket "unknown".
	for _, a := range out.Archetypes {
		switch a.Archetype {
		case "control", "combo":
			if a.TotalGames != 20 {
				t.Errorf("%s TotalGames = %d, want 20", a.Archetype, a.TotalGames)
			}
		}
	}
	// Locate both archetypes.
	var control, combo *heimdall.ArchetypeMetaTrend
	for i := range out.Archetypes {
		if out.Archetypes[i].Archetype == "control" {
			control = &out.Archetypes[i]
		}
		if out.Archetypes[i].Archetype == "combo" {
			combo = &out.Archetypes[i]
		}
	}
	if control == nil || combo == nil {
		t.Fatalf("missing archetype rows; got %+v", out.Archetypes)
	}
	if control.Direction != "down" {
		t.Errorf("control direction = %q, want 'down' (Δ=%.3f)", control.Direction, control.Delta)
	}
	if combo.Direction != "up" {
		t.Errorf("combo direction = %q, want 'up' (Δ=%.3f)", combo.Direction, combo.Delta)
	}
	// Combo should top the gainers list; control should top the
	// losers list.
	if len(out.BiggestGainers) == 0 || out.BiggestGainers[0].Archetype != "combo" {
		t.Errorf("BiggestGainers[0] = %+v, want 'combo'", out.BiggestGainers)
	}
	if len(out.BiggestLosers) == 0 || out.BiggestLosers[0].Archetype != "control" {
		t.Errorf("BiggestLosers[0] = %+v, want 'control'", out.BiggestLosers)
	}
}

// TestHandleMetaTrends_WeeksParam pins ?weeks=N parsing + clamping.
func TestHandleMetaTrends_WeeksParam(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)

	cases := []struct {
		query string
		want  int
	}{
		{"?weeks=2", 2},
		{"?weeks=99", 26},  // clamped to max
		{"?weeks=0", 4},    // falls back to default
		{"?weeks=abc", 4},  // unparseable → default
		{"", 4},            // missing → default
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", "/api/meta/trends"+c.query, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", c.query, rec.Code)
		}
		var out heimdall.MetaTrends
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: decode %v", c.query, err)
		}
		if out.Weeks != c.want {
			t.Errorf("%s: Weeks = %d, want %d", c.query, out.Weeks, c.want)
		}
	}
}

// insertSeededGame is a thin wrapper around the deck_version_trends
// test helper that accepts arbitrary game keys without forcing the
// same deck on every seat.
func insertSeededGame(t *testing.T, h *Handler, owner, deckID string, finishedAt int64, winnerSeat int) {
	t.Helper()
	insertDeckGame(t, h, owner, deckID, finishedAt, winnerSeat)
}
