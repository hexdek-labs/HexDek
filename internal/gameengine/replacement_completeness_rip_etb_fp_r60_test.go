package gameengine

// Loki r60 deep-stress / seed 271828 game 4773 —
// ReplacementCompleteness false-positive when Rest in Peace is cast and
// resolves on the SAME turn an earlier instant/sorcery has already
// resolved into the graveyard from the stack.
//
// Event trail (excerpt, len(EventLog) ~= 605):
//   [585] zone_change  seat=0 source=Rapier Wit  to_zone=graveyard
//   [586] resolve      seat=0 source=Rapier Wit
//   [587] pay_mana     seat=0 source=Rest in Peace
//   [588] cast         seat=0 source=Rest in Peace
//   [594] enter_battlefield seat=0 source=Rest in Peace
//
// At cleanup the invariant scans the last 20 events, sees Rest in Peace
// on the battlefield, and flags Rapier Wit's zone_change at event 585 —
// but Rest in Peace only entered at event 594, so its §614 graveyard
// replacement could not have been in effect. The existing destroy-event
// guard doesn't cover spells routed directly from stack → graveyard
// (no preceding `destroy` event).
//
// Fix: if Rest in Peace / Leyline of the Void has an `enter_battlefield`
// event LATER in the scan window than the zone_change in question,
// skip — the exile effect wasn't on the battlefield at the time.

import (
	"strings"
	"testing"
)

func TestReplacementCompleteness_RIPEnteredAfterZoneChange_NoFP(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card:       &Card{Name: "Rest in Peace", Owner: 0, Types: []string{"enchantment"}},
		Controller: 0, Owner: 0,
	})

	// Synthesize the deep-stress event trail.
	gs.LogEvent(Event{
		Kind: "zone_change", Seat: 0, Source: "Rapier Wit",
		Details: map[string]interface{}{"to_zone": "graveyard", "card": "Rapier Wit"},
	})
	gs.LogEvent(Event{Kind: "resolve", Seat: 0, Source: "Rapier Wit"})
	gs.LogEvent(Event{Kind: "pay_mana", Seat: 0, Source: "Rest in Peace"})
	gs.LogEvent(Event{Kind: "cast", Seat: 0, Source: "Rest in Peace"})
	gs.LogEvent(Event{Kind: "stack_push", Seat: 0, Source: "Rest in Peace"})
	gs.LogEvent(Event{Kind: "stack_resolve", Seat: 0, Source: "Rest in Peace"})
	gs.LogEvent(Event{Kind: "enter_battlefield", Seat: 0, Source: "Rest in Peace"})

	if err := checkReplacementCompleteness(gs); err != nil {
		t.Fatalf("RIP ETB after the zone_change must not fire ReplacementCompleteness — got: %v", err)
	}
}

func TestReplacementCompleteness_RIPEnteredBeforeZoneChange_StillFlags(t *testing.T) {
	// Negative control: a real skipped replacement (RIP ETB precedes the
	// zone_change, no replacement_applied event) must still flag.
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card:       &Card{Name: "Rest in Peace", Owner: 0, Types: []string{"enchantment"}},
		Controller: 0, Owner: 0,
	})

	gs.LogEvent(Event{Kind: "enter_battlefield", Seat: 0, Source: "Rest in Peace"})
	gs.LogEvent(Event{
		Kind: "zone_change", Seat: 0, Source: "Rapier Wit",
		Details: map[string]interface{}{"to_zone": "graveyard", "card": "Rapier Wit"},
	})

	err := checkReplacementCompleteness(gs)
	if err == nil {
		t.Fatal("RIP ETB BEFORE the zone_change with no replacement_applied must still fire ReplacementCompleteness")
	}
	if !strings.Contains(err.Error(), "Rapier Wit") {
		t.Errorf("error should name the offending card; got: %v", err)
	}
}

func TestReplacementCompleteness_LeylineEnteredAfterZoneChange_NoFP(t *testing.T) {
	// Same guard must apply to Leyline of the Void (opponent-only exile).
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card:       &Card{Name: "Leyline of the Void", Owner: 0, Types: []string{"enchantment"}},
		Controller: 0, Owner: 0,
	})

	gs.LogEvent(Event{
		Kind: "zone_change", Seat: 1, Source: "Lightning Bolt",
		Details: map[string]interface{}{"to_zone": "graveyard", "card": "Lightning Bolt", "controller": 1},
	})
	gs.LogEvent(Event{Kind: "enter_battlefield", Seat: 0, Source: "Leyline of the Void"})

	if err := checkReplacementCompleteness(gs); err != nil {
		t.Fatalf("Leyline ETB after the zone_change must not fire ReplacementCompleteness — got: %v", err)
	}
}
