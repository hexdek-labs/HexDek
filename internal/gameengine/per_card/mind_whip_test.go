package per_card

import (
	"testing"
)

func TestMindWhip_EnchantedControllersUpkeep_DamageAndTap(t *testing.T) {
	gs := newGame(t, 2)
	victim := addPerm(gs, 1, "Serra Angel", "creature", "angel")
	whip := addPerm(gs, 0, "Mind Whip", "enchantment", "aura")
	whip.AttachedTo = victim
	gs.Seats[1].Life = 40

	mindWhipUpkeep(gs, whip, map[string]interface{}{"active_seat": 1})

	if gs.Seats[1].Life != 38 {
		t.Errorf("victim should lose 2 life (40→38), got %d", gs.Seats[1].Life)
	}
	if !victim.Tapped {
		t.Error("enchanted creature should be tapped")
	}
}

func TestMindWhip_OtherPlayersUpkeep_NoEffect(t *testing.T) {
	gs := newGame(t, 2)
	victim := addPerm(gs, 1, "Serra Angel", "creature", "angel")
	whip := addPerm(gs, 0, "Mind Whip", "enchantment", "aura")
	whip.AttachedTo = victim
	gs.Seats[1].Life = 40

	// Aura controller's (seat 0) upkeep — not the enchanted creature's
	// controller, so nothing happens.
	mindWhipUpkeep(gs, whip, map[string]interface{}{"active_seat": 0})

	if gs.Seats[1].Life != 40 {
		t.Errorf("victim life must be untouched on a non-controller upkeep, got %d", gs.Seats[1].Life)
	}
	if victim.Tapped {
		t.Error("enchanted creature must not be tapped on a non-controller upkeep")
	}
}

func TestMindWhip_Unattached_NoPanic(t *testing.T) {
	gs := newGame(t, 2)
	whip := addPerm(gs, 0, "Mind Whip", "enchantment", "aura")
	mindWhipUpkeep(gs, whip, map[string]interface{}{"active_seat": 0}) // AttachedTo nil — must no-op
}
