package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestFireTrigger_NoHandlersEmitsSyntheticTriggerEvaluated
// reproduces the TriggerCompleteness false-positive surfaced by the
// layer-stress 1000-game seed-42 sweep (PR #735): game 467 turn 47
// reported "TriggerCompleteness: death event sba_704_5f at index
// 2537 with trigger-bearer(s) [Daxos, Blessed by the Sun] on
// battlefield, but no subsequent trigger/effect event found."
//
// Root cause: fireTrigger's len(hitsBySeat) == 0 early-return path
// at registry.go:507-509 was the only return path in fireTrigger
// that did NOT emit a trigger_evaluated event. When
// checkTriggerCompleteness consults HasTriggerHook for a bearer
// on the battlefield but the actual onTrigger registry returns
// zero handlers (event-alias mismatch, name-canonicalisation
// gap, mid-dispatch unregistration), the invariant searches for
// a follow-up trigger event in the event log and finds none —
// false-positive.
//
// Fix: emit a synthetic trigger_evaluated event before the
// zero-hits return, matching the cap-trigger-depth (line
// 442-452) and cap-trigger-total (line 457-467) synthetic emits.
//
// This test fires an event for which NO per_card handlers exist
// against a clean registry, then asserts that a
// trigger_evaluated event was logged. Pre-fix, no such event
// fires.
func TestFireTrigger_NoHandlersEmitsSyntheticTriggerEvaluated(t *testing.T) {
	Reset() // fresh registry per test — no handlers registered

	gs := gameengine.NewGameState(2, nil, nil)
	// At least one permanent on the battlefield so fireTrigger
	// has something to iterate — without a permanent the loop
	// short-circuits before the registry lookup even runs.
	perm := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name:  "Daxos, Blessed by the Sun",
			Types: []string{"creature", "enchantment", "legendary"},
		},
		Controller: 0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	// Snapshot event log size + dispatch a creature_dies event for
	// which no per_card handlers exist (Reset wiped the registry).
	before := len(gs.EventLog)
	fireTrigger(gs, "creature_dies", map[string]interface{}{
		"seat": 0,
	})

	// Post-fix: a synthetic trigger_evaluated should be logged.
	foundSynthetic := false
	for _, ev := range gs.EventLog[before:] {
		if ev.Kind == "trigger_evaluated" {
			if ev.Details != nil {
				if capped, _ := ev.Details["capped"].(string); capped == "no_handlers" {
					foundSynthetic = true
					break
				}
			}
		}
	}
	if !foundSynthetic {
		t.Errorf("expected a trigger_evaluated event with capped=no_handlers after zero-hits dispatch, got none (pre-fix path leaves checkTriggerCompleteness with no follow-up event)")
	}
}

// TestFireTrigger_NoHandlersEmissionCarriesEventName verifies the
// synthetic trigger_evaluated event carries the originating event
// name in Details so log scanners (debug tools, the
// TriggerCompleteness invariant's follow-up search) can correlate
// the no-handlers marker with the specific event that produced it.
func TestFireTrigger_NoHandlersEmissionCarriesEventName(t *testing.T) {
	Reset()
	gs := gameengine.NewGameState(2, nil, nil)
	perm := &gameengine.Permanent{
		Card:       &gameengine.Card{Name: "Some Creature", Types: []string{"creature"}},
		Controller: 0,
		Timestamp:  gs.NextTimestamp(),
		Flags:      map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	before := len(gs.EventLog)
	fireTrigger(gs, "sacrifice", map[string]interface{}{"seat": 0})

	for _, ev := range gs.EventLog[before:] {
		if ev.Kind == "trigger_evaluated" && ev.Details != nil {
			if capped, _ := ev.Details["capped"].(string); capped == "no_handlers" {
				if evName, _ := ev.Details["event"].(string); evName != "sacrifice" {
					t.Errorf("synthetic event Details.event: want \"sacrifice\", got %q", evName)
				}
				if rule, _ := ev.Details["rule"].(string); rule != "603.3" {
					t.Errorf("synthetic event Details.rule: want \"603.3\", got %q", rule)
				}
				return
			}
		}
	}
	t.Errorf("expected a no_handlers synthetic trigger_evaluated event for sacrifice, got none")
}
