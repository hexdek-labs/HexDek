package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestLurkingPredators_PutsCreatureOnBattlefieldWhenOpponentCasts(t *testing.T) {
	gs := newGame(t, 2)
	lp := addPerm(gs, 0, "Lurking Predators", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Llanowar Elves", 0, "creature"),
	}
	lurkingPredatorsTrigger(gs, lp, map[string]interface{}{
		"caster_seat": 1,
		"spell_name":  "Lightning Bolt",
	})
	creatureOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Llanowar Elves" {
			creatureOnBF = true
		}
	}
	if !creatureOnBF {
		t.Fatalf("expected Llanowar Elves on battlefield, got %v", permNames(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Library) != 0 {
		t.Errorf("expected library empty after putting Elves onto battlefield, got %d", len(gs.Seats[0].Library))
	}
}

func TestLurkingPredators_PutsNoncreatureOnBottomOfLibrary(t *testing.T) {
	gs := newGame(t, 2)
	lp := addPerm(gs, 0, "Lurking Predators", "enchantment")
	noncreature := newCardWithTypes("Counterspell", 0, "instant")
	filler := newCardWithTypes("Forest", 0, "land")
	gs.Seats[0].Library = []*gameengine.Card{noncreature, filler}

	lurkingPredatorsTrigger(gs, lp, map[string]interface{}{"caster_seat": 1})

	if len(gs.Seats[0].Battlefield) != 1 { // just the LP itself
		t.Errorf("expected LP only on battlefield, got %v", permNames(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Library) != 2 {
		t.Fatalf("expected 2-card library after bottom, got %d", len(gs.Seats[0].Library))
	}
	if gs.Seats[0].Library[1].DisplayName() != "Counterspell" {
		t.Errorf("expected Counterspell at bottom (index 1), got %q", gs.Seats[0].Library[1].DisplayName())
	}
	if gs.Seats[0].Library[0].DisplayName() != "Forest" {
		t.Errorf("expected Forest now on top, got %q", gs.Seats[0].Library[0].DisplayName())
	}
}

func TestLurkingPredators_DoesNotFireOnOwnCast(t *testing.T) {
	gs := newGame(t, 2)
	lp := addPerm(gs, 0, "Lurking Predators", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("Llanowar Elves", 0, "creature")}
	lurkingPredatorsTrigger(gs, lp, map[string]interface{}{"caster_seat": 0})
	if len(gs.Seats[0].Battlefield) != 1 { // only the LP itself
		t.Errorf("LP must not fire on own controller's cast")
	}
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("library should be unchanged after own-cast no-op")
	}
}

func TestLurkingPredators_NoopOnEmptyLibrary(t *testing.T) {
	gs := newGame(t, 2)
	lp := addPerm(gs, 0, "Lurking Predators", "enchantment")
	lurkingPredatorsTrigger(gs, lp, map[string]interface{}{"caster_seat": 1})
	// No crash, no events.
}
