package hexapi

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// newStubRoom builds a SpectateRoom with the minimum state TeardownRoom and
// the lifecycle helpers expect, without launching a real game loop.
func newStubRoom(rm *RoomManager, deckKey string) *SpectateRoom {
	roomID := fmt.Sprintf("sr-test-%s", shortHash(deckKey))
	r := &SpectateRoom{
		ID:              roomID,
		DeckKey:         deckKey,
		Commander:       "Test Commander",
		CreatedAt:       time.Now(),
		speedMultiplier: 1.0,
		running:         true,
		stopCh:          make(chan struct{}),
		spectators:      make(map[*spectatorConn]struct{}),
	}
	rm.rooms[roomID] = r
	rm.byDeck[deckKey] = roomID
	return r
}

func TestRoomManager_SpawnOrReuse_ReusesExistingRoom(t *testing.T) {
	rm := NewRoomManager()
	deckKey := "owner/deck-a"

	rm.mu.Lock()
	existing := newStubRoom(rm, deckKey)
	rm.mu.Unlock()

	// Calling SpawnOrReuse with the same deck key should return the stub
	// room without consulting sm.findDeckInPool2 (so a nil sm is safe).
	got, err := rm.SpawnOrReuse(nil, deckKey)
	if err != nil {
		t.Fatalf("SpawnOrReuse returned error for existing deck: %v", err)
	}
	if got != existing {
		t.Fatalf("SpawnOrReuse returned %p, want existing room %p", got, existing)
	}

	if len(rm.rooms) != 1 {
		t.Fatalf("expected 1 room after reuse, got %d", len(rm.rooms))
	}
}

func TestRoomManager_SpawnOrReuse_MaxRoomsCap(t *testing.T) {
	rm := NewRoomManager()

	rm.mu.Lock()
	for i := 0; i < maxSpectateRooms; i++ {
		newStubRoom(rm, fmt.Sprintf("owner/deck-%d", i))
	}
	rm.mu.Unlock()

	if got, want := len(rm.rooms), maxSpectateRooms; got != want {
		t.Fatalf("expected %d rooms before cap test, got %d", want, got)
	}

	// New deck key should be rejected with a cap error. Reaches the cap
	// check before findDeckInPool2 since the deckKey isn't in byDeck.
	_, err := rm.SpawnOrReuse(nil, "owner/overflow")
	if err == nil {
		t.Fatal("expected error when exceeding maxSpectateRooms, got nil")
	}
}

func TestRoomManager_TeardownRoom_RemovesFromIndexes(t *testing.T) {
	rm := NewRoomManager()
	deckKey := "owner/deck-teardown"

	rm.mu.Lock()
	room := newStubRoom(rm, deckKey)
	rm.mu.Unlock()
	roomID := room.ID

	rm.TeardownRoom(roomID)

	rm.mu.RLock()
	_, hasRoom := rm.rooms[roomID]
	mappedID, hasDeck := rm.byDeck[deckKey]
	rm.mu.RUnlock()
	if hasRoom {
		t.Fatalf("room %s still present in rooms map after teardown", roomID)
	}
	if hasDeck {
		t.Fatalf("byDeck still maps %s -> %s after teardown", deckKey, mappedID)
	}

	// stopCh must be closed so the game loop unblocks.
	select {
	case <-room.stopCh:
	default:
		t.Fatal("expected stopCh to be closed after TeardownRoom")
	}

	room.mu.RLock()
	running := room.running
	room.mu.RUnlock()
	if running {
		t.Fatal("room still marked running after TeardownRoom")
	}
}

func TestRoomManager_TeardownRoom_Idempotent(t *testing.T) {
	rm := NewRoomManager()

	rm.mu.Lock()
	room := newStubRoom(rm, "owner/deck-idem")
	rm.mu.Unlock()

	rm.TeardownRoom(room.ID)
	// Second teardown must not double-close stopCh (which would panic).
	rm.TeardownRoom(room.ID)
	rm.TeardownRoom("sr-does-not-exist")
}

func TestRoomManager_TeardownRoom_Concurrent(t *testing.T) {
	rm := NewRoomManager()

	rm.mu.Lock()
	for i := 0; i < 8; i++ {
		newStubRoom(rm, fmt.Sprintf("owner/concurrent-%d", i))
	}
	ids := make([]string, 0, len(rm.rooms))
	for id := range rm.rooms {
		ids = append(ids, id)
	}
	rm.mu.Unlock()

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(2)
		go func(id string) {
			defer wg.Done()
			rm.TeardownRoom(id)
		}(id)
		// Racing teardown against itself catches double-close panics.
		go func(id string) {
			defer wg.Done()
			rm.TeardownRoom(id)
		}(id)
	}
	wg.Wait()

	if len(rm.rooms) != 0 {
		t.Fatalf("expected all rooms torn down, %d remain", len(rm.rooms))
	}
	if len(rm.byDeck) != 0 {
		t.Fatalf("expected byDeck empty, %d remain", len(rm.byDeck))
	}
}

func TestRoomManager_ListRooms_ReportsState(t *testing.T) {
	rm := NewRoomManager()

	rm.mu.Lock()
	r1 := newStubRoom(rm, "owner/list-a")
	r2 := newStubRoom(rm, "owner/list-b")
	rm.mu.Unlock()

	r1.mu.Lock()
	r1.gameNumber = 5
	r1.speedMultiplier = 2.0
	r1.mu.Unlock()
	r2.mu.Lock()
	r2.gameNumber = 1
	r2.speedMultiplier = 0.5
	r2.mu.Unlock()

	infos := rm.ListRooms()
	if len(infos) != 2 {
		t.Fatalf("expected 2 rooms in ListRooms, got %d", len(infos))
	}

	byDeck := map[string]SpectateRoomInfo{}
	for _, info := range infos {
		byDeck[info.DeckKey] = info
	}
	if got := byDeck["owner/list-a"].Game; got != 5 {
		t.Errorf("list-a game number = %d, want 5", got)
	}
	if got := byDeck["owner/list-a"].Speed; got != 2.0 {
		t.Errorf("list-a speed = %v, want 2.0", got)
	}
	if got := byDeck["owner/list-b"].Speed; got != 0.5 {
		t.Errorf("list-b speed = %v, want 0.5", got)
	}
	for _, info := range infos {
		if info.Viewers != 0 {
			t.Errorf("expected 0 viewers on stub room %s, got %d", info.ID, info.Viewers)
		}
	}
}

func TestRoomManager_GetRoom(t *testing.T) {
	rm := NewRoomManager()

	if got := rm.GetRoom("nonexistent"); got != nil {
		t.Fatalf("GetRoom on missing id should return nil, got %p", got)
	}

	rm.mu.Lock()
	room := newStubRoom(rm, "owner/get")
	rm.mu.Unlock()

	if got := rm.GetRoom(room.ID); got != room {
		t.Fatalf("GetRoom returned %p, want %p", got, room)
	}
}

func TestShortHash_Stable(t *testing.T) {
	// Two separate calls with the same input must return the same
	// value (determinism). The temp vars are intentional — staticcheck
	// SA4000 would flag `shortHash("foo") != shortHash("foo")` as
	// "same expression both sides" even though we're verifying that
	// two distinct calls produce identical output.
	a := shortHash("foo")
	b := shortHash("foo")
	if a != b {
		t.Fatal("shortHash is not deterministic")
	}
	if shortHash("foo") == shortHash("bar") {
		t.Fatal("shortHash collision on small inputs")
	}
}
