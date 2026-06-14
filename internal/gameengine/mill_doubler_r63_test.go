package gameengine

// r63 — mill doublers (CR §614 / Bruvac the Grandiloquent). "If an opponent
// would mill one or more cards, they mill twice that many cards instead." The
// parser emits Bruvac's clause as a generic if_intervening_tail the engine
// never consumed, so opponent mills were NOT doubled. The doubler is applied at
// the mill AMOUNT in resolveMill.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func millLib(seat, n int) []*Card {
	out := make([]*Card, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, &Card{Name: "Lib", Owner: seat, Types: []string{"creature"}})
	}
	return out
}

// millDoubleFactor: Bruvac doubles its controller's OPPONENTS, not the controller.
func TestMillDoubler_FactorOpponentsOnly(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Battlefield = []*Permanent{{
		Card: &Card{Name: "Bruvac the Grandiloquent", Owner: 0, Types: []string{"creature", "legendary"}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{},
	}}
	if f := millDoubleFactor(gs, 1); f != 2 {
		t.Fatalf("Bruvac should double its opponent's mill (seat 1): factor=%d, want 2", f)
	}
	if f := millDoubleFactor(gs, 0); f != 1 {
		t.Fatalf("Bruvac must NOT double its controller's own mill (seat 0): factor=%d, want 1", f)
	}
}

// End-to-end: an opponent of Bruvac's controller milled for 3 mills 6.
func TestMillDoubler_OpponentMillDoubled(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	// Seat 0 controls Bruvac and the mill source.
	gs.Seats[0].Battlefield = []*Permanent{{
		Card: &Card{Name: "Bruvac the Grandiloquent", Owner: 0, Types: []string{"creature", "legendary"}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{},
	}}
	src := &Permanent{
		Card: &Card{Name: "Mind Sculpt", Owner: 0, Types: []string{"sorcery"}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[1].Library = millLib(1, 8)

	// Untargeted "mill 3" → defaults to the opponent (seat 1).
	resolveMill(gs, src, &gameast.Mill{Count: *gameast.NumInt(3), Target: gameast.Filter{Base: "opponent"}})

	if got := len(gs.Seats[1].Graveyard); got != 6 {
		t.Fatalf("Bruvac should double opponent mill 3 → 6: graveyard=%d", got)
	}
	if got := len(gs.Seats[1].Library); got != 2 {
		t.Fatalf("opponent library should be 8-6=2 after doubled mill; got %d", got)
	}
}

// Without Bruvac the same mill is undoubled (3), proving the doubler is the cause.
func TestMillDoubler_NoBruvacUndoubled(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	src := &Permanent{
		Card: &Card{Name: "Mind Sculpt", Owner: 0, Types: []string{"sorcery"}},
		Controller: 0, Owner: 0, Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[1].Library = millLib(1, 8)

	resolveMill(gs, src, &gameast.Mill{Count: *gameast.NumInt(3), Target: gameast.Filter{Base: "opponent"}})

	if got := len(gs.Seats[1].Graveyard); got != 3 {
		t.Fatalf("without a doubler, mill 3 → 3: graveyard=%d", got)
	}
}
