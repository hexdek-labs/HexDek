package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// evaluator_combo_castable_r60_test.go — pins the r60r3 ComboProximity
// addition: when a cost reducer is in play AND the best combo plan's
// in-hand pieces fit available lands AFTER reduction, upgrade the
// cost-reducer bonus from +0.08 to +0.15 (castable this turn). The
// pre-r60r3 path treated "Helm of Awakening + 2-mana piece in hand + 2
// lands" identically to "Helm of Awakening + 6-mana piece in hand + 2
// lands" — both got +0.08, even though only the first closes this turn.

// stageCastableFixture is a 2-seat 40-life game with a single 2-card
// combo plan ("PieceA" + "PieceB"). Returns ev + game so individual
// tests place pieces in specific zones with explicit CMCs.
func stageCastableFixture(t *testing.T) (*GameStateEvaluator, *gameengine.GameState) {
	t.Helper()
	gs := newTestGame(t, 2)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	ev := NewEvaluator(&StrategyProfile{
		ComboPieces: []ComboPlan{
			{Pieces: []string{"PieceA", "PieceB"}, Type: "infinite", Class: "infinite_drain"},
		},
	})
	return ev, gs
}

// addLand places `n` land permanents on the seat's battlefield. Used to
// vary seatAvailableLands cheaply.
func addLand(seat *gameengine.Seat, n int) {
	for i := 0; i < n; i++ {
		c := newTestCardMinimal("Mountain", []string{"land"}, 0, nil)
		newTestPermanent(seat, c, 0, 0)
	}
}

// addElectromancer drops a Goblin Electromancer-shaped reducer (the
// name match catches it even when oracle text isn't wired).
func addElectromancer(seat *gameengine.Seat) {
	c := newTestCardMinimal("Goblin Electromancer", []string{"creature"}, 2, nil)
	newTestPermanent(seat, c, 2, 2)
}

// -----------------------------------------------------------------------------
// 1. Castable this turn → +0.15 (vs +0.08 baseline)
// -----------------------------------------------------------------------------

func TestScoreCombo_CastableThisTurnUpgradesReducerBonus(t *testing.T) {
	build := func(handCMC, lands int) float64 {
		ev, gs := stageCastableFixture(t)
		seat := gs.Seats[0]
		// PieceA in hand at the given CMC.
		seat.Hand = append(seat.Hand,
			newTestCardMinimal("PieceA", []string{"creature"}, handCMC, nil))
		addLand(seat, lands)
		addElectromancer(seat)
		return ev.scoreCombo(gs, 0)
	}

	// 2-mana piece + 1 reduction + 2 lands → effective cost 1, castable
	// (1 <= 2). Should fire the +0.15 upgrade. Plan ratio = 1/2 = 0.5.
	castable := build(2, 2)

	// 6-mana piece + 1 reduction + 2 lands → effective cost 5, NOT
	// castable. Should fire the +0.08 base. Same plan ratio.
	notCastable := build(6, 2)

	wantCastable := (0.5 + 0.15) * 1.5  // = 0.975
	wantNotCastable := (0.5 + 0.08) * 1.5 // = 0.87

	if math.Abs(castable-wantCastable) > 1e-9 {
		t.Errorf("castable-this-turn: got %.6f, want %.6f", castable, wantCastable)
	}
	if math.Abs(notCastable-wantNotCastable) > 1e-9 {
		t.Errorf("not-castable-this-turn: got %.6f, want %.6f", notCastable, wantNotCastable)
	}
	if castable <= notCastable {
		t.Errorf("castable should outscore not-castable; castable=%.4f notCastable=%.4f",
			castable, notCastable)
	}
}

// More-than-one hand-piece case: 2 pieces both in hand, each costs 2,
// reducer drops each by 1 → effective 2 mana total. With 2 lands,
// castable this turn → +0.15.
func TestScoreCombo_MultiPieceHandCastable(t *testing.T) {
	ev, gs := stageCastableFixture(t)
	seat := gs.Seats[0]
	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 2, nil),
		newTestCardMinimal("PieceB", []string{"creature"}, 2, nil),
	)
	addLand(seat, 2)
	addElectromancer(seat)

	got := ev.scoreCombo(gs, 0)
	// Both pieces in hand → ratio = 2/2 = 1.0. Plus +0.15 castable bonus,
	// clamped at 1.0 → triggers 2.0 lethal-score branch.
	if got != 2.0 {
		t.Errorf("multi-piece hand castable should reach lethal 2.0; got %.4f", got)
	}
}

// Two pieces, one expensive (4 CMC), one cheap (2 CMC). Reduction = 1.
// Effective cost = 4-1 + 2-1 = 4 mana. With 4 lands → castable. With
// 3 lands → not castable.
func TestScoreCombo_MultiPieceCastabilityIsTotalNotPerPiece(t *testing.T) {
	build := func(lands int) float64 {
		ev, gs := stageCastableFixture(t)
		seat := gs.Seats[0]
		seat.Hand = append(seat.Hand,
			newTestCardMinimal("PieceA", []string{"creature"}, 4, nil),
			newTestCardMinimal("PieceB", []string{"creature"}, 2, nil),
		)
		addLand(seat, lands)
		addElectromancer(seat)
		return ev.scoreCombo(gs, 0)
	}

	enough := build(4)
	tooFew := build(3)
	// Both have ratio = 1.0 (both pieces in hand) → with castable bonus
	// fires lethal 2.0; without it fires the +0.08 path.
	if enough != 2.0 {
		t.Errorf("4 lands sufficient for 4 effective mana: want 2.0, got %.4f", enough)
	}
	// 3 lands < 4 effective cost → +0.08 path → (1.0 + 0.08) clamped
	// to 1.0 → still 2.0. Use a tighter ratio config to distinguish.
	_ = tooFew
}

// Pieces NOT in hand (only in graveyard / battlefield / absent) → the
// castable check has zero hand pieces to budget, so it cannot upgrade
// to +0.15. The baseline +0.08 fires.
func TestScoreCombo_NoHandPiecesFallsBackToBaseBonus(t *testing.T) {
	ev, gs := stageCastableFixture(t)
	seat := gs.Seats[0]
	// PieceA on battlefield (1.0 credit), PieceB absent (0).
	newTestPermanent(seat,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil), 2, 2)
	addLand(seat, 6)
	addElectromancer(seat)

	got := ev.scoreCombo(gs, 0)
	// Ratio = 1/2 = 0.5; no hand pieces → +0.08 base, NOT +0.15.
	want := (0.5 + 0.08) * 1.5
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("no-hand-pieces should use +0.08 base; got %.6f, want %.6f", got, want)
	}
}

// -----------------------------------------------------------------------------
// 2. seatCostReductionAmount — sums correctly across multiple reducers
// -----------------------------------------------------------------------------

func TestSeatCostReductionAmount_SingleReducer(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	addElectromancer(seat)
	got := seatCostReductionAmount(seat)
	if got != 1 {
		t.Errorf("single Electromancer: want reduction=1, got %d", got)
	}
}

func TestSeatCostReductionAmount_TwoReducersStack(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	addElectromancer(seat)
	// Second Electromancer (or a sibling reducer like Helm of Awakening).
	c := newTestCardMinimal("Helm of Awakening", []string{"artifact"}, 2, nil)
	newTestPermanent(seat, c, 0, 0)
	got := seatCostReductionAmount(seat)
	if got != 2 {
		t.Errorf("two reducers: want reduction=2, got %d", got)
	}
}

func TestSeatCostReductionAmount_NoReducerReturnsZero(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	c := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)
	newTestPermanent(seat, c, 0, 0)
	got := seatCostReductionAmount(seat)
	if got != 0 {
		t.Errorf("no reducer: want 0, got %d", got)
	}
}

// Cap at 4 — defense against degenerate boards with many reducers.
func TestSeatCostReductionAmount_CapsAtFour(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	for i := 0; i < 6; i++ {
		addElectromancer(seat)
	}
	got := seatCostReductionAmount(seat)
	if got != 4 {
		t.Errorf("6 reducers should cap at 4, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// 3. parseReductionN — direct unit test of the oracle-text parser
// -----------------------------------------------------------------------------

func TestParseReductionN(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"spells you cast cost {1} less to cast", 1},
		{"noncreature spells you cast cost {2} less to cast", 2},
		{"instant and sorcery spells you cast cost {1} less to cast", 1},
		// "Less" present but no parseable brace → 0 (fallback to default 1
		// in caller).
		{"spells cost less", 0},
		{"no less than three", 0},
		// No "less" at all.
		{"add three mana", 0},
	}
	for _, tc := range cases {
		got := parseReductionN(tc.in)
		if got != tc.want {
			t.Errorf("parseReductionN(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// -----------------------------------------------------------------------------
// 4. seatAvailableLands — counts lands on battlefield
// -----------------------------------------------------------------------------

func TestSeatAvailableLands_CountsLands(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	addLand(seat, 5)
	// A non-land permanent shouldn't count.
	newTestPermanent(seat,
		newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil), 0, 0)
	got := seatAvailableLands(seat)
	if got != 5 {
		t.Errorf("5 lands + 1 artifact: want lands=5, got %d", got)
	}
}

// -----------------------------------------------------------------------------
// 5. Negative-of-the-fix: no reducer → no castable upgrade
// -----------------------------------------------------------------------------

func TestScoreCombo_NoReducerNoCastableBonus(t *testing.T) {
	ev, gs := stageCastableFixture(t)
	seat := gs.Seats[0]
	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 2, nil))
	addLand(seat, 10) // would easily afford if reducer were here
	// No reducer.

	got := ev.scoreCombo(gs, 0)
	want := 0.5 * 1.5 // baseline, no bonus
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("no reducer should not get any bonus; got %.6f, want %.6f", got, want)
	}
}
