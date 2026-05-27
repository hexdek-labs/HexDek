package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// evaluator_threat_stack_r60_test.go — pins the r60 ThreatExposure
// additions: stack-pending-targeting-me pressure + board-leader wipe-
// magnet penalty. Pre-r60 scoreThreat never read gs.Stack and never
// compared our own board to opponents' — so a Murder aimed at our
// Atraxa, a Counterspell aimed at our wincon cast, or a 30-power
// dominant board all contributed 0.0 to threat.

// freshTwoSeatGame builds a 2-seat Commander game with 40 life apiece.
// Returns the game so tests can populate stacks / boards. Using 2 seats
// rather than 4 keeps lethalRatio math + maxOppPow predictable.
func freshTwoSeatGame(t *testing.T) *gameengine.GameState {
	gs := newTestGame(t, 2)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	return gs
}

// pushThreatStackItem is the local thin wrapper around pushStackItem (the
// shared helper in priority_stack_depth_r60_test.go) — collapses the
// variadic-targets signature into the slice form these tests build.
func pushThreatStackItem(gs *gameengine.GameState, controller int, targets []gameengine.Target) *gameengine.StackItem {
	return pushStackItem(gs, nil, controller, targets...)
}

// -----------------------------------------------------------------------------
// 1. Stack item targeting our seat → +0.30 per item
// -----------------------------------------------------------------------------

func TestScoreThreat_StackItemTargetingMyseat(t *testing.T) {
	gs := freshTwoSeatGame(t)
	ev := NewEvaluator(nil)
	baseline := ev.scoreThreat(gs, 0)

	// Opponent (seat 1) casts Lightning Bolt at us (seat 0).
	pushThreatStackItem(gs, 1, []gameengine.Target{
		{Kind: gameengine.TargetKindSeat, Seat: 0},
	})
	got := ev.scoreThreat(gs, 0)
	delta := baseline - got
	if delta < 0.29 || delta > 0.31 {
		t.Errorf("seat-targeting stack item: want -0.30 delta, got %.4f (baseline %.4f, after %.4f)",
			delta, baseline, got)
	}
}

// Stack item controlled by us should NOT contribute — our own removal
// spell on the stack is good for us.
func TestScoreThreat_OwnStackItemNoLeak(t *testing.T) {
	gs := freshTwoSeatGame(t)
	ev := NewEvaluator(nil)
	baseline := ev.scoreThreat(gs, 0)

	// We (seat 0) cast a spell targeting seat 1.
	pushThreatStackItem(gs, 0, []gameengine.Target{
		{Kind: gameengine.TargetKindSeat, Seat: 1},
	})
	got := ev.scoreThreat(gs, 0)
	if !floatClose(got, baseline) {
		t.Errorf("own stack item leaked into threat: got %.4f, baseline %.4f", got, baseline)
	}
}

// Stack item targeting a DIFFERENT seat (not ours) should NOT contribute.
func TestScoreThreat_StackItemTargetingOtherSeatNoLeak(t *testing.T) {
	gs := newTestGame(t, 3) // need a 3rd seat to be the target
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	ev := NewEvaluator(nil)
	baseline := ev.scoreThreat(gs, 0)

	// Seat 1 casts at seat 2 — has nothing to do with us.
	pushThreatStackItem(gs, 1, []gameengine.Target{
		{Kind: gameengine.TargetKindSeat, Seat: 2},
	})
	got := ev.scoreThreat(gs, 0)
	if !floatClose(got, baseline) {
		t.Errorf("foreign-target stack item leaked: got %.4f, baseline %.4f", got, baseline)
	}
}

// -----------------------------------------------------------------------------
// 2. Stack item targeting our permanent → +0.20 per item
// -----------------------------------------------------------------------------

func TestScoreThreat_StackItemTargetingMyPermanent(t *testing.T) {
	gs := freshTwoSeatGame(t)
	ev := NewEvaluator(nil)

	// We control an Atraxa.
	atraxa := newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Atraxa", []string{"creature", "legendary"}, 4, nil), 4, 4)
	baseline := ev.scoreThreat(gs, 0)

	// Opponent casts Murder targeting Atraxa.
	pushThreatStackItem(gs, 1, []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: atraxa},
	})
	got := ev.scoreThreat(gs, 0)
	delta := baseline - got
	if delta < 0.19 || delta > 0.21 {
		t.Errorf("permanent-targeting stack item: want -0.20 delta, got %.4f (baseline %.4f, after %.4f)",
			delta, baseline, got)
	}
}

// Permanent targeted but controlled by someone else (not us) — should
// not contribute.
func TestScoreThreat_StackItemTargetingNonOwnedPermanentNoLeak(t *testing.T) {
	gs := freshTwoSeatGame(t)
	ev := NewEvaluator(nil)

	// Opponent has its own permanent on the field.
	oppPerm := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Goblin", []string{"creature"}, 1, nil), 1, 1)
	baseline := ev.scoreThreat(gs, 0)

	// Opponent removes its OWN goblin (sacrifice outlet etc.). We don't care.
	pushThreatStackItem(gs, 1, []gameengine.Target{
		{Kind: gameengine.TargetKindPermanent, Permanent: oppPerm},
	})
	got := ev.scoreThreat(gs, 0)
	if !floatClose(got, baseline) {
		t.Errorf("non-owned permanent target leaked: got %.4f, baseline %.4f", got, baseline)
	}
}

// -----------------------------------------------------------------------------
// 3. Stack item targeting our own stack item → +0.35 (counterspell)
// -----------------------------------------------------------------------------

func TestScoreThreat_CounterspellTargetingOurCast(t *testing.T) {
	gs := freshTwoSeatGame(t)
	ev := NewEvaluator(nil)

	// We cast a wincon — it's on the stack first.
	ourCast := pushThreatStackItem(gs, 0, nil)
	baseline := ev.scoreThreat(gs, 0)

	// Opponent casts Counterspell targeting our wincon.
	pushThreatStackItem(gs, 1, []gameengine.Target{
		{Kind: gameengine.TargetKindStackItem, Stack: ourCast},
	})
	got := ev.scoreThreat(gs, 0)
	delta := baseline - got
	if delta < 0.34 || delta > 0.36 {
		t.Errorf("counterspell targeting our cast: want -0.35 delta, got %.4f (baseline %.4f, after %.4f)",
			delta, baseline, got)
	}
}

// -----------------------------------------------------------------------------
// 4. Stack pressure caps at 0.80
// -----------------------------------------------------------------------------

func TestScoreThreat_StackPressureCaps(t *testing.T) {
	gs := freshTwoSeatGame(t)
	ev := NewEvaluator(nil)
	baseline := ev.scoreThreat(gs, 0)

	// 5 opponent stack items each targeting us — would sum to 1.50, must
	// cap at 0.80.
	for i := 0; i < 5; i++ {
		pushThreatStackItem(gs, 1, []gameengine.Target{
			{Kind: gameengine.TargetKindSeat, Seat: 0},
		})
	}
	got := ev.scoreThreat(gs, 0)
	delta := baseline - got
	if delta < 0.79 || delta > 0.81 {
		t.Errorf("stack pressure cap: want -0.80 delta, got %.4f (baseline %.4f, after %.4f)",
			delta, baseline, got)
	}
}

// IsCopy stack items should NOT contribute — the original already
// counted, copies are CR §707.10 ceasing items.
func TestScoreThreat_CopyStackItemSkipped(t *testing.T) {
	gs := freshTwoSeatGame(t)
	ev := NewEvaluator(nil)
	baseline := ev.scoreThreat(gs, 0)

	item := pushThreatStackItem(gs, 1, []gameengine.Target{
		{Kind: gameengine.TargetKindSeat, Seat: 0},
	})
	item.IsCopy = true
	got := ev.scoreThreat(gs, 0)
	if !floatClose(got, baseline) {
		t.Errorf("copy stack item leaked: got %.4f, baseline %.4f", got, baseline)
	}
}

// -----------------------------------------------------------------------------
// 5. Wipe-magnet: board-leader by 2x → -0.30 penalty
// -----------------------------------------------------------------------------

func TestScoreThreat_WipeMagnetWhenBoardLeader(t *testing.T) {
	build := func(myPower, oppPower int) *gameengine.GameState {
		gs := freshTwoSeatGame(t)
		if myPower > 0 {
			newTestPermanent(gs.Seats[0],
				newTestCardMinimal("Big", []string{"creature"}, 4, nil), myPower, myPower)
		}
		if oppPower > 0 {
			newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Small", []string{"creature"}, 2, nil), oppPower, oppPower)
		}
		return gs
	}
	ev := NewEvaluator(nil)

	// Parity-ish boards: 8 vs 6 (ratio 1.33) → no magnet penalty.
	parity := ev.scoreThreat(build(8, 6), 0)

	// Dominant board: 12 vs 4 (ratio 3.0) → -0.30 magnet.
	dominant := ev.scoreThreat(build(12, 4), 0)

	deltaToParity := parity - dominant
	// The dominant board carries MORE wipe magnet but LESS lethal-ratio
	// pressure (we're not being killed). The magnet penalty dominates.
	// Assert: dominant scores worse than parity (delta > 0) by at least
	// ~0.15.
	if deltaToParity < 0.15 {
		t.Errorf("dominant board should score worse than parity by wipe-magnet penalty; parity=%.4f dominant=%.4f delta=%.4f",
			parity, dominant, deltaToParity)
	}
}

// Below the 6-power floor, no wipe-magnet penalty even at high ratios
// (early game, tiny boards).
func TestScoreThreat_NoWipeMagnetBelowBoardFloor(t *testing.T) {
	gs := freshTwoSeatGame(t)
	// Tiny board, no opponent presence: 3 power vs 1 power (3x ratio
	// but below the 6-power floor).
	newTestPermanent(gs.Seats[0],
		newTestCardMinimal("Squire", []string{"creature"}, 2, nil), 3, 3)
	newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Bird", []string{"creature"}, 1, nil), 1, 1)
	ev := NewEvaluator(nil)
	withTiny := ev.scoreThreat(gs, 0)

	// Empty board baseline.
	gs2 := freshTwoSeatGame(t)
	emptyBoard := ev.scoreThreat(gs2, 0)

	// The two should differ ONLY by the lethal-ratio change from the
	// opponent's 1 power — NOT by a wipe-magnet hit. Specifically the
	// score should stay well above (less negative than) -0.30.
	if withTiny < emptyBoard-0.25 {
		t.Errorf("tiny board should NOT trigger wipe magnet; emptyBoard=%.4f withTiny=%.4f",
			emptyBoard, withTiny)
	}
}
