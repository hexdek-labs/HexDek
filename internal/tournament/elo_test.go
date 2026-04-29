package tournament

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/deckparser"
)

func TestELO_InitialRatings(t *testing.T) {
	elo := NewELORatings([]string{"A", "B", "C"})
	for _, name := range []string{"A", "B", "C"} {
		if elo.Rating[name] != 1500.0 {
			t.Errorf("%s should start at 1500, got %.1f", name, elo.Rating[name])
		}
	}
}

func TestELO_WinnerGainsLoserLoses(t *testing.T) {
	elo := NewELORatings([]string{"A", "B"})
	elo.Update("A", []string{"A", "B"})

	if elo.Rating["A"] <= 1500 {
		t.Errorf("winner should gain rating, got %.1f", elo.Rating["A"])
	}
	if elo.Rating["B"] >= 1500 {
		t.Errorf("loser should lose rating, got %.1f", elo.Rating["B"])
	}
}

func TestELO_DrawNoChange(t *testing.T) {
	elo := NewELORatings([]string{"A", "B"})
	elo.Update("", []string{"A", "B"})

	// Equal-rated draw should result in no change.
	if math.Abs(elo.Rating["A"]-1500) > 0.01 {
		t.Errorf("draw should not change equal ratings, A=%.3f", elo.Rating["A"])
	}
	if math.Abs(elo.Rating["B"]-1500) > 0.01 {
		t.Errorf("draw should not change equal ratings, B=%.3f", elo.Rating["B"])
	}
}

func TestELO_Multiplayer4(t *testing.T) {
	elo := NewELORatings([]string{"A", "B", "C", "D"})

	// A wins a 4-player game.
	elo.Update("A", []string{"A", "B", "C", "D"})

	if elo.Rating["A"] <= 1500 {
		t.Errorf("4p winner should gain, got %.1f", elo.Rating["A"])
	}
	for _, name := range []string{"B", "C", "D"} {
		if elo.Rating[name] >= 1500 {
			t.Errorf("4p loser %s should lose, got %.1f", name, elo.Rating[name])
		}
	}

	// Losers should lose roughly the same amount (small differences from
	// sequential pairwise update order are expected).
	if math.Abs(elo.Rating["B"]-elo.Rating["C"]) > 1.0 {
		t.Errorf("equal-rated losers should lose roughly equally: B=%.3f C=%.3f",
			elo.Rating["B"], elo.Rating["C"])
	}
}

func TestELO_ConvergenceOverManyGames(t *testing.T) {
	elo := NewELORatings([]string{"Strong", "Weak"})

	// Strong wins 80% of games.
	for i := 0; i < 100; i++ {
		if i%5 == 0 {
			elo.Update("Weak", []string{"Strong", "Weak"})
		} else {
			elo.Update("Strong", []string{"Strong", "Weak"})
		}
	}

	if elo.Rating["Strong"] <= elo.Rating["Weak"] {
		t.Errorf("strong (%.1f) should rate higher than weak (%.1f)",
			elo.Rating["Strong"], elo.Rating["Weak"])
	}

	spread := elo.Rating["Strong"] - elo.Rating["Weak"]
	if spread < 100 {
		t.Errorf("80%% win rate should produce >100 spread, got %.1f", spread)
	}
}

func TestELO_Snapshot_SortedDescending(t *testing.T) {
	elo := NewELORatings([]string{"A", "B", "C"})
	elo.Rating["A"] = 1600
	elo.Rating["B"] = 1400
	elo.Rating["C"] = 1500

	snap := elo.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(snap))
	}
	if snap[0].Commander != "A" || snap[1].Commander != "C" || snap[2].Commander != "B" {
		t.Errorf("snapshot not sorted descending: %v", snap)
	}
}

func TestELO_GamesCounter(t *testing.T) {
	elo := NewELORatings([]string{"A", "B", "C"})
	elo.Update("A", []string{"A", "B"})
	elo.Update("B", []string{"B", "C"})
	elo.Update("A", []string{"A", "B", "C"})

	if elo.Games["A"] != 2 {
		t.Errorf("A played 2 games, got %d", elo.Games["A"])
	}
	if elo.Games["B"] != 3 {
		t.Errorf("B played 3 games, got %d", elo.Games["B"])
	}
	if elo.Games["C"] != 2 {
		t.Errorf("C played 2 games, got %d", elo.Games["C"])
	}
}

func TestELO_IntegratedInTournamentResult(t *testing.T) {
	corpus, meta := loadCorpus(t)
	paths := findDecks(t, 2)
	if len(paths) < 2 {
		t.Skip("need 2 decks")
	}
	nSeats := len(paths)
	if nSeats > 4 {
		nSeats = 4
	}

	decks := make([]*deckparser.TournamentDeck, 0, nSeats)
	for _, p := range paths[:nSeats] {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decks = append(decks, d)
	}

	cfg := TournamentConfig{
		Decks:         decks,
		NSeats:        nSeats,
		NGames:        50,
		Seed:          42,
		Workers:       1,
		CommanderMode: true,
	}

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(result.ELO) == 0 {
		t.Fatal("ELO should be populated after tournament")
	}
	if len(result.ELO) != nSeats {
		t.Errorf("expected %d ELO entries, got %d", nSeats, len(result.ELO))
	}

	// Verify sorted descending.
	for i := 1; i < len(result.ELO); i++ {
		if result.ELO[i].Rating > result.ELO[i-1].Rating {
			t.Errorf("ELO not sorted: %s(%.1f) > %s(%.1f)",
				result.ELO[i].Commander, result.ELO[i].Rating,
				result.ELO[i-1].Commander, result.ELO[i-1].Rating)
		}
	}
}
