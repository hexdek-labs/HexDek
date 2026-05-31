package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Budget-system r60 pressure tune covers three additions:
//
//   - gameStatePressure: 3-component (turn + life + board) 0..1 score
//   - adaptiveComplexityThreshold: turn-scaled degrade gate
//   - effectiveBudget non-high-stakes pressure lift (up to +50%)
//   - isHighStakesDecision pressure bridge at >= 0.6
//
// Existing budget tests (budget_high_stakes_r60_test.go) pin exact
// Budget values on the high-stakes path; the pressure lift is gated
// to the NON-high-stakes path so those tests stay green. New tests
// here pin the new behavior independently.

// ---------------------------------------------------------------------
// gameStatePressure component tests
// ---------------------------------------------------------------------

func TestGameStatePressure_DefaultGameIsZero(t *testing.T) {
	gs := newTierTestGame(t, 4)
	got := gameStatePressure(gs)
	if got != 0 {
		t.Errorf("default game (turn 0, full life, no board): pressure should be 0, got %.4f", got)
	}
}

func TestGameStatePressure_NilSafe(t *testing.T) {
	if got := gameStatePressure(nil); got != 0 {
		t.Errorf("nil game: pressure should be 0, got %.4f", got)
	}
}

func TestGameStatePressure_TurnRampInRange(t *testing.T) {
	// Turn ramp: 0 below turn 6, +0.05 per turn from 6 to 12, then
	// saturated at 0.35.
	cases := []struct {
		turn      int
		wantTurn  float64
	}{
		{0, 0.0},
		{5, 0.0},
		{6, 0.0}, // (6-6)*0.05 = 0
		{8, 0.10},
		{10, 0.20},
		{12, 0.35}, // saturated
		{20, 0.35}, // capped
	}
	for _, tc := range cases {
		gs := newTierTestGame(t, 4)
		gs.Turn = tc.turn
		// Default life and no board → only turn pressure contributes.
		got := gameStatePressure(gs)
		// Allow small float drift.
		if got < tc.wantTurn-1e-9 || got > tc.wantTurn+1e-9 {
			t.Errorf("turn=%d: pressure=%.4f want %.4f", tc.turn, got, tc.wantTurn)
		}
	}
}

func TestGameStatePressure_LifeComponentScales(t *testing.T) {
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	gs.Seats[1].Life = 20 // ratio 0.5 → (1-0.5)*0.35 = 0.175
	got := gameStatePressure(gs)
	if got < 0.17 || got > 0.18 {
		t.Errorf("opp at 20 life: pressure=%.4f want ~0.175", got)
	}
}

func TestGameStatePressure_LostSeatExcluded(t *testing.T) {
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	gs.Seats[1].Lost = true
	gs.Seats[1].Life = 0 // would be max pressure if counted
	got := gameStatePressure(gs)
	if got != 0 {
		t.Errorf("lost seat at 0 life should NOT contribute pressure; got %.4f", got)
	}
}

// ---------------------------------------------------------------------
// adaptiveComplexityThreshold tests
// ---------------------------------------------------------------------

func TestAdaptiveComplexityThreshold_BaselineUnchangedBelowTurn10(t *testing.T) {
	gs := newTierTestGame(t, 4)
	for _, turn := range []int{0, 5, 10} {
		gs.Turn = turn
		got := adaptiveComplexityThreshold(gs)
		if got != adaptiveBudgetComplexityThreshold {
			t.Errorf("turn=%d: threshold=%d want %d (baseline)", turn, got, adaptiveBudgetComplexityThreshold)
		}
	}
}

func TestAdaptiveComplexityThreshold_ScalesPastTurn10(t *testing.T) {
	gs := newTierTestGame(t, 4)
	gs.Turn = 15
	got := adaptiveComplexityThreshold(gs)
	want := adaptiveBudgetComplexityThreshold + (15-10)*3 // 60 + 15 = 75
	if got != want {
		t.Errorf("turn=15: threshold=%d want %d", got, want)
	}
}

func TestAdaptiveComplexityThreshold_CappedAt120(t *testing.T) {
	gs := newTierTestGame(t, 4)
	gs.Turn = 50
	got := adaptiveComplexityThreshold(gs)
	if got != 120 {
		t.Errorf("turn=50: threshold=%d want 120 (cap)", got)
	}
}

func TestAdaptiveComplexityThreshold_NilGameUsesBaseline(t *testing.T) {
	got := adaptiveComplexityThreshold(nil)
	if got != adaptiveBudgetComplexityThreshold {
		t.Errorf("nil gs: threshold=%d want %d", got, adaptiveBudgetComplexityThreshold)
	}
}

// ---------------------------------------------------------------------
// effectiveBudget pressure lift
// ---------------------------------------------------------------------

func TestEffectiveBudget_PressureLiftsBudgetOnNonHighStakes(t *testing.T) {
	// Build a state with moderate pressure (turn 8 + opp at 20 life) and
	// NO complexity / high-stakes triggers. Pressure components:
	//   turn (8-6)*0.05 = 0.10
	//   life (1 - 20/40)*0.35 = 0.175
	//   board 0
	//   total = 0.275
	// Lift: budget * (1 + 0.275*0.5) = budget * 1.1375 → 100 → 113.
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	gs.Turn = 8
	gs.Seats[1].Life = 20

	h := newTierHat(100)
	got := h.effectiveBudget(gs)
	if got <= 100 {
		t.Errorf("moderate pressure should lift budget above 100; got %d", got)
	}
	if got < 110 || got > 120 {
		t.Errorf("turn=8/life=20 pressure lift off: got %d (want ~113)", got)
	}
}

func TestEffectiveBudget_PressureLiftNotAppliedOnHighStakesPath(t *testing.T) {
	// High-stakes (low-life opp) path unchanged — even though pressure
	// would contribute, the lift is gated to the non-high-stakes branch
	// so existing tests stay pinned at exact Budget values.
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	gs.Seats[1].Life = 4 // triggers binary highStakes via the life threshold

	h := newTierHat(100)
	got := h.effectiveBudget(gs)
	if got != 100 {
		t.Errorf("high-stakes path must return unmodified Budget (preserves test contract); got %d", got)
	}
}

func TestEffectiveBudget_NoPressureNoLift(t *testing.T) {
	// Default game = 0 pressure = budget unchanged.
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	h := newTierHat(100)
	got := h.effectiveBudget(gs)
	if got != 100 {
		t.Errorf("zero pressure: budget=%d want 100", got)
	}
}

// ---------------------------------------------------------------------
// isHighStakesDecision pressure bridge
// ---------------------------------------------------------------------

func TestIsHighStakesDecision_PressureBridgeFires(t *testing.T) {
	// Turn 12 saturated turn pressure (0.35) + opp at 14 life
	// (1 - 14/40)*0.35 = 0.2275 + 12 board pow ~0.17 = 0.7475 → >= 0.6.
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	gs.Turn = 12
	gs.Seats[1].Life = 14
	c := &gameengine.Card{Name: "Bear", Types: []string{"creature"}, BasePower: 12, BaseToughness: 12}
	gs.Seats[2].Battlefield = append(gs.Seats[2].Battlefield, &gameengine.Permanent{
		Card: c, Controller: 2, Owner: 2,
	})

	h := newTierHat(100)
	if !h.isHighStakesDecision(gs) {
		t.Errorf("pressure bridge should fire (turn 12 + opp at 14 life + 12 board pow)")
	}
}

func TestIsHighStakesDecision_PressureBelowBridgeFloorDoesNotFire(t *testing.T) {
	// Turn 8 + opp at 25 life + no board. Pressure:
	//   turn 0.10 + life (1-25/40)*0.35 = 0.131 = 0.231 < 0.6.
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	gs.Turn = 8
	gs.Seats[1].Life = 25

	h := newTierHat(100)
	if h.isHighStakesDecision(gs) {
		t.Errorf("pressure 0.23 (below 0.6 bridge) should NOT fire high-stakes")
	}
}

// ---------------------------------------------------------------------
// adaptiveComplexityThreshold integration via effectiveBudget
// ---------------------------------------------------------------------

func TestEffectiveBudget_TurnScaledThresholdPreservesLateGameBudget(t *testing.T) {
	// At turn 15 with 70 permanents (above flat 60 but below scaled 75)
	// and NO high-stakes, the lift normally would degrade — but turn-
	// scaled threshold allows it. Verify budget != 0.
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	gs.Turn = 15
	perSeat := 70/len(gs.Seats) + 1 // ~18 each → ~72 total
	for _, s := range gs.Seats {
		for i := 0; i < perSeat; i++ {
			s.Battlefield = append(s.Battlefield, &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "filler"},
				Owner:      s.Idx,
				Controller: s.Idx,
			})
		}
	}

	h := newTierHat(100)
	got := h.effectiveBudget(gs)
	if got == 0 {
		t.Errorf("turn=15 with ~72 permanents should be UNDER scaled threshold 75; got 0 (degraded)")
	}
}

func TestEffectiveBudget_FlatThresholdSemanticsPreservedAtEarlyTurns(t *testing.T) {
	// At turn 5 with 70 permanents (above the unchanged threshold 60),
	// no high-stakes → still degrades to 0. The early-turn threshold
	// behavior is unchanged.
	gs := newTierTestGame(t, 4)
	for _, s := range gs.Seats {
		s.StartingLife = 40
		s.Life = 40
	}
	gs.Turn = 5
	perSeat := 70/len(gs.Seats) + 1
	for _, s := range gs.Seats {
		for i := 0; i < perSeat; i++ {
			s.Battlefield = append(s.Battlefield, &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "filler"},
				Owner:      s.Idx,
				Controller: s.Idx,
			})
		}
	}

	h := newTierHat(100)
	got := h.effectiveBudget(gs)
	if got != 0 {
		t.Errorf("turn=5 with 72 permanents should degrade (above baseline 60); got %d", got)
	}
}
