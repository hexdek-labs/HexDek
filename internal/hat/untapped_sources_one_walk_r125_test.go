package hat

import "testing"

// r125 — the one-walk colour-source count must agree with the per-colour
// walk it replaces, for EVERY colour, on a board that actually has every
// colour.
//
// This is a pure hoist: r124 populated Card.ManaCostString for the first
// time, which turned a loop that had never executed into up to five full
// battlefield walks per evaluation and halved grinder throughput. The
// semantics must be untouched, so the guard is agreement with the original
// function rather than a hand-written expectation.
//
// THE BOARD IS THE POINT. A first draft of this test borrowed a fixture
// with no blue source, and a deliberate mutation that skipped slot 1 (U)
// passed it. The board below gives every colour a distinct, non-equal
// count, with a tapped land per colour so "untapped" is load-bearing too —
// so a mutation to any single slot has something to break.
func TestUntappedSourcesOneWalkR125_AgreesWithPerColour(t *testing.T) {
	_, gs := makeColorScrewEvalGame(t)
	seat := gs.Seats[0]
	// Distinct counts per colour so no two slots can be confused, plus a
	// tapped land of each colour that must NOT be counted.
	want := map[string]int{"W": 1, "U": 2, "B": 3, "R": 4, "G": 5}
	for _, c := range []string{"W", "U", "B", "R", "G"} {
		for i := 0; i < want[c]; i++ {
			makeColoredLand(seat, c, false)
		}
		makeColoredLand(seat, c, true) // tapped — must be excluded
	}

	all := untappedFieldColorSourcesAll(seat)
	for i, c := range []string{"W", "U", "B", "R", "G"} {
		perColour := untappedFieldColorSources(seat, c)
		if perColour != want[c] {
			t.Fatalf("fixture: per-colour walk for %s = %d, want %d — "+
				"the board is not what this test assumes", c, perColour, want[c])
		}
		if all[i] != perColour {
			t.Errorf("colour %s: one-walk = %d, per-colour walk = %d", c, all[i], perColour)
		}
	}
}

func TestUntappedSourcesOneWalkR125_NilSeat(t *testing.T) {
	if got := untappedFieldColorSourcesAll(nil); got != [5]int{} {
		t.Errorf("nil seat = %v, want all zeros", got)
	}
}
