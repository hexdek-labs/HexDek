package gameengine

import "testing"

// TestNoLifeMutationAfterGameEnd pins the §104.1-2 post-end guard on the
// two life-mutation chokepoints. Surfaced by judge sweep round 2 (seed 99
// game 225, layers-seeded run): an end-step per-card trigger chain killed
// the last opponent — ending the game — and the rest of the already-
// executing chain drained the WINNER to life=0, tripping the
// SBACompleteness invariant ("seat has life=0, Lost=false — SBA 704.5a
// missed") on a seat that can no longer legally lose.
func TestNoLifeMutationAfterGameEnd(t *testing.T) {
	gs := newTestGameState(2)
	gs.Seats[0].Lost = true
	gs.Seats[1].Won = true
	gs.Seats[1].Life = 3
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["ended"] = 1

	LoseLife(gs, 1, 5, "post-end trailing handler")
	DealDamage(gs, 1, 5, "post-end trailing handler")

	if gs.Seats[1].Life != 3 {
		t.Errorf("winner's life mutated after game end: %d, want 3", gs.Seats[1].Life)
	}
}

// TestWinnerNotDrainedByMidFlightHandlerChain reproduces the game-225
// shape end to end: a per-card trigger handler whose FIRST effect kills
// the last opponent (game ends, the survivor wins) and whose SECOND
// effect tries to drain the survivor. The trailing drain must no-op and
// the final state must satisfy checkSBACompleteness.
func TestWinnerNotDrainedByMidFlightHandlerChain(t *testing.T) {
	gs := newTestGameState(2)
	gs.Active = 0
	gs.Phase = "ending"
	gs.Seats[0].Life = 10
	gs.Seats[1].Life = 2

	src := &Permanent{
		Card: &Card{
			Name: "Synthetic Drain Chain", Types: []string{"creature"},
			Owner: 0, BasePower: 2, BaseToughness: 2,
		},
		Controller: 0, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, src)

	handler := func(gs *GameState, p *Permanent, ctx map[string]interface{}) {
		// First effect: lethal drain on the opponent. The SBA pass inside
		// the inline resolve marks seat 1 Lost and CheckEnd crowns seat 0.
		LoseLife(gs, 1, 2, "drain chain")
		StateBasedActions(gs)
		gs.CheckEnd()
		// Second effect: the chain keeps executing post-end and tries to
		// drain its own controller (the winner).
		LoseLife(gs, 0, 10, "drain chain recoil")
	}
	PushPerCardTrigger(gs, src, handler, nil)

	if !gs.Seats[1].Lost {
		t.Fatal("seat 1 should have lost to the lethal drain")
	}
	if !gs.Seats[0].Won {
		t.Fatal("seat 0 should have won when the last opponent died")
	}
	if gs.Seats[0].Life != 10 {
		t.Errorf("winner drained post-end: life=%d, want 10", gs.Seats[0].Life)
	}
	if err := checkSBACompleteness(gs); err != nil {
		t.Errorf("SBACompleteness violated after game end: %v", err)
	}
}

// TestSimultaneousZeroLifeIsDrawNotWin pins the CheckEnd pre-sweep
// (CR §704.3 / §104.4a): when the would-be §104.2a winner is itself
// sitting at life ≤ 0 un-SBA'd at game-over evaluation, the pending
// §704.5a loss must apply first and the game is a draw — not a win for
// the seat that happened to be SBA'd last. Exact shape of judge sweep
// round 2 seed 99 game 225 (Zurgo Stormrender end-step cascade).
func TestSimultaneousZeroLifeIsDrawNotWin(t *testing.T) {
	gs := newTestGameState(2)
	gs.Seats[1].Lost = true // last opponent, already marked by §704.5a
	gs.Seats[0].Life = 0    // survivor drained in the same cascade, not yet SBA'd

	if !gs.CheckEnd() {
		t.Fatal("game should end when every seat has lost")
	}
	if gs.Seats[0].Won {
		t.Error("seat 0 declared winner at life=0; want §104.3b draw")
	}
	if !gs.Seats[0].Lost {
		t.Error("seat 0 at life=0 not marked Lost by CheckEnd's §704.5a pre-sweep")
	}
	if err := checkSBACompleteness(gs); err != nil {
		t.Errorf("SBACompleteness violated after draw: %v", err)
	}
}
