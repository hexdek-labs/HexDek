package hexapi

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r131 — the heap abort now carries its own evidence.
//
// HONEST FRAMING: this is instrumentation, not a behaviour fix, so
// nothing here is "red pre-change" in the usual sense — the functions
// did not exist. What justifies the round is a measured gap: r130 could
// identify the decks for only 2 of 18 historical aborts, because the
// other 16 emitted no SLOW GAME line to join against. These tests pin
// that the abort line can now answer that question by itself.

func r131Seat(life int, bf []*gameengine.Permanent, hand, gy, exile, lib int) *gameengine.Seat {
	mk := func(n int) []*gameengine.Card {
		out := make([]*gameengine.Card, n)
		for i := range out {
			out[i] = &gameengine.Card{Name: "filler"}
		}
		return out
	}
	return &gameengine.Seat{
		Life:        life,
		Battlefield: bf,
		Hand:        mk(hand),
		Graveyard:   mk(gy),
		Exile:       mk(exile),
		Library:     mk(lib),
	}
}

// r131Perms builds n permanents, the first `tokens` of which are tokens
// (IsToken reads "token" from Card.Types).
func r131Perms(n, tokens int) []*gameengine.Permanent {
	out := make([]*gameengine.Permanent, n)
	for i := range out {
		types := []string{"creature"}
		if i < tokens {
			types = append(types, "token")
		}
		out[i] = &gameengine.Permanent{Card: &gameengine.Card{Name: "p", Types: types}}
	}
	return out
}

// THE POINT OF THE ROUND: a runaway shows up as an absurd count of
// something, and the census has to say which something.
func TestR131_CensusSurfacesARunawayTokenCount(t *testing.T) {
	gs := &gameengine.GameState{
		Seats: []*gameengine.Seat{
			r131Seat(40, r131Perms(5000, 4980), 3, 11, 2, 60),
			r131Seat(12, r131Perms(4, 0), 7, 2, 0, 80),
		},
	}
	got := gameCensus(gs)
	for _, want := range []string{"bf=5000(tok=4980)", "perms=5004", "tokens=4980", "life=40", "life=12"} {
		if !strings.Contains(got, want) {
			t.Errorf("census must surface %q; got %s", want, got)
		}
	}
}

// Every zone is reported, so whichever one is runaway is visible —
// a graveyard or exile blowup reads as clearly as a battlefield one.
func TestR131_CensusReportsEveryZone(t *testing.T) {
	gs := &gameengine.GameState{
		Stack: make([]*gameengine.StackItem, 7),
		Seats: []*gameengine.Seat{r131Seat(31, r131Perms(2, 1), 5, 9000, 4, 61)},
	}
	got := gameCensus(gs)
	for _, want := range []string{"stack=7", "hand=5", "gy=9000", "exile=4", "lib=61"} {
		if !strings.Contains(got, want) {
			t.Errorf("census must report %q; got %s", want, got)
		}
	}
}

// A guard that panics while reporting a failure is worse than no
// report: nil state and nil seats must not take the process down.
func TestR131_CensusSurvivesNilState(t *testing.T) {
	if got := gameCensus(nil); got == "" {
		t.Fatal("nil game must still render something")
	}
	gs := &gameengine.GameState{Seats: []*gameengine.Seat{nil, r131Seat(40, nil, 0, 0, 0, 99)}}
	got := gameCensus(gs)
	if !strings.Contains(got, "s0=nil") {
		t.Errorf("a nil seat must be reported, not skipped silently; got %s", got)
	}
	if !strings.Contains(got, "lib=99") {
		t.Errorf("the surviving seat must still be censused; got %s", got)
	}
}

// The identity half — this is exactly what r130 had to reconstruct by
// joining to SLOW GAME, and could only get for 2 of 18 aborts.
func TestR131_AbortContextNamesSeedAndEveryDeck(t *testing.T) {
	keys := []string{"moxfield/alpha", "moxfield/beta", "moxfield/gamma", "moxfield/delta"}
	got := abortContext(1789817169370804852, keys)
	if !strings.Contains(got, "seed=1789817169370804852") {
		t.Errorf("abort context must carry the seed; got %s", got)
	}
	for _, k := range keys {
		if !strings.Contains(got, k) {
			t.Errorf("abort context must name every deck, missing %q; got %s", k, got)
		}
	}
}
