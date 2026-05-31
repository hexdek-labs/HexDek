package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r60-cedh-sequencer: companion tests for the broadened Assembling gate
// in combo_sequencer.go Evaluate and the combo-priority cast-order bias
// in cardHeuristic. The pre-tuning gate required missing == 1; the new
// gate is realPiecesFound >= 1 AND tutorsInHand >= missing. The cast-
// order bias pushes combo pieces and tutors strictly above value engines
// when the hat is in PlanAssemble or PlanExecute.
//
// The two existing sequencer-test contracts continue to hold under the
// broadened gate:
//   - TestComboSequencer_Assembling_OnePieceMissingWithTutor:
//     1 piece + 1 tutor / 2-piece combo → Assembling (boundary case
//     where missing == 1 AND tutors == 1 ≥ 1).
//   - TestComboSequencer_NotAssembling_NoTutor:
//     1 piece + 0 tutors / 2-piece combo → not Assembling (tutors < missing).

// makeThreeCardSequencerProfile returns a profile with a single 3-card
// combo plan, used to demonstrate that 2-piece-missing cases now trigger
// Assembling when enough tutors are in hand.
func makeThreeCardSequencerProfile() *StrategyProfile {
	return &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"PieceA", "PieceB", "PieceC"}, Type: "infinite", Class: "infinite_drain"},
		},
	}
}

func sequencerTutor(name string, cmc int) *gameengine.Card {
	return cardWithStaticText(name, []string{"sorcery"}, cmc,
		"search your library for a card, put it into your hand, then shuffle")
}

// TestComboSequencer_AssemblingBroadenedToMultiTutorReach pins the
// headline gate change: 1 anchoring piece + 2 tutors / 3-piece combo
// now reports Assembling=true (previously Assembling stayed false
// because missing == 2, not 1).
func TestComboSequencer_AssemblingBroadenedToMultiTutorReach(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		sequencerTutor("Demonic Tutor", 2),
		sequencerTutor("Vampiric Tutor", 1),
	)

	cs := NewComboSequencer(makeThreeCardSequencerProfile())
	result := cs.Evaluate(gs, 0)

	if result.Executable {
		t.Fatalf("not executable yet (2 missing pieces); got Executable=true")
	}
	if !result.Assembling {
		t.Fatalf("should be Assembling (1 piece + 2 tutors covers both missing slots); got Assembling=false")
	}
	if result.PiecesFound != 1 {
		t.Errorf("PiecesFound = %d, want 1", result.PiecesFound)
	}
	if result.MissingPiece == "" {
		t.Errorf("MissingPiece should be set to the first missing slot for the tutor target")
	}
}

// TestComboSequencer_NotAssemblingTutorsBelowMissing pins the gate
// counterfactual: 1 anchoring piece + 1 tutor / 3-piece combo does NOT
// trigger Assembling because tutors (1) < missing (2). Defends against
// over-broadening — we don't want every "1 piece + 1 tutor" hand to
// claim Assembling in deeper combos than the tutors can cover.
func TestComboSequencer_NotAssemblingTutorsBelowMissing(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		sequencerTutor("Demonic Tutor", 2),
	)

	cs := NewComboSequencer(makeThreeCardSequencerProfile())
	result := cs.Evaluate(gs, 0)

	if result.Assembling {
		t.Fatalf("should NOT be Assembling: 1 tutor cannot cover 2 missing slots; got Assembling=true")
	}
}

// TestComboSequencer_NotAssemblingZeroPieces pins the anchor-required
// half of the gate: 0 real pieces + 3 tutors still does NOT trigger
// Assembling, mirroring the scoreCombo anchor guard
// (TestScoreCombo_TutorCappedAtOnePerPlan).
func TestComboSequencer_NotAssemblingZeroPieces(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		sequencerTutor("Demonic Tutor", 2),
		sequencerTutor("Vampiric Tutor", 1),
		sequencerTutor("Imperial Seal", 1),
	)

	cs := NewComboSequencer(makeThreeCardSequencerProfile())
	result := cs.Evaluate(gs, 0)

	if result.Assembling {
		t.Fatalf("should NOT be Assembling without an anchoring piece; got Assembling=true")
	}
}

// TestCardHeuristic_ComboPriorityBiasInPlanAssemble pins the cast-order
// bias: when planState.Current is PlanAssemble, a combo piece scores
// strictly higher than a same-CMC value engine (which now takes the
// -0.15 demotion). The bias is what actually pulls the cast queue
// toward wincon assembly — without it, PlanAssemble only changes the
// MCTS dimensional weights at the leaves and the gameplan picks the
// same spells first as before.
func TestCardHeuristic_ComboPriorityBiasInPlanAssemble(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"Thassa's Oracle", "Demonic Consultation"}, Type: "infinite"},
		},
		ValueEngineKeys: []string{"Rhystic Study"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	// Force planState into PlanAssemble — the gate is exercised by
	// the broadened sequencer above; here we want to isolate the
	// heuristic bias, not the gating logic.
	h.planState.Current = PlanAssemble

	combo := newTestCardMinimal("Thassa's Oracle", []string{"creature"}, 2, nil)
	value := newTestCardMinimal("Rhystic Study", []string{"enchantment"}, 3, nil)

	comboScore := h.cardHeuristic(gs, 0, combo)
	valueScore := h.cardHeuristic(gs, 0, value)

	if !(comboScore > valueScore) {
		t.Fatalf("combo piece should outrank value engine in PlanAssemble; got combo=%.3f value=%.3f", comboScore, valueScore)
	}
	// Quantitative pin: the +0.40 / -0.15 delta should produce a gap
	// of at least 0.40 in favor of the combo piece, before any
	// other dimension-specific shifts (these two test cards are
	// minimal so no other bonuses apply).
	if comboScore-valueScore < 0.40 {
		t.Errorf("combo-vs-value gap should be at least 0.40 (combo +0.40, value -0.15); got gap=%.3f", comboScore-valueScore)
	}
}

// TestCardHeuristic_TutorPriorityBiasInPlanAssemble: a tutor scores
// strictly higher than a non-combo value engine in PlanAssemble. The
// tutor +0.35 bonus is one step below the combo-piece +0.40 (so combo
// pieces stay above tutors when both are in hand), but the tutor still
// clearly outranks the demoted value engine (-0.15).
func TestCardHeuristic_TutorPriorityBiasInPlanAssemble(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		ComboPieces:  []ComboPlan{{Pieces: []string{"PieceA", "PieceB"}, Type: "infinite"}},
		ValueEngineKeys: []string{"Rhystic Study"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	h.planState.Current = PlanAssemble

	tutor := sequencerTutor("Demonic Tutor", 2)
	value := newTestCardMinimal("Rhystic Study", []string{"enchantment"}, 3, nil)

	tutorScore := h.cardHeuristic(gs, 0, tutor)
	valueScore := h.cardHeuristic(gs, 0, value)

	if !(tutorScore > valueScore) {
		t.Fatalf("tutor should outrank value engine in PlanAssemble; got tutor=%.3f value=%.3f", tutorScore, valueScore)
	}
}

// TestCardHeuristic_ComboPriorityBiasNotInPlanDevelop pins the gating:
// in PlanDevelop (the default plan, no combo signal), the combo-
// priority bias should NOT fire. Value engines and combo pieces score
// without the assembling bias.
func TestCardHeuristic_ComboPriorityBiasNotInPlanDevelop(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		ComboPieces:  []ComboPlan{{Pieces: []string{"PieceA", "PieceB"}, Type: "infinite"}},
		ValueEngineKeys: []string{"Rhystic Study"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	// Default is PlanDevelop; assert explicitly.
	if h.planState.Current != PlanDevelop {
		t.Fatalf("test setup: planState should default to PlanDevelop; got %v", h.planState.Current)
	}

	combo := newTestCardMinimal("PieceA", []string{"creature"}, 2, nil)
	value := newTestCardMinimal("Rhystic Study", []string{"enchantment"}, 3, nil)

	comboScore := h.cardHeuristic(gs, 0, combo)
	valueScore := h.cardHeuristic(gs, 0, value)

	// The combo-priority bias adds +0.40 to combo pieces. In PlanDevelop
	// that block doesn't fire, so the gap should be smaller than the
	// PlanAssemble case. Specifically: combo - value should be < 0.40
	// here (it'd be > 0.40 if the bias incorrectly fired).
	gap := comboScore - valueScore
	if gap >= 0.40 {
		t.Errorf("combo-priority bias should NOT fire in PlanDevelop; got combo-value gap=%.3f (should be < 0.40)", gap)
	}
}

// TestCardHeuristic_ComboPieceOverridesValueEngine pins the priority
// ordering when a card is BOTH a combo piece and a value engine (e.g.,
// Wishclaw Talisman, which is in some decks' ComboPieces AND
// ValueEngines lists). The combo-piece branch fires first, so the card
// gets +0.40 (not -0.15). Defends the priority ordering of the switch.
func TestCardHeuristic_ComboPieceOverridesValueEngine(t *testing.T) {
	gs := newTestGame(t, 2)

	sp := &StrategyProfile{
		ComboPieces:  []ComboPlan{{Pieces: []string{"Wishclaw Talisman", "PieceB"}, Type: "infinite"}},
		ValueEngineKeys: []string{"Wishclaw Talisman"},
	}
	h := NewYggdrasilHatWithNoise(sp, 0, 0)
	h.planState.Current = PlanAssemble

	dualRole := newTestCardMinimal("Wishclaw Talisman", []string{"artifact"}, 2, nil)
	plain := newTestCardMinimal("Bauble", []string{"artifact"}, 1, nil)

	dualScore := h.cardHeuristic(gs, 0, dualRole)
	plainScore := h.cardHeuristic(gs, 0, plain)

	if !(dualScore > plainScore) {
		t.Fatalf("Wishclaw (combo piece AND value engine) should get the combo +0.40 bonus, not the value-engine -0.15; got dual=%.3f plain=%.3f",
			dualScore, plainScore)
	}
}
