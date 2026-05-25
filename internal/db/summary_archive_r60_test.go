package db

import (
	"context"
	"database/sql"
	"testing"
)

func newArchiveTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(t.TempDir() + "/archive.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// seedArchiveGame inserts a game + seats + a placeholder snapshot
// row so the INNER JOIN on showmatch_game_observation finds it.
// Returns the game_id for assertions.
func seedArchiveGame(t *testing.T, d *sql.DB, finishedAt int64, winner int, winnerName string, seats []struct{ Commander, DeckKey string }) int64 {
	t.Helper()
	ctx := context.Background()
	res, err := d.ExecContext(ctx,
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason, rng_seed)
		 VALUES (?, ?, 10, ?, ?, 'combat', 4242)`,
		finishedAt-3600, finishedAt, winner, winnerName)
	if err != nil {
		t.Fatalf("insert game: %v", err)
	}
	gid, _ := res.LastInsertId()
	for i, s := range seats {
		if _, err := d.ExecContext(ctx,
			`INSERT INTO showmatch_game_seat (game_id, seat, commander, deck_key, life, hand_size, library_size, gy_size, bf_size, lost)
			 VALUES (?, ?, ?, ?, 40, 7, 80, 0, 0, 0)`,
			gid, i, s.Commander, s.DeckKey); err != nil {
			t.Fatalf("insert seat: %v", err)
		}
	}
	if err := InsertGameObservation(ctx, d, gid, `{"seed":{"rng_seed":4242}}`); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	return gid
}

func TestSearchGameSummaries_OnlyReturnsGamesWithSnapshots(t *testing.T) {
	d := newArchiveTestDB(t)
	ctx := context.Background()

	// Game WITH snapshot — should appear.
	withSnap := seedArchiveGame(t, d, 1000, 0, "Krenko", []struct{ Commander, DeckKey string }{
		{"Krenko, Mob Boss", "alice/krenko"},
	})

	// Game WITHOUT snapshot — should not appear (INNER JOIN excludes).
	_, err := d.ExecContext(ctx,
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason, rng_seed)
		 VALUES (1000, 1100, 5, 0, 'NoSnap', 'combat', 0)`)
	if err != nil {
		t.Fatalf("insert no-snap game: %v", err)
	}

	rows, err := SearchGameSummaries(ctx, d, SummaryArchiveFilters{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row (only the snapshotted game), got %d", len(rows))
	}
	if rows[0].GameID != withSnap {
		t.Errorf("returned game id mismatch: want %d, got %d", withSnap, rows[0].GameID)
	}
}

func TestSearchGameSummaries_OrderingNewestFirst(t *testing.T) {
	d := newArchiveTestDB(t)

	gOld := seedArchiveGame(t, d, 100, 0, "Old", []struct{ Commander, DeckKey string }{{"X", "a"}})
	gMid := seedArchiveGame(t, d, 500, 0, "Mid", []struct{ Commander, DeckKey string }{{"X", "a"}})
	gNew := seedArchiveGame(t, d, 1000, 0, "New", []struct{ Commander, DeckKey string }{{"X", "a"}})

	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	wantOrder := []int64{gNew, gMid, gOld}
	for i, want := range wantOrder {
		if rows[i].GameID != want {
			t.Errorf("position %d: want game %d, got %d", i, want, rows[i].GameID)
		}
	}
}

func TestSearchGameSummaries_DateRangeFilter(t *testing.T) {
	d := newArchiveTestDB(t)
	seedArchiveGame(t, d, 100, 0, "T100", []struct{ Commander, DeckKey string }{{"X", "a"}})
	g500 := seedArchiveGame(t, d, 500, 0, "T500", []struct{ Commander, DeckKey string }{{"X", "a"}})
	g800 := seedArchiveGame(t, d, 800, 0, "T800", []struct{ Commander, DeckKey string }{{"X", "a"}})
	seedArchiveGame(t, d, 1500, 0, "T1500", []struct{ Commander, DeckKey string }{{"X", "a"}})

	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{
		Since: 400, Until: 1000,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows in [400, 1000], got %d", len(rows))
	}
	gotIDs := map[int64]bool{rows[0].GameID: true, rows[1].GameID: true}
	if !gotIDs[g500] || !gotIDs[g800] {
		t.Errorf("expected g500 + g800, got %+v", rows)
	}
}

func TestSearchGameSummaries_CommanderFilter(t *testing.T) {
	d := newArchiveTestDB(t)
	gKrenko := seedArchiveGame(t, d, 100, 0, "A", []struct{ Commander, DeckKey string }{
		{"Krenko, Mob Boss", "alice/krenko"},
		{"Atraxa, Praetors' Voice", "bob/atraxa"},
	})
	seedArchiveGame(t, d, 200, 0, "B", []struct{ Commander, DeckKey string }{
		{"Edgar Markov", "carol/edgar"},
		{"Muldrotha, the Gravetide", "dave/muldrotha"},
	})

	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{
		Commander: "Atraxa",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 || rows[0].GameID != gKrenko {
		t.Errorf("commander filter should match seat 1's Atraxa; got %+v", rows)
	}
}

func TestSearchGameSummaries_DeckFilter(t *testing.T) {
	d := newArchiveTestDB(t)
	gAlice := seedArchiveGame(t, d, 100, 0, "A", []struct{ Commander, DeckKey string }{
		{"Krenko", "alice/krenko"}, {"X", "carol/x"},
	})
	seedArchiveGame(t, d, 200, 0, "B", []struct{ Commander, DeckKey string }{
		{"Edgar", "bob/edgar"}, {"Y", "dave/y"},
	})

	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{Deck: "alice/"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 || rows[0].GameID != gAlice {
		t.Errorf("deck filter should match alice/krenko; got %+v", rows)
	}
}

func TestSearchGameSummaries_WinnerFilter(t *testing.T) {
	d := newArchiveTestDB(t)
	gKrenko := seedArchiveGame(t, d, 100, 0, "Krenko, Mob Boss", []struct{ Commander, DeckKey string }{{"X", "a"}})
	seedArchiveGame(t, d, 200, 1, "Atraxa, Praetors' Voice", []struct{ Commander, DeckKey string }{{"X", "a"}})

	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{Winner: "Krenko"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 || rows[0].GameID != gKrenko {
		t.Errorf("winner filter should match 'Krenko'; got %+v", rows)
	}
}

func TestSearchGameSummaries_CommandersAndDeckKeysJoined(t *testing.T) {
	d := newArchiveTestDB(t)
	seedArchiveGame(t, d, 100, 0, "Krenko", []struct{ Commander, DeckKey string }{
		{"Krenko, Mob Boss", "alice/krenko"},
		{"Atraxa, Praetors' Voice", "bob/atraxa"},
		{"Edgar Markov", "carol/edgar"},
		{"Muldrotha, the Gravetide", "dave/muldrotha"},
	})

	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if len(rows[0].Commanders) != 4 || rows[0].Commanders[0] != "Krenko, Mob Boss" {
		t.Errorf("commanders not joined in seat order: %v", rows[0].Commanders)
	}
	if len(rows[0].DeckKeys) != 4 || rows[0].DeckKeys[3] != "dave/muldrotha" {
		t.Errorf("deck_keys not joined in seat order: %v", rows[0].DeckKeys)
	}
}

func TestSearchGameSummaries_LimitClampsAndOffsetPages(t *testing.T) {
	d := newArchiveTestDB(t)
	for i := 0; i < 10; i++ {
		seedArchiveGame(t, d, int64(100+i*10), 0, "T", []struct{ Commander, DeckKey string }{{"X", "a"}})
	}

	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{Limit: 3})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("limit=3 should return 3; got %d", len(rows))
	}

	page2, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{Limit: 3, Offset: 3})
	if err != nil {
		t.Fatalf("search page 2: %v", err)
	}
	if len(page2) != 3 {
		t.Errorf("offset=3 limit=3 should return 3; got %d", len(page2))
	}
	if rows[2].GameID == page2[0].GameID {
		t.Errorf("page 2 first row should differ from page 1 last; both = %d", rows[2].GameID)
	}
}

func TestSearchGameSummaries_LimitCappedAt500(t *testing.T) {
	d := newArchiveTestDB(t)
	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{Limit: 999999})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if rows != nil && len(rows) > 500 {
		t.Errorf("Limit should cap at 500; got %d", len(rows))
	}
}

func TestSearchGameSummaries_EmptyResultSet(t *testing.T) {
	d := newArchiveTestDB(t)
	// No games seeded.
	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("empty DB should return 0 rows; got %d", len(rows))
	}
}

func TestSearchGameSummaries_MultiFilterCombines(t *testing.T) {
	d := newArchiveTestDB(t)
	// Match: commander Atraxa AND winner Krenko AND in date range.
	gMatch := seedArchiveGame(t, d, 500, 0, "Krenko", []struct{ Commander, DeckKey string }{
		{"Krenko, Mob Boss", "alice/krenko"},
		{"Atraxa, Praetors' Voice", "bob/atraxa"},
	})
	// Non-match: same commander but wrong winner.
	seedArchiveGame(t, d, 600, 1, "Atraxa, Praetors' Voice", []struct{ Commander, DeckKey string }{
		{"Krenko", "alice/krenko"}, {"Atraxa", "bob/atraxa"},
	})

	rows, err := SearchGameSummaries(context.Background(), d, SummaryArchiveFilters{
		Since: 400, Until: 700, Commander: "Atraxa", Winner: "Krenko",
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 || rows[0].GameID != gMatch {
		t.Errorf("multi-filter intersection should return only gMatch; got %+v", rows)
	}
}
