package hexapi

import (
	"context"
	"database/sql"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// snapshotDBAdapter satisfies heimdall.SnapshotSink by marshaling the
// snapshot to JSON and upserting it into showmatch_game_observation
// via db.InsertGameObservation. One adapter per *sql.DB; safe to
// share across the live observer pipeline since both the marshal
// step and the underlying ExecContext are stateless.
type snapshotDBAdapter struct {
	db *sql.DB
}

// NewSnapshotDBAdapter constructs the DB-backed snapshot sink that
// production wires into heimdall.Observer.SetSnapshotSink. The
// returned value is a heimdall.SnapshotSink so callers don't take
// a dependency on this package's concrete type.
func NewSnapshotDBAdapter(d *sql.DB) heimdall.SnapshotSink {
	return &snapshotDBAdapter{db: d}
}

// PersistGameObservation marshals the snapshot once per call and
// hands the JSON to db.InsertGameObservation. InsertGameObservation
// is an UPSERT, so incremental live persistence (called repeatedly
// during a game) overwrites the prior row each time.
func (a *snapshotDBAdapter) PersistGameObservation(ctx context.Context, gameID int64, snap heimdall.GameObservationSnapshot) error {
	payload, err := heimdall.MarshalSnapshot(snap)
	if err != nil {
		return err
	}
	return db.InsertGameObservation(ctx, a.db, gameID, payload)
}
