package gameengine

// Regression for the silent library-top exile no-op (r63 OUTCOME
// phase-7 finding, 84-card cluster).
//
// "Exile the top card of your library" parses to Exile{Filter.Base:
// "library_top"}. resolveExile had arms for stack spells, graveyard
// cards, and battlefield permanents — but no library arm, so the
// harmful pick found no battlefield permanent of that base and the
// exile silently NO-OPED. The impulse-draw family (Abbot of Keral
// Keep, Aerial Caravan, Alania's Pathmaker, …) never exiled its card,
// leaving the paired play_exiled_cards grant with nothing to play —
// the whole mechanic was dead in live games.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func exileTopTestState(t *testing.T) (*GameState, *Permanent) {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(63)), nil)
	src := &Permanent{
		Card:       &Card{Name: "Abbot", Owner: 0, Types: []string{"creature"}, BasePower: 2, BaseToughness: 1},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)
	for i := 0; i < 4; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{
			Name: "Filler", Owner: 0, Types: []string{"creature"},
		})
	}
	return gs, src
}

func TestExile_LibraryTop_MovesTopCardToExile(t *testing.T) {
	gs, src := exileTopTestState(t)

	ResolveEffect(gs, src, &gameast.Exile{
		Target: gameast.Filter{Base: "library_top", Quantifier: "one",
			Count: gameast.NumInt(1), Targeted: true},
	})

	if got := len(gs.Seats[0].Library); got != 3 {
		t.Errorf("library = %d, want 3 (top card not exiled — the impulse-family no-op)", got)
	}
	if got := len(gs.Seats[0].Exile); got != 1 {
		t.Errorf("exile = %d, want 1", got)
	}
}

func TestExile_LibraryTop_CountAndEmptyLibrary(t *testing.T) {
	gs, src := exileTopTestState(t)

	// Count 3: exiles three.
	ResolveEffect(gs, src, &gameast.Exile{
		Target: gameast.Filter{Base: "library_top", Quantifier: "one",
			Count: gameast.NumInt(3), Targeted: true},
	})
	if got := len(gs.Seats[0].Exile); got != 3 {
		t.Fatalf("exile = %d, want 3", got)
	}

	// Count past the library end: exiles what's there, no panic.
	ResolveEffect(gs, src, &gameast.Exile{
		Target: gameast.Filter{Base: "library_top", Quantifier: "one",
			Count: gameast.NumInt(5), Targeted: true},
	})
	if got := len(gs.Seats[0].Library); got != 0 {
		t.Errorf("library = %d, want 0", got)
	}
	if got := len(gs.Seats[0].Exile); got != 4 {
		t.Errorf("exile = %d, want 4 (3 + the remaining 1)", got)
	}
}
