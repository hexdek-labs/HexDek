package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// soulWardenCard builds a Card whose reconstructed oracle text matches
// the Soul Warden lifegain pattern ("whenever … gain … life"). The hat
// package reads via OracleTextLower which walks card.AST.Abilities, so
// a single Static node with the canonical raw text drives the match.
func soulWardenCard() *gameengine.Card {
	ast := &gameast.CardAST{
		Name: "Soul Warden",
		Abilities: []gameast.Ability{
			&gameast.Static{Raw: "whenever another creature enters, you gain 1 life"},
		},
	}
	return newTestCardMinimal("Soul Warden", []string{"creature"}, 1, ast)
}

// Pins the LifeResource dimension upgrades: convex danger curve and
// lifegain-on-board recovery dampener.

// setSingleSeatLife is a tiny helper so each test reads cleanly.
func setSingleSeatLife(gs *gameengine.GameState, seat, life int) {
	gs.Seats[seat].Life = life
	gs.Seats[seat].StartingLife = 40
}

// TestScoreLife_ConvexBelow10: each life point lost below 10 should
// hurt more than the previous one. Specifically the delta from 10→5
// should be smaller than the delta from 5→1.
func TestScoreLife_ConvexBelow10(t *testing.T) {
	build := func(life int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, life)
		setSingleSeatLife(gs, 1, 40) // no opponent pressure
		return gs
	}

	ev := NewEvaluator(nil)
	at10 := ev.scoreLife(build(10), 0)
	at5 := ev.scoreLife(build(5), 0)
	at1 := ev.scoreLife(build(1), 0)

	d105 := at10 - at5 // positive = at5 is worse
	d51 := at5 - at1   // positive = at1 is worse

	if d105 <= 0 || d51 <= 0 {
		t.Fatalf("monotonicity broken: at10=%.3f at5=%.3f at1=%.3f", at10, at5, at1)
	}
	if d51 <= d105 {
		t.Errorf("convexity failed: delta(5→1)=%.3f should exceed delta(10→5)=%.3f",
			d51, d105)
	}
}

// TestScoreLife_ContinuousAt10Boundary: the quadratic penalty is 0
// exactly at life=10, so the score at life=10 should equal the
// pre-tune linear result (ratio - 0.5 with no quadratic add).
// Guards against accidental discontinuity at the band boundary.
func TestScoreLife_ContinuousAt10Boundary(t *testing.T) {
	gs := newTestGame(t, 2)
	setSingleSeatLife(gs, 0, 10)
	setSingleSeatLife(gs, 1, 40)

	ev := NewEvaluator(nil)
	got := ev.scoreLife(gs, 0)
	// At life=10: ratio=0.25, base = 0.25 - 0.5 = -0.25; danger=0, square=0.
	// No engines, no payoffs, no pressure. Expect exactly -0.25.
	if got != -0.25 {
		t.Errorf("life=10 should be exactly -0.25 (band boundary), got %.3f", got)
	}
}

// TestScoreLife_LifegainDampenerWhenLow: at low life, a Soul-Warden-
// style lifegain engine should soften the penalty by ~10%.
func TestScoreLife_LifegainDampenerWhenLow(t *testing.T) {
	build := func(withEngine bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 5)
		setSingleSeatLife(gs, 1, 40)
		if withEngine {
			newTestPermanent(gs.Seats[0], soulWardenCard(), 1, 1)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	without := ev.scoreLife(build(false), 0)
	with := ev.scoreLife(build(true), 0)

	if with <= without {
		t.Errorf("lifegain engine should soften the penalty: without=%.3f with=%.3f",
			without, with)
	}
	// Pin the 10% dampener exactly.
	want := without * 0.9
	if !floatNear(with, want, 1e-9) {
		t.Errorf("dampener: got %.4f, want %.4f (without * 0.9, without=%.4f)",
			with, want, without)
	}
}

// TestScoreLife_LifegainCapsAtThree: 4 engines shouldn't help more
// than 3 — the dampener caps so the dimension still tracks the
// underlying clock.
func TestScoreLife_LifegainCapsAtThree(t *testing.T) {
	build := func(engines int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 5)
		setSingleSeatLife(gs, 1, 40)
		for i := 0; i < engines; i++ {
			newTestPermanent(gs.Seats[0], soulWardenCard(), 1, 1)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	three := ev.scoreLife(build(3), 0)
	four := ev.scoreLife(build(4), 0)

	if three != four {
		t.Errorf("4 engines should match 3 (cap): three=%.4f four=%.4f", three, four)
	}
}

// TestScoreLife_LifelinkRequiresPower: a lifelink 1/1 doesn't recoup
// meaningfully — engines should only count when power ≥ 2. Lifelink
// utility-tokens (Hopeful Eidolon) shouldn't soften the danger penalty.
func TestScoreLife_LifelinkRequiresPower(t *testing.T) {
	build := func(power int) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 5)
		setSingleSeatLife(gs, 1, 40)
		c := newTestCardMinimal("Lifelinker", []string{"creature"}, 1, nil)
		p := newTestPermanent(gs.Seats[0], c, power, power)
		addKeyword(p, "lifelink")
		return gs
	}

	ev := NewEvaluator(nil)
	weak := ev.scoreLife(build(1), 0)
	strong := ev.scoreLife(build(3), 0)

	if weak == strong {
		t.Fatalf("expected lifelink power gating to matter: weak=%.4f strong=%.4f",
			weak, strong)
	}
	if strong <= weak {
		t.Errorf("3-power lifelink should soften more than 1-power: weak=%.4f strong=%.4f",
			weak, strong)
	}
}

// TestScoreLife_LifegainNoOpAtHighLife: at 40 life, base is positive,
// so engines shouldn't matter (they only dampen NEGATIVE base; they
// don't push positive scores higher).
func TestScoreLife_LifegainNoOpAtHighLife(t *testing.T) {
	build := func(withEngine bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		setSingleSeatLife(gs, 0, 40)
		setSingleSeatLife(gs, 1, 40)
		if withEngine {
			newTestPermanent(gs.Seats[0], soulWardenCard(), 1, 1)
		}
		return gs
	}

	ev := NewEvaluator(nil)
	without := ev.scoreLife(build(false), 0)
	with := ev.scoreLife(build(true), 0)

	if with != without {
		t.Errorf("at 40 life (positive base) engines should be no-op: without=%.4f with=%.4f",
			without, with)
	}
}

// TestScoreLife_ClampedAtNeg1: with very low life and strong opp
// pressure, the convex penalty + (no) pressure offset shouldn't drop
// below -1. Defends against the dimension going wildly out of band.
func TestScoreLife_ClampedAtNeg1(t *testing.T) {
	gs := newTestGame(t, 2)
	setSingleSeatLife(gs, 0, 1)
	setSingleSeatLife(gs, 1, 1) // no pressure component (opp also dying)

	ev := NewEvaluator(nil)
	got := ev.scoreLife(gs, 0)
	if got < -1 {
		t.Errorf("score should be clamped at -1, got %.4f", got)
	}
}

func floatNear(a, b, eps float64) bool {
	if a-b < eps && b-a < eps {
		return true
	}
	return false
}
