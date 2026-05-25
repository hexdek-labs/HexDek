package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/heimdall"
)

// player_compare_test.go — integration tests for
// GET /api/players/compare. Reuses newTrendsTestEnv + seedGame from
// player_trends_r60_test.go.

func TestHandlePlayerCompare_HappyPath(t *testing.T) {
	h, decksDir := newTrendsTestEnv(t)
	writeArchetypeStrategy(t, decksDir, "alice", "ctrl", "control")
	writeArchetypeStrategy(t, decksDir, "bob", "combo", "combo")

	// 5 games where alice (seat 0) faces bob (seat 1) along with two
	// other players. Alice wins 3, Bob wins 1, other-player wins 1.
	for i, winner := range []int{0, 0, 0, 1, 2} {
		seedGame(t, h, int64(100*(i+1)), winner,
			[4]string{"AliceCmdr", "BobCmdr", "C", "D"},
			[4]string{"alice", "bob", "carol", "dave"},
			[4]string{"ctrl", "combo", "x", "y"})
	}

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET",
		"/api/players/compare?a=alice&b=bob&window=10&recent_window=5", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.PlayerComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.PlayerA.PlayerID != "alice" || out.PlayerB.PlayerID != "bob" {
		t.Errorf("embed ids: %s / %s", out.PlayerA.PlayerID, out.PlayerB.PlayerID)
	}
	if out.PlayerA.GamesPlayed != 5 || out.PlayerB.GamesPlayed != 5 {
		t.Errorf("games: A=%d B=%d, want 5/5", out.PlayerA.GamesPlayed, out.PlayerB.GamesPlayed)
	}
	if out.PlayerA.Wins != 3 || out.PlayerB.Wins != 1 {
		t.Errorf("wins: A=%d B=%d", out.PlayerA.Wins, out.PlayerB.Wins)
	}
	// Head-to-head: 5 shared games (always seats 0 and 1). Alice
	// wins 3, Bob 1, "other" 1.
	if out.HeadToHead.GamesTogether != 5 {
		t.Errorf("h2h games: %d, want 5", out.HeadToHead.GamesTogether)
	}
	if out.HeadToHead.PlayerAWins != 3 || out.HeadToHead.PlayerBWins != 1 {
		t.Errorf("h2h wins: A=%d B=%d, want 3/1", out.HeadToHead.PlayerAWins, out.HeadToHead.PlayerBWins)
	}
}

// TestHandlePlayerCompare_NoSharedGames pins the case where two
// players have games but never sat at the same table.
func TestHandlePlayerCompare_NoSharedGames(t *testing.T) {
	h, decksDir := newTrendsTestEnv(t)
	writeArchetypeStrategy(t, decksDir, "alice", "x", "control")
	writeArchetypeStrategy(t, decksDir, "bob", "y", "combo")

	// Alice plays games where Bob isn't seated; Bob plays separate games.
	for i, winner := range []int{0, 0} {
		seedGame(t, h, int64(100*(i+1)), winner,
			[4]string{"AC", "Foe1", "Foe2", "Foe3"},
			[4]string{"alice", "other", "other", "other"},
			[4]string{"x", "a", "b", "c"})
	}
	for i, winner := range []int{0, 1} {
		seedGame(t, h, int64(500+100*(i+1)), winner,
			[4]string{"BC", "Foe1", "Foe2", "Foe3"},
			[4]string{"bob", "other", "other", "other"},
			[4]string{"y", "a", "b", "c"})
	}

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET", "/api/players/compare?a=alice&b=bob", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.PlayerComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.HeadToHead.GamesTogether != 0 {
		t.Errorf("h2h games = %d, want 0 (never sat together)", out.HeadToHead.GamesTogether)
	}
}

// TestHandlePlayerCompare_RejectsBlankOrEqualIDs pins the validation
// gates: missing param, invalid path-component characters, and a==b.
func TestHandlePlayerCompare_RejectsBlankOrEqualIDs(t *testing.T) {
	h, _ := newTrendsTestEnv(t)
	mux := http.NewServeMux()
	h.Register(mux)

	cases := []struct {
		name, url string
	}{
		{"missing both", "/api/players/compare"},
		{"missing a", "/api/players/compare?b=bob"},
		{"missing b", "/api/players/compare?a=alice"},
		{"a equals b", "/api/players/compare?a=alice&b=alice"},
		{"invalid a", "/api/players/compare?a=../bad&b=bob"},
		{"invalid b", "/api/players/compare?a=alice&b=..%2Fbad"},
	}
	for _, c := range cases {
		req := httptest.NewRequest("GET", c.url, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", c.name, rec.Code)
		}
	}
}

// TestHandlePlayerCompare_DBUnavailable returns 503 when h.Showmatch
// isn't wired.
func TestHandlePlayerCompare_DBUnavailable(t *testing.T) {
	h := &Handler{DecksDir: t.TempDir()}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/players/compare?a=alice&b=bob", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", rec.Code)
	}
}

// TestHandlePlayerCompare_VelocityComputedFromRecentVsOverall pins
// that the rating velocity actually fires up/down/flat using the
// recent_window slice — i.e., the handler is plumbing recent and
// overall into BuildRatingVelocityFromHalves correctly.
func TestHandlePlayerCompare_VelocityComputedFromRecentVsOverall(t *testing.T) {
	h, decksDir := newTrendsTestEnv(t)
	writeArchetypeStrategy(t, decksDir, "alice", "ctrl", "control")
	writeArchetypeStrategy(t, decksDir, "bob", "combo", "combo")

	// Insert 20 games for alice. Recent 10 (highest finished_at) are
	// all wins; older 10 are all losses → alice trends UP.
	// Insert 20 mirror games for bob: recent losses, older wins → DOWN.
	// We'll pile alice + bob into the same games so they're also h2h
	// data, but the outcome of the older games has alice losing while
	// bob is winning (winner=1) and the newer games have alice winning
	// (winner=0).
	for i := 1; i <= 10; i++ {
		// older era — alice loses (winner=1=bob)
		seedGame(t, h, int64(i), 1,
			[4]string{"AC", "BC", "C", "D"},
			[4]string{"alice", "bob", "carol", "dave"},
			[4]string{"ctrl", "combo", "x", "y"})
	}
	for i := 11; i <= 20; i++ {
		// recent era — alice wins (winner=0)
		seedGame(t, h, int64(1000+i), 0,
			[4]string{"AC", "BC", "C", "D"},
			[4]string{"alice", "bob", "carol", "dave"},
			[4]string{"ctrl", "combo", "x", "y"})
	}

	mux := http.NewServeMux()
	h.Register(mux)
	req := httptest.NewRequest("GET",
		"/api/players/compare?a=alice&b=bob&window=20&recent_window=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d (%s)", rec.Code, rec.Body.String())
	}
	var out heimdall.PlayerComparison
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.RatingVelocityA.Direction != "up" {
		t.Errorf("alice velocity: %q (V=%.3f), want 'up'",
			out.RatingVelocityA.Direction, out.RatingVelocityA.Velocity)
	}
	if out.RatingVelocityB.Direction != "down" {
		t.Errorf("bob velocity: %q (V=%.3f), want 'down'",
			out.RatingVelocityB.Direction, out.RatingVelocityB.Velocity)
	}
}
