package gameengine

import (
	"math/rand"
	"testing"
)

func newVentureGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(3)), nil)
	// Stock the library so room-1 scry / room-4 draw have cards.
	gs.Seats[0].Library = make([]*Card, 0, 12)
	for i := 0; i < 12; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &Card{Name: "Filler", Types: []string{"land"}})
	}
	return gs
}

// CR §704.5t / §309 — completing a dungeon must DURABLY register: the persistent
// "dungeon_completed" marker (read by Ravenloft Adventurer, Acererak, etc.) must
// survive state-based-action passes. Before r63 the §704.5t SBA deleted it, so a
// completed dungeon read as not-completed on the very next SBA.
func TestVenture_CompletionPersistsAcrossSBA(t *testing.T) {
	gs := newVentureGame(t)
	for i := 0; i < 4; i++ { // 4 rooms → complete the dungeon
		VentureIntoDungeon(gs, 0)
	}
	if gs.Seats[0].Flags["dungeon_completed"] == 0 {
		t.Fatal("dungeon should be completed after reaching room 4")
	}
	// The completion must survive an SBA pass (the §704.5t dungeon-removal SBA).
	StateBasedActions(gs)
	if gs.Seats[0].Flags["dungeon_completed"] == 0 {
		t.Fatal("completing a dungeon must durably register — dungeon_completed was wiped by the SBA")
	}
	// And the next venture starts a fresh dungeon at room 1.
	if room := VentureIntoDungeon(gs, 0); room != 1 {
		t.Fatalf("after completion the next venture should start a fresh dungeon at room 1, got %d", room)
	}
}

// The §704.5t completion event is logged exactly once per completion (not on
// every SBA pass).
func TestVenture_CompletionLoggedOncePerDungeon(t *testing.T) {
	gs := newVentureGame(t)
	for i := 0; i < 4; i++ {
		VentureIntoDungeon(gs, 0)
	}
	StateBasedActions(gs)
	StateBasedActions(gs) // extra passes must not re-log
	StateBasedActions(gs)
	if n := countEventsOfKind(gs, "dungeon_completed"); n != 1 {
		t.Fatalf("one dungeon completion should log exactly one dungeon_completed event, got %d", n)
	}

	// Complete a SECOND dungeon → count bumps to 2, logged a second time.
	for i := 0; i < 4; i++ {
		VentureIntoDungeon(gs, 0)
	}
	StateBasedActions(gs)
	if gs.Seats[0].Flags["dungeon_completed"] != 2 {
		t.Fatalf("two completed dungeons should leave dungeon_completed=2, got %d", gs.Seats[0].Flags["dungeon_completed"])
	}
	if n := countEventsOfKind(gs, "dungeon_completed"); n != 2 {
		t.Fatalf("two completions should log two dungeon_completed events, got %d", n)
	}
}

// Initiative ventures into the (same-track) dungeon (property g).
func TestVenture_InitiativeVentures(t *testing.T) {
	gs := newVentureGame(t)
	TakeInitiative(gs, 0)
	if !HasInitiative(gs, 0) {
		t.Fatal("seat 0 should hold the initiative")
	}
	if gs.Seats[0].Flags["dungeon_room"] != 1 {
		t.Fatalf("taking the initiative should venture (enter room 1), got room %d", gs.Seats[0].Flags["dungeon_room"])
	}
}
