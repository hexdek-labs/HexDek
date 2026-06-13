package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// ucover_shard_r63_test.go — regression pins for the shard-U..Z per-card
// DOA sweep (batch 1: Winds of Change, Whispering Madness, Wheel and
// Deal). Each card previously parsed to an inert raw-text residual and
// produced NO observable effect; these tests assert the new per_card
// handlers now produce the printed effect. They exercise the OnResolve
// hook directly (no stack needed for the spell body).

func handOf(seat int, names ...string) []*gameengine.Card {
	out := make([]*gameengine.Card, 0, len(names))
	for _, n := range names {
		out = append(out, &gameengine.Card{Name: n, Owner: seat})
	}
	return out
}

func cardSet(cards []*gameengine.Card) map[string]bool {
	m := map[string]bool{}
	for _, c := range cards {
		m[c.Name] = true
	}
	return m
}

// -----------------------------------------------------------------------------
// Winds of Change — each player shuffles hand into library, draws that many.
// Card-neutral: hand size preserved, library size preserved, full multiset
// of a seat's cards conserved across hand+library.
// -----------------------------------------------------------------------------

func TestWindsOfChange_RefreshesHandSamesize(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hand = handOf(0, "HandA", "HandB")
	addLibrary(gs, 0, "L1", "L2", "L3", "L4")
	gs.Seats[1].Hand = handOf(1, "Solo")
	addLibrary(gs, 1, "X1", "X2", "X3")

	card := addCard(gs, 0, "Winds of Change", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	// Seat 0: hand back to 2, library back to 4, all 6 cards still present.
	if got := len(gs.Seats[0].Hand); got != 2 {
		t.Fatalf("seat0 hand = %d, want 2 (drew that many back)", got)
	}
	if got := len(gs.Seats[0].Library); got != 4 {
		t.Fatalf("seat0 library = %d, want 4 (net unchanged)", got)
	}
	all := cardSet(append(append([]*gameengine.Card{}, gs.Seats[0].Hand...), gs.Seats[0].Library...))
	for _, n := range []string{"HandA", "HandB", "L1", "L2", "L3", "L4"} {
		if !all[n] {
			t.Fatalf("seat0 lost card %q (conservation break): %v", n, all)
		}
	}
	// Seat 1: hand back to 1, library back to 3.
	if got := len(gs.Seats[1].Hand); got != 1 {
		t.Fatalf("seat1 hand = %d, want 1", got)
	}
	if got := len(gs.Seats[1].Library); got != 3 {
		t.Fatalf("seat1 library = %d, want 3", got)
	}
}

// Empty hand draws zero (count captured before emptying).
func TestWindsOfChange_EmptyHandDrawsZero(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hand = nil
	addLibrary(gs, 0, "L1", "L2", "L3")
	card := addCard(gs, 0, "Winds of Change", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if got := len(gs.Seats[0].Hand); got != 0 {
		t.Fatalf("seat0 hand = %d, want 0 (nothing to redraw)", got)
	}
	if got := len(gs.Seats[0].Library); got != 3 {
		t.Fatalf("seat0 library = %d, want 3 (untouched)", got)
	}
}

// -----------------------------------------------------------------------------
// Whispering Madness — each player discards hand, draws = greatest discarded.
// -----------------------------------------------------------------------------

func TestWhisperingMadness_DrawsGreatestDiscarded(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hand = handOf(0, "A", "B", "C") // discards 3 — the max
	addLibrary(gs, 0, "p", "q", "r", "s", "t")
	gs.Seats[1].Hand = handOf(1, "Z") // discards 1
	addLibrary(gs, 1, "u", "v", "w", "x")
	gy0, gy1 := len(gs.Seats[0].Graveyard), len(gs.Seats[1].Graveyard)

	card := addCard(gs, 0, "Whispering Madness", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	// Max discarded = 3 → everyone draws 3.
	if got := len(gs.Seats[0].Hand); got != 3 {
		t.Fatalf("seat0 hand = %d, want 3 (drew max-discarded)", got)
	}
	if got := len(gs.Seats[1].Hand); got != 3 {
		t.Fatalf("seat1 hand = %d, want 3 (drew max-discarded, not its own 1)", got)
	}
	if got := len(gs.Seats[0].Graveyard) - gy0; got != 3 {
		t.Fatalf("seat0 discarded %d, want 3", got)
	}
	if got := len(gs.Seats[1].Graveyard) - gy1; got != 1 {
		t.Fatalf("seat1 discarded %d, want 1", got)
	}
}

// -----------------------------------------------------------------------------
// Wheel and Deal — target opponents wheel to 7; the CASTER draws only 1.
// -----------------------------------------------------------------------------

func TestWheelAndDeal_OpponentsWheelCasterCantrips(t *testing.T) {
	gs := newGame(t, 3)
	gs.Seats[0].Hand = handOf(0, "MyA", "MyB") // caster: does NOT wheel
	addLibrary(gs, 0, "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8")
	gs.Seats[1].Hand = handOf(1, "o1", "o2")
	addLibrary(gs, 1, "d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8")
	gs.Seats[2].Hand = handOf(2, "p1")
	addLibrary(gs, 2, "e1", "e2", "e3", "e4", "e5", "e6", "e7", "e8")
	gy0 := len(gs.Seats[0].Graveyard)

	card := addCard(gs, 0, "Wheel and Deal", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	// Opponents discarded their hands and drew 7.
	if got := len(gs.Seats[1].Hand); got != 7 {
		t.Fatalf("opp seat1 hand = %d, want 7", got)
	}
	if got := len(gs.Seats[2].Hand); got != 7 {
		t.Fatalf("opp seat2 hand = %d, want 7", got)
	}
	// Caster did NOT wheel — 2 kept + 1 cantrip = 3, graveyard untouched.
	if got := len(gs.Seats[0].Hand); got != 3 {
		t.Fatalf("caster hand = %d, want 3 (kept 2 + drew 1, no wheel)", got)
	}
	if got := len(gs.Seats[0].Graveyard) - gy0; got != 0 {
		t.Fatalf("caster discarded %d, want 0 (caster does not wheel)", got)
	}
}

// All three handlers survive a registry Reset (init AddResetHook wiring).
func TestUZShard_SurvivesReset(t *testing.T) {
	Reset()
	for _, name := range []string{"Winds of Change", "Whispering Madness", "Wheel and Deal"} {
		if !HasResolve(name) {
			t.Fatalf("handler for %q missing after Reset()", name)
		}
	}
}
