package per_card

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// r92HandFlyer adds a creature card with printed flying to a seat's
// hand. Indominus Rex's as-enters choice only discards creature cards
// that carry one of its twelve listed keywords, so a card without an
// AST keyword makes the fixture inert.
func r92HandFlyer(gs *gameengine.GameState, seat int) *gameengine.Card {
	c := &gameengine.Card{
		Name:  "R92 Flyer",
		Types: []string{"creature"},
		Owner: seat,
		AST:   &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Keyword{Name: "flying"}}},
	}
	gs.Seats[seat].Hand = append(gs.Seats[seat].Hand, c)
	return c
}
