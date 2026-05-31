package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ComboProximity r60-final regressions covering three layered signals
// on top of the existing dimension (piece availability + graveyard
// recursion engine + tutor credit + cost-reducer + emergent synergy):
//
//   - Signal A: command-zone combo-piece credit (0.8 weight). Pre-r60
//     scoreCombo ignored CommandZone, so a commander-piece in command
//     zone scored as zero like the library.
//   - Signal B: protected-combo-piece-on-battlefield bonus. Combo
//     pieces with hexproof / shroud / indestructible / ward /
//     protection-from are secured against the targeted-removal lane
//     and represent narrower-disruption-window proximity.
//   - Signal C: stack-window interruption penalty. When close to
//     closing (bestRatio > 0.7) AND the pod has 4+ total untapped
//     sources, the cast-and-respond exposure deflates proximity.
//
// Each signal pinned independently; cross-signal pin confirms all
// three additions stack without breaking the existing dimension
// contract (bestRatio still saturates at 1.0 → 2.0 final).

// ---------------------------------------------------------------------
// Shared builders
// ---------------------------------------------------------------------

// cpGame: 4-seat commander game with seat 0 perspective, opt-in
// Strategy.ComboPieces.
func cpGame(t *testing.T, pieces []string) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 4)
	gs.CommanderFormat = true
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	_ = pieces
	return gs
}

func cpStrategy(pieces []string) *StrategyProfile {
	return &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: pieces, Type: "infinite"},
		},
	}
}

// ---------------------------------------------------------------------
// Signal A — command-zone combo-piece credit
// ---------------------------------------------------------------------

func TestScoreComboR60v2_CommandZonePieceCreditsAt08(t *testing.T) {
	gs := cpGame(t, nil)
	// 3-piece combo, 1 piece in command zone, 1 piece in hand, 1 missing.
	gs.Seats[0].CommanderNames = []string{"Kiki-Jiki, Mirror Breaker"}
	kiki := newTestCardMinimal("Kiki-Jiki, Mirror Breaker", []string{"creature", "legendary"}, 5, nil)
	gs.Seats[0].CommandZone = []*gameengine.Card{kiki}
	gs.Seats[0].Hand = []*gameengine.Card{
		newTestCardMinimal("Restoration Angel", []string{"creature"}, 4, nil),
	}

	ev := NewEvaluator(cpStrategy([]string{"Kiki-Jiki, Mirror Breaker", "Restoration Angel", "Felidar Guardian"}))
	withCmdZone := ev.scoreCombo(gs, 0)

	// Counterfactual: same setup but commander missing entirely.
	gsNoCmdr := cpGame(t, nil)
	gsNoCmdr.Seats[0].Hand = []*gameengine.Card{
		newTestCardMinimal("Restoration Angel", []string{"creature"}, 4, nil),
	}
	noCmdZone := ev.scoreCombo(gsNoCmdr, 0)

	if withCmdZone <= noCmdZone {
		t.Errorf("commander piece in command zone should out-score absent commander; with=%.4f no=%.4f",
			withCmdZone, noCmdZone)
	}
	// Expected ratio: (1.0 + 0.8 + 0) / 3 = 0.6; × 1.5 final = 0.9.
	// Counterfactual ratio: (1.0 + 0 + 0) / 3 = 0.333; × 1.5 = 0.5.
	// Delta ~ 0.4.
	delta := withCmdZone - noCmdZone
	if delta < 0.30 || delta > 0.50 {
		t.Errorf("cmd-zone delta off: %.4f (want ~0.40)", delta)
	}
}

// ---------------------------------------------------------------------
// Signal B — protected combo piece on battlefield
// ---------------------------------------------------------------------

func TestScoreComboR60v2_ProtectedPieceBonus(t *testing.T) {
	mk := func(protectKiki bool) *gameengine.GameState {
		gs := cpGame(t, nil)
		kiki := newTestCardMinimal("Kiki-Jiki, Mirror Breaker", []string{"creature", "legendary"}, 5, nil)
		p := newTestPermanent(gs.Seats[0], kiki, 2, 4)
		if protectKiki {
			addKeyword(p, "indestructible")
		}
		// One piece in hand to push bestRatio > 0.2 so the protection
		// bonus is gated open.
		gs.Seats[0].Hand = []*gameengine.Card{
			newTestCardMinimal("Restoration Angel", []string{"creature"}, 4, nil),
		}
		return gs
	}

	ev := NewEvaluator(cpStrategy([]string{"Kiki-Jiki, Mirror Breaker", "Restoration Angel", "Felidar Guardian"}))
	withProtect := ev.scoreCombo(mk(true), 0)
	noProtect := ev.scoreCombo(mk(false), 0)

	if withProtect <= noProtect {
		t.Errorf("indestructible combo piece should out-score vanilla; with=%.4f no=%.4f",
			withProtect, noProtect)
	}
}

func TestScoreComboR60v2_ProtectedPieceBonus_GatedBelowQuarterRatio(t *testing.T) {
	// 5-piece combo, 1 protected piece, 4 missing → ratio = 0.2 (exactly
	// the boundary). Strict > 0.2 means bonus does NOT fire.
	gs := cpGame(t, nil)
	kiki := newTestCardMinimal("Kiki-Jiki, Mirror Breaker", []string{"creature", "legendary"}, 5, nil)
	p := newTestPermanent(gs.Seats[0], kiki, 2, 4)
	addKeyword(p, "indestructible")

	ev := NewEvaluator(cpStrategy([]string{
		"Kiki-Jiki, Mirror Breaker", "Piece B", "Piece C", "Piece D", "Piece E",
	}))
	withProtect := ev.scoreCombo(gs, 0)

	// Counterfactual: no indestructible.
	gs2 := cpGame(t, nil)
	kiki2 := newTestCardMinimal("Kiki-Jiki, Mirror Breaker", []string{"creature", "legendary"}, 5, nil)
	_ = newTestPermanent(gs2.Seats[0], kiki2, 2, 4)
	noProtect := ev.scoreCombo(gs2, 0)

	if withProtect != noProtect {
		t.Errorf("at bestRatio == 0.2 (boundary), protection bonus should NOT fire (strict > 0.2 gate); with=%.4f no=%.4f",
			withProtect, noProtect)
	}
}

// ---------------------------------------------------------------------
// Signal C — stack-window interruption penalty
// ---------------------------------------------------------------------

func TestScoreComboR60v2_StackWindowPenalty_FiresWhenClose(t *testing.T) {
	mk := func(oppUntapped int) *gameengine.GameState {
		gs := cpGame(t, nil)
		// 2-of-2 pieces in hand → bestRatio = 1.0 before bonuses → close.
		gs.Seats[0].Hand = []*gameengine.Card{
			newTestCardMinimal("Piece A", []string{"creature"}, 2, nil),
			newTestCardMinimal("Piece B", []string{"creature"}, 2, nil),
		}
		// Opp 1 gets the untapped lands.
		for i := 0; i < oppUntapped; i++ {
			_ = newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Island", []string{"land"}, 0, nil), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(cpStrategy([]string{"Piece A", "Piece B"}))
	openPod := ev.scoreCombo(mk(8), 0)   // 8 untapped → over=5, penalty=0.10 (capped)
	tappedOut := ev.scoreCombo(mk(0), 0) // 0 untapped → no penalty

	if openPod >= tappedOut {
		t.Errorf("close-to-closing combo vs open pod should score WORSE than tapped-out pod; open=%.4f tapped=%.4f",
			openPod, tappedOut)
	}
}

func TestScoreComboR60v2_StackWindowPenalty_InertWhenFarFromClose(t *testing.T) {
	// 3-piece combo, 0 pieces in hand → bestRatio = 0 → far from close.
	// Stack-window penalty must NOT fire (bestRatio > 0.7 gate).
	mk := func(oppUntapped int) *gameengine.GameState {
		gs := cpGame(t, nil)
		for i := 0; i < oppUntapped; i++ {
			_ = newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Island", []string{"land"}, 0, nil), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(cpStrategy([]string{"Piece A", "Piece B", "Piece C"}))
	openPod := ev.scoreCombo(mk(10), 0)
	tappedOut := ev.scoreCombo(mk(0), 0)

	if openPod != tappedOut {
		t.Errorf("far-from-close should not trigger stack-window penalty; open=%.4f tapped=%.4f",
			openPod, tappedOut)
	}
}

func TestScoreComboR60v2_StackWindowPenalty_GatedAt4Sources(t *testing.T) {
	// Close to closing, opp has 3 untapped → below the >=4 floor.
	mk := func(oppUntapped int) *gameengine.GameState {
		gs := cpGame(t, nil)
		gs.Seats[0].Hand = []*gameengine.Card{
			newTestCardMinimal("Piece A", []string{"creature"}, 2, nil),
			newTestCardMinimal("Piece B", []string{"creature"}, 2, nil),
		}
		for i := 0; i < oppUntapped; i++ {
			_ = newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Island", []string{"land"}, 0, nil), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(cpStrategy([]string{"Piece A", "Piece B"}))
	belowGate := ev.scoreCombo(mk(3), 0)
	aboveGate := ev.scoreCombo(mk(5), 0) // over=2 → penalty=0.04

	if belowGate <= aboveGate {
		t.Errorf("3 untapped (below gate) should NOT penalize; 5 untapped SHOULD; below=%.4f above=%.4f",
			belowGate, aboveGate)
	}
}

// ---------------------------------------------------------------------
// Cross-signal: all three additions co-exist correctly
// ---------------------------------------------------------------------

func TestScoreComboR60v2_AllSignalsCompose(t *testing.T) {
	// Setup: 3-piece combo with commander as a piece.
	//   - Kiki in command zone (cmd-zone credit: 0.8)
	//   - Restoration Angel on battlefield with hexproof (1.0 + prot bonus)
	//   - Felidar Guardian in hand (1.0)
	// Total: complete combo. bestRatio = (0.8 + 1.0 + 1.0)/3 = 0.933.
	// Protection bonus pushes ratio higher (+0.04 = 0.973).
	// Stack-window penalty: 0 opp untapped → no fire.
	gs := cpGame(t, nil)
	gs.Seats[0].CommanderNames = []string{"Kiki-Jiki, Mirror Breaker"}
	gs.Seats[0].CommandZone = []*gameengine.Card{
		newTestCardMinimal("Kiki-Jiki, Mirror Breaker", []string{"creature", "legendary"}, 5, nil),
	}
	angel := newTestCardMinimal("Restoration Angel", []string{"creature"}, 4, nil)
	p := newTestPermanent(gs.Seats[0], angel, 3, 4)
	addKeyword(p, "hexproof")
	gs.Seats[0].Hand = []*gameengine.Card{
		newTestCardMinimal("Felidar Guardian", []string{"creature"}, 4, nil),
	}

	ev := NewEvaluator(cpStrategy([]string{"Kiki-Jiki, Mirror Breaker", "Restoration Angel", "Felidar Guardian"}))
	got := ev.scoreCombo(gs, 0)

	// Counterfactual: same plan but Kiki absent entirely (no cmd zone,
	// no battlefield, no hand), Angel vanilla.
	gsBase := cpGame(t, nil)
	angelBase := newTestCardMinimal("Restoration Angel", []string{"creature"}, 4, nil)
	_ = newTestPermanent(gsBase.Seats[0], angelBase, 3, 4)
	gsBase.Seats[0].Hand = []*gameengine.Card{
		newTestCardMinimal("Felidar Guardian", []string{"creature"}, 4, nil),
	}
	base := ev.scoreCombo(gsBase, 0)

	if got <= base {
		t.Errorf("combined signals should out-score baseline; got=%.4f base=%.4f", got, base)
	}
}
