package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// WinConPursuit r60 regressions. Pins the new signal across the
// canonical game states (no pieces / partial / close / complete /
// graveyard-recursion / tutor-reach / mana-gated) AND the
// cardHeuristic boost at pursuit >= 0.7.
//
// Signal calibration recap:
//   0.0..0.4  — no pursuit (early game, scattered pieces)
//   0.4..0.7  — assembling but boost gate closed
//   >= 0.7    — high pursuit: cardHeuristic adds pursuit*0.30 to
//               tutors and pursuit*0.20 to combo-relevant cards

// ---------------------------------------------------------------------
// Shared builders
// ---------------------------------------------------------------------

func wcpStrategy(pieces []string, tutorTargets []string) *StrategyProfile {
	sp := &StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: pieces, Type: "infinite"},
		},
	}
	sp.TutorTargets = tutorTargets
	return sp
}

func wcpGameWithHandLands(t *testing.T, hand []string, lands int) *gameengine.GameState {
	t.Helper()
	gs := newTestGame(t, 4)
	gs.CommanderFormat = true
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
	for _, name := range hand {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			newTestCardMinimal(name, []string{"creature"}, 3, nil))
	}
	for i := 0; i < lands; i++ {
		newTestPermanent(gs.Seats[0],
			gyCardWithOracle("Plains", []string{"land"}, 0, "{t}: add {w}"), 0, 0)
	}
	return gs
}

// ---------------------------------------------------------------------
// Signal calibration tests
// ---------------------------------------------------------------------

func TestWinConPursuit_NilSafe(t *testing.T) {
	var h *YggdrasilHat
	if got := h.WinConPursuit(nil, 0); got != 0 {
		t.Errorf("nil hat: got %.4f want 0", got)
	}
}

func TestWinConPursuit_NoStrategyReturnsZero(t *testing.T) {
	h := NewYggdrasilHat(nil, 0)
	gs := newTestGame(t, 2)
	if got := h.WinConPursuit(gs, 0); got != 0 {
		t.Errorf("no strategy: got %.4f want 0", got)
	}
}

func TestWinConPursuit_NoPiecesReturnsZero(t *testing.T) {
	sp := wcpStrategy([]string{"Piece A", "Piece B"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, nil, 4)
	if got := h.WinConPursuit(gs, 0); got != 0 {
		t.Errorf("no pieces accessible: got %.4f want 0", got)
	}
}

func TestWinConPursuit_OneOfThreeInHand(t *testing.T) {
	// 1/3 pieces in hand → bestRatio = 0.333, no tutors, no mana
	// bonus (below 0.5 ratio gate) → pursuit ≈ 0.333.
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, []string{"Piece A"}, 4)
	got := h.WinConPursuit(gs, 0)
	if got < 0.30 || got > 0.40 {
		t.Errorf("1-of-3 pursuit off: got %.4f want ~0.333", got)
	}
}

func TestWinConPursuit_TwoOfThreeInHand(t *testing.T) {
	// 2/3 pieces in hand → bestRatio = 0.667, mana bonus fires
	// (ratio >= 0.5 AND 4 untapped lands >= 3) → pursuit ≈ 0.767.
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, []string{"Piece A", "Piece B"}, 4)
	got := h.WinConPursuit(gs, 0)
	if got < 0.70 || got > 0.80 {
		t.Errorf("2-of-3 + mana pursuit off: got %.4f want ~0.767", got)
	}
}

func TestWinConPursuit_AllThreeInHand(t *testing.T) {
	// 3/3 pieces → bestRatio = 1.0, mana bonus would push past 1.0 but
	// pursuit caps.
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, []string{"Piece A", "Piece B", "Piece C"}, 4)
	got := h.WinConPursuit(gs, 0)
	if got != 1.0 {
		t.Errorf("complete + mana: got %.4f want 1.0 (capped)", got)
	}
}

func TestWinConPursuit_TutorReachInHand(t *testing.T) {
	// 1 piece in hand + 2 tutors in hand → tutor credit caps at
	// realPieces+1 = 2 missing slots filled out of 2; bestRatio = 1.0.
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, []string{"Piece A"}, 4)
	// Add 2 tutors.
	for i := 0; i < 2; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
				"search your library for a card and put that card into your hand."))
	}
	got := h.WinConPursuit(gs, 0)
	// realPieces=1, missing=2, tutors=2, credit = min(2, 2, 1+1=2) = 2.
	// foundWeight = 1+2 = 3, ratio = 3/3 = 1.0. Mana bonus pushes past
	// the 1.0 cap, so result clamps to 1.0.
	if got != 1.0 {
		t.Errorf("1 piece + 2 tutors should pursuit=1.0 (full reach); got %.4f", got)
	}
}

func TestWinConPursuit_TutorWithoutAnchorCapped(t *testing.T) {
	// 0 real pieces + 3 tutors → tutor credit cap = realPieces+1 = 1.
	// foundWeight = 0+1 = 1, ratio = 1/3 = 0.333. No mana bonus
	// (ratio < 0.5).
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, nil, 4)
	for i := 0; i < 3; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand,
			gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
				"search your library for a card and put that card into your hand."))
	}
	got := h.WinConPursuit(gs, 0)
	if got < 0.30 || got > 0.40 {
		t.Errorf("3 tutors / 0 anchors should cap at ~0.333; got %.4f", got)
	}
}

func TestWinConPursuit_GraveyardCreditDampenedWithoutEngine(t *testing.T) {
	// 1 piece in graveyard (no recursion engine) → weight 0.5.
	// 0 in hand, 0 on battlefield. ratio = 0.5/3 = 0.167.
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, nil, 4)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		newTestCardMinimal("Piece A", []string{"creature"}, 3, nil))
	got := h.WinConPursuit(gs, 0)
	if got < 0.13 || got > 0.20 {
		t.Errorf("graveyard piece (no engine): got %.4f want ~0.167", got)
	}
}

func TestWinConPursuit_GraveyardCreditUpgradedWithEngine(t *testing.T) {
	// Same setup but Animate Dead on battlefield → graveyard weight
	// upgrades to 0.9. ratio = 0.9/3 = 0.30.
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, nil, 4)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		newTestCardMinimal("Piece A", []string{"creature"}, 3, nil))
	// Animate Dead-style recursion engine.
	engine := gyCardWithOracle("Animate Dead", []string{"enchantment"}, 2,
		"return target creature card from a graveyard to the battlefield.")
	newTestPermanent(gs.Seats[0], engine, 0, 0)
	got := h.WinConPursuit(gs, 0)
	if got < 0.25 || got > 0.35 {
		t.Errorf("graveyard piece WITH engine (weight 0.9): got %.4f want ~0.30", got)
	}
}

func TestWinConPursuit_ManaBonusGatedOnRatioFloor(t *testing.T) {
	// 1-of-5 pieces (ratio 0.2, below 0.5 mana-bonus gate) — even
	// with 10 lands, no mana lift.
	sp := wcpStrategy([]string{"A", "B", "C", "D", "E"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, []string{"A"}, 10)
	got := h.WinConPursuit(gs, 0)
	// Pure 1/5 = 0.20, no mana bonus.
	if got < 0.18 || got > 0.22 {
		t.Errorf("mana bonus fired below ratio floor: got %.4f want ~0.20", got)
	}
}

func TestWinConPursuit_ManaBonusGatedOnUntappedLands(t *testing.T) {
	// 2-of-3 in hand (ratio 0.667 above gate) but only 2 lands (below
	// the 3-untapped floor for mana bonus) → no lift.
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)
	gs := wcpGameWithHandLands(t, []string{"Piece A", "Piece B"}, 2)
	got := h.WinConPursuit(gs, 0)
	if got < 0.65 || got > 0.70 {
		t.Errorf("mana bonus fired below land floor (2 lands): got %.4f want ~0.667", got)
	}
}

// ---------------------------------------------------------------------
// cardHeuristic integration — pursuit-driven boost
// ---------------------------------------------------------------------

func TestCardHeuristic_HighPursuitBoostsTutor(t *testing.T) {
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, []string{"Piece A"})
	h := NewYggdrasilHat(sp, 0)
	h.confidenceThreshold = 0.75

	// 2-of-3 in hand + 4 lands → pursuit ≈ 0.767 (high).
	gs := wcpGameWithHandLands(t, []string{"Piece A", "Piece B"}, 4)
	tutor := gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
		"search your library for a card and put that card into your hand.")
	highScore := h.cardHeuristic(gs, 0, tutor)

	// Counterfactual: 0-of-3 → pursuit = 0 → no boost.
	gsLow := wcpGameWithHandLands(t, nil, 4)
	lowScore := h.cardHeuristic(gsLow, 0, tutor)

	if highScore <= lowScore {
		t.Errorf("high pursuit should boost tutor score; high=%.4f low=%.4f",
			highScore, lowScore)
	}
}

func TestCardHeuristic_LowPursuitNoBoost(t *testing.T) {
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C", "Piece D", "Piece E"}, nil)
	h := NewYggdrasilHat(sp, 0)
	h.confidenceThreshold = 0.75

	// 1-of-5 + few lands → pursuit ≈ 0.20 (well below 0.7 gate).
	gs := wcpGameWithHandLands(t, []string{"Piece A"}, 2)
	tutor := gyCardWithOracle("Demonic Tutor", []string{"sorcery"}, 2,
		"search your library for a card.")
	pursuitScore := h.cardHeuristic(gs, 0, tutor)

	// Baseline: 0-pursuit setup.
	gsBase := wcpGameWithHandLands(t, nil, 2)
	baseScore := h.cardHeuristic(gsBase, 0, tutor)

	// At low pursuit, no boost fires — scores should be ~equal.
	delta := pursuitScore - baseScore
	if delta > 0.05 {
		t.Errorf("low pursuit (~0.20) should not fire tutor boost; delta=%.4f", delta)
	}
}

// TestCardHeuristic_PursuitGateAtExact0_7: pin the strict >= 0.7 gate.
func TestCardHeuristic_PursuitGateBoundary(t *testing.T) {
	sp := wcpStrategy([]string{"Piece A", "Piece B", "Piece C"}, nil)
	h := NewYggdrasilHat(sp, 0)

	// 2-of-3 in hand + 4 lands → pursuit ≈ 0.767, well past 0.7.
	gs := wcpGameWithHandLands(t, []string{"Piece A", "Piece B"}, 4)
	got := h.WinConPursuit(gs, 0)
	if got < 0.7 {
		t.Errorf("2-of-3 + mana should cross 0.7 gate; got %.4f", got)
	}

	// 1-of-3 + 4 lands → pursuit = 0.333, below gate.
	gsLow := wcpGameWithHandLands(t, []string{"Piece A"}, 4)
	gotLow := h.WinConPursuit(gsLow, 0)
	if gotLow >= 0.7 {
		t.Errorf("1-of-3 + mana should NOT cross 0.7 gate; got %.4f", gotLow)
	}
}
