package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func newObservationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := Open(t.TempDir() + "/observation.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func seedObservationGame(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	ctx := context.Background()
	res, err := d.ExecContext(ctx,
		`INSERT INTO showmatch_game (started_at, finished_at, turns, winner, winner_name, end_reason, rng_seed)
		 VALUES (100, 200, 12, 1, 'Atraxa', 'combat', 4242)`)
	if err != nil {
		t.Fatalf("seed game: %v", err)
	}
	gid, _ := res.LastInsertId()
	return gid
}

func TestInsertAndLoadGameObservation_RoundTrip(t *testing.T) {
	d := newObservationTestDB(t)
	ctx := context.Background()
	gid := seedObservationGame(t, d)

	want := `{"seed":{"rng_seed":4242,"winner":1,"turns":12},"final_turn":12}`
	if err := InsertGameObservation(ctx, d, gid, want); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := LoadGameObservation(ctx, d, gid)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Errorf("payload mismatch:\nwant %q\ngot  %q", want, got)
	}
}

func TestLoadGameObservation_ReturnsErrNoRowsForMissingSnapshot(t *testing.T) {
	d := newObservationTestDB(t)
	ctx := context.Background()
	gid := seedObservationGame(t, d)

	_, err := LoadGameObservation(ctx, d, gid)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("want sql.ErrNoRows for missing snapshot, got %v", err)
	}
}

func TestInsertGameObservation_UpsertsOnSecondInsert(t *testing.T) {
	d := newObservationTestDB(t)
	ctx := context.Background()
	gid := seedObservationGame(t, d)

	if err := InsertGameObservation(ctx, d, gid, `{"v":1}`); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := InsertGameObservation(ctx, d, gid, `{"v":2}`); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := LoadGameObservation(ctx, d, gid)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != `{"v":2}` {
		t.Errorf("upsert should overwrite; got %q", got)
	}
}

func TestDeleteGameObservation_RemovesRow(t *testing.T) {
	d := newObservationTestDB(t)
	ctx := context.Background()
	gid := seedObservationGame(t, d)

	if err := InsertGameObservation(ctx, d, gid, `{"v":1}`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := DeleteGameObservation(ctx, d, gid); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := LoadGameObservation(ctx, d, gid)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("post-delete load should return ErrNoRows, got %v", err)
	}
}

func TestDeleteGameObservation_NoOpForUnknownRow(t *testing.T) {
	d := newObservationTestDB(t)
	ctx := context.Background()

	if err := DeleteGameObservation(ctx, d, 99999); err != nil {
		t.Errorf("delete of unknown row should be no-op; got %v", err)
	}
}

// ON DELETE CASCADE: deleting the parent showmatch_game row should
// also drop the snapshot.
func TestGameObservation_CascadesOnParentDelete(t *testing.T) {
	d := newObservationTestDB(t)
	ctx := context.Background()
	gid := seedObservationGame(t, d)

	if err := InsertGameObservation(ctx, d, gid, `{"v":1}`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if _, err := d.ExecContext(ctx, `DELETE FROM showmatch_game WHERE game_id = ?`, gid); err != nil {
		t.Fatalf("delete parent: %v", err)
	}

	_, err := LoadGameObservation(ctx, d, gid)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("snapshot should cascade-delete with parent game; got %v", err)
	}
}
