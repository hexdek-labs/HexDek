package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CR §702.29c: a typecycling ability searches the library, reveals the
// found card, puts it into the player's hand, THEN shuffles the library.
// Regression for the r63 cycling probe: searchLibraryForType moved the
// card to hand but never shuffled (nor revealed), leaking the post-search
// library order. The shuffle is mandatory and fires whether or not a
// matching card was found (CR §701.19e — searching a library shuffles it).

func cyc_swampcyclingCard(gs *GameState, seat int) *Card {
	c := addP0HandCard(gs, seat, "Twisted Abomination", 2, "creature")
	c.AST = &gameast.CardAST{
		Name: "Twisted Abomination",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "swampcycling", Args: []interface{}{float64(2)}},
		},
	}
	return c
}

func TestCycling_TypecyclingShufflesAndReveals(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 5
	card := cyc_swampcyclingCard(gs, 0)

	addP0LibraryTyped(gs, 0, "Bear", "creature")
	addP0LibraryTyped(gs, 0, "Swamp", "land", "swamp")
	addP0LibraryTyped(gs, 0, "Mountain", "land", "mountain")

	if err := ActivateCycling(gs, 0, card); err != nil {
		t.Fatalf("swampcycling should succeed: %v", err)
	}

	// Found the Swamp.
	foundSwamp := false
	for _, c := range gs.Seats[0].Hand {
		if c.Name == "Swamp" {
			foundSwamp = true
		}
	}
	if !foundSwamp {
		t.Fatal("swampcycling should put a Swamp in hand")
	}
	// CR §702.29c — must reveal and must shuffle.
	if p0CountEvents(gs, "reveal") != 1 {
		t.Errorf("expected a reveal event for the fetched card, got %d",
			p0CountEvents(gs, "reveal"))
	}
	if p0CountEvents(gs, "library_shuffled") != 1 {
		t.Errorf("typecycling must shuffle the library (CR §702.29c); got %d shuffle events",
			p0CountEvents(gs, "library_shuffled"))
	}
	// Typecycling replaces the draw.
	if p0CountEvents(gs, "draw") != 0 {
		t.Errorf("typecycling must not draw, got %d", p0CountEvents(gs, "draw"))
	}
}

// CR §701.19e: searching a library shuffles it even on a whiff (no
// matching card). The discard + shuffle still happen; no draw, no fetch.
func TestCycling_TypecyclingShufflesOnWhiff(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 5
	card := cyc_swampcyclingCard(gs, 0)

	// No swamp in the library.
	addP0LibraryTyped(gs, 0, "Bear", "creature")
	addP0LibraryTyped(gs, 0, "Mountain", "land", "mountain")
	preLib := len(gs.Seats[0].Library)

	if err := ActivateCycling(gs, 0, card); err != nil {
		t.Fatalf("swampcycling whiff should still succeed: %v", err)
	}

	// Library unchanged in size (nothing fetched) but still shuffled.
	if len(gs.Seats[0].Library) != preLib {
		t.Errorf("whiff should not remove a card; lib %d -> %d", preLib, len(gs.Seats[0].Library))
	}
	if p0CountEvents(gs, "library_shuffled") != 1 {
		t.Errorf("typecycling must shuffle even on a whiff (CR §701.19e); got %d",
			p0CountEvents(gs, "library_shuffled"))
	}
	if p0CountEvents(gs, "draw") != 0 {
		t.Errorf("typecycling must not draw on a whiff, got %d", p0CountEvents(gs, "draw"))
	}
	// The cycled card was still discarded.
	if p0CountEvents(gs, "cycling") != 1 {
		t.Errorf("expected the cycling event, got %d", p0CountEvents(gs, "cycling"))
	}
}

// Property (a): cycling genuinely discards — a "whenever you discard"
// observer must see the cycled card hit the graveyard (cycling counts as
// a discard; ties to the discard-primitive work).
func TestCycling_CountsAsDiscard(t *testing.T) {
	gs := newP0Game(t)
	gs.Seats[0].ManaPool = 5
	card := addP0HandCard(gs, 0, "Renewed Faith", 2, "instant")
	card.AST = &gameast.CardAST{
		Name: "Renewed Faith",
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "cycling", Args: []interface{}{float64(2)}},
		},
	}
	addP0LibraryTyped(gs, 0, "Drawn Card", "creature")

	if err := ActivateCycling(gs, 0, card); err != nil {
		t.Fatalf("cycling should succeed: %v", err)
	}
	// The cycled card is in the graveyard and a card_discarded trigger fired.
	inGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			inGY = true
		}
	}
	if !inGY {
		t.Error("cycled card must be discarded to the graveyard")
	}
	// Cycling routes through the canonical DiscardCard, so the discard
	// bookkeeping that "whenever you discard" / madness conditions read
	// must be set (this is the discard-primitive tie-in).
	if gs.Seats[0].Turn.Discarded != 1 {
		t.Errorf("cycling must register as a discard (Turn.Discarded); got %d",
			gs.Seats[0].Turn.Discarded)
	}
	if gs.Flags["discarded_0"] < 1 {
		t.Errorf("cycling must set the discarded_<seat> flag for discard conditions; got %d",
			gs.Flags["discarded_0"])
	}
	// Plain cycling draws.
	if p0CountEvents(gs, "draw") != 1 {
		t.Errorf("plain cycling should draw exactly 1, got %d", p0CountEvents(gs, "draw"))
	}
}
