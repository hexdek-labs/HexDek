package gameengine

import "testing"

// TestGrantConsumeExtraTurn_LIFO pins CR §720.2: the most recently created
// extra turn is taken first.
func TestGrantConsumeExtraTurn_LIFO(t *testing.T) {
	gs := NewGameState(4, nil, nil)

	GrantExtraTurn(gs, 1)
	GrantExtraTurn(gs, 2)
	GrantExtraTurn(gs, 1)
	if got := gs.Flags["extra_turns_pending"]; got != 3 {
		t.Fatalf("counter mirror: want 3, got %d", got)
	}
	if got := len(gs.ExtraTurns); got != 3 {
		t.Fatalf("stack len: want 3, got %d", got)
	}

	// LIFO: 1 (last), then 2, then 1.
	want := []int{1, 2, 1}
	for i, w := range want {
		seat, ok := ConsumeExtraTurn(gs)
		if !ok {
			t.Fatalf("pop %d: expected ok", i)
		}
		if seat != w {
			t.Fatalf("pop %d: want seat %d, got %d", i, w, seat)
		}
		if gs.Flags["extra_turns_pending"] != len(gs.ExtraTurns) {
			t.Fatalf("pop %d: counter mirror drift: flag=%d len=%d", i,
				gs.Flags["extra_turns_pending"], len(gs.ExtraTurns))
		}
	}
	if _, ok := ConsumeExtraTurn(gs); ok {
		t.Fatalf("empty stack must return ok=false")
	}
}

// TestExtraTurnPerSeatMarkerSync pins that the per-seat extra_turn_queued
// marker (read by Brago/Notion-Thief-style handlers) tracks grant/consume.
func TestExtraTurnPerSeatMarkerSync(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	GrantExtraTurn(gs, 1)
	if gs.Seats[1].Flags["extra_turn_queued"] != 1 {
		t.Fatalf("grant: want queued=1, got %d", gs.Seats[1].Flags["extra_turn_queued"])
	}
	ConsumeExtraTurn(gs)
	if gs.Seats[1].Flags["extra_turn_queued"] != 0 {
		t.Fatalf("consume: want queued=0, got %d", gs.Seats[1].Flags["extra_turn_queued"])
	}
}

// TestGrantConsumeSkipTurn pins skip-turn flag stacking + consumption.
func TestGrantConsumeSkipTurn(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	if ConsumeSkipTurn(gs, 0) {
		t.Fatalf("no skip pending: want false")
	}
	GrantSkipNextTurn(gs, 0)
	GrantSkipNextTurn(gs, 0)
	if !ConsumeSkipTurn(gs, 0) {
		t.Fatalf("first skip: want true")
	}
	if !ConsumeSkipTurn(gs, 0) {
		t.Fatalf("second skip: want true")
	}
	if ConsumeSkipTurn(gs, 0) {
		t.Fatalf("exhausted skips: want false")
	}
}

// TestAdvanceActiveSeat_ExtraTurnKeepsSamePlayer pins that an owed extra turn
// keeps the same player active (sameSeatExtraTurn=true) rather than rotating.
func TestAdvanceActiveSeat_ExtraTurnKeepsSamePlayer(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	gs.Active = 0
	GrantExtraTurn(gs, 0)

	same := AdvanceActiveSeat(gs)
	if !same {
		t.Fatalf("extra turn for active player: want sameSeatExtraTurn=true")
	}
	if gs.Active != 0 {
		t.Fatalf("extra turn: active should stay 0, got %d", gs.Active)
	}
	// Next advance has no extra turn pending -> normal rotation.
	same = AdvanceActiveSeat(gs)
	if same {
		t.Fatalf("no extra turn: want sameSeatExtraTurn=false")
	}
	if gs.Active != 1 {
		t.Fatalf("normal rotation: want active 1, got %d", gs.Active)
	}
}

// TestAdvanceActiveSeat_ExtraTurnToOpponent pins the politics case: Time Warp
// targeting an opponent moves the extra turn to that opponent (CR §720.1).
func TestAdvanceActiveSeat_ExtraTurnToOpponent(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	gs.Active = 0
	GrantExtraTurn(gs, 2) // seat 0 gives seat 2 an extra turn

	same := AdvanceActiveSeat(gs)
	if !same {
		t.Fatalf("extra turn to opponent: want sameSeatExtraTurn=true")
	}
	if gs.Active != 2 {
		t.Fatalf("extra turn owner: want active 2, got %d", gs.Active)
	}
}

// TestAdvanceActiveSeat_SkipTurn pins that a seat owing a skipped turn is
// passed over during normal rotation.
func TestAdvanceActiveSeat_SkipTurn(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	gs.Active = 0
	GrantSkipNextTurn(gs, 1) // seat 1 skips its next turn

	same := AdvanceActiveSeat(gs)
	if same {
		t.Fatalf("skip rotation is not an extra turn")
	}
	if gs.Active != 2 {
		t.Fatalf("seat 1 skipped: want active 2, got %d", gs.Active)
	}
	// The skip was consumed: next rotation from 2 lands on 3 then 0 normally.
	if gs.Seats[1].Flags["skip_next_turn"] != 0 {
		t.Fatalf("skip flag should be consumed, got %d", gs.Seats[1].Flags["skip_next_turn"])
	}
}

// TestAdvanceActiveSeat_DeadSeatExtraTurnEvaporates pins that an extra turn
// owed to a seat that has lost/left is discarded, not granted.
func TestAdvanceActiveSeat_DeadSeatExtraTurnEvaporates(t *testing.T) {
	gs := NewGameState(4, nil, nil)
	gs.Active = 0
	gs.Seats[2].Lost = true
	GrantExtraTurn(gs, 2)

	same := AdvanceActiveSeat(gs)
	if same {
		t.Fatalf("dead seat's extra turn must evaporate, not be taken")
	}
	if gs.Active != 1 {
		t.Fatalf("after dropping dead extra turn, rotate to 1, got %d", gs.Active)
	}
}
