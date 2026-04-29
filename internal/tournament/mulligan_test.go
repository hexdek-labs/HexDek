package tournament

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// =============================================================================
// Wave 4 — London mulligan tests (CR §103.5).
// =============================================================================

func newMulliganGame(t *testing.T) *gameengine.GameState {
	t.Helper()
	rng := rand.New(rand.NewSource(42))
	gs := gameengine.NewGameState(2, rng, nil)
	return gs
}

func fillLibrary(gs *gameengine.GameState, seat, count int) {
	for i := 0; i < count; i++ {
		c := &gameengine.Card{
			Name:  "Card",
			Owner: seat,
			CMC:   i % 7,
		}
		// Put lands in ~40% of the library so GreedyHat's mulligan check
		// sees enough lands in the opening hand to keep.
		if i%5 < 2 {
			c.Name = "Land"
			c.Types = []string{"land"}
			c.CMC = 0
		}
		gs.Seats[seat].Library = append(gs.Seats[seat].Library, c)
	}
}

func TestLondonMulligan_KeepOpenerDraws7(t *testing.T) {
	gs := newMulliganGame(t)
	fillLibrary(gs, 0, 60)
	gs.Seats[0].Hat = &hat.GreedyHat{} // always keep
	RunLondonMulligan(gs, 0)
	if len(gs.Seats[0].Hand) != 7 {
		t.Fatalf("expected 7 cards in hand, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 53 {
		t.Fatalf("expected 53 cards in library, got %d", len(gs.Seats[0].Library))
	}
}

// mulliganOnceHat mulligans the first time, then keeps.
type mulliganOnceHat struct {
	hat.GreedyHat
	mulliganCount int
}

func (h *mulliganOnceHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) bool {
	h.mulliganCount++
	return h.mulliganCount <= 1 // mulligan first time only
}

func TestLondonMulligan_OneMulliganBottomsOneCard(t *testing.T) {
	gs := newMulliganGame(t)
	fillLibrary(gs, 0, 60)
	gs.Seats[0].Hat = &mulliganOnceHat{}
	RunLondonMulligan(gs, 0)
	// After 1 mulligan: draw 7, put 1 on bottom -> 6 in hand.
	if len(gs.Seats[0].Hand) != 6 {
		t.Fatalf("expected 6 cards in hand after 1 mulligan, got %d", len(gs.Seats[0].Hand))
	}
	// Library: 60 - 7 (first draw) + 7 (shuffle back) - 7 (second draw) + 1 (bottomed) = 54.
	if len(gs.Seats[0].Library) != 54 {
		t.Fatalf("expected 54 cards in library, got %d", len(gs.Seats[0].Library))
	}
	// Check for mulligan events.
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "mulligan" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing mulligan event")
	}
}

// mulliganAlwaysHat always mulligans.
type mulliganAlwaysHat struct {
	hat.GreedyHat
}

func (h *mulliganAlwaysHat) ChooseMulligan(gs *gameengine.GameState, seatIdx int, hand []*gameengine.Card) bool {
	return true // always mulligan
}

func TestLondonMulligan_MaxMulligansConverges(t *testing.T) {
	gs := newMulliganGame(t)
	fillLibrary(gs, 0, 60)
	gs.Seats[0].Hat = &mulliganAlwaysHat{}
	RunLondonMulligan(gs, 0)
	// After 7 mulligans the hand should be 0 cards (7 - 7 = 0).
	if len(gs.Seats[0].Hand) != 0 {
		t.Fatalf("expected 0 cards in hand after max mulligans, got %d", len(gs.Seats[0].Hand))
	}
}

func TestLondonMulligan_EmptyLibraryHandlesGracefully(t *testing.T) {
	gs := newMulliganGame(t)
	// Empty library.
	gs.Seats[0].Hat = &hat.GreedyHat{}
	RunLondonMulligan(gs, 0)
	if len(gs.Seats[0].Hand) != 0 {
		t.Fatalf("expected 0 cards drawn from empty library, got %d", len(gs.Seats[0].Hand))
	}
}

func TestLondonMulligan_NilSafe(t *testing.T) {
	// Should not panic.
	RunLondonMulligan(nil, 0)

	gs := newMulliganGame(t)
	RunLondonMulligan(gs, -1)
	RunLondonMulligan(gs, 99)
}
