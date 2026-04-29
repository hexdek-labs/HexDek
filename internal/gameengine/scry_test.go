package gameengine

import (
	"testing"
)

// ---------------------------------------------------------------------------
// P1 #3: Scry / Surveil Tests
// ---------------------------------------------------------------------------

func TestScry_ReordersLibrary(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	seat.Hat = &GreedyHatStub{}

	// Stack library: A (top), B, C, D, E.
	a := &Card{Name: "A", Types: []string{"instant", "cost:1"}}
	b := &Card{Name: "B", Types: []string{"instant", "cost:1"}}
	c := &Card{Name: "C", Types: []string{"instant", "cost:1"}}
	d := &Card{Name: "D", Types: []string{"instant", "cost:1"}}
	e := &Card{Name: "E", Types: []string{"instant", "cost:1"}}
	seat.Library = []*Card{a, b, c, d, e}

	// Scry 3 — GreedyHatStub keeps all on top.
	Scry(gs, 0, 3)

	if len(seat.Library) != 5 {
		t.Fatalf("expected 5 cards in library, got %d", len(seat.Library))
	}
	// First 3 should still be A, B, C (kept on top).
	if seat.Library[0] != a || seat.Library[1] != b || seat.Library[2] != c {
		t.Fatalf("scry top 3 should be A,B,C; got %s,%s,%s",
			seat.Library[0].Name, seat.Library[1].Name, seat.Library[2].Name)
	}
	// D, E unchanged.
	if seat.Library[3] != d || seat.Library[4] != e {
		t.Fatalf("remaining library should be D,E")
	}

	// Verify scry event was logged.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "scry" && ev.Seat == 0 && ev.Amount == 3 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'scry' event in log")
	}
}

func TestScry_ZeroIsNoop(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	a := &Card{Name: "A"}
	seat.Library = []*Card{a}

	Scry(gs, 0, 0)
	if len(seat.Library) != 1 || seat.Library[0] != a {
		t.Fatal("scry 0 should not modify library")
	}
}

func TestScry_MoreThanLibrary(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	seat.Hat = &GreedyHatStub{}
	a := &Card{Name: "A", Types: []string{"instant", "cost:1"}}
	b := &Card{Name: "B", Types: []string{"instant", "cost:1"}}
	seat.Library = []*Card{a, b}

	Scry(gs, 0, 5)
	if len(seat.Library) != 2 {
		t.Fatalf("expected 2 cards after scry 5 with 2 cards, got %d", len(seat.Library))
	}
}

func TestSurveil_PutsCardsInGraveyard(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	// Use a hat that puts expensive cards in graveyard.
	seat.Hat = &GreedyHatStub{}

	a := &Card{Name: "Cheap", Types: []string{"land"}}
	b := &Card{Name: "Expensive", Types: []string{"sorcery", "cost:10"}}
	c := &Card{Name: "Medium", Types: []string{"instant", "cost:5"}}
	d := &Card{Name: "Bottom", Types: []string{"creature", "cost:1"}}
	seat.Library = []*Card{a, b, c, d}

	Surveil(gs, 0, 3)

	// Library should be smaller, graveyard should have cards.
	totalCards := len(seat.Library) + len(seat.Graveyard)
	if totalCards != 4 {
		t.Fatalf("expected 4 total cards (library + graveyard), got %d", totalCards)
	}

	// Verify surveil event was logged.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "surveil" && ev.Seat == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'surveil' event in log")
	}
}

func TestSurveil_EmptyLibrary(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	seat := gs.Seats[0]
	seat.Library = nil

	Surveil(gs, 0, 3)
	if len(seat.Library) != 0 && seat.Library != nil {
		t.Fatal("surveil on empty library should be no-op")
	}
}
