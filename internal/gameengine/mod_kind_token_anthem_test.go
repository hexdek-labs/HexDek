package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Generic "token_anthem" scaffold-kind coverage (dev/scaffold-kind-self-r63):
// "Creature tokens [you control | all] get +X/+Y" — previously inert.
// Exercised through the real static dispatch (RegisterContinuousEffectsForPermanent
// → registerASTStaticEffects → the new "token_anthem" case).

func tokenAnthemAST(pow, tough int) *gameast.CardAST {
	return &gameast.CardAST{
		Name: "Token Anthem Source",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "token_anthem", Args: []interface{}{pow, tough}, Layer: "7c",
			}},
		},
	}
}

// Positive token anthem buffs the controller's creature TOKENS only — not
// nontoken creatures, not opponents' tokens.
func TestTokenAnthem_PositiveBuffsYourTokens(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefieldWithAST(gs, 0, "Token Anthem Source", 0, 0, tokenAnthemAST(1, 1), "enchantment")
	RegisterContinuousEffectsForPermanent(gs, src)

	myToken := addBattlefield(gs, 0, "My Token", 1, 1, "token", "creature")
	myNontoken := addBattlefield(gs, 0, "My Real Creature", 2, 2, "creature")
	oppToken := addBattlefield(gs, 1, "Opp Token", 1, 1, "token", "creature")

	if c := GetEffectiveCharacteristics(gs, myToken); c.Power != 2 || c.Toughness != 2 {
		t.Errorf("your creature token should be +1/+1 → 2/2, got %d/%d", c.Power, c.Toughness)
	}
	if c := GetEffectiveCharacteristics(gs, myNontoken); c.Power != 2 {
		t.Errorf("nontoken creature must be unaffected, got power %d", c.Power)
	}
	if c := GetEffectiveCharacteristics(gs, oppToken); c.Power != 1 {
		t.Errorf("opponent's token must be unaffected by a positive token anthem, got power %d", c.Power)
	}
}

// Negative token anthem (token hate, e.g. Illness in the Ranks / Virulent
// Plague) shrinks ALL creature tokens, both players'.
func TestTokenAnthem_NegativeHitsAllTokens(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefieldWithAST(gs, 0, "Virulent Plague", 0, 0, tokenAnthemAST(-2, -2), "enchantment")
	RegisterContinuousEffectsForPermanent(gs, src)

	myToken := addBattlefield(gs, 0, "My Token", 2, 2, "token", "creature")
	oppToken := addBattlefield(gs, 1, "Opp Token", 2, 2, "token", "creature")
	oppNontoken := addBattlefield(gs, 1, "Opp Real Creature", 2, 2, "creature")

	if c := GetEffectiveCharacteristics(gs, myToken); c.Power != 0 || c.Toughness != 0 {
		t.Errorf("token-hate should shrink your token to 0/0, got %d/%d", c.Power, c.Toughness)
	}
	if c := GetEffectiveCharacteristics(gs, oppToken); c.Power != 0 || c.Toughness != 0 {
		t.Errorf("token-hate should shrink opponent token to 0/0, got %d/%d", c.Power, c.Toughness)
	}
	if c := GetEffectiveCharacteristics(gs, oppNontoken); c.Power != 2 {
		t.Errorf("nontoken creature must be unaffected by token hate, got power %d", c.Power)
	}
}
