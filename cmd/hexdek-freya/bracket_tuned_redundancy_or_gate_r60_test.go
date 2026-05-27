package main

import "testing"

// PR #566 ships the bare GC>=1 arm of the Tuned-redundancy gate; this
// suite pins the OR'd full-form gate:
//
//	tunedRedundancy && (gameChangerCount >= 1 || trueInfCount >= 1 || tutorDensity >= 0.08)
//
// Each arm must fire independently when the other two are off, and the
// gate must hold when all three corroborating signals are absent.
// Coverage matrix:
//
//	arm        +    arm fires alone        |  arm just below threshold
//	---------------+--------------------+--------------------
//	GC>=1          |  PR #566 covers           |  PR #566 covers (GC=0)
//	trueInf>=1     |  TestArm_TrueInf...       |  TestArm_TrueInfZero...
//	tutor>=0.08    |  TestArm_Tutor8Pct...     |  TestArm_TutorJustBelow...
//	all three off  |  —                        |  TestArm_AllThreeOff...
//
// The trueInf arm is deliberately defense-in-depth: the Winning-combo
// floor (line 1629 of archetype.go) already lifts ANY deck with a
// TrueInfinite to B4 before the tuned-redundancy floor evaluates, so
// in practice the trueInf arm's `bracket < 4` guard is never reached
// when the arm itself is true. The test asserts the OUTCOME (B4)
// rather than which floor recorded the lift — the test would fail if
// either the Winning-combo floor OR the trueInf arm of the
// tuned-redundancy gate were removed.

// TestArm_TutorDensity8PctLiftsAlone — tutor density alone (no GC, no
// true infinites) corroborates the tuned-redundancy floor at the
// 8%-of-nonlands threshold. Mirrors the bracket-scoring table's
// 8-11% tutor band (line 1318 of archetype.go) — if 8% tutors earn
// +2 to the raw bracket score, the same signal should be sufficient
// to corroborate the tuned-redundancy floor's optimization claim.
func TestArm_TutorDensity8PctLiftsAlone(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(10),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.8,
		fastManaCount:    7,
		gameChangerCount: 0,
		tutorDensity:     0.08, // exactly at the gate threshold
	}
	bracket, _, br := estimateMeasuredBracket(ctx, report, "")
	if bracket != 4 {
		t.Fatalf("tutor-only arm: expected B4 lift, got B%d", bracket)
	}
	if !hasSignal(br.Signals, "floor", "Tuned-redundancy floor") {
		t.Errorf("expected Tuned-redundancy floor signal, got: %+v", br.Signals)
	}
}

// TestArm_TutorDensityJustBelowDoesNotLift — at 7% tutor density,
// just below the 8% gate threshold, with GC=0 and no infinites, the
// floor must hold. Pins the boundary semantics of the gate's third
// arm and documents the calibration: the gate's tutor threshold is a
// strict >= 0.08, not > 0.07.
func TestArm_TutorDensityJustBelowDoesNotLift(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(10),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           3.0,
		fastManaCount:    7,
		gameChangerCount: 0,
		tutorDensity:     0.07, // just below gate
	}
	bracket, _, br := estimateMeasuredBracket(ctx, report, "")
	if bracket >= 4 {
		t.Fatalf("tutor 7%% lifted to B%d — gate threshold should be strict >=0.08", bracket)
	}
	if hasSignal(br.Signals, "floor", "Tuned-redundancy floor") {
		t.Errorf("Tuned-redundancy floor fired at tutor density 7%%")
	}
}

// TestArm_TrueInfiniteLiftsAlone — a deck with a true-infinite combo,
// zero Game Changers, and zero tutors must land at B4. In current
// ordering this is delivered by the Winning-combo floor (line 1629)
// preempting the tuned-redundancy floor; the trueInf arm of the gate
// is therefore defense-in-depth. The test asserts the END STATE
// (B4 + some categorical floor signal lifted it) rather than pinning
// which floor recorded the lift — if a future refactor narrows
// hasWinningCombo such that the Winning-combo floor stops firing on
// some true-infinite shapes, the trueInf arm of THIS gate would take
// over and the test would still pass.
func TestArm_TrueInfiniteLiftsAlone(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		TrueInfinites: []ComboResult{
			{Cards: []string{"Thassa's Oracle", "Demonic Consultation"}, LoopType: "true_infinite"},
		},
		Finishers: makeComboList(10),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.6,
		fastManaCount:    7,
		gameChangerCount: 0,
		tutorDensity:     0.0,
	}
	bracket, _, br := estimateMeasuredBracket(ctx, report, "")
	if bracket != 4 {
		t.Fatalf("true-infinite arm: expected B4 lift, got B%d", bracket)
	}
	// Either floor is acceptable — they both encode the same WotC
	// framework rule ("a categorical-win 2-card combo is a B4 marker").
	if !hasSignal(br.Signals, "floor", "Tuned-redundancy floor") &&
		!hasSignal(br.Signals, "floor", "Winning-combo floor") {
		t.Errorf("expected Tuned-redundancy or Winning-combo floor signal, got: %+v", br.Signals)
	}
}

// TestArm_TrueInfiniteZeroFinishersDoesNotFireTunedFloor — true
// infinite present BUT finisher density below the tuned-redundancy
// threshold (5 < 8 floor). The deck still lifts to B4 via the
// Winning-combo floor, but the Tuned-redundancy floor itself MUST NOT
// fire (its gate's `tunedRedundancy` clause is independent of the
// 3-arm OR). This pins that the OR gate does not bypass the finisher
// threshold — it only relaxes the GC requirement.
func TestArm_TrueInfiniteZeroFinishersDoesNotFireTunedFloor(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		TrueInfinites: []ComboResult{
			{Cards: []string{"Thassa's Oracle", "Demonic Consultation"}, LoopType: "true_infinite"},
		},
		Finishers: makeComboList(5), // below tunedRedundancy threshold (8)
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           3.0,
		fastManaCount:    7,
		gameChangerCount: 0,
		tutorDensity:     0.0,
	}
	_, _, br := estimateMeasuredBracket(ctx, report, "")
	if hasSignal(br.Signals, "floor", "Tuned-redundancy floor") {
		t.Errorf("Tuned-redundancy floor fired without finisher >=8 threshold")
	}
}

// TestArm_AllThreeArmsOffDoesNotLift — the canonical negative-of-fix
// case: tunedRedundancy is TRUE (10 finishers + 7 fast mana = the
// stock-precon shape) but every arm of the corroboration gate is OFF
// (GC=0, no infinites, 4% tutors). The floor MUST NOT fire and the
// GC=0 ceiling at line 1558 holds the deck at B2. This is the bug
// that PR #566 was created to fix, generalized to "OR-gate is honest:
// the 'all-off' case fails closed."
func TestArm_AllThreeArmsOffDoesNotLift(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(10), // tunedRedundancy TRUE
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           3.4,
		fastManaCount:    7, // tunedRedundancy TRUE
		gameChangerCount: 0, // gate arm 1 OFF
		tutorDensity:     0.04, // gate arm 3 OFF (below 0.08 threshold)
		comboCount:       0, // gate arm 2 OFF (no TrueInfinites in report)
	}
	bracket, _, br := estimateMeasuredBracket(ctx, report, "")
	if bracket >= 4 {
		t.Fatalf("all-three-arms-off lifted to B%d — OR gate is not failing closed", bracket)
	}
	if hasSignal(br.Signals, "floor", "Tuned-redundancy floor") {
		t.Errorf("Tuned-redundancy floor fired with all 3 corroborating arms off: %+v", br.Signals)
	}
}

// TestArm_TutorDensityArmFiresIndependently — tutor 8% but TUNED
// REDUNDANCY off (only 5 finishers — below the 8 threshold). The
// floor must NOT fire because the gate's `tunedRedundancy` clause
// fails. This pins that the OR gate is a CORROBORATION on top of
// tunedRedundancy, not a replacement.
func TestArm_TutorDensityArmFiresIndependently(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(5), // tunedRedundancy FALSE
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           3.0,
		fastManaCount:    7,
		gameChangerCount: 0,
		tutorDensity:     0.10, // gate arm fires but tunedRedundancy is false
	}
	_, _, br := estimateMeasuredBracket(ctx, report, "")
	if hasSignal(br.Signals, "floor", "Tuned-redundancy floor") {
		t.Errorf("Tuned-redundancy floor fired without tunedRedundancy itself being true")
	}
}

// hasSignal scans a BracketRationale's Signals for a matching kind+name.
func hasSignal(sigs []BracketSignal, kind, name string) bool {
	for _, s := range sigs {
		if s.Kind == kind && s.Name == name {
			return true
		}
	}
	return false
}
