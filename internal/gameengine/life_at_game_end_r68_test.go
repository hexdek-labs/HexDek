package gameengine

import "testing"

// r68 — CheckEnd must freeze every seat's life AT the decision point.
//
// The Judge reports winners sitting at <=0 life. Whether that is a real
// CR §104.4a draw the engine scored as a win, or merely post-end
// residue, turns entirely on whether the life crossed zero before or
// after the outcome was decided. StateBasedActions returns early once
// ended=1, so after that moment life can fall with nothing marking the
// seat Lost — and the final state alone cannot tell the two apart.

func TestLifeAtGameEndR68_FrozenAtDecisionPointNotAfter(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Life = 17
	gs.Seats[1].Life = 5

	// Seat 1 dies; seat 0 is the last seat standing, healthy.
	gs.Seats[1].Life = 0
	if !gs.CheckEnd() {
		t.Fatal("expected the game to end with one seat at 0 life")
	}
	if gs.Flags["ended"] != 1 {
		t.Fatal("expected ended=1")
	}

	if !gs.Seats[0].LifeAtGameEndSet {
		t.Fatal("decision-point life was not captured for the winner")
	}
	if got := gs.Seats[0].LifeAtGameEnd; got != 17 {
		t.Errorf("LifeAtGameEnd = %d, want 17 (the life at the instant the outcome was decided)", got)
	}

	// Post-end drain — the exact thing that produces the live
	// "winner at -44 life" lines. It must NOT rewrite the record.
	gs.Seats[0].Life = -44
	StateBasedActions(gs)

	if got := gs.Seats[0].LifeAtGameEnd; got != 17 {
		t.Errorf("post-end drain changed LifeAtGameEnd to %d — the snapshot is not frozen, "+
			"so it cannot distinguish residue from a mis-scored game", got)
	}
	if gs.Seats[0].Lost {
		t.Error("post-end SBAs marked the winner Lost; StateBasedActions should early-return once ended=1")
	}
}

// The capture must not fire before there is an outcome, or every
// in-progress game would carry a misleading "decision point".
func TestLifeAtGameEndR68_NotCapturedWhileGameContinues(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Life = 20
	gs.Seats[1].Life = 20

	if gs.CheckEnd() {
		t.Fatal("two healthy seats — the game must not be over")
	}
	for i, s := range gs.Seats {
		if s.LifeAtGameEndSet {
			t.Errorf("seat %d captured a decision-point life while the game is still running", i)
		}
	}
}
