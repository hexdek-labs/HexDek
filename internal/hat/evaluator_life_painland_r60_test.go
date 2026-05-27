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
// at low life should make the danger penalty stronger.
func TestScoreLife_PainlandAmplifiesDanger(t *testing.T) {
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

// TestScoreLife_PainlandNoOpAtHighLife: at 40 life base is positive, so
// painlands don't amplify anything (they only steepen the danger curve).
func TestScoreLife_PainlandNoOpAtHighLife(t *testing.T) {
	build := func(painlands int) *gameengine.GameState {
		gs := newTestGame(t, 2)
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
		t.Errorf("painlands at full life should be no-op: zero=%.4f three=%.4f", zero, three)
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
