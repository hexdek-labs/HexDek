package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// -----------------------------------------------------------------------------
// Dralnu, Lich Lord
// -----------------------------------------------------------------------------

func TestDralnu_TapGrantsFlashbackToTarget(t *testing.T) {
	gs := newGame(t, 2)
	dralnu := addPerm(gs, 0, "Dralnu, Lich Lord", "creature", "legendary")
	bolt := &gameengine.Card{
		Name:  "Lightning Bolt",
		Owner: 0,
		Types: []string{"instant"},
		CMC:   1,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	gameengine.InvokeActivatedHook(gs, dralnu, 0, map[string]interface{}{
		"target_card": bolt,
	})

	if !dralnu.Tapped {
		t.Errorf("Dralnu should be tapped after activation")
	}
	if grant := gameengine.GetZoneCastGrant(gs, bolt); grant == nil {
		t.Errorf("Bolt should have a flashback grant after Dralnu activation")
	} else if grant.Keyword != "flashback" {
		t.Errorf("expected flashback keyword on grant, got %q", grant.Keyword)
	}
	if hasEvent(gs, "activated_flashback_grant") < 1 {
		t.Errorf("expected activated_flashback_grant event")
	}
}

func TestDralnu_AlreadyTappedFailsGracefully(t *testing.T) {
	gs := newGame(t, 2)
	dralnu := addPerm(gs, 0, "Dralnu, Lich Lord", "creature", "legendary")
	dralnu.Tapped = true
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	gameengine.InvokeActivatedHook(gs, dralnu, 0, map[string]interface{}{
		"target_card": bolt,
	})

	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when Dralnu already tapped")
	}
	if grant := gameengine.GetZoneCastGrant(gs, bolt); grant != nil {
		t.Errorf("should not grant flashback when activation fails")
	}
}

func TestDralnu_NoInstantOrSorceryInGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	dralnu := addPerm(gs, 0, "Dralnu, Lich Lord", "creature", "legendary")
	bear := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}, CMC: 2}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bear)

	gameengine.InvokeActivatedHook(gs, dralnu, 0, nil)

	if !dralnu.Tapped {
		t.Errorf("Dralnu should still be tapped (cost paid before effect)")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed when no i/s in graveyard")
	}
}

// -----------------------------------------------------------------------------
// Sapphire Collector
// -----------------------------------------------------------------------------

func TestSapphireCollector_PaysCostAndGrants(t *testing.T) {
	gs := newGame(t, 2)
	sapph := addPerm(gs, 0, "Sapphire Collector", "creature")
	gs.Seats[0].ManaPool = 5
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	gameengine.InvokeActivatedHook(gs, sapph, 0, map[string]interface{}{
		"target_card": bolt,
	})

	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected 2 mana left after paying {2}{U} (=3), got %d", gs.Seats[0].ManaPool)
	}
	if grant := gameengine.GetZoneCastGrant(gs, bolt); grant == nil {
		t.Errorf("Bolt should have a flashback grant after Sapphire Collector activation")
	}
}

func TestSapphireCollector_InsufficientManaFails(t *testing.T) {
	gs := newGame(t, 2)
	sapph := addPerm(gs, 0, "Sapphire Collector", "creature")
	gs.Seats[0].ManaPool = 1
	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt)

	gameengine.InvokeActivatedHook(gs, sapph, 0, map[string]interface{}{
		"target_card": bolt,
	})

	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("mana pool should not be touched on insufficient_mana fail, got %d", gs.Seats[0].ManaPool)
	}
	if grant := gameengine.GetZoneCastGrant(gs, bolt); grant != nil {
		t.Errorf("should not grant flashback when cost can't be paid")
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event")
	}
}

// -----------------------------------------------------------------------------
// Magus of the Will
// -----------------------------------------------------------------------------

func TestMagusOfTheWill_ExilesSelfAndGrantsAllInstantsAndSorceries(t *testing.T) {
	gs := newGame(t, 2)
	magus := addPerm(gs, 0, "Magus of the Will", "creature")
	gs.Seats[0].ManaPool = 5

	bolt := &gameengine.Card{Name: "Lightning Bolt", Owner: 0, Types: []string{"instant"}, CMC: 1}
	wrath := &gameengine.Card{Name: "Wrath of God", Owner: 0, Types: []string{"sorcery"}, CMC: 4}
	bear := &gameengine.Card{Name: "Grizzly Bears", Owner: 0, Types: []string{"creature"}, CMC: 2}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, bolt, wrath, bear)

	gameengine.InvokeActivatedHook(gs, magus, 0, nil)

	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected 2 mana left after paying {2}{B} (=3), got %d", gs.Seats[0].ManaPool)
	}
	// Magus should be exiled — gone from battlefield, present in exile.
	for _, p := range gs.Seats[0].Battlefield {
		if p == magus {
			t.Errorf("Magus should have been exiled off the battlefield")
		}
	}
	foundInExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == magus.Card {
			foundInExile = true
			break
		}
	}
	if !foundInExile {
		t.Errorf("Magus card should be in seat 0's exile")
	}

	// Both i/s in graveyard should have a flashback grant; bear should not.
	if grant := gameengine.GetZoneCastGrant(gs, bolt); grant == nil {
		t.Errorf("bolt should have a flashback grant after Magus")
	}
	if grant := gameengine.GetZoneCastGrant(gs, wrath); grant == nil {
		t.Errorf("wrath should have a flashback grant after Magus")
	}
	if grant := gameengine.GetZoneCastGrant(gs, bear); grant != nil {
		t.Errorf("creature card (bear) should NOT receive a flashback grant")
	}
	if hasEvent(gs, "activated_flashback_grant") < 1 {
		t.Errorf("expected activated_flashback_grant event")
	}
	if hasEvent(gs, "per_card_partial") < 2 {
		t.Errorf("expected at least 2 per_card_partial events for lands + replacement effect, got %d",
			hasEvent(gs, "per_card_partial"))
	}
}

// -----------------------------------------------------------------------------
// Registry smoke — all three handlers must be wired
// -----------------------------------------------------------------------------

func TestActivatedFlashbackGrant_AllThreeRegistered(t *testing.T) {
	for _, name := range []string{
		"Dralnu, Lich Lord",
		"Sapphire Collector",
		"Magus of the Will",
	} {
		if !HasActivated(name) {
			t.Errorf("expected activated handler registered for %q", name)
		}
	}
}
