package tournament

// r63 — STACK/PRIORITY: non-active players now receive instant-speed
// priority windows on an opponent's turn (CR §405.3 / §117.1).
//
// Before r63 the turn driver only ever called runInstantPriority(gs, active)
// at the upkeep/draw/end-step windows, so a NON-active player could never act
// at instant speed when the stack was empty — it could only REACT to an object
// an opponent put on the stack. That made the AI structurally unable to cast
// end-of-turn removal, hold up interaction on an opponent's turn, or flash in
// a blocker. runInstantPriorityAPNAP closes the gap by polling the active
// player AND every opponent in APNAP order.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
)

// instantSpyHat records whether its instant-speed cast hook was invoked and
// casts the first offered instant. Embeds GreedyHat for the rest of the
// (large) Hat interface.
type instantSpyHat struct {
	hat.GreedyHat
	castAsked   int
	castReturns bool
}

func (h *instantSpyHat) ChooseCastFromHand(gs *gameengine.GameState, seatIdx int, castable []*gameengine.Card) *gameengine.Card {
	h.castAsked++
	if h.castReturns && len(castable) > 0 {
		return castable[0]
	}
	return nil
}

// apnapBasic returns an untapped basic land producing one mana.
func apnapBasic(gs *gameengine.GameState, owner int) *gameengine.Permanent {
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

// newEndStepGame positions a 2-seat game at seat 0's end step with seat 1
// (non-active) holding an affordable instant and an untapped land.
func newEndStepGame(t *testing.T, oppHat gameengine.Hat) *gameengine.GameState {
	t.Helper()
	gs := gameengine.NewGameState(2, nil, nil)
	gs.Seats[0].Hat = &hat.GreedyHat{}
	gs.Seats[1].Hat = oppHat
	gs.Seats[0].Life = 40
	gs.Seats[1].Life = 40
	gs.Active = 0
	gs.Turn = 3
	gs.Phase, gs.Step = "ending", "end"
	// Seat 1 holds a 1-mana instant and has one untapped land to pay for it.
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, apnapBasic(gs, 1))
	gs.Seats[1].Hand = append(gs.Seats[1].Hand, &gameengine.Card{
		Name: "Quick Trick", TypeLine: "Instant",
		Types: []string{"instant", "cost:1"}, CMC: 1, Owner: 1,
	})
	return gs
}

// TestAPNAP_NonActivePlayerGetsEndStepWindow is the headline regression: a
// non-active player with an affordable instant is offered a cast window on the
// active player's end step.
func TestAPNAP_NonActivePlayerGetsEndStepWindow(t *testing.T) {
	spy := &instantSpyHat{castReturns: false}
	gs := newEndStepGame(t, spy)

	runInstantPriorityAPNAP(gs, 0)

	if spy.castAsked == 0 {
		t.Fatalf("non-active seat 1 was never offered an instant-speed window on seat 0's end step")
	}
}

// TestAPNAP_NonActivePlayerCastsEndOfTurnInstant proves the window leads to a
// real cast: the non-active player's instant leaves its hand.
func TestAPNAP_NonActivePlayerCastsEndOfTurnInstant(t *testing.T) {
	spy := &instantSpyHat{castReturns: true}
	gs := newEndStepGame(t, spy)
	before := len(gs.Seats[1].Hand)

	runInstantPriorityAPNAP(gs, 0)

	if len(gs.Seats[1].Hand) >= before {
		t.Fatalf("non-active seat 1 did not cast its end-of-turn instant (hand %d → %d)",
			before, len(gs.Seats[1].Hand))
	}
}

// TestAPNAP_DisabledTogglePreservesOldBehavior is the A/B control: with the
// toggle set, ONLY the active player is polled — reproducing the pre-r63 gap
// where the non-active seat never acts.
func TestAPNAP_DisabledTogglePreservesOldBehavior(t *testing.T) {
	nonActiveInstantWindowsDisabled = true
	defer func() { nonActiveInstantWindowsDisabled = false }()

	spy := &instantSpyHat{castReturns: true}
	gs := newEndStepGame(t, spy)
	before := len(gs.Seats[1].Hand)

	runInstantPriorityAPNAP(gs, 0)

	if spy.castAsked != 0 {
		t.Errorf("with non-active windows disabled, seat 1 should never be polled; got %d asks", spy.castAsked)
	}
	if len(gs.Seats[1].Hand) != before {
		t.Errorf("with non-active windows disabled, seat 1 should not cast; hand %d → %d", before, len(gs.Seats[1].Hand))
	}
}

// TestAPNAP_NoAffordablePlayNoWindow confirms the cheap gate: a non-active
// seat with no untapped mana is not polled (so the added windows are free when
// the seat can't act).
func TestAPNAP_NoAffordablePlayNoWindow(t *testing.T) {
	spy := &instantSpyHat{castReturns: true}
	gs := newEndStepGame(t, spy)
	// Tap seat 1's only land — now it can't afford the instant.
	for _, p := range gs.Seats[1].Battlefield {
		p.Tapped = true
	}

	runInstantPriorityAPNAP(gs, 0)

	if spy.castAsked != 0 {
		t.Errorf("seat 1 with no available mana should not be polled; got %d asks", spy.castAsked)
	}
}
