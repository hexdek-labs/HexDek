package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// living_weapon_r63_test.go — Living weapon (CR §702.91, New Phyrexia).
// ApplyLivingWeapon existed but had NO caller — living weapon was inert (no
// Germ token, no auto-attach). These pin: 0/0 black Germ via the canonical
// token path, auto-attach, the equip buff making the Germ a real creature,
// token_created firing, the token doubler, and the unequip → 0/0 SBA death.

// livingWeaponAST: a +4/+4 living-weapon equipment (Batterskull-shaped).
func livingWeaponAST(name string) *gameast.CardAST {
	return &gameast.CardAST{
		Name: name,
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "living weapon"},
			&gameast.Static{Modification: &gameast.Modification{ModKind: "equip_buff_grant", Args: []interface{}{4, 4}}},
		},
	}
}

func findGerm(gs *GameState, seat int) *Permanent {
	for _, p := range gs.Seats[seat].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Phyrexian Germ Token" {
			return p
		}
	}
	return nil
}

// (1)+(2)+(3) Living weapon equipment entering creates a 0/0 black Phyrexian
// Germ token, auto-attaches, and the +4/+4 makes the Germ a real 4/4.
func TestLivingWeapon_CreatesGermAttachesAndBuffs(t *testing.T) {
	gs := newCombatGame(t)
	eq := addBattlefieldWithAST(gs, 0, "Batterskull", 0, 0, livingWeaponAST("Batterskull"), "artifact", "equipment")
	FirePermanentETBTriggers(gs, eq)

	germ := findGerm(gs, 0)
	if germ == nil {
		t.Fatal("living weapon should create a Phyrexian Germ token")
	}
	if !germ.IsCreature() {
		t.Fatal("the Germ should be a creature")
	}
	if len(germ.Card.Colors) != 1 || germ.Card.Colors[0] != "B" {
		t.Fatalf("the Germ should be black, got %v", germ.Card.Colors)
	}
	if !cardHasType(germ.Card, "token") {
		t.Fatal("the Germ should be a token")
	}
	if eq.AttachedTo != germ {
		t.Fatal("the equipment should auto-attach to the Germ")
	}
	chars := GetEffectiveCharacteristics(gs, germ)
	if chars.Power != 4 || chars.Toughness != 4 {
		t.Fatalf("with the +4/+4 equip buff the Germ should be 4/4, got %d/%d", chars.Power, chars.Toughness)
	}
}

// token_created fires for the Germ (canonical token path, not a raw loop).
func TestLivingWeapon_FiresTokenCreated(t *testing.T) {
	prev := TriggerHook
	defer func() { TriggerHook = prev }()
	fired := 0
	TriggerHook = func(gs *GameState, ev string, ctx map[string]interface{}) {
		if ev == "token_created" {
			fired++
		}
	}
	gs := newCombatGame(t)
	eq := addBattlefieldWithAST(gs, 0, "Batterskull", 0, 0, livingWeaponAST("Batterskull"), "artifact", "equipment")
	FirePermanentETBTriggers(gs, eq)
	if fired == 0 {
		t.Fatal("living weapon Germ creation should fire token_created")
	}
}

// Token doubler (Doubling Season) makes two Germs (the equipment attaches to one).
func TestLivingWeapon_TokenDoublerMakesTwoGerms(t *testing.T) {
	gs := newCombatGame(t)
	addDoublerSource(gs, 0, "Doubling Season", RegisterDoublingSeason)
	eq := addBattlefieldWithAST(gs, 0, "Batterskull", 0, 0, livingWeaponAST("Batterskull"), "artifact", "equipment")
	FirePermanentETBTriggers(gs, eq)

	germs := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Phyrexian Germ Token" {
			germs++
		}
	}
	if germs != 2 {
		t.Fatalf("living weapon with Doubling Season should make 2 Germs, got %d", germs)
	}
	if eq.AttachedTo == nil {
		t.Fatal("the equipment should still attach to a Germ")
	}
}

// (4) When the equipment leaves, the Germ reverts to 0/0 and dies as an SBA.
func TestLivingWeapon_GermDiesWhenEquipmentLeaves(t *testing.T) {
	gs := newCombatGame(t)
	eq := addBattlefieldWithAST(gs, 0, "Batterskull", 0, 0, livingWeaponAST("Batterskull"), "artifact", "equipment")
	FirePermanentETBTriggers(gs, eq)

	germ := findGerm(gs, 0)
	if germ == nil {
		t.Fatal("precondition: Germ should exist")
	}

	// Equipment leaves the battlefield → its +4/+4 stops applying → 0/0 Germ.
	removePermanentFromBattlefield(gs, eq)
	StateBasedActions(gs)

	if findGerm(gs, 0) != nil {
		t.Fatal("the 0/0 Germ should die as a state-based action once the equipment is gone")
	}
}
