package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Pins the R60 LifeResource additions: painland mana-base tax + pay-life
// tutors in hand counting as life payoffs.

// cityOfBrassCard: canonical "damage to you" painland shape.
func cityOfBrassCard() *gameengine.Card {
	ast := &gameast.CardAST{
		Name: "City of Brass",
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "whenever city of brass becomes tapped, it deals 1 damage to you"},
		},
	}
	return newTestCardMinimal("City of Brass", []string{"land"}, 0, ast)
}

// manaConfluenceCard: "pay 1 life" + "add" activated-cost shape.
func manaConfluenceCard() *gameengine.Card {
	ast := &gameast.CardAST{
		Name: "Mana Confluence",
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "tap, pay 1 life: add one mana of any color"},
		},
	}
	return newTestCardMinimal("Mana Confluence", []string{"land"}, 0, ast)
}

// plainsLandCard: a vanilla basic — should NOT trip the painland detector.
func plainsLandCard() *gameengine.Card {
	return newTestCardMinimal("Plains", []string{"land"}, 0, nil)
}

// vampiricTutorCard: canonical pay-life-tutor shape ("pay 2 life: search").
func vampiricTutorCard() *gameengine.Card {
	ast := &gameast.CardAST{
		Name: "Vampiric Tutor",
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "search your library for a card, then shuffle and put that card on top. you lose 2 life. pay 2 life as you cast this spell."},
		},
	}
	return newTestCardMinimal("Vampiric Tutor", []string{"instant"}, 1, ast)
}

// TestScoreLife_PainlandAmplifiesDanger: a painland on the battlefield
// at low life should make the danger penalty stronger. Turn=0 isolates
// the low-life amplifier from the R60 compounding-turn erosion (which
// has its own test below).
func TestScoreLife_PainlandAmplifiesDanger(t *testing.T) {
	build := func(painlands int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Turn = 0
		setSingleSeatLife(gs, 0, 5)
		setSingleSeatLife(gs, 1, 40)
		for i := 0; i < painlands; i++ {
			newTestPermanent(gs.Seats[0], cityOfBrassCard(), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	zero := ev.scoreLife(build(0), 0)
	one := ev.scoreLife(build(1), 0)

	if one >= zero {
		t.Errorf("one painland should worsen the score at low life: zero=%.4f one=%.4f", zero, one)
	}
	// Pin the 5% amplifier exactly: one painland multiplies negative base by 1.05.
	want := zero * 1.05
	if !floatNear(one, want, 1e-9) {
		t.Errorf("painland amplifier: got %.4f, want %.4f (zero * 1.05, zero=%.4f)", one, want, zero)
	}
}

// TestScoreLife_PainlandManaConfluenceShape: Mana Confluence ("pay 1 life:
// add") matches the alternate activated-cost shape (not "damage to you").
func TestScoreLife_PainlandManaConfluenceShape(t *testing.T) {
	build := func(withConfluence bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 5)
		setSingleSeatLife(gs, 1, 40)
		if withConfluence {
			newTestPermanent(gs.Seats[0], manaConfluenceCard(), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	without := ev.scoreLife(build(false), 0)
	with := ev.scoreLife(build(true), 0)

	if with >= without {
		t.Errorf("Mana Confluence should amplify danger: without=%.4f with=%.4f", without, with)
	}
}

// TestScoreLife_PainlandCapsAtFive: 6 painlands shouldn't amplify more
// than 5 — the multiplier caps to avoid runaway penalties.
func TestScoreLife_PainlandCapsAtFive(t *testing.T) {
	build := func(painlands int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 5)
		setSingleSeatLife(gs, 1, 40)
		for i := 0; i < painlands; i++ {
			newTestPermanent(gs.Seats[0], cityOfBrassCard(), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	five := ev.scoreLife(build(5), 0)
	six := ev.scoreLife(build(6), 0)

	if five != six {
		t.Errorf("6 painlands should match 5 (cap): five=%.4f six=%.4f", five, six)
	}
}

// TestScoreLife_BasicLandsIgnored: a battlefield of plain Plains should
// not trip the painland detector.
func TestScoreLife_BasicLandsIgnored(t *testing.T) {
	build := func(withLands bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 5)
		setSingleSeatLife(gs, 1, 40)
		if withLands {
			for i := 0; i < 5; i++ {
				newTestPermanent(gs.Seats[0], plainsLandCard(), 0, 0)
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	without := ev.scoreLife(build(false), 0)
	with := ev.scoreLife(build(true), 0)

	if with != without {
		t.Errorf("basic lands should not trip painland detector: without=%.4f with=%.4f",
			without, with)
	}
}

// TestScoreLife_PainlandAmpNoOpAtHighLife: at 40 life base is positive,
// so the low-life *amplifier* is a no-op (it only steepens the danger
// curve). Turn=0 isolates from the R60 compounding-turn erosion.
func TestScoreLife_PainlandAmpNoOpAtHighLife(t *testing.T) {
	build := func(painlands int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Turn = 0
		setSingleSeatLife(gs, 0, 40)
		setSingleSeatLife(gs, 1, 40)
		for i := 0; i < painlands; i++ {
			newTestPermanent(gs.Seats[0], cityOfBrassCard(), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	zero := ev.scoreLife(build(0), 0)
	three := ev.scoreLife(build(3), 0)

	if zero != three {
		t.Errorf("painlands at full life should be no-op for amplifier with turn=0: zero=%.4f three=%.4f", zero, three)
	}
}

// TestScoreLife_PainlandErosionGrowsWithTurn: the compounding turn-based
// erosion penalty fires at healthy life and gets stronger as the game
// drags on. Painland on turn 30 should hurt more than painland on turn 5.
func TestScoreLife_PainlandErosionGrowsWithTurn(t *testing.T) {
	build := func(turn int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Turn = turn
		setSingleSeatLife(gs, 0, 40)
		setSingleSeatLife(gs, 1, 40)
		newTestPermanent(gs.Seats[0], cityOfBrassCard(), 0, 0)
		return gs
	}

	ev := NewEvaluator(nil)
	early := ev.scoreLife(build(5), 0)
	late := ev.scoreLife(build(30), 0)

	if late >= early {
		t.Errorf("erosion should compound: early(turn=5)=%.4f late(turn=30)=%.4f", early, late)
	}
}

// TestScoreLife_PainlandErosionExactMath: pin the formula exactly so
// future tuning is intentional. 4 painlands on turn 20 = 4*20/400 = 0.20,
// clamped at 0.15. Compare to a baseline run with turn=0 painlands
// (no erosion fires) at the same life.
func TestScoreLife_PainlandErosionExactMath(t *testing.T) {
	build := func(painlands, turn int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Turn = turn
		setSingleSeatLife(gs, 0, 40)
		setSingleSeatLife(gs, 1, 40)
		for i := 0; i < painlands; i++ {
			newTestPermanent(gs.Seats[0], cityOfBrassCard(), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	baseline := ev.scoreLife(build(0, 20), 0)

	// 2 painlands * 20 turns / 400 = 0.10 (uncapped).
	two := ev.scoreLife(build(2, 20), 0)
	if !floatNear(two, baseline-0.10, 1e-9) {
		t.Errorf("2 painlands @ turn 20: got %.4f, want %.4f (baseline - 0.10)", two, baseline-0.10)
	}

	// 4 painlands * 20 turns / 400 = 0.20 -> clamped at 0.15.
	four := ev.scoreLife(build(4, 20), 0)
	if !floatNear(four, baseline-0.15, 1e-9) {
		t.Errorf("4 painlands @ turn 20 (clamped): got %.4f, want %.4f (baseline - 0.15)", four, baseline-0.15)
	}
}

// TestScoreLife_PainlandErosionClampsAtFifteen: the 0.15 ceiling caps
// erosion so it can't dominate the dimension at very late turns / many
// painlands. 5 painlands * 30 turns / 400 = 0.375 -> 0.15; 5 painlands
// * 60 turns / 400 = 0.75 -> 0.15.
func TestScoreLife_PainlandErosionClampsAtFifteen(t *testing.T) {
	build := func(turn int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Turn = turn
		setSingleSeatLife(gs, 0, 40)
		setSingleSeatLife(gs, 1, 40)
		for i := 0; i < 5; i++ {
			newTestPermanent(gs.Seats[0], cityOfBrassCard(), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	turn30 := ev.scoreLife(build(30), 0)
	turn60 := ev.scoreLife(build(60), 0)

	if turn30 != turn60 {
		t.Errorf("erosion should clamp at 0.15: turn30=%.4f turn60=%.4f", turn30, turn60)
	}
}

// TestScoreLife_PainlandErosionTurnZeroNoOp: with gs.Turn=0, erosion is
// gated off entirely. Guards the early-game / test-harness path.
func TestScoreLife_PainlandErosionTurnZeroNoOp(t *testing.T) {
	build := func(painlands int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Turn = 0
		setSingleSeatLife(gs, 0, 40)
		setSingleSeatLife(gs, 1, 40)
		for i := 0; i < painlands; i++ {
			newTestPermanent(gs.Seats[0], cityOfBrassCard(), 0, 0)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	zero := ev.scoreLife(build(0), 0)
	five := ev.scoreLife(build(5), 0)
	if zero != five {
		t.Errorf("erosion should be no-op at turn=0: zero=%.4f five=%.4f", zero, five)
	}
}

// TestScoreLife_PainlandErosionBasicsImmune: a battlefield of Plains
// should not trigger erosion either — only painlands accrue this debt.
func TestScoreLife_PainlandErosionBasicsImmune(t *testing.T) {
	build := func(withLands bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		gs.Turn = 30
		setSingleSeatLife(gs, 0, 40)
		setSingleSeatLife(gs, 1, 40)
		if withLands {
			for i := 0; i < 5; i++ {
				newTestPermanent(gs.Seats[0], plainsLandCard(), 0, 0)
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	without := ev.scoreLife(build(false), 0)
	with := ev.scoreLife(build(true), 0)
	if with != without {
		t.Errorf("basic lands should not erode: without=%.4f with=%.4f", without, with)
	}
}

// TestScoreLife_PayLifeTutorInHandCountsAsPayoff: Vampiric Tutor in hand
// at comfortable life should soften the low-life penalty, just like a
// battlefield life-payoff would.
func TestScoreLife_PayLifeTutorInHandCountsAsPayoff(t *testing.T) {
	build := func(withTutor bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 8)
		setSingleSeatLife(gs, 1, 40)
		if withTutor {
			gs.Seats[0].Hand = append(gs.Seats[0].Hand, vampiricTutorCard())
		}
		return gs
	}

	ev := NewEvaluator(nil)
	without := ev.scoreLife(build(false), 0)
	with := ev.scoreLife(build(true), 0)

	if with <= without {
		t.Errorf("pay-life tutor in hand should soften the penalty: without=%.4f with=%.4f",
			without, with)
	}
}

// TestScoreLife_PayLifeTutorIgnoredWhenLifeTooLow: with only 3 life, a
// pay-life tutor in hand is functionally unusable (2-life cost back-to-
// back blows the seat out), so it should NOT count as a payoff.
func TestScoreLife_PayLifeTutorIgnoredWhenLifeTooLow(t *testing.T) {
	build := func(withTutor bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 3)
		setSingleSeatLife(gs, 1, 40)
		if withTutor {
			gs.Seats[0].Hand = append(gs.Seats[0].Hand, vampiricTutorCard())
		}
		return gs
	}

	ev := NewEvaluator(nil)
	without := ev.scoreLife(build(false), 0)
	with := ev.scoreLife(build(true), 0)

	if with != without {
		t.Errorf("pay-life tutor should be ignored when life<4: without=%.4f with=%.4f",
			without, with)
	}
}
