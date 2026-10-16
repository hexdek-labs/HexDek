package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r132 — escape was unreachable from the production driver.
//
// CR §702.138a: "You may cast this card from your graveyard by paying
// [cost] rather than paying its mana cost. Casting a spell using its
// escape ability follows the rules for paying alternative costs in
// rules 601.2b and 601.2f-h."
//
// Same shape as r127's flashback gap, found the same way: CastWithEscape
// is fully built and had ZERO production callers, because
// buildCastableList walked seat.Hand only. Escape appears in 194 of
// 1,758 live decks.
//
// The one thing escape has that flashback does not is an additional cost
// paid out of the zone the spell is cast from — "exile N other cards
// from your graveyard" — so the graveyard must hold N+1 cards.
//
// These drive the real enumerator (r127Game / r127Offered reused).

// r132Escape builds a sorcery with escape in the corpus's arg shape:
// the mana cost string at Args[0] and the exile count at Args[1].
func r132Escape(name string, cmc int, escapeCost string, exileN int, manaCost string) *gameengine.Card {
	return &gameengine.Card{
		Name:           name,
		CMC:            cmc,
		ManaCostString: manaCost,
		Types:          []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "escape", Args: []any{escapeCost, float64(exileN)}},
			},
		},
	}
}

func r132Filler(n int) []*gameengine.Card {
	out := make([]*gameengine.Card, n)
	for i := range out {
		out[i] = &gameengine.Card{Name: "Filler", Types: []string{"creature"}}
	}
	return out
}

// THE BUG: an escape card in the graveyard with the mana and the fodder
// to pay for it was never offered at all.
func TestR132_EscapeCardIsOffered(t *testing.T) {
	card := r132Escape("Uro", 3, "{1}{G}{U}", 5, "{1}{G}{U}")
	gy := append([]*gameengine.Card{card}, r132Filler(5)...)
	gs, seatIdx := r127Game(t, 0, gy...)
	gameengine.AddMana(gs, gs.Seats[0], "G", 1, "r132 Forest")
	gameengine.AddMana(gs, gs.Seats[0], "U", 1, "r132 Island")
	gameengine.AddMana(gs, gs.Seats[0], "C", 1, "r132 Wastes")

	list := buildCastableList(gs, seatIdx)
	if !r127Offered(list, "Uro") {
		t.Fatal("an affordable escape card in the graveyard must be offered")
	}
	if !card.CastingEscape {
		t.Error("the candidate must be marked CastingEscape so the driver routes to CastWithEscape")
	}
}

// MUST NEVER OFFER: the exile cost is paid from the same graveyard, so
// N+1 cards are required. Four fodder cards cannot pay "exile five".
func TestR132_NotOfferedWhenGraveyardCannotPayTheExileCost(t *testing.T) {
	card := r132Escape("Uro", 3, "{1}{G}{U}", 5, "{1}{G}{U}")
	gy := append([]*gameengine.Card{card}, r132Filler(4)...)
	gs, seatIdx := r127Game(t, 0, gy...)
	gameengine.AddMana(gs, gs.Seats[0], "G", 1, "r132 Forest")
	gameengine.AddMana(gs, gs.Seats[0], "U", 1, "r132 Island")
	gameengine.AddMana(gs, gs.Seats[0], "C", 1, "r132 Wastes")

	if r127Offered(buildCastableList(gs, seatIdx), "Uro") {
		t.Fatal("escape needs N OTHER cards to exile; 4 fodder cannot pay exile-5")
	}
	if card.CastingEscape {
		t.Error("the marker must be cleared when the card is not offered")
	}
}

// MUST NEVER OFFER: §202.1a applies to the alternative cost (r128).
// Three colourless is enough in TOTAL for {1}{G}{U} and has neither pip.
func TestR132_NotOfferedWhenColourCannotPayTheEscapeCost(t *testing.T) {
	card := r132Escape("Uro", 3, "{1}{G}{U}", 5, "{1}{G}{U}")
	gy := append([]*gameengine.Card{card}, r132Filler(5)...)
	gs, seatIdx := r127Game(t, 0, gy...)
	gameengine.AddMana(gs, gs.Seats[0], "C", 3, "r132 Wastes")

	if r127Offered(buildCastableList(gs, seatIdx), "Uro") {
		t.Fatal("three colourless cannot pay {1}{G}{U}: §202.1a governs the alternative cost too")
	}
}

// MUST NEVER OFFER: not affordable on total mana either.
func TestR132_NotOfferedWhenManaIsShort(t *testing.T) {
	card := r132Escape("Uro", 3, "{1}{G}{U}", 5, "{1}{G}{U}")
	gy := append([]*gameengine.Card{card}, r132Filler(5)...)
	gs, seatIdx := r127Game(t, 0, gy...)
	gameengine.AddMana(gs, gs.Seats[0], "G", 1, "r132 Forest")

	if r127Offered(buildCastableList(gs, seatIdx), "Uro") {
		t.Fatal("one mana cannot pay a three-mana escape cost")
	}
}

// MUST NEVER OFFER: a graveyard card with no escape keyword.
func TestR132_PlainGraveyardCardIsNotOffered(t *testing.T) {
	plain := &gameengine.Card{Name: "Plain Sorcery", CMC: 2, Types: []string{"sorcery"}, ManaCostString: "{1}{G}"}
	gy := append([]*gameengine.Card{plain}, r132Filler(5)...)
	gs, seatIdx := r127Game(t, 0, gy...)
	gameengine.AddMana(gs, gs.Seats[0], "G", 5, "r132 Forest")

	if r127Offered(buildCastableList(gs, seatIdx), "Plain Sorcery") {
		t.Fatal("a graveyard card without escape must not be offered as an escape cast")
	}
}

// The exile-target policy: pay with inert cards before spending other
// recursion. Exiling your own escape fodder to pay for an escape is
// strictly worse than exiling an inert card.
func TestR132_ExileTargetsPreferInertCardsOverRecursion(t *testing.T) {
	self := r132Escape("Uro", 3, "{1}{G}{U}", 2, "{1}{G}{U}")
	other := r132Escape("Kroxa", 2, "{1}{B}{R}", 2, "{1}{B}{R}")
	inertA := &gameengine.Card{Name: "InertA", Types: []string{"creature"}}
	inertB := &gameengine.Card{Name: "InertB", Types: []string{"creature"}}
	gs, _ := r127Game(t, 0, self, other, inertA, inertB)

	got := escapeExileTargets(gs.Seats[0], self, 2)
	if len(got) != 2 {
		t.Fatalf("want 2 exile targets, got %d", len(got))
	}
	for _, c := range got {
		if c == self {
			t.Error("the escaping card must never be one of its own exile targets")
		}
		if c == other {
			t.Error("inert cards were available; recursion should not have been exiled")
		}
	}
}

// ... but recursion IS spent when there is nothing else, rather than
// declining a legal cast.
func TestR132_ExileTargetsFallBackToRecursionWhenNothingElse(t *testing.T) {
	self := r132Escape("Uro", 3, "{1}{G}{U}", 2, "{1}{G}{U}")
	otherA := r132Escape("Kroxa", 2, "{1}{B}{R}", 2, "{1}{B}{R}")
	otherB := r132Escape("Phoenix", 2, "{1}{R}", 2, "{1}{R}")
	gs, _ := r127Game(t, 0, self, otherA, otherB)

	if got := escapeExileTargets(gs.Seats[0], self, 2); len(got) != 2 {
		t.Fatalf("recursion must be spent when it is the only fodder; got %d targets", len(got))
	}
}
