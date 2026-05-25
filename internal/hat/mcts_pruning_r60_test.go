package hat

// R60 round 5 — MCTSHat candidate pruning. Branches whose cheap
// heuristic falls below baseScore + PruneThreshold are cut BEFORE the
// expensive rollout / UCB1 evaluation. The cut spares rollout budget
// from obvious losing branches without changing the chosen action in
// the normal case. Pass is never pruned; if every candidate would be
// pruned the highest-heuristic one is restored as a safety floor.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestMCTSHat_PruneThreshold_DefaultIsNegative(t *testing.T) {
	mcts := NewMCTSHat(&GreedyHat{}, nil, 50)
	if mcts.PruneThreshold >= 0 {
		t.Errorf("PruneThreshold default should be negative (pruning enabled); got %.3f", mcts.PruneThreshold)
	}
	if mcts.PruneThreshold != DefaultPruneThreshold {
		t.Errorf("PruneThreshold default should be %.3f; got %.3f", DefaultPruneThreshold, mcts.PruneThreshold)
	}
}

func TestMCTSHat_ShouldPrune_Disabled(t *testing.T) {
	mcts := NewMCTSHat(&GreedyHat{}, nil, 50)
	mcts.PruneThreshold = 0
	if mcts.shouldPrune(-100.0, 0.0) {
		t.Error("PruneThreshold=0 must disable pruning entirely")
	}
}

func TestMCTSHat_ShouldPrune_BelowCut(t *testing.T) {
	mcts := NewMCTSHat(&GreedyHat{}, nil, 50)
	mcts.PruneThreshold = -0.5
	// baseScore=1.0, threshold=-0.5 → cut at 0.5.
	if !mcts.shouldPrune(0.49, 1.0) {
		t.Error("heuristic 0.49 must prune when cut is 0.5")
	}
	if mcts.shouldPrune(0.50, 1.0) {
		t.Error("heuristic exactly at the cut should NOT prune (strict <)")
	}
	if mcts.shouldPrune(1.0, 1.0) {
		t.Error("heuristic equal to baseScore should NOT prune")
	}
}

// Integration: with pruning aggressively enabled, a clearly-losing
// candidate must NOT trigger a rollout (i.e., TurnRunner is invoked
// fewer times than the candidate count + pass).
func TestMCTSHat_PruningSkipsRolloutForLosingBranch(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 200)
	// Aggressive cut: anything below baseScore prunes. evaluateCandidate
	// adds +0.15 tempo + 0.2 mana-rock + 0.2 CMC eff to ramp; for a
	// massively off-curve spell (CMC=15, 2 mana) the CMC-efficiency
	// term goes deeply negative and the net bonus < 0.
	mcts.PruneThreshold = -0.05

	turnRunnerCalls := 0
	mcts.TurnRunner = func(gs *gameengine.GameState) {
		turnRunnerCalls++
	}

	onCurve := newTestCardMinimal("Bears", []string{"creature"}, 2, nil)
	onCurve.CMC = 2
	offCurve := newTestCardMinimal("Eldrazi", []string{"creature"}, 15, nil)
	offCurve.CMC = 15 // huge CMC vs 2 mana sources → -0.67 efficiency bonus

	castable := []*gameengine.Card{onCurve, offCurve}
	gs.Seats[0].Hand = append([]*gameengine.Card{}, castable...)
	gs.Seats[0].ManaPool = 2
	// Give seat 0 minimal mana sources so CMC efficiency for offCurve
	// (|15-2|/3 ≈ -3.3 → -0.67 efficiency bonus) drives its heuristic
	// below baseScore.
	land := newTestCardMinimal("Mountain", []string{"land"}, 0, nil)
	newTestPermanent(gs.Seats[0], land, 0, 0)
	newTestPermanent(gs.Seats[0], land, 0, 0)

	preCalls := turnRunnerCalls
	_ = mcts.ChooseCastFromHand(gs, 0, castable)
	callsForChoice := turnRunnerCalls - preCalls

	if mcts.PrunedCount == 0 {
		t.Fatalf("expected at least 1 pruned candidate; got 0 (PrunedCount=%d). Off-curve heuristic must have passed the cut — adjust the test fixture.", mcts.PrunedCount)
	}
	// 2 candidates + pass = 3 rollout slots; with one pruned we should
	// see at most 2 rollout starts (each runs multiple turn ticks, so
	// "calls" >> rollouts — the assertion is on the pruning bookkeeping
	// + that we still made fewer rollout invocations than the no-prune
	// baseline. Sanity-check the latter:
	mctsNoPrune := NewMCTSHat(inner, nil, 200)
	mctsNoPrune.PruneThreshold = 0 // disable
	noPruneCalls := 0
	mctsNoPrune.TurnRunner = func(gs *gameengine.GameState) { noPruneCalls++ }
	gs.Seats[0].Hand = append([]*gameengine.Card{}, castable...)
	_ = mctsNoPrune.ChooseCastFromHand(gs, 0, castable)

	if callsForChoice >= noPruneCalls {
		t.Errorf("pruning should reduce TurnRunner calls: pruned=%d nopruned=%d", callsForChoice, noPruneCalls)
	}
	if mctsNoPrune.PrunedCount != 0 {
		t.Errorf("PruneThreshold=0 must not increment PrunedCount; got %d", mctsNoPrune.PrunedCount)
	}
}

// The pass action is never pruned regardless of how aggressive the
// threshold is — passing is the always-safe fallback.
func TestMCTSHat_PassIsNeverPruned(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 50) // heuristic path, no rollout
	mcts.PruneThreshold = +10.0        // absurdly aggressive (positive cuts everything)
	mcts.TurnRunner = func(gs *gameengine.GameState) {}

	// Hand of two cards, both with normal-ish heuristics.
	bears := newTestCardMinimal("Bears", []string{"creature"}, 2, nil)
	wolf := newTestCardMinimal("Wolf", []string{"creature"}, 3, nil)
	castable := []*gameengine.Card{bears, wolf}
	gs.Seats[0].Hand = append([]*gameengine.Card{}, castable...)
	gs.Seats[0].ManaPool = 3
	land := newTestCardMinimal("Mountain", []string{"land"}, 0, nil)
	newTestPermanent(gs.Seats[0], land, 0, 0)
	newTestPermanent(gs.Seats[0], land, 0, 0)
	newTestPermanent(gs.Seats[0], land, 0, 0)

	// With -10 threshold, every candidate gets pruned. The safety floor
	// should restore the best-pruned, and the hat must NOT silently
	// return nil with no action chosen via the actionStats pass key.
	choice := mcts.ChooseCastFromHand(gs, 0, castable)
	if choice == nil {
		// Pass is fine — but PrunedCount must show every candidate was
		// considered for pruning (the pass option itself isn't pruned).
		if mcts.PrunedCount < 2 {
			t.Errorf("expected both candidates to enter the prune gate; PrunedCount=%d", mcts.PrunedCount)
		}
		// And the actionStats should have a recorded pass action, NOT a
		// silent return with no recorded action.
		passKey := ""
		for k := range mcts.actionStats {
			if len(k) > 0 && k[len(k)-len("__pass__"):] == "__pass__" {
				passKey = k
				break
			}
		}
		if passKey == "" {
			t.Error("pass action must be recorded in actionStats even when all candidates pruned")
		}
	} else {
		// Safety floor kicked in — that's also valid.
		if mcts.PrunedCount == 0 {
			t.Error("safety floor implies candidates WERE pruned; PrunedCount=0 contradicts")
		}
	}
}

// The safety floor: if pruning rejects every candidate, the highest-
// heuristic pruned candidate must be selectable (not lost forever).
func TestMCTSHat_SafetyFloor_RestoresBestPruned(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	inner := &GreedyHat{}
	mcts := NewMCTSHat(inner, nil, 50) // heuristic path
	mcts.PruneThreshold = +10.0        // positive aggressive — prunes every candidate

	a := newTestCardMinimal("Small", []string{"instant"}, 1, nil)
	b := newTestCardMinimal("Bigger", []string{"creature"}, 2, nil)
	castable := []*gameengine.Card{a, b}
	gs.Seats[0].Hand = append([]*gameengine.Card{}, castable...)
	gs.Seats[0].ManaPool = 2
	land := newTestCardMinimal("Mountain", []string{"land"}, 0, nil)
	newTestPermanent(gs.Seats[0], land, 0, 0)
	newTestPermanent(gs.Seats[0], land, 0, 0)

	// The DecisionLog should mention "safety:" when the floor activates.
	log := []string{}
	mcts.DecisionLog = &log

	_ = mcts.ChooseCastFromHand(gs, 0, castable)

	sawSafety := false
	for _, line := range log {
		if len(line) >= 9 && line[:9] == "  safety:" {
			sawSafety = true
			break
		}
	}
	// Safety floor only triggers when the heuristic path empties the
	// candidate pool. With -10.0 cut and the +0.15 tempo bonus baseline,
	// both candidates should fall under the cut; safety should fire.
	if !sawSafety && mcts.PrunedCount >= 2 {
		t.Errorf("with all %d candidates pruned, safety floor must restore one; log: %v",
			mcts.PrunedCount, log)
	}
}

// Disabled pruning preserves legacy behavior: PrunedCount stays 0 and
// the inner-budget=0 delegation path is untouched.
func TestMCTSHat_PruneThreshold_DisabledKeepsLegacyChoice(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	off := newTestCardMinimal("Eldrazi", []string{"creature"}, 15, nil)
	on := newTestCardMinimal("Bears", []string{"creature"}, 2, nil)

	mcts := NewMCTSHat(&GreedyHat{}, nil, 50)
	mcts.PruneThreshold = 0 // disabled

	castable := []*gameengine.Card{off, on}
	gs.Seats[0].Hand = append([]*gameengine.Card{}, castable...)
	gs.Seats[0].ManaPool = 2

	_ = mcts.ChooseCastFromHand(gs, 0, castable)
	if mcts.PrunedCount != 0 {
		t.Errorf("PruneThreshold=0 must not prune; PrunedCount=%d", mcts.PrunedCount)
	}
}

// Sanity: pruning is independent of the budget=0 delegation path —
// budget=0 routes everything through the inner hat and bypasses the
// MCTS evaluator entirely, so pruning must NOT fire on that path.
func TestMCTSHat_Budget0_BypassesPruningEntirely(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	mcts := NewMCTSHat(&GreedyHat{}, nil, 0) // budget 0
	mcts.PruneThreshold = +10.0              // aggressive — would prune everything if reached

	a := newTestCardMinimal("A", []string{"instant"}, 1, nil)
	b := newTestCardMinimal("B", []string{"creature"}, 2, nil)
	castable := []*gameengine.Card{a, b}
	gs.Seats[0].Hand = append([]*gameengine.Card{}, castable...)
	gs.Seats[0].ManaPool = 2

	_ = mcts.ChooseCastFromHand(gs, 0, castable)
	if mcts.PrunedCount != 0 {
		t.Errorf("budget=0 must bypass pruning; PrunedCount=%d", mcts.PrunedCount)
	}
}

// Belt-and-suspenders: even with pruning enabled, a strategy with a
// combo piece in the pool must still cast the combo piece — the combo
// bonus (+0.5 in evaluateCandidate) keeps it well above any threshold.
// This pins that pruning didn't accidentally regress the combo-pref
// behavior that TestMCTSHat_PrefersCombopiece guards.
func TestMCTSHat_PruningDoesNotBreakComboPreference(t *testing.T) {
	gs := newTestGame(t, 2)
	gs.Seats[0].Life = 40
	gs.Seats[0].StartingLife = 40
	gs.Seats[1].Life = 40
	gs.Seats[1].StartingLife = 40

	sp := &StrategyProfile{
		Archetype: ArchetypeCombo,
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Oracle", "Consultation"}, Type: "infinite"},
		},
	}

	mcts := NewMCTSHat(NewPokerHat(), sp, 50)
	// Default pruning enabled.

	oracle := newTestCardMinimal("Oracle", []string{"creature"}, 2, nil)
	generic := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	consultCard := newTestCardMinimal("Consultation", []string{"instant"}, 1, nil)
	newTestPermanent(gs.Seats[0], consultCard, 0, 0)

	castable := []*gameengine.Card{oracle, generic}
	gs.Seats[0].Hand = castable
	gs.Seats[0].ManaPool = 2

	choice := mcts.ChooseCastFromHand(gs, 0, castable)
	if choice == nil {
		t.Fatal("should pick a card")
	}
	if choice.DisplayName() != "Oracle" {
		t.Errorf("default pruning must not regress combo preference: got %s", choice.DisplayName())
	}
}

// Compile-time + runtime check that the new fields don't break the Hat
// interface implementation.
func TestMCTSHat_PruningFieldsInterfaceStable(t *testing.T) {
	var _ gameengine.Hat = &MCTSHat{
		Inner:          &GreedyHat{},
		Evaluator:      NewEvaluator(nil),
		Budget:         50,
		PruneThreshold: -0.5,
		PrunedCount:    0,
	}
}
