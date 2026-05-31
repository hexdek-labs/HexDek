package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestLinSivvi_TutorRebelAtOrUnderX(t *testing.T) {
	gs := newGame(t, 2)
	lin := addPerm(gs, 0, "Lin Sivvi, Defiant Hero", "creature", "rebel")
	small := newCardWithTypes("Small Rebel", 0, "creature", "rebel", "cmc:1")
	mid := newCardWithTypes("Mid Rebel", 0, "creature", "rebel", "cmc:3")
	big := newCardWithTypes("Big Rebel", 0, "creature", "rebel", "cmc:5")
	gs.Seats[0].Library = []*gameengine.Card{small, mid, big}

	linSivviActivate(gs, lin, 0, map[string]interface{}{"x": 3})

	foundMid := false
	foundBig := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		switch p.Card.DisplayName() {
		case "Mid Rebel":
			foundMid = true
		case "Big Rebel":
			foundBig = true
		}
	}
	if !foundMid {
		t.Errorf("expected highest-eligible Mid Rebel (CMC 3) to enter, got %v", permNames(gs.Seats[0].Battlefield))
	}
	if foundBig {
		t.Errorf("Big Rebel (CMC 5) is above X=3 and must not be tutored")
	}
}

func TestLinSivvi_NoEligibleRebelEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	lin := addPerm(gs, 0, "Lin Sivvi, Defiant Hero", "creature", "rebel")
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Goblin", 0, "creature", "goblin", "cmc:1"),
	}
	linSivviActivate(gs, lin, 0, map[string]interface{}{"x": 5})
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_failed" {
			if d, ok := ev.Details["slug"].(string); ok && d == "lin_sivvi_tutor_rebel" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_failed for no_rebel_at_or_under_x")
	}
}

func TestLinSivvi_RebelMustBePermanent(t *testing.T) {
	gs := newGame(t, 2)
	lin := addPerm(gs, 0, "Lin Sivvi, Defiant Hero", "creature", "rebel")
	// Instant with Rebel subtype somehow — not a permanent card.
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Instant Rebel", 0, "instant", "rebel", "cmc:1"),
	}
	linSivviActivate(gs, lin, 0, map[string]interface{}{"x": 3})
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Instant Rebel" {
			t.Errorf("Lin Sivvi must only tutor Rebel PERMANENT cards")
		}
	}
}

func TestLinSivvi_BottomFromGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	lin := addPerm(gs, 0, "Lin Sivvi, Defiant Hero", "creature", "rebel")
	dead := newCardWithTypes("Defiant Falcon", 0, "creature", "rebel")
	gs.Seats[0].Graveyard = []*gameengine.Card{dead}
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("Top", 0, "creature")}
	linSivviActivate(gs, lin, 1, map[string]interface{}{"target_card": dead})
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected target removed from graveyard")
	}
	if len(gs.Seats[0].Library) != 2 || gs.Seats[0].Library[1] != dead {
		t.Errorf("expected target appended to library bottom (index 1)")
	}
}

func TestLinSivvi_BottomRefusesNonRebel(t *testing.T) {
	gs := newGame(t, 2)
	lin := addPerm(gs, 0, "Lin Sivvi, Defiant Hero", "creature", "rebel")
	nonRebel := newCardWithTypes("Llanowar Elves", 0, "creature", "elf")
	gs.Seats[0].Graveyard = []*gameengine.Card{nonRebel}
	linSivviActivate(gs, lin, 1, map[string]interface{}{"target_card": nonRebel})
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("non-Rebel must not be bottomed")
	}
}
