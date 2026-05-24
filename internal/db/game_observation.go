package db

import (
	"context"
	"database/sql"
)

// InsertGameObservation upserts the JSON-serialized observation
// payload for a game. Idempotent — re-running after a re-replay
// overwrites the prior snapshot rather than failing on PK conflict,
// so backfill workers can safely replay over already-snapshotted
// games. payload is opaque to the DB layer; callers
// (heimdall.MarshalSnapshot) own its shape.
func InsertGameObservation(ctx context.Context, db *sql.DB, gameID int64, payload string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO showmatch_game_observation (game_id, payload, created_at)
		 VALUES (?, ?, unixepoch())
		 ON CONFLICT(game_id) DO UPDATE SET
		   payload = excluded.payload,
		   created_at = excluded.created_at`,
		gameID, payload)
	return err
}

// LoadGameObservation fetches the JSON-serialized observation
// payload for a game. Returns sql.ErrNoRows verbatim when no
// snapshot has been persisted so callers can distinguish "snapshot
// missing — fall back to db_only summary" from "DB error".
func LoadGameObservation(ctx context.Context, db *sql.DB, gameID int64) (string, error) {
	var payload string
	err := db.QueryRowContext(ctx,
		`SELECT payload FROM showmatch_game_observation WHERE game_id = ?`,
		gameID).Scan(&payload)
	return payload, err
}

// DeleteGameObservation removes the snapshot for a game. Provided
// for tests and for the operational case where a deck rename
// invalidates a stale snapshot before a re-replay runs. No-op when
// the row doesn't exist.
func DeleteGameObservation(ctx context.Context, db *sql.DB, gameID int64) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM showmatch_game_observation WHERE game_id = ?`, gameID)
	return err
}
