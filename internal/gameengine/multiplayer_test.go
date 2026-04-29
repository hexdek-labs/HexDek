package gameengine

import (
	"math/rand"
	"testing"
)

// -----------------------------------------------------------------------------
// Phase 9 — multiplayer tests (CR §800, §101.4, §104.2a).
//
// Extends the 2-player fixture helpers in resolve_test.go to N-seat
// setups. Covers:
//   - APNAP ordering with 4 seats
//   - Opp / OpponentsOf / LivingOpponents semantics
//   - Threat-score attack targeting across multiple opps
//   - §800.4a seat-elimination cleanup
//   - §104.2a last-seat-standing CheckEnd
// -----------------------------------------------------------------------------

// newMultiplayerGame spins up an N-seat game with empty libraries.
func newMultiplayerGame(t *testing.T, n int) *GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	return NewGameState(n, rng, nil)
}

// -----------------------------------------------------------------------------
// APNAP ordering
// -----------------------------------------------------------------------------

func TestAPNAPOrder_FourSeatsFromActive(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Active = 0
	got := gs.APNAPOrder(-1) // -1 triggers anchor = gs.Active
	want := []int{0, 1, 2, 3}
	if !equalIntSlice(got, want) {
		t.Fatalf("APNAPOrder(-1) from seat 0: want %v got %v", want, got)
	}
}

func TestAPNAPOrder_FourSeatsFromNonActive(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Active = 1
	got := gs.APNAPOrder(-1)
	want := []int{1, 2, 3, 0}
	if !equalIntSlice(got, want) {
		t.Fatalf("APNAP from seat 1: want %v got %v", want, got)
	}
}

func TestAPNAPOrder_ExplicitAnchor(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Active = 0
	got := gs.APNAPOrder(2)
	want := []int{2, 3, 0, 1}
	if !equalIntSlice(got, want) {
		t.Fatalf("APNAP from seat 2: want %v got %v", want, got)
	}
}

// -----------------------------------------------------------------------------
// Opp / OpponentsOf / LivingOpponents
// -----------------------------------------------------------------------------

func TestOpponentsOf_FourSeatsAllLiving(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	got := gs.OpponentsOf(0)
	want := []int{1, 2, 3}
	if !equalIntSlice(got, want) {
		t.Fatalf("OpponentsOf(0): want %v got %v", want, got)
	}
}

func TestLivingOpponents_SkipsDead(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Seats[2].Lost = true
	got := gs.LivingOpponents(0)
	want := []int{1, 3}
	if !equalIntSlice(got, want) {
		t.Fatalf("LivingOpponents(0) skipping seat 2: want %v got %v", want, got)
	}
}

func TestOpp_LegacyFirstLivingInAPNAP(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Seats[1].Lost = true
	got := gs.Opp(0) // first living opp in APNAP from seat 0 = seat 2
	if got != 2 {
		t.Fatalf("Opp(0) with seat 1 dead: want 2 got %d", got)
	}
}

// -----------------------------------------------------------------------------
// Attack target selection across opps
// -----------------------------------------------------------------------------

func TestDeclareAttackers_FourSeatsPickLowestLifeOpp(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	// Seat 2 is lowest life — attacker should pick it.
	gs.Seats[1].Life = 20
	gs.Seats[2].Life = 5
	gs.Seats[3].Life = 20
	atk := addBattlefield(gs, 0, "Goblin", 2, 1, "creature")
	atk.SummoningSick = false
	gs.Active = 0
	attackers := DeclareAttackers(gs, 0)
	if len(attackers) != 1 {
		t.Fatalf("expected 1 attacker, got %d", len(attackers))
	}
	def, ok := AttackerDefender(attackers[0])
	if !ok {
		t.Fatal("attacker has no defender_seat flag")
	}
	if def != 2 {
		t.Fatalf("attacker should target lowest-life seat 2, got seat %d", def)
	}
}

func TestDeclareAttackers_SkipsDeadOpp(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Seats[2].Life = 1
	gs.Seats[2].Lost = true // dead — should not be picked
	gs.Seats[1].Life = 5
	atk := addBattlefield(gs, 0, "Goblin", 2, 1, "creature")
	atk.SummoningSick = false
	gs.Active = 0
	attackers := DeclareAttackers(gs, 0)
	def, _ := AttackerDefender(attackers[0])
	if def == 2 {
		t.Fatal("attacker should skip dead seat 2")
	}
	if def != 1 {
		t.Fatalf("expected seat 1 (next-lowest living), got %d", def)
	}
}

// -----------------------------------------------------------------------------
// CombatPhase end-to-end in 4-seat game
// -----------------------------------------------------------------------------

func TestCombatPhase_FourSeatsDamagesTargetedOpp(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Active = 0
	// Attacker on seat 0; seat 2 is lowest life.
	atk := addBattlefield(gs, 0, "Goblin", 3, 1, "creature")
	atk.SummoningSick = false
	gs.Seats[1].Life = 20
	gs.Seats[2].Life = 5
	gs.Seats[3].Life = 20
	CombatPhase(gs)
	if gs.Seats[2].Life != 2 {
		t.Fatalf("seat 2 should have taken 3 damage (20-5=damage to 5, now 2), got %d", gs.Seats[2].Life)
	}
	if gs.Seats[1].Life != 20 || gs.Seats[3].Life != 20 {
		t.Fatal("seats 1 and 3 should not have taken damage")
	}
}

// -----------------------------------------------------------------------------
// Turn-order cycling (0→1→2→3→0)
// -----------------------------------------------------------------------------

func TestTurnOrderCyclesFourSeats(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	// Simulate cycling Active through APNAP.
	for expected, got := range []int{0, 1, 2, 3, 0, 1} {
		gs.Active = expected % 4
		order := gs.APNAPOrder(-1)
		if order[0] != got%4 {
			t.Fatalf("turn %d: first APNAP seat should be %d, got %d", expected, got%4, order[0])
		}
	}
}

// -----------------------------------------------------------------------------
// Seat elimination (§800.4a)
// -----------------------------------------------------------------------------

func TestHandleSeatElimination_RemovesOwnedPermanents(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	p := addBattlefield(gs, 1, "Llanowar Elves", 1, 1, "creature")
	p.Owner = 1
	if len(gs.Seats[1].Battlefield) != 1 {
		t.Fatal("setup failed")
	}
	gs.Seats[1].Lost = true
	HandleSeatElimination(gs, 1)
	if len(gs.Seats[1].Battlefield) != 0 {
		t.Fatal("seat 1's permanents should have been removed (CR §800.4a)")
	}
	if !gs.Seats[1].LeftGame {
		t.Fatal("LeftGame should be set")
	}
	if countEvents(gs, "seat_eliminated") == 0 {
		t.Fatal("missing seat_eliminated event")
	}
}

func TestHandleSeatElimination_Idempotent(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Seats[1].Lost = true
	HandleSeatElimination(gs, 1)
	HandleSeatElimination(gs, 1) // should no-op
	if countEvents(gs, "seat_eliminated") != 1 {
		t.Fatal("HandleSeatElimination should be idempotent")
	}
}

func TestHandleSeatElimination_PurgesStackItems(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	p := addBattlefield(gs, 1, "src", 0, 0, "creature")
	gs.Stack = []*StackItem{
		{ID: 1, Controller: 0, Source: p},
		{ID: 2, Controller: 1, Source: p},
		{ID: 3, Controller: 2, Source: p},
	}
	gs.Seats[1].Lost = true
	HandleSeatElimination(gs, 1)
	if len(gs.Stack) != 2 {
		t.Fatalf("expected 2 stack items remaining, got %d", len(gs.Stack))
	}
	for _, item := range gs.Stack {
		if item.Controller == 1 {
			t.Fatal("seat 1's stack items should be purged")
		}
	}
}

func TestHandleSeatElimination_PreservesOpponentPermanents(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	addBattlefield(gs, 0, "ally", 1, 1, "creature")
	addBattlefield(gs, 2, "unrelated", 1, 1, "creature")
	gs.Seats[1].Lost = true
	HandleSeatElimination(gs, 1)
	if len(gs.Seats[0].Battlefield) != 1 {
		t.Fatal("seat 0's permanents should be intact")
	}
	if len(gs.Seats[2].Battlefield) != 1 {
		t.Fatal("seat 2's permanents should be intact")
	}
}

// -----------------------------------------------------------------------------
// Ownership-based cleanup (Gilded Drake scenario)
// -----------------------------------------------------------------------------

func TestHandleSeatElimination_RemovesStolenOwnedPermanent(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	// Seat 1 owns the card; seat 0 controls it (Gilded Drake trade).
	p := addBattlefield(gs, 0, "Stolen Angel", 4, 4, "creature")
	p.Owner = 1
	// Seat 1 leaves the game. Their owned permanent should leave.
	gs.Seats[1].Lost = true
	HandleSeatElimination(gs, 1)
	for _, bf := range gs.Seats[0].Battlefield {
		if bf == p {
			t.Fatal("seat 1's owned permanent should have left even under seat 0's control (CR §800.4a + §108.3)")
		}
	}
}

// -----------------------------------------------------------------------------
// CheckEnd — §104.2a last seat standing
// -----------------------------------------------------------------------------

func TestCheckEnd_TwoLivingSeatsContinues(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Seats[0].Lost = true
	gs.Seats[1].Lost = true
	if gs.CheckEnd() {
		t.Fatal("game should continue with 2 living seats")
	}
}

func TestCheckEnd_ThreeSeatsDeadLastWins(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	gs.Seats[0].Lost = true
	gs.Seats[1].Lost = true
	gs.Seats[2].Lost = true
	if !gs.CheckEnd() {
		t.Fatal("game should end with 1 living seat")
	}
	if gs.Flags["winner"] != 3 {
		t.Fatalf("winner should be seat 3, got %d", gs.Flags["winner"])
	}
	if !gs.Seats[3].Won {
		t.Fatal("seat 3 should have Won flag set")
	}
}

func TestCheckEnd_AllDeadIsDraw(t *testing.T) {
	gs := newMultiplayerGame(t, 4)
	for i := 0; i < 4; i++ {
		gs.Seats[i].Lost = true
	}
	if !gs.CheckEnd() {
		t.Fatal("game should end with 0 living seats")
	}
	// winner flag should be unset (or zero)
	if _, ok := gs.Flags["winner"]; ok {
		t.Fatal("draw should not set a winner")
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
