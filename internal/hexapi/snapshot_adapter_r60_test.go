package hexapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"database/sql"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// End-to-end: a heimdall.Observer wired with the DB-backed adapter
// + an incremental RecordSnapshot call lands a row that
// db.LoadGameObservation can read back, and decoding that row via
// heimdall.UnmarshalSnapshot yields the original snapshot.
func TestSnapshotDBAdapter_PersistAndReload(t *testing.T) {
	h, _ := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 8, "combat")

	adapter := NewSnapshotDBAdapter(h.db)
	observer := &heimdall.Observer{}
	observer.SetSnapshotSink(adapter)

	obs := heimdall.Observation{
		Seed:    heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 8},
		MVPCards: []heimdall.MVPCard{
			{Seat: 1, CardName: "Atraxa, Praetors' Voice", CMC: 4, Score: 8, IsCommander: true},
		},
	}
	observer.RecordSnapshot(context.Background(), gid, obs, nil)

	payload, err := db.LoadGameObservation(context.Background(), h.db, gid)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	snap, err := heimdall.UnmarshalSnapshot(payload)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if snap.Seed.RNGSeed != 4242 || snap.Seed.Winner != 1 {
		t.Errorf("Seed round-trip lost: %+v", snap.Seed)
	}
	if len(snap.MVPCards) != 1 || snap.MVPCards[0].CardName != "Atraxa, Praetors' Voice" {
		t.Errorf("MVPCards round-trip lost: %+v", snap.MVPCards)
	}
}

// Incremental persistence: three RecordSnapshot calls upsert the
// row, and only the latest payload survives. Verifies the
// "live snapshot tracks game progress" contract end-to-end.
func TestSnapshotDBAdapter_IncrementalUpserts(t *testing.T) {
	h, _ := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 0, 1, "incremental")

	adapter := NewSnapshotDBAdapter(h.db)
	observer := &heimdall.Observer{}
	observer.SetSnapshotSink(adapter)

	for turn, winner := range []int{-1, -1, 0} {
		observer.RecordSnapshot(context.Background(), gid, heimdall.Observation{
			Seed: heimdall.GameSeed{RNGSeed: int64(100 + turn), Winner: winner, Turns: turn + 1},
		}, nil)
	}

	payload, err := db.LoadGameObservation(context.Background(), h.db, gid)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	snap, err := heimdall.UnmarshalSnapshot(payload)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Final state should reflect the LAST RecordSnapshot call.
	if snap.Seed.RNGSeed != 102 || snap.Seed.Winner != 0 || snap.Seed.Turns != 3 {
		t.Errorf("final snapshot should reflect last call; got %+v", snap.Seed)
	}
}

// Live spectator → historical-replay handoff: after the live
// snapshot is persisted, the /api/games/{id}/replay endpoint can
// stream it back without any further game state.
func TestSnapshotDBAdapter_PersistedSnapshotPowersReplayEndpoint(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 5, "combat")

	adapter := NewSnapshotDBAdapter(h.db)
	observer := &heimdall.Observer{}
	observer.SetSnapshotSink(adapter)

	// Simulate a live game persisting a final snapshot mid-game.
	observer.RecordSnapshot(context.Background(), gid, heimdall.Observation{
		Seed: heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 5},
		MVPCards: []heimdall.MVPCard{
			{Seat: 1, CardName: "Sol Ring", CMC: 1, TurnPlayed: 1, Score: 1},
		},
	}, nil)

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/replay", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("replay status: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	events := decodeNDJSON(t, rec.Body.String())
	// MVP (turn 1) + game_end (turn 5) = 2 events.
	if len(events) != 2 {
		t.Fatalf("want 2 events from live-persisted snapshot, got %d (%+v)", len(events), events)
	}
	if events[0].Kind != "mvp_emerged" || events[0].CardName != "Sol Ring" {
		t.Errorf("position 0: %+v", events[0])
	}
	if events[1].Kind != "game_end" {
		t.Errorf("position 1: %+v", events[1])
	}
}

// FK violation: PersistGameObservation rejects writes for a
// gameID that doesn't exist in showmatch_game. Verifies the
// adapter surfaces DB errors rather than swallowing them.
func TestSnapshotDBAdapter_FKViolationReturnsError(t *testing.T) {
	h, _ := newSummaryTestHandler(t)
	adapter := NewSnapshotDBAdapter(h.db)

	err := adapter.PersistGameObservation(context.Background(), 999999, heimdall.GameObservationSnapshot{
		Seed: heimdall.GameSeed{RNGSeed: 1},
	})
	if err == nil {
		t.Fatal("expected FK violation for unknown gameID, got nil")
	}
	// Must NOT be sql.ErrNoRows — that would mean the adapter
	// silently treated FK failure as missing-row.
	if errors.Is(err, sql.ErrNoRows) {
		t.Errorf("FK violation surfaced as ErrNoRows: %v", err)
	}
}

// And finally: live persistence followed by the /api/games/{id}/summary
// endpoint returns the rich-path GameSummary, confirming end-to-end
// that the live observer's writes feed the read-side rich path.
func TestSnapshotDBAdapter_PersistedSnapshotPowersSummaryEndpoint(t *testing.T) {
	h, mux := newSummaryTestHandler(t)
	gid := seedSummaryGame(t, h, 1, 8, "combat")

	adapter := NewSnapshotDBAdapter(h.db)
	observer := &heimdall.Observer{}
	observer.SetSnapshotSink(adapter)

	observer.RecordSnapshot(context.Background(), gid, heimdall.Observation{
		Seed: heimdall.GameSeed{RNGSeed: 4242, Winner: 1, Turns: 8},
		MVPCards: []heimdall.MVPCard{
			{Seat: 1, CardName: "Atraxa, Praetors' Voice", CMC: 4, Score: 8, IsCommander: true},
		},
		CommanderZoneVisits: []heimdall.CommanderZoneVisit{
			{Seat: 1, CommanderName: "Atraxa, Praetors' Voice", CastCount: 1, InZoneAtEnd: false, Visits: 1},
		},
	}, nil)

	req := httptest.NewRequest("GET", "/api/games/"+intToStr(gid)+"/summary", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary status: want 200, got %d", rec.Code)
	}
	// The summary endpoint returns the rich path when a snapshot
	// exists — verified separately in #180's handler tests. Here
	// we just sanity-check the body length to confirm the rich path
	// fired (it produces a larger body than the db_only path).
	if len(rec.Body.Bytes()) < 200 {
		t.Errorf("rich path should produce a substantive body; got %d bytes: %s",
			len(rec.Body.Bytes()), rec.Body.String())
	}
}
