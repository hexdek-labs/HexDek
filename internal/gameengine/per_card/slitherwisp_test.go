package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/gameast"
)

// Slitherwisp: cast another spell with flash → draw 1 and each opponent loses
// 1 life. Previously inert (parsed_effect_residual).

func flashSpell(name string, owner int) *gameengine.Card {
	return &gameengine.Card{
		Name:  name,
		Owner: owner,
		Types: []string{"instant"},
		AST: &gameast.CardAST{
			Abilities: []gameast.Ability{&gameast.Keyword{Name: "flash"}},
		},
	}
}

func fireSpellCastCard(gs *gameengine.GameState, casterSeat int, spell *gameengine.Card) {
	gameengine.FireCardTrigger(gs, "spell_cast", map[string]interface{}{
		"caster_seat": casterSeat,
		"card":        spell,
		"spell_name":  spell.DisplayName(),
	})
}

func TestSlitherwisp_FlashCastDrawsAndDrains(t *testing.T) {
	gs := newGame(t, 3)
	s := addPerm(gs, 0, "Slitherwisp", "creature")
	s.Card.BasePower = 1
	s.Card.BaseToughness = 2
	addLibrary(gs, 0, "A", "B")
	startHand := len(gs.Seats[0].Hand)
	l1, l2 := gs.Seats[1].Life, gs.Seats[2].Life

	fireSpellCastCard(gs, 0, flashSpell("Brazen Borrower", 0))

	if len(gs.Seats[0].Hand) != startHand+1 {
		t.Fatalf("flash cast should draw 1, delta=%d", len(gs.Seats[0].Hand)-startHand)
	}
	if gs.Seats[1].Life != l1-1 || gs.Seats[2].Life != l2-1 {
		t.Fatalf("each opponent should lose 1 life, got %d,%d", gs.Seats[1].Life, gs.Seats[2].Life)
	}
}

func TestSlitherwisp_NonFlashCastDoesNothing(t *testing.T) {
	gs := newGame(t, 2)
	s := addPerm(gs, 0, "Slitherwisp", "creature")
	s.Card.BasePower = 1
	s.Card.BaseToughness = 2
	addLibrary(gs, 0, "A", "B")
	startHand := len(gs.Seats[0].Hand)
	l1 := gs.Seats[1].Life

	// A spell with no flash keyword (empty AST) must not trigger.
	plain := &gameengine.Card{Name: "Counterspell", Owner: 0, Types: []string{"instant"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{}}}
	fireSpellCastCard(gs, 0, plain)

	if len(gs.Seats[0].Hand) != startHand || gs.Seats[1].Life != l1 {
		t.Fatalf("non-flash spell must not trigger Slitherwisp")
	}
}
