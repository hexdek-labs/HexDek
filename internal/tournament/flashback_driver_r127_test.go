package tournament

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r127 — flashback was unreachable from the production driver.
//
// CR §702.34a / §118.9: flashback is an alternative cost paid instead of
// the mana cost, casting the card from the graveyard. CastFlashback is
// fully built — intrinsic keyword, Snapcaster-style per-card grants,
// Lier-style graveyard-wide grants, cost resolution, exile on resolve —
// and had ZERO production callers, because buildCastableList walks
// seat.Hand only and returned early on an empty hand.
//
// Measured exposure: flashback appears in 760 of 1,758 live decks, the
// largest single reachability gap of the 28 Cast* entry points that have
// no production caller.
//
// These drive the real enumerator, not the helper.

// r127Flashback builds a sorcery with a printed flashback cost, in the
// shape the corpus produces: the keyword carries the cost as its arg, and
// ManaCostString carries the printed pips so §202.1a applies to the
// alternative cost too.
func r127Flashback(name string, cmc int, flashbackCost int, manaCost string) *gameengine.Card {
	return &gameengine.Card{
		Name:           name,
		CMC:            cmc,
		ManaCostString: manaCost,
		Types:          []string{"sorcery"},
		AST: &gameast.CardAST{
			Name: name,
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "flashback", Args: []any{flashbackCost}},
			},
		},
	}
}

func r127Game(t *testing.T, red int, gy ...*gameengine.Card) (*gameengine.GameState, int) {
	t.Helper()
	gs := canonGame(2)
	seat := gs.Seats[0]
	seat.Hand = nil
	seat.Graveyard = gy
	seat.Mana = &gameengine.ColoredManaPool{}
	if red > 0 {
		gameengine.AddMana(gs, seat, "R", red, "r127 Mountain")
	}
	return gs, 0
}

func r127Offered(list []*gameengine.Card, name string) bool {
	for _, c := range list {
		if c != nil && c.Name == name {
			return true
		}
	}
	return false
}

// The headline: a flashback card in the graveyard, with mana to pay the
// flashback cost, must be offered — even though the hand is EMPTY.
func TestFlashbackDriverR127_OfferedFromEmptyHand(t *testing.T) {
	card := r127Flashback("r127 Fiery Return", 4, 2, "{1}{R}")
	gs, s := r127Game(t, 3, card)
	if !r127Offered(buildCastableList(gs, s), "r127 Fiery Return") {
		t.Fatal("flashback card in graveyard NOT offered with an empty hand — " +
			"the enumerator still only sees the hand")
	}
	if !card.CastingFlashback {
		t.Error("offered card is not marked CastingFlashback, so the driver " +
			"would route it to CastSpell and cast it from nowhere")
	}
}

// Not enough mana for the flashback cost — must not be offered.
func TestFlashbackDriverR127_UnaffordableNotOffered(t *testing.T) {
	card := r127Flashback("r127 Too Dear", 4, 5, "{4}{R}")
	gs, s := r127Game(t, 2, card)
	if r127Offered(buildCastableList(gs, s), "r127 Too Dear") {
		t.Error("flashback cost 5 offered on 2 mana")
	}
	if card.CastingFlashback {
		t.Error("rejected card left marked CastingFlashback — a stale marker " +
			"would misroute it on a later pass")
	}
}

// §202.1a applies to the alternative cost as well: a blue flashback card
// is not castable off red mana.
func TestFlashbackDriverR127_ColourAppliesToTheAltCost(t *testing.T) {
	card := r127Flashback("r127 Blue Return", 3, 2, "{1}{U}")
	gs, s := r127Game(t, 3, card)
	if r127Offered(buildCastableList(gs, s), "r127 Blue Return") {
		t.Error("a {1}{U} flashback card was offered off three RED mana (§202.1a)")
	}
}

// A card with no flashback keyword must never be offered from the
// graveyard — otherwise every dead card becomes castable.
func TestFlashbackDriverR127_PlainCardInGraveyardNotOffered(t *testing.T) {
	plain := &gameengine.Card{
		Name: "r127 Just Dead", CMC: 1, ManaCostString: "{R}",
		Types: []string{"sorcery"},
		AST:   &gameast.CardAST{Name: "r127 Just Dead"},
	}
	gs, s := r127Game(t, 3, plain)
	if r127Offered(buildCastableList(gs, s), "r127 Just Dead") {
		t.Error("a graveyard card with NO flashback was offered as castable")
	}
}

// An empty hand AND an empty graveyard must still short-circuit.
func TestFlashbackDriverR127_NothingAnywhere(t *testing.T) {
	gs, s := r127Game(t, 3)
	if got := buildCastableList(gs, s); len(got) != 0 {
		t.Errorf("empty hand and graveyard produced %d candidates", len(got))
	}
}
