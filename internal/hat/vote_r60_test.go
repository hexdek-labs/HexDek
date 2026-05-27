package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// vote_r60_test.go — pins the r60 vote-decision primitives:
// MostThreateningOpponent + ChooseVoteAgainstThreat. These are the
// hat-side decision functions for CR §701.32 voting cards (Council's
// Judgment / Magister of Worth / Coercive Portal / Custodi Lich).
// The engine plumbing in keywords_will_of_council.go +
// keywords_councils_dilemma.go has been in place since r41 but has
// no per-card consumers; the hat now exposes the policy a per-card
// handler will plug into.

// fortyLifeAll sets all seats to 40 life so threat readings aren't
// inflated by life-pressure differentials.
func fortyLifeAll(gs *gameengine.GameState) {
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
}

// -----------------------------------------------------------------------------
// MostThreateningOpponent
// -----------------------------------------------------------------------------

func TestMostThreateningOpponent_UsesProfileThreatLevel(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)

	// Seat 2 is the most threatening per profile.
	h.opponentProfiles[1].Archetype = "control"
	h.opponentProfiles[1].ThreatLevel = 0.3
	h.opponentProfiles[2].Archetype = "combo"
	h.opponentProfiles[2].ThreatLevel = 0.9
	h.opponentProfiles[3].Archetype = "aggro"
	h.opponentProfiles[3].ThreatLevel = 0.6

	got := h.MostThreateningOpponent(gs, 0)
	if got != 2 {
		t.Errorf("most-threatening: got seat %d, want 2", got)
	}
}

func TestMostThreateningOpponent_FallsBackToOffensivePower(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)
	// All profiles at 0 threat — fallback path should fire.
	// Seat 3 has the biggest creature on board.
	newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Bear", []string{"creature"}, 2, nil), 2, 2)
	newTestPermanent(gs.Seats[2],
		newTestCardMinimal("Wolf", []string{"creature"}, 3, nil), 3, 3)
	newTestPermanent(gs.Seats[3],
		newTestCardMinimal("Dragon", []string{"creature"}, 6, nil), 6, 6)

	got := h.MostThreateningOpponent(gs, 0)
	if got != 3 {
		t.Errorf("fallback: got seat %d, want 3 (biggest board)", got)
	}
}

func TestMostThreateningOpponent_IgnoresLostSeat(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)
	h.opponentProfiles[1].ThreatLevel = 0.5
	h.opponentProfiles[2].ThreatLevel = 0.9 // highest but lost
	h.opponentProfiles[3].ThreatLevel = 0.4
	gs.Seats[2].Lost = true

	got := h.MostThreateningOpponent(gs, 0)
	if got != 1 {
		t.Errorf("lost seat must be skipped: got seat %d, want 1 (next-highest live)", got)
	}
}

func TestMostThreateningOpponent_NoOpponentsReturnsNegativeOne(t *testing.T) {
	gs := newTestGame(t, 2)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(2)
	gs.Seats[1].Lost = true

	got := h.MostThreateningOpponent(gs, 0)
	if got != -1 {
		t.Errorf("all opps lost: want -1, got %d", got)
	}
}

func TestMostThreateningOpponent_DefensiveNilGame(t *testing.T) {
	h := primedYggdrasilHat(2)
	if got := h.MostThreateningOpponent(nil, 0); got != -1 {
		t.Errorf("nil game: want -1, got %d", got)
	}
}

func TestMostThreateningOpponent_FallsBackToFirstLiveOpponent(t *testing.T) {
	// Profile path returns nothing (all 0), fallback OffensivePower
	// returns nothing (no creatures), but seat 2 IS alive — helper
	// must return a usable index, not -1.
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)
	gs.Seats[1].Lost = true
	// Seats 2 and 3 alive but no creatures. First-live-opp fallback
	// should pick seat 2.
	got := h.MostThreateningOpponent(gs, 0)
	if got != 2 {
		t.Errorf("no-threat fallback: want first living opp (2), got %d", got)
	}
}

// -----------------------------------------------------------------------------
// ChooseVoteAgainstThreat
// -----------------------------------------------------------------------------

func TestChooseVoteAgainstThreat_PicksOptionTargetingThreatLeader(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)
	// Seat 2 is the threat leader.
	h.opponentProfiles[2].Archetype = "combo"
	h.opponentProfiles[2].ThreatLevel = 0.9
	h.opponentProfiles[3].Archetype = "aggro"
	h.opponentProfiles[3].ThreatLevel = 0.5

	// Options: [perm-controlled-by-3, perm-controlled-by-2, perm-controlled-by-3].
	optionSeats := []int{3, 2, 3}
	got := h.ChooseVoteAgainstThreat(gs, 0, optionSeats)
	if got != 1 {
		t.Errorf("vote against threat leader: want index 1 (seat 2), got %d", got)
	}
}

// No option targets the threat leader — picks the first opponent-
// targeting option as a fallback.
func TestChooseVoteAgainstThreat_FallsBackToAnyOpponent(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)
	h.opponentProfiles[2].ThreatLevel = 0.9 // threat leader
	h.opponentProfiles[3].ThreatLevel = 0.5

	// No option targets seat 2; must pick first opp-targeting one.
	optionSeats := []int{0, 3, 3} // [self, opp, opp]
	got := h.ChooseVoteAgainstThreat(gs, 0, optionSeats)
	if got != 1 {
		t.Errorf("fallback to any opp: want index 1 (first non-self / non-(-1)), got %d", got)
	}
}

// All options target us — return 0 (engine demands an index, abstain is separate).
func TestChooseVoteAgainstThreat_AllSelfTargeted(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)
	optionSeats := []int{0, 0, 0}
	got := h.ChooseVoteAgainstThreat(gs, 0, optionSeats)
	if got != 0 {
		t.Errorf("all-self options: want 0 fallback, got %d", got)
	}
}

// Mix of -1 (no-seat-target) options and seat-target options — the
// -1 entries should be skipped during fallback.
func TestChooseVoteAgainstThreat_SkipsMinusOneInFallback(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)
	h.opponentProfiles[2].ThreatLevel = 0.9 // threat leader
	// No option targets seat 2; fallback should skip -1 entries.
	optionSeats := []int{-1, -1, 3}
	got := h.ChooseVoteAgainstThreat(gs, 0, optionSeats)
	if got != 2 {
		t.Errorf("fallback should skip -1 entries: want 2, got %d", got)
	}
}

func TestChooseVoteAgainstThreat_EmptyOptionsReturnsZero(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)
	got := h.ChooseVoteAgainstThreat(gs, 0, nil)
	if got != 0 {
		t.Errorf("nil options: want 0 (defensive), got %d", got)
	}
}

// -----------------------------------------------------------------------------
// Integration: Council's Judgment-shaped scenario
// -----------------------------------------------------------------------------

// Council's Judgment exiles the permanent that gets the most votes.
// Realistic scenario: 4 seats, each opponent has a permanent up for
// vote. The hat at seat 0 should pick the option corresponding to
// the most-threatening opponent's permanent.
func TestChooseVoteAgainstThreat_CouncilsJudgmentShape(t *testing.T) {
	gs := newTestGame(t, 4)
	fortyLifeAll(gs)
	h := primedYggdrasilHat(4)

	// Build threat profile: seat 3 has a combo deck about to win.
	h.opponentProfiles[1].Archetype = "aggro"
	h.opponentProfiles[1].ThreatLevel = 0.4
	h.opponentProfiles[2].Archetype = "control"
	h.opponentProfiles[2].ThreatLevel = 0.2
	h.opponentProfiles[3].Archetype = "combo"
	h.opponentProfiles[3].ThreatLevel = 0.95

	// Options: one permanent per opponent. The handler does the
	// permanent → controller mapping; we just consume the seat slice.
	optionSeats := []int{1, 2, 3}
	got := h.ChooseVoteAgainstThreat(gs, 0, optionSeats)
	if got != 2 {
		t.Errorf("Council's Judgment vs combo leader: want index 2 (seat 3 permanent), got %d", got)
	}
}
