package tournament

// r61 — regression tests for the lazy mana-tap gate on runInstantPriority.
//
// Bug: the turn driver tapped the entire mana base EAGERLY at the top of
// every instant-speed priority window (upkeep/draw/end-step/etc.) so that
// the affordability builders — which read already-floated mana via
// seat.Mana.Total() — would work. But when the hat had nothing to do, the
// lands stayed tapped through every opponent's turn, so the AI could never
// hold up instant-speed interaction.
//
// Fix: hasAffordableInstantPlay gates the eager tap. It computes POTENTIAL
// (untapped) mana via gameengine.AvailableManaEstimate (which counts untapped
// lands + rocks WITHOUT tapping) and only taps when the seat plausibly has an
// instant-speed spell it can afford OR a legal instant-speed activated ability.
// A false positive merely taps as before (status quo, harmless); a false
// negative would skip a window the hat wanted, so the gate leans optimistic.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// newLazyTapSeat seeds a 2-seat game with seat 0 at an instant-speed priority
// window (upkeep). The caller stocks seat 0's battlefield + hand. Returns the
// game state and seat 0 for assertion.
func newLazyTapSeat(t *testing.T) (*gameengine.GameState, *gameengine.Seat) {
	t.Helper()
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = &hat.GreedyHat{}
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Active = 0
	gs.Turn = 3
	// Position at an instant-speed priority window.
	gs.Phase = "beginning"
	gs.Step = "upkeep"
	return gs, gs.Seats[0]
}

// untappedBasic returns an untapped basic-land permanent producing one mana.
func untappedBasic(gs *gameengine.GameState, owner int) *gameengine.Permanent {
	return &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Wastes", TypeLine: "Land",
			Types: []string{"land"}, Owner: owner,
		},
		Controller: owner, Owner: owner,
		Counters: map[string]int{}, Flags: map[string]int{},
		Timestamp: gs.NextTimestamp(),
	}
}

func countTappedLands(seat *gameengine.Seat) int {
	n := 0
	for _, p := range seat.Battlefield {
		if p != nil && p.IsLand() && p.Tapped {
			n++
		}
	}
	return n
}

// TestLazyTap_NoInstants_LandsStayUntapped is the headline regression: a seat
// holding only a sorcery-speed creature (and no instant-speed abilities) must
// NOT have its lands tapped by runInstantPriority — so it can hold mana up.
func TestLazyTap_NoInstants_LandsStayUntapped(t *testing.T) {
	gs, seat := newLazyTapSeat(t)
	// Three untapped lands.
	for i := 0; i < 3; i++ {
		seat.Battlefield = append(seat.Battlefield, untappedBasic(gs, 0))
	}
	// Hand has only a sorcery-speed creature — nothing castable at instant speed.
	seat.Hand = append(seat.Hand, &gameengine.Card{
		Name: "Grizzly Bears", TypeLine: "Creature",
		Types: []string{"creature", "cost:2"}, CMC: 2, Owner: 0,
	})

	runInstantPriority(gs, 0)

	if got := countTappedLands(seat); got != 0 {
		t.Errorf("seat with no instant-speed play should keep lands untapped; got %d tapped", got)
	}
	if seat.Mana.Total() != 0 {
		t.Errorf("no mana should have been floated; got total=%d", seat.Mana.Total())
	}
}

// TestLazyTap_UnaffordableInstant_LandsStayUntapped pins the affordability arm:
// the seat's only instant costs more than its potential mana, so the gate must
// return false and leave lands untapped.
func TestLazyTap_UnaffordableInstant_LandsStayUntapped(t *testing.T) {
	gs, seat := newLazyTapSeat(t)
	// Two untapped lands → potential 2 mana.
	for i := 0; i < 2; i++ {
		seat.Battlefield = append(seat.Battlefield, untappedBasic(gs, 0))
	}
	// A 5-mana instant — unaffordable from 2 potential mana.
	seat.Hand = append(seat.Hand, &gameengine.Card{
		Name: "Expensive Bolt", TypeLine: "Instant",
		Types: []string{"instant", "cost:5"}, CMC: 5, Owner: 0,
	})

	if hasAffordableInstantPlay(gs, 0) {
		t.Fatalf("5-mana instant should NOT be affordable from 2 potential mana")
	}

	runInstantPriority(gs, 0)

	if got := countTappedLands(seat); got != 0 {
		t.Errorf("unaffordable instant should keep lands untapped; got %d tapped", got)
	}
}

// TestLazyTap_AffordableInstant_TapsAndCanCast pins the positive arm: the seat
// holds an affordable instant, so the gate returns true and runInstantPriority
// taps the mana base (the existing builders then read the floated mana).
func TestLazyTap_AffordableInstant_TapsAndCanCast(t *testing.T) {
	gs, seat := newLazyTapSeat(t)
	// Two untapped lands → potential 2 mana.
	for i := 0; i < 2; i++ {
		seat.Battlefield = append(seat.Battlefield, untappedBasic(gs, 0))
	}
	// A 2-mana instant — affordable from 2 potential mana.
	seat.Hand = append(seat.Hand, &gameengine.Card{
		Name: "Cheap Bolt", TypeLine: "Instant",
		Types: []string{"instant", "cost:2"}, CMC: 2, Owner: 0,
	})

	if !hasAffordableInstantPlay(gs, 0) {
		t.Fatalf("2-mana instant SHOULD be affordable from 2 potential mana")
	}

	runInstantPriority(gs, 0)

	// The gate fired, so the lands were tapped for mana. (Whether the
	// GreedyHat actually casts the instant is a hat decision; the
	// load-bearing assertion is that the tap happened — the mana window
	// was opened — rather than being skipped.)
	if got := countTappedLands(seat); got != 2 {
		t.Errorf("affordable instant should open the mana window (tap lands); got %d tapped", got)
	}
}

// TestLazyTap_EmptyHand_NoLands sanity: empty hand + no permanents → no play,
// gate false, nothing tapped.
func TestLazyTap_EmptyHand_NoTap(t *testing.T) {
	gs, seat := newLazyTapSeat(t)
	for i := 0; i < 3; i++ {
		seat.Battlefield = append(seat.Battlefield, untappedBasic(gs, 0))
	}
	// No hand.

	if hasAffordableInstantPlay(gs, 0) {
		t.Fatalf("empty hand with no instant abilities should not register a play")
	}
	runInstantPriority(gs, 0)
	if got := countTappedLands(seat); got != 0 {
		t.Errorf("empty hand should keep lands untapped; got %d tapped", got)
	}
}
