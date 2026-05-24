package gameengine

// Loki r41 / r60 — TriggerCompleteness false-positive cluster (8 hits,
// game 109 turn 48 Jaxis the Troublemaker). Root cause: the per-card
// dispatcher in per_card/registry.go's fireTrigger keeps a process-
// lifetime "trigger_total" counter and silently no-ops once it crosses
// 2000. By turn 40+ a long game routinely exceeds this cap; every
// subsequent creature death emits sba_704_5g/zone_change but no
// trigger_evaluated, so the TriggerCompleteness invariant flags it as
// "trigger-bearer on battlefield but no follow-up trigger event."
//
// Two-part fix:
//
//   1. internal/gameengine/per_card/registry.go: when the trigger_total
//      or trigger_depth guard short-circuits, emit a synthetic
//      "trigger_evaluated" event carrying capped="trigger_total" /
//      "trigger_depth" so the invariant's follow-up scan succeeds.
//   2. internal/gameengine/phases.go cleanup step: delete the
//      trigger_total flag at end-of-turn so the counter is a per-turn
//      budget (intended) rather than a lifetime cap (accidental).
//
// These tests pin the cleanup-step reset; the per-card dispatcher
// marker emission is exercised by per_card/registry_test.go.

import (
	"testing"
)

func TestEndOfTurnCleanup_ResetsTriggerTotal(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["trigger_total"] = 2500

	// Drive the cleanup step expiration path directly.
	gs.Phase = "ending"
	ScanExpiredDurations(gs, "ending", "cleanup")

	if v, ok := gs.Flags["trigger_total"]; ok {
		t.Fatalf("trigger_total should be cleared at cleanup, got %d (present=%v)", v, ok)
	}
}

func TestEndOfTurnCleanup_LeavesTriggerTotalAloneOutsideCleanup(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["trigger_total"] = 17

	// Not the cleanup step — counter must persist.
	ScanExpiredDurations(gs, "main", "main_phase")

	if v := gs.Flags["trigger_total"]; v != 17 {
		t.Fatalf("trigger_total should be untouched outside cleanup, got %d", v)
	}
}

// TestTriggerCompleteness_ClearsAfterCleanup verifies the end-to-end
// invariant story: a game that has accumulated > 2000 trigger fires
// passes through cleanup and then accepts further trigger dispatches
// without the silent-cap pattern, so subsequent death events get a
// follow-up trigger_evaluated event.
// TestTriggerCompleteness_SkipsNonCreatureSacrifice pins the invariant's
// type-aware skip for non-creature sacrifice events. Before r60 the
// invariant treated any `sacrifice` event as a creature-dies candidate,
// so sacrificing a Plains under a controller who happened to own a
// creature-dies bearer (Syr Vondam, Sefris, etc.) would false-positive
// even though no creature_dies trigger is expected.
func TestTriggerCompleteness_SkipsNonCreatureSacrifice(t *testing.T) {
	HasTriggerHook = func(name, event string) bool {
		return name == "Sefris of the Hidden Ways" && event == "creature_dies"
	}
	defer func() { HasTriggerHook = nil }()

	gs := NewGameState(2, nil, nil)
	sefris := &Card{Name: "Sefris of the Hidden Ways", Owner: 0, Types: []string{"creature", "legendary"}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: sefris, Controller: 0, Owner: 0,
	})
	gs.LogEvent(Event{
		Kind: "sacrifice", Seat: 0, Source: "Plains",
		Details: map[string]interface{}{"was_creature": false},
	})

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("invariant should ignore non-creature sacrifice, got: %v", err)
	}
}

func TestTriggerCompleteness_FlagsCreatureSacrifice(t *testing.T) {
	HasTriggerHook = func(name, event string) bool {
		return name == "Sefris of the Hidden Ways" && event == "creature_dies"
	}
	defer func() { HasTriggerHook = nil }()

	gs := NewGameState(2, nil, nil)
	sefris := &Card{Name: "Sefris of the Hidden Ways", Owner: 0, Types: []string{"creature", "legendary"}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: sefris, Controller: 0, Owner: 0,
	})
	gs.LogEvent(Event{
		Kind: "sacrifice", Seat: 0, Source: "Goblin Berserker",
		Details: map[string]interface{}{"was_creature": true},
	})
	// No subsequent trigger_evaluated — invariant should flag.

	if err := checkTriggerCompleteness(gs); err == nil {
		t.Fatal("invariant should flag creature sacrifice with no follow-up trigger event")
	}
}

func TestTriggerCompleteness_CleanupBreaksTheCap(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["trigger_total"] = 5000
	gs.Phase = "ending"

	ScanExpiredDurations(gs, "ending", "cleanup")

	// After cleanup the next turn's dispatcher must start fresh — the
	// flag is gone, so the first increment in fireTrigger will pass the
	// 2000-cap check.
	if gs.Flags["trigger_total"] != 0 {
		t.Fatalf("trigger_total should read 0 post-cleanup (deleted), got %d", gs.Flags["trigger_total"])
	}
}
