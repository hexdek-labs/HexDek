package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// seedOwnerGamesPerfData populates showmatch_elo + showmatch_game +
// showmatch_game_seat with a representative-shaped dataset for the
// LoadOwnerGames perf test / benchmark.
//
// Defaults to nOwners=40, decksPerOwner=50 (→ 2000 elo rows), and
// nGames=500 with 4 seats each (→ 2000 seat rows). At that size the
// without-index variant scans ~2000 elo rows per call; with the index
// the search collapses to ~50 rows for any one owner. Big enough to
// see the difference in a benchmark, small enough that seeding takes
// well under a second per benchmark setup.
func seedOwnerGamesPerfData(tb testing.TB, db *sql.DB, nOwners, decksPerOwner, nGames int) {
	tb.Helper()
	ctx := context.Background()

	// 1. ELO rows: nOwners × decksPerOwner.
	var elos []ELORecord
	for o := 0; o < nOwners; o++ {
		owner := fmt.Sprintf("owner_%03d", o)
		for d := 0; d < decksPerOwner; d++ {
			elos = append(elos, ELORecord{
				DeckKey:   fmt.Sprintf("%s_deck_%03d", owner, d),
				Commander: fmt.Sprintf("Cmdr %d", d%20),
				Owner:     owner,
				Rating:    1500,
			})
		}
	}
	if err := BatchUpsertELO(ctx, db, elos); err != nil {
		tb.Fatalf("BatchUpsertELO: %v", err)
	}

	// 2. Games + 4 seats each. The middle owner (nOwners/2) is the
	// "probe" owner the bench/test queries against; spread their decks
	// across nGames so the join produces a realistic row count.
	probeOwner := fmt.Sprintf("owner_%03d", nOwners/2)
	for g := 0; g < nGames; g++ {
		gameID, err := InsertGame(ctx, db, GameRecord{
			StartedAt:  int64(1700000000 + g*100),
			FinishedAt: int64(1700000000 + g*100 + 60),
			Turns:      10,
			Winner:     g % 4,
			WinnerName: "Cmdr",
			EndReason:  "decked",
		})
		if err != nil {
			tb.Fatalf("InsertGame: %v", err)
		}
		// Seat 0 = probe owner's deck so LoadOwnerGames returns a
		// non-trivial number of rows. Seats 1-3 = other owners.
		seats := []struct {
			seat    int
			owner   string
			deckIdx int
		}{
			{0, probeOwner, g % decksPerOwner},
			{1, fmt.Sprintf("owner_%03d", (g+1)%nOwners), g % decksPerOwner},
			{2, fmt.Sprintf("owner_%03d", (g+2)%nOwners), g % decksPerOwner},
			{3, fmt.Sprintf("owner_%03d", (g+3)%nOwners), g % decksPerOwner},
		}
		for _, s := range seats {
			deckKey := fmt.Sprintf("%s_deck_%03d", s.owner, s.deckIdx)
			if _, err := db.ExecContext(ctx,
				`INSERT INTO showmatch_game_seat (game_id, seat, commander, deck_key, life, hand_size, library_size, gy_size, bf_size, lost, battlefield_cards)
				 VALUES (?, ?, ?, ?, 40, 7, 92, 0, 0, 0, '[]')`,
				gameID, s.seat, "Cmdr", deckKey); err != nil {
				tb.Fatalf("insert seat: %v", err)
			}
		}
	}
}

// TestLoadOwnerGames_UsesOwnerIndex asserts the planner uses
// idx_showmatch_elo_owner for the LoadOwnerGames query. This is the
// invariant we care about — without the index the query plan does a
// full SCAN of showmatch_elo, which is the bottleneck the r60 index
// is meant to remove.
func TestLoadOwnerGames_UsesOwnerIndex(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	seedOwnerGamesPerfData(t, db, 8, 5, 20)

	rows, err := db.Query(`EXPLAIN QUERY PLAN
		SELECT g.game_id, g.finished_at, g.turns, g.winner, g.winner_name, g.end_reason, me.seat, me.commander
		FROM showmatch_game g
		JOIN showmatch_game_seat me ON me.game_id = g.game_id
		JOIN showmatch_elo e ON e.deck_key = me.deck_key AND e.owner = ?
		ORDER BY g.finished_at DESC LIMIT ?`, "owner_004", 20)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()
	var planLines []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		planLines = append(planLines, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	plan := strings.Join(planLines, "\n")
	if !strings.Contains(plan, "idx_showmatch_elo_owner") {
		t.Errorf("query plan should use idx_showmatch_elo_owner; got:\n%s", plan)
	}
	if strings.Contains(plan, "SCAN e\n") || strings.HasSuffix(plan, "SCAN e") {
		t.Errorf("query plan still does a full SCAN of showmatch_elo:\n%s", plan)
	}
}

// TestLoadOwnerGames_BeforeAfterIndex compares the EXPLAIN QUERY PLAN
// of the LoadOwnerGames query before vs. after the index is dropped
// — the without-index plan must SCAN showmatch_elo, the with-index
// plan must SEARCH via idx_showmatch_elo_owner. Documents the
// improvement at the planner level so a future schema change that
// silently drops the index is caught.
func TestLoadOwnerGames_BeforeAfterIndex(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	seedOwnerGamesPerfData(t, db, 8, 5, 20)

	// With the index in place:
	planWith := explainOwnerGamesPlan(t, db)
	if !strings.Contains(planWith, "idx_showmatch_elo_owner") {
		t.Fatalf("with-index plan missing the new index:\n%s", planWith)
	}

	// Drop the index and re-explain — must regress to a SCAN of e.
	if _, err := db.Exec("DROP INDEX idx_showmatch_elo_owner"); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	planWithout := explainOwnerGamesPlan(t, db)
	if strings.Contains(planWithout, "idx_showmatch_elo_owner") {
		t.Fatalf("without-index plan still references the index:\n%s", planWithout)
	}
	// Pre-fix shape: planner walks seats first ("SCAN me") and looks up
	// each row's elo by deck_key via the auto-PK index, applying
	// owner=? as a residual filter — every seat row is touched even
	// when only one owner's games are wanted. The post-fix plan
	// (verified above with the index in place) starts from
	// idx_showmatch_elo_owner instead.
	if !strings.Contains(planWithout, "SCAN me") {
		t.Fatalf("expected pre-fix plan to SCAN showmatch_game_seat (alias me); got:\n%s", planWithout)
	}
}

func explainOwnerGamesPlan(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`EXPLAIN QUERY PLAN
		SELECT g.game_id, g.finished_at, g.turns, g.winner, g.winner_name, g.end_reason, me.seat, me.commander
		FROM showmatch_game g
		JOIN showmatch_game_seat me ON me.game_id = g.game_id
		JOIN showmatch_elo e ON e.deck_key = me.deck_key AND e.owner = ?
		ORDER BY g.finished_at DESC LIMIT ?`, "owner_004", 20)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		out = append(out, detail)
	}
	return strings.Join(out, "\n")
}

// BenchmarkLoadOwnerGames_WithIndex measures the post-fix latency.
func BenchmarkLoadOwnerGames_WithIndex(b *testing.B) {
	db, err := Open(":memory:")
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer db.Close()
	seedOwnerGamesPerfData(b, db, 40, 50, 500)
	probeOwner := "owner_020"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		games, err := LoadOwnerGames(context.Background(), db, probeOwner, 20)
		if err != nil {
			b.Fatalf("LoadOwnerGames: %v", err)
		}
		if len(games) == 0 {
			b.Fatal("no rows returned — bench would be meaningless")
		}
	}
}

// BenchmarkLoadOwnerGames_WithoutIndex measures the pre-fix latency.
// Drops the index post-seed so the planner falls back to the full
// SCAN of showmatch_elo. Direct apples-to-apples against the
// WithIndex variant — same data, same query, only the index differs.
func BenchmarkLoadOwnerGames_WithoutIndex(b *testing.B) {
	db, err := Open(":memory:")
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	defer db.Close()
	seedOwnerGamesPerfData(b, db, 40, 50, 500)
	if _, err := db.Exec("DROP INDEX idx_showmatch_elo_owner"); err != nil {
		b.Fatalf("drop index: %v", err)
	}
	probeOwner := "owner_020"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		games, err := LoadOwnerGames(context.Background(), db, probeOwner, 20)
		if err != nil {
			b.Fatalf("LoadOwnerGames: %v", err)
		}
		if len(games) == 0 {
			b.Fatal("no rows returned")
		}
	}
}
