package gameengine

import (
	"testing"
)

// TestProjectAndApply_NoOpLoop_EmitsInfiniteCycleEvent pins the
// Huginn 2.0 cycle-detection hook: when projectAndApply breaks out
// of a no-op loop, the engine must emit a structured "infinite_cycle"
// event alongside the legacy loop_shortcut event, with cycle_length
// + participating_iids + participating_names + detected_by populated
// so analytics.DetectEngineCycles can ingest it without re-walking
// the stack-fingerprint history.
func TestProjectAndApply_NoOpLoop_EmitsInfiniteCycleEvent(t *testing.T) {
	gs := NewGameState(2, nil, nil)

	// Synthetic Worldgorger Dragon + Animate Dead infinite — period 2.
	// Each snapshot needs a stack-top *Card with a minted InstanceID so
	// the cycle extractor surfaces both participants.
	dragon := &Card{
		Name:       "Worldgorger Dragon",
		Owner:      0,
		InstanceID: "h0OGVR700008",
		Types:      []string{"creature"},
	}
	animate := &Card{
		Name:       "Animate Dead",
		Owner:      0,
		InstanceID: "h0OGVB400012",
		Types:      []string{"enchantment"},
	}
	// Seat the loop detector with alternating Dragon/Animate stack-top
	// snapshots, period 2, at least loopMinReps full repetitions.
	ld := newLoopDetector()
	for i := 0; i < loopMinReps*2; i++ {
		// Swap stack-top to the appropriate card for this iteration so
		// captureSnapshot picks up the right InstanceID.
		card := dragon
		if i%2 == 1 {
			card = animate
		}
		gs.Stack = []*StackItem{{Card: card, Controller: 0, ID: i + 1}}
		fp := stackTopFingerprint(gs)
		ld.record(gs, fp)
	}

	if !ld.projectAndApply(gs) {
		t.Fatal("expected projectAndApply to detect no-op loop and break")
	}

	// Find the infinite_cycle event in the log.
	var cycleEvent *Event
	for i := range gs.EventLog {
		if gs.EventLog[i].Kind == "infinite_cycle" {
			cycleEvent = &gs.EventLog[i]
			break
		}
	}
	if cycleEvent == nil {
		t.Fatal("expected infinite_cycle event in log; got none")
	}

	if cycleEvent.Source != "no_op_loop" {
		t.Errorf("Source: got %q, want no_op_loop", cycleEvent.Source)
	}
	if cycleEvent.Amount != 2 {
		t.Errorf("Amount (cycle_length): got %d, want 2", cycleEvent.Amount)
	}

	// cycle_length in Details should match Amount.
	if v, ok := cycleEvent.Details["cycle_length"].(int); !ok || v != 2 {
		t.Errorf("Details.cycle_length: got %v, want 2", cycleEvent.Details["cycle_length"])
	}

	// detected_by must tag the engine path.
	if v, ok := cycleEvent.Details["detected_by"].(string); !ok || v != "engine_no_op_loop" {
		t.Errorf("Details.detected_by: got %v, want engine_no_op_loop", cycleEvent.Details["detected_by"])
	}

	// participating_iids must be the deduped ordered InstanceID set
	// across the cycle's period — both Dragon and Animate Dead.
	iids, ok := cycleEvent.Details["participating_iids"].([]string)
	if !ok {
		t.Fatalf("Details.participating_iids missing or wrong type: %T", cycleEvent.Details["participating_iids"])
	}
	if len(iids) != 2 {
		t.Fatalf("participating_iids: got %v, want 2 entries", iids)
	}
	seen := map[string]bool{}
	for _, id := range iids {
		seen[id] = true
	}
	if !seen["h0OGVR700008"] || !seen["h0OGVB400012"] {
		t.Errorf("participating_iids: expected both Dragon and Animate Dead IIDs; got %v", iids)
	}

	// participating_names should align.
	names, ok := cycleEvent.Details["participating_names"].([]string)
	if !ok || len(names) != 2 {
		t.Fatalf("participating_names: got %v", cycleEvent.Details["participating_names"])
	}
	seenN := map[string]bool{}
	for _, n := range names {
		seenN[n] = true
	}
	if !seenN["Worldgorger Dragon"] || !seenN["Animate Dead"] {
		t.Errorf("participating_names: expected both Dragon and Animate Dead; got %v", names)
	}

	// Sanity: the legacy loop_shortcut event should also still be emitted.
	hasLegacy := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "loop_shortcut" && ev.Source == "no_op_loop" {
			hasLegacy = true
			break
		}
	}
	if !hasLegacy {
		t.Error("legacy loop_shortcut event missing — must be emitted alongside infinite_cycle for backward compat")
	}
}

// TestExtractCycleIIDs_DedupAndOrder confirms the helper that the
// projectAndApply event-emit calls behaves correctly: dedup by first-
// seen, preserve order, filter empty (legacy/unminted) entries.
func TestExtractCycleIIDs_DedupAndOrder(t *testing.T) {
	snapshots := []loopSnapshot{
		{stackTopIID: "h0OGVR700008", stackTopName: "Dragon"},
		{stackTopIID: "h0OGVB400012", stackTopName: "Animate"},
		{stackTopIID: "h0OGVR700008", stackTopName: "Dragon"}, // dup, suppressed
		{stackTopIID: "", stackTopName: "Unminted"},           // filtered
		{stackTopIID: "h0OGVB400012", stackTopName: "Animate"}, // dup, suppressed
	}
	iids, names := extractCycleIIDs(snapshots, 5)
	if len(iids) != 2 {
		t.Fatalf("expected 2 deduped IIDs, got %v", iids)
	}
	if iids[0] != "h0OGVR700008" || iids[1] != "h0OGVB400012" {
		t.Errorf("order not preserved: got %v", iids)
	}
	if names[0] != "Dragon" || names[1] != "Animate" {
		t.Errorf("name alignment broken: got %v", names)
	}
}

func TestExtractCycleIIDs_EmptyInputs(t *testing.T) {
	if iids, names := extractCycleIIDs(nil, 0); iids != nil || names != nil {
		t.Errorf("nil snapshots: got iids=%v names=%v, want nil/nil", iids, names)
	}
	if iids, _ := extractCycleIIDs([]loopSnapshot{{stackTopIID: "x"}}, 5); iids != nil {
		t.Errorf("period > len(snapshots) should return nil; got %v", iids)
	}
}
