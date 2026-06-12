package per_card

import (
	"testing"
	"time"

)

// Judge firehose r63: Plargg and Nassari's upkeep handler walked EVERY
// seat's library with an unbounded `for len(Library) > 0 { MoveCard }`
// loop. MoveCard's r62 CR §800.4a chokepoint guard NO-OPS moves for
// cards owned by a player who has left the game, while the eliminated
// seat's zone slices deliberately retain their pointers — so the loop
// spun forever (seed 60606 hung the 800-game firehose 32+ minutes; the
// rare trigger is simply "an opponent was eliminated while Plargg's
// controller still takes upkeeps"). Possibility Storm and Chaos Wand
// carried the same loop shape and got the same no-progress guard.
func TestPlarggNassari_EliminatedSeatLibraryDoesNotHang(t *testing.T) {
	gs := newGame(t, 2)
	gs.Active = 0
	plargg := addPerm(gs, 0, "Plargg and Nassari", "legendary", "creature")
	addLibrary(gs, 0, "A1", "A2") // live controller library: nonland tops

	// Seat 1 was eliminated; its library is retained forensically (all
	// lands, so even without the §800.4a no-op the old loop would have
	// to chew through them — with the no-op it chewed forever).
	addLibrary(gs, 1, "Wastes1", "Wastes2", "Wastes3")
	for _, c := range gs.Seats[1].Library {
		c.Types = []string{"basic", "land"}
	}
	gs.Seats[1].Lost = true
	gs.Seats[1].LeftGame = true
	deadLibBefore := len(gs.Seats[1].Library)

	done := make(chan struct{})
	go func() {
		plarggNassariEra3Upkeep(gs, plargg, map[string]interface{}{"active_seat": 0})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("plarggNassariEra3Upkeep hung on an eliminated seat's retained library (the seed-60606 firehose hang)")
	}

	// The dead seat's forensic zones must be untouched.
	if got := len(gs.Seats[1].Library); got != deadLibBefore {
		t.Errorf("eliminated seat's retained library mutated: %d -> %d", deadLibBefore, got)
	}
	// The live controller's exile still worked (first card exiled —
	// it's a nonland, so exactly one).
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("live seat library = %d, want 1 (one nonland exiled then stop)", len(gs.Seats[0].Library))
	}
}
