package hexapi

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// maintenance_trueskill_r62_test.go — reports 06 C3 + 09 C-2.
//
// C3: maintenance mode suppressed the grinder and 503'd new gauntlets /
// spectate rooms, but already-running rated loops (the fishtank and any
// live room) kept playing, rating, and persisting. The loops now poll the
// maintenance flag between turns and halt unrated.
//
// C-2: the showmatch ELO load path reconstructed TrueSkill sigma at its
// sigma_0 default (400) on every boot because mu/sigma were never
// persisted. ts_mu/ts_sigma now round-trip through flushELO →
// loadPersistedState.

// ---------------------------------------------------------------------
// synthetic deck fixtures (no corpus needed)
// ---------------------------------------------------------------------

func syntheticDeck(idx int) *deckparser.TournamentDeck {
	cmdr := &gameengine.Card{
		Name:          fmt.Sprintf("Test Commander %d", idx),
		Types:         []string{"creature", "legendary", "cost:3"},
		BasePower:     3,
		BaseToughness: 3,
	}
	lib := make([]*gameengine.Card, 0, 60)
	for i := 0; i < 36; i++ {
		lib = append(lib, &gameengine.Card{
			Name:  "Forest",
			Types: []string{"land", "basic", "forest"},
		})
	}
	for i := 0; i < 24; i++ {
		lib = append(lib, &gameengine.Card{
			Name:          fmt.Sprintf("Bear %d-%d", idx, i),
			Types:         []string{"creature", "cost:2"},
			BasePower:     2,
			BaseToughness: 2,
		})
	}
	return &deckparser.TournamentDeck{
		Path:           fmt.Sprintf("/tmp/synthetic/deck-%d.txt", idx),
		CommanderName:  cmdr.Name,
		CommanderCards: []*gameengine.Card{cmdr},
		Library:        lib,
	}
}

// maintenanceFixture builds the minimum Showmatch a rated game loop
// touches: a ready 4-deck pool, empty rating state, and an initialized
// curse map (getOrCreateCursePool writes into it).
func maintenanceFixture() *Showmatch {
	sm := &Showmatch{
		ready:     true,
		elo:       map[string]*eloState{},
		cursePool: map[string]*hat.CursePool{},
	}
	for i := 0; i < showmatchSeats; i++ {
		sm.deckPool = append(sm.deckPool, syntheticDeck(i))
	}
	return sm
}

// assertUnrated fails the test if any rating/history mutation happened.
func assertUnrated(t *testing.T, sm *Showmatch, label string) {
	t.Helper()
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.stats.gamesPlayed != 0 {
		t.Fatalf("%s: gamesPlayed = %d, want 0 (game was rated despite maintenance)", label, sm.stats.gamesPlayed)
	}
	if len(sm.gameHistory) != 0 {
		t.Fatalf("%s: gameHistory has %d entries, want 0", label, len(sm.gameHistory))
	}
	for k, e := range sm.elo {
		if e.games != 0 {
			t.Fatalf("%s: elo[%s].games = %d, want 0 (updateELO ran)", label, k, e.games)
		}
	}
}

// TestMaintenance_FishtankGameHaltsUnrated pins report 06 C3 for the
// fishtank loop: with maintenance set, an in-flight runOneGame halts at
// its between-turns check — no ELO update, no history append, no rated
// result of any kind. (The flag is set before the call; the guard under
// test is the per-turn check inside the turn loop, which is the same
// line a mid-game flip would hit.)
func TestMaintenance_FishtankGameHaltsUnrated(t *testing.T) {
	sm := maintenanceFixture()
	sm.SetMaintenance(true)

	sm.runOneGame(rand.New(rand.NewSource(7)))

	assertUnrated(t, sm, "fishtank")
}

// TestMaintenance_SpectateRoomGameHaltsUnrated pins report 06 C3 for a
// live spectate room: a room alive when maintenance flips stops its game
// at the between-turns check without recording a rated result.
func TestMaintenance_SpectateRoomGameHaltsUnrated(t *testing.T) {
	sm := maintenanceFixture()
	room := &SpectateRoom{
		ID:              "test-room",
		DeckKey:         deckKeyFromPath(sm.deckPool[0].Path),
		sm:              sm,
		rng:             rand.New(rand.NewSource(11)),
		stopCh:          make(chan struct{}),
		speedMultiplier: 1000, // null out phase pacing sleeps
	}
	sm.SetMaintenance(true)

	room.runOneSpectateGame()

	assertUnrated(t, sm, "spectate-room")
}

// TestMaintenance_GameRunsAndRatesWhenOff is the control: the identical
// fixture with maintenance OFF plays a full fishtank game and records a
// rated result — proving the fixture exercises the real rated path and
// the halt tests aren't passing vacuously.
func TestMaintenance_GameRunsAndRatesWhenOff(t *testing.T) {
	if testing.Short() {
		t.Skip("full synthetic game; skipped in -short")
	}
	sm := maintenanceFixture()

	sm.runOneGame(rand.New(rand.NewSource(7)))

	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.stats.gamesPlayed != 1 {
		t.Fatalf("control game did not complete: gamesPlayed = %d, want 1", sm.stats.gamesPlayed)
	}
	if len(sm.gameHistory) != 1 {
		t.Fatalf("control game not recorded: gameHistory = %d entries, want 1", len(sm.gameHistory))
	}
}

// ---------------------------------------------------------------------
// report 09 C-2 — TrueSkill mu/sigma restart round-trip
// ---------------------------------------------------------------------

// TestTrueSkill_MuSigmaSurvivesRestart pins the full save→load cycle the
// daily server restart exercises: flushELO persists each deck's raw
// mu/sigma, and loadPersistedState restores them instead of
// reconstructing sigma at sigma_0=400.
func TestTrueSkill_MuSigmaSurvivesRestart(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "sm.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	// "Before restart": a deck with an earned, tight sigma.
	sm1 := &Showmatch{
		sqlDB: d,
		elo: map[string]*eloState{
			"deck-a": {
				rating: 1730.5, hexRating: 1200, tsMu: 1990.25, tsSigma: 61.75,
				games: 412, wins: 130, delta: 4.2, hexDelta: -1.1,
				commander: "Cmdr A", owner: "tester", bracket: 3,
			},
		},
	}
	sm1.flushELO()

	// "After restart": a fresh Showmatch loads from the same database.
	sm2 := &Showmatch{sqlDB: d, elo: map[string]*eloState{}}
	sm2.loadPersistedState()

	e, ok := sm2.elo["deck-a"]
	if !ok {
		t.Fatalf("deck-a not loaded")
	}
	if e.tsMu != 1990.25 || e.tsSigma != 61.75 {
		t.Fatalf("mu/sigma did not survive restart: got (%v, %v) want (1990.25, 61.75) — restart re-injected sigma_0 noise",
			e.tsMu, e.tsSigma)
	}
}

// TestTrueSkill_LegacyRowFallsBackToReconstruction pins the migration
// boundary: a row written before the ts_mu/ts_sigma columns existed
// (ts_sigma == 0) loads via the legacy reconstruction — sigma_0 with
// mu = rating + 3*sigma — exactly the pre-r62 behavior, clamps included.
func TestTrueSkill_LegacyRowFallsBackToReconstruction(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "sm-legacy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	if err := db.UpsertELO(context.Background(), d, db.ELORecord{
		DeckKey: "deck-legacy", Rating: 1500, Games: 50,
		// TSMu / TSSigma zero — pre-migration row.
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	sm := &Showmatch{sqlDB: d, elo: map[string]*eloState{}}
	sm.loadPersistedState()

	e, ok := sm.elo["deck-legacy"]
	if !ok {
		t.Fatalf("deck-legacy not loaded")
	}
	wantSigma := tsDefaultSigma
	wantMu := clampF(1500+3*wantSigma, tsMuFloor, tsMuCeil)
	if e.tsSigma != wantSigma || e.tsMu != wantMu {
		t.Fatalf("legacy fallback wrong: got (mu=%v, sigma=%v) want (mu=%v, sigma=%v)",
			e.tsMu, e.tsSigma, wantMu, wantSigma)
	}
}
