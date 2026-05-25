package gameengine

import (
	"testing"
)

// Loki r60 round 1-3 residual signature (bit-stable seed-42 game 4512,
// turn 22 cleanup, event 567):
//
//   "TriggerCompleteness: death event "sba_704_5g" at index 567 with
//   trigger-bearer(s) [{Gerrard, Weatherlight Hero 3}] on battlefield,
//   but no subsequent trigger/effect event found"
//
// Root cause: Rest in Peace (on seat 0) replaces "would be put into a
// graveyard from anywhere" with "exile it instead". A creature on seat 3
// (Firemane Commando) dies to lethal combat damage; destroyPermSBA's
// §614 chain (FireDieEvent) lets Rest in Peace flip the destination to
// exile. The resulting sba_704_5g event has Details["to_zone"]="exile".
// Per CR §700.4 "dies" = battlefield → GRAVEYARD only — when the
// destination is exile, no creature_dies trigger fires (this is
// rules-correct).
//
// Gerrard, Weatherlight Hero is on seat 3 and HAS a creature_dies
// trigger ("When Gerrard dies, exile it and return…"). The
// TriggerCompleteness invariant scans recent death events and demands
// a follow-up trigger event when a trigger-bearer is on the same seat.
// It didn't check Details["to_zone"], so the graveyard-redirected death
// false-positived: there's no creature_dies follow-up because the
// rules say there shouldn't be one.
//
// Same anti-pattern as the existing "skip non-creature sacrifice events"
// guard added in the 2026-05-24 TriggerCompleteness resolved entry.
//
// Fix: checkTriggerCompleteness now skips death events whose
// Details["to_zone"] is anything other than "" or "graveyard".

func TestTriggerCompleteness_SkipsRestInPeaceExileRedirect(t *testing.T) {
	gs := newFixtureGame(t)

	// Gerrard on seat 0 (a trigger-bearer with creature_dies dispatch).
	// We register the per_card hook indirectly by setting HasTriggerHook;
	// the test verifies the invariant ITSELF doesn't trip regardless of
	// whether the dispatcher fired.
	addBattlefield(gs, 0, "Gerrard, Weatherlight Hero", 3, 3, "creature", "legendary")
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Gerrard, Weatherlight Hero" && event == "creature_dies"
	}
	t.Cleanup(func() { HasTriggerHook = nil })

	// Synthesize the event log shape Rest in Peace produces: a
	// destroy + sba_704_5g + zone_change sequence where to_zone="exile".
	gs.EventLog = []Event{
		{Kind: "damage", Seat: 0, Source: "Firemane Commando", Amount: 4},
		{Kind: "replacement_applied", Seat: 0, Source: "Rest in Peace"},
		{Kind: "destroy", Seat: 0, Source: "Firemane Commando", Details: map[string]interface{}{
			"to_zone": "exile",
			"rule":    "701.7",
		}},
		{Kind: "sba_704_5g", Seat: 0, Source: "Firemane Commando", Details: map[string]interface{}{
			"to_zone": "exile",
			"rule":    "704.5g",
			"reason":  "lethal_damage",
		}},
		{Kind: "zone_change", Seat: 0, Source: "Firemane Commando", Details: map[string]interface{}{
			"from_zone": "battlefield",
			"to_zone":   "exile",
		}},
	}

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("invariant must not flag a §614 graveyard-redirected death: %v", err)
	}
}

// TestTriggerCompleteness_StillFlagsGraveyardDeathWithoutTrigger guards
// the negative case: a genuine death-to-graveyard with no follow-up
// trigger event MUST still trip the invariant.
func TestTriggerCompleteness_StillFlagsGraveyardDeathWithoutTrigger(t *testing.T) {
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Blood Artist", 0, 1, "creature")
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Blood Artist" && event == "creature_dies"
	}
	t.Cleanup(func() { HasTriggerHook = nil })

	// Genuine death to graveyard with no subsequent trigger event.
	gs.EventLog = []Event{
		{Kind: "destroy", Seat: 0, Source: "Bear", Details: map[string]interface{}{
			"to_zone": "graveyard",
		}},
		{Kind: "sba_704_5g", Seat: 0, Source: "Bear", Details: map[string]interface{}{
			"to_zone": "graveyard",
			"rule":    "704.5g",
		}},
	}

	if err := checkTriggerCompleteness(gs); err == nil {
		t.Fatalf("invariant must STILL flag a graveyard death with no trigger follow-up")
	}
}

// TestTriggerCompleteness_AcceptsGraveyardDeathWithTrigger guards the
// happy path: graveyard death + subsequent trigger_evaluated = clean.
func TestTriggerCompleteness_AcceptsGraveyardDeathWithTrigger(t *testing.T) {
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Blood Artist", 0, 1, "creature")
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Blood Artist" && event == "creature_dies"
	}
	t.Cleanup(func() { HasTriggerHook = nil })

	gs.EventLog = []Event{
		{Kind: "destroy", Seat: 0, Source: "Bear", Details: map[string]interface{}{
			"to_zone": "graveyard",
		}},
		{Kind: "sba_704_5g", Seat: 0, Source: "Bear", Details: map[string]interface{}{
			"to_zone": "graveyard",
		}},
		{Kind: "trigger_evaluated", Seat: 0, Source: "Blood Artist"},
	}

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("invariant must accept graveyard death with follow-up trigger: %v", err)
	}
}

// TestTriggerCompleteness_SkipsAnafenzaCommandZoneRedirect — a sibling
// scenario where Anafenza the Foremost exiles an OPPONENT'S dying
// creature. The destination is "exile" too, same guard applies.
func TestTriggerCompleteness_SkipsAnafenzaCommandZoneRedirect(t *testing.T) {
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Gerrard, Weatherlight Hero", 3, 3, "creature", "legendary")
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Gerrard, Weatherlight Hero" && event == "creature_dies"
	}
	t.Cleanup(func() { HasTriggerHook = nil })

	gs.EventLog = []Event{
		{Kind: "replacement_applied", Seat: 0, Source: "Anafenza, the Foremost"},
		{Kind: "sba_704_5g", Seat: 0, Source: "Opp Bear", Details: map[string]interface{}{
			"to_zone": "exile",
		}},
	}

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("invariant must not flag an Anafenza-redirected death: %v", err)
	}
}

// TestTriggerCompleteness_SkipsLeylineOfTheVoidExileRedirect — the
// other canonical §614 graveyard-replacement card.
func TestTriggerCompleteness_SkipsLeylineOfTheVoidExileRedirect(t *testing.T) {
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Gerrard, Weatherlight Hero", 3, 3, "creature", "legendary")
	HasTriggerHook = func(cardName, event string) bool {
		return cardName == "Gerrard, Weatherlight Hero" && event == "creature_dies"
	}
	t.Cleanup(func() { HasTriggerHook = nil })

	gs.EventLog = []Event{
		{Kind: "replacement_applied", Seat: 0, Source: "Leyline of the Void"},
		{Kind: "sba_704_5g", Seat: 0, Source: "Opp Bear", Details: map[string]interface{}{
			"to_zone": "exile",
		}},
	}

	if err := checkTriggerCompleteness(gs); err != nil {
		t.Fatalf("invariant must not flag a Leyline-redirected death: %v", err)
	}
}
