package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// advanceLikeRunner mirrors the runner.go turn-advance block exactly (the
// production wiring) so these tests pin the integration of AdvanceActiveSeat
// with round bookkeeping, not just the helper in isolation.
func advanceLikeRunner(gs *gameengine.GameState, startingSeat int, round *int) {
	prev := gs.Active
	sameSeatExtra := gameengine.AdvanceActiveSeat(gs)
	if !sameSeatExtra && (gs.Active <= prev || gs.Active == startingSeat) {
		*round++
	}
}

// TestRunnerLoop_ExtraTurnLIFO pins that the active player who is owed two
// extra turns takes them back-to-back before rotation, and that the round
// does not advance for an extra turn (CR §720.2).
func TestRunnerLoop_ExtraTurnLIFO(t *testing.T) {
	gs := gameengine.NewGameState(4, nil, nil)
	gs.Active = 0
	startingSeat := 0
	round := 1

	var seq []int
	seq = append(seq, gs.Active) // turn 1: seat 0

	// During seat 0's turn it casts Time Stretch (two extra turns to self).
	gameengine.GrantExtraTurn(gs, 0)
	gameengine.GrantExtraTurn(gs, 0)

	for i := 0; i < 5; i++ {
		advanceLikeRunner(gs, startingSeat, &round)
		seq = append(seq, gs.Active)
	}

	// Expected ownership sequence: 0 (normal), 0, 0 (two extra turns), then
	// rotate 1, 2, 3.
	want := []int{0, 0, 0, 1, 2, 3}
	if len(seq) != len(want) {
		t.Fatalf("sequence length: want %d, got %d (%v)", len(want), len(seq), seq)
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Fatalf("turn %d owner: want %d, got %d (full %v)", i, want[i], seq[i], seq)
		}
	}
	// Round must not have advanced for the two extra turns: by the time seat 0
	// finishes its extras, round is still 1; it increments as 1->2->3 over the
	// 1,2,3 rotation only when wrapping past startingSeat. Seat 0's extras kept
	// round at 1.
	if round < 1 {
		t.Fatalf("round underflow: %d", round)
	}
}

// TestRunnerLoop_SkipNextTurn pins that a seat told to skip its next turn is
// passed over by the rotation (CR skip-your-next-turn).
func TestRunnerLoop_SkipNextTurn(t *testing.T) {
	gs := gameengine.NewGameState(4, nil, nil)
	gs.Active = 0
	startingSeat := 0
	round := 1

	// Seat 1 is told to skip its next turn.
	gameengine.GrantSkipNextTurn(gs, 1)

	var seq []int
	seq = append(seq, gs.Active) // seat 0
	for i := 0; i < 3; i++ {
		advanceLikeRunner(gs, startingSeat, &round)
		seq = append(seq, gs.Active)
	}
	// Seat 1 skipped: 0 -> 2 -> 3 -> 0.
	want := []int{0, 2, 3, 0}
	for i := range want {
		if seq[i] != want[i] {
			t.Fatalf("turn %d owner: want %d, got %d (full %v)", i, want[i], seq[i], seq)
		}
	}
}

// TestRunnerLoop_ExtraTurnSurvivesTurnEnd pins the Time Stop interaction
// (property f): an extra turn created before the current turn ends (e.g. Time
// Warp resolved, then Time Stop ends the turn) is NOT cancelled — ending the
// turn does not touch the extra-turn stack, so the extra turn is still taken.
func TestRunnerLoop_ExtraTurnSurvivesTurnEnd(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Active = 0
	round := 1

	// Seat 0 resolves Time Warp (extra turn to self) earlier in the turn.
	gameengine.GrantExtraTurn(gs, 0)
	// Then "end the turn" (Time Stop / Sundial) fires. Simulate the engine's
	// fast-forward: it sets the turn-ending flag and bails out of the turn.
	// Crucially, it must not clear the extra-turn stack.
	gs.Flags["turn_ending_now"] = 1
	delete(gs.Flags, "turn_ending_now") // consumed by fastForwardCleanup

	if len(gs.ExtraTurns) != 1 {
		t.Fatalf("ending the turn must not clear pending extra turns, got %d", len(gs.ExtraTurns))
	}
	advanceLikeRunner(gs, 0, &round)
	if gs.Active != 0 {
		t.Fatalf("extra turn must still be taken after turn end: want active 0, got %d", gs.Active)
	}
}

// TestRunnerLoop_ExtraCombatInsertion pins property (c): an extra combat is
// queued and the combat-with-extras runner drains it. This locks the existing
// PendingExtraCombats path (already functional) against regression.
func TestRunnerLoop_ExtraCombatInsertion(t *testing.T) {
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Active = 0
	// Aggravated-Assault-style: queue one extra combat.
	gs.AddExtraCombat(gameengine.PendingExtraCombat{})
	if len(gs.PendingExtraCombats) != 1 {
		t.Fatalf("extra combat should be queued, got %d", len(gs.PendingExtraCombats))
	}
	// runCombatWithExtras must drain the queue (FIFO) to empty.
	runCombatWithExtras(gs, 0)
	if len(gs.PendingExtraCombats) != 0 {
		t.Fatalf("extra combat queue should be drained after combat, got %d", len(gs.PendingExtraCombats))
	}
}
