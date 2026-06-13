package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Sethron, Hurloon General: nontoken Minotaur you control enters → create a
// 2/3 red Minotaur token. Previously inert (untyped_effect).

func fireSethronETB(gs *gameengine.GameState, controllerSeat int, perm *gameengine.Permanent) {
	gameengine.FireCardTrigger(gs, "permanent_etb", map[string]interface{}{
		"controller_seat": controllerSeat,
		"perm":            perm,
	})
}

func countMinotaurTokens(gs *gameengine.GameState, seat int) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.IsToken() && p.Card != nil && p.Card.DisplayName() == "Minotaur" {
			n++
		}
	}
	return n
}

func TestSethron_NontokenMinotaurEntersMakesToken(t *testing.T) {
	gs := newGame(t, 2)
	se := addPerm(gs, 0, "Sethron, Hurloon General", "creature", "legendary", "minotaur")
	se.Card.BasePower = 4
	se.Card.BaseToughness = 4

	other := addPerm(gs, 0, "Felhide Brawler", "creature", "minotaur")
	other.Card.BasePower = 2
	other.Card.BaseToughness = 3
	fireSethronETB(gs, 0, other)

	if got := countMinotaurTokens(gs, 0); got != 1 {
		t.Fatalf("expected 1 Minotaur token from nontoken Minotaur ETB, got %d", got)
	}
	// Verify it's a 2/3.
	for _, p := range gs.Seats[0].Battlefield {
		if p.IsToken() && p.Card.DisplayName() == "Minotaur" {
			if p.Power() != 2 || p.Toughness() != 3 {
				t.Fatalf("expected 2/3 Minotaur token, got %d/%d", p.Power(), p.Toughness())
			}
		}
	}
}

func TestSethron_TokenMinotaurDoesNotLoop(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sethron, Hurloon General", "creature", "legendary", "minotaur")

	// A TOKEN Minotaur entering must NOT make another token (nontoken clause).
	tok := gameengine.CreateCreatureToken(gs, 0, "Minotaur", []string{"creature", "minotaur"}, 2, 3)
	before := countMinotaurTokens(gs, 0)
	fireSethronETB(gs, 0, tok)
	if got := countMinotaurTokens(gs, 0); got != before {
		t.Fatalf("token Minotaur ETB must not create another token (loop guard), before=%d after=%d", before, got)
	}
}

func TestSethron_NonMinotaurNoToken(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Sethron, Hurloon General", "creature", "legendary", "minotaur")

	bear := addPerm(gs, 0, "Grizzly Bears", "creature", "bear")
	bear.Card.BasePower = 2
	bear.Card.BaseToughness = 2
	fireSethronETB(gs, 0, bear)
	if got := countMinotaurTokens(gs, 0); got != 0 {
		t.Fatalf("non-Minotaur ETB must not make a token, got %d", got)
	}
}
