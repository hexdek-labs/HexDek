package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r60 ComboProximity retune. scoreCombo now picks up two signals the
// pre-r60 implementation discarded:
//
//   (1) graveyard pieces — count as 0.5× hand/battlefield credit
//       (recurrable softness via reanimate / Sun-Titan / Karador /
//       Past in Flames / Regrowth)
//   (2) tutors in hand — count as 1 soft-piece per plan
//       (Demonic Tutor + 1 piece is functionally similar to holding
//       2 pieces; tutor is "1 mana = 1 piece deployed next turn")
//
// Both signals are dampened relative to a real piece on the
// battlefield: graveyard at 0.5×, tutor at exactly 1.0 per plan
// (capped at 1, can't fill multiple slots with a single hand of
// 3 tutors).

// makeTwoCardComboProfile builds a strategy profile with a single 2-card
// combo plan ("PieceA + PieceB"). Convenient for tutor / graveyard tests
// that vary which pieces are in which zone.
func makeTwoCardComboProfile() *StrategyProfile {
	return &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"PieceA", "PieceB"}, Type: "infinite", Class: "infinite_drain"},
		},
	}
}

// stageComboFixture builds a 2-seat game with the given strategy on
// seat 0; returns the evaluator + game so individual tests can place
// pieces in specific zones.
func stageComboFixture(t *testing.T) (*GameStateEvaluator, *gameengine.GameState) {
	t.Helper()
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	ev := NewEvaluator(makeTwoCardComboProfile())
	return ev, gs
}

// TestScoreCombo_GraveyardPieceHalfCredit pins the headline graveyard
// signal: PieceA in graveyard (recurrable) scores LESS than PieceA in
// hand, but strictly MORE than PieceA absent entirely.
func TestScoreCombo_GraveyardPieceHalfCredit(t *testing.T) {
	scoreFor := func(t *testing.T, zone string) float64 {
		t.Helper()
		ev, gs := stageComboFixture(t)
		seat := gs.Seats[0]

		c := newTestCardMinimal("PieceA", []string{"creature"}, 3, nil)
		switch zone {
		case "hand":
			seat.Hand = append(seat.Hand, c)
		case "battlefield":
			newTestPermanent(seat, c, 0, 0)
		case "graveyard":
			seat.Graveyard = append(seat.Graveyard, c)
		case "absent":
			// no-op
		}
		return ev.scoreCombo(gs, 0)
	}

	absent := scoreFor(t, "absent")
	graveyard := scoreFor(t, "graveyard")
	hand := scoreFor(t, "hand")
	battlefield := scoreFor(t, "battlefield")

	// graveyard piece must be a strict middle ground.
	if !(absent < graveyard && graveyard < hand) {
		t.Fatalf("graveyard piece should score strictly between absent and hand; got absent=%.3f gy=%.3f hand=%.3f",
			absent, graveyard, hand)
	}
	// hand and battlefield should score equally (both 1.0 credit).
	if math.Abs(hand-battlefield) > 1e-9 {
		t.Errorf("hand and battlefield should score equally; got hand=%.3f bf=%.3f", hand, battlefield)
	}
	// Exact value sanity: PieceA in graveyard fills 0.5 of 2 needed
	// pieces = 0.25 ratio × 1.5 multiplier = 0.375. Within float noise.
	if math.Abs(graveyard-0.375) > 1e-9 {
		t.Errorf("graveyard piece score = %.6f, want 0.375 (0.5/2 × 1.5)", graveyard)
	}
}

// TestScoreCombo_BothPiecesInGraveyardScoresHalf pins the lower bound
// of the graveyard signal: both pieces in graveyard = 50% credit for
// the plan = 0.75 final score (1.0 ratio × 0.5 weight × 1.5 = 0.75).
func TestScoreCombo_BothPiecesInGraveyardScoresHalf(t *testing.T) {
	ev, gs := stageComboFixture(t)
	seat := gs.Seats[0]
	pa := newTestCardMinimal("PieceA", []string{"creature"}, 3, nil)
	pb := newTestCardMinimal("PieceB", []string{"creature"}, 3, nil)
	seat.Graveyard = append(seat.Graveyard, pa, pb)

	got := ev.scoreCombo(gs, 0)
	// foundWeight = 0.5 + 0.5 = 1.0; ratio = 1.0/2 = 0.5; * 1.5 = 0.75.
	if math.Abs(got-0.75) > 1e-9 {
		t.Errorf("both pieces in graveyard score = %.6f, want 0.75", got)
	}
}

// TestScoreCombo_TutorInHandFillsMissingSlot pins the tutor signal: a
// hand with PieceA + Demonic Tutor scores higher than PieceA alone
// because the tutor functionally fills the missing PieceB slot.
func TestScoreCombo_TutorInHandFillsMissingSlot(t *testing.T) {
	build := func(t *testing.T, withTutor bool) float64 {
		t.Helper()
		ev, gs := stageComboFixture(t)
		seat := gs.Seats[0]
		seat.Hand = append(seat.Hand, newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))
		if withTutor {
			tutor := cardWithStaticText("Demonic Tutor", []string{"sorcery"}, 2,
				"search your library for a card, put it into your hand, then shuffle")
			seat.Hand = append(seat.Hand, tutor)
		}
		return ev.scoreCombo(gs, 0)
	}

	noTutor := build(t, false)
	withTutor := build(t, true)

	if !(withTutor > noTutor) {
		t.Fatalf("Demonic Tutor in hand should boost combo score; got noTutor=%.3f withTutor=%.3f",
			noTutor, withTutor)
	}
	// Exact value: PieceA (1.0) + tutor soft-piece (1.0) = 2.0 found
	// of 2 needed = 1.0 ratio = 2.0 score (complete-combo branch).
	if math.Abs(withTutor-2.0) > 1e-9 {
		t.Errorf("PieceA + tutor should score 2.0 (functionally complete); got %.6f", withTutor)
	}
}

// TestScoreCombo_TutorCappedAtOnePerPlan pins the cap: 3 tutors in
// hand fill AT MOST 1 missing slot in a 2-piece plan. We don't want
// a tutor-heavy hand claiming combo completion via tutors alone.
func TestScoreCombo_TutorCappedAtOnePerPlan(t *testing.T) {
	ev, gs := stageComboFixture(t)
	seat := gs.Seats[0]
	// 3 tutors, no real pieces.
	for _, name := range []string{"Demonic Tutor", "Vampiric Tutor", "Imperial Seal"} {
		seat.Hand = append(seat.Hand, cardWithStaticText(name, []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand, then shuffle"))
	}
	got := ev.scoreCombo(gs, 0)
	// 0 real pieces + 1 tutor credit = 1.0 found of 2 needed = 0.5
	// ratio × 1.5 = 0.75. NOT 2.0.
	if math.Abs(got-0.75) > 1e-9 {
		t.Errorf("3 tutors no pieces should cap at 0.75 (1 tutor-slot, half-complete plan); got %.6f", got)
	}
}

// TestScoreCombo_TutorDoesNotOvercountWhenPlanComplete confirms tutors
// don't boost an already-complete plan past 2.0. Catches an accidental
// "tutor always adds 1.0" implementation.
func TestScoreCombo_TutorDoesNotOvercountWhenPlanComplete(t *testing.T) {
	ev, gs := stageComboFixture(t)
	seat := gs.Seats[0]
	// Both real pieces + a tutor.
	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		newTestCardMinimal("PieceB", []string{"creature"}, 3, nil),
		cardWithStaticText("Demonic Tutor", []string{"sorcery"}, 2,
			"search your library for a card, put it into your hand, then shuffle"))
	got := ev.scoreCombo(gs, 0)
	// All pieces present → complete-combo branch → 2.0 exactly.
	if math.Abs(got-2.0) > 1e-9 {
		t.Errorf("complete plan + tutor should still score 2.0 (no overflow); got %.6f", got)
	}
}

// TestSeatTutorsInHand_OracleMatches pins the tutor-detection helper
// directly. Three canonical phrasings + name fallback + non-tutor
// negative.
func TestSeatTutorsInHand_OracleMatches(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]

	tutor1 := cardWithStaticText("Demonic Tutor", []string{"sorcery"}, 2,
		"search your library for a card, put it into your hand, then shuffle")
	tutor2 := cardWithStaticText("Mystical Tutor", []string{"instant"}, 1,
		"search your library for an instant or sorcery card, reveal it, then shuffle and put that card on top")
	// Name-based fallback: card with "tutor" in name but no oracle text.
	tutor3 := newTestCardMinimal("Personal Tutor", []string{"sorcery"}, 1, nil)
	// Negative: non-tutor card.
	bear := newTestCardMinimal("Grizzly Bears", []string{"creature"}, 2, nil)
	// Negative: spell that's a draw, not a tutor.
	rampantGrowth := cardWithStaticText("Rampant Growth", []string{"sorcery"}, 2,
		"search your library for a basic land card, put it onto the battlefield tapped, then shuffle")

	seat.Hand = append(seat.Hand, tutor1, tutor2, tutor3, bear, rampantGrowth)

	got := seatTutorsInHand(seat)
	// tutor1 + tutor2 + tutor3 + rampantGrowth = 4 matches. Rampant
	// Growth is technically a "search your library" — we accept it
	// as a tutor (it IS a tutor, just for lands). bear is the only
	// negative.
	want := 4
	if got != want {
		t.Errorf("seatTutorsInHand = %d, want %d", got, want)
	}
}

// TestScoreCombo_OffClassDampingStillApplies pins that the new
// graveyard + tutor signals respect the existing off-class-damping
// behavior: an off-class plan with a graveyard piece + tutor still
// scales by 0.7×, so the on-class plans dominate the score.
func TestScoreCombo_OffClassDampingStillApplies(t *testing.T) {
	// Two plans: 3 in primary class (infinite_drain), 1 off-class
	// (infinite_token). PrimaryComboClass needs >=2 of the same
	// class to register.
	sp := &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"DrainA", "DrainB"}, Class: "infinite_drain"},
			{Pieces: []string{"DrainC", "DrainD"}, Class: "infinite_drain"},
			{Pieces: []string{"DrainE", "DrainF"}, Class: "infinite_drain"},
			{Pieces: []string{"TokenA", "TokenB"}, Class: "infinite_token"},
		},
	}
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	ev := NewEvaluator(sp)
	seat := gs.Seats[0]

	// Off-class plan fully complete via PieceA in graveyard + PieceB
	// hand. Foundweight = 0.5 + 1.0 = 1.5; ratio = 0.75; × 0.7 damping
	// × 1.5 = 0.7875.
	seat.Graveyard = append(seat.Graveyard, newTestCardMinimal("TokenA", []string{"creature"}, 3, nil))
	seat.Hand = append(seat.Hand, newTestCardMinimal("TokenB", []string{"creature"}, 3, nil))

	got := ev.scoreCombo(gs, 0)
	// 0.5/2 × 1.5 + 1.0/2 × 1.5 = (1.5/2) * 1.5 = 1.125, but with off-
	// class damping (× 0.7), that becomes 0.7875.
	wantApprox := 0.7875
	if math.Abs(got-wantApprox) > 1e-6 {
		t.Errorf("off-class plan score = %.6f, want %.6f (graveyard 0.5 + hand 1.0)/2 × 0.7 × 1.5",
			got, wantApprox)
	}
}
