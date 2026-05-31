package hat

import (
	"math"
	"testing"
)

// r60-cedh-tuning: multi-tutor credit. The pre-tuning scoreCombo capped
// tutor-credit at 1 soft-piece per plan regardless of how many tutors
// the seat held. That cap was correct as a "no tutor-only completion"
// guard but wrong as a ceiling — the canonical cEDH reach pattern is
// (1 anchoring piece in hand + 2 tutors → 3-piece combo functionally
// complete). These tests pin the new behavior: tutor credit scales with
// missing slots, capped by (realPiecesFound + 1) so the no-piece guard
// stays intact.
//
// Companion to evaluator_combo_zone_tutor_r60_test.go's existing
// TestScoreCombo_TutorCappedAtOnePerPlan (3 tutors / no real pieces →
// 0.75, unchanged) and TestScoreCombo_TutorInHandFillsMissingSlot (1
// real + 1 tutor / 2-piece plan → 2.0, unchanged).

// makeThreeCardComboProfile builds a strategy profile with a single
// 3-card combo plan. Used to test the (1 piece + 2 tutors) reach case
// where the original 1-cap missed.
func makeThreeCardComboProfile() *StrategyProfile {
	return &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"PieceA", "PieceB", "PieceC"}, Type: "infinite", Class: "infinite_drain"},
		},
	}
}

// TestScoreCombo_MultiTutorMultipleMissingSlots is the headline new
// behavior: 1 anchoring real piece + 2 tutors closes a 3-piece combo
// in cEDH practice. The pre-tuning code returned 1.0 (foundWeight = 1.0
// real + 1.0 tutor = 2.0/3 = 0.667 × 1.5); the tuned code returns 2.0
// (foundWeight = 1.0 + 2 tutors capped at realPiecesFound+1=2 → 3/3 = 1.0).
func TestScoreCombo_MultiTutorMultipleMissingSlots(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	ev := NewEvaluator(makeThreeCardComboProfile())
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		cardWithStaticText("Demonic Tutor", []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand, then shuffle"),
		cardWithStaticText("Vampiric Tutor", []string{"instant"}, 1,
			"search your library for a card, then shuffle and put that card on top"),
	)

	got := ev.scoreCombo(gs, 0)
	// foundWeight = 1.0 (PieceA) + 2.0 (2 tutors, capped at realPiecesFound+1=2)
	// = 3.0 / 3 = 1.0 ratio → complete-combo branch → 2.0.
	if math.Abs(got-2.0) > 1e-9 {
		t.Errorf("(1 piece + 2 tutors / 3-piece combo) should score 2.0 (functionally complete); got %.6f", got)
	}
}

// TestScoreCombo_MultiTutorAnchorRequired pins the guard direction: 0
// real pieces + 2 tutors in a 3-piece combo STILL caps at 1 soft-piece,
// preserving the "no completion via tutors alone" rule that
// TestScoreCombo_TutorCappedAtOnePerPlan enforces.
func TestScoreCombo_MultiTutorAnchorRequired(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	ev := NewEvaluator(makeThreeCardComboProfile())
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		cardWithStaticText("Demonic Tutor", []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand, then shuffle"),
		cardWithStaticText("Vampiric Tutor", []string{"instant"}, 1,
			"search your library for a card, then shuffle and put that card on top"),
	)

	got := ev.scoreCombo(gs, 0)
	// realPiecesFound = 0, cap = 1, tutorCredit = min(2, 3, 1) = 1.
	// foundWeight = 1.0 / 3 = 0.333... ratio × 1.5 = 0.5.
	want := 0.5
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("(0 pieces + 2 tutors / 3-piece combo) should cap at %.6f (anchor guard); got %.6f", want, got)
	}
}

// TestScoreCombo_MultiTutorPiecesMatchMissing pins the missing-slot cap:
// 2 real pieces + 3 tutors in a 3-piece combo cannot overflow past the
// 1 missing slot. foundWeight = 2.0 + min(3 tutors, 1 missing, 2+1 cap) =
// 2.0 + 1 = 3.0 → complete (2.0).
func TestScoreCombo_MultiTutorPiecesMatchMissing(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	ev := NewEvaluator(makeThreeCardComboProfile())
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		newTestCardMinimal("PieceB", []string{"creature"}, 3, nil),
		cardWithStaticText("Demonic Tutor", []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand, then shuffle"),
		cardWithStaticText("Vampiric Tutor", []string{"instant"}, 1,
			"search your library for a card, then shuffle and put that card on top"),
		cardWithStaticText("Imperial Seal", []string{"sorcery"}, 1,
			"search your library for a card, then shuffle and put that card on top"),
	)

	got := ev.scoreCombo(gs, 0)
	// Complete via 2 real + 1 tutor credit; remaining 2 tutors discarded.
	if math.Abs(got-2.0) > 1e-9 {
		t.Errorf("(2 pieces + 3 tutors / 3-piece combo) should score 2.0 (complete, no overflow); got %.6f", got)
	}
}

// TestScoreCombo_OneTutorOnePieceTwoPieceCombo is the existing-behavior
// preservation case: 1 piece + 1 tutor in a 2-piece combo still scores
// 2.0 (same as the pre-tuning TestScoreCombo_TutorInHandFillsMissingSlot).
// Defends against an accidental regression in the simple-case path while
// the multi-tutor branch is being tuned.
func TestScoreCombo_OneTutorOnePieceTwoPieceCombo(t *testing.T) {
	ev, gs := stageComboFixture(t)
	seat := gs.Seats[0]

	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		cardWithStaticText("Demonic Tutor", []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand, then shuffle"),
	)

	got := ev.scoreCombo(gs, 0)
	if math.Abs(got-2.0) > 1e-9 {
		t.Errorf("(1 piece + 1 tutor / 2-piece combo) should score 2.0; got %.6f", got)
	}
}

// TestScoreCombo_MultiTutorGraveyardAnchor confirms the anchor count
// includes graveyard pieces. PieceA in graveyard (weight 0.5) anchors
// 2 tutors in a 3-piece combo because realPiecesFound counts graveyard
// presence (it IS a real piece, just at half credit). foundWeight =
// 0.5 + min(2, 2, 1+1=2) = 0.5 + 2.0 = 2.5 / 3 = 0.833 ratio × 1.5 = 1.25.
func TestScoreCombo_MultiTutorGraveyardAnchor(t *testing.T) {
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	ev := NewEvaluator(makeThreeCardComboProfile())
	seat := gs.Seats[0]

	seat.Graveyard = append(seat.Graveyard, newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))
	seat.Hand = append(seat.Hand,
		cardWithStaticText("Demonic Tutor", []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand, then shuffle"),
		cardWithStaticText("Vampiric Tutor", []string{"instant"}, 1,
			"search your library for a card, then shuffle and put that card on top"),
	)

	got := ev.scoreCombo(gs, 0)
	// foundWeight = 0.5 (graveyard) + 2.0 (2 tutors capped at realPiecesFound+1=2)
	// = 2.5; ratio = 2.5/3 = 0.8333...; × 1.5 = 1.25.
	want := 1.25
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("(graveyard piece + 2 tutors / 3-piece combo) should score %.6f; got %.6f", want, got)
	}
}
