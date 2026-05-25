package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Pins the evasion + summoning-sick + hard-to-answer weighting added to
// ThreatExposure (scoreThreat). The dimension used to read the same value
// for "5/5 flying haster" vs "5/5 vanilla summoning-sick"; that conflation
// was the surveyed gap.

// fortyLifePair sets seats 0 and 1 to 40-life Commander defaults so the
// lethal-ratio math has stable denominators across these tests.
func fortyLifePair(gs *gameengine.GameState) {
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
		gs.Seats[i].StartingLife = 40
	}
}

// TestEffectiveOffensivePower_FlyingWeightsHigher: a 5-power flying
// creature should register more offensive pressure than a 5-power vanilla
// creature, because ground blockers can't apply.
func TestEffectiveOffensivePower_FlyingWeightsHigher(t *testing.T) {
	gs := newTestGame(t, 2)
	fortyLifePair(gs)

	flier := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Sky Demon", []string{"creature"}, 5, nil), 5, 5)
	addKeyword(flier, "flying")

	flyingPow := effectiveOffensivePower(gs, gs.Seats[1])

	// Replace flier with a vanilla creature of the same stats.
	gs.Seats[1].Battlefield = nil
	newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Ogre", []string{"creature"}, 5, nil), 5, 5)
	vanillaPow := effectiveOffensivePower(gs, gs.Seats[1])

	if flyingPow <= vanillaPow {
		t.Errorf("flying 5/5 (%.2f) should out-weight vanilla 5/5 (%.2f)",
			flyingPow, vanillaPow)
	}
	// Pin the 1.5x multiplier exactly so a retune is a deliberate edit.
	want := vanillaPow * 1.5
	if flyingPow != want {
		t.Errorf("flying multiplier: got %.2f, want %.2f (1.5x)", flyingPow, want)
	}
}

// TestEffectiveOffensivePower_SummoningSickDiscount: a creature that
// just hit the battlefield without haste shouldn't pressure us the same
// as one that can swing this turn — there's a turn of warning.
func TestEffectiveOffensivePower_SummoningSickDiscount(t *testing.T) {
	gs := newTestGame(t, 2)
	fortyLifePair(gs)

	sick := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Fresh Ogre", []string{"creature"}, 5, nil), 5, 5)
	sick.SummoningSick = true

	sickPow := effectiveOffensivePower(gs, gs.Seats[1])

	gs.Seats[1].Battlefield = nil
	hasted := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Haster", []string{"creature"}, 5, nil), 5, 5)
	hasted.SummoningSick = true
	addKeyword(hasted, "haste")

	hastedPow := effectiveOffensivePower(gs, gs.Seats[1])

	if sickPow >= hastedPow {
		t.Errorf("summoning-sick non-haste (%.2f) should weight lower than haste (%.2f)",
			sickPow, hastedPow)
	}
	if sickPow != hastedPow*0.6 {
		t.Errorf("summoning-sick discount: got %.2f, want %.2f (0.6x of %.2f)",
			sickPow, hastedPow*0.6, hastedPow)
	}
}

// TestEffectiveOffensivePower_EvasionStacks: flying + trample multiplies
// (1.5 * 1.25 = 1.875x). Pins the stacking behavior so a future single-
// multiplier rewrite trips the test.
func TestEffectiveOffensivePower_EvasionStacks(t *testing.T) {
	gs := newTestGame(t, 2)
	fortyLifePair(gs)

	dragon := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Skyshroud Dragon", []string{"creature"}, 6, nil), 4, 4)
	addKeyword(dragon, "flying")
	addKeyword(dragon, "trample")

	got := effectiveOffensivePower(gs, gs.Seats[1])
	want := 4.0 * 1.5 * 1.25
	if got != want {
		t.Errorf("flying+trample stacking: got %.4f, want %.4f", got, want)
	}
}

// TestHardToAnswerScore_GatesOnPower: a 1/1 with ward shouldn't move
// the dimension — the threat is "this kills me" not "this is annoying".
func TestHardToAnswerScore_GatesOnPower(t *testing.T) {
	gs := newTestGame(t, 2)
	fortyLifePair(gs)

	small := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Annoying Mage", []string{"creature"}, 2, nil), 1, 1)
	addKeyword(small, "ward")

	if h := hardToAnswerScore(gs, gs.Seats[1]); h != 0 {
		t.Errorf("1-power ward creature should score 0, got %.2f", h)
	}

	// Add a 3-power ward creature alongside — that one should register.
	big := newTestPermanent(gs.Seats[1],
		newTestCardMinimal("Real Threat", []string{"creature"}, 3, nil), 3, 3)
	addKeyword(big, "ward")
	if h := hardToAnswerScore(gs, gs.Seats[1]); h <= 0 {
		t.Errorf("3-power ward creature should score > 0, got %.2f", h)
	}
}

// TestHardToAnswerScore_SaturatesAtThree: 3+ hard-to-answer attackers
// should saturate the score at 1.0 — beyond that we're already in
// "draw an answer or lose" territory.
func TestHardToAnswerScore_SaturatesAtThree(t *testing.T) {
	gs := newTestGame(t, 2)
	fortyLifePair(gs)

	for i := 0; i < 5; i++ {
		p := newTestPermanent(gs.Seats[1],
			newTestCardMinimal("Indestructible Brute", []string{"creature"}, 4, nil), 4, 4)
		addKeyword(p, "indestructible")
	}
	if h := hardToAnswerScore(gs, gs.Seats[1]); h != 1.0 {
		t.Errorf("5 hard-to-answer brutes should saturate at 1.0, got %.2f", h)
	}
}

// TestScoreThreat_EvasiveBoardIsScarier: end-to-end — a 12-power
// all-flying board should produce a more negative ThreatExposure than
// an identical 12-power all-ground board at the same life total.
// This is the dimension-level read of the offensive-power upgrade.
func TestScoreThreat_EvasiveBoardIsScarier(t *testing.T) {
	build := func(flying bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		fortyLifePair(gs)
		// Wide enough margin that flying's 1.5x doesn't push past lethal
		// — we want to compare ratios, not saturate at -1.0.
		gs.Seats[0].Life = 30
		for i := 0; i < 3; i++ {
			p := newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Ogre", []string{"creature"}, 4, nil), 4, 4)
			if flying {
				addKeyword(p, "flying")
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	groundThreat := ev.scoreThreat(build(false), 0)
	flyingThreat := ev.scoreThreat(build(true), 0)

	if flyingThreat >= groundThreat {
		t.Errorf("flying board threat (%.3f) should be more negative than ground (%.3f)",
			flyingThreat, groundThreat)
	}
}

// TestScoreThreat_HardToAnswerCompounds: same 12-power board, but one
// version has indestructible attackers. The indestructible version
// should produce a strictly more negative score — we burn into specific
// answers instead of generic removal.
func TestScoreThreat_HardToAnswerCompounds(t *testing.T) {
	build := func(indestructible bool) *gameengine.GameState {
		gs := newTestGame(t, 2)
		fortyLifePair(gs)
		gs.Seats[0].Life = 30
		for i := 0; i < 3; i++ {
			p := newTestPermanent(gs.Seats[1],
				newTestCardMinimal("Brute", []string{"creature"}, 4, nil), 4, 4)
			if indestructible {
				addKeyword(p, "indestructible")
			}
		}
		return gs
	}

	ev := NewEvaluator(nil)
	plain := ev.scoreThreat(build(false), 0)
	hard := ev.scoreThreat(build(true), 0)

	if hard >= plain {
		t.Errorf("indestructible board (%.3f) should be more threatening than plain (%.3f)",
			hard, plain)
	}
}
