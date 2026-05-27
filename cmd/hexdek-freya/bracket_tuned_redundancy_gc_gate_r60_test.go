package main

import "testing"

// TestTunedRedundancyFloor_GCZeroDoesNotLift pins the r60 fix that the
// Tuned-redundancy floor does NOT re-lift a GC=0 deck back to B4 after
// the GC=0 ceiling capped at B2. Reproduces the stock-precon B4 FP
// surface documented across docs/precon-shape-scans/group-{a,b,c}.md
// (Urza's Iron Alliance, Madison Li, Kalemne, Wilhelt, Hosts of Mordor,
// Hearthhull, Hazel, Blame Game, Family Matters, Cabaretti Cacophony,
// Ixhel, Creative Energy, Coven Counters — all 0 GC, 0 tutors, but
// finisherCount >= 8 + fastManaCount >= 6 from stock signet/Sol Ring
// loadout + creature-dense top end).
func TestTunedRedundancyFloor_GCZeroDoesNotLift(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(10),
	}
	// Stock-precon shape: dense finishers + signet-counted "fast mana"
	// but zero Game Changers, zero tutors, zero combo lines. Pre-fix this
	// landed at B4; post-fix the GC=0 ceiling holds at B2.
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           3.4,
		fastManaCount:    7,
		gameChangerCount: 0,
		tutorDensity:     0.0,
		comboCount:       0,
	}
	bracket, _, br := estimateMeasuredBracket(ctx, report, "")
	if bracket >= 4 {
		t.Fatalf("GC=0 precon shape lifted to B%d — Tuned-redundancy floor over-fires", bracket)
	}
	for _, sig := range br.Signals {
		if sig.Kind == "floor" && sig.Name == "Tuned-redundancy floor" {
			t.Errorf("Tuned-redundancy floor fired on GC=0 deck: %+v", sig)
		}
	}
}

// TestTunedRedundancyFloor_GCOneStillLifts pins that the GC>=1 gate
// preserves the intended lift path. A deck with one Game Changer plus
// the redundancy signals is the canonical Voja-tribal-Voltron shape
// (the original calibration target from PR #129's tuned-redundancy
// commit) and must continue to land at B4.
func TestTunedRedundancyFloor_GCOneStillLifts(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(10),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.6,
		fastManaCount:    8,
		gameChangerCount: 1,
	}
	bracket, _, br := estimateMeasuredBracket(ctx, report, "")
	if bracket != 4 {
		t.Fatalf("GC=1 + tuned-redundancy: expected B4 lift, got B%d", bracket)
	}
	found := false
	for _, sig := range br.Signals {
		if sig.Kind == "floor" && sig.Name == "Tuned-redundancy floor" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected Tuned-redundancy floor signal in rationale, got: %+v", br.Signals)
	}
}

// TestTunedRedundancyFloor_GCZeroBoundaryThresholds pins behavior at
// the predicate's exact thresholds (finishers==8, fastMana==6) with
// GC=0 — even at the edge values that historically fired the floor,
// the gate now holds. Documents the calibration intent: "redundancy
// signals alone are not a B4 marker; you need at least one explicit
// optimization vehicle."
func TestTunedRedundancyFloor_GCZeroBoundaryThresholds(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(8),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           3.0,
		fastManaCount:    6,
		gameChangerCount: 0,
	}
	bracket, _, _ := estimateMeasuredBracket(ctx, report, "")
	if bracket >= 4 {
		t.Fatalf("threshold-boundary GC=0 deck lifted to B%d", bracket)
	}
}

// TestTunedRedundancyFloor_BelowFinisherThresholdDoesNotFire covers the
// orthogonal case: even with a high GC count, the floor doesn't fire
// without the redundancy signals (the GC=1-3 ceiling alone may permit
// B4 only via the explicit hasWinningCombo path).
func TestTunedRedundancyFloor_BelowFinisherThresholdDoesNotFire(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(5),
	}
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           3.2,
		fastManaCount:    7,
		gameChangerCount: 2,
	}
	_, _, br := estimateMeasuredBracket(ctx, report, "")
	for _, sig := range br.Signals {
		if sig.Kind == "floor" && sig.Name == "Tuned-redundancy floor" {
			t.Errorf("Tuned-redundancy floor fired without finisher threshold: %+v", sig)
		}
	}
}
