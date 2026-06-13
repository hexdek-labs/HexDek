package per_card

import (
	"testing"
)

func TestKarona_EachUpkeep_GainsControlAndUntaps(t *testing.T) {
	gs := newGame(t, 2)
	karona := addPerm(gs, 0, "Karona, False God", "creature", "avatar")
	karona.Tapped = true

	// Seat 1's upkeep: seat 1 untaps Karona and gains control of it.
	karonaUpkeep(gs, karona, map[string]interface{}{"active_seat": 1})

	if karona.Controller != 1 {
		t.Fatalf("Karona should be controlled by seat 1 after their upkeep, got %d", karona.Controller)
	}
	if karona.Tapped {
		t.Error("Karona should be untapped by the upkeep trigger")
	}
	if permOnSeatBF(gs, 0, karona) {
		t.Error("Karona must be removed from seat 0's battlefield on control change")
	}
	if !permOnSeatBF(gs, 1, karona) {
		t.Error("Karona must be on seat 1's battlefield after control change")
	}
}

func TestKarona_ControllersOwnUpkeep_JustUntaps(t *testing.T) {
	gs := newGame(t, 2)
	karona := addPerm(gs, 0, "Karona, False God", "creature", "avatar")
	karona.Tapped = true

	karonaUpkeep(gs, karona, map[string]interface{}{"active_seat": 0})

	if karona.Controller != 0 {
		t.Errorf("Karona should remain under seat 0 on seat 0's own upkeep, got %d", karona.Controller)
	}
	if karona.Tapped {
		t.Error("Karona should be untapped on its controller's upkeep")
	}
	if !permOnSeatBF(gs, 0, karona) {
		t.Error("Karona should still be on seat 0's battlefield")
	}
}
