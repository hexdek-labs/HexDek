package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestOwlbear_ETBDraws(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 5; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, &gameengine.Card{Name: "X", Owner: 0})
	}
	pre := len(gs.Seats[0].Hand)
	gameengine.InvokeETBHook(gs, addPerm(gs, 0, "Owlbear", "creature"))
	if len(gs.Seats[0].Hand)-pre != 1 {
		t.Errorf("Owlbear ETB should draw 1")
	}
}

func TestRedDragon_ETBDamagesEachOpponent(t *testing.T) {
	gs := newGame(t, 3)
	for i := range gs.Seats {
		gs.Seats[i].Life = 40
	}
	gameengine.InvokeETBHook(gs, addPerm(gs, 0, "Red Dragon", "creature"))
	if gs.Seats[1].Life != 36 || gs.Seats[2].Life != 36 {
		t.Errorf("each opponent should take 4: got %d, %d", gs.Seats[1].Life, gs.Seats[2].Life)
	}
	if gs.Seats[0].Life != 40 {
		t.Errorf("controller should be untouched, got %d", gs.Seats[0].Life)
	}
}

func TestMyconidSporeTender_ETBDestroysArtifactOrEnchantment(t *testing.T) {
	gs := newGame(t, 2)
	art := addPerm(gs, 1, "Sol Ring", "artifact")
	art.Card.BaseToughness = 0
	gameengine.InvokeETBHook(gs, addPerm(gs, 0, "Myconid Spore Tender", "creature"))
	for _, p := range gs.Seats[1].Battlefield {
		if p == art {
			t.Errorf("Myconid Spore Tender should destroy the opponent artifact")
		}
	}
}
