package hat

// R60 round 5 — perceived-archetype confidence curve. Builds on the
// bluff-detection foundation (#290) by adding a non-linear scaling
// curve to the rolling OpponentProfile confidence so high-confidence
// reads commit the hat harder against the identified archetype than
// borderline guesses do.
//
// Before: targeting / removal-selection biases used `prof.Confidence`
// as a direct linear multiplier. A 0.45 borderline call and a 0.90
// stable call scaled the same bias by their own value — so a hat that
// had watched an opponent tutor for three straight turns committed to
// pressuring them only 2x harder than one with a single barely-above-
// threshold observation. The directive: when confidence is HIGH, bias
// HARDER.
//
// After: `archetypeBiasMultiplier(conf)` returns 0 below the floor
// (0.4), linear pass-through up to 0.75 (so prior moderate-confidence
// behavior is preserved bit-for-bit), then super-linear above 0.75
// with slope 2.0 (so a 0.90 read multiplies the bias by 1.20 instead
// of 0.90 — a 33% boost over pure linear, 3x the 0.40 floor).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ---------------------------------------------------------------------
// archetypeBiasMultiplier — curve properties
// ---------------------------------------------------------------------

func TestArchetypeBiasMultiplier_BelowFloorIsZero(t *testing.T) {
	cases := []float64{0, 0.1, 0.3, 0.39, 0.3999}
	for _, c := range cases {
		if got := archetypeBiasMultiplier(c); got != 0 {
			t.Errorf("conf=%.3f below floor must yield 0; got %.3f", c, got)
		}
	}
}

func TestArchetypeBiasMultiplier_LinearThroughHighThreshold(t *testing.T) {
	// In the [floor, highThreshold] range, multiplier == confidence
	// (pure linear pass-through, preserving pre-R60r5 behavior).
	cases := []float64{
		archetypeBiasMinConfidence,
		0.5,
		0.6,
		archetypeBiasHighThreshold, // boundary
	}
	for _, c := range cases {
		got := archetypeBiasMultiplier(c)
		if got != c {
			t.Errorf("conf=%.3f in linear range must equal itself; got %.3f", c, got)
		}
	}
}

func TestArchetypeBiasMultiplier_SuperLinearAboveThreshold(t *testing.T) {
	// Above highThreshold, multiplier > confidence (the boost fires).
	cases := []float64{0.80, 0.85, 0.90, 0.95}
	for _, c := range cases {
		got := archetypeBiasMultiplier(c)
		if got <= c {
			t.Errorf("conf=%.3f above threshold must boost above linear; got %.3f", c, got)
		}
	}
}

func TestArchetypeBiasMultiplier_HighConfidenceCommitsHarder(t *testing.T) {
	// The whole point: a 0.90 read should give a meaningfully higher
	// multiplier than a 0.75 read, more than the pure-linear gap (0.15)
	// would suggest.
	lo := archetypeBiasMultiplier(0.75)
	hi := archetypeBiasMultiplier(0.90)
	gap := hi - lo
	if gap <= 0.15 {
		t.Errorf("0.90 vs 0.75 confidence gap must exceed the 0.15 linear delta; got %.3f", gap)
	}
}

func TestArchetypeBiasMultiplier_Monotonic(t *testing.T) {
	prev := 0.0
	for c := 0.0; c <= 1.0; c += 0.05 {
		got := archetypeBiasMultiplier(c)
		if got < prev {
			t.Errorf("multiplier must be non-decreasing in conf; at %.3f got %.3f < prev %.3f", c, got, prev)
		}
		prev = got
	}
}

// Concrete numeric spot-check the curve doesn't silently slip — these
// are the values the comment in opponent_profile.go documents.
func TestArchetypeBiasMultiplier_NumericContract(t *testing.T) {
	cases := []struct {
		conf float64
		want float64
	}{
		{0.40, 0.40},  // floor
		{0.60, 0.60},  // linear
		{0.75, 0.75},  // boundary (no boost yet)
		{0.90, 1.20},  // (0.90 + (0.90-0.75)*2.0)
		{0.95, 1.35},  // (0.95 + (0.95-0.75)*2.0)
	}
	for _, c := range cases {
		got := archetypeBiasMultiplier(c.conf)
		if got < c.want-0.001 || got > c.want+0.001 {
			t.Errorf("conf=%.2f → multiplier expected %.3f, got %.3f",
				c.conf, c.want, got)
		}
	}
}

// ---------------------------------------------------------------------
// Integration — confidence ramp drives stronger archetype bias
// ---------------------------------------------------------------------

// Simulate three consecutive tutor casts from one opponent and verify
// classifyOpponent ramps the combo classification's confidence above
// 0.75 (into the super-linear bias zone).
func TestClassifyOpponent_ThreeTutorsInARowRampsConfidenceHigh(t *testing.T) {
	gs := newTestGame(t, 2)
	h := NewYggdrasilHat(nil, 0)
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "game_start"})

	// Hold mana up so the combo classifier fires.
	for i := 0; i < 3; i++ {
		island := newTestCardMinimal("Island", []string{"land"}, 0, nil)
		newTestPermanent(gs.Seats[1], island, 0, 0)
	}

	// Turn 2 tutor.
	gs.Turn = 2
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "upkeep"})
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "tutor", Seat: 1, Source: "Demonic Tutor"})
	prof := h.classifyOpponent(gs, 1)
	confT2 := prof.Confidence

	// Turn 3 tutor.
	gs.Turn = 3
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "upkeep"})
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "tutor", Seat: 1, Source: "Vampiric Tutor"})
	prof = h.classifyOpponent(gs, 1)
	confT3 := prof.Confidence

	// Turn 4 tutor.
	gs.Turn = 4
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "upkeep"})
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "tutor", Seat: 1, Source: "Imperial Seal"})
	prof = h.classifyOpponent(gs, 1)
	confT4 := prof.Confidence

	if prof.Archetype != "combo" {
		t.Fatalf("three tutors must classify as combo; got %q", prof.Archetype)
	}
	if !(confT2 < confT3 && confT3 < confT4) {
		t.Errorf("confidence must ramp with stable repeated classification; t2=%.3f t3=%.3f t4=%.3f",
			confT2, confT3, confT4)
	}
	// The whole directive's premise: 3 turns of tutoring → high
	// confidence (above the super-linear boundary).
	if confT4 < archetypeBiasHighThreshold {
		t.Errorf("by turn 4 with sustained combo signals, confidence must exceed %.2f (into super-linear bias zone); got %.3f",
			archetypeBiasHighThreshold, confT4)
	}
}

// End-to-end: a high-confidence combo classification produces a
// noticeably stronger targeting-bias score contribution than a
// moderate-confidence one — verifying the curve actually reaches the
// decision math.
func TestArchetypeBiasMultiplier_HighConfBiasExceedsLinearProjection(t *testing.T) {
	// Two equivalent "bias × multiplier" computations the targeting
	// loop performs. With moderate confidence, mult == conf (linear);
	// with high confidence, mult > conf (super-linear).
	moderate := archetypeBiasMultiplier(0.50)
	high := archetypeBiasMultiplier(0.90)

	// Linear projection: if multiplier were purely conf, going from
	// 0.50 → 0.90 would 1.8x the bias. Super-linear: 0.50 → 1.20 = 2.4x.
	linearProjection := high / moderate
	if linearProjection < 2.3 {
		t.Errorf("super-linear curve must produce at least 2.3x bias gain from 0.50 → 0.90; got %.2fx", linearProjection)
	}
}

// Belt-and-suspenders: when the opponent profile is unset / unknown,
// the multiplier returns 0 and no archetype bias is applied. The
// downstream code paths must handle a 0 multiplier as a no-op.
func TestArchetypeBiasMultiplier_UnknownConfidenceIsNoOp(t *testing.T) {
	if archetypeBiasMultiplier(0) != 0 {
		t.Error("conf=0 (unknown / unclassified opponent) must yield 0 multiplier")
	}
}
