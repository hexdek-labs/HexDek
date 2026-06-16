package gameengine

import "testing"

// winbyeffect_outcome_r63_test.go — firespot r63: a §104.2c win-by-effect
// end must mark every opponent Lost (CR §104.2c) so the seat-outcome
// OUTCOME pillar sees no "unresolved_at_end" / "zombie outcome" violation
// (the ungated check that fired ~thousands of times per grinder run on
// win-by-effect bracket games), and so the engine has a complete standing.
func TestWinByEffect_MarksOpponentsLost_NoZombieOutcome(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	gs.SeatOutcome = NewSeatOutcomeChecker()
	for _, s := range gs.Seats {
		s.Life = 20
	}
	gs.Seats[0].Won = true // §104.2c "you win the game"

	if !gs.CheckEnd() {
		t.Fatal("CheckEnd should report the game ended")
	}

	// Every opponent marked Lost → complete standing.
	for i := 1; i < 4; i++ {
		if !gs.Seats[i].Lost {
			t.Fatalf("opponent seat %d must be Lost (CR §104.2c)", i)
		}
	}

	// CheckEnd already ran the game_end consistency snapshot. Re-run it to
	// assert the OUTCOME pillar is clean — no unresolved/zombie seats.
	before := len(gs.SeatOutcome.Violations)
	gs.SeatOutcome.CheckConsistency(gs, "game_end_probe")
	for _, v := range gs.SeatOutcome.Violations[before:] {
		if v.Kind == "unresolved_at_end" || v.Kind == "winner_count" {
			t.Fatalf("seat-outcome %q after complete win-by-effect end: %s", v.Kind, v.Detail)
		}
	}

	// FinalPlacements is a complete 1..4 permutation, winner 1st.
	pl := gs.FinalPlacements()
	if pl[0] != 1 {
		t.Fatalf("winner placement = %d, want 1", pl[0])
	}
	seen := map[int]bool{}
	for _, p := range pl {
		if p < 1 || p > 4 || seen[p] {
			t.Fatalf("placement vector not a complete 1..4 permutation: %v", pl)
		}
		seen[p] = true
	}
}
