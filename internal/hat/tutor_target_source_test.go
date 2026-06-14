package hat

import "testing"

// TestTutorTargets_IncludeSynergyEngine pins the r63 source fix: a deck's
// value-engine (synergy) pieces must be surfaced as tutor targets, ranked
// after combo/win-line pieces. Before the fix, TutorTargets carried ONLY
// win-line pieces, so the hat's ChooseTarget tutor path (which scores only
// cards in TutorTargets) could never tutor up the deck's signature engine
// and fell back to generic card quality.
func TestTutorTargets_IncludeSynergyEngine(t *testing.T) {
	fj := &freyaJSON{
		Archetype: &freyaArchetype{Primary: "midrange"},
		WinLines: &freyaWinLines{
			Lines: []freyaWinLine{
				{Pieces: []string{"Combo Piece A", "Combo Piece B"}, Type: "infinite"},
			},
		},
		ValueChains: []freyaValueChain{
			{
				Name:        "landfall treasures",
				Steps:       []freyaValueChainStep{{Cards: []string{"Synergy Engine Creature"}}},
				BridgeCards: []string{"Bridge Enabler"},
			},
		},
	}

	sp := buildStrategyProfile(fj)

	// Combo pieces stay FIRST (assembling the win beats powering it).
	if len(sp.TutorTargets) < 2 ||
		sp.TutorTargets[0] != "Combo Piece A" || sp.TutorTargets[1] != "Combo Piece B" {
		t.Fatalf("combo/win-line pieces must lead TutorTargets; got %v", sp.TutorTargets)
	}

	// The synergy engine + bridge card (value-engine keys) must now appear
	// as tutor targets — previously they did not.
	want := map[string]bool{"Synergy Engine Creature": false, "Bridge Enabler": false}
	for _, tt := range sp.TutorTargets {
		if _, ok := want[tt]; ok {
			want[tt] = true
		}
	}
	for card, present := range want {
		if !present {
			t.Errorf("value-engine key %q missing from TutorTargets %v", card, sp.TutorTargets)
		}
	}

	// And they must be ranked AFTER the combo pieces, not before.
	idx := map[string]int{}
	for i, tt := range sp.TutorTargets {
		idx[tt] = i
	}
	if idx["Synergy Engine Creature"] < idx["Combo Piece B"] {
		t.Errorf("synergy engine ranked above combo piece: %v", sp.TutorTargets)
	}
}

// TestTutorTargets_NoComboStillSurfacesEngine: a deck with no win-line combo
// (pure value/synergy deck) still surfaces its engine pieces as tutor
// targets, so the hat tutors toward the deck's gameplan rather than generic
// good cards.
func TestTutorTargets_NoComboStillSurfacesEngine(t *testing.T) {
	fj := &freyaJSON{
		Archetype: &freyaArchetype{Primary: "midrange"},
		ValueChains: []freyaValueChain{
			{
				Name:  "engine",
				Steps: []freyaValueChainStep{{Cards: []string{"Engine Piece One", "Engine Piece Two"}}},
			},
		},
	}

	sp := buildStrategyProfile(fj)
	if len(sp.TutorTargets) == 0 {
		t.Fatal("value-only deck should still expose engine pieces as tutor targets")
	}
	got := map[string]bool{}
	for _, tt := range sp.TutorTargets {
		got[tt] = true
	}
	if !got["Engine Piece One"] || !got["Engine Piece Two"] {
		t.Errorf("engine pieces missing from TutorTargets %v", sp.TutorTargets)
	}
}
