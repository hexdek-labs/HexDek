package hexapi

import (
	"math/rand"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// maintenance_stop_r62_test.go — pins that maintenance mode stops
// ALREADY-RUNNING rated-game loops, not just the start/spawn endpoints
// (review r62, report 06 C3). Pre-fix, a fishtank loop or a spectate room
// alive when HEXDEK_MAINTENANCE came up kept self-playing, rating
// (updateELO), and persisting games. The fix routes every rated-game loop
// through pauseForMaintenance at the top of each iteration.

func TestPauseForMaintenance_OffIsImmediateFalse(t *testing.T) {
	sm := &Showmatch{}
	start := time.Now()
	if sm.pauseForMaintenance() {
		t.Fatal("pauseForMaintenance returned true with maintenance off")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("pauseForMaintenance slept %v with maintenance off — must be a free check on the hot loop path", elapsed)
	}
}

func TestPauseForMaintenance_OnPausesAndReportsTrue(t *testing.T) {
	old := maintenancePollInterval
	maintenancePollInterval = 10 * time.Millisecond
	defer func() { maintenancePollInterval = old }()

	sm := &Showmatch{}
	sm.maintenance.Store(true)
	if !sm.pauseForMaintenance() {
		t.Fatal("pauseForMaintenance returned false with maintenance on")
	}
}

// TestSpectateRoomGameLoop_ParksDuringMaintenance is the load-bearing
// regression: a room's gameLoop must not start a game while maintenance is
// on. The fixture's deck pool holds nil entries and sm.ready is true, so
// any attempt to actually run a game (the pre-fix behavior) panics on the
// first deck dereference — gameLoop's recover() then logs and RETURNS,
// which this test observes as an early exit. A correctly parked loop stays
// alive across many poll intervals and only exits via stopCh.
func TestSpectateRoomGameLoop_ParksDuringMaintenance(t *testing.T) {
	old := maintenancePollInterval
	maintenancePollInterval = 5 * time.Millisecond
	defer func() { maintenancePollInterval = old }()

	sm := &Showmatch{
		ready:    true,
		deckPool: make([]*deckparser.TournamentDeck, showmatchSeats), // nil decks: a started game would panic
	}
	sm.maintenance.Store(true)

	room := &SpectateRoom{
		ID:              "sr-maint-test",
		DeckKey:         "owner/deck",
		sm:              sm,
		rng:             rand.New(rand.NewSource(1)),
		speedMultiplier: 1.0,
		running:         true,
		stopCh:          make(chan struct{}),
		spectators:      make(map[*spectatorConn]struct{}),
	}

	done := make(chan struct{})
	go func() {
		room.gameLoop()
		close(done)
	}()

	// ~20 poll intervals: a parked loop stays alive; the pre-fix loop
	// attempts a game, panics on the nil deck, and exits early.
	select {
	case <-done:
		t.Fatal("gameLoop exited during maintenance — it attempted a game instead of parking")
	case <-time.After(100 * time.Millisecond):
	}

	close(room.stopCh)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("gameLoop did not exit after stopCh closed")
	}
}

// TestSpectateRoomGameLoop_ResumesWhenMaintenanceClears pins the park (not
// kill) semantics: clearing the flag lets the loop attempt games again.
// With the nil-deck fixture that attempt panics and exits the loop — which
// is exactly the observable we want: the loop was parked, then woke up.
func TestSpectateRoomGameLoop_ResumesWhenMaintenanceClears(t *testing.T) {
	old := maintenancePollInterval
	maintenancePollInterval = 5 * time.Millisecond
	defer func() { maintenancePollInterval = old }()

	sm := &Showmatch{
		ready:    true,
		deckPool: make([]*deckparser.TournamentDeck, showmatchSeats),
	}
	sm.maintenance.Store(true)

	room := &SpectateRoom{
		ID:              "sr-maint-resume-test",
		DeckKey:         "owner/deck",
		sm:              sm,
		rng:             rand.New(rand.NewSource(1)),
		speedMultiplier: 1.0,
		running:         true,
		stopCh:          make(chan struct{}),
		spectators:      make(map[*spectatorConn]struct{}),
	}

	done := make(chan struct{})
	go func() {
		room.gameLoop()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("gameLoop exited while parked")
	case <-time.After(50 * time.Millisecond):
	}

	sm.maintenance.Store(false)
	select {
	case <-done: // woke up, attempted a game (panicked on the nil-deck fixture)
	case <-time.After(2 * time.Second):
		t.Fatal("gameLoop never resumed after maintenance cleared")
	}
}
