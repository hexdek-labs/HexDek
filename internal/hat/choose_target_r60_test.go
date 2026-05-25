package hat

import (
	"testing"
)

// choose_target_r60_test.go — pure-function tests for the politics-aware
// threat bias added to ChooseTarget. politicsThreatAdjustment is a
// pure function over (threats, relPos, targetSeat) so the two
// table-politics signals — "hit the leader" when competitive and
// "kingmaker dodge" when behind — can be pinned independently of the
// rest of the ChooseTarget plumbing.

// -----------------------------------------------------------------------------
// Pure-function tests
// -----------------------------------------------------------------------------

func TestPoliticsThreatAdjustment_NoLeader_ReturnsZero(t *testing.T) {
	threats := []seatThreat{
		{Seat: 1, EvalScore: 5.0},
		{Seat: 2, EvalScore: 4.5}, // gap = 0.5, well below leadGap=3.0
	}
	if got := politicsThreatAdjustment(threats, 0.0, 1); got != 0 {
		t.Fatalf("no clear lead → 0, got %v", got)
	}
	if got := politicsThreatAdjustment(threats, 0.0, 2); got != 0 {
		t.Fatalf("no clear lead → 0, got %v", got)
	}
}

func TestPoliticsThreatAdjustment_CompetitiveWithLeader_BoostsLeader(t *testing.T) {
	threats := []seatThreat{
		{Seat: 1, EvalScore: 10.0}, // leader
		{Seat: 2, EvalScore: 5.0},  // runner-up (gap=5)
		{Seat: 3, EvalScore: 3.0},  // also-ran
	}
	// relPos >= -0.3 = competitive.
	if got := politicsThreatAdjustment(threats, 0.0, 1); got != 3.0 {
		t.Fatalf("competitive + leader → +3.0, got %v", got)
	}
	if got := politicsThreatAdjustment(threats, 0.0, 2); got != 0 {
		t.Fatalf("competitive + non-leader → 0, got %v", got)
	}
	if got := politicsThreatAdjustment(threats, 0.0, 3); got != 0 {
		t.Fatalf("competitive + also-ran → 0, got %v", got)
	}
}

func TestPoliticsThreatAdjustment_BehindWithLeader_DodgesLeaderBoostsRunnerup(t *testing.T) {
	threats := []seatThreat{
		{Seat: 1, EvalScore: 10.0}, // leader
		{Seat: 2, EvalScore: 5.0},  // runner-up
		{Seat: 3, EvalScore: 3.0},  // also-ran
	}
	// relPos < -0.3 = behind.
	if got := politicsThreatAdjustment(threats, -0.5, 1); got != -3.0 {
		t.Fatalf("behind + leader → -3.0 (dodge), got %v", got)
	}
	if got := politicsThreatAdjustment(threats, -0.5, 2); got != 2.0 {
		t.Fatalf("behind + runner-up → +2.0, got %v", got)
	}
	if got := politicsThreatAdjustment(threats, -0.5, 3); got != 0 {
		t.Fatalf("behind + also-ran → 0, got %v", got)
	}
}

func TestPoliticsThreatAdjustment_BoundaryRelPos(t *testing.T) {
	threats := []seatThreat{
		{Seat: 1, EvalScore: 10.0},
		{Seat: 2, EvalScore: 5.0},
	}
	// relPos == -0.3 exactly should count as "not behind" (>= -0.3).
	if got := politicsThreatAdjustment(threats, -0.3, 1); got != 3.0 {
		t.Fatalf("relPos at -0.3 boundary should still treat us as competitive (leader → +3.0); got %v", got)
	}
	// Just past the threshold: kingmaker-dodge fires.
	if got := politicsThreatAdjustment(threats, -0.31, 1); got != -3.0 {
		t.Fatalf("relPos -0.31 → behind → leader gets -3.0; got %v", got)
	}
}

func TestPoliticsThreatAdjustment_SingleOpponent_ReturnsZero(t *testing.T) {
	threats := []seatThreat{{Seat: 1, EvalScore: 10.0}}
	if got := politicsThreatAdjustment(threats, -0.5, 1); got != 0 {
		t.Fatalf("len(threats)<2 → 0 (no runner-up to compare), got %v", got)
	}
}

func TestPoliticsThreatAdjustment_TiedTop_NoClearLeader(t *testing.T) {
	threats := []seatThreat{
		{Seat: 1, EvalScore: 10.0},
		{Seat: 2, EvalScore: 10.0}, // tied — no clear leader
		{Seat: 3, EvalScore: 3.0},
	}
	// Whichever seat is found "first" as leader, the runner-up
	// will be the other 10.0, and the gap will be 0 < 3.0.
	if got := politicsThreatAdjustment(threats, 0.0, 1); got != 0 {
		t.Fatalf("tied top → no clear lead → 0; got %v", got)
	}
	if got := politicsThreatAdjustment(threats, 0.0, 2); got != 0 {
		t.Fatalf("tied top → no clear lead → 0; got %v", got)
	}
}

func TestPoliticsThreatAdjustment_LeaderIdentifiedDespiteUnsortedInput(t *testing.T) {
	// politicsThreatAdjustment must NOT assume threats is sorted —
	// callers pass through assessAllThreats's output as-is and its
	// ordering isn't guaranteed by EvalScore.
	threats := []seatThreat{
		{Seat: 3, EvalScore: 3.0},
		{Seat: 1, EvalScore: 10.0}, // leader, but listed second
		{Seat: 2, EvalScore: 5.0},
	}
	if got := politicsThreatAdjustment(threats, 0.0, 1); got != 3.0 {
		t.Fatalf("leader at non-first index should still be identified; got %v", got)
	}
	if got := politicsThreatAdjustment(threats, -0.5, 2); got != 2.0 {
		t.Fatalf("runner-up at non-second index should still be identified; got %v", got)
	}
}

func TestPoliticsThreatAdjustment_SmallButNonzeroLeadStillBelowGap(t *testing.T) {
	threats := []seatThreat{
		{Seat: 1, EvalScore: 10.0},
		{Seat: 2, EvalScore: 7.5}, // gap = 2.5, below leadGap=3.0
	}
	if got := politicsThreatAdjustment(threats, 0.0, 1); got != 0 {
		t.Fatalf("gap below threshold should suppress bias; got %v", got)
	}
}

func TestPoliticsThreatAdjustment_NegativeEvalScores(t *testing.T) {
	// Defensive: EvalScores can sit below zero (losing positions). The
	// helper's top-two scan uses -1e18 sentinels so negative scores
	// must still rank correctly relative to each other.
	threats := []seatThreat{
		{Seat: 1, EvalScore: -1.0},
		{Seat: 2, EvalScore: -5.0},
		{Seat: 3, EvalScore: -10.0},
	}
	// Gap = 4.0 (-1.0 - -5.0) → clear leader.
	if got := politicsThreatAdjustment(threats, 0.0, 1); got != 3.0 {
		t.Fatalf("negative-score leader should still receive +3.0; got %v", got)
	}
}

// -----------------------------------------------------------------------------
// Behavioral sanity: bias additive, doesn't override extreme scores
// -----------------------------------------------------------------------------

func TestPoliticsThreatAdjustment_DoesNotMaskDominantBaseScores(t *testing.T) {
	// Sanity: the politics adjustment is bounded at +/-3.0. A target
	// whose base score is dominated should not be flipped by politics
	// alone when the base score gap is large. We assert the function
	// returns at most +3.0 / -3.0, never more, by sweeping a few
	// scenarios.
	threats := []seatThreat{
		{Seat: 1, EvalScore: 100.0},
		{Seat: 2, EvalScore: 50.0},
	}
	cases := []float64{-1.0, -0.5, -0.3, -0.29, 0.0, 0.5, 1.0}
	for _, rp := range cases {
		for _, seat := range []int{1, 2} {
			adj := politicsThreatAdjustment(threats, rp, seat)
			if adj > 3.0 || adj < -3.0 {
				t.Fatalf("bias outside ±3.0 envelope for relPos=%.2f seat=%d → %v", rp, seat, adj)
			}
		}
	}
}
