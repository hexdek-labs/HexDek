package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// confidence floor: a no-tutor, no-redundancy, no-protection 2-card combo
// lands at 0 since all three signals report 0.
func TestWinLineConfidence_AllZeroFloor(t *testing.T) {
	wl := &WinLine{
		Pieces: []string{"Piece A", "Piece B"},
		Type:   "infinite",
	}
	got := computeWinLineConfidence(wl, map[string]CardProfile{}, 0.0)
	if got != 0 {
		t.Fatalf("all-zero confidence = %.3f, want 0", got)
	}
}

// confidence ceiling: every piece tutorable, ample alternatives in deck,
// saturated protection density → 1.0.
func TestWinLineConfidence_AllOneCeiling(t *testing.T) {
	wl := &WinLine{
		Pieces: []string{"Sanguine Bond", "Exquisite Blood"},
		Type:   "infinite",
		TutorPaths: []TutorChain{
			{Tutor: "Demonic Tutor", Finds: "Sanguine Bond"},
			{Tutor: "Vampiric Tutor", Finds: "Exquisite Blood"},
		},
	}
	// 4+ functional alternatives per piece via LifegainToDrain / LifelossToPump
	profiles := map[string]CardProfile{
		"Sanguine Bond":   {Name: "Sanguine Bond", LifegainToDrain: true},
		"Vito":            {Name: "Vito", LifegainToDrain: true},
		"Vizkopa Guildmage": {Name: "Vizkopa Guildmage", LifegainToDrain: true},
		"Exquisite Blood":         {Name: "Exquisite Blood", LifelossToPump: true},
		"Marauding Blight-Priest": {Name: "Marauding Blight-Priest", LifelossToPump: true},
		"Cliffhaven Vampire":      {Name: "Cliffhaven Vampire", LifelossToPump: true},
	}
	got := computeWinLineConfidence(wl, profiles, 1.0)
	if math.Abs(got-1.0) > 1e-9 {
		t.Fatalf("saturated confidence = %.3f, want 1.0", got)
	}
}

// formula pin: 0.45*tutor + 0.30*redundancy + 0.25*protection. Half-coverage
// on tutor (1 of 2 pieces), 1 alternative for one piece (0.5/2 = 0.25
// redundancy avg), 0.4 protection ⇒ 0.45*0.5 + 0.30*0.25 + 0.25*0.4 = 0.4.
func TestWinLineConfidence_FormulaWeights(t *testing.T) {
	wl := &WinLine{
		Pieces: []string{"Piece A", "Piece B"},
		Type:   "infinite",
		TutorPaths: []TutorChain{
			{Tutor: "Demonic Tutor", Finds: "Piece A"},
		},
	}
	profiles := map[string]CardProfile{
		"Piece A": {Name: "Piece A", IsOutlet: true},
		"Piece B": {Name: "Piece B", IsWinCon: true},
		"AltA":    {Name: "AltA", IsOutlet: true},
	}
	got := computeWinLineConfidence(wl, profiles, 0.4)
	want := 0.4
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("formula confidence = %.6f, want %.6f", got, want)
	}
}

// non-combo lines (combat, commander_damage) use a neutral 0.5/0.5 on the
// tutor + redundancy axes — they're not card slots a tutor would fetch.
func TestWinLineConfidence_CombatLineNeutralAxes(t *testing.T) {
	wl := &WinLine{
		Pieces: []string{"15 threats + 3 pumps"},
		Type:   "combat",
	}
	got := computeWinLineConfidence(wl, map[string]CardProfile{}, 0.0)
	want := 0.45*0.5 + 0.30*0.5 + 0.25*0.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("combat-line confidence = %.6f, want %.6f", got, want)
	}
}

// nil safety.
func TestWinLineConfidence_NilSafe(t *testing.T) {
	if got := computeWinLineConfidence(nil, nil, 0); got != 0 {
		t.Fatalf("nil winline confidence = %v, want 0", got)
	}
}

// protection density saturates at 15% of deck cards.
func TestProtectionDensity_SaturatesAtFifteenPercent(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{
			TotalCards: 100,
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: 20,
				RoleProtection:   5,
			},
		},
	}
	got := computeWinLineProtectionDensity(report, map[string]CardProfile{})
	if got != 1.0 {
		t.Fatalf("25%% protection density = %.3f, want 1.0 (saturated)", got)
	}
}

// finisher-count filter: lines with Confidence < 0.3 drop out of the
// bracket-scoring finisher count.
func TestConfidenceFilteredFinisherCount_DropsLowConfidence(t *testing.T) {
	report := &FreyaReport{
		Finishers: []ComboResult{
			{Cards: []string{"X-Burn"}}, {Cards: []string{"Mass Pump"}}, {Cards: []string{"Storm"}},
		},
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Type: "finisher", Confidence: 0.9},
				{Type: "finisher", Confidence: 0.5},
				{Type: "finisher", Confidence: 0.1},
				{Type: "infinite", Confidence: 0.9}, // wrong type — ignored
			},
		},
	}
	got := confidenceFilteredFinisherCount(report)
	if got != 2 {
		t.Fatalf("filtered finisher count = %d, want 2 (0.1 < 0.3 dropped)", got)
	}
}

// falls back to raw report.Finishers length when WinLines unavailable.
func TestConfidenceFilteredFinisherCount_FallbackNoWinLines(t *testing.T) {
	report := &FreyaReport{
		Finishers: []ComboResult{
			{Cards: []string{"A"}}, {Cards: []string{"B"}},
		},
	}
	if got := confidenceFilteredFinisherCount(report); got != 2 {
		t.Fatalf("no-WinLines fallback = %d, want 2", got)
	}
}

// weighted combo count: confidence >= 0.7 → 1.5x, [0.3, 0.7) → 1x, < 0.3
// → 0. Three lines at 0.9 / 0.5 / 0.1 ⇒ 1.5 + 1.0 + 0 = 2.5 → rounded 3.
func TestWeightedComboCount_HighConfidenceWeighted1_5x(t *testing.T) {
	report := &FreyaReport{
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Type: "infinite", Confidence: 0.9},
				{Type: "determined", Confidence: 0.5},
				{Type: "infinite", Confidence: 0.1},
				{Type: "finisher", Confidence: 0.9}, // wrong type
				{Type: "combat", Confidence: 0.9},   // wrong type
			},
		},
	}
	ctx := &classifyContext{comboCount: 99} // fallback should NOT be used
	got := weightedComboCount(report, ctx)
	if got != 3 {
		t.Fatalf("weighted combo count = %d, want 3 (1.5 + 1.0 + 0 rounded)", got)
	}
}

// weighted combo count falls back to ctx.comboCount when WinLines absent.
func TestWeightedComboCount_FallbackNoWinLines(t *testing.T) {
	report := &FreyaReport{}
	ctx := &classifyContext{comboCount: 4}
	if got := weightedComboCount(report, ctx); got != 4 {
		t.Fatalf("no-WinLines fallback = %d, want 4", got)
	}
}

// ComputeWinLines stamps Confidence on every line (backend-only — verified
// non-zero on at least the infinite line; non-combo lines come out neutral
// even with no protection because of the 0.5 floor).
func TestComputeWinLines_StampsConfidence(t *testing.T) {
	report := &FreyaReport{
		Commander:  "Edric, Spymaster of Trest",
		TotalCards: 100,
		Roles: &RoleAnalysis{
			TotalCards: 100,
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: 6,
			},
		},
		TrueInfinites: []ComboResult{
			{Cards: []string{"Sanguine Bond", "Exquisite Blood"}, Description: "drain loop"},
		},
	}
	qtyProfiles := []CardProfileQty{
		{Profile: CardProfile{Name: "Sanguine Bond", LifegainToDrain: true}, Qty: 1},
		{Profile: CardProfile{Name: "Exquisite Blood", LifelossToPump: true}, Qty: 1},
		{Profile: CardProfile{Name: "Vito", LifegainToDrain: true}, Qty: 1},
		{Profile: CardProfile{Name: "Demonic Tutor", IsTutor: true}, Qty: 1},
	}
	wla := ComputeWinLines(report, qtyProfiles, nil)
	if wla == nil || len(wla.WinLines) == 0 {
		t.Fatalf("expected at least one win line")
	}
	var infiniteLine *WinLine
	for i := range wla.WinLines {
		if wla.WinLines[i].Type == "infinite" {
			infiniteLine = &wla.WinLines[i]
			break
		}
	}
	if infiniteLine == nil {
		t.Fatalf("expected the Sanguine+Exquisite line to be present as infinite")
	}
	if infiniteLine.Confidence <= 0 {
		t.Fatalf("Confidence not stamped: %.3f", infiniteLine.Confidence)
	}
	if infiniteLine.Confidence > 1.0 {
		t.Fatalf("Confidence above ceiling: %.3f", infiniteLine.Confidence)
	}
}

// BACKEND-ONLY contract: Confidence must NOT appear in JSON, text, or
// markdown report output. The JSON jsonWinLine struct has no Confidence
// field, but we double-pin by marshaling and asserting the literal token
// is absent.
func TestWinLineConfidence_NotInJSONOutput(t *testing.T) {
	wla := &WinLineAnalysis{
		WinLines: []WinLine{
			{
				Pieces:     []string{"Thassa's Oracle", "Demonic Consultation"},
				Type:       "infinite",
				Confidence: 0.84,
			},
		},
	}
	jw := buildJSONWinLines(wla)
	bs, err := json.Marshal(jw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(bs), "confidence") {
		t.Fatalf("BACKEND-ONLY violated — confidence appeared in JSON: %s", string(bs))
	}
	if strings.Contains(string(bs), "0.84") {
		t.Fatalf("BACKEND-ONLY violated — confidence value leaked into JSON: %s", string(bs))
	}
}
