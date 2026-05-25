package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// evaluator_rebalance_r60_test.go — cross-cutting global eval-weight
// rebalance. After rounds 1-4 archetype-specific tuning, two gaps in
// the rescaleWeights state-stage / position adjustments applied
// uniformly to every archetype (including the midrange fallback used
// by every unknown deck):
//
//   - Late-game LifeResource was never bumped. A long late game
//     (turn 13+) means every seat has taken combat damage, drain
//     engines lethal sooner, and life as a resource is genuinely
//     scarcer — but the evaluator weighted life the same at turn 2
//     and turn 20.
//   - The "ahead" branch bumped CardAdvantage / ManaAdvantage /
//     LifeResource but skipped BoardPresence — so a board-ahead deck
//     valued the next creature the same as a board-behind deck would,
//     contradicting "consolidate the lead."

// helperBuildEvenGame produces a 2-seat game with both seats at
// starting life and empty boards so positionSignal == 0 (neither
// branch fires).
func helperBuildEvenGame(t *testing.T) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40
	return gs
}

// helperPushAheadOnBoard stuffs seat 0 with several attacker permanents
// and leaves seat 1 empty so positionSignal pushes well past 0.3.
func helperPushAheadOnBoard(t *testing.T, gs *gameengine.GameState) {
	t.Helper()
	for i := 0; i < 5; i++ {
		c := newTestCardMinimal("Bear", []string{"creature"}, 2, nil)
		newTestPermanent(gs.Seats[0], c, 3, 3)
	}
}

// -----------------------------------------------------------------------------
// Late-game LifeResource bump
// -----------------------------------------------------------------------------

func TestRescaleWeights_LateGameBoostsLifeResource(t *testing.T) {
	gs := helperBuildEvenGame(t)
	ev := NewEvaluator(nil)

	gs.Turn = 2
	wEarly := ev.rescaleWeights(gs, 0)

	gs.Turn = 25
	wLate := ev.rescaleWeights(gs, 0)

	if wLate.LifeResource <= wEarly.LifeResource {
		t.Errorf("late game LifeResource (%.3f) should exceed early game (%.3f) — life as a resource matters more once combat damage accumulates",
			wLate.LifeResource, wEarly.LifeResource)
	}
}

func TestRescaleWeights_LateGameLifeResourceBumpRespectsLateFactor(t *testing.T) {
	// At turn 12 lateFactor == 0 (boundary). At turn 32+ lateFactor
	// caps at ~1.0. The bump should scale monotonically with turn.
	gs := helperBuildEvenGame(t)
	ev := NewEvaluator(nil)

	gs.Turn = 12
	wBoundary := ev.rescaleWeights(gs, 0)

	gs.Turn = 30
	wFar := ev.rescaleWeights(gs, 0)

	if wFar.LifeResource <= wBoundary.LifeResource {
		t.Errorf("LifeResource should scale UP with late-game progression; turn12=%.3f turn30=%.3f",
			wBoundary.LifeResource, wFar.LifeResource)
	}
}

// -----------------------------------------------------------------------------
// Ahead branch BoardPresence bump
// -----------------------------------------------------------------------------

func TestRescaleWeights_AheadBranchBoostsBoardPresence(t *testing.T) {
	gs := helperBuildEvenGame(t)
	ev := NewEvaluator(nil)

	gs.Turn = 8
	wEven := ev.rescaleWeights(gs, 0)

	// Now make seat 0 well-ahead on board.
	helperPushAheadOnBoard(t, gs)
	wAhead := ev.rescaleWeights(gs, 0)

	if wAhead.BoardPresence <= wEven.BoardPresence {
		t.Errorf("ahead-on-board should boost BoardPresence (extend lead); even=%.3f ahead=%.3f",
			wEven.BoardPresence, wAhead.BoardPresence)
	}
}

func TestRescaleWeights_BehindBranchDoesNotBoostBoardPresenceArm(t *testing.T) {
	// Sanity: the new ahead-branch bump must not accidentally apply to
	// a behind seat. Build a game where seat 0 is behind, then verify
	// BoardPresence isn't above the even baseline.
	gs := helperBuildEvenGame(t)
	ev := NewEvaluator(nil)
	gs.Turn = 8
	wEven := ev.rescaleWeights(gs, 0)

	// Stuff seat 1 to push seat 0 behind.
	for i := 0; i < 5; i++ {
		c := newTestCardMinimal("Bear", []string{"creature"}, 2, nil)
		newTestPermanent(gs.Seats[1], c, 3, 3)
	}
	wBehind := ev.rescaleWeights(gs, 0)

	// BoardPresence at this turn (8, mid-stage) has lateFactor=0.0 so
	// no late bump. The ahead branch must not fire here. Verify equality
	// (within float tolerance) — the rebalance did not bump behind.
	if wBehind.BoardPresence > wEven.BoardPresence+1e-9 {
		t.Errorf("BoardPresence must not increase when behind; even=%.3f behind=%.3f",
			wEven.BoardPresence, wBehind.BoardPresence)
	}
}

// -----------------------------------------------------------------------------
// No regression: existing weight relationships remain
// -----------------------------------------------------------------------------

func TestRescaleWeights_ExistingEarlyVsLateInvariantsHold(t *testing.T) {
	// Re-pin the existing TestEvaluator_DynamicRescaling expectations
	// so the rebalance didn't inadvertently flip the ManaAdvantage or
	// ComboProximity directions.
	gs := helperBuildEvenGame(t)
	ev := NewEvaluator(nil)

	gs.Turn = 2
	wEarly := ev.rescaleWeights(gs, 0)
	gs.Turn = 20
	wLate := ev.rescaleWeights(gs, 0)

	if wEarly.ManaAdvantage <= wLate.ManaAdvantage {
		t.Errorf("early ManaAdvantage (%.3f) must still exceed late (%.3f)",
			wEarly.ManaAdvantage, wLate.ManaAdvantage)
	}
	if wLate.ComboProximity <= wEarly.ComboProximity {
		t.Errorf("late ComboProximity (%.3f) must still exceed early (%.3f)",
			wLate.ComboProximity, wEarly.ComboProximity)
	}
}

func TestRescaleWeights_AheadBranchStillBoostsCardAdvantage(t *testing.T) {
	// Re-pin: the existing ahead-branch CardAdvantage bump must remain.
	gs := helperBuildEvenGame(t)
	ev := NewEvaluator(nil)
	gs.Turn = 8

	wEven := ev.rescaleWeights(gs, 0)
	helperPushAheadOnBoard(t, gs)
	wAhead := ev.rescaleWeights(gs, 0)

	if wAhead.CardAdvantage <= wEven.CardAdvantage {
		t.Errorf("ahead branch must still boost CardAdvantage; even=%.3f ahead=%.3f",
			wEven.CardAdvantage, wAhead.CardAdvantage)
	}
}
