package tournament

// r63 — CR §702.34a: a madness card the controller doesn't cast is put into the
// graveyard when the window closes. ResolveMadnessWindow had NO production
// caller, so a declined madness card strayed in exile forever. The turn driver
// now closes open windows at end of turn. Driving a turn with an open
// (unaffordable, hence declined) madness window must leave the card in the
// graveyard, not exile.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

func TestMadness_DeclinedWindowClosedByDriver(t *testing.T) {
	gs := gameengine.NewGameState(2, rand.New(rand.NewSource(8)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
		s.Hat = &hat.GreedyHat{}
		for i := 0; i < 8; i++ {
			s.Library = append(s.Library, &gameengine.Card{Name: "Forest", Owner: 0, Types: []string{"land", "forest"}})
		}
	}
	gs.Turn = 3
	gs.Active = 0

	// A madness card with an unaffordable cost (99) so the controller can never
	// cast it from the window — it must be DECLINED and go to the graveyard.
	m := &gameengine.Card{
		Name: "Unaffordable Madness", Owner: 0, Types: []string{"instant"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "madness", Args: []interface{}{99}},
		}},
	}
	gs.Seats[0].Hand = []*gameengine.Card{m}
	if !gameengine.OnDiscardMadness(gs, 0, m) {
		t.Fatalf("precondition: madness exile should fire (card → exile, window open)")
	}

	takeTurnImpl(gs, nil)

	inGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == m {
			inGrave = true
		}
	}
	inExile := false
	for _, c := range gs.Seats[0].Exile {
		if c == m {
			inExile = true
		}
	}
	if inExile || !inGrave {
		t.Fatalf("declined madness card must be put into the graveyard by end of turn; grave=%v exile=%v", inGrave, inExile)
	}
	if gameengine.HasOpenMadnessWindow(gs, 0, m) {
		t.Fatalf("the madness window must be closed by end of turn")
	}
}
