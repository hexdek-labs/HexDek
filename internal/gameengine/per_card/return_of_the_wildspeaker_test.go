package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestReturnOfTheWildspeaker_DrawsByGreatestNonHumanPower(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "a", "b", "c", "d", "e", "f")
	beast := addPerm(gs, 0, "Big Beast", "creature", "beast")
	beast.Card.BasePower, beast.Card.BaseToughness = 5, 5
	small := addPerm(gs, 0, "Elf", "creature", "elf")
	small.Card.BasePower, small.Card.BaseToughness = 1, 1
	// A bigger HUMAN should NOT count.
	human := addPerm(gs, 0, "Big Human", "creature", "human")
	human.Card.BasePower, human.Card.BaseToughness = 7, 7

	start := len(gs.Seats[0].Hand)
	card := addCard(gs, 0, "Return of the Wildspeaker", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	if got := len(gs.Seats[0].Hand) - start; got != 5 {
		t.Fatalf("expected to draw 5 (greatest non-Human power), drew %d", got)
	}
}

func TestReturnOfTheWildspeaker_NoNonHumanNoDraw(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "a", "b")
	human := addPerm(gs, 0, "Soldier", "creature", "human")
	human.Card.BasePower, human.Card.BaseToughness = 4, 4
	start := len(gs.Seats[0].Hand)
	card := addCard(gs, 0, "Return of the Wildspeaker", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if len(gs.Seats[0].Hand) != start {
		t.Fatalf("expected no draw with only Human creatures")
	}
}
