package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// mulligan_history_r60_test.go — pins that RunLondonMulligan stamps
// gs.MulliganHistory[seatIdx] with the final post-bottom hand and
// the right mulligan count. Heimdall's ExtractMulliganStats relies on
// this being populated regardless of gs.RetainEvents.

func TestRunLondonMulligan_RecordsHistoryOnKeep(t *testing.T) {
	gs := newMulliganGame(t)
	fillLibrary(gs, 0, 60)
	gs.Seats[0].Hat = &hat.GreedyHat{} // always keep

	RunLondonMulligan(gs, 0)

	if len(gs.MulliganHistory) < 1 {
		t.Fatalf("MulliganHistory not populated, len=%d", len(gs.MulliganHistory))
	}
	h := gs.MulliganHistory[0]
	if h.MulligansTaken != 0 {
		t.Errorf("MulligansTaken = %d, want 0 (kept first hand)", h.MulligansTaken)
	}
	if len(h.OpeningHand) != 7 {
		t.Errorf("OpeningHand len = %d, want 7", len(h.OpeningHand))
	}
	// At least the lands in the synthetic deck carry Types (fillLibrary
	// stamps "land" on the land slots and leaves non-land slots
	// untyped). Real corpus cards always carry Types — verifying the
	// land entries round-trip is enough to confirm the snapshot
	// preserved them.
	landSnapshots := 0
	for _, c := range h.OpeningHand {
		for _, ty := range c.Types {
			if ty == "land" {
				landSnapshots++
				break
			}
		}
	}
	if landSnapshots == 0 {
		t.Errorf("OpeningHand snapshot dropped all land Types — classifier will misread")
	}
}

func TestRunLondonMulligan_RecordsHistoryAfterMulligan(t *testing.T) {
	gs := newMulliganGame(t)
	fillLibrary(gs, 0, 60)
	gs.Seats[0].Hat = &mulliganOnceHat{}

	RunLondonMulligan(gs, 0)

	if len(gs.MulliganHistory) < 1 {
		t.Fatalf("MulliganHistory not populated")
	}
	h := gs.MulliganHistory[0]
	if h.MulligansTaken != 1 {
		t.Errorf("MulligansTaken = %d, want 1", h.MulligansTaken)
	}
	// After 1 mulligan + bottom: 6 cards remain in hand.
	if len(h.OpeningHand) != 6 {
		t.Errorf("OpeningHand len = %d, want 6 (7 drawn, 1 bottomed)", len(h.OpeningHand))
	}
}

func TestRunLondonMulligan_PerSeatIsolation(t *testing.T) {
	// Two seats run mulligan with different decisions — each entry
	// must land in the right slot without overwriting the sibling.
	gs := newMulliganGame(t) // 2 seats
	fillLibrary(gs, 0, 60)
	fillLibrary(gs, 1, 60)
	gs.Seats[0].Hat = &hat.GreedyHat{} // keep, 0 mulligans
	gs.Seats[1].Hat = &mulliganOnceHat{}

	RunLondonMulligan(gs, 0)
	RunLondonMulligan(gs, 1)

	if len(gs.MulliganHistory) != 2 {
		t.Fatalf("MulliganHistory should have 2 slots, got %d", len(gs.MulliganHistory))
	}
	if gs.MulliganHistory[0].MulligansTaken != 0 {
		t.Errorf("seat 0 MulligansTaken = %d, want 0", gs.MulliganHistory[0].MulligansTaken)
	}
	if gs.MulliganHistory[1].MulligansTaken != 1 {
		t.Errorf("seat 1 MulligansTaken = %d, want 1", gs.MulliganHistory[1].MulligansTaken)
	}
	if len(gs.MulliganHistory[0].OpeningHand) != 7 {
		t.Errorf("seat 0 hand len = %d, want 7", len(gs.MulliganHistory[0].OpeningHand))
	}
	if len(gs.MulliganHistory[1].OpeningHand) != 6 {
		t.Errorf("seat 1 hand len = %d, want 6", len(gs.MulliganHistory[1].OpeningHand))
	}
}

// Snapshot must capture values (not aliases) so that later zone
// changes / token reuse on the same *Card pointers can't mutate the
// recorded names or types.
func TestRunLondonMulligan_HistoryDoesNotAliasLiveCards(t *testing.T) {
	gs := newMulliganGame(t)
	fillLibrary(gs, 0, 60)
	gs.Seats[0].Hat = &hat.GreedyHat{}

	RunLondonMulligan(gs, 0)

	snap := gs.MulliganHistory[0]
	snapTypesBefore := make([]string, len(snap.OpeningHand[0].Types))
	copy(snapTypesBefore, snap.OpeningHand[0].Types)
	snapNameBefore := snap.OpeningHand[0].Name

	// Mutate the live Card aggressively — names, types, even nil out.
	for _, c := range gs.Seats[0].Hand {
		if c == nil {
			continue
		}
		c.Name = "MUTATED"
		c.Types = []string{"mutated"}
	}
	gs.Seats[0].Hand = nil

	// Snapshot must still read the original values.
	if snap.OpeningHand[0].Name != snapNameBefore {
		t.Errorf("snapshot Name aliased to live Card: was %q, now %q", snapNameBefore, snap.OpeningHand[0].Name)
	}
	if len(snap.OpeningHand[0].Types) != len(snapTypesBefore) {
		t.Errorf("snapshot Types aliased to live Card")
	}
	// Re-verify against gs.MulliganHistory (the public surface) too.
	if gs.MulliganHistory[0].OpeningHand[0].Name == "MUTATED" {
		t.Errorf("gs.MulliganHistory aliased to live hand — snapshot must capture by value")
	}

	// Touch gameengine to avoid an unused-import lint when shrinking tests.
	_ = gameengine.SeatMulliganStats{}
}
