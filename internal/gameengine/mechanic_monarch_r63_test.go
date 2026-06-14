package gameengine

// r63 — Monarch (CR §720) + Initiative (CR §722) designation audit.

import (
	"math/rand"
	"testing"
)

func monGame() *GameState {
	gs := NewGameState(3, rand.New(rand.NewSource(5)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func stockLib(gs *GameState, seat, n int) {
	lib := make([]*Card, 0, n)
	for i := 0; i < n; i++ {
		lib = append(lib, &Card{Name: "Lib", Owner: seat, Types: []string{"land"}})
	}
	gs.Seats[seat].Library = lib
}

// (a) exactly one monarch; (c) combat damage to the monarch steals it.
func TestMonarch_SingleAndCombatSteal(t *testing.T) {
	gs := monGame()
	BecomeMonarch(gs, 0)
	BecomeMonarch(gs, 1) // crown transfers
	if !IsMonarch(gs, 1) || IsMonarch(gs, 0) || IsMonarch(gs, 2) {
		t.Fatalf("exactly one monarch (seat 1) expected; got 0=%v 1=%v 2=%v", IsMonarch(gs, 0), IsMonarch(gs, 1), IsMonarch(gs, 2))
	}
	// Seat 2's creature deals combat damage to the monarch (seat 1) → steal.
	CheckMonarchCombatSteal(gs, 1, 2)
	if !IsMonarch(gs, 2) || IsMonarch(gs, 1) {
		t.Fatalf("combat damage to the monarch should transfer the crown to the attacker (seat 2)")
	}
	// Self-damage does not steal.
	CheckMonarchCombatSteal(gs, 2, 2)
	if !IsMonarch(gs, 2) {
		t.Fatalf("a creature damaging its own controller-monarch must not change the monarch")
	}
}

// (b) the monarch draws ONLY on the monarch's own end step, not every end step.
func TestMonarch_EndStepDrawOnlyOnOwnTurn(t *testing.T) {
	gs := monGame()
	BecomeMonarch(gs, 1)
	stockLib(gs, 1, 10)

	// An opponent's end step (active = seat 0): the monarch must NOT draw.
	gs.Active = 0
	before := len(gs.Seats[1].Hand)
	FireMonarchEndStep(gs)
	if len(gs.Seats[1].Hand) != before {
		t.Fatalf("monarch must NOT draw on a non-monarch's end step (active=0); hand %d → %d", before, len(gs.Seats[1].Hand))
	}

	// The monarch's own end step (active = seat 1): draws exactly one.
	gs.Active = 1
	FireMonarchEndStep(gs)
	if len(gs.Seats[1].Hand) != before+1 {
		t.Fatalf("monarch should draw 1 on its own end step (active=1); hand %d → %d", before, len(gs.Seats[1].Hand))
	}
}

// (d) the designation persists until stolen / holder leaves.
func TestMonarch_PersistsUntilChanged(t *testing.T) {
	gs := monGame()
	BecomeMonarch(gs, 2)
	if !IsMonarch(gs, 2) {
		t.Fatalf("monarch should persist")
	}
	// A non-combat, non-steal event does not change it.
	CheckMonarchCombatSteal(gs, 0, 1) // seat 0 isn't the monarch → no-op
	if !IsMonarch(gs, 2) {
		t.Fatalf("monarch unchanged when a non-monarch is damaged")
	}
}

// (f) initiative: combat damage to the initiative-holder steals it (and the
// taker ventures). This was the missing analogue of the monarch combat-steal.
func TestInitiative_CombatSteal(t *testing.T) {
	gs := monGame()
	stockLib(gs, 0, 10)
	stockLib(gs, 1, 10)
	TakeInitiative(gs, 0)
	if !HasInitiative(gs, 0) {
		t.Fatalf("seat 0 should hold the initiative")
	}
	roomBefore := gs.Seats[1].Flags["dungeon_room"]

	// Seat 1's creature deals combat damage to the initiative-holder (seat 0).
	CheckInitiativeCombatSteal(gs, 0, 1)

	if !HasInitiative(gs, 1) || HasInitiative(gs, 0) {
		t.Fatalf("combat damage to the initiative-holder should transfer it to the attacker (seat 1)")
	}
	if gs.Seats[1].Flags["dungeon_room"] != roomBefore+1 {
		t.Fatalf("taking the initiative should venture into the Undercity (dungeon_room %d → %d)", roomBefore, gs.Seats[1].Flags["dungeon_room"])
	}
}

// (g) monarch and initiative are independent designations.
func TestMonarch_InitiativeIndependent(t *testing.T) {
	gs := monGame()
	BecomeMonarch(gs, 0)
	TakeInitiative(gs, 1)
	if !IsMonarch(gs, 0) || HasInitiative(gs, 0) {
		t.Fatalf("seat 0 should be monarch but not initiative-holder")
	}
	if !HasInitiative(gs, 1) || IsMonarch(gs, 1) {
		t.Fatalf("seat 1 should hold the initiative but not be monarch")
	}
}
