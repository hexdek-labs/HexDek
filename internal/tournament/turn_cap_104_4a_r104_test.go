package tournament

// r104 regressions — CR §104.3b eligibility and the §104.4a draw at the
// turn cap. See turn_cap_104_4a_r104.go.

import "testing"

// (1) The defect. A sole survivor sitting at negative life was crowned
// turn_cap_leader with Won=true, because `!Lost` was read as "alive".
func TestTurnCap_SoleSurvivorAtNegativeLife_IsNotCrowned_r104(t *testing.T) {
	gs := canonGame(4, -3, 12, 5, 8)
	gs.Seats[1].Lost = true
	gs.Seats[2].Lost = true
	gs.Seats[3].Lost = true

	w, reason := AdjudicateGameEnd(gs, 4, false)

	if w != -1 {
		t.Errorf("CR 104.3b: a seat at -3 life has lost; it must not win the cap (got winner %d)", w)
	}
	if reason != "turn_cap_all_dead" {
		t.Errorf("CR 104.4a: every remaining player has lost, want turn_cap_all_dead, got %q", reason)
	}
	if gs.Seats[0].Won {
		t.Error("a seat at -3 life must not be marked Won")
	}
}

// (2) Exactly 0 is a loss, not a survivable total. §104.3b says "0 or
// less"; the old loop's -999 sentinel let 0 win a table of negatives.
func TestTurnCap_AllAtOrBelowZero_IsADraw_r104(t *testing.T) {
	gs := canonGame(4, -3, 0, -10, -1)

	w, reason := AdjudicateGameEnd(gs, 4, false)

	if w != -1 {
		t.Errorf("CR 104.4a: no seat is above 0 life, want a draw, got winner %d (life %d)",
			w, gs.Seats[w].Life)
	}
	if reason != "turn_cap_all_dead" {
		t.Errorf("want turn_cap_all_dead, got %q", reason)
	}
}

// (3) NEGATIVE GUARD. An ordinary cap — somebody is genuinely alive —
// must be completely unchanged, including the seat-index tiebreak and
// the Lost/LossReason bookkeeping AdjudicateGameEnd owns.
func TestTurnCap_OrdinaryCapUnchanged_r104(t *testing.T) {
	gs := canonGame(4, 12, 31, 7, 31)
	gs.Seats[2].Lost = true

	w, reason := AdjudicateGameEnd(gs, 4, false)

	if w != 1 || reason != "turn_cap_tie" {
		t.Fatalf("ordinary cap tie: got (%d, %q), want (1, turn_cap_tie)", w, reason)
	}
	if !gs.Seats[0].Lost || gs.Seats[0].LossReason != "turn_cap" {
		t.Errorf("trailing seat 0 should be Lost(turn_cap), got Lost=%v reason=%q",
			gs.Seats[0].Lost, gs.Seats[0].LossReason)
	}
	if !gs.Seats[3].Lost || gs.Seats[3].LossReason != "turn_cap_tie" {
		t.Errorf("tied seat 3 should be Lost(turn_cap_tie), got Lost=%v reason=%q",
			gs.Seats[3].Lost, gs.Seats[3].LossReason)
	}
	if !gs.Seats[1].Won {
		t.Error("seat 1 should be marked Won")
	}
}

// (4) A dead-but-unmarked seat must not outrank a genuinely living one.
// This is the shape that silently changed results: the negative seat
// never wins, but before r104 a seat at 20 life with Lost unset could
// also be beaten by nobody — the risk is the reverse, that filtering
// drops the real winner. Pin that it does not.
func TestTurnCap_LiveSeatStillWinsOverUnmarkedDead_r104(t *testing.T) {
	gs := canonGame(4, 20, 3, -5, -5)

	w, reason := AdjudicateGameEnd(gs, 4, false)

	if w != 0 || reason != "turn_cap_leader" {
		t.Fatalf("got (%d, %q), want (0, turn_cap_leader)", w, reason)
	}
	for _, i := range []int{2, 3} {
		if !gs.Seats[i].Lost {
			t.Errorf("seat %d at negative life should be marked Lost by the cap", i)
		}
	}
}

// (5) The helper itself, which the four hexapi/paritycheck call sites
// use directly and which does NOT mutate state.
func TestTurnCapWinner_IsReadOnly_r104(t *testing.T) {
	gs := canonGame(4, -3, -1, -2, -4)

	w, reason := TurnCapWinner(gs)
	if w != -1 || reason != "turn_cap_all_dead" {
		t.Errorf("all negative: got (%d, %q), want (-1, turn_cap_all_dead)", w, reason)
	}
	for i, s := range gs.Seats {
		if s.Lost || s.Won {
			t.Errorf("TurnCapWinner must not mutate seat %d (Lost=%v Won=%v)", i, s.Lost, s.Won)
		}
	}

	gs2 := canonGame(4, 5, 9, -1, 9)
	w2, r2 := TurnCapWinner(gs2)
	if w2 != 1 || r2 != "turn_cap_leader" {
		t.Errorf("tie on 9 breaks to lowest seat index: got (%d, %q), want (1, turn_cap_leader)", w2, r2)
	}
}
