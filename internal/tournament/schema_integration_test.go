package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
)

// loadDecksOrSkip parses N decks from the shared corpus. If fewer
// real decks are available, parsed decks are duplicated (with fresh
// parse calls so libraries don't alias) until N entries are
// returned — the schema validator doesn't care about deck identity,
// it cares that the producing Run* code path emits a structurally
// correct result. Skips when the corpus isn't loadable at all.
func loadDecksOrSkip(t *testing.T, n int) []*deckparser.TournamentDeck {
	t.Helper()
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, n)
	if len(paths) == 0 {
		t.Skip("no deck files available")
	}
	decks := make([]*deckparser.TournamentDeck, 0, n)
	for i := 0; i < n; i++ {
		p := paths[i%len(paths)]
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		decks = append(decks, d)
	}
	return decks
}

// TestSchema_RealRotateModeValidates runs the default rotate-mode
// path and confirms ValidateSchema(result) == nil. This is the
// canonical "happy path" most tournament consumers see.
func TestSchema_RealRotateModeValidates(t *testing.T) {
	decks := loadDecksOrSkip(t, 4)
	r, err := Run(TournamentConfig{
		Decks:         decks,
		NSeats:        4,
		NGames:        4, // small but enough to hit every rotation
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Mode != ModeRotate {
		t.Errorf("Mode = %q, want %q", r.Mode, ModeRotate)
	}
	if err := ValidateSchema(r); err != nil {
		t.Errorf("rotate-mode result failed schema validation: %v", err)
	}
}

// TestSchema_RealPoolModeValidates runs the pool-mode path.
func TestSchema_RealPoolModeValidates(t *testing.T) {
	decks := loadDecksOrSkip(t, 6)
	r, err := Run(TournamentConfig{
		Decks:         decks,
		NSeats:        4,
		NGames:        4,
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
		PoolMode:      true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.Mode != ModePool {
		t.Errorf("Mode = %q, want %q", r.Mode, ModePool)
	}
	if err := ValidateSchema(r); err != nil {
		t.Errorf("pool-mode result failed schema validation: %v", err)
	}
}

// TestSchema_RealSwissValidates runs RunSwiss.
func TestSchema_RealSwissValidates(t *testing.T) {
	decks := loadDecksOrSkip(t, 4)
	r, err := RunSwiss(SwissConfig{
		Decks:         decks,
		NSeats:        4,
		Rounds:        2,
		GamesPerPod:   1,
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
	})
	if err != nil {
		t.Fatalf("RunSwiss: %v", err)
	}
	if r.Mode != ModeSwiss {
		t.Errorf("Mode = %q, want %q", r.Mode, ModeSwiss)
	}
	if err := ValidateSchema(r); err != nil {
		t.Errorf("swiss result failed schema validation: %v", err)
	}
	if len(r.SwissStandings) == 0 {
		t.Error("swiss: standings missing")
	}
}

// TestSchema_RealDoubleElimValidates runs RunDoubleElimination.
func TestSchema_RealDoubleElimValidates(t *testing.T) {
	decks := loadDecksOrSkip(t, 4)
	r, err := RunDoubleElimination(DoubleEliminationConfig{
		Decks:         decks,
		NSeats:        4,
		GamesPerPod:   1,
		MaxRounds:     6,
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
	})
	if err != nil {
		t.Fatalf("RunDoubleElimination: %v", err)
	}
	if r.Mode != ModeDoubleElim {
		t.Errorf("Mode = %q, want %q", r.Mode, ModeDoubleElim)
	}
	if err := ValidateSchema(r); err != nil {
		t.Errorf("double-elim result failed schema validation: %v", err)
	}
}

// TestSchema_RealBalancedPoolValidates runs RunBalancedPool.
func TestSchema_RealBalancedPoolValidates(t *testing.T) {
	decks := loadDecksOrSkip(t, 4)
	r, err := RunBalancedPool(BalancedPoolConfig{
		Decks:         decks,
		NSeats:        4,
		GamesPerPod:   1,
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
		Strengths:     []float64{2.0, 1.5, 1.0, 0.5},
	})
	if err != nil {
		t.Fatalf("RunBalancedPool: %v", err)
	}
	if r.Mode != ModeBalancedPool {
		t.Errorf("Mode = %q, want %q", r.Mode, ModeBalancedPool)
	}
	if err := ValidateSchema(r); err != nil {
		t.Errorf("balanced-pool result failed schema validation: %v", err)
	}
	if len(r.BalancedPodPlan) == 0 {
		t.Error("balanced-pool: pod plan missing")
	}
}

// TestSchema_RealRoundRobinValidates runs RunRoundRobin.
func TestSchema_RealRoundRobinValidates(t *testing.T) {
	decks := loadDecksOrSkip(t, 4)
	r, err := RunRoundRobin(RoundRobinConfig{
		Decks:         decks,
		NSeats:        4,
		GamesPerPod:   1,
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
	})
	if err != nil {
		t.Fatalf("RunRoundRobin: %v", err)
	}
	if r.Mode != ModeRoundRobin {
		t.Errorf("Mode = %q, want %q", r.Mode, ModeRoundRobin)
	}
	if err := ValidateSchema(r); err != nil {
		t.Errorf("round-robin result failed schema validation: %v", err)
	}
}
