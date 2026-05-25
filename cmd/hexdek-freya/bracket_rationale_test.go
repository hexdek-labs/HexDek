package main

import (
	"testing"
)

// TestBracketRationale_ScoringSignalsRecorded verifies each scoring
// signal produces a BracketSignal entry with Kind="score" and the
// correct point contribution and measurement string.
func TestBracketRationale_ScoringSignalsRecorded(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(5),
	}
	ctx := &classifyContext{
		roleRatios:           map[RoleTag]float64{RoleLand: 0.28, RoleCounterspell: 0.08},
		avgCMC:               1.9,
		tutorDensity:         0.13,
		comboCount:           3,
		fastManaCount:        12,
		gameChangerCount:     10,
		gameChangerNames:     []string{"Mana Crypt", "Force of Will", "Demonic Tutor"},
		freeInteractionCount: 4,
		freeInteractionNames: []string{"Force of Will", "Mental Misstep"},
	}

	_, _, br := estimateBracket(ctx, report)
	if br == nil {
		t.Fatalf("expected non-nil rationale")
	}

	// Build a map of signal name → entry for assertions
	byName := map[string]BracketSignal{}
	for _, s := range br.Signals {
		if s.Kind == "score" {
			byName[s.Name] = s
		}
	}

	cases := []struct {
		name             string
		wantContribution int
		wantTier         string
	}{
		{"Game Changers", 4, "8+"},
		{"Tutor density", 3, "12%+"},
		{"Combo lines", 2, "2-4"},
		{"Average CMC", 2, "lean (<2.0)"},
		{"Fast mana", 3, "10+"},
		{"Counterspell density", 1, "6%+"},
		{"Land ratio", 1, "<30%"},
		{"Finisher density", 1, "4-7"},
		{"Free interaction", 3, "4+"},
	}
	for _, tc := range cases {
		sig, ok := byName[tc.name]
		if !ok {
			t.Errorf("missing scoring signal %q", tc.name)
			continue
		}
		if sig.Contribution != tc.wantContribution {
			t.Errorf("%s contribution: want %+d, got %+d", tc.name, tc.wantContribution, sig.Contribution)
		}
		if sig.Tier != tc.wantTier {
			t.Errorf("%s tier: want %q, got %q", tc.name, tc.wantTier, sig.Tier)
		}
		if sig.Measurement == "" {
			t.Errorf("%s missing measurement", tc.name)
		}
	}

	// Raw score = sum of contributions = 4+3+2+2+3+1+1+1+3 = 20
	if br.RawScore != 20 {
		t.Errorf("raw score: want 20, got %d", br.RawScore)
	}
	if br.FinalBracket != 5 || br.FinalLabel != "cEDH" {
		t.Errorf("final: want B5 cEDH, got B%d %s", br.FinalBracket, br.FinalLabel)
	}
}

// TestBracketRationale_EvidenceCardsAttached verifies the curated-list
// signals (Game Changers, Free interaction) carry the actual card
// names so report readers can see WHICH cards drove the call.
func TestBracketRationale_EvidenceCardsAttached(t *testing.T) {
	report := &FreyaReport{Roles: &RoleAnalysis{RoleCounts: map[RoleTag]int{}}}
	ctx := &classifyContext{
		roleRatios:           map[RoleTag]float64{},
		avgCMC:               2.4,
		gameChangerCount:     3,
		gameChangerNames:     []string{"Cyclonic Rift", "Smothering Tithe", "Rhystic Study"},
		freeInteractionCount: 2,
		freeInteractionNames: []string{"Force of Will", "Mental Misstep"},
	}
	_, _, br := estimateBracket(ctx, report)

	var gcSig, freeSig *BracketSignal
	for i := range br.Signals {
		switch br.Signals[i].Name {
		case "Game Changers":
			gcSig = &br.Signals[i]
		case "Free interaction":
			freeSig = &br.Signals[i]
		}
	}
	if gcSig == nil {
		t.Fatalf("missing Game Changers signal")
	}
	if len(gcSig.Evidence) != 3 {
		t.Errorf("GC evidence: want 3 cards, got %d (%v)", len(gcSig.Evidence), gcSig.Evidence)
	}
	if freeSig == nil {
		t.Fatalf("missing Free interaction signal")
	}
	if len(freeSig.Evidence) != 2 {
		t.Errorf("free-int evidence: want 2 cards, got %d (%v)", len(freeSig.Evidence), freeSig.Evidence)
	}
}

// TestBracketRationale_GateDemotionRecorded verifies the B5 gate logs
// an adjustment entry with Kind="gate" when it demotes a high-score
// deck to B4 for missing the cEDH markers.
func TestBracketRationale_GateDemotionRecorded(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(8),
	}
	// Score >= 12 but no free interaction, tutors < 12%, GC < 8.
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{RoleLand: 0.28},
		avgCMC:           2.4,
		tutorDensity:     0.08,
		comboCount:       2,
		fastManaCount:    10,
		gameChangerCount: 4,
	}
	bracket, _, br := estimateBracket(ctx, report)
	if bracket != 4 {
		t.Fatalf("expected demotion to B4, got B%d", bracket)
	}

	var gate *BracketSignal
	for i := range br.Signals {
		if br.Signals[i].Kind == "gate" {
			gate = &br.Signals[i]
			break
		}
	}
	if gate == nil {
		t.Fatalf("expected B5 gate adjustment in rationale, got: %+v", br.Signals)
	}
	if gate.Name != "B5 gate" {
		t.Errorf("gate name: want %q, got %q", "B5 gate", gate.Name)
	}
	if gate.Note == "" {
		t.Errorf("gate note should explain demotion reason")
	}
}

// TestBracketRationale_FloorLiftRecorded verifies a floor-lift (tuned
// redundancy promoting B3→B4) gets recorded as Kind="floor" with the
// pre-lift bracket and triggering measurements in the Note.
func TestBracketRationale_FloorLiftRecorded(t *testing.T) {
	report := &FreyaReport{
		Roles:     &RoleAnalysis{TotalCards: 99, RoleCounts: map[RoleTag]int{}},
		Finishers: makeComboList(10),
	}
	// Score should land in B3 territory; finishers + fast mana lift to B4.
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		avgCMC:           2.6,
		fastManaCount:    8,
		gameChangerCount: 1,
	}
	bracket, _, br := estimateBracket(ctx, report)
	if bracket != 4 {
		t.Fatalf("expected tuned-redundancy floor to B4, got B%d", bracket)
	}
	foundFloor := false
	for _, sig := range br.Signals {
		if sig.Kind == "floor" && sig.Name == "Tuned-redundancy floor" {
			foundFloor = true
			break
		}
	}
	if !foundFloor {
		t.Errorf("expected tuned-redundancy floor adjustment, got: %+v", br.Signals)
	}
}
