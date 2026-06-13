package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mod_kind_self_calc_pt_test.go — regression pins for the generic
// self_calculated_pt CDA handler (worker hex-dev-5).

func selfCalcCreature(gs *GameState, seat int, name, phrase string) *Permanent {
	ast := &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "self_calculated_pt", Args: []interface{}{phrase}, Layer: "",
			}},
		},
	}
	p := addBattlefieldWithAST(gs, seat, name, 0, 0, ast, "creature")
	RegisterContinuousEffectsForPermanent(gs, p)
	return p
}

func TestSelfCalcPT_LandsYouControl(t *testing.T) {
	gs := newFixtureGame(t)
	goyf := selfCalcCreature(gs, 0, "Land Beast",
		"~'s power and toughness are each equal to the number of lands you control")
	addBattlefield(gs, 0, "Forest1", 0, 0, "land")
	addBattlefield(gs, 0, "Forest2", 0, 0, "land")
	addBattlefield(gs, 0, "Forest3", 0, 0, "land")
	c := GetEffectiveCharacteristics(gs, goyf)
	if c.Power != 3 || c.Toughness != 3 {
		t.Errorf("P/T should equal 3 lands, got %d/%d", c.Power, c.Toughness)
	}
}

func TestSelfCalcPT_DistinctCardTypesInAllGraveyards_Tarmogoyf(t *testing.T) {
	gs := newFixtureGame(t)
	goyf := selfCalcCreature(gs, 0, "Tarmogoyf",
		"~'s power is equal to the number of card types among cards in all graveyards and its toughness is equal to that number plus 1")
	// Toughness clause "plus 1" isn't parsed by us → we set power only;
	// toughness stays at the printed base (0). Pin the power behavior.
	gs.Seats[1].Graveyard = []*Card{
		{Name: "Bear", Owner: 1, Types: []string{"creature"}},
		{Name: "Bolt", Owner: 1, Types: []string{"instant"}},
		{Name: "Forest", Owner: 1, Types: []string{"land"}},
	}
	c := GetEffectiveCharacteristics(gs, goyf)
	if c.Power != 3 {
		t.Errorf("power should equal 3 distinct card types, got %d", c.Power)
	}
}

func TestSelfCalcPT_CardsInHand(t *testing.T) {
	gs := newFixtureGame(t)
	p := selfCalcCreature(gs, 0, "Maro",
		"~'s power and toughness are each equal to the number of cards in your hand")
	gs.Seats[0].Hand = []*Card{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}}
	c := GetEffectiveCharacteristics(gs, p)
	if c.Power != 4 || c.Toughness != 4 {
		t.Errorf("P/T should equal 4 cards in hand, got %d/%d", c.Power, c.Toughness)
	}
}

func TestSelfCalcPT_Dynamic(t *testing.T) {
	gs := newFixtureGame(t)
	p := selfCalcCreature(gs, 0, "Creature Beast",
		"~'s power and toughness are each equal to the number of creatures you control")
	// Only itself so far.
	if c := GetEffectiveCharacteristics(gs, p); c.Power != 1 {
		t.Fatalf("expected 1 (itself), got %d", c.Power)
	}
	addBattlefield(gs, 0, "Buddy", 2, 2, "creature")
	gs.InvalidateCharacteristicsCache()
	if c := GetEffectiveCharacteristics(gs, p); c.Power != 2 {
		t.Errorf("expected 2 creatures after a buddy enters, got %d", c.Power)
	}
}

func TestSelfCalcPT_UnknownPhraseSkipped(t *testing.T) {
	gs := newFixtureGame(t)
	p := selfCalcCreature(gs, 0, "Weird",
		"~'s power is equal to the number of llamas summoned on alternate tuesdays")
	c := GetEffectiveCharacteristics(gs, p)
	if c.Power != 0 {
		t.Errorf("unrecognized count phrase should be skipped (stay base 0), got %d", c.Power)
	}
}
