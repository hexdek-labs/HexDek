package main

import (
	"strings"
	"testing"
)

// minimalReport builds a FreyaReport scaffold with the role-count
// fields coaching reads. Tests overwrite the relevant counters
// (RemovalCount, NonLandTutorCount, etc.) per-case.
func minimalReport(roleCounts map[RoleTag]int, removalCount, tutorCount int) *FreyaReport {
	if roleCounts == nil {
		roleCounts = map[RoleTag]int{}
	}
	return &FreyaReport{
		Roles: &RoleAnalysis{
			RoleCounts: roleCounts,
			TotalCards: 99,
		},
		RemovalCount:      removalCount,
		NonLandTutorCount: tutorCount,
	}
}

// hasTipWithTitle returns the first tip whose Title contains substr,
// or nil. Used so tests can pin BOTH that a tip exists AND that its
// Detail/Action carry the named signal.
func hasTipWithTitle(tips []CoachingTip, substr string) *CoachingTip {
	for i := range tips {
		if strings.Contains(strings.ToLower(tips[i].Title), strings.ToLower(substr)) {
			return &tips[i]
		}
	}
	return nil
}

// TestCoaching_CriticalLandShortageIsTopPriority pins the highest-
// priority rule: a deck under its recommended land count gets the
// land-shortage tip at priority 10, ahead of every other suggestion.
// Without this, a stalled-mana deck would get told to "add tutors"
// before "add lands" — the wrong call.
func TestCoaching_CriticalLandShortageIsTopPriority(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Combo",
		Bracket:          4,
		AvgCMC:           3.2,
		LandCount:        32,
		RecommendedLands: 38,
		LandVerdict:      "too_few",
		ManaBaseGrade:    "C",
		RampCount:        4, // would also trigger ramp tip
		DrawCount:        4, // would also trigger draw tip
	}
	tips := computeCoachingTips(dp, minimalReport(nil, 3, 1))
	if len(tips) == 0 {
		t.Fatal("expected coaching tips, got none")
	}
	if tips[0].Category != "manabase" || tips[0].Priority != 10 {
		t.Errorf("top tip should be priority-10 manabase, got %+v", tips[0])
	}
	if !strings.Contains(tips[0].Title, "Add") || !strings.Contains(tips[0].Title, "land") {
		t.Errorf("land-shortage tip should headline the fix, got %q", tips[0].Title)
	}
}

// TestCoaching_BracketScalesTutorExpectations pins the bracket-aware
// branch: a combo deck at bracket 5 with 2 tutors gets a tutor tip;
// the same combo deck at bracket 2 does NOT — casual players don't
// need to be lectured about cEDH tutor density.
func TestCoaching_BracketScalesTutorExpectations(t *testing.T) {
	makeCombo := func(bracket int) *DeckProfile {
		return &DeckProfile{
			PrimaryArchetype: "Combo",
			Bracket:          bracket,
			AvgCMC:           2.8,
			LandCount:        36,
			RecommendedLands: 36,
			LandVerdict:      "ok",
			RampCount:        10, DrawCount: 11,
		}
	}
	cedh := computeCoachingTips(makeCombo(5), minimalReport(
		map[RoleTag]int{RoleBoardWipe: 1, RoleCounterspell: 8}, 8, 2))
	if hasTipWithTitle(cedh, "tutor") == nil {
		t.Errorf("bracket-5 combo with 2 tutors should be told to add tutors, got %v", titlesOf(cedh))
	}
	casual := computeCoachingTips(makeCombo(2), minimalReport(
		map[RoleTag]int{RoleBoardWipe: 1, RoleCounterspell: 4}, 6, 2))
	if hasTipWithTitle(casual, "tutor") != nil {
		t.Errorf("bracket-2 casual combo should NOT be told to add tutors, got %v", titlesOf(casual))
	}
}

// TestCoaching_NonComboArchetypeNoTutorLecture pins the archetype
// gating: an aggro/voltron deck at bracket 4 with 2 tutors does not
// get a tutor tip — tutors aren't the consistency mechanism for those
// archetypes. Only combo/storm decks get the tutor lecture.
func TestCoaching_NonComboArchetypeNoTutorLecture(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Voltron",
		Bracket:          4,
		AvgCMC:           2.5,
		LandCount:        36, RecommendedLands: 36, LandVerdict: "ok",
		RampCount: 10, DrawCount: 9,
	}
	tips := computeCoachingTips(dp, minimalReport(
		map[RoleTag]int{RoleBoardWipe: 1, RoleCounterspell: 0}, 10, 2))
	if hasTipWithTitle(tips, "tutor") != nil {
		t.Errorf("Voltron deck shouldn't be told to add tutors, got %v", titlesOf(tips))
	}
}

// TestCoaching_NoBoardWipeFires pins the zero-wipe rule: a deck with
// RoleBoardWipe=0 always gets the "add a board wipe" tip. Bracket
// affects priority but never silences the tip.
func TestCoaching_NoBoardWipeFires(t *testing.T) {
	for _, bracket := range []int{2, 3, 4, 5} {
		dp := &DeckProfile{
			PrimaryArchetype: "Midrange",
			Bracket:          bracket,
			LandCount:        36, RecommendedLands: 36, LandVerdict: "ok",
			RampCount: 10, DrawCount: 10,
		}
		tips := computeCoachingTips(dp, minimalReport(
			map[RoleTag]int{RoleBoardWipe: 0, RoleCounterspell: 3}, 10, 3))
		if hasTipWithTitle(tips, "board wipe") == nil {
			t.Errorf("bracket %d deck with zero board wipes should always get the tip, got %v",
				bracket, titlesOf(tips))
		}
	}
}

// TestCoaching_WellTunedDeckIsQuiet pins the "no fabricated noise"
// rule: a deck that clears every threshold returns 0 tips. Coaching
// intentionally does not pad to a minimum count — we'd rather under-
// surface than emit stylistic suggestions that aren't load-bearing.
func TestCoaching_WellTunedDeckIsQuiet(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Midrange",
		Bracket:          4,
		AvgCMC:           2.8,
		LandCount:        37, RecommendedLands: 37, LandVerdict: "ok",
		ManaBaseGrade:    "A",
		RampCount:        12, DrawCount: 12,
		WinLineCount:     3,
		SinglePointCount: 0,
		CommanderSynergy: 0.55,
		ProtectedKeyPieces: 4, UnprotectedKeyPieces: 0,
	}
	tips := computeCoachingTips(dp, minimalReport(
		map[RoleTag]int{RoleBoardWipe: 3, RoleCounterspell: 6}, 12, 4))
	if len(tips) > 0 {
		t.Errorf("well-tuned deck should return 0 coaching tips, got %d: %v",
			len(tips), titlesOf(tips))
	}
}

// TestCoaching_CapsAtFive pins the truncation rule: a deeply broken
// deck with 7+ candidate tips is truncated to 5 by descending
// priority. The Decks screen renders a fixed-height panel; more than
// 5 would overflow.
func TestCoaching_CapsAtFive(t *testing.T) {
	// Construct a deck that triggers every available rule.
	dp := &DeckProfile{
		PrimaryArchetype: "Combo",
		Bracket:          4,
		AvgCMC:           3.5,
		LandCount:        30, RecommendedLands: 38, LandVerdict: "too_few", // P10
		ManaBaseGrade:    "F",                                              // P8
		RampCount:        3, DrawCount: 3,                                  // P6, P7
		WinLineCount:     1,                                                // P7
		SinglePointCount: 2,                                                // P6
		CommanderSynergy: 0.10,                                             // P5
		ProtectedKeyPieces: 0, UnprotectedKeyPieces: 5,                     // P7
		CuttableCards: []CardQuality{                                       // P5
			{Name: "Bad Card 1"}, {Name: "Bad Card 2"}, {Name: "Bad Card 3"},
		},
	}
	tips := computeCoachingTips(dp, minimalReport(
		map[RoleTag]int{RoleBoardWipe: 0, RoleCounterspell: 0}, 2, 1)) // wipes + interaction + tutors
	if len(tips) > 5 {
		t.Errorf("coaching should cap at 5 tips, got %d: %v", len(tips), titlesOf(tips))
	}
	if len(tips) != 5 {
		t.Errorf("deeply broken deck should hit the 5-tip cap exactly, got %d", len(tips))
	}
	// Sorted descending by priority.
	for i := 1; i < len(tips); i++ {
		if tips[i].Priority > tips[i-1].Priority {
			t.Errorf("tips not sorted by priority desc: %d (%d) after %d (%d)",
				tips[i].Priority, i, tips[i-1].Priority, i-1)
		}
	}
	// Priority-10 manabase tip must be in the top 5 — it's the most
	// impactful and survives truncation.
	top := tips[0]
	if top.Category != "manabase" || top.Priority != 10 {
		t.Errorf("priority-10 manabase tip should be top, got %+v", top)
	}
}

// TestCoaching_CutTipReferencesCuttableCards pins the cut-tip
// integration with the existing CuttableCards analysis: when ≥3
// cuttable cards exist, the tip names them in the Action so the user
// has a concrete first cut, not a generic "trim something" line.
func TestCoaching_CutTipReferencesCuttableCards(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Midrange",
		Bracket:          3,
		LandCount:        36, RecommendedLands: 36, LandVerdict: "ok",
		RampCount: 10, DrawCount: 10,
		CuttableCards: []CardQuality{
			{Name: "Hedron Archive"}, {Name: "Aether Hub"},
			{Name: "Wayfarer's Bauble"}, {Name: "Cathartic Reunion"},
		},
	}
	tips := computeCoachingTips(dp, minimalReport(
		map[RoleTag]int{RoleBoardWipe: 1, RoleCounterspell: 2}, 8, 2))
	cut := hasTipWithTitle(tips, "underperformers")
	if cut == nil {
		t.Fatalf("expected a cut tip with ≥3 cuttable cards, got %v", titlesOf(tips))
	}
	if !strings.Contains(cut.Detail, "Hedron Archive") {
		t.Errorf("cut tip should name the top cuttable card in Detail, got %q", cut.Detail)
	}
	if !strings.Contains(cut.Action, "Hedron Archive") {
		t.Errorf("cut tip Action should name a concrete first cut, got %q", cut.Action)
	}
}

// TestCoaching_TipsCarryActionableNextStep pins that EVERY tip
// carries a non-empty Action string. The Decks screen renders Action
// as the "do this" arrow line; a tip without it is just a complaint.
func TestCoaching_TipsCarryActionableNextStep(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Combo",
		Bracket:          4,
		AvgCMC:           3.5,
		LandCount:        32, RecommendedLands: 38, LandVerdict: "too_few",
		ManaBaseGrade:    "D",
		RampCount:        5, DrawCount: 5,
		WinLineCount:     1,
		ProtectedKeyPieces: 0, UnprotectedKeyPieces: 4,
	}
	tips := computeCoachingTips(dp, minimalReport(
		map[RoleTag]int{RoleBoardWipe: 0, RoleCounterspell: 1}, 3, 2))
	if len(tips) == 0 {
		t.Fatal("expected tips, got none")
	}
	for _, t2 := range tips {
		if t2.Title == "" {
			t.Errorf("tip with empty Title: %+v", t2)
		}
		if t2.Action == "" {
			t.Errorf("tip %q has no Action — Coach surfaces Action as the do-this line, must be populated", t2.Title)
		}
		if t2.Priority < 1 || t2.Priority > 10 {
			t.Errorf("tip %q has out-of-range priority %d (want 1-10)", t2.Title, t2.Priority)
		}
		validCategory := map[string]bool{
			"cut": true, "add": true, "manabase": true, "ratio": true, "consistency": true,
		}
		if !validCategory[t2.Category] {
			t.Errorf("tip %q has invalid Category %q", t2.Title, t2.Category)
		}
	}
}

// TestCoaching_BracketScalesInteractionFloor pins that the interaction
// floor moves with bracket: 6 interaction passes B3 (floor 8 still
// triggers tip but at lower urgency) but the same 6 fails B5 (floor
// 12). The tip wording must reference the bracket explicitly so the
// user understands the target.
func TestCoaching_BracketScalesInteractionFloor(t *testing.T) {
	makeAt := func(bracket int) []CoachingTip {
		dp := &DeckProfile{
			PrimaryArchetype: "Midrange",
			Bracket:          bracket,
			LandCount:        36, RecommendedLands: 36, LandVerdict: "ok",
			RampCount: 10, DrawCount: 10,
		}
		return computeCoachingTips(dp, minimalReport(
			map[RoleTag]int{RoleBoardWipe: 2, RoleCounterspell: 3}, 3, 3)) // 6 interaction total
	}
	b3 := makeAt(3)
	if tip := hasTipWithTitle(b3, "interaction"); tip == nil {
		t.Errorf("6 interaction in a B3 deck should still trigger the interaction tip (floor=8)")
	}
	b5 := makeAt(5)
	if tip := hasTipWithTitle(b5, "interaction"); tip == nil {
		t.Errorf("6 interaction in a B5 deck must trigger the interaction tip (floor=12)")
	} else if !strings.Contains(tip.Detail, "5") {
		t.Errorf("B5 interaction tip should name the bracket in Detail, got %q", tip.Detail)
	}
}

func titlesOf(tips []CoachingTip) []string {
	out := make([]string, 0, len(tips))
	for _, t := range tips {
		out = append(out, t.Title)
	}
	return out
}
