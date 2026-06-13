package per_card

import (
	"testing"
)

// Wavebreak Hippocamp: first spell each opponent's turn → draw. Previously
// inert (ordinal_trigger_tail).

func TestWavebreakHippocamp_FirstSpellOnOpponentTurnDraws(t *testing.T) {
	gs := newGame(t, 2)
	wh := addPerm(gs, 0, "Wavebreak Hippocamp", "creature", "enchantment")
	wh.Card.BasePower = 1
	wh.Card.BaseToughness = 3 // nonzero so SBA does not kill it when its trigger resolves
	addLibrary(gs, 0, "A", "B", "C")
	gs.Active = 1 // opponent's turn
	start := len(gs.Seats[0].Hand)

	fireSpellCast(gs, 0, "Brainstorm")
	if len(gs.Seats[0].Hand) != start+1 {
		t.Fatalf("first spell on opponent's turn should draw 1, delta=%d", len(gs.Seats[0].Hand)-start)
	}
	// Second spell same turn: no extra draw.
	fireSpellCast(gs, 0, "Ponder")
	if len(gs.Seats[0].Hand) != start+1 {
		t.Fatalf("second spell same turn must not draw again")
	}
}

func TestWavebreakHippocamp_OwnTurnDoesNotDraw(t *testing.T) {
	gs := newGame(t, 2)
	wh := addPerm(gs, 0, "Wavebreak Hippocamp", "creature", "enchantment")
	wh.Card.BasePower = 1
	wh.Card.BaseToughness = 3 // nonzero so SBA does not kill it when its trigger resolves
	addLibrary(gs, 0, "A", "B")
	gs.Active = 0 // controller's own turn
	start := len(gs.Seats[0].Hand)

	fireSpellCast(gs, 0, "Brainstorm")
	if len(gs.Seats[0].Hand) != start {
		t.Fatalf("casting on your own turn must not draw")
	}
}

func TestWavebreakHippocamp_ResetsAcrossTurns(t *testing.T) {
	gs := newGame(t, 2)
	wh := addPerm(gs, 0, "Wavebreak Hippocamp", "creature", "enchantment")
	wh.Card.BasePower = 1
	wh.Card.BaseToughness = 3 // nonzero so SBA does not kill it when its trigger resolves
	addLibrary(gs, 0, "A", "B", "C", "D")
	gs.Active = 1
	start := len(gs.Seats[0].Hand)

	fireSpellCast(gs, 0, "S1")
	gs.Turn++ // a later opponent's turn
	fireSpellCast(gs, 0, "S2")
	if len(gs.Seats[0].Hand) != start+2 {
		t.Fatalf("expected one draw per opponent turn (2 total), delta=%d", len(gs.Seats[0].Hand)-start)
	}
}
