package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r60-cedh-planstate: pins mid-turn PlanState refresh + effectiveBudget
// lift on Assembling / Executing plans.
//
// Pre-tuning baseline: PlanState.Evaluate only fired at upkeep, so
// a tutor that resolved mid-turn AND flipped the Assembling gate
// didn't change planState until the NEXT upkeep. The cardHeuristic
// combo-priority bias is gated on planState.Current ∈
// {PlanAssemble, PlanExecute}, so it never fired on the critical
// turn the reach pattern arrived. Two surfaces fix this:
//
//   (1) refreshPlanState is called from ChooseCastFromHand so the
//       first cast decision in main phase sees the current plan.
//   (2) effectiveBudget lifts +50% (Assemble) / +100% (Execute) so
//       MCTS depth scales with how close the wincon is.

// planStateRefreshTutor is a small tutor card with rich-enough oracle
// text that seatTutorsInHand picks it up.
func planStateRefreshTutor(name string, cmc int) *gameengine.Card {
	return cardWithStaticText(name, []string{"sorcery"}, cmc,
		"search your library for a card, put it into your hand, then shuffle")
}

// TestRefreshPlanState_FlipsToAssembleMidTurn pins the headline:
// calling refreshPlanState directly (the same call ChooseCastFromHand
// now makes at its entry) flips planState from PlanDevelop to
// PlanAssemble when a multi-tutor reach hand has been assembled mid-
// turn. Without this hook, planState would stay PlanDevelop until
// the next upkeep and the cast-order bias would never fire.
func TestRefreshPlanState_FlipsToAssembleMidTurn(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		planStateRefreshTutor("Demonic Tutor", 2),
		planStateRefreshTutor("Vampiric Tutor", 1),
	)

	sp := &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"PieceA", "PieceB", "PieceC"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 50, 0)
	if h.planState.Current != PlanDevelop {
		t.Fatalf("test setup: planState should start PlanDevelop; got %v", h.planState.Current)
	}

	h.refreshPlanState(gs, 0)
	if h.planState.Current != PlanAssemble {
		t.Fatalf("after refresh with multi-tutor reach hand, planState should be PlanAssemble; got %v", h.planState.Current)
	}
}

// TestRefreshPlanState_NoFlipWhenInsufficientTutors pins the gate
// counterfactual: when tutors don't cover all missing pieces,
// refreshPlanState leaves planState in PlanDevelop. The same gate as
// the broadened ComboAssessment.Assembling.
func TestRefreshPlanState_NoFlipWhenInsufficientTutors(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		planStateRefreshTutor("Demonic Tutor", 2),
	)

	sp := &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"PieceA", "PieceB", "PieceC"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, 50, 0)

	h.refreshPlanState(gs, 0)
	if h.planState.Current != PlanDevelop {
		t.Errorf("planState should stay PlanDevelop (1 tutor can't cover 2 missing slots); got %v", h.planState.Current)
	}
}

// TestRefreshPlanState_NilComboSeqSafeNoop pins safety: a hat built
// from a profile with no ComboPieces has comboSeq=nil; refreshPlanState
// must no-op cleanly (no panic, no plan change).
func TestRefreshPlanState_NilComboSeqSafeNoop(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHatWithNoise(&StrategyProfile{}, 50, 0)
	if h.comboSeq != nil {
		t.Fatalf("test setup: empty ComboPieces should yield nil comboSeq")
	}
	// Should not panic, should not change plan.
	h.refreshPlanState(gs, 0)
	if h.planState.Current != PlanDevelop {
		t.Errorf("planState changed despite nil comboSeq; got %v", h.planState.Current)
	}
}

// TestEffectiveBudget_LiftInPlanAssemble pins the budget lift on
// PlanAssemble: budget = base * 3/2.
func TestEffectiveBudget_LiftInPlanAssemble(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	h := NewYggdrasilHatWithNoise(&StrategyProfile{}, 50, 0)
	h.planState.Current = PlanAssemble

	got := h.effectiveBudget(gs)
	want := 50 * 3 / 2 // = 75
	if got != want {
		t.Errorf("PlanAssemble effectiveBudget = %d, want %d (base 50 +50%%)", got, want)
	}
}

// TestEffectiveBudget_LiftInPlanExecute pins the budget lift on
// PlanExecute: budget = base * 2.
func TestEffectiveBudget_LiftInPlanExecute(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	h := NewYggdrasilHatWithNoise(&StrategyProfile{}, 50, 0)
	h.planState.Current = PlanExecute

	got := h.effectiveBudget(gs)
	want := 50 * 2
	if got != want {
		t.Errorf("PlanExecute effectiveBudget = %d, want %d (base 50 +100%%)", got, want)
	}
}

// TestEffectiveBudget_NoLiftInPlanDevelop pins the baseline: PlanDevelop
// returns the base budget unchanged. Defends against an accidental
// always-on multiplier.
func TestEffectiveBudget_NoLiftInPlanDevelop(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	h := NewYggdrasilHatWithNoise(&StrategyProfile{}, 50, 0)
	if h.planState.Current != PlanDevelop {
		t.Fatalf("test setup: planState default should be PlanDevelop")
	}

	got := h.effectiveBudget(gs)
	want := 50
	if got != want {
		t.Errorf("PlanDevelop effectiveBudget = %d, want %d (base unchanged)", got, want)
	}
}

// TestEffectiveBudget_NoLiftWhenZeroBudget pins the heuristic-only
// path: when h.Budget == 0 (Mjolnir tier), the lift must NOT kick in
// because the early-return at the top of effectiveBudget short-
// circuits. A 0 -> 0 budget should stay 0 regardless of plan.
func TestEffectiveBudget_NoLiftWhenZeroBudget(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	h := NewYggdrasilHatWithNoise(&StrategyProfile{}, 0, 0)
	h.planState.Current = PlanExecute

	got := h.effectiveBudget(gs)
	if got != 0 {
		t.Errorf("zero-base effectiveBudget = %d, want 0 (lift must not bypass heuristic-only)", got)
	}
}

// TestEffectiveBudget_LiftRespectsHighStakesBypass pins that the lift
// stacks correctly on top of the high-stakes complexity bypass: a
// 60+ permanent board in PlanAssemble should get the LIFTED budget,
// not 0 (degraded) — comboAssembling is one of the high-stakes
// signals, so the complexity floor is bypassed AND the lift applies.
func TestEffectiveBudget_LiftRespectsHighStakesBypass(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	seat := gs.Seats[0]
	// Stage a combo line so comboAssembling returns true → high stakes.
	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 2, nil),
		planStateRefreshTutor("Demonic Tutor", 2),
	)
	for i := 0; i < adaptiveBudgetComplexityThreshold+5; i++ {
		filler := newTestCardMinimal("Filler", []string{"creature"}, 1, nil)
		newTestPermanent(seat, filler, 0, 0)
	}
	sp := &StrategyProfile{
		ComboPieces: []ComboPlan{{Pieces: []string{"PieceA", "PieceB"}, Type: "infinite"}},
	}
	h := NewYggdrasilHatWithNoise(sp, 50, 0)
	gs.Active = 0
	h.planState.Current = PlanAssemble

	got := h.effectiveBudget(gs)
	want := 75 // +50% lift
	if got != want {
		t.Errorf("high-stakes Assemble effectiveBudget = %d, want %d (combo bypasses complexity floor + lift applies)", got, want)
	}
}
