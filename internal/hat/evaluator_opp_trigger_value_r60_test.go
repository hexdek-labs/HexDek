package hat

import (
	"math"
	"testing"
)

// r60 follow-up: scoreCards now credits the Rhystic Study / Smothering
// Tithe class — passive triggers on an opponent action that produce a
// free resource (card / treasure / token / mana) for the controller.
// The new term scales with living-opponent count (0.4 per engine per
// opp), capturing the "cEDH Rhystic vs duel Rhystic" asymmetry that
// the flat draw-engine rate-class can't express.
//
// The opp-trigger-value term stacks on top of drawEngineCredit when
// the engine is also a draw engine (Rhystic, Mystic Remora, Esper
// Sentinel). Treasure / mana / token engines that AREN'T draw engines
// (Smothering Tithe, Trouble in Pairs) get nonzero CA credit here for
// the first time.

// TestOppTriggerValue_RhysticStudyCounts pins that Rhystic Study
// counts as an opp-trigger value engine.
func TestOppTriggerValue_RhysticStudyCounts(t *testing.T) {
	rhystic := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
		"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], rhystic, 0, 0)
	if !isOpponentTriggeredValueEngine(p) {
		t.Errorf("Rhystic Study should register as opp-trigger value engine")
	}
}

// TestOppTriggerValue_SmotheringTitheCounts pins that Smothering Tithe
// counts even though it produces treasure, not card draw.
func TestOppTriggerValue_SmotheringTitheCounts(t *testing.T) {
	tithe := cardWithStaticText("Smothering Tithe", []string{"enchantment"}, 4,
		"whenever an opponent draws a card, unless that player pays {2}, you create a tapped treasure token.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], tithe, 0, 0)
	if !isOpponentTriggeredValueEngine(p) {
		t.Errorf("Smothering Tithe should register (treasure-on-opp-draw)")
	}
}

// TestOppTriggerValue_TroubleInPairsCounts pins the dual-resource
// case: opp action produces both a card and a treasure.
func TestOppTriggerValue_TroubleInPairsCounts(t *testing.T) {
	tip := cardWithStaticText("Trouble in Pairs", []string{"enchantment"}, 4,
		"whenever an opponent draws a card except the first one they draw in each of their draw steps, you draw a card and create a treasure token.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], tip, 0, 0)
	if !isOpponentTriggeredValueEngine(p) {
		t.Errorf("Trouble in Pairs should register (draw + treasure on opp draw)")
	}
}

// TestOppTriggerValue_MysticRemoraCounts pins Mystic Remora.
func TestOppTriggerValue_MysticRemoraCounts(t *testing.T) {
	remora := cardWithStaticText("Mystic Remora", []string{"enchantment"}, 1,
		"cumulative upkeep {1}. whenever an opponent casts a noncreature spell, you may draw a card unless that player pays {4}.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], remora, 0, 0)
	if !isOpponentTriggeredValueEngine(p) {
		t.Errorf("Mystic Remora should register as opp-trigger value engine")
	}
}

// TestOppTriggerValue_PhyrexianArenaDoesNotCount pins the gate: own-
// upkeep value engines do NOT register, because the trigger isn't
// scoped to an opp action.
func TestOppTriggerValue_PhyrexianArenaDoesNotCount(t *testing.T) {
	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], arena, 0, 0)
	if isOpponentTriggeredValueEngine(p) {
		t.Errorf("Phyrexian Arena fires on your upkeep, not opp action — should NOT register")
	}
}

// TestOppTriggerValue_HowlingMineDoesNotCount pins the symmetric-
// engine gate: each-player effects are not opp-asymmetric and are
// already discounted in drawEngineRate.
func TestOppTriggerValue_HowlingMineDoesNotCount(t *testing.T) {
	mine := cardWithStaticText("Howling Mine", []string{"artifact"}, 2,
		"at the beginning of each player's draw step, if howling mine is untapped, that player draws an additional card")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], mine, 0, 0)
	if isOpponentTriggeredValueEngine(p) {
		t.Errorf("Howling Mine is symmetric, not opp-asymmetric — should NOT register")
	}
}

// TestOppTriggerValue_UnderworldDreamsDoesNotCount pins the resource
// gate: punishing opp draws via damage doesn't give controller a
// resource (that's ThreatExposure, not CardAdvantage).
func TestOppTriggerValue_UnderworldDreamsDoesNotCount(t *testing.T) {
	dreams := cardWithStaticText("Underworld Dreams", []string{"enchantment"}, 3,
		"whenever an opponent draws a card, underworld dreams deals 1 damage to that player.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], dreams, 0, 0)
	if isOpponentTriggeredValueEngine(p) {
		t.Errorf("Underworld Dreams deals damage but gives no resource — should NOT register")
	}
}

// TestOppTriggerValue_LandDoesNotCount pins the land exclusion.
func TestOppTriggerValue_LandDoesNotCount(t *testing.T) {
	// Synthetic land with an opp-trigger value text (no such real
	// card exists in the canonical class but the gate must hold).
	land := cardWithStaticText("Test Tithe Land", []string{"land"}, 0,
		"whenever an opponent draws a card, you may draw a card.")
	gs := newTestGame(t, 2)
	p := newTestPermanent(gs.Seats[0], land, 0, 0)
	if isOpponentTriggeredValueEngine(p) {
		t.Errorf("Lands are excluded from the opp-trigger value class — should NOT register")
	}
}

// TestOppTriggerValue_CountCappedAt4 pins the per-seat 4 cap so a
// wide stax/value board doesn't blow out the dimension.
func TestOppTriggerValue_CountCappedAt4(t *testing.T) {
	gs := newTestGame(t, 2)
	for i := 0; i < 6; i++ {
		c := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
			"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
		newTestPermanent(gs.Seats[0], c, 0, 0)
	}
	got := opponentTriggerValueCount(gs.Seats[0])
	if got != 4 {
		t.Errorf("six engines should cap at 4; got %d", got)
	}
}

// TestScoreCards_SmotheringTitheGetsCreditInPod pins the headline
// result: pre-fix Smothering Tithe scored zero CardAdvantage (it's
// not a draw engine); post-fix it gets credit scaled by opp count.
func TestScoreCards_SmotheringTitheGetsCreditInPod(t *testing.T) {
	gs := newTestGame(t, 4)
	equalizeLifeAndZones(t, gs, 4, 40)

	tithe := cardWithStaticText("Smothering Tithe", []string{"enchantment"}, 4,
		"whenever an opponent draws a card, unless that player pays {2}, you create a tapped treasure token.")
	newTestPermanent(gs.Seats[0], tithe, 0, 0)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if !(r0.CardAdvantage > r1.CardAdvantage) {
		t.Errorf("Smothering Tithe seat should out-score empty seat on CardAdvantage; got tithe=%.3f empty=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

// TestScoreCards_RhysticScalesWithPodSize pins the scaling invariant:
// the same Rhystic Study contributes MORE CardAdvantage in a 4-player
// pod (3 opponents) than in a 2-player game (1 opponent).
//
// Builds two parallel game states with identical zones and lives,
// differing only in seat count, and confirms the gap between the
// Rhystic seat's CardAdvantage and an empty opponent's grows with
// pod size. The 0.4-per-opp scaling is the new headline behavior.
func TestScoreCards_RhysticScalesWithPodSize(t *testing.T) {
	mkGame := func(n int) (*GameStateEvaluator, float64) {
		gs := newTestGame(t, n)
		equalizeLifeAndZones(t, gs, 4, 40)
		rhystic := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
			"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
		newTestPermanent(gs.Seats[0], rhystic, 0, 0)
		ev := NewEvaluator(nil)
		r := ev.EvaluateDetailed(gs, 0)
		return ev, r.CardAdvantage
	}

	_, ca2 := mkGame(2)
	_, ca4 := mkGame(4)

	if !(ca4 > ca2) {
		t.Errorf("Rhystic in 4-player pod (3 opps) should score higher CA than in 2-player (1 opp); got 4p=%.3f 2p=%.3f", ca4, ca2)
	}
	// Gap should be at least 2 * 0.4 = 0.8 (two extra opponents,
	// 0.4 per opp). Verify the gap is meaningful, not just barely
	// positive.
	gap := ca4 - ca2
	if gap < 0.5 {
		t.Errorf("Rhystic 4p-vs-2p gap should be >=0.5 (scaling captures the extra opps); got gap=%.3f", gap)
	}
}

// TestScoreCards_RhysticStillOutscoresArena pins the existing pre-r60
// invariant: Rhystic Study still out-scores Phyrexian Arena. Rhystic
// gets the rate-class 1.5 + the new opp-trigger bonus; Arena gets
// only 1.0 (Arena fires on own upkeep, not opp action — not an opp-
// trigger value engine). The new term doesn't break the ordering.
func TestScoreCards_RhysticStillOutscoresArena(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	rhystic := cardWithStaticText("Rhystic Study", []string{"enchantment"}, 3,
		"whenever an opponent casts a spell, unless that player pays {1}, you may draw a card")
	newTestPermanent(gs.Seats[0], rhystic, 0, 0)

	arena := cardWithStaticText("Phyrexian Arena", []string{"enchantment"}, 3,
		"at the beginning of your upkeep, you draw a card and you lose 1 life")
	newTestPermanent(gs.Seats[1], arena, 0, 0)

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if !(r0.CardAdvantage > r1.CardAdvantage) {
		t.Errorf("Rhystic should still out-score Arena; got rhystic=%.3f arena=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}

// TestScoreCards_OppTriggerSeatPenalizesObservers confirms symmetry:
// from an opp's perspective, Rhystic Study on seat 0 contributes a
// (small) negative CA term for seat 1 (the opp is on the receiving
// end of the engine even if they don't suffer card loss directly —
// the opp's relative CA position is diminished). The 0.4-per-opp
// scaling applies both to the controller and to the observers.
func TestScoreCards_OppTriggerSeatPenalizesObservers(t *testing.T) {
	gs := newTestGame(t, 4)
	equalizeLifeAndZones(t, gs, 4, 40)

	tithe := cardWithStaticText("Smothering Tithe", []string{"enchantment"}, 4,
		"whenever an opponent draws a card, unless that player pays {2}, you create a tapped treasure token.")
	newTestPermanent(gs.Seats[0], tithe, 0, 0)

	// Compare a "no Tithe at the table" baseline against the
	// real game from seat 1's POV.
	gsNo := newTestGame(t, 4)
	equalizeLifeAndZones(t, gsNo, 4, 40)

	ev := NewEvaluator(nil)
	r1Real := ev.EvaluateDetailed(gs, 1)
	r1None := ev.EvaluateDetailed(gsNo, 1)

	if !(r1Real.CardAdvantage < r1None.CardAdvantage) {
		t.Errorf("seat 1's CA should be lower when seat 0 has a Tithe than when no one does; got real=%.3f none=%.3f",
			r1Real.CardAdvantage, r1None.CardAdvantage)
	}
}

// TestScoreCards_BothSeatsEqualEnginesNoNetChange confirms the
// symmetry: when two seats have the same opp-trigger value engine,
// the term cancels and neither gets a CA advantage from it. Guards
// against accidentally double-crediting via the controller-side bonus
// without subtracting the opp's mirror engine.
func TestScoreCards_BothSeatsEqualEnginesNoNetChange(t *testing.T) {
	gs := newTestGame(t, 2)
	equalizeLifeAndZones(t, gs, 4, 40)

	for _, idx := range []int{0, 1} {
		tithe := cardWithStaticText("Smothering Tithe", []string{"enchantment"}, 4,
			"whenever an opponent draws a card, unless that player pays {2}, you create a tapped treasure token.")
		newTestPermanent(gs.Seats[idx], tithe, 0, 0)
	}

	ev := NewEvaluator(nil)
	r0 := ev.EvaluateDetailed(gs, 0)
	r1 := ev.EvaluateDetailed(gs, 1)

	if math.Abs(r0.CardAdvantage-r1.CardAdvantage) > 1e-9 {
		t.Errorf("two seats with identical engines should have equal CA; got s0=%.3f s1=%.3f",
			r0.CardAdvantage, r1.CardAdvantage)
	}
}
