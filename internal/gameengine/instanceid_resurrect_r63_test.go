package gameengine

import (
	"strings"
	"testing"
)

// r63 conservation firespot — orphan-sweep RESURRECTION.
//
// Root cause of the live-grinder zone_conservation cluster (Circuit Mender
// / Mesa Enchantress / Dire Tactics / Migration Path, each "present in
// zone … but not in (Minted − Ceased) — stale ceased entry"): the orphan
// sweep one-way-ceases any OG card transiently absent from every census
// zone (a multi-step op interrupted by an SBA pass running the sweep);
// when the card reappears its now-permanently-ceased ID trips the
// fabrication census forever. The resurrection pass un-ceases an OG card
// that is demonstrably present in a zone with its owner still in the game.

// TestResurrect_CeasedOGCardReappearing pins the fix: a minted OG card
// that was wrongly ceased while transiently absent is un-ceased once it
// is present again, clearing the fabrication census violation.
func TestResurrect_CeasedOGCardReappearing(t *testing.T) {
	gs := newPhase4GameState(t)
	card := &Card{Name: "Circuit Mender", Owner: 0, Colors: nil}
	MintOGInstanceID(gs, card)
	id := card.InstanceID
	if id == "" || id[2:4] != "OG" {
		t.Fatalf("expected OG-minted id, got %q", id)
	}
	// The card is present on the battlefield (it reappeared after the
	// transient window) ...
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: card, Controller: 0, Owner: 0,
	})
	// ... but the orphan sweep already ceased its ID during the window.
	MarkInstanceIDCeased(gs, id)

	// Pre-fix shape: census flags the present-but-ceased card.
	if err := checkZoneConservationByInstanceID(gs); err == nil {
		t.Fatalf("expected a fabrication/stale-ceased violation before resurrection")
	} else if !strings.Contains(err.Error(), "stale ceased entry") {
		t.Fatalf("unexpected violation: %v", err)
	}

	SweepOrphanedInstanceIDs(gs)

	if _, ceased := gs.CeasedInstanceIDs[id]; ceased {
		t.Fatalf("present OG card %q should have been resurrected (un-ceased)", id)
	}
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census should be clean after resurrection, got: %v", err)
	}
	// Audit event emitted.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "iid_orphan_resurrect" && ev.Details["instance_id"] == id {
			found = true
		}
	}
	if !found {
		t.Fatal("expected an iid_orphan_resurrect audit event")
	}
}

// TestResurrect_LibraryAndGrantZones covers the other reported zones: a
// resurrected card may reappear in the library (Migration Path) or the
// ZoneCastGrants sideband (Dire Tactics), not just the battlefield.
func TestResurrect_LibraryAndGrantZones(t *testing.T) {
	gs := newPhase4GameState(t)

	lib := &Card{Name: "Migration Path", Owner: 0}
	MintOGInstanceID(gs, lib)
	gs.Seats[0].Library = append(gs.Seats[0].Library, lib)
	MarkInstanceIDCeased(gs, lib.InstanceID)

	grant := &Card{Name: "Dire Tactics", Owner: 1}
	MintOGInstanceID(gs, grant)
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
	}
	gs.ZoneCastGrants[grant] = &ZoneCastPermission{}
	MarkInstanceIDCeased(gs, grant.InstanceID)

	SweepOrphanedInstanceIDs(gs)

	if _, ceased := gs.CeasedInstanceIDs[lib.InstanceID]; ceased {
		t.Fatalf("library card should be resurrected")
	}
	if _, ceased := gs.CeasedInstanceIDs[grant.InstanceID]; ceased {
		t.Fatalf("zone_cast_grants card should be resurrected")
	}
	if err := checkZoneConservationByInstanceID(gs); err != nil {
		t.Fatalf("census should be clean after resurrection, got: %v", err)
	}
}

// TestResurrect_OwnerLeftNotResurrected pins the §800.4a gate: a card
// owned by a player who has LEFT the game stays ceased even if a stale
// reference lingers in a surviving seat's zone — that is the §800.4a
// removal-completeness concern, NOT an over-cease, and must not be masked.
func TestResurrect_OwnerLeftNotResurrected(t *testing.T) {
	gs := newPhase4GameState(t)
	card := &Card{Name: "Stolen Creature", Owner: 1}
	MintOGInstanceID(gs, card)
	// Owner (seat 1) left the game; the card lingers on survivor seat 0's
	// battlefield (a §800.4a removal gap).
	gs.Seats[1].LeftGame = true
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, &Permanent{
		Card: card, Controller: 0, Owner: 1,
	})
	MarkInstanceIDCeased(gs, card.InstanceID)

	SweepOrphanedInstanceIDs(gs)

	if _, ceased := gs.CeasedInstanceIDs[card.InstanceID]; !ceased {
		t.Fatalf("owner-left card must stay ceased (not masked by resurrection)")
	}
}

// TestResurrect_GenuineOrphanStillCeased guards the existing sweep
// behavior: a truly-absent OG card is still ceased (resurrection must not
// suppress the orphan cease for absent cards).
func TestResurrect_GenuineOrphanStillCeased(t *testing.T) {
	gs := newPhase4GameState(t)
	orph := &Card{Name: "Lost Card", Owner: 0}
	MintOGInstanceID(gs, orph)
	// Not added to any zone.
	swept := SweepOrphanedInstanceIDs(gs)
	if swept != 1 {
		t.Fatalf("expected the absent orphan to be swept, got %d", swept)
	}
	if _, ceased := gs.CeasedInstanceIDs[orph.InstanceID]; !ceased {
		t.Fatalf("absent orphan should be ceased")
	}
}
