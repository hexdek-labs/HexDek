package per_card

import (
	"testing"
)

// Mangara, the Diplomat: "Whenever an opponent casts their second spell each
// turn, draw a card." Pins the previously-inert untyped_effect scaffold now
// producing a draw. (Combat clause is intentionally out of scope — see
// handler doc.)

func TestMangara_SecondOpponentSpellDraws(t *testing.T) {
	gs := newGame(t, 2)
	m := addPerm(gs, 0, "Mangara, the Diplomat", "creature", "legendary")
	m.Card.BasePower = 2
	m.Card.BaseToughness = 4 // real Mangara is 2/4; nonzero toughness so SBAs don't kill it mid-test
	startHand := len(gs.Seats[0].Hand)
	// Stock the library so the draw can succeed.
	addLibrary(gs, 0, "Plains", "Island", "Swamp")

	fireOppCast(gs, 1, "Llanowar Elves")
	if len(gs.Seats[0].Hand) != startHand {
		t.Fatalf("first opponent spell must not draw")
	}

	fireOppCast(gs, 1, "Brainstorm")
	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Fatalf("second opponent spell should draw 1, hand delta=%d", len(gs.Seats[0].Hand)-startHand)
	}

	// Third spell does not draw again.
	fireOppCast(gs, 1, "Counterspell")
	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Fatalf("third opponent spell must not draw again, hand delta=%d", len(gs.Seats[0].Hand)-startHand)
	}
}

func TestMangara_OwnSpellsDoNotDraw(t *testing.T) {
	gs := newGame(t, 2)
	m := addPerm(gs, 0, "Mangara, the Diplomat", "creature", "legendary")
	m.Card.BasePower = 2
	m.Card.BaseToughness = 4
	addLibrary(gs, 0, "Plains", "Island")
	startHand := len(gs.Seats[0].Hand)

	fireOppCast(gs, 0, "Sol Ring")
	fireOppCast(gs, 0, "Mana Crypt")
	if len(gs.Seats[0].Hand) != startHand {
		t.Fatalf("controller's own spells must not draw")
	}
}
