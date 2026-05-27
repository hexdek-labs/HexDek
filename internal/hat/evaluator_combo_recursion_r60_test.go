package hat

import (
	"math"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// evaluator_combo_recursion_r60_test.go — pins the r60r2 ComboProximity
// additions: graveyard-recursion engine upgrades graveyard piece credit
// from 0.5 → 0.9, and cost-reducer on battlefield adds +0.08 to bestRatio
// when meaningful progress (bestRatio > 0.4) exists.
//
// Pre-r60r2 a Worldgorger Dragon in graveyard with Animate Dead already
// in play scored identically to a Worldgorger Dragon in graveyard with
// nothing in play — the flat 0.5 graveyard weight didn't read board
// state. And a Goblin Electromancer in play with a 2-piece combo
// half-assembled contributed 0.0 to ComboProximity, even though the
// combo became castable a turn earlier.

// makeCostReducerCard builds a Goblin Electromancer-shaped card so
// seatHasComboCostReducer's oracle-text path fires.
func makeCostReducerCard(name string) *gameengine.Card {
	c := newTestCardMinimal(name, []string{"creature"}, 2, nil)
	// Override AST oracle text via the helper's standard mechanism:
	// the helper attaches an empty AST, so we modify the card's Name
	// + leave oracle to come from gameengine.OracleTextLower's
	// fallback path. To make the substring scanner pick up the
	// reducer pattern, wrap with a card that has an oracle-bearing
	// AST. The simplest test path: use the name match (Goblin
	// Electromancer / Birgi / etc.) which the helper falls back to.
	return c
}

// stageRecursionFixture is a 2-seat 40-life game seeded with a 2-card
// combo plan. Returns evaluator + game so individual tests place pieces.
func stageRecursionFixture(t *testing.T) (*GameStateEvaluator, *gameengine.GameState) {
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

// makeRecursionEnginePerm builds an Underworld Breach-shaped permanent
// (oracle text contains "graveyard" + "may cast") so the engine detector
// fires via the oracle-text path. We use a name that the name-anchor
// fallback also catches so the test passes regardless of which path
// fires first.
func makeRecursionEnginePerm(seat *gameengine.Seat) *gameengine.Permanent {
	c := newTestCardMinimal("Underworld Breach", []string{"enchantment"}, 2, nil)
	return newTestPermanent(seat, c, 0, 0)
}

// -----------------------------------------------------------------------------
// 1. Graveyard piece + recursion engine → upgraded credit (0.5 → 0.9)
// -----------------------------------------------------------------------------

func TestScoreCombo_RecursionEngineUpgradesGraveyardCredit(t *testing.T) {
	build := func(withEngine bool) float64 {
		ev, gs := stageRecursionFixture(t)
		seat := gs.Seats[0]
		// PieceA in graveyard, PieceB absent.
		seat.Graveyard = append(seat.Graveyard,
			newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))
		if withEngine {
			makeRecursionEnginePerm(seat)
		}
		return ev.scoreCombo(gs, 0)
	}

	without := build(false)
	with := build(true)

	if with <= without {
		t.Errorf("recursion engine in play should upgrade graveyard credit; without=%.4f with=%.4f", without, with)
	}
	// Exact pin: pre-fix value = 0.5 / 2 × 1.5 = 0.375.
	if math.Abs(without-0.375) > 1e-9 {
		t.Errorf("without-engine baseline drifted: got %.6f, want 0.375", without)
	}
	// With engine: 0.9 / 2 × 1.5 = 0.675.
	if math.Abs(with-0.675) > 1e-9 {
		t.Errorf("with-engine score: got %.6f, want 0.675 (0.9/2 × 1.5)", with)
	}
}

// Both pieces in graveyard + recursion engine should approach (but not
// reach) the lethal-combo 2.0 threshold — 0.9 + 0.9 = 1.8 / 2 = 0.9 × 1.5
// = 1.35. Confirms the engine doesn't promote graveyard pieces to full
// hand-credit (which would trigger the 2.0 = "ready to win" branch).
func TestScoreCombo_BothPiecesInGraveyardWithEngineDoesNotReachLethal(t *testing.T) {
	ev, gs := stageRecursionFixture(t)
	seat := gs.Seats[0]
	seat.Graveyard = append(seat.Graveyard,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil),
		newTestCardMinimal("PieceB", []string{"creature"}, 3, nil),
	)
	makeRecursionEnginePerm(seat)

	got := ev.scoreCombo(gs, 0)
	want := (0.9 + 0.9) / 2.0 * 1.5 // = 1.35
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("both-in-grave with engine: got %.6f, want %.6f", got, want)
	}
	if got >= 2.0 {
		t.Errorf("graveyard-only with engine should NOT trigger 2.0 (only hand+battlefield should); got %.4f", got)
	}
}

// Negative-of-the-fix: a recursion engine on an OPPONENT's battlefield
// doesn't help us — the upgrade should only fire when WE control the
// engine.
func TestScoreCombo_OpponentRecursionDoesNotUpgrade(t *testing.T) {
	ev, gs := stageRecursionFixture(t)
	seat := gs.Seats[0]
	seat.Graveyard = append(seat.Graveyard,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))
	// Opponent has the recursion engine.
	makeRecursionEnginePerm(gs.Seats[1])

	got := ev.scoreCombo(gs, 0)
	want := 0.5 / 2.0 * 1.5 // = 0.375 (no upgrade)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("opponent-side engine leaked into our score: got %.6f, want %.6f", got, want)
	}
}

// -----------------------------------------------------------------------------
// 2. Cost reducer on battlefield → +0.08 when bestRatio > 0.4
// -----------------------------------------------------------------------------

func TestScoreCombo_CostReducerAddsBonusWhenProgressMeaningful(t *testing.T) {
	build := func(withReducer bool) float64 {
		ev, gs := stageRecursionFixture(t)
		seat := gs.Seats[0]
		// One piece in hand → bestRatio = 1/2 = 0.5 (above the 0.4 gate)
		seat.Hand = append(seat.Hand,
			newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))
		if withReducer {
			c := newTestCardMinimal("Goblin Electromancer", []string{"creature"}, 2, nil)
			newTestPermanent(seat, c, 2, 2)
		}
		return ev.scoreCombo(gs, 0)
	}

	without := build(false)
	with := build(true)

	if with <= without {
		t.Errorf("cost reducer should add bonus to mid-progress combo; without=%.4f with=%.4f", without, with)
	}
	// Pre-fix: 1/2 × 1.5 = 0.75. With reducer: (0.5 + 0.08) × 1.5 = 0.87.
	wantWithout := 0.5 * 1.5
	wantWith := (0.5 + 0.08) * 1.5
	if math.Abs(without-wantWithout) > 1e-9 {
		t.Errorf("baseline drifted: got %.6f, want %.6f", without, wantWithout)
	}
	if math.Abs(with-wantWith) > 1e-9 {
		t.Errorf("with-reducer: got %.6f, want %.6f", with, wantWith)
	}
}

// Negative-of-the-fix: cost reducer with NO combo progress (bestRatio
// at or below 0.4) should NOT fire the bonus — a reducer doesn't help
// when we have nothing to cast yet.
func TestScoreCombo_CostReducerDoesNotFireBelowProgressGate(t *testing.T) {
	ev, gs := stageRecursionFixture(t)
	seat := gs.Seats[0]
	// No pieces in any zone → bestRatio = 0.
	c := newTestCardMinimal("Goblin Electromancer", []string{"creature"}, 2, nil)
	newTestPermanent(seat, c, 2, 2)

	got := ev.scoreCombo(gs, 0)
	if got != 0 {
		t.Errorf("cost reducer with no combo progress should yield 0; got %.6f", got)
	}
}

// Negative-of-the-fix: opponent-side cost reducer doesn't help us.
func TestScoreCombo_OpponentCostReducerNoLeak(t *testing.T) {
	ev, gs := stageRecursionFixture(t)
	seat := gs.Seats[0]
	seat.Hand = append(seat.Hand,
		newTestCardMinimal("PieceA", []string{"creature"}, 3, nil))
	// Opponent has the reducer.
	c := newTestCardMinimal("Goblin Electromancer", []string{"creature"}, 2, nil)
	newTestPermanent(gs.Seats[1], c, 2, 2)

	got := ev.scoreCombo(gs, 0)
	want := 0.5 * 1.5 // no bonus
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("opponent reducer leaked: got %.6f, want %.6f", got, want)
	}
}

// -----------------------------------------------------------------------------
// Helper-level pins — direct exercise of the two new detectors so a
// regression in their oracle-text or name-anchor branches is caught
// even if the scoreCombo integration test passes.
// -----------------------------------------------------------------------------

func TestSeatHasComboGraveyardRecursion_PositiveCases(t *testing.T) {
	cases := []string{
		"Underworld Breach",
		"Yawgmoth's Will",
		"Past in Flames",
		"Snapcaster Mage",
	}
	for _, name := range cases {
		gs := newTestGame(t, 2)
		seat := gs.Seats[0]
		c := newTestCardMinimal(name, []string{"creature"}, 2, nil)
		newTestPermanent(seat, c, 0, 0)
		if !seatHasComboGraveyardRecursion(seat) {
			t.Errorf("%q should be detected as a graveyard recursion engine", name)
		}
	}
}

func TestSeatHasComboGraveyardRecursion_NegativeCases(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	// An unrelated permanent should not fire the detector.
	c := newTestCardMinimal("Sol Ring", []string{"artifact"}, 1, nil)
	newTestPermanent(seat, c, 0, 0)
	if seatHasComboGraveyardRecursion(seat) {
		t.Error("Sol Ring should not be detected as graveyard recursion")
	}
}

func TestSeatHasComboCostReducer_PositiveCases(t *testing.T) {
	cases := []string{
		"Goblin Electromancer",
		"Helm of Awakening",
		"Birgi, God of Storytelling",
		"Jhoira, Weatherlight Captain",
	}
	for _, name := range cases {
		gs := newTestGame(t, 2)
		seat := gs.Seats[0]
		c := newTestCardMinimal(name, []string{"creature"}, 2, nil)
		newTestPermanent(seat, c, 2, 2)
		if !seatHasComboCostReducer(seat) {
			t.Errorf("%q should be detected as a cost reducer", name)
		}
	}
}

func TestSeatHasComboCostReducer_NegativeCases(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	c := newTestCardMinimal("Lightning Bolt", []string{"instant"}, 1, nil)
	newTestPermanent(seat, c, 0, 0)
	if seatHasComboCostReducer(seat) {
		t.Error("Lightning Bolt should not be detected as cost reducer")
	}
}
