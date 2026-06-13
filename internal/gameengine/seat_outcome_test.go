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

// CR §104.4a: a game in which every remaining player loses
// simultaneously is a legal DRAW (zero winners). Game 395 / seed
// 3950043 of the correctness sweep: the last two living seats both
// died to one Howling Banshee "each player loses 3 life" ETB, so the
// engine recorded a simultaneous_elimination_draw (winner=-1, no seat
// Won). The winner_count check must NOT flag this — every seat is Lost,
// so there is no unresolved seat that "should have won".
func TestSeatOutcome_SimultaneousEliminationDrawIsLegal(t *testing.T) {
	gs := outcomeFixture(t)
	gs.Flags["ended"] = 1
	// Both remaining seats died to the same effect (life <= 0).
	gs.Seats[0].Lost = true
	gs.Seats[0].Life = 0
	gs.Seats[0].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Seats[1].Lost = true
	gs.Seats[1].Life = -1
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"

	gs.SeatOutcome.CheckConsistency(gs, "game_end")

	if n := len(gs.SeatOutcome.Violations); n != 0 {
		t.Fatalf("an all-players-lost draw (CR 104.4a) must produce no violations, got %d: %v",
			n, gs.SeatOutcome.Violations)
	}
}

// A zero-winner end with a seat STILL ALIVE is not a legal draw — it is
// a premature end or a winner that was never marked. winner_count must
// still flag it (distinct from the clean all-lost draw above).
func TestSeatOutcome_ZeroWinnersWithSurvivorFlagged(t *testing.T) {
	gs := outcomeFixture(t)
	gs.Flags["ended"] = 1
	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Seats[0].Life = -2
	// Seat 1 alive, not marked Won — should have won, or the end is
	// premature.
	gs.SeatOutcome.CheckConsistency(gs, "game_end")
	if outcomeKinds(gs)["winner_count"] == 0 {
		t.Error("ended game with 0 winners but a surviving seat must flag winner_count")
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

// r63 eliminated-seat deep sweep — the lost_seat_active stack-item check
// must gate on LeftGame (§800.4a cleanup done), not Lost (loss SBA just
// marked). The ride-along fires at the SBA checkpoint, before CheckEnd ->
// HandleSeatElimination purges the stack, so a seat freshly marked Lost in
// the same pass is legitimately Lost && !LeftGame with an un-purged spell
// for a brief window. Seed-99 game-935 turn-48: Countryside Crusher,
// LeftGame=false — purged moments later. Only a survivor AFTER LeftGame is
// a genuine post-cleanup leak.
func TestSeatOutcome_LostButNotLeftGameStackItemNotFlagged(t *testing.T) {
	gs := outcomeFixture(t)
	gs.Seats[1].Lost = true
	gs.Seats[1].Life = -3
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"
	// §800.4a cleanup pending: LeftGame still false, stack item survives.
	gs.Stack = append(gs.Stack, &StackItem{
		ID: 1, Controller: 1,
		Card: &Card{Name: "Countryside Crusher", Owner: 1},
	})

	gs.SeatOutcome.CheckConsistency(gs, "sba")

	if outcomeKinds(gs)["lost_seat_active"] != 0 {
		t.Fatalf("Lost && !LeftGame seat with an un-purged stack item is a pre-cleanup transient, must not flag lost_seat_active: %v",
			gs.SeatOutcome.Violations)
	}
}

func TestSeatOutcome_LeftGameStackItemFlagged(t *testing.T) {
	// The genuine leak: §800.4a cleanup ran (LeftGame=true) yet a stack
	// item controlled by the departed seat survived — must still flag.
	gs := outcomeFixture(t)
	gs.Seats[1].Lost = true
	gs.Seats[1].LeftGame = true
	gs.Seats[1].LossReason = "life total 0 or less (CR 704.5a)"
	gs.Stack = append(gs.Stack, &StackItem{
		ID: 1, Controller: 1,
		Card: &Card{Name: "Leaked Spell", Owner: 1},
	})

	gs.SeatOutcome.CheckConsistency(gs, "sba")

	if outcomeKinds(gs)["lost_seat_active"] == 0 {
		t.Fatalf("departed (LeftGame) seat still controlling a stack item is a real §800.4a leak, must flag lost_seat_active")
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
	StateBasedActions(gs)            // sba hook must no-op
	HandleSeatElimination(gs, 1)     // elimination hooks must no-op
	var c *SeatOutcomeChecker
	c.CheckConsistency(gs, "sba")    // explicit nil receiver
	c.BeginElimination(gs, 0)
	c.VerifyEliminationCleanup(gs, 0)
}
