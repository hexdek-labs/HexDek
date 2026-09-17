package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// bolasDrainAST mirrors Bolas's Citadel's real ability layout: three
// static abilities, then the single activated drain
// ("{T}, Sacrifice ten nonland permanents: Each opponent loses 10
// life"). The engine passes the RAW index of the activated node — 3.
func bolasDrainAST() *gameast.CardAST {
	return &gameast.CardAST{
		Name: "Bolas's Citadel",
		Abilities: []gameast.Ability{
			&gameast.Static{}, // may_peek_top
			&gameast.Static{}, // play_from_top
			&gameast.Static{}, // if_intervening_tail / cast_alt_path
			&gameast.Activated{
				Cost:   gameast.Cost{Tap: true},
				Effect: &gameast.LoseLife{},
			},
		},
	}
}

// TestBolassCitadel_SacTenDrainFires pins the r64 fix. The prior handler
// switched on abilityIdx 0/1, but the engine passes the raw AST index (3);
// the drain silently never fired. It also aborted on `src.Tapped` — and
// this is a non-mana ability, so the source is already tapped by its paid
// cost when the effect resolves. Both are fixed: the drain fires at the
// real index, tapped, hitting every opponent for 10.
func TestBolassCitadel_SacTenDrainFires(t *testing.T) {
	gs := newGame(t, 4)
	citadel := addPerm(gs, 0, "Bolas's Citadel", "artifact")
	citadel.Card.AST = bolasDrainAST()
	citadel.Tapped = true // engine tapped it as the paid cost

	before := make([]int, len(gs.Seats))
	for i, s := range gs.Seats {
		before[i] = s.Life
	}

	// Activate the drain at its REAL raw index (3).
	bolassCitadelActivate(gs, citadel, 3, nil)

	for i := 1; i < len(gs.Seats); i++ {
		if got := gs.Seats[i].Life; got != before[i]-10 {
			t.Errorf("opponent seat %d life = %d, want %d (lost 10) — the "+
				"drain did not fire at the real index", i, got, before[i]-10)
		}
	}
	if got := gs.Seats[0].Life; got != before[0] {
		t.Errorf("controller life = %d, want %d — controller must not lose life", got, before[0])
	}
	if hasEvent(gs, "per_card_handler") < 1 {
		t.Error("no per_card_handler breadcrumb — handler did not run")
	}
}

// TestBolassCitadel_DrainIgnoresNonDrainNode confirms the node-keying: the
// handler must do nothing when invoked for a non-drain ability index (a
// static), so it can never fire on the wrong ability.
func TestBolassCitadel_DrainIgnoresNonDrainNode(t *testing.T) {
	gs := newGame(t, 4)
	citadel := addPerm(gs, 0, "Bolas's Citadel", "artifact")
	citadel.Card.AST = bolasDrainAST()

	before := gs.Seats[1].Life
	bolassCitadelActivate(gs, citadel, 0, nil) // a Static, not the drain
	if got := gs.Seats[1].Life; got != before {
		t.Errorf("opponent lost life on a non-drain ability index: %d -> %d", before, got)
	}
}
