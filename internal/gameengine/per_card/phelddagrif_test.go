package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestPhelddagrif_TrampleAndOpponentHippoToken(t *testing.T) {
	gs := newGame(t, 2)
	phel := addPerm(gs, 0, "Phelddagrif", "creature")
	phelddagrifActivate(gs, phel, 0, map[string]interface{}{"target_seat": 1})
	if phel.Flags["kw:trample"] != 1 {
		t.Errorf("expected trample granted UEOT, got %d", phel.Flags["kw:trample"])
	}
	if phel.Flags["trample_eot"] != 1 {
		t.Errorf("expected trample_eot cleanup marker")
	}
	found := false
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Hippo Token" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Hippo Token on opponent's battlefield, got %v", permNames(gs.Seats[1].Battlefield))
	}
}

func TestPhelddagrif_FlyingAndOpponentLifeGain(t *testing.T) {
	gs := newGame(t, 2)
	phel := addPerm(gs, 0, "Phelddagrif", "creature")
	preLife := gs.Seats[1].Life
	phelddagrifActivate(gs, phel, 1, map[string]interface{}{"target_seat": 1})
	if phel.Flags["kw:flying"] != 1 {
		t.Errorf("expected flying granted")
	}
	if gs.Seats[1].Life != preLife+2 {
		t.Errorf("expected opponent +2 life, got %d → %d", preLife, gs.Seats[1].Life)
	}
}

func TestPhelddagrif_BounceAndOpponentDraw(t *testing.T) {
	gs := newGame(t, 2)
	phel := addPerm(gs, 0, "Phelddagrif", "creature")
	gs.Seats[1].Library = []*gameengine.Card{newCardWithTypes("X", 1, "creature")}
	phelddagrifActivate(gs, phel, 2, map[string]interface{}{"target_seat": 1})
	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == phel {
			stillOnBF = true
		}
	}
	if stillOnBF {
		t.Errorf("Phelddagrif should have bounced off the battlefield")
	}
	if len(gs.Seats[1].Hand) != 1 {
		t.Errorf("expected opponent to draw 1, got %d cards in hand", len(gs.Seats[1].Hand))
	}
}

func TestPhelddagrif_DefaultsToFirstOpponentWhenTargetSeatMissing(t *testing.T) {
	gs := newGame(t, 3)
	phel := addPerm(gs, 0, "Phelddagrif", "creature")
	phelddagrifActivate(gs, phel, 0, map[string]interface{}{})
	// Seat 1 is the first opponent — Hippo should land there.
	foundSeat := -1
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p != nil && p.Card != nil && p.Card.DisplayName() == "Hippo Token" {
				foundSeat = i
			}
		}
	}
	if foundSeat != 1 {
		t.Errorf("expected Hippo on seat 1 (first opponent), got seat %d", foundSeat)
	}
}
