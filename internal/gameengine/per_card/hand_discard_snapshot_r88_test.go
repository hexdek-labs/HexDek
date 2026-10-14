package per_card

import (
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// hand_discard_snapshot_r88_test.go — regressions for the non-terminating
// "discards their hand" loop found wedging both grinder workers on prod
// (.206, 2026-10-13; filed 1e9fbc70, fixed r88).
//
// The shape under test: a handler drains a hand with
// `for len(Hand) > 0 { DiscardCard(...) }`, DiscardCard fires
// card_discarded, and that trigger puts a card back into the same hand.
// Pre-r88 the loop cannot exit. Post-r88 the handler discards the
// SNAPSHOT taken when the effect was applied (CR §701.9a) and the
// refilled cards stay in hand.

// r88HandCard puts a card into seat's hand and returns it.
func r88HandCard(gs *gameengine.GameState, seat int, name string) *gameengine.Card {
	c := &gameengine.Card{Name: name, Owner: seat}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}

// r88Refiller registers a card_discarded handler that moves a card from
// the discarder's library into their hand — the real shape of the prod
// hang (a wheel resolving into a "whenever you discard, draw" payoff).
// It stops after maxRefills so a REGRESSION of this bug fails the
// assertions instead of hanging the package run; the genuinely unbounded
// case is covered by TestHandDiscardSnapshotR88_TerminatesAgainstUnboundedRefill.
//
// The watcher is an ENCHANTMENT on purpose: addPerm leaves
// BasePower/BaseToughness at zero and every trigger dispatch opens a
// priority round, so a creature watcher would be destroyed by the
// §704.5f state-based action before the second discard.
//
// Each caller passes its OWN watcher name: Registry.OnTrigger dedupes by
// (card name, event, function pointer), and two closures written at the
// same source line share a code pointer — so reusing one name would make
// every test after the first silently reuse the FIRST test's closure,
// with its own already-exhausted refill counter, and report "inert".
func r88Refiller(t *testing.T, gs *gameengine.GameState, name string, watcherSeat int, maxRefills int) *int {
	t.Helper()
	refills := 0
	Global().OnTrigger(name, "card_discarded",
		func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			if refills >= maxRefills {
				return
			}
			seat, _ := ctx["discarder_seat"].(int)
			if seat < 0 || seat >= len(gs.Seats) {
				return
			}
			s := gs.Seats[seat]
			if s == nil || len(s.Library) == 0 {
				return
			}
			refills++
			gameengine.MoveCard(gs, s.Library[0], seat, "library", "hand", "draw")
		})
	addPerm(gs, watcherSeat, name, "enchantment")
	return &refills
}

func r88LibraryOf(gs *gameengine.GameState, seat, n int) {
	for i := 0; i < n; i++ {
		addLibrary(gs, seat, "R88 Filler")
	}
}

// TestHandDiscardSnapshotR88_WhisperingMadnessDiscardsOnlyTheSnapshot is
// the probe: it FAILS against the pre-r88 drain loop, which keeps
// discarding every card the trigger puts back.
func TestHandDiscardSnapshotR88_WhisperingMadnessDiscardsOnlyTheSnapshot(t *testing.T) {
	gs := newGame(t, 2)
	for seat := 0; seat < 2; seat++ {
		r88LibraryOf(gs, seat, 40)
	}
	for i := 0; i < 3; i++ {
		r88HandCard(gs, 0, "R88 Hand 0")
	}
	r88HandCard(gs, 1, "R88 Hand 1")
	refills := r88Refiller(t, gs, "R88 Watcher Whispering", 0, 50)

	whisperingMadnessResolve(gs, &gameengine.StackItem{Controller: 0})

	if *refills == 0 {
		t.Fatalf("fixture inert: the card_discarded refiller never fired, so this test could not see the bug")
	}
	// Seat 0 discarded 3 (its snapshot) and seat 1 discarded 1; the
	// greatest discarded this way is 3, so each player then draws 3.
	// The cards the refiller put back are still in hand on top of that.
	gy0 := len(gs.Seats[0].Graveyard)
	if gy0 != 3 {
		t.Errorf("seat 0 graveyard = %d, want 3 (its hand as the spell resolved); the drain loop swept in trigger-refilled cards", gy0)
	}
	if gy1 := len(gs.Seats[1].Graveyard); gy1 != 1 {
		t.Errorf("seat 1 graveyard = %d, want 1", gy1)
	}
}

// TestHandDiscardSnapshotR88_TerminatesAgainstUnboundedRefill is the
// prod case: a refiller with no count limit at all. Pre-r88 this never
// returns. The refiller self-disarms on a deadline so a regression
// fails loudly in bounded time instead of wedging the package run the
// way it wedged both grinder workers.
func TestHandDiscardSnapshotR88_TerminatesAgainstUnboundedRefill(t *testing.T) {
	gs := newGame(t, 2)
	r88LibraryOf(gs, 0, 5000)
	r88LibraryOf(gs, 1, 10)
	r88HandCard(gs, 0, "R88 Hand 0")
	r88HandCard(gs, 1, "R88 Hand 1")

	deadline := time.Now().Add(3 * time.Second)
	refills := 0
	name := "R88 Unbounded Refill Watcher"
	Global().OnTrigger(name, "card_discarded",
		func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			if time.Now().After(deadline) {
				return
			}
			seat, _ := ctx["discarder_seat"].(int)
			if seat != 0 || len(gs.Seats[0].Library) == 0 {
				return
			}
			refills++
			gameengine.MoveCard(gs, gs.Seats[0].Library[0], 0, "library", "hand", "draw")
		})
	addPerm(gs, 0, name, "enchantment")

	start := time.Now()
	whisperingMadnessResolve(gs, &gameengine.StackItem{Controller: 0})
	elapsed := time.Since(start)

	if refills == 0 {
		t.Fatalf("fixture inert: refiller never fired")
	}
	if elapsed >= 3*time.Second {
		t.Errorf("resolve ran %v — it only stopped because the refiller disarmed itself; the loop is still unbounded", elapsed)
	}
	// MEASURED, pre-migration: the deadline is NOT what stopped the old
	// loop here — it ran 0.09s and discarded 1001 cards, halted by the
	// engine's per-turn trigger budget inside this single-turn fixture.
	// Prod had no such mercy: the grinder's loop sat runnable for 70+
	// minutes. So the graveyard count below is the assertion that
	// actually catches a regression; the deadline is only a guard so a
	// regression cannot wedge the package run.
	// One card in hand at resolution, so exactly one discard. Everything
	// the refiller handed back must still be in hand.
	if gy := len(gs.Seats[0].Graveyard); gy != 1 {
		t.Errorf("seat 0 graveyard = %d, want 1 (the single card in hand as the spell resolved)", gy)
	}
}

// TestHandDiscardSnapshotR88_NoDuplicationWhenTriggerRemovesFromHand
// pins the other half of the fix. DiscardCard does not check that the
// card is still in hand: removeCardFromZone silently no-ops and MoveCard
// then appends to the graveyard anyway. A snapshot WITHOUT the
// cardStillInHand re-check therefore trades the hang for a CardIdentity
// violation, so this test must stay green alongside the one above.
func TestHandDiscardSnapshotR88_NoDuplicationWhenTriggerRemovesFromHand(t *testing.T) {
	gs := newGame(t, 1)
	r88LibraryOf(gs, 0, 10)
	first := r88HandCard(gs, 0, "R88 First")
	victim := r88HandCard(gs, 0, "R88 Victim")

	name := "R88 Hand Stripper"
	fired := 0
	Global().OnTrigger(name, "card_discarded",
		func(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
			fired++
			// On the first discard, yank the SECOND snapshot entry out of
			// hand before the loop reaches it.
			if fired != 1 {
				return
			}
			gameengine.MoveCard(gs, victim, 0, "hand", "exile", "r88_strip")
		})
	addPerm(gs, 0, name, "enchantment")

	n := discardHandSnapshot(gs, 0)

	if fired == 0 {
		t.Fatalf("fixture inert: stripper never fired")
	}
	if n != 1 {
		t.Errorf("discardHandSnapshot returned %d, want 1 — only %q was still in hand when its turn came", n, first.Name)
	}
	seen := map[*gameengine.Card]int{}
	for _, c := range gs.Seats[0].Graveyard {
		seen[c]++
	}
	if seen[victim] != 0 {
		t.Errorf("the stripped card is in the graveyard %dx — it had already left hand; MoveCard appended a phantom copy", seen[victim])
	}
	for c, n := range seen {
		if n > 1 {
			t.Errorf("card %q appears %d times in the graveyard (CardIdentity violation)", c.Name, n)
		}
	}
	if len(gs.Seats[0].Exile) != 1 {
		t.Errorf("exile = %d, want 1 (the stripped card)", len(gs.Seats[0].Exile))
	}
}

// TestHandDiscardSnapshotR88_SiblingsTerminate covers the six handlers
// that shared the drain shape. Each is driven through its own entry
// point with the refiller live; the assertion is that control returns
// and the refilled cards were not swept in.
func TestHandDiscardSnapshotR88_SiblingsTerminate(t *testing.T) {
	cases := []struct {
		name string
		// run performs the effect on a 2-seat game where both seats hold
		// handSize cards; wantGY0 is seat 0's graveyard afterwards.
		handSize int
		// wantGY is the graveyard size to assert, on seat checkSeat.
		checkSeat int
		wantGY    int
		run       func(gs *gameengine.GameState)
	}{
		{
			name:      "Windfall",
			handSize:  2,
			checkSeat: 0,
			wantGY:    2,
			run:       func(gs *gameengine.GameState) { windfallResolve(gs, &gameengine.StackItem{Controller: 0}) },
		},
		{
			name:      "Wheel of Fortune",
			handSize:  2,
			checkSeat: 0,
			wantGY:    2,
			run:       func(gs *gameengine.GameState) { wheelOfFortuneResolve(gs, &gameengine.StackItem{Controller: 0}) },
		},
		{
			name:     "Wheel and Deal",
			handSize: 2,
			// Seat 0 is the caster and never discards, so asserting on it
			// would pass pre-migration too — check the opponent.
			checkSeat: 1,
			wantGY:    2,
			run:       func(gs *gameengine.GameState) { wheelAndDealResolve(gs, &gameengine.StackItem{Controller: 0}) },
		},
		{
			name:      "Will of the Jeskai",
			handSize:  2,
			checkSeat: 0,
			wantGY:    2,
			run:       func(gs *gameengine.GameState) { willOfTheJeskaiDiscardDrawFive(gs, 0) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := newGame(t, 2)
			for seat := 0; seat < 2; seat++ {
				r88LibraryOf(gs, seat, 60)
				for i := 0; i < tc.handSize; i++ {
					r88HandCard(gs, seat, "R88 Hand")
				}
			}
			refills := r88Refiller(t, gs, "R88 Watcher "+tc.name, tc.checkSeat, 50)

			tc.run(gs)

			if *refills == 0 {
				t.Fatalf("fixture inert: refiller never fired, so this case could not see the bug")
			}
			if gy := len(gs.Seats[tc.checkSeat].Graveyard); gy != tc.wantGY {
				t.Errorf("seat %d graveyard = %d, want %d — trigger-refilled cards were swept into the discard",
					tc.checkSeat, gy, tc.wantGY)
			}
		})
	}
}

// TestHandDiscardSnapshotR88_LionsEyeDiamondDiscardsSnapshot covers the
// activated-ability member of the family. LED's discard is a COST, and
// the same drain shape sat in it.
func TestHandDiscardSnapshotR88_LionsEyeDiamondDiscardsSnapshot(t *testing.T) {
	gs := newGame(t, 2)
	r88LibraryOf(gs, 0, 60)
	for i := 0; i < 3; i++ {
		r88HandCard(gs, 0, "R88 Hand")
	}
	led := addPerm(gs, 0, "Lion's Eye Diamond", "artifact")
	refills := r88Refiller(t, gs, "R88 Watcher LED", 0, 50)

	lionsEyeDiamondActivate(gs, led, 0, nil)

	if *refills == 0 {
		t.Fatalf("fixture inert: refiller never fired")
	}
	// LED sacrifices itself as part of the cost, so it is in the
	// graveyard too — count only the discarded hand cards.
	discarded := 0
	for _, c := range gs.Seats[0].Graveyard {
		if c.Name != "Lion's Eye Diamond" {
			discarded++
		}
	}
	if discarded != 3 {
		t.Errorf("seat 0 discarded %d cards, want 3 (the hand as the ability was activated)", discarded)
	}
}

// TestHandDiscardSnapshotR88_WindsOfChangeShufflesSnapshot covers the
// one sibling that moves the hand to the LIBRARY rather than discarding
// it. Same drain shape, same unbounded-loop exposure through zone-change
// triggers, so it takes the same snapshot treatment.
//
// HONEST NOTE: this one PASSES pre-migration. There is no per_card
// trigger hook on a hand-to-library move to build a refiller from, the
// way card_discarded gives us one, so what this pins is conservation —
// two in, two out, nothing lost or duplicated by the rewrite. The
// termination argument for Winds of Change is the shared one in
// hand_discard_snapshot_r88.go, not this test.
func TestHandDiscardSnapshotR88_WindsOfChangeShufflesSnapshot(t *testing.T) {
	gs := newGame(t, 2)
	for seat := 0; seat < 2; seat++ {
		r88LibraryOf(gs, seat, 40)
		for i := 0; i < 2; i++ {
			r88HandCard(gs, seat, "R88 Hand")
		}
	}

	windsOfChangeResolve(gs, &gameengine.StackItem{Controller: 0})

	// Two in, two out: hand size is preserved and no card is lost.
	for seat := 0; seat < 2; seat++ {
		if h := len(gs.Seats[seat].Hand); h != 2 {
			t.Errorf("seat %d hand = %d, want 2", seat, h)
		}
		if lib := len(gs.Seats[seat].Library); lib != 40 {
			t.Errorf("seat %d library = %d, want 40 (2 shuffled in, 2 drawn)", seat, lib)
		}
	}
}
