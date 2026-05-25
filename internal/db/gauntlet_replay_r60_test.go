package db

import (
	"context"
	"testing"
	"time"
)

// gauntlet_replay_r60_test.go — gauntlet replay archive
// (gauntlet_run_game junction + pending/finalize lifecycle).

// -----------------------------------------------------------------------------
// Pending → finalize lifecycle
// -----------------------------------------------------------------------------

func TestInsertPendingGauntletRun_ReservesRowWithZeroAggregates(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	started := time.Unix(1000, 0)
	runID, err := InsertPendingGauntletRun(ctx, d, "alice/iroh", "Iroh", started)
	if err != nil {
		t.Fatalf("InsertPendingGauntletRun: %v", err)
	}
	if runID <= 0 {
		t.Fatalf("expected positive runID, got %d", runID)
	}

	// Aggregate counters must start at 0 — finalize overwrites.
	rec, err := LoadGauntletRunByID(ctx, d, runID)
	if err != nil {
		t.Fatalf("LoadGauntletRunByID: %v", err)
	}
	if rec.Games != 0 || rec.Wins != 0 || rec.Losses != 0 {
		t.Errorf("pending row should have zero counters; got games=%d wins=%d losses=%d",
			rec.Games, rec.Wins, rec.Losses)
	}
	if rec.DeckKey != "alice/iroh" || rec.Commander != "Iroh" {
		t.Errorf("pending row identity wrong: deck=%q cmdr=%q", rec.DeckKey, rec.Commander)
	}
}

func TestFinalizeGauntletRun_OverwritesPendingRow(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	started := time.Unix(1000, 0)
	runID, err := InsertPendingGauntletRun(ctx, d, "alice/iroh", "Iroh", started)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	final := GauntletRunRecord{
		RunID:      runID,
		DeckKey:    "alice/iroh",
		Commander:  "Iroh",
		StartedAt:  started,
		FinishedAt: time.Unix(9000, 0),
		Games:      100,
		Wins:       42,
		Losses:     58,
		WinRate:    42.0,
		ELOStart:   1500,
		ELOEnd:     1525.5,
		ELODelta:   25.5,
		AvgTurns:   8.4,
		Place1st:   42,
		Place2nd:   30,
		Place3rd:   18,
		Place4th:   10,
	}
	if err := FinalizeGauntletRun(ctx, d, final); err != nil {
		t.Fatalf("FinalizeGauntletRun: %v", err)
	}

	loaded, err := LoadGauntletRunByID(ctx, d, runID)
	if err != nil {
		t.Fatalf("LoadGauntletRunByID: %v", err)
	}
	if loaded.Games != 100 || loaded.Wins != 42 || loaded.Losses != 58 {
		t.Errorf("finalize didn't overwrite; got games=%d wins=%d losses=%d",
			loaded.Games, loaded.Wins, loaded.Losses)
	}
	if loaded.AvgTurns != 8.4 {
		t.Errorf("AvgTurns = %v, want 8.4", loaded.AvgTurns)
	}
	if loaded.Place1st != 42 || loaded.Place4th != 10 {
		t.Errorf("placements lost: 1st=%d 4th=%d", loaded.Place1st, loaded.Place4th)
	}
	// Row count should still be 1 (UPDATE, not INSERT).
	var n int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM gauntlet_runs`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row after finalize, got %d", n)
	}
}

func TestFinalizeGauntletRun_RequiresRunID(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	rec := GauntletRunRecord{RunID: 0, DeckKey: "x/y"}
	if err := FinalizeGauntletRun(context.Background(), d, rec); err == nil {
		t.Errorf("expected error for RunID=0")
	}
}

// -----------------------------------------------------------------------------
// Junction table (gauntlet_run_game)
// -----------------------------------------------------------------------------

func seedGame(t *testing.T, ctx context.Context, finishedAt int64) GameRecord {
	t.Helper()
	return GameRecord{
		StartedAt:  finishedAt - 60,
		FinishedAt: finishedAt,
		Turns:      6,
		Winner:     0,
		WinnerName: "X",
		EndReason:  "combat",
		Seed:       finishedAt,
	}
}

func TestInsertGauntletRunGame_LinksToRun(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	runID, err := InsertPendingGauntletRun(ctx, d, "alice/iroh", "Iroh", time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// Persist three real showmatch_game rows so the FK is satisfied,
	// then link each to the gauntlet.
	gameIDs := make([]int64, 3)
	for i := range gameIDs {
		g := seedGame(t, ctx, int64(1100+i*100))
		seats := []GameSeatRecord{
			{Seat: 0, Commander: "Iroh", DeckKey: "alice/iroh", Lost: false},
			{Seat: 1, Commander: "Atraxa", DeckKey: "bob/atraxa", Lost: true},
			{Seat: 2, Commander: "Edgar", DeckKey: "carol/edgar", Lost: true},
			{Seat: 3, Commander: "Krenko", DeckKey: "dave/krenko", Lost: true},
		}
		gameID, err := PersistGameTx(ctx, d, g, seats)
		if err != nil {
			t.Fatalf("persist game %d: %v", i, err)
		}
		gameIDs[i] = gameID
		if err := InsertGauntletRunGame(ctx, d, runID, gameID, i); err != nil {
			t.Fatalf("link game %d: %v", i, err)
		}
	}

	got, err := LoadGauntletRunGames(ctx, d, runID)
	if err != nil {
		t.Fatalf("LoadGauntletRunGames: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 linked games, got %d", len(got))
	}
	for i, id := range got {
		if id != gameIDs[i] {
			t.Errorf("linked games[%d] = %d, want %d (sequence preserved by game_index)", i, id, gameIDs[i])
		}
	}
}

func TestInsertGauntletRunGame_OrdersByGameIndex(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	runID, _ := InsertPendingGauntletRun(ctx, d, "alice/iroh", "Iroh", time.Unix(1000, 0))

	// Insert in REVERSE order — LoadGauntletRunGames must still sort
	// by game_index asc, not by insertion order.
	gameIDs := make([]int64, 3)
	for i := 2; i >= 0; i-- {
		g := seedGame(t, ctx, int64(1100+i*100))
		gameID, err := PersistGameTx(ctx, d, g, []GameSeatRecord{
			{Seat: 0, Commander: "Iroh", DeckKey: "alice/iroh"},
			{Seat: 1, Commander: "X", DeckKey: "x/y"},
			{Seat: 2, Commander: "Y", DeckKey: "y/z"},
			{Seat: 3, Commander: "Z", DeckKey: "z/a"},
		})
		if err != nil {
			t.Fatalf("persist: %v", err)
		}
		gameIDs[i] = gameID
		if err := InsertGauntletRunGame(ctx, d, runID, gameID, i); err != nil {
			t.Fatalf("link: %v", err)
		}
	}

	got, _ := LoadGauntletRunGames(ctx, d, runID)
	for i, id := range got {
		if id != gameIDs[i] {
			t.Errorf("got[%d] = %d, want %d (game_index ordering broken)", i, id, gameIDs[i])
		}
	}
}

func TestInsertGauntletRunGame_IdempotentOnPrimaryKey(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	runID, _ := InsertPendingGauntletRun(ctx, d, "alice/iroh", "Iroh", time.Unix(1000, 0))
	g := seedGame(t, ctx, 1100)
	gameID, _ := PersistGameTx(ctx, d, g, []GameSeatRecord{
		{Seat: 0, Commander: "Iroh", DeckKey: "alice/iroh"},
		{Seat: 1, Commander: "X", DeckKey: "x/y"},
		{Seat: 2, Commander: "Y", DeckKey: "y/z"},
		{Seat: 3, Commander: "Z", DeckKey: "z/a"},
	})

	// First link.
	if err := InsertGauntletRunGame(ctx, d, runID, gameID, 0); err != nil {
		t.Fatalf("first link: %v", err)
	}
	// Duplicate link — must not error and must not duplicate.
	if err := InsertGauntletRunGame(ctx, d, runID, gameID, 0); err != nil {
		t.Fatalf("duplicate link should be idempotent: %v", err)
	}
	got, _ := LoadGauntletRunGames(ctx, d, runID)
	if len(got) != 1 {
		t.Errorf("expected 1 linked game after dedupe, got %d", len(got))
	}
}

func TestLoadGauntletRunGames_EmptyForUnknownGauntlet(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	got, err := LoadGauntletRunGames(context.Background(), d, 9999)
	if err != nil {
		t.Fatalf("LoadGauntletRunGames: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result for unknown gauntlet, got %d games", len(got))
	}
}

// -----------------------------------------------------------------------------
// Cascade: deleting a gauntlet drops its junction rows.
// -----------------------------------------------------------------------------

func TestGauntletRunGame_CascadeOnGauntletDelete(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	ctx := context.Background()

	runID, _ := InsertPendingGauntletRun(ctx, d, "alice/iroh", "Iroh", time.Unix(1000, 0))
	g := seedGame(t, ctx, 1100)
	gameID, _ := PersistGameTx(ctx, d, g, []GameSeatRecord{
		{Seat: 0, Commander: "Iroh", DeckKey: "alice/iroh"},
		{Seat: 1, Commander: "X", DeckKey: "x/y"},
		{Seat: 2, Commander: "Y", DeckKey: "y/z"},
		{Seat: 3, Commander: "Z", DeckKey: "z/a"},
	})
	_ = InsertGauntletRunGame(ctx, d, runID, gameID, 0)

	if _, err := d.ExecContext(ctx, `DELETE FROM gauntlet_runs WHERE run_id = ?`, runID); err != nil {
		t.Fatalf("delete gauntlet: %v", err)
	}
	got, _ := LoadGauntletRunGames(ctx, d, runID)
	if len(got) != 0 {
		t.Errorf("expected junction rows cascade-deleted; got %d", len(got))
	}
	// The showmatch_game row itself should still exist (junction
	// cascade goes one way — the game data outlives its gauntlet
	// archive link).
	var present int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM showmatch_game WHERE game_id = ?`, gameID).Scan(&present)
	if present != 1 {
		t.Errorf("showmatch_game row should survive gauntlet delete; got count=%d", present)
	}
}
