package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// The Meathook Massacre: ETB -X/-X to all creatures (X = mana paid). The wipe
// was inert (parsed_effect_residual); only the death-drains worked.

func TestMeathookMassacre_AppliesMinusXMinusXToAllCreatures(t *testing.T) {
	gs := newGame(t, 2)
	small := addPerm(gs, 1, "Goblin", "creature")
	small.Card.BasePower, small.Card.BaseToughness = 2, 2
	big := addPerm(gs, 1, "Ogre", "creature")
	big.Card.BasePower, big.Card.BaseToughness = 4, 4
	mine := addPerm(gs, 0, "My Bear", "creature")
	mine.Card.BasePower, mine.Card.BaseToughness = 3, 3

	meathook := addPerm(gs, 0, "The Meathook Massacre", "enchantment", "legendary")
	meathook.Flags["x_paid"] = 3

	gameengine.InvokeETBHook(gs, meathook)

	// Every creature should have effective toughness reduced by 3 (symmetric).
	if got := gs.ToughnessOf(big); got != 1 {
		t.Fatalf("Ogre 4/4 should be 4-3=1 toughness, got %d", got)
	}
	if got := gs.ToughnessOf(small); got > 0 {
		t.Fatalf("Goblin 2/2 should be at <=0 toughness (lethal), got %d", got)
	}
	if got := gs.ToughnessOf(mine); got != 0 {
		t.Fatalf("own Bear 3/3 should be 3-3=0 (symmetric wipe), got %d", got)
	}
}

func TestMeathookMassacre_ZeroXNoWipe(t *testing.T) {
	gs := newGame(t, 2)
	bear := addPerm(gs, 1, "Bear", "creature")
	bear.Card.BasePower, bear.Card.BaseToughness = 2, 2
	meathook := addPerm(gs, 0, "The Meathook Massacre", "enchantment", "legendary")
	// x_paid unset (0) → no wipe.
	gameengine.InvokeETBHook(gs, meathook)
	if gs.ToughnessOf(bear) != 2 {
		t.Fatalf("with X=0 no creature should be modified, bear toughness=%d", gs.ToughnessOf(bear))
	}
}
