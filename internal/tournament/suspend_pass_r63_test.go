package tournament

// r63 — generic suspend-from-hand entry point (CR §702.62b). Before this,
// only Jhoira's per-card handler could suspend anything; the other 68 suspend
// cards parsed a suspend keyword that no play path exposed. runSuspendPass now
// takes the suspend special action in the main phase. Driving a turn with a
// suspend-only card in hand must exile it with N time counters.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

func TestSuspend_GenericPassExilesHandCardUnderDriver(t *testing.T) {
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

	// A realistic suspend creature: a 7-CMC bomb (Errant Ephemeron shape) the
	// seat cannot hard-cast off Forests, with a cheap suspend cost {0}. It is
	// therefore excluded from the castable list and reaches the suspend pass,
	// which exiles it with its 2 time counters.
	c := &gameengine.Card{
		Name: "Test Suspendling", Owner: 0, Types: []string{"creature"},
		ManaCostString: "{6}{U}", CMC: 7,
		BasePower: 6, BaseToughness: 4,
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "suspend", Raw: "suspend 2-{0}", Args: []interface{}{"2", "{0}"}},
		}},
	}
	gs.Seats[0].Hand = []*gameengine.Card{c}

	takeTurnImpl(gs, nil)

	// The driver's suspend pass must have exiled it with 2 time counters.
	inExile := false
	for _, e := range gs.Seats[0].Exile {
		if e == c {
			inExile = true
		}
	}
	if !inExile {
		t.Fatalf("suspend-only card must be exiled (suspended) by the main-phase suspend pass")
	}
	for _, h := range gs.Seats[0].Hand {
		if h == c {
			t.Fatalf("suspended card must leave hand")
		}
	}
	if n, _ := c.Meta["suspend_counters"].(int); n != 2 {
		t.Fatalf("suspended card must carry its N=2 time counters, got %d", n)
	}
	if susp, _ := c.Meta["suspended"].(bool); !susp {
		t.Fatalf("suspended card must be marked suspended")
	}
}
