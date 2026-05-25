package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/heimdall"
)

// archetype_matrix_test.go — integration tests for
// GET /api/meta/archetype-vs-archetype. Reuses newVersionTrendsEnv
// + insertDeckGame + writeStrategyFile from the existing test
// helpers.

func matrixArchIdx(m heimdall.ArchetypeMatrix, name string) int {
	for i, a := range m.Archetypes {
		if a == name {
			return i
		}
	}
	return -1
}

// TestHandleArchetypeMatrix_NoDB returns a well-formed empty 22×22
// matrix when the showmatch DB isn't wired.
func TestHandleArchetypeMatrix_NoDB(t *testing.T) {
	h := &Handler{DecksDir: t.TempDir()}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/meta/archetype-vs-archetype", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.ArchetypeMatrix
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Archetypes) != 22 || len(out.Cells) != 22 || out.TotalCells != 22*22 {
		t.Errorf("dims: archetypes=%d cells=%d total=%d, want 22/22/484",
			len(out.Archetypes), len(out.Cells), out.TotalCells)
	}
	if out.TotalGames != 0 {
		t.Errorf("TotalGames = %d, want 0", out.TotalGames)
	}
}

// TestHandleArchetypeMatrix_HappyPath seeds 4 decks with archetypes,
// runs a handful of 4-player games, and checks that specific cells
// reflect the observed matchup wins.
func TestHandleArchetypeMatrix_HappyPath(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)
	// Four decks, four archetypes — index aligns with seat number.
	mustWriteArchetype(t, decksDir, "alice", "ctrl", "control")
	mustWriteArchetype(t, decksDir, "bob", "combo", "combo")
	mustWriteArchetype(t, decksDir, "carol", "volt", "voltron")
	mustWriteArchetype(t, decksDir, "dave", "aris", "aristocrats")

	now := time.Now().Unix()
	// 5 games: control wins 3, combo wins 1, voltron wins 1.
	winners := []int{0, 0, 0, 1, 2}
	for i, w := range winners {
		seedFourSeatGame(t, h, int64(i+1)*100+now-60*60*24*7, w,
			[4]string{"AC", "BC", "CC", "DC"},
			[4]string{"alice", "bob", "carol", "dave"},
			[4]string{"ctrl", "combo", "volt", "aris"})
	}

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/meta/archetype-vs-archetype?weeks=4", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.ArchetypeMatrix
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.TotalGames != 5 {
		t.Errorf("TotalGames = %d, want 5", out.TotalGames)
	}
	ic := matrixArchIdx(out, "control")
	icc := matrixArchIdx(out, "combo")
	iv := matrixArchIdx(out, "voltron")

	// control × combo: 5 games (each game has both at the table),
	// control won 3.
	c := out.Cells[ic][icc]
	if c.Games != 5 || c.Wins != 3 {
		t.Errorf("cells[control][combo] = %+v, want 5g/3w", c)
	}
	// combo × control: same 5 games, but combo only won 1.
	c = out.Cells[icc][ic]
	if c.Games != 5 || c.Wins != 1 {
		t.Errorf("cells[combo][control] = %+v, want 5g/1w", c)
	}
	// voltron × control: 5 games, voltron won 1.
	c = out.Cells[iv][ic]
	if c.Games != 5 || c.Wins != 1 {
		t.Errorf("cells[voltron][control] = %+v, want 5g/1w", c)
	}
	// aristocrats × control: 5 games, aristocrats won 0.
	ia := matrixArchIdx(out, "aristocrats")
	c = out.Cells[ia][ic]
	if c.Games != 5 || c.Wins != 0 {
		t.Errorf("cells[aristocrats][control] = %+v, want 5g/0w", c)
	}
}

// TestHandleArchetypeMatrix_WeeksParam pins the ?weeks parsing +
// clamping (default 12, max 52).
func TestHandleArchetypeMatrix_WeeksParam(t *testing.T) {
	h, _ := newVersionTrendsEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)

	// We can't directly observe the weeks param in the response, but
	// we CAN verify that requests with crazy values still succeed
	// (200 + parseable body).
	for _, q := range []string{"", "?weeks=4", "?weeks=999", "?weeks=0", "?weeks=abc"} {
		req := httptest.NewRequest("GET", "/api/meta/archetype-vs-archetype"+q, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", q, rec.Code)
		}
		var out heimdall.ArchetypeMatrix
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Errorf("%s: decode %v", q, err)
		}
	}
}

// TestHandleArchetypeMatrix_OutsideWindowExcluded pins that games
// older than the rolling window aren't counted.
func TestHandleArchetypeMatrix_OutsideWindowExcluded(t *testing.T) {
	h, decksDir := newVersionTrendsEnv(t)
	mustWriteArchetype(t, decksDir, "alice", "ctrl", "control")
	mustWriteArchetype(t, decksDir, "bob", "combo", "combo")
	mustWriteArchetype(t, decksDir, "carol", "volt", "voltron")
	mustWriteArchetype(t, decksDir, "dave", "aris", "aristocrats")

	now := time.Now().Unix()
	weekSec := int64(7 * 24 * 3600)

	// One in-window game (last week)
	seedFourSeatGame(t, h, now-1*weekSec, 0,
		[4]string{"AC", "BC", "CC", "DC"},
		[4]string{"alice", "bob", "carol", "dave"},
		[4]string{"ctrl", "combo", "volt", "aris"})
	// One out-of-window game (20 weeks ago — falls outside the
	// default 12-week window)
	seedFourSeatGame(t, h, now-20*weekSec, 1,
		[4]string{"AC", "BC", "CC", "DC"},
		[4]string{"alice", "bob", "carol", "dave"},
		[4]string{"ctrl", "combo", "volt", "aris"})

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/meta/archetype-vs-archetype?weeks=12", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out heimdall.ArchetypeMatrix
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.TotalGames != 1 {
		t.Errorf("TotalGames = %d, want 1 (out-of-window game excluded)", out.TotalGames)
	}
}

// seedFourSeatGame is the 4-seat variant of insertDeckGame so the
// matrix tests can place all four players in the same game.
func seedFourSeatGame(t *testing.T, h *Handler, finishedAt int64, winnerSeat int, commanders, owners, deckIDs [4]string) {
	t.Helper()
	seedGame(t, h, finishedAt, winnerSeat, commanders, owners, deckIDs)
}
