package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/seedcontract"
)

// r62 (review 08, C-H2) — the shared elimination / end-adjudication
// helpers are the canonical spec both the live runner and heimdall's
// anti-cheat replay digest from. These tests pin the slot-assignment
// and adjudication rules so neither side can drift without failing.

func canonGame(nSeats int, lives ...int) *gameengine.GameState {
	gs := &gameengine.GameState{
		Seats: make([]*gameengine.Seat, nSeats),
		Flags: map[string]int{},
	}
	for i := range gs.Seats {
		life := 40
		if i < len(lives) {
			life = lives[i]
		}
		gs.Seats[i] = &gameengine.Seat{Idx: i, Life: life}
	}
	return gs
}

func TestElimTracker_SlotAssignment(t *testing.T) {
	gs := canonGame(4)
	tr := NewElimTracker()
	tr.Mark(gs) // nobody lost yet
	for i, s := range tr.Slots {
		if s != -1 {
			t.Fatalf("pre-game slot %d = %d, want -1", i, s)
		}
	}

	// Seat 2 dies first.
	gs.Seats[2].Lost = true
	tr.Mark(gs)
	if tr.Slots[2] != 0 {
		t.Fatalf("first elimination should take slot 0, got %d", tr.Slots[2])
	}

	// Seats 0 and 3 die in the same turn — seat-index order breaks the tie.
	gs.Seats[0].Lost = true
	gs.Seats[3].Lost = true
	tr.Mark(gs)
	if tr.Slots[0] != 1 || tr.Slots[3] != 2 {
		t.Fatalf("same-turn tie must break by seat index: got seat0=%d seat3=%d, want 1,2",
			tr.Slots[0], tr.Slots[3])
	}

	// Winner (seat 1) fills last.
	tr.FillRemaining(gs)
	if tr.Slots[1] != 3 {
		t.Fatalf("winner slot = %d, want 3", tr.Slots[1])
	}
}

func TestAdjudicateGameEnd_NaturalWin(t *testing.T) {
	gs := canonGame(4)
	gs.Seats[0].Lost = true
	gs.Seats[2].Lost = true
	gs.Seats[3].Lost = true
	gs.Flags["ended"] = 1
	gs.Flags["winner"] = 1

	w, reason := AdjudicateGameEnd(gs, 4, true)
	if w != 1 || reason != "last_seat_standing" {
		t.Fatalf("natural win: got (%d, %q), want (1, last_seat_standing)", w, reason)
	}
}

func TestAdjudicateGameEnd_Draw(t *testing.T) {
	gs := canonGame(4)
	gs.Flags["ended"] = 1 // ended without a winner flag
	w, reason := AdjudicateGameEnd(gs, 4, true)
	if w != -1 || reason != "draw" {
		t.Fatalf("draw: got (%d, %q), want (-1, draw)", w, reason)
	}
}

func TestAdjudicateGameEnd_TurnCapLeader(t *testing.T) {
	gs := canonGame(4, 12, 31, 7, 31)
	gs.Seats[2].Lost = true // already dead before the cap

	w, reason := AdjudicateGameEnd(gs, 4, false)
	// Seats 1 and 3 tie on 31 life — lowest seat index wins, the other
	// tied leader is marked turn_cap_tie, the trailing seat turn_cap.
	if w != 1 || reason != "turn_cap_tie" {
		t.Fatalf("cap tie: got (%d, %q), want (1, turn_cap_tie)", w, reason)
	}
	if !gs.Seats[0].Lost || gs.Seats[0].LossReason != "turn_cap" {
		t.Errorf("trailing seat 0 should be Lost(turn_cap), got Lost=%v reason=%q",
			gs.Seats[0].Lost, gs.Seats[0].LossReason)
	}
	if !gs.Seats[3].Lost || gs.Seats[3].LossReason != "turn_cap_tie" {
		t.Errorf("tied seat 3 should be Lost(turn_cap_tie), got Lost=%v reason=%q",
			gs.Seats[3].Lost, gs.Seats[3].LossReason)
	}
	if !gs.Seats[1].Won || gs.Flags["winner"] != 1 || gs.Flags["turn_capped"] != 1 {
		t.Errorf("winner mutations missing: Won=%v winner=%d capped=%d",
			gs.Seats[1].Won, gs.Flags["winner"], gs.Flags["turn_capped"])
	}
}

func TestAdjudicateGameEnd_TurnCapAllDead(t *testing.T) {
	gs := canonGame(2)
	gs.Seats[0].Lost = true
	gs.Seats[1].Lost = true
	w, reason := AdjudicateGameEnd(gs, 2, false)
	if w != -1 || reason != "turn_cap_all_dead" {
		t.Fatalf("all dead: got (%d, %q), want (-1, turn_cap_all_dead)", w, reason)
	}
}

// The seal recipe (Mark per turn → AdjudicateGameEnd → FillRemaining →
// Seal) and the replay recipe (identical calls in replayForOutcome)
// must digest identically for the same game history. This drives the
// full recipe twice over one history and pins digest equality — the
// honest-game round-trip in miniature, with no corpus dependency.
func TestSealAndReplayRecipes_DigestParity(t *testing.T) {
	playHistory := func() (*gameengine.GameState, *ElimTracker) {
		gs := canonGame(4, 3, 40, 9, 1)
		tr := NewElimTracker()
		tr.Mark(gs)
		// Turn 3: seat 0 dies. Turn 7: seats 2 and 3 die. Game ends.
		gs.Seats[0].Lost = true
		tr.Mark(gs)
		gs.Seats[2].Lost = true
		gs.Seats[3].Lost = true
		tr.Mark(gs)
		gs.Flags["ended"] = 1
		gs.Flags["winner"] = 1
		gs.Turn = 7
		return gs, tr
	}

	buildOutcome := func() seedcontract.Outcome {
		gs, tr := playHistory()
		winner, endReason := AdjudicateGameEnd(gs, 4, true)
		tr.FillRemaining(gs)
		return seedcontract.Outcome{
			Winner:           winner,
			Turns:            gs.Turn,
			EndReason:        endReason,
			EliminationOrder: tr.Slots,
			FinalLife:        FinalLifeFromState(gs, 4),
		}
	}

	sealSide := seedcontract.New(seedcontract.Inputs{RNGSeed: 1})
	sealSide.Seal(buildOutcome())
	replaySide := seedcontract.New(seedcontract.Inputs{RNGSeed: 1})
	replaySide.Seal(buildOutcome())

	if sealSide.OutcomeDigest != replaySide.OutcomeDigest {
		t.Fatalf("recipe digest parity broken:\n  seal   %s\n  replay %s",
			sealSide.OutcomeDigest, replaySide.OutcomeDigest)
	}
	// The digest must cover a REAL elimination order (the pre-r62
	// replay hardcoded all -1 — guard against that regressing).
	want := [seedcontract.MaxSeats]int{0, 3, 1, 2}
	if sealSide.Outcome.EliminationOrder != want {
		t.Fatalf("EliminationOrder = %v, want %v", sealSide.Outcome.EliminationOrder, want)
	}
}
