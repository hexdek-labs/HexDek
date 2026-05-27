package main

import (
	"strings"
	"testing"
)

// r60 rebalance: estimatePowerPercentile's mana-base contribution was
// stale. Pre-r60 the table was:
//
//	A → +10 (logged), B → +5 (SILENT), C → 0, D → -10, F → -10
//
// with three concrete bugs:
//   - B silently credited +5 with no user-facing factor message
//   - C entirely ignored despite being a real signal (~7.5% of the
//     moxfield corpus lands at C)
//   - D and F collapsed at the same -10 — squashed two distinct grades
//
// New table:
//
//	A → +10, B → +5, C → -3, D → -8, F → -15
//
// All rungs now log a factor message for transparency.

// neutralProfileForManaTest returns a DeckProfile + FreyaReport pair
// configured so EVERY OTHER factor in estimatePowerPercentile sits in
// its dead zone — the only delta from the 50-baseline comes from the
// mana-base grade. Used by the rung tests below to isolate the
// mana-base contribution exactly.
func neutralProfileForManaTest(grade string) (*DeckProfile, *FreyaReport) {
	dp := &DeckProfile{
		ManaBaseGrade: grade,
		// Win-line count in the no-bonus / no-penalty zone (2 →
		// neither >=3 nor <=1 → 0)
		WinLineCount: 2,
		// Interaction CMC in the no-bonus zone (2.5 → > 2.0 and
		// <= 3.5 → 0)
		InteractionQuality: 2.5,
		// Draw count in the no-bonus zone (8 → < 12 and >= 5 → 0)
		DrawCount: 8,
		// Commander synergy in the no-bonus zone (0.40 → < 0.60
		// and >= 0.25 → 0)
		CommanderSynergy: 0.40,
		// AvgCMC in the no-bonus zone (3.0 → >= 2.5 and <= 3.8 → 0)
		AvgCMC: 3.0,
		// KeepableHandPct in the no-bonus zone (75 → < 85 and
		// >= 70 → 0)
		KeepableHandPct: 75,
	}
	report := &FreyaReport{
		// NonLandTutorCount in the no-bonus zone (3 → < 5 and > 1 → 0)
		NonLandTutorCount: 3,
	}
	return dp, report
}

// TestPowerPercentileManaGrade_AGives10Logged confirms A-grade
// remains +10 with the same factor message.
func TestPowerPercentileManaGrade_AGives10Logged(t *testing.T) {
	dp, report := neutralProfileForManaTest("A")
	score, factors := estimatePowerPercentile(dp, report)
	if score != 60 {
		t.Errorf("A-grade should be 50 + 10 = 60; got %d", score)
	}
	found := false
	for _, f := range factors {
		if strings.Contains(f, "A-grade mana base") && strings.Contains(f, "+10") {
			found = true
		}
	}
	if !found {
		t.Errorf("A-grade should log a +10 factor; got %v", factors)
	}
}

// TestPowerPercentileManaGrade_BGives5Logged pins the new
// transparency: B-grade now logs a +5 factor (previously silent).
func TestPowerPercentileManaGrade_BGives5Logged(t *testing.T) {
	dp, report := neutralProfileForManaTest("B")
	score, factors := estimatePowerPercentile(dp, report)
	if score != 55 {
		t.Errorf("B-grade should be 50 + 5 = 55; got %d", score)
	}
	found := false
	for _, f := range factors {
		if strings.Contains(f, "B-grade mana base") && strings.Contains(f, "+5") {
			found = true
		}
	}
	if !found {
		t.Errorf("B-grade should now LOG a +5 factor (previously silent); got %v", factors)
	}
}

// TestPowerPercentileManaGrade_CGivesMinus3Logged pins the new
// C-grade rung. Pre-r60 C was ignored (silently 0).
func TestPowerPercentileManaGrade_CGivesMinus3Logged(t *testing.T) {
	dp, report := neutralProfileForManaTest("C")
	score, factors := estimatePowerPercentile(dp, report)
	if score != 47 {
		t.Errorf("C-grade should be 50 - 3 = 47; got %d", score)
	}
	found := false
	for _, f := range factors {
		if strings.Contains(f, "C-grade mana base") && strings.Contains(f, "-3") {
			found = true
		}
	}
	if !found {
		t.Errorf("C-grade should log a -3 factor (previously ignored); got %v", factors)
	}
}

// TestPowerPercentileManaGrade_DGivesMinus8Logged pins the D split:
// previously D collapsed at -10 with F; now D is -8.
func TestPowerPercentileManaGrade_DGivesMinus8Logged(t *testing.T) {
	dp, report := neutralProfileForManaTest("D")
	score, factors := estimatePowerPercentile(dp, report)
	if score != 42 {
		t.Errorf("D-grade should be 50 - 8 = 42; got %d", score)
	}
	found := false
	for _, f := range factors {
		if strings.Contains(f, "D-grade mana base") && strings.Contains(f, "-8") {
			found = true
		}
	}
	if !found {
		t.Errorf("D-grade should log a -8 factor; got %v", factors)
	}
}

// TestPowerPercentileManaGrade_FGivesMinus15Logged pins the F split:
// previously F collapsed at -10 with D; now F is -15 (catastrophic
// mana base — 4+ color gaps, 8+ taplands, no fixing).
func TestPowerPercentileManaGrade_FGivesMinus15Logged(t *testing.T) {
	dp, report := neutralProfileForManaTest("F")
	score, factors := estimatePowerPercentile(dp, report)
	if score != 35 {
		t.Errorf("F-grade should be 50 - 15 = 35; got %d", score)
	}
	found := false
	for _, f := range factors {
		if strings.Contains(f, "F-grade mana base") && strings.Contains(f, "-15") {
			found = true
		}
	}
	if !found {
		t.Errorf("F-grade should log a -15 factor; got %v", factors)
	}
}

// TestPowerPercentileManaGrade_FWorseThanD confirms the spread: F is
// strictly worse than D. Pre-r60 they were identical at -10.
func TestPowerPercentileManaGrade_FWorseThanD(t *testing.T) {
	dpD, repD := neutralProfileForManaTest("D")
	dpF, repF := neutralProfileForManaTest("F")
	scoreD, _ := estimatePowerPercentile(dpD, repD)
	scoreF, _ := estimatePowerPercentile(dpF, repF)
	if !(scoreF < scoreD) {
		t.Errorf("F should score strictly lower than D; got F=%d D=%d", scoreF, scoreD)
	}
}

// TestPowerPercentileManaGrade_OrderingMonotonic confirms the full
// A>B>C>D>F monotonic ordering. Each rung must strictly out-score the
// next-lower rung.
func TestPowerPercentileManaGrade_OrderingMonotonic(t *testing.T) {
	scores := make(map[string]int)
	for _, g := range []string{"A", "B", "C", "D", "F"} {
		dp, report := neutralProfileForManaTest(g)
		s, _ := estimatePowerPercentile(dp, report)
		scores[g] = s
	}
	pairs := [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}, {"D", "F"}}
	for _, p := range pairs {
		if !(scores[p[0]] > scores[p[1]]) {
			t.Errorf("ordering broken: %s (%d) should be > %s (%d)",
				p[0], scores[p[0]], p[1], scores[p[1]])
		}
	}
}

// TestPowerPercentileManaGrade_AllRungsLogFactor confirms transparency
// invariant: every rung (A through F) now writes a user-facing factor
// message. Previously B and C were silent.
func TestPowerPercentileManaGrade_AllRungsLogFactor(t *testing.T) {
	for _, g := range []string{"A", "B", "C", "D", "F"} {
		dp, report := neutralProfileForManaTest(g)
		_, factors := estimatePowerPercentile(dp, report)
		found := false
		for _, f := range factors {
			if strings.Contains(f, g+"-grade mana base") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("grade %s should log a mana-base factor; got %v", g, factors)
		}
	}
}

// TestPowerPercentileManaGrade_EmptyGradeNoCredit confirms the
// default case: when ManaBaseGrade is the empty string (the field
// hasn't been populated — e.g. oracle data unavailable),
// estimatePowerPercentile contributes neither credit nor penalty
// from the mana-base term.
func TestPowerPercentileManaGrade_EmptyGradeNoCredit(t *testing.T) {
	dp, report := neutralProfileForManaTest("")
	score, factors := estimatePowerPercentile(dp, report)
	if score != 50 {
		t.Errorf("empty-grade should sit at the 50 baseline; got %d", score)
	}
	for _, f := range factors {
		if strings.Contains(f, "mana base") {
			t.Errorf("empty-grade should NOT log a mana-base factor; got %q", f)
		}
	}
}
