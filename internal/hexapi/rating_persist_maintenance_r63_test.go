package hexapi

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/hat"
)

// rating_persist_maintenance_r63_test.go — two rating-integrity guarantees.
//
// (A) Persistence round-trips WITHOUT double-counting: r62 wired
//     flushELO → showmatch_elo → loadPersistedState (incl. ts_mu/ts_sigma).
//     This pins the absolute-count contract: a reload restores games=N,
//     and the NEXT rated game increments to N+1 — never 2N+1.
// (B) The updateELO chokepoint refuses to move ratings in maintenance.
//     r62 halts game loops at their between-turns check; r63 adds the
//     can't-miss guard inside updateELO itself, so a maintenance flip in
//     the final window (or any future caller) cannot corrupt or persist
//     real ratings.

// ratingFixture builds a 2-deck Showmatch with an opened DB and seeded
// in-band ratings (values chosen to pass loadPersistedState's clamps
// unchanged: rating∈[0,4000], μ∈[0,6000], σ∈[40,400]).
func ratingFixture(t *testing.T) (*Showmatch, *sql.DB) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "rating.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sm := &Showmatch{
		sqlDB:        d,
		elo:          map[string]*eloState{},
		bracketCache: map[string]int{},
		strategies:   map[string]*hat.StrategyProfile{},
	}
	sm.elo["deck-a"] = &eloState{rating: 1500, hexRating: 1200, tsMu: 1700, tsSigma: 100, games: 40, wins: 12, commander: "Cmdr A", owner: "tester", bracket: 3}
	sm.elo["deck-b"] = &eloState{rating: 1500, hexRating: 1200, tsMu: 1700, tsSigma: 100, games: 40, wins: 10, commander: "Cmdr B", owner: "tester", bracket: 3}
	return sm, d
}

func loadRecord(t *testing.T, d *sql.DB, key string) db.ELORecord {
	t.Helper()
	recs, err := db.LoadAllELO(context.Background(), d)
	if err != nil {
		t.Fatalf("LoadAllELO: %v", err)
	}
	for _, r := range recs {
		if r.DeckKey == key {
			return r
		}
	}
	t.Fatalf("record %q not found in DB", key)
	return db.ELORecord{}
}

// TestRatingReload_NoDoubleCount (Part A): persist → "restart" reload →
// one more rated game. Games must read N then N+1, proving reload restores
// the ABSOLUTE count rather than re-adding the prior total.
func TestRatingReload_NoDoubleCount(t *testing.T) {
	sm1, d := ratingFixture(t)
	defer d.Close()
	sm1.flushELO()

	// "After restart": a fresh Showmatch loads from the same DB.
	sm2 := &Showmatch{
		sqlDB:        d,
		elo:          map[string]*eloState{},
		bracketCache: map[string]int{},
		strategies:   map[string]*hat.StrategyProfile{},
	}
	sm2.loadPersistedState()

	if g := sm2.elo["deck-a"].games; g != 40 {
		t.Fatalf("reload changed the absolute game count: got %d want 40 (double-count?)", g)
	}
	if w := sm2.elo["deck-a"].wins; w != 12 {
		t.Fatalf("reload changed wins: got %d want 12", w)
	}

	// One more rated game (maintenance off): winner = deck-a.
	sm2.mu.Lock()
	sm2.updateELO([]string{"deck-a", "deck-b"}, []string{"Cmdr A", "Cmdr B"}, nil, 0, 12)
	sm2.mu.Unlock()

	if g := sm2.elo["deck-a"].games; g != 41 {
		t.Fatalf("post-reload game count = %d, want 41 (40 reloaded + 1); a double-count would give 81", g)
	}
	if w := sm2.elo["deck-a"].wins; w != 13 {
		t.Fatalf("post-reload wins = %d, want 13", w)
	}
	if g := sm2.elo["deck-b"].games; g != 41 {
		t.Fatalf("loser game count = %d, want 41", g)
	}
}

// TestMaintenance_UpdateELO_DoesNotMoveRatings (Part B): with maintenance
// ON, updateELO is a no-op — in-memory rating state is untouched and a
// subsequent flush writes no change to the persisted ratings.
func TestMaintenance_UpdateELO_DoesNotMoveRatings(t *testing.T) {
	sm, d := ratingFixture(t)
	defer d.Close()
	sm.flushELO()
	before := loadRecord(t, d, "deck-a")

	// Snapshot in-memory baseline.
	baseGames := sm.elo["deck-a"].games
	baseWins := sm.elo["deck-a"].wins
	baseMu := sm.elo["deck-a"].tsMu
	baseSigma := sm.elo["deck-a"].tsSigma
	baseRating := sm.elo["deck-a"].rating
	baseHex := sm.elo["deck-a"].hexRating

	sm.SetMaintenance(true)
	// Mirror production: every updateELO caller holds sm.mu.Lock().
	sm.mu.Lock()
	got := sm.updateELO([]string{"deck-a", "deck-b"}, []string{"Cmdr A", "Cmdr B"}, nil, 0, 12)
	sm.mu.Unlock()
	if got != nil {
		t.Fatalf("updateELO returned non-nil prior effects in maintenance: %+v", got)
	}

	// In-memory state untouched.
	e := sm.elo["deck-a"]
	if e.games != baseGames || e.wins != baseWins || e.tsMu != baseMu ||
		e.tsSigma != baseSigma || e.rating != baseRating || e.hexRating != baseHex {
		t.Fatalf("maintenance updateELO mutated in-memory rating: games %d→%d wins %d→%d mu %v→%v sigma %v→%v rating %v→%v hex %v→%v",
			baseGames, e.games, baseWins, e.wins, baseMu, e.tsMu, baseSigma, e.tsSigma, baseRating, e.rating, baseHex, e.hexRating)
	}

	// Persisted ratings unchanged after a flush.
	sm.flushELO()
	after := loadRecord(t, d, "deck-a")
	if after.Games != before.Games || after.Wins != before.Wins ||
		after.Rating != before.Rating || after.HexRating != before.HexRating ||
		after.TSMu != before.TSMu || after.TSSigma != before.TSSigma {
		t.Fatalf("maintenance game moved PERSISTED ratings:\n before=%+v\n after =%+v", before, after)
	}
}

// TestMaintenance_UpdateELO_Control proves the no-op above is real and not
// vacuous: the identical fixture with maintenance OFF moves the ratings.
func TestMaintenance_UpdateELO_Control(t *testing.T) {
	sm, d := ratingFixture(t)
	defer d.Close()

	baseGames := sm.elo["deck-a"].games
	baseMu := sm.elo["deck-a"].tsMu

	sm.mu.Lock()
	sm.updateELO([]string{"deck-a", "deck-b"}, []string{"Cmdr A", "Cmdr B"}, nil, 0, 12)
	sm.mu.Unlock()

	if sm.elo["deck-a"].games != baseGames+1 {
		t.Fatalf("control: winner games = %d, want %d (rated path did not run)", sm.elo["deck-a"].games, baseGames+1)
	}
	if sm.elo["deck-a"].wins != 13 {
		t.Fatalf("control: winner wins = %d, want 13", sm.elo["deck-a"].wins)
	}
	if sm.elo["deck-a"].tsMu == baseMu {
		t.Fatalf("control: winner μ did not move from %v — TrueSkill update did not run", baseMu)
	}
}
