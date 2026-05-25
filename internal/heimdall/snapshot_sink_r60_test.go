package heimdall

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mockSnapshotSink captures every PersistGameObservation call for
// assertion. Thread-safe so concurrent-write tests can use it.
type mockSnapshotSink struct {
	mu    sync.Mutex
	calls []mockSnapCall
	err   error // if non-nil, returned from every call
}

type mockSnapCall struct {
	gameID int64
	snap   GameObservationSnapshot
}

func (m *mockSnapshotSink) PersistGameObservation(ctx context.Context, gameID int64, snap GameObservationSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockSnapCall{gameID: gameID, snap: snap})
	return m.err
}

func (m *mockSnapshotSink) Calls() []mockSnapCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mockSnapCall, len(m.calls))
	copy(out, m.calls)
	return out
}

func newSinkObserver() *Observer {
	return &Observer{}
}

func newSnapGS(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := gameengine.NewGameState(4, rand.New(rand.NewSource(1)), nil)
	gs.Turn = 7
	gs.CardFirstPlayed = map[string]int{"Sol Ring": 1, "Atraxa, Praetors' Voice": 4}
	gs.Seats[1].CommanderNames = []string{"Atraxa, Praetors' Voice"}
	return gs
}

// Basic wiring: SetSnapshotSink + HasSnapshotSink + a single
// RecordSnapshot call lands on the sink with the correct gameID
// and snapshot contents.
func TestObserver_RecordSnapshot_RoutesToSink(t *testing.T) {
	obs := newSinkObserver()
	if obs.HasSnapshotSink() {
		t.Fatal("fresh observer should report no sink wired")
	}
	mock := &mockSnapshotSink{}
	obs.SetSnapshotSink(mock)
	if !obs.HasSnapshotSink() {
		t.Fatal("after SetSnapshotSink, HasSnapshotSink should be true")
	}

	gs := newSnapGS(t)
	o := Observation{
		Seed: GameSeed{RNGSeed: 42, Winner: 1, Turns: 7},
		MVPCards: []MVPCard{
			{Seat: 1, CardName: "Atraxa, Praetors' Voice", CMC: 4, Score: 8, IsCommander: true},
		},
	}

	obs.RecordSnapshot(context.Background(), 99, o, gs)

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	if calls[0].gameID != 99 {
		t.Errorf("gameID: want 99, got %d", calls[0].gameID)
	}
	if calls[0].snap.Seed.RNGSeed != 42 || calls[0].snap.Seed.Winner != 1 {
		t.Errorf("snap.Seed mismatch: %+v", calls[0].snap.Seed)
	}
	if len(calls[0].snap.MVPCards) != 1 || calls[0].snap.MVPCards[0].CardName != "Atraxa, Praetors' Voice" {
		t.Errorf("MVPCards not carried through: %+v", calls[0].snap.MVPCards)
	}
	if calls[0].snap.CardFirstPlayed["Atraxa, Praetors' Voice"] != 4 {
		t.Errorf("CardFirstPlayed not extracted from gs: %+v", calls[0].snap.CardFirstPlayed)
	}
}

// Incremental persistence: multiple RecordSnapshot calls for the
// same gameID all hit the sink — the UPSERT semantics live on the
// sink, not on the Observer.
func TestObserver_RecordSnapshot_IncrementalCallsAllHitSink(t *testing.T) {
	obs := newSinkObserver()
	mock := &mockSnapshotSink{}
	obs.SetSnapshotSink(mock)

	gs := newSnapGS(t)

	// Turn 3 — early state.
	gs.Turn = 3
	obs.RecordSnapshot(context.Background(), 7, Observation{
		Seed: GameSeed{RNGSeed: 1, Winner: -1, Turns: 3},
	}, gs)
	// Turn 5 — mid-game.
	gs.Turn = 5
	obs.RecordSnapshot(context.Background(), 7, Observation{
		Seed: GameSeed{RNGSeed: 1, Winner: -1, Turns: 5},
	}, gs)
	// Turn 7 — game end.
	gs.Turn = 7
	obs.RecordSnapshot(context.Background(), 7, Observation{
		Seed:     GameSeed{RNGSeed: 1, Winner: 1, Turns: 7},
		MVPCards: []MVPCard{{Seat: 1, CardName: "Atraxa, Praetors' Voice", Score: 8}},
	}, gs)

	calls := mock.Calls()
	if len(calls) != 3 {
		t.Fatalf("want 3 incremental calls, got %d", len(calls))
	}
	// Each call should reflect a fresh snapshot of the game's state
	// at the time it was made. FinalTurn comes from gs.Turn at call
	// time.
	wantFinal := []int{3, 5, 7}
	for i, c := range calls {
		if c.gameID != 7 {
			t.Errorf("call %d: gameID should stay 7, got %d", i, c.gameID)
		}
		if c.snap.FinalTurn != wantFinal[i] {
			t.Errorf("call %d: FinalTurn want %d, got %d", i, wantFinal[i], c.snap.FinalTurn)
		}
	}
	// Last call carries the winner + MVP.
	last := calls[2]
	if last.snap.Seed.Winner != 1 || len(last.snap.MVPCards) != 1 {
		t.Errorf("final call should carry winner+MVP: %+v", last.snap)
	}
}

// No-op when no sink is wired — RecordSnapshot must not panic and
// must not do any snapshot-building work that could fail (e.g.,
// nil-deref on gs.Seats).
func TestObserver_RecordSnapshot_NoSinkWiredIsNoOp(t *testing.T) {
	obs := newSinkObserver()
	// Pass a deliberately nil gs to make sure the early-return
	// happens before any work that could deref it.
	obs.RecordSnapshot(context.Background(), 99, Observation{Seed: GameSeed{RNGSeed: 1}}, nil)
}

// Sink errors are swallowed (logged) — the live game loop must not
// stall on persistence failure. The test asserts the call doesn't
// panic and a subsequent call still reaches the sink.
func TestObserver_RecordSnapshot_SinkErrorsAreSwallowed(t *testing.T) {
	obs := newSinkObserver()
	mock := &mockSnapshotSink{err: errors.New("DB went away")}
	obs.SetSnapshotSink(mock)

	gs := newSnapGS(t)
	obs.RecordSnapshot(context.Background(), 1, Observation{Seed: GameSeed{RNGSeed: 1}}, gs)

	// Clear the error and try again — sink should still be wired
	// and the second call should land.
	mock.mu.Lock()
	mock.err = nil
	mock.mu.Unlock()
	obs.RecordSnapshot(context.Background(), 2, Observation{Seed: GameSeed{RNGSeed: 2}}, gs)

	calls := mock.Calls()
	if len(calls) != 2 {
		t.Errorf("want 2 calls (one failed, one succeeded); got %d", len(calls))
	}
}

// RecordSnapshot with nil gs degrades — the snapshot still carries
// obs's signals + Seed, but gs-derived fields stay empty. Verifies
// the SnapshotFromObservation nil-gs path is exercised through the
// Observer wiring.
func TestObserver_RecordSnapshot_NilGameStateTolerated(t *testing.T) {
	obs := newSinkObserver()
	mock := &mockSnapshotSink{}
	obs.SetSnapshotSink(mock)

	obs.RecordSnapshot(context.Background(), 5, Observation{
		Seed: GameSeed{RNGSeed: 9, Winner: 0, Turns: 3},
		RegretCards: []RegretCard{
			{Seat: 0, CardName: "Plague Wind", CMC: 9, Reason: "stranded_in_hand"},
		},
	}, nil)

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(calls))
	}
	c := calls[0]
	if len(c.snap.RegretCards) != 1 {
		t.Errorf("obs signals should still carry: %+v", c.snap)
	}
	if c.snap.FinalTurn != 0 || c.snap.CardFirstPlayed != nil {
		t.Errorf("nil-gs fields should stay zero: %+v", c.snap)
	}
}

// Re-wiring the sink is honored: a second SetSnapshotSink replaces
// the first, and subsequent calls land on the new sink.
func TestObserver_SetSnapshotSink_Rewiring(t *testing.T) {
	obs := newSinkObserver()
	first := &mockSnapshotSink{}
	second := &mockSnapshotSink{}

	obs.SetSnapshotSink(first)
	obs.RecordSnapshot(context.Background(), 1, Observation{Seed: GameSeed{RNGSeed: 1}}, nil)

	obs.SetSnapshotSink(second)
	obs.RecordSnapshot(context.Background(), 2, Observation{Seed: GameSeed{RNGSeed: 2}}, nil)

	if len(first.Calls()) != 1 {
		t.Errorf("first sink should have 1 call; got %d", len(first.Calls()))
	}
	if len(second.Calls()) != 1 {
		t.Errorf("second sink should have 1 call; got %d", len(second.Calls()))
	}

	// Setting nil disables.
	obs.SetSnapshotSink(nil)
	obs.RecordSnapshot(context.Background(), 3, Observation{Seed: GameSeed{RNGSeed: 3}}, nil)
	if len(second.Calls()) != 1 {
		t.Errorf("after SetSnapshotSink(nil), second sink should still have 1 call; got %d",
			len(second.Calls()))
	}
}
