package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Monologue Tax: "Whenever an opponent casts their second spell each turn,
// you create a Treasure token." These pins prove the handler produces the
// Treasure (the ability was fully inert before — parsed to an untyped_effect
// scaffold with the trigger condition stripped).

func fireOppCast(gs *gameengine.GameState, casterSeat int, spellName string) {
	gameengine.FireCardTrigger(gs, "spell_cast_by_opponent", map[string]interface{}{
		"caster_seat": casterSeat,
		"spell_name":  spellName,
	})
}

func TestMonologueTax_SecondOpponentSpellMakesTreasure(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Monologue Tax", "enchantment")

	// Opponent's FIRST spell: no Treasure yet.
	fireOppCast(gs, 1, "Llanowar Elves")
	if got := gs.Seats[0].Turn.TreasuresCreated; got != 0 {
		t.Fatalf("after first opponent spell expected 0 Treasures, got %d", got)
	}

	// Opponent's SECOND spell: one Treasure for Monologue Tax's controller.
	fireOppCast(gs, 1, "Brainstorm")
	if got := gs.Seats[0].Turn.TreasuresCreated; got != 1 {
		t.Fatalf("after second opponent spell expected 1 Treasure, got %d", got)
	}

	// Third+ spells do NOT make further Treasures (only the second triggers).
	fireOppCast(gs, 1, "Counterspell")
	if got := gs.Seats[0].Turn.TreasuresCreated; got != 1 {
		t.Fatalf("after third opponent spell expected still 1 Treasure, got %d", got)
	}
}

func TestMonologueTax_OwnSpellsDoNotCount(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Monologue Tax", "enchantment")

	// Controller's own two spells must not trigger (it must be an OPPONENT).
	fireOppCast(gs, 0, "Sol Ring")
	fireOppCast(gs, 0, "Mana Vault")
	if got := gs.Seats[0].Turn.TreasuresCreated; got != 0 {
		t.Fatalf("own spells must not trigger Monologue Tax, got %d Treasures", got)
	}
}

func TestMonologueTax_PerOpponentAndTurnReset(t *testing.T) {
	gs := newGame(t, 4)
	addPerm(gs, 0, "Monologue Tax", "enchantment")

	// Each opponent's tally is independent: one spell each = no second spell.
	fireOppCast(gs, 1, "A")
	fireOppCast(gs, 2, "B")
	fireOppCast(gs, 3, "C")
	if got := gs.Seats[0].Turn.TreasuresCreated; got != 0 {
		t.Fatalf("one spell from each opponent must not trigger, got %d", got)
	}

	// Opponent 1's second spell triggers exactly one Treasure.
	fireOppCast(gs, 1, "A2")
	if got := gs.Seats[0].Turn.TreasuresCreated; got != 1 {
		t.Fatalf("opponent 1 second spell expected 1 Treasure, got %d", got)
	}

	// New turn resets the per-opponent tally.
	gs.Turn++
	gs.Seats[0].Turn.TreasuresCreated = 0
	fireOppCast(gs, 1, "next-turn-1")
	if got := gs.Seats[0].Turn.TreasuresCreated; got != 0 {
		t.Fatalf("first spell of new turn must not trigger, got %d", got)
	}
	fireOppCast(gs, 1, "next-turn-2")
	if got := gs.Seats[0].Turn.TreasuresCreated; got != 1 {
		t.Fatalf("second spell of new turn expected 1 Treasure, got %d", got)
	}
}
