package hat

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func newTierTestGame(t *testing.T, seats int) *gameengine.GameState {
	t.Helper()
	return gameengine.NewGameState(seats, rand.New(rand.NewSource(1)), nil)
}

func newTierHat(budget int) *YggdrasilHat {
	return NewYggdrasilHatWithNoise(nil, budget, 0)
}

func TestClassifyDecisionBudgetZeroIsMjolnir(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(0)
	if got := h.classifyDecision(gs); got != TierMjolnir {
		t.Fatalf("Budget=0: want Mjolnir, got %s", got)
	}
}

func TestClassifyDecisionEvaluatorBudgetIsGungnir(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(100)
	if got := h.classifyDecision(gs); got != TierGungnir {
		t.Fatalf("Budget=100 (no rollout): want Gungnir, got %s", got)
	}
}

func TestClassifyDecisionRolloutBudgetWithRunnerIsRagnarok(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(rolloutBudgetGe)
	h.TurnRunner = func(*gameengine.GameState) {}
	if got := h.classifyDecision(gs); got != TierRagnarok {
		t.Fatalf("Budget=%d w/ runner: want Ragnarok, got %s", rolloutBudgetGe, got)
	}
}

func TestClassifyDecisionRolloutBudgetWithoutRunnerDegradesToGungnir(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(rolloutBudgetGe)
	// No TurnRunner — rollouts impossible.
	if got := h.classifyDecision(gs); got != TierGungnir {
		t.Fatalf("Budget=%d w/o runner: want Gungnir, got %s", rolloutBudgetGe, got)
	}
}

func TestClassifyDecisionBoardComplexityDegradesToGungnir(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(rolloutBudgetGe)
	h.TurnRunner = func(*gameengine.GameState) {}

	// Stuff the battlefields above the complexity threshold.
	per := adaptiveBudgetComplexityThreshold/len(gs.Seats) + 2
	for _, s := range gs.Seats {
		for i := 0; i < per; i++ {
			s.Battlefield = append(s.Battlefield, &gameengine.Permanent{
				Card:  &gameengine.Card{Name: "filler"},
				Owner: s.Idx, Controller: s.Idx,
			})
		}
	}

	// r61 PR-8: a complex non-high-stakes board no longer cliffs to the
	// pure-heuristic Mjolnir tier. effectiveBudget tapers to a degraded
	// evaluator budget, so the tier report degrades to Gungnir (evaluator-
	// guided) — NOT all the way to Mjolnir, and NOT up to rollout
	// (Ragnarok), since the taper drops the budget below rolloutBudgetGe.
	if got := h.classifyDecision(gs); got != TierGungnir {
		t.Fatalf("complex board: want Gungnir (graceful degrade), got %s", got)
	}
}

func TestClassifyDecisionTurnBudgetExhaustedForcesMjolnir(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(rolloutBudgetGe)
	h.TurnRunner = func(*gameengine.GameState) {}
	h.TurnBudget = 5
	// Spend it all.
	h.spendTurnBudget(gs, 5)

	if got := h.classifyDecision(gs); got != TierMjolnir {
		t.Fatalf("turn budget exhausted: want Mjolnir, got %s", got)
	}
}

func TestClassifyDecisionTurnBudgetLowDegradesRagnarokToGungnir(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(rolloutBudgetGe)
	h.TurnRunner = func(*gameengine.GameState) {}
	h.TurnBudget = rolloutEvalCost + 1
	h.spendTurnBudget(gs, 2) // remaining = rolloutEvalCost - 1, below the threshold.

	if got := h.classifyDecision(gs); got != TierGungnir {
		t.Fatalf("turn remaining < rolloutEvalCost: want Gungnir, got %s", got)
	}
}

// makeComboAssemblingHat seats the player with one combo piece + a tutor
// so ComboSequencer.Evaluate reports Assembling=true.
func makeComboAssemblingHat(t *testing.T, gs *gameengine.GameState, seatIdx int, budget int) *YggdrasilHat {
	t.Helper()
	sp := &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"PieceA", "PieceB"}, Type: "infinite"},
		},
	}
	h := NewYggdrasilHatWithNoise(sp, budget, 0)

	gs.Active = seatIdx
	seat := gs.Seats[seatIdx]

	pieceA := &gameengine.Card{
		Name: "PieceA",
		AST:  &gameast.CardAST{Name: "PieceA"},
	}
	pieceA.Types = []string{"creature"}
	seat.Hand = append(seat.Hand, pieceA)

	tutorAST := &gameast.CardAST{
		Name: "Demonic Tutor",
		Abilities: []gameast.Ability{
			&gameast.Activated{
				Raw:    "Search your library for a card, put it into your hand, then shuffle.",
				Effect: &gameast.Tutor{Destination: "hand", Query: gameast.Filter{Base: "card"}},
			},
		},
	}
	tutor := &gameengine.Card{Name: "Demonic Tutor", AST: tutorAST, Types: []string{"sorcery"}}
	seat.Hand = append(seat.Hand, tutor)

	return h
}

func TestClassifyDecisionComboAssemblingOverridesComplexity(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := makeComboAssemblingHat(t, gs, 0, rolloutBudgetGe)
	h.TurnRunner = func(*gameengine.GameState) {}

	// Saturate the board so the non-combo path would force Mjolnir.
	per := adaptiveBudgetComplexityThreshold/len(gs.Seats) + 2
	for _, s := range gs.Seats {
		for i := 0; i < per; i++ {
			s.Battlefield = append(s.Battlefield, &gameengine.Permanent{
				Card:  &gameengine.Card{Name: "filler"},
				Owner: s.Idx, Controller: s.Idx,
			})
		}
	}

	if !h.comboAssembling(gs) {
		t.Fatalf("test setup: combo should be Assembling")
	}
	if got := h.classifyDecision(gs); got != TierRagnarok {
		t.Fatalf("combo assembling on complex board: want Ragnarok (override), got %s", got)
	}
}

func TestRecordDecisionTierAndStats(t *testing.T) {
	h := newTierHat(50)

	h.recordDecisionTier(TierMjolnir)
	h.recordDecisionTier(TierMjolnir)
	h.recordDecisionTier(TierGungnir)
	h.recordDecisionTier(TierRagnarok)

	stats := h.MjolnirStats()
	if stats.Mjolnir != 2 || stats.Gungnir != 1 || stats.Ragnarok != 1 {
		t.Fatalf("counts wrong: %+v", stats)
	}
	if stats.Total() != 4 {
		t.Fatalf("Total: want 4, got %d", stats.Total())
	}
}

func TestRecordDecisionTierIgnoresOutOfRange(t *testing.T) {
	h := newTierHat(50)
	h.recordDecisionTier(DecisionTier(-1))
	h.recordDecisionTier(DecisionTier(99))
	if got := h.MjolnirStats().Total(); got != 0 {
		t.Fatalf("invalid tiers should not count: total=%d", got)
	}
}

func TestResetMjolnirStats(t *testing.T) {
	h := newTierHat(50)
	h.recordDecisionTier(TierGungnir)
	h.recordDecisionTier(TierGungnir)
	h.ResetMjolnirStats()
	if got := h.MjolnirStats().Total(); got != 0 {
		t.Fatalf("after reset: total=%d, want 0", got)
	}
}

func TestChooseCastFromHandIncrementsCounter(t *testing.T) {
	gs := newTierTestGame(t, 4)
	h := newTierHat(50)
	gs.Seats[0].Hat = h

	before := h.MjolnirStats().Total()
	_ = h.ChooseCastFromHand(gs, 0, nil)
	after := h.MjolnirStats().Total()

	if after != before+1 {
		t.Fatalf("ChooseCastFromHand should record exactly one tier: before=%d after=%d", before, after)
	}
}

func TestDecisionTierStringRoundTrip(t *testing.T) {
	cases := []struct {
		tier DecisionTier
		want string
	}{
		{TierMjolnir, "Mjolnir"},
		{TierGungnir, "Gungnir"},
		{TierRagnarok, "Ragnarok"},
		{DecisionTier(42), "Unknown"},
	}
	for _, c := range cases {
		if got := c.tier.String(); got != c.want {
			t.Errorf("DecisionTier(%d).String() = %q, want %q", c.tier, got, c.want)
		}
	}
}
