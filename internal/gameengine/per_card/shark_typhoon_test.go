package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Shark Typhoon: noncreature cast → X/X blue flying Shark (X = spell mv).
// Previously inert (parsed_effect_residual).

func countTokensNamed(gs *gameengine.GameState, seat int, name string) int {
	n := 0
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == name {
			n++
		}
	}
	return n
}

func fireNoncreatureCast(gs *gameengine.GameState, casterSeat int, spell *gameengine.Card) {
	gameengine.FireCardTrigger(gs, "noncreature_spell_cast", map[string]interface{}{
		"caster_seat": casterSeat,
		"card":        spell,
		"spell_name":  spell.DisplayName(),
	})
}

func TestSharkTyphoon_MakesXXShark(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Shark Typhoon", "enchantment")

	spell := &gameengine.Card{Name: "Cryptic Command", Owner: 0, CMC: 4, Types: []string{"instant"}}
	fireNoncreatureCast(gs, 0, spell)

	sharks := 0
	var shark *gameengine.Permanent
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Shark" {
			sharks++
			shark = p
		}
	}
	if sharks != 1 {
		t.Fatalf("expected 1 Shark token, got %d", sharks)
	}
	if shark.Power() != 4 || shark.Toughness() != 4 {
		t.Fatalf("expected 4/4 Shark, got %d/%d", shark.Power(), shark.Toughness())
	}
	if shark.Flags["kw:flying"] != 1 {
		t.Fatalf("expected Shark to have flying")
	}
}

func TestSharkTyphoon_OnlyControllerNoncreature(t *testing.T) {
	gs := newGame(t, 2)
	addPerm(gs, 0, "Shark Typhoon", "enchantment")

	// Opponent's noncreature cast does not trigger.
	spell := &gameengine.Card{Name: "Opt", Owner: 1, CMC: 1, Types: []string{"instant"}}
	fireNoncreatureCast(gs, 1, spell)
	if countTokensNamed(gs, 0, "Shark") != 0 {
		t.Fatalf("opponent's cast must not make a Shark for the controller")
	}
}
