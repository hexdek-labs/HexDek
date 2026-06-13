package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// glcover_mr_batch2_r60_test.go — regression pins for shard M-R batch 2
// (Minions' Murmurs, Monumental Corruption, Plow Under, Make a Wish).

func mrFillLib(gs *gameengine.GameState, seat, n int) {
	for i := 0; i < n; i++ {
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, &gameengine.Card{Name: "Filler", Owner: seat})
	}
}

func TestMinionsMurmurs_DrawsAndLosesPerCreature(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	mrFillLib(gs, 0, 10)
	addPerm(gs, 0, "Bear", "creature")
	addPerm(gs, 0, "Elf", "creature")
	addPerm(gs, 0, "Sol Ring", "artifact") // not a creature
	preHand := len(gs.Seats[0].Hand)

	card := addCard(gs, 0, "Minions' Murmurs", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	if got := len(gs.Seats[0].Hand) - preHand; got != 2 {
		t.Errorf("drew %d, want 2 (creatures controlled)", got)
	}
	if gs.Seats[0].Life != 18 {
		t.Errorf("life %d, want 18 (-2)", gs.Seats[0].Life)
	}
}

func TestMonumentalCorruption_TargetDrawsLosesPerArtifact(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 10
	mrFillLib(gs, 1, 10)
	addPerm(gs, 0, "Sol Ring", "artifact")
	addPerm(gs, 0, "Mox", "artifact")
	addPerm(gs, 0, "Signet", "artifact")
	preHand := len(gs.Seats[1].Hand)

	card := addCard(gs, 0, "Monumental Corruption", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	if got := len(gs.Seats[1].Hand) - preHand; got != 3 {
		t.Errorf("target drew %d, want 3 (caster artifacts)", got)
	}
	if gs.Seats[1].Life != 7 {
		t.Errorf("target life %d, want 7 (-3)", gs.Seats[1].Life)
	}
}

func TestPlowUnder_TucksTwoOpponentLands(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 1, "Forest", "land")
	addPerm(gs, 1, "Island", "land")
	addPerm(gs, 1, "Mountain", "land")
	preLib := len(gs.Seats[1].Library)

	card := addCard(gs, 0, "Plow Under", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	lands := 0
	for _, p := range gs.Seats[1].Battlefield {
		if p != nil && p.IsLand() {
			lands++
		}
	}
	if lands != 1 {
		t.Errorf("opponent lands left = %d, want 1 (2 tucked)", lands)
	}
	if got := len(gs.Seats[1].Library) - preLib; got != 2 {
		t.Errorf("library grew by %d, want 2", got)
	}
}

func TestMakeAWish_ReturnsTwoFromGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Graveyard = []*gameengine.Card{
		{Name: "Big", Owner: 0, CMC: 6},
		{Name: "Mid", Owner: 0, CMC: 3},
		{Name: "Small", Owner: 0, CMC: 1},
	}
	preHand := len(gs.Seats[0].Hand)

	card := addCard(gs, 0, "Make a Wish", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	if got := len(gs.Seats[0].Hand) - preHand; got != 2 {
		t.Errorf("returned %d to hand, want 2", got)
	}
	if got := len(gs.Seats[0].Graveyard); got != 1 {
		t.Errorf("graveyard = %d, want 1 left", got)
	}
}
