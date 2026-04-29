package gameengine

import (
	"math/rand"
	"testing"
)

func TestDeclareCompanion(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	card := &Card{Name: "Lurrus of the Dream-Den", Owner: 0}
	DeclareCompanion(gs, 0, card)

	if gs.Seats[0].Companion == nil {
		t.Fatal("companion should be set")
	}
	if gs.Seats[0].Companion.Name != "Lurrus of the Dream-Den" {
		t.Fatalf("wrong companion name: %s", gs.Seats[0].Companion.Name)
	}
	if countEvents(gs, "companion_declared") == 0 {
		t.Fatal("missing companion_declared event")
	}
}

func TestMoveCompanionToHand_Success(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	card := &Card{Name: "Lurrus of the Dream-Den", Owner: 0}
	DeclareCompanion(gs, 0, card)

	// Give seat 0 enough mana.
	gs.Seats[0].ManaPool = 5
	gs.Active = 0

	err := MoveCompanionToHand(gs, 0)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if len(gs.Seats[0].Hand) != 1 {
		t.Fatalf("expected 1 card in hand, got %d", len(gs.Seats[0].Hand))
	}
	if gs.Seats[0].Hand[0].Name != "Lurrus of the Dream-Den" {
		t.Fatal("wrong card in hand")
	}
	if gs.Seats[0].ManaPool != 2 {
		t.Fatalf("expected 2 mana remaining, got %d", gs.Seats[0].ManaPool)
	}
	if !gs.Seats[0].CompanionMoved {
		t.Fatal("CompanionMoved should be true")
	}
	if countEvents(gs, "companion_to_hand") == 0 {
		t.Fatal("missing companion_to_hand event")
	}
}

func TestMoveCompanionToHand_InsufficientMana(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	card := &Card{Name: "Lurrus of the Dream-Den", Owner: 0}
	DeclareCompanion(gs, 0, card)

	gs.Seats[0].ManaPool = 2 // less than 3
	gs.Active = 0

	err := MoveCompanionToHand(gs, 0)
	if err == nil {
		t.Fatal("expected error for insufficient mana")
	}
}

func TestMoveCompanionToHand_NotYourTurn(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	card := &Card{Name: "Lurrus of the Dream-Den", Owner: 0}
	DeclareCompanion(gs, 0, card)

	gs.Seats[0].ManaPool = 5
	gs.Active = 1 // not seat 0's turn

	err := MoveCompanionToHand(gs, 0)
	if err == nil {
		t.Fatal("expected error for wrong turn")
	}
}

func TestMoveCompanionToHand_Double(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	gs := NewGameState(2, rng, nil)

	card := &Card{Name: "Lurrus of the Dream-Den", Owner: 0}
	DeclareCompanion(gs, 0, card)

	gs.Seats[0].ManaPool = 10
	gs.Active = 0

	_ = MoveCompanionToHand(gs, 0)
	err := MoveCompanionToHand(gs, 0)
	if err == nil {
		t.Fatal("expected error for double companion move")
	}
}
