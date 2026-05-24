package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// budget_high_stakes_r60_test.go — R60 adaptive-budget improvement.
// Pre-R60 the complexity degrade in effectiveBudget was unconditional:
// once total permanents hit 60, every decision returned Budget=0
// regardless of game state. classifyDecision had a comboAssembling
// escape hatch, but effectiveBudget didn't honor it — so the tier
// report drifted from the actual compute path. Now both gates share
// isHighStakesDecision, which also covers low-life seats and deep
// stacks (counter wars / triggered chains where decisions matter
// disproportionately).

// fillBoard stuffs each seat's battlefield until total permanents
// crosses the complexity threshold by `over`.
func fillBoard(t *testing.T, gs *gameengine.GameState, over int) {
	t.Helper()
	target := adaptiveBudgetComplexityThreshold + over
	perSeat := target/len(gs.Seats) + 1
	for _, s := range gs.Seats {
		for i := 0; i < perSeat; i++ {
			s.Battlefield = append(s.Battlefield, &gameengine.Permanent{
				Card:       &gameengine.Card{Name: "filler"},
				Owner:      s.Idx,
				Controller: s.Idx,
			})
		}
	}
}

// -----------------------------------------------------------------------------
// effectiveBudget: high-stakes overrides
// -----------------------------------------------------------------------------

func TestEffectiveBudget_ComplexityDegradesByDefault(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	fillBoard(t, gs, 5)

	if got := h.effectiveBudget(gs); got != 0 {
		t.Fatalf("complex board with no high-stakes signal: want 0, got %d", got)
	}
}

func TestEffectiveBudget_LowLifeOpponentOverridesDegrade(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	fillBoard(t, gs, 5)
	gs.Seats[1].Life = 4 // any opp at ≤ 8 → high stakes

	if got := h.effectiveBudget(gs); got != 100 {
		t.Fatalf("low-life opponent should preserve budget; want 100, got %d", got)
	}
}

func TestEffectiveBudget_LowLifeSelfOverridesDegrade(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	fillBoard(t, gs, 5)
	gs.Seats[0].Life = 5

	if got := h.effectiveBudget(gs); got != 100 {
		t.Fatalf("our seat at low life should preserve budget; want 100, got %d", got)
	}
}

func TestEffectiveBudget_DeepStackOverridesDegrade(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	fillBoard(t, gs, 5)
	// Push 3 stack items (the threshold).
	for i := 0; i < highStakesStackDepth; i++ {
		gs.Stack = append(gs.Stack, &gameengine.StackItem{
			Card: &gameengine.Card{Name: "spell"},
		})
	}

	if got := h.effectiveBudget(gs); got != 100 {
		t.Fatalf("deep stack should preserve budget; want 100, got %d", got)
	}
}

func TestEffectiveBudget_LostSeatLowLifeIgnored(t *testing.T) {
	// A LOST seat with 0 life shouldn't trigger the override — they're
	// already out, the game is no more urgent for them.
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	fillBoard(t, gs, 5)
	gs.Seats[1].Lost = true
	gs.Seats[1].Life = 0
	gs.Seats[2].Life = 20
	gs.Seats[3].Life = 20
	gs.Seats[0].Life = 20

	if got := h.effectiveBudget(gs); got != 0 {
		t.Fatalf("lost seat at 0 life must not trigger high-stakes; want 0, got %d", got)
	}
}

func TestEffectiveBudget_TurnBudgetExhaustedOverriddenByHighStakes(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	h.TurnBudget = 5
	h.spendTurnBudget(gs, 5)
	gs.Seats[1].Life = 4

	if got := h.effectiveBudget(gs); got != 100 {
		t.Fatalf("exhausted turn budget + high-stakes: want 100, got %d", got)
	}
}

func TestEffectiveBudget_TurnBudgetExhaustedNoHighStakesReturns0(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	h.TurnBudget = 5
	h.spendTurnBudget(gs, 5)

	if got := h.effectiveBudget(gs); got != 0 {
		t.Fatalf("exhausted turn budget no high-stakes: want 0, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// classifyDecision: same gate keeps tier report aligned
// -----------------------------------------------------------------------------

func TestClassifyDecision_LowLifeOpponentPreservesTier(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	fillBoard(t, gs, 5)
	gs.Seats[1].Life = 4

	// Pre-R60: complexity-degrade forced TierMjolnir even though combo
	// override (then absent) would have kept Gungnir. Now low-life =
	// high stakes = Gungnir preserved.
	if got := h.classifyDecision(gs); got != TierGungnir {
		t.Fatalf("low-life opp: want Gungnir, got %s", got)
	}
}

func TestClassifyDecision_DeepStackPreservesTier(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	fillBoard(t, gs, 5)
	for i := 0; i < highStakesStackDepth; i++ {
		gs.Stack = append(gs.Stack, &gameengine.StackItem{
			Card: &gameengine.Card{Name: "spell"},
		})
	}

	if got := h.classifyDecision(gs); got != TierGungnir {
		t.Fatalf("deep stack: want Gungnir, got %s", got)
	}
}

// -----------------------------------------------------------------------------
// isHighStakesDecision unit cases
// -----------------------------------------------------------------------------

func TestIsHighStakesDecision_DefaultGameIsLowStakes(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	if h.isHighStakesDecision(gs) {
		t.Fatal("fresh game: must NOT be high stakes")
	}
}

func TestIsHighStakesDecision_NilSafe(t *testing.T) {
	var h *YggdrasilHat
	if h.isHighStakesDecision(nil) {
		t.Fatal("nil hat: must NOT be high stakes")
	}
	h = newTierHat(100)
	if h.isHighStakesDecision(nil) {
		t.Fatal("nil gs: must NOT be high stakes")
	}
}

func TestIsHighStakesDecision_LifeAtThresholdTriggers(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	gs.Seats[2].Life = highStakesLifeThreshold
	if !h.isHighStakesDecision(gs) {
		t.Fatal("seat at exactly the threshold should trigger high stakes")
	}
}

func TestIsHighStakesDecision_LifeJustAboveThresholdDoesNotTrigger(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	gs.Seats[2].Life = highStakesLifeThreshold + 1
	if h.isHighStakesDecision(gs) {
		t.Fatal("seat one above threshold should NOT trigger")
	}
}

func TestIsHighStakesDecision_StackAtThresholdTriggers(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	for i := 0; i < highStakesStackDepth; i++ {
		gs.Stack = append(gs.Stack, &gameengine.StackItem{
			Card: &gameengine.Card{Name: "spell"},
		})
	}
	if !h.isHighStakesDecision(gs) {
		t.Fatal("stack at threshold depth should trigger")
	}
}

func TestIsHighStakesDecision_StackBelowThresholdDoesNotTrigger(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	for i := 0; i < highStakesStackDepth-1; i++ {
		gs.Stack = append(gs.Stack, &gameengine.StackItem{
			Card: &gameengine.Card{Name: "spell"},
		})
	}
	if h.isHighStakesDecision(gs) {
		t.Fatal("stack one below threshold should NOT trigger")
	}
}
