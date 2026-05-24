package gameengine

// Loki r60 round 3 / game 4512 — Gerrard, Weatherlight Hero +
// Rest in Peace produces a TriggerCompleteness false-positive.
//
// Sequence (from the loki event log):
//   [565] replacement_applied seat=0 source=Rest in Peace target=seat0
//   [566] destroy seat=3 source=Firemane Commando
//   [567] sba_704_5g seat=3 source=Firemane Commando
//   [568] zone_change seat=3 source=Firemane Commando   ← to exile, not graveyard
//   [569] sba_cycle_complete seat=-1
//
// The invariant fires on event 567 (sba_704_5g, a death event), sees
// Gerrard on the battlefield with a registered creature_dies trigger,
// scans the next ~10 events for a follow-up trigger event, and finds
// none — because Gerrard's dies trigger correctly didn't fire (Rest
// in Peace's §614 replacement redirected the graveyard destination to
// exile, and per CR §700.4 "dies" means battlefield → graveyard
// specifically).
//
// Root cause: FireZoneChangeTriggers's dies-dispatch path is gated on
// `toZone == "graveyard"` — correctly. But when the gate fails the
// dispatcher silently produces NO event, so downstream observers
// (including the TriggerCompleteness invariant) can't tell the
// difference between "no trigger because none fired" and "no trigger
// because the engine forgot to fire one." Same false-positive class
// as the r60 sacrifice-of-non-creature filter and the trigger_total
// lifetime-cap silence.
//
// Fix: when fromZone == "battlefield" and toZone != "graveyard" for a
// creature, emit a synthetic `trigger_evaluated` event with
// skipped="destination_not_graveyard" and the actual to_zone. The
// invariant's existing follow-up scan accepts trigger_evaluated.

import (
	"testing"
)

func TestFireZoneChangeTriggers_CreatureExileRedirect_EmitsMarker(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	card := &Card{Name: "Firemane Commando", Owner: 0, Types: []string{"creature"}}
	perm := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}

	preCount := len(gs.EventLog)
	FireZoneChangeTriggers(gs, perm, card, "battlefield", "exile")

	var marker *Event
	for i := preCount; i < len(gs.EventLog); i++ {
		ev := &gs.EventLog[i]
		if ev.Kind == "trigger_evaluated" && ev.Source == "Firemane Commando" {
			if d := ev.Details; d != nil {
				if skip, _ := d["skipped"].(string); skip == "destination_not_graveyard" {
					marker = ev
					break
				}
			}
		}
	}
	if marker == nil {
		t.Fatalf("expected trigger_evaluated marker for creature destroyed→exile redirect, got events: %v", gs.EventLog[preCount:])
	}
	if to, _ := marker.Details["to_zone"].(string); to != "exile" {
		t.Errorf("marker to_zone should be %q, got %q", "exile", to)
	}
}

func TestFireZoneChangeTriggers_CreatureToGraveyard_NoSyntheticMarker(t *testing.T) {
	// The marker must ONLY appear when the destination was redirected.
	// A normal creature death (toZone == "graveyard") still fires the
	// real dies trigger and should NOT also emit the
	// destination_not_graveyard synthetic marker.
	gs := NewGameState(2, nil, nil)
	card := &Card{Name: "Plain Goblin", Owner: 0, Types: []string{"creature"}}
	perm := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}

	preCount := len(gs.EventLog)
	FireZoneChangeTriggers(gs, perm, card, "battlefield", "graveyard")

	for i := preCount; i < len(gs.EventLog); i++ {
		ev := &gs.EventLog[i]
		if ev.Kind == "trigger_evaluated" && ev.Details != nil {
			if skip, _ := ev.Details["skipped"].(string); skip == "destination_not_graveyard" {
				t.Fatalf("normal graveyard death must NOT emit destination_not_graveyard marker, got: %v", ev)
			}
		}
	}
}

func TestFireZoneChangeTriggers_NonCreatureExileRedirect_NoSyntheticMarker(t *testing.T) {
	// The marker is specifically for creatures whose dies-trigger should
	// have fired but didn't. A non-creature (artifact, enchantment) has
	// no dies trigger, so no marker needed.
	gs := NewGameState(2, nil, nil)
	card := &Card{Name: "Sol Ring", Owner: 0, Types: []string{"artifact"}}
	perm := &Permanent{Card: card, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}

	preCount := len(gs.EventLog)
	FireZoneChangeTriggers(gs, perm, card, "battlefield", "exile")

	for i := preCount; i < len(gs.EventLog); i++ {
		ev := &gs.EventLog[i]
		if ev.Kind == "trigger_evaluated" && ev.Details != nil {
			if skip, _ := ev.Details["skipped"].(string); skip == "destination_not_graveyard" {
				t.Fatalf("non-creature redirect must NOT emit destination_not_graveyard marker (no dies trigger to skip), got: %v", ev)
			}
		}
	}
}

// End-to-end: the invariant must accept the marker and not flag the
// Gerrard-style death-redirected-by-RIP scenario.
func TestTriggerCompleteness_AcceptsRedirectMarker(t *testing.T) {
	HasTriggerHook = func(name, event string) bool {
		return name == "Gerrard, Weatherlight Hero" && event == "creature_dies"
	}
	defer func() { HasTriggerHook = nil }()

	gs := NewGameState(2, nil, nil)
	gerrard := &Card{Name: "Gerrard, Weatherlight Hero", Owner: 0, Types: []string{"creature", "legendary"}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: gerrard, Controller: 0, Owner: 0,
	})

	gs.LogEvent(Event{Kind: "sba_704_5g", Seat: 0, Source: "Firemane Commando"})

	// Fire the redirect path — the synthetic marker should land in EventLog.
	dying := &Card{Name: "Firemane Commando", Owner: 0, Types: []string{"creature"}}
	dyingPerm := &Permanent{Card: dying, Controller: 0, Owner: 0}
	FireZoneChangeTriggers(gs, dyingPerm, dying, "battlefield", "exile")

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("invariant should accept the destination_not_graveyard marker as a valid follow-up, got: %v", err)
	}
}

// Negative control: without any follow-up event, the invariant still
// flags the missing trigger (pre-fix behavior on a real bug).
func TestTriggerCompleteness_NoMarkerNoTrigger_StillFlags(t *testing.T) {
	HasTriggerHook = func(name, event string) bool {
		return name == "Gerrard, Weatherlight Hero" && event == "creature_dies"
	}
	defer func() { HasTriggerHook = nil }()

	gs := NewGameState(2, nil, nil)
	gerrard := &Card{Name: "Gerrard, Weatherlight Hero", Owner: 0, Types: []string{"creature", "legendary"}}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: gerrard, Controller: 0, Owner: 0,
	})

	gs.LogEvent(Event{Kind: "sba_704_5g", Seat: 0, Source: "Some Creature"})
	// No follow-up trigger event of any kind — invariant must flag.

	if err := checkTriggerCompleteness(gs); err == nil {
		t.Fatal("invariant must still flag a death event with NO follow-up trigger event")
	}
}
