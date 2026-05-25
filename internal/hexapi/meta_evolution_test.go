package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/heimdall"
)

// meta_evolution_test.go — integration test for /api/meta/evolution.

func TestHandleMetaEvolution_NoDB(t *testing.T) {
	h := &Handler{DecksDir: t.TempDir()}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/meta/evolution?weeks=4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.MetaEvolution
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Weeks != 4 || out.TotalGames != 0 {
		t.Errorf("envelope: %+v", out)
	}
	if out.Archetypes == nil || out.BiggestPlayRateGainers == nil ||
		out.BiggestPlayRateLosers == nil || out.BiggestWinRateGainers == nil ||
		out.BiggestWinRateLosers == nil {
		t.Errorf("nil slices: %+v", out)
	}
}

// TestHandleMetaEvolution_HappyPath seeds games where one archetype's
// PLAY rate climbs and another's WIN rate falls, then verifies the
// movers lists pick them up.
func TestHandleMetaEvolution_HappyPath(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)
	mustWriteArchetype(t, decksDir, "alice", "control_deck", "control")
	mustWriteArchetype(t, decksDir, "bob", "combo_deck", "combo")
	// Backing decks for the other two seats — no archetype mapping
	// so they bucket as "unknown" and don't compete with control/
	// combo in the gainers/losers lists.
	mustWriteArchetype(t, decksDir, "carol", "x", "midrange")
	mustWriteArchetype(t, decksDir, "dave", "y", "voltron")

	now := time.Now().Unix()
	weekSec := int64(7 * 24 * 3600)
	priorTs := now - 3*weekSec + 100
	recentTs := now - 1*weekSec - 100

	// Prior half: 5 control games + 10 combo games (combo dominates,
	// combo wins most). Recent half: 10 control + 5 combo (flipped).
	// Use seedGame which seats 4 distinct players per game.
	for i := 0; i < 5; i++ {
		// seat 0 (alice / control) wins
		seedGame(t, h, priorTs+int64(i), 0,
			[4]string{"AC", "BC", "CC", "DC"},
			[4]string{"alice", "bob", "carol", "dave"},
			[4]string{"control_deck", "combo_deck", "x", "y"})
	}
	for i := 0; i < 10; i++ {
		// seat 1 (bob / combo) wins — combo dominates prior half
		seedGame(t, h, priorTs+int64(100+i), 1,
			[4]string{"AC", "BC", "CC", "DC"},
			[4]string{"alice", "bob", "carol", "dave"},
			[4]string{"control_deck", "combo_deck", "x", "y"})
	}
	for i := 0; i < 10; i++ {
		// recent half: control wins more
		seedGame(t, h, recentTs+int64(i), 0,
			[4]string{"AC", "BC", "CC", "DC"},
			[4]string{"alice", "bob", "carol", "dave"},
			[4]string{"control_deck", "combo_deck", "x", "y"})
	}
	for i := 0; i < 5; i++ {
		// recent: combo loses to seat 2 (carol / midrange) to drag
		// combo's recent winrate way down
		seedGame(t, h, recentTs+int64(100+i), 2,
			[4]string{"AC", "BC", "CC", "DC"},
			[4]string{"alice", "bob", "carol", "dave"},
			[4]string{"control_deck", "combo_deck", "x", "y"})
	}

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/meta/evolution?weeks=4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.MetaEvolution
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// 30 games × 4 seats = 120 seat-rows.
	if out.TotalGames != 120 {
		t.Errorf("TotalGames = %d, want 120", out.TotalGames)
	}
	// Combo wins fell from prior to recent → should appear in
	// BiggestWinRateLosers.
	foundCombo := false
	for _, m := range out.BiggestWinRateLosers {
		if m.Archetype == "combo" {
			foundCombo = true
			if m.WinRateDirection != "down" {
				t.Errorf("combo win-rate dir = %q, want 'down'", m.WinRateDirection)
			}
			break
		}
	}
	if !foundCombo {
		t.Errorf("combo not in BiggestWinRateLosers: %+v", out.BiggestWinRateLosers)
	}
	// Control wins climbed → should appear in BiggestWinRateGainers.
	foundControl := false
	for _, m := range out.BiggestWinRateGainers {
		if m.Archetype == "control" {
			foundControl = true
			break
		}
	}
	if !foundControl {
		t.Errorf("control not in BiggestWinRateGainers: %+v", out.BiggestWinRateGainers)
	}
}

func TestHandleMetaEvolution_WeeksParam(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)

	cases := []struct {
		query string
		want  int
	}{
		{"?weeks=2", 2},
		{"?weeks=99", 26},
		{"?weeks=0", 4},
		{"?weeks=abc", 4},
		{"", 4},
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", "/api/meta/evolution"+c.query, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", c.query, rec.Code)
		}
		var out heimdall.MetaEvolution
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: decode %v", c.query, err)
		}
		if out.Weeks != c.want {
			t.Errorf("%s: Weeks = %d, want %d", c.query, out.Weeks, c.want)
		}
	}
}
