package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Timing+floor B5 gate tests. Each fixture hand-constructs a minimal
// DeckProfile carrying ComboTiming + InteractionFloor entries shaped
// to the test's bracket scenario, then exercises applyTimingFloorB5Gate
// directly. Hand-construction avoids the full BuildDeckProfile pipeline
// so the gate's behavior is isolated from the rest of the analyzer.
//
// Coverage: confirm path (cEDH-shaped combo present → B5 retained),
// demote paths (timing too slow, floor too high, neither), B4 deck
// untouched, combo-less deck demoted with diagnostic note, rubber-stamp
// override preserved, rationale signal appended for both directions.
// ---------------------------------------------------------------------------

func dpWithCombos(measured int, timingEntries []ComboTimingEstimate, floorEntries []ComboInteractionFloor) *DeckProfile {
	dp := &DeckProfile{
		MeasuredBracket:      measured,
		MeasuredBracketLabel: bracketLabelForTest(measured),
		Bracket:              measured,
		BracketLabel:         bracketLabelForTest(measured),
		BracketRationale:     &BracketRationale{},
	}
	if timingEntries != nil {
		dp.ComboTiming = &ComboTimingReport{PerCombo: timingEntries}
	}
	if floorEntries != nil {
		dp.InteractionFloor = &ComboInteractionFloorReport{PerCombo: floorEntries}
	}
	return dp
}

func bracketLabelForTest(b int) string {
	switch b {
	case 5:
		return "cEDH"
	case 4:
		return "Optimized"
	case 3:
		return "Upgraded"
	}
	return ""
}

// TestB5Gate_ConfirmFastCombo: B5 with a turn-2 combo at floor 1 stays B5.
func TestB5Gate_ConfirmFastCombo(t *testing.T) {
	dp := dpWithCombos(5,
		[]ComboTimingEstimate{
			{ComboIndex: 0, Label: "Thoracle + Consultation", EarliestTurn: 2},
			{ComboIndex: 1, Label: "Other + Other", EarliestTurn: 5},
		},
		[]ComboInteractionFloor{
			{ComboIndex: 0, Label: "Thoracle + Consultation", InteractionFloor: 1},
			{ComboIndex: 1, Label: "Other + Other", InteractionFloor: 3},
		},
	)
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 5 {
		t.Errorf("MeasuredBracket: got %d, want 5 (cEDH confirmed)", dp.MeasuredBracket)
	}
	if dp.Bracket != 5 {
		t.Errorf("Bracket: got %d, want 5", dp.Bracket)
	}
	if len(dp.BracketRationale.Signals) != 1 {
		t.Fatalf("expected 1 rationale signal, got %d", len(dp.BracketRationale.Signals))
	}
	sig := dp.BracketRationale.Signals[0]
	if sig.Kind != "gate" || sig.Name != "Timing+floor gate" {
		t.Errorf("rationale shape: got %+v", sig)
	}
	if !strings.Contains(sig.Note, "confirmed") {
		t.Errorf("rationale note should say confirmed, got: %s", sig.Note)
	}
	if !strings.Contains(sig.Note, "Thoracle") {
		t.Errorf("rationale note should name the qualifying combo, got: %s", sig.Note)
	}
}

// TestB5Gate_DemoteSlowCombo: B5 with only turn-7+ combos demoted to B4
// — the combos are too slow for cEDH play pattern.
func TestB5Gate_DemoteSlowCombo(t *testing.T) {
	dp := dpWithCombos(5,
		[]ComboTimingEstimate{
			{ComboIndex: 0, Label: "Slow1 + Slow2", EarliestTurn: 7},
			{ComboIndex: 1, Label: "Slow3 + Slow4", EarliestTurn: 8},
		},
		[]ComboInteractionFloor{
			{ComboIndex: 0, Label: "Slow1 + Slow2", InteractionFloor: 1},
			{ComboIndex: 1, Label: "Slow3 + Slow4", InteractionFloor: 1},
		},
	)
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4 (demoted)", dp.MeasuredBracket)
	}
	if dp.MeasuredBracketLabel != "Optimized" {
		t.Errorf("MeasuredBracketLabel: got %q, want \"Optimized\"", dp.MeasuredBracketLabel)
	}
	if dp.Bracket != 4 {
		t.Errorf("Bracket synced: got %d, want 4", dp.Bracket)
	}
	if len(dp.BracketRationale.Signals) != 1 {
		t.Fatalf("expected 1 rationale signal, got %d", len(dp.BracketRationale.Signals))
	}
	sig := dp.BracketRationale.Signals[0]
	if !strings.Contains(sig.Note, "demoted") {
		t.Errorf("rationale should say demoted, got: %s", sig.Note)
	}
	if !strings.Contains(sig.Note, "0/2 combos turn") {
		t.Errorf("rationale should diagnose timing miss, got: %s", sig.Note)
	}
}

// TestB5Gate_DemoteHighFloor: B5 with fast combos but high InteractionFloor
// (3+) demoted. The combos may be quick but the deck needs too many
// answers to defend them.
func TestB5Gate_DemoteHighFloor(t *testing.T) {
	dp := dpWithCombos(5,
		[]ComboTimingEstimate{
			{ComboIndex: 0, Label: "Fast1 + Fast2", EarliestTurn: 3},
			{ComboIndex: 1, Label: "Fast3 + Fast4", EarliestTurn: 4},
		},
		[]ComboInteractionFloor{
			{ComboIndex: 0, Label: "Fast1 + Fast2", InteractionFloor: 4},
			{ComboIndex: 1, Label: "Fast3 + Fast4", InteractionFloor: 3},
		},
	)
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4", dp.MeasuredBracket)
	}
	sig := dp.BracketRationale.Signals[0]
	if !strings.Contains(sig.Note, "0/2 combos floor") {
		t.Errorf("rationale should diagnose floor miss, got: %s", sig.Note)
	}
}

// TestB5Gate_RequireBothAxes: a combo with EarliestTurn=3 BUT floor=3
// doesn't qualify; another combo with EarliestTurn=6 BUT floor=1 also
// doesn't qualify. Need a SINGLE combo with BOTH. Demoted.
func TestB5Gate_RequireBothAxes(t *testing.T) {
	dp := dpWithCombos(5,
		[]ComboTimingEstimate{
			{ComboIndex: 0, Label: "FastButHigh + Pair", EarliestTurn: 3},
			{ComboIndex: 1, Label: "SlowButCheap + Pair", EarliestTurn: 6},
		},
		[]ComboInteractionFloor{
			{ComboIndex: 0, Label: "FastButHigh + Pair", InteractionFloor: 3}, // fast but high floor
			{ComboIndex: 1, Label: "SlowButCheap + Pair", InteractionFloor: 1}, // cheap but slow
		},
	)
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4 (no combo has BOTH fast AND cheap)",
			dp.MeasuredBracket)
	}
}

// TestB5Gate_B4Untouched: B4 deck is not touched by the gate.
func TestB5Gate_B4Untouched(t *testing.T) {
	dp := dpWithCombos(4,
		[]ComboTimingEstimate{{ComboIndex: 0, Label: "Slow + Slow", EarliestTurn: 7}},
		[]ComboInteractionFloor{{ComboIndex: 0, Label: "Slow + Slow", InteractionFloor: 5}},
	)
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4 (B4 should be untouched)",
			dp.MeasuredBracket)
	}
	if len(dp.BracketRationale.Signals) != 0 {
		t.Errorf("expected no rationale append for B4 input, got %d signals",
			len(dp.BracketRationale.Signals))
	}
}

// TestB5Gate_ComboLessDemoted: B5 deck with no combo data demoted (with
// diagnostic note that no combos were detected).
func TestB5Gate_ComboLessDemoted(t *testing.T) {
	dp := dpWithCombos(5, nil, nil)
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4", dp.MeasuredBracket)
	}
	if len(dp.BracketRationale.Signals) != 1 {
		t.Fatalf("expected 1 rationale signal, got %d", len(dp.BracketRationale.Signals))
	}
	if !strings.Contains(dp.BracketRationale.Signals[0].Note, "no combo timing+floor data") {
		t.Errorf("rationale should diagnose missing data, got: %s",
			dp.BracketRationale.Signals[0].Note)
	}
}

// TestB5Gate_EmptyComboListDemoted: B5 deck with empty ComboTiming
// (somehow reached B5 via card-list signals but produces no combos)
// — demoted with the "no combos detected" note.
func TestB5Gate_EmptyComboListDemoted(t *testing.T) {
	dp := dpWithCombos(5, []ComboTimingEstimate{}, []ComboInteractionFloor{})
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4", dp.MeasuredBracket)
	}
	if len(dp.BracketRationale.Signals) != 1 {
		t.Fatalf("expected 1 rationale signal, got %d", len(dp.BracketRationale.Signals))
	}
}

// TestB5Gate_PreservesRubberStampOverride: when dp.Bracket is set
// independently (rubber-stamp B2 for a WotC precon scenario), the gate
// must not sync Bracket — only MeasuredBracket changes.
func TestB5Gate_PreservesRubberStampOverride(t *testing.T) {
	dp := dpWithCombos(5,
		[]ComboTimingEstimate{{ComboIndex: 0, Label: "Slow + Slow", EarliestTurn: 7}},
		[]ComboInteractionFloor{{ComboIndex: 0, Label: "Slow + Slow", InteractionFloor: 5}},
	)
	// Rubber-stamp override: declared bracket is 2 even though measured
	// is 5 (would be).
	dp.Bracket = 2
	dp.BracketLabel = "Precon"
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4 (demoted)", dp.MeasuredBracket)
	}
	if dp.Bracket != 2 {
		t.Errorf("Bracket: got %d, want 2 (rubber-stamp override preserved)", dp.Bracket)
	}
	if dp.BracketLabel != "Precon" {
		t.Errorf("BracketLabel: got %q, want \"Precon\"", dp.BracketLabel)
	}
}

// TestB5Gate_NilSafe: nil dp / nil rationale don't panic.
func TestB5Gate_NilSafe(t *testing.T) {
	applyTimingFloorB5Gate(nil)
	// Should not panic.
	dp := &DeckProfile{MeasuredBracket: 5}
	// nil rationale + missing combo data — should demote without panicking.
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4", dp.MeasuredBracket)
	}
}

// TestB5Gate_MismatchedComboIndices: a combo timing entry with no
// matching InteractionFloor entry doesn't qualify (defensive against
// index drift between the two reports).
func TestB5Gate_MismatchedComboIndices(t *testing.T) {
	dp := dpWithCombos(5,
		[]ComboTimingEstimate{
			{ComboIndex: 0, Label: "Fast", EarliestTurn: 2},
		},
		[]ComboInteractionFloor{
			{ComboIndex: 99, Label: "Other", InteractionFloor: 1}, // index doesn't match
		},
	)
	applyTimingFloorB5Gate(dp)
	if dp.MeasuredBracket != 4 {
		t.Errorf("MeasuredBracket: got %d, want 4 (no aligned entry qualifies)",
			dp.MeasuredBracket)
	}
}

// TestB5Gate_BoundaryTurn4Floor2: edge of the qualifying window.
// EarliestTurn=4 AND floor=2 should qualify (≤4 and ≤2 are both
// inclusive). EarliestTurn=5 or floor=3 should not.
func TestB5Gate_BoundaryTurn4Floor2(t *testing.T) {
	cases := []struct {
		turn, floor int
		wantB5      bool
		desc        string
	}{
		{4, 2, true, "edge inside window: turn=4 floor=2 qualifies"},
		{4, 3, false, "edge outside floor: turn=4 floor=3 fails"},
		{5, 2, false, "edge outside turn: turn=5 floor=2 fails"},
		{1, 1, true, "deep inside window: turn=1 floor=1 qualifies"},
		{3, 1, true, "comfortably inside: turn=3 floor=1 qualifies"},
	}
	for _, c := range cases {
		dp := dpWithCombos(5,
			[]ComboTimingEstimate{{ComboIndex: 0, Label: "X + Y", EarliestTurn: c.turn}},
			[]ComboInteractionFloor{{ComboIndex: 0, Label: "X + Y", InteractionFloor: c.floor}},
		)
		applyTimingFloorB5Gate(dp)
		wantBracket := 4
		if c.wantB5 {
			wantBracket = 5
		}
		if dp.MeasuredBracket != wantBracket {
			t.Errorf("%s: got bracket %d, want %d", c.desc, dp.MeasuredBracket, wantBracket)
		}
	}
}
