package per_card

import (
	"testing"
)

func TestYeva_ETBSetsFlashPermissionFlag(t *testing.T) {
	gs := newGame(t, 2)
	yeva := addPerm(gs, 0, "Yeva, Nature's Herald", "creature", "elf", "shaman")
	yevaETB(gs, yeva)
	if gs.Flags["yeva_green_flash_seat_0"] == 0 {
		t.Errorf("expected yeva_green_flash_seat_0 flag set after ETB")
	}
}

func TestYeva_LTBClearsFlashPermissionFlag(t *testing.T) {
	gs := newGame(t, 2)
	yeva := addPerm(gs, 0, "Yeva, Nature's Herald", "creature")
	yevaETB(gs, yeva)
	if gs.Flags["yeva_green_flash_seat_0"] == 0 {
		t.Fatal("setup failed: flag should be set after ETB")
	}
	yevaLTB(gs, yeva, map[string]interface{}{"perm": yeva})
	if _, ok := gs.Flags["yeva_green_flash_seat_0"]; ok {
		t.Errorf("expected flag deleted after Yeva LTB, still present")
	}
}

func TestYeva_LTBNoopOnUnrelatedLTB(t *testing.T) {
	gs := newGame(t, 2)
	yeva := addPerm(gs, 0, "Yeva, Nature's Herald", "creature")
	other := addPerm(gs, 0, "Other Creature", "creature")
	yevaETB(gs, yeva)
	yevaLTB(gs, yeva, map[string]interface{}{"perm": other})
	if gs.Flags["yeva_green_flash_seat_0"] == 0 {
		t.Errorf("Yeva LTB handler must only clear flag when Yeva leaves, not on unrelated LTB")
	}
}
