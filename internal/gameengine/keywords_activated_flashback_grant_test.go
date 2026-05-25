package gameengine

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ActivatedFlashbackGrant primitive — single-target mode
// ---------------------------------------------------------------------------

func TestActivatedFlashbackGrant_ExplicitTarget(t *testing.T) {
	gs := newFlashbackGame(t)
	bolt := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	granted := ActivatedFlashbackGrant(gs, ActivatedFlashbackGrantOptions{
		Source: "Dralnu, Lich Lord",
		Seat:   0,
		Target: bolt,
	})
	if len(granted) != 1 || granted[0] != bolt {
		t.Fatalf("expected granted=[bolt], got %+v", granted)
	}
	if grant := GetZoneCastGrant(gs, bolt); grant == nil || grant.Keyword != "flashback" {
		t.Fatalf("expected flashback grant on Bolt, got %+v", grant)
	}
}

func TestActivatedFlashbackGrant_AutoPickHighestCMC(t *testing.T) {
	gs := newFlashbackGame(t)
	bolt := newInstantCard("Lightning Bolt", 0, 1)
	wrath := &Card{Name: "Wrath of God", Owner: 0, Types: []string{"sorcery", "cost:4"}, CMC: 4}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, wrath)

	granted := ActivatedFlashbackGrant(gs, ActivatedFlashbackGrantOptions{
		Source: "Dralnu, Lich Lord",
		Seat:   0,
	})
	if len(granted) != 1 || granted[0] != wrath {
		t.Fatalf("expected auto-pick to choose highest-CMC (Wrath), got %+v", granted)
	}
}

func TestActivatedFlashbackGrant_RejectsCreatureTarget(t *testing.T) {
	gs := newFlashbackGame(t)
	bolt := newInstantCard("Lightning Bolt", 0, 1)
	bear := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}, CMC: 2}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, bear)

	granted := ActivatedFlashbackGrant(gs, ActivatedFlashbackGrantOptions{
		Source: "Dralnu, Lich Lord",
		Seat:   0,
		Target: bear, // ineligible — falls back to auto-pick
	})
	if len(granted) != 1 || granted[0] != bolt {
		t.Fatalf("expected fallback to bolt, got %+v", granted)
	}
	if grant := GetZoneCastGrant(gs, bear); grant != nil {
		t.Fatal("creature should never receive a flashback grant")
	}
}

func TestActivatedFlashbackGrant_NoEligibleReturnsEmpty(t *testing.T) {
	gs := newFlashbackGame(t)
	bear := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}, CMC: 2}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bear)

	granted := ActivatedFlashbackGrant(gs, ActivatedFlashbackGrantOptions{
		Source: "Dralnu, Lich Lord",
		Seat:   0,
	})
	if len(granted) != 0 {
		t.Fatalf("expected empty grant set on graveyard with no instants/sorceries, got %+v", granted)
	}
}

// ---------------------------------------------------------------------------
// ActivatedFlashbackGrant primitive — AllInZone mode (Yawg Will family)
// ---------------------------------------------------------------------------

func TestActivatedFlashbackGrant_AllInZone(t *testing.T) {
	gs := newFlashbackGame(t)
	bolt := newInstantCard("Lightning Bolt", 0, 1)
	wrath := &Card{Name: "Wrath of God", Owner: 0, Types: []string{"sorcery", "cost:4"}, CMC: 4}
	bear := &Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}, CMC: 2}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, wrath, bear)

	granted := ActivatedFlashbackGrant(gs, ActivatedFlashbackGrantOptions{
		Source:    "Magus of the Will",
		Seat:      0,
		AllInZone: true,
	})
	if len(granted) != 2 {
		t.Fatalf("expected 2 grants (bolt + wrath), got %d: %+v", len(granted), granted)
	}
	// Creature must not have been granted.
	if grant := GetZoneCastGrant(gs, bear); grant != nil {
		t.Fatal("AllInZone should NOT grant flashback to creature cards")
	}
	if grant := GetZoneCastGrant(gs, bolt); grant == nil {
		t.Fatal("bolt should have a flashback grant in AllInZone mode")
	}
	if grant := GetZoneCastGrant(gs, wrath); grant == nil {
		t.Fatal("wrath should have a flashback grant in AllInZone mode")
	}
}

func TestActivatedFlashbackGrant_LogsEvent(t *testing.T) {
	gs := newFlashbackGame(t)
	bolt := newInstantCard("Lightning Bolt", 0, 1)
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	before := len(gs.EventLog)
	ActivatedFlashbackGrant(gs, ActivatedFlashbackGrantOptions{
		Source: "Dralnu, Lich Lord",
		Seat:   0,
		Target: bolt,
	})
	found := false
	for _, ev := range gs.EventLog[before:] {
		if ev.Kind == "activated_flashback_grant" && ev.Source == "Dralnu, Lich Lord" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected activated_flashback_grant event, got log tail %+v", gs.EventLog[before:])
	}
}

func TestActivatedFlashbackGrant_InvalidSeatNoPanic(t *testing.T) {
	gs := newFlashbackGame(t)
	got := ActivatedFlashbackGrant(gs, ActivatedFlashbackGrantOptions{Source: "X", Seat: 99})
	if len(got) != 0 {
		t.Fatalf("expected empty grants for invalid seat, got %+v", got)
	}
}
