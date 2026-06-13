package gameengine

// seat_outcome_test.go — phase-1 regressions for the per-seat win/loss
// self-checker (r63, owner design). Includes the would-have-caught-#1046
// proof: a simulated stolen-permanent vanish trips owned_census_dropped.

import (
	"testing"
)

func outcomeFixture(t *testing.T) *GameState {
	t.Helper()
	gs := newFixtureGame(t)
	gs.SeatOutcome = NewSeatOutcomeChecker()
	return gs
}

func outcomeKinds(gs *GameState) map[string]int {
	out := map[string]int{}
	for _, v := range gs.SeatOutcome.Violations {
		out[v.Kind]++
	}
	return out
}

// --- Part 1: EvaluateSeatOutcome -------------------------------------------

func TestEvaluateSeatOutcome_LossConditions(t *testing.T) {
	gs := newFixtureGame(t)

	gs.Seats[0].Life = -3
	if st, _ := EvaluateSeatOutcome(gs, 0); st != "lost" {
		t.Errorf("life -3: want lost, got %s", st)
	}
	gs.Seats[0].Life = 20

	gs.Seats[0].PoisonCounters = 10
	if st, reason := EvaluateSeatOutcome(gs, 0); st != "lost" {
		t.Errorf("10 poison: want lost, got %s (%s)", st, reason)
	}
	gs.Seats[0].PoisonCounters = 0

	gs.Seats[0].CommanderDamage = map[int]map[string]int{1: {"Geth, Lord of the Vault": 21}}
	if st, _ := EvaluateSeatOutcome(gs, 0); st != "lost" {
		t.Error("21 commander damage: want lost")
	}
	gs.Seats[0].CommanderDamage[1]["Geth, Lord of the Vault"] = 20
	if st, _ := EvaluateSeatOutcome(gs, 0); st != "alive" {
		t.Error("20 commander damage: want alive")
	}
}

func TestEvaluateSeatOutcome_CantLoseGates(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = -6

	// Real Platinum Angel registration on the seat's own battlefield.
	angel := addBattlefield(gs, 0, "Platinum Angel", 4, 4, "artifact", "creature")
	RegisterReplacementsForPermanent(gs, angel)

	if st, reason := EvaluateSeatOutcome(gs, 0); st != "alive" {
		t.Errorf("Platinum Angel must gate the life loss: got %s (%s)", st, reason)
	}

	// Remove the Angel's replacements — the loss re-derives.
	gs.UnregisterReplacementsForPermanent(angel)
	if st, _ := EvaluateSeatOutcome(gs, 0); st != "lost" {
		t.Error("without the Angel the seat must compute lost")
	}
}

// --- Part 2: cross-seat consistency -----------------------------------------

func TestSeatOutcome_LossNotMarkedFlagged(t *testing.T) {
	gs := outcomeFixture(t)
	gs.Seats[1].Life = -4 // engine flag NOT set — simulated missed SBA

	gs.SeatOutcome.CheckConsistency(gs, "sba")

	if outcomeKinds(gs)["loss_not_marked"] == 0 {
		t.Fatal("seat at -4 life with Lost=false must flag loss_not_marked")
	}
}

func TestSeatOutcome_GameEndWinnerCount(t *testing.T) {
	gs := outcomeFixture(t)
	gs.Flags["ended"] = 1
	// Nobody marked Won; seat 1 not even Lost: two violations expected.
	gs.Seats[0].Lost = true

	gs.SeatOutcome.CheckConsistency(gs, "game_end")

	kinds := outcomeKinds(gs)
	if kinds["winner_count"] == 0 {
		t.Error("ended game with 0 winners must flag winner_count")
	}
	if kinds["unresolved_at_end"] == 0 {
		t.Error("ended game with an unresolved seat must flag unresolved_at_end")
	}
}

func TestSeatOutcome_CleanGameNoViolations(t *testing.T) {
	gs := outcomeFixture(t)
	gs.Seats[0].Won = true
	gs.Seats[1].Lost = true
	gs.Seats[1].Life = -2
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Flags["ended"] = 1

	gs.SeatOutcome.CheckConsistency(gs, "game_end")

	if n := len(gs.SeatOutcome.Violations); n != 0 {
		t.Fatalf("clean ended game must produce no violations, got %d: %v",
			n, gs.SeatOutcome.Violations)
	}
}

// --- Part 3: leave-game cleanup verification --------------------------------

// The would-have-caught-#1046 proof: simulate the pre-fix elimination
// sweep (stolen permanent dropped from the battlefield without routing)
// and assert owned_census_dropped fires for the victim seat.
func TestSeatOutcome_EliminationCatchesVanishedStolenCard(t *testing.T) {
	gs := outcomeFixture(t)
	stolen := &Permanent{
		Card:       &Card{Name: "Stolen Bear", Owner: 0, Types: []string{"creature"}},
		Controller: 1, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, stolen)

	gs.SeatOutcome.BeginElimination(gs, 1)
	// Pre-#1046 bug shape: drop the permanent from the slice, route the
	// card NOWHERE.
	gs.Seats[1].Battlefield = gs.Seats[1].Battlefield[:0]
	gs.Seats[1].LeftGame = true
	gs.SeatOutcome.VerifyEliminationCleanup(gs, 1)

	kinds := outcomeKinds(gs)
	if kinds["owned_census_dropped"] == 0 {
		t.Fatal("vanished stolen card must trip owned_census_dropped — this is the #1046 leak shape")
	}
	for _, v := range gs.SeatOutcome.Violations {
		if v.Kind == "owned_census_dropped" && v.Seat != 0 {
			t.Errorf("violation must name the VICTIM seat 0, got %d", v.Seat)
		}
	}
}

// The post-#1046 real elimination path must be violation-free: stolen
// permanent reverts to the owner's exile, leaver's objects leave.
func TestSeatOutcome_RealEliminationClean(t *testing.T) {
	gs := outcomeFixture(t)
	stolen := &Permanent{
		Card:       &Card{Name: "Stolen Bear", Owner: 0, Types: []string{"creature"}},
		Controller: 1, Owner: 0,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
	MintOGInstanceID(gs, stolen.Card)
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, stolen)
	own := addBattlefield(gs, 1, "Own Bear", 2, 2, "creature")
	own.Owner = 1

	HandleSeatElimination(gs, 1) // hooks run inside (Begin + Verify)

	if n := len(gs.SeatOutcome.Violations); n != 0 {
		t.Fatalf("real elimination (post-#1046) must be clean, got %d: %v",
			n, gs.SeatOutcome.Violations)
	}
}

// --- nil-safety --------------------------------------------------------------

func TestSeatOutcome_NilCheckerNoOps(t *testing.T) {
	gs := newFixtureGame(t) // gs.SeatOutcome nil
	gs.Seats[0].Life = -10
	StateBasedActions(gs)        // sba hook must no-op
	HandleSeatElimination(gs, 1) // elimination hooks must no-op
	var c *SeatOutcomeChecker
	c.CheckConsistency(gs, "sba") // explicit nil receiver
	c.BeginElimination(gs, 0)
	c.VerifyEliminationCleanup(gs, 0)
}

// TestSeatOutcome_DrawExemptsWinnerCount pins the r63 long-tail fix: a
// game that ends in a DRAW (CR §104.4 — simultaneous elimination, or a
// mandatory-loop draw) legitimately has 0 winners, and the SeatOutcome
// self-checker must NOT flag winner_count for it. A DECISIVE game that
// ends with no winner (not a draw) must still be flagged.
func TestSeatOutcome_DrawExemptsWinnerCount(t *testing.T) {
	// (1) Drawn game: ended + 0 winners + game_draw set → no winner_count.
	gs := outcomeFixture(t)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	for _, s := range gs.Seats {
		s.Lost = true // simultaneous elimination
	}
	gs.Flags["ended"] = 1
	gs.Flags["game_draw"] = 1
	gs.SeatOutcome.CheckConsistency(gs, "game_end")
	if n := outcomeKinds(gs)["winner_count"]; n != 0 {
		t.Errorf("drawn game (0 winners, game_draw set) wrongly flagged winner_count %d time(s)", n)
	}

	// (2) Decisive game with NO winner and NOT a draw → still flagged (a
	// real "game ended but nobody won" bug must not be masked).
	gs2 := outcomeFixture(t)
	if gs2.Flags == nil {
		gs2.Flags = map[string]int{}
	}
	for _, s := range gs2.Seats {
		s.Lost = true
	}
	gs2.Flags["ended"] = 1
	// no game_draw flag
	gs2.SeatOutcome.CheckConsistency(gs2, "game_end")
	if n := outcomeKinds(gs2)["winner_count"]; n != 1 {
		t.Errorf("ended-with-no-winner non-draw: want 1 winner_count flag, got %d", n)
	}

	// (3) Decisive game with exactly one winner → no winner_count.
	gs3 := outcomeFixture(t)
	if gs3.Flags == nil {
		gs3.Flags = map[string]int{}
	}
	gs3.Seats[0].Won = true
	gs3.Seats[1].Lost = true
	gs3.Flags["ended"] = 1
	gs3.Flags["winner"] = 0
	gs3.SeatOutcome.CheckConsistency(gs3, "game_end")
	if n := outcomeKinds(gs3)["winner_count"]; n != 0 {
		t.Errorf("decisive 1-winner game wrongly flagged winner_count %d time(s)", n)
	}
}
