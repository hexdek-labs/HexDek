package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestRakdosCharm_DestroysArtifact(t *testing.T) {
	gs := newGame(t, 2)
	rock := addPerm(gs, 1, "Sol Ring", "artifact")
	rock.Card.CMC = 1
	card := addCard(gs, 0, "Rakdos Charm", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if countArtifacts(gs, 1) != 0 {
		t.Fatalf("expected opponent artifact destroyed")
	}
}

func TestRakdosCharm_ExilesGraveyardWhenNoArtifact(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Graveyard = []*gameengine.Card{
		{Name: "A", Owner: 1}, {Name: "B", Owner: 1}, {Name: "C", Owner: 1},
	}
	card := addCard(gs, 0, "Rakdos Charm", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if len(gs.Seats[1].Graveyard) != 0 {
		t.Fatalf("expected opponent graveyard exiled, %d remain", len(gs.Seats[1].Graveyard))
	}
}
