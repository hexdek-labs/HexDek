package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Cyber Conversion: instant, turn target creature face down (it becomes a
// 2/2 Cyberman artifact creature). Handler targets the highest-power
// opponent creature.

func TestCyberConversion_TurnsBiggestOpponentCreatureFaceDown(t *testing.T) {
	gs := newGame(t, 2)
	small := addPerm(gs, 1, "Grizzly Bears", "creature")
	small.Card.BasePower, small.Card.BaseToughness = 2, 2
	big := addPerm(gs, 1, "Colossal Dreadmaw", "creature")
	big.Card.BasePower, big.Card.BaseToughness = 6, 6
	big.Card.CMC = 6

	card := addCard(gs, 0, "Cyber Conversion", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if !big.Card.FaceDown {
		t.Error("biggest opponent creature should be turned face down")
	}
	if big.FaceDownTemplate != "cyber" {
		t.Errorf("template=%q want cyber", big.FaceDownTemplate)
	}
	if small.Card.FaceDown {
		t.Error("only the targeted (biggest) creature should be face down")
	}
	chars := gameengine.GetEffectiveCharacteristics(gs, big)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Errorf("converted creature should be a 2/2, got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestCyberConversion_FizzlesWithNoOpponentCreature(t *testing.T) {
	gs := newGame(t, 2)
	// No opponent creatures; an OWN creature must not be auto-targeted.
	own := addPerm(gs, 0, "Grizzly Bears", "creature")
	own.Card.BasePower, own.Card.BaseToughness = 2, 2

	card := addCard(gs, 0, "Cyber Conversion", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if own.Card.FaceDown {
		t.Error("should not convert the caster's own creature when no opponent target exists")
	}
}
