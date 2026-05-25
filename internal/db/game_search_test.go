package db

import (
	"context"
	"database/sql"
	"testing"
)

// game_search_test.go — DB-level tests for SearchGames covering the
// owner / won-lost / turn-range / q / end-reason filter combinations
// that the /api/games/search endpoint surfaces.

func openSearchDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func insertSearchGame(t *testing.T, d *sql.DB, finishedAt int64, turns, winner int,
	commanders, owners, decks [4]string, endReason, winnerName string) int64 {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if owners[i] == "" {
			continue
		}
		deckKey := owners[i] + "/" + decks[i]
		_, err := d.ExecContext(ctx,
			`INSERT OR IGNORE INTO showmatch_elo (deck_key, commander, owner, rating, hex_rating, games, wins, losses, delta, hex_delta, bracket, updated_at)
			 VALUES (?, ?, ?, 1500, 0, 0, 0, 0, 0, 0, 0, unixepoch())`,
			deckKey, commanders[i], owners[i])
		if err != nil {
			t.Fatalf("elo insert: %v", err)
		}
	}
	g := GameRecord{
		StartedAt:  finishedAt - 600,
		FinishedAt: finishedAt,
		Turns:      turns,
		Winner:     winner,
		WinnerName: winnerName,
		EndReason:  endReason,
		Seed:       finishedAt,
	}
	seats := make([]GameSeatRecord, 0, 4)
	for i := 0; i < 4; i++ {
		seats = append(seats, GameSeatRecord{
			Seat:      i,
			Commander: commanders[i],
			DeckKey:   owners[i] + "/" + decks[i],
			Lost:      i != winner,
		})
	}
	gid, err := PersistGameTx(ctx, d, g, seats)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	return gid
}

// TestSearchGames_NoFiltersReturnsAll pins the no-filter baseline:
// every game in the DB comes back, newest first.
func TestSearchGames_NoFiltersReturnsAll(t *testing.T) {
	d := openSearchDB(t)
	insertSearchGame(t, d, 100, 8, 0,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "A")
	insertSearchGame(t, d, 200, 12, 1,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "decking", "B")

	rows, err := SearchGames(context.Background(), d, GameSearchFilters{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	// Newest first.
	if rows[0].FinishedAt != 200 || rows[1].FinishedAt != 100 {
		t.Errorf("order: got [%d, %d], want [200, 100]",
			rows[0].FinishedAt, rows[1].FinishedAt)
	}
}

// TestSearchGames_OwnerFilterIncludesOwnerSeatAndOwnerWon pins that
// when owner is set, OwnerSeat is the seat index of the owner's
// deck, and OwnerWon == (owner_seat == winner).
func TestSearchGames_OwnerFilterIncludesOwnerSeatAndOwnerWon(t *testing.T) {
	d := openSearchDB(t)
	// alice sits at seat 0, wins the first game; sits at seat 0,
	// loses the second.
	insertSearchGame(t, d, 100, 8, 0,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "A")
	insertSearchGame(t, d, 200, 10, 1,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "B")
	// dave (not alice) plays a separate game — alice not seated.
	insertSearchGame(t, d, 300, 6, 0,
		[4]string{"X", "Y", "Z", "W"},
		[4]string{"dave", "e", "f", "g"},
		[4]string{"x1", "y1", "z1", "w1"}, "combat", "X")

	rows, err := SearchGames(context.Background(), d, GameSearchFilters{Owner: "alice"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (alice in 2 of 3 games)", len(rows))
	}
	for _, r := range rows {
		if r.OwnerSeat != 0 {
			t.Errorf("game %d: OwnerSeat = %d, want 0", r.GameID, r.OwnerSeat)
		}
	}
	// Newest first: gid for finishedAt=200 should be first, with OwnerWon=false.
	if rows[0].FinishedAt != 200 || rows[0].OwnerWon {
		t.Errorf("rows[0]: %+v, want finishedAt=200 + OwnerWon=false", rows[0])
	}
	if rows[1].FinishedAt != 100 || !rows[1].OwnerWon {
		t.Errorf("rows[1]: %+v, want finishedAt=100 + OwnerWon=true", rows[1])
	}
}

// TestSearchGames_WonFilterFiltersToOwnerWonOrLost pins the won bool
// filter: true → only owner-won games, false → only owner-lost.
func TestSearchGames_WonFilterFiltersToOwnerWonOrLost(t *testing.T) {
	d := openSearchDB(t)
	// 3 games for alice: 2 wins, 1 loss
	for i, w := range []int{0, 0, 1} {
		insertSearchGame(t, d, int64(100*(i+1)), 8, w,
			[4]string{"A", "B", "C", "D"},
			[4]string{"alice", "bob", "c", "d"},
			[4]string{"a1", "b1", "c1", "d1"}, "combat", "X")
	}
	trueVal := true
	falseVal := false

	wonRows, err := SearchGames(context.Background(), d, GameSearchFilters{Owner: "alice", Won: &trueVal})
	if err != nil {
		t.Fatalf("won=true: %v", err)
	}
	if len(wonRows) != 2 {
		t.Errorf("won=true: got %d rows, want 2", len(wonRows))
	}
	for _, r := range wonRows {
		if !r.OwnerWon {
			t.Errorf("won=true row has OwnerWon=false: %+v", r)
		}
	}

	lostRows, err := SearchGames(context.Background(), d, GameSearchFilters{Owner: "alice", Won: &falseVal})
	if err != nil {
		t.Fatalf("won=false: %v", err)
	}
	if len(lostRows) != 1 {
		t.Errorf("won=false: got %d rows, want 1", len(lostRows))
	}
	if lostRows[0].OwnerWon {
		t.Errorf("won=false row has OwnerWon=true: %+v", lostRows[0])
	}
}

// TestSearchGames_TurnRangeFilters pins min_turn / max_turn (inclusive).
func TestSearchGames_TurnRangeFilters(t *testing.T) {
	d := openSearchDB(t)
	for i, turns := range []int{3, 5, 8, 12, 20} {
		insertSearchGame(t, d, int64(100*(i+1)), turns, 0,
			[4]string{"A", "B", "C", "D"},
			[4]string{"a", "b", "c", "d"},
			[4]string{"a1", "b1", "c1", "d1"}, "combat", "A")
	}
	// MinTurn=5 → drops the 3-turn game (4 results).
	rows, err := SearchGames(context.Background(), d, GameSearchFilters{MinTurn: 5})
	if err != nil {
		t.Fatalf("min_turn=5: %v", err)
	}
	if len(rows) != 4 {
		t.Errorf("MinTurn=5 → %d rows, want 4", len(rows))
	}
	// MaxTurn=10 → 3, 5, 8 → 3 rows.
	rows, err = SearchGames(context.Background(), d, GameSearchFilters{MaxTurn: 10})
	if err != nil {
		t.Fatalf("max_turn=10: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("MaxTurn=10 → %d rows, want 3", len(rows))
	}
	// Both: 5 ≤ turns ≤ 12 → 5, 8, 12 → 3 rows.
	rows, err = SearchGames(context.Background(), d, GameSearchFilters{MinTurn: 5, MaxTurn: 12})
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("range → %d rows, want 3", len(rows))
	}
}

// TestSearchGames_EndReasonFiltersCaseInsensitive pins that end_reason
// is matched case-insensitively (exact match).
func TestSearchGames_EndReasonFiltersCaseInsensitive(t *testing.T) {
	d := openSearchDB(t)
	insertSearchGame(t, d, 100, 8, 0,
		[4]string{"A", "B", "C", "D"},
		[4]string{"a", "b", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "A")
	insertSearchGame(t, d, 200, 8, 0,
		[4]string{"A", "B", "C", "D"},
		[4]string{"a", "b", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "DECKING", "A")
	rows, err := SearchGames(context.Background(), d, GameSearchFilters{EndReason: "decking"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
}

// TestSearchGames_QMatchesWinnerNameEndReasonOrCommander pins the
// three-way OR in the q filter.
func TestSearchGames_QMatchesWinnerNameEndReasonOrCommander(t *testing.T) {
	d := openSearchDB(t)
	// Game 1: winner_name "Krenko" should match q="krenko"
	insertSearchGame(t, d, 100, 8, 0,
		[4]string{"Krenko", "Foo", "Bar", "Baz"},
		[4]string{"a", "b", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "Krenko")
	// Game 2: end_reason "wipe" should match q="WIPE"
	insertSearchGame(t, d, 200, 8, 0,
		[4]string{"X", "Y", "Z", "W"},
		[4]string{"a", "b", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "wipe", "X")
	// Game 3: opponent seat carries commander "Atraxa" — should
	// match q="atraxa"
	insertSearchGame(t, d, 300, 8, 0,
		[4]string{"Mine", "Atraxa", "Z", "W"},
		[4]string{"a", "b", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "Mine")

	for _, q := range []string{"krenko", "Krenko", "KRENKO"} {
		rows, _ := SearchGames(context.Background(), d, GameSearchFilters{Q: q})
		if len(rows) != 1 || rows[0].WinnerName != "Krenko" {
			t.Errorf("q=%q → %+v, want exactly Krenko game", q, rows)
		}
	}
	rows, _ := SearchGames(context.Background(), d, GameSearchFilters{Q: "WIPE"})
	if len(rows) != 1 || rows[0].EndReason != "wipe" {
		t.Errorf("q=WIPE → %+v, want wipe game", rows)
	}
	rows, _ = SearchGames(context.Background(), d, GameSearchFilters{Q: "atraxa"})
	if len(rows) != 1 || rows[0].FinishedAt != 300 {
		t.Errorf("q=atraxa (commander seat) → %+v, want game with Atraxa", rows)
	}
}

// TestSearchGames_CombinedFiltersCompose pins that multiple filters
// AND together — the user's example "find all my games where I lost
// to a board wipe on turn 5".
func TestSearchGames_CombinedFiltersCompose(t *testing.T) {
	d := openSearchDB(t)
	// alice lost to a wipe on turn 5 — matches.
	insertSearchGame(t, d, 100, 5, 1,
		[4]string{"A", "Wipe Bringer", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "wipe", "B")
	// alice WON on turn 5 with a wipe — same winner_name + end_reason
	// but should be filtered out by won=false.
	insertSearchGame(t, d, 200, 5, 0,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "wipe", "A")
	// alice lost on turn 12 (not 5) — wrong turn.
	insertSearchGame(t, d, 300, 12, 1,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "wipe", "B")
	// alice lost to combat (not wipe) on turn 5 — wrong end_reason.
	insertSearchGame(t, d, 400, 5, 1,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "B")

	falseVal := false
	rows, err := SearchGames(context.Background(), d, GameSearchFilters{
		Owner: "alice", Won: &falseVal, MinTurn: 5, MaxTurn: 5, EndReason: "wipe",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 (only the alice-lost-wipe-turn-5 game)", len(rows))
	}
	if rows[0].FinishedAt != 100 || rows[0].OwnerWon || rows[0].Turns != 5 || rows[0].EndReason != "wipe" {
		t.Errorf("matched wrong game: %+v", rows[0])
	}
}

// TestSearchGames_OwnerFilterExcludesGamesWithoutOwnerSeated pins
// that owner filter drops games where the owner didn't have a seat.
func TestSearchGames_OwnerFilterExcludesGamesWithoutOwnerSeated(t *testing.T) {
	d := openSearchDB(t)
	insertSearchGame(t, d, 100, 8, 0,
		[4]string{"A", "B", "C", "D"},
		[4]string{"alice", "bob", "c", "d"},
		[4]string{"a1", "b1", "c1", "d1"}, "combat", "A")
	insertSearchGame(t, d, 200, 8, 0,
		[4]string{"X", "Y", "Z", "W"},
		[4]string{"dave", "e", "f", "g"},
		[4]string{"x1", "y1", "z1", "w1"}, "combat", "X")
	rows, err := SearchGames(context.Background(), d, GameSearchFilters{Owner: "alice"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 || rows[0].FinishedAt != 100 {
		t.Errorf("got %+v, want only the alice-seated game", rows)
	}
}

// TestSearchGames_LimitAndOffsetPagination pins pagination behavior.
func TestSearchGames_LimitAndOffsetPagination(t *testing.T) {
	d := openSearchDB(t)
	for i := 1; i <= 10; i++ {
		insertSearchGame(t, d, int64(i*100), 8, 0,
			[4]string{"A", "B", "C", "D"},
			[4]string{"a", "b", "c", "d"},
			[4]string{"a1", "b1", "c1", "d1"}, "combat", "A")
	}
	rows, err := SearchGames(context.Background(), d, GameSearchFilters{Limit: 3})
	if err != nil {
		t.Fatalf("limit=3: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("limit=3 → %d rows", len(rows))
	}
	// Offset 5 → skip newest 5 → return next 3 (5th-7th from newest)
	rowsOffset, err := SearchGames(context.Background(), d, GameSearchFilters{Limit: 3, Offset: 5})
	if err != nil {
		t.Fatalf("offset=5: %v", err)
	}
	if len(rowsOffset) != 3 {
		t.Errorf("offset=5 limit=3 → %d rows", len(rowsOffset))
	}
	if rowsOffset[0].FinishedAt == rows[0].FinishedAt {
		t.Errorf("offset didn't move the cursor: page1[0]=%d page2[0]=%d",
			rows[0].FinishedAt, rowsOffset[0].FinishedAt)
	}
}
