package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// commander_background_r63_test.go — Background static grants (CR §702.124e).
//
// The commander_anthem handler used to (1) buff EVERY creature the controller
// controlled (wrong filter) and (2) read a flat +1/+1 off a raw-text arg
// (wrong values), so Raised by Giants made the whole board ~+1/+1 instead of
// setting the OWNER's commander creatures to base 10/10 Giants. These pin the
// fix: scoped to commander creatures you own, with the real P/T + type grant.

func bgCommanderGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(7)), nil)
	gs.CommanderFormat = true
	return gs
}

// raisedByGiantsAST builds the Background's commander_anthem static with the
// real raw-text arg the parser emits.
func raisedByGiantsAST() *gameast.CardAST {
	return &gameast.CardAST{
		Name: "Raised by Giants",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "commander_anthem",
				Args:    []interface{}{"base power and toughness 10/10 and are giants in addition to their other types"},
			}},
		},
	}
}

// (1) The owner's commander CREATURE gets base P/T 10/10 and the Giant type.
func TestBackground_CommanderAnthem_BuffsCommanderCreature(t *testing.T) {
	gs := bgCommanderGame(t)
	gs.Seats[0].CommanderNames = []string{"Wilson, Refined Grizzly"}

	bg := addBattlefieldWithAST(gs, 0, "Raised by Giants", 0, 0, raisedByGiantsAST(), "enchantment", "background")
	registerASTStaticEffects(gs, bg)

	cmdr := addBattlefield(gs, 0, "Wilson, Refined Grizzly", 4, 4, "creature")

	chars := GetEffectiveCharacteristics(gs, cmdr)
	if chars.Power != 10 || chars.Toughness != 10 {
		t.Fatalf("commander creature should be base P/T 10/10, got %d/%d", chars.Power, chars.Toughness)
	}
	if !charsHaveType(chars.Types, "giant") {
		t.Fatalf("commander creature should gain the Giant type, got types %v", chars.Types)
	}
}

// (2) THE FILTER FIX: a non-commander creature the controller controls is NOT
// affected — the old handler buffed it too.
func TestBackground_CommanderAnthem_DoesNotBuffNonCommander(t *testing.T) {
	gs := bgCommanderGame(t)
	gs.Seats[0].CommanderNames = []string{"Wilson, Refined Grizzly"}

	bg := addBattlefieldWithAST(gs, 0, "Raised by Giants", 0, 0, raisedByGiantsAST(), "enchantment", "background")
	registerASTStaticEffects(gs, bg)

	bear := addBattlefield(gs, 0, "Grizzly Bears", 2, 2, "creature")

	chars := GetEffectiveCharacteristics(gs, bear)
	if chars.Power != 2 || chars.Toughness != 2 {
		t.Fatalf("non-commander creature must be unaffected, got %d/%d", chars.Power, chars.Toughness)
	}
	if charsHaveType(chars.Types, "giant") {
		t.Fatal("non-commander creature must not gain the Giant type")
	}
}

// (3) A commander creature owned by a DIFFERENT player is not affected by your
// Background (the grant is "commander creatures YOU own").
func TestBackground_CommanderAnthem_OnlyOwnersCommander(t *testing.T) {
	gs := bgCommanderGame(t)
	gs.Seats[0].CommanderNames = []string{"Wilson, Refined Grizzly"}
	gs.Seats[1].CommanderNames = []string{"Other Commander"}

	bg := addBattlefieldWithAST(gs, 0, "Raised by Giants", 0, 0, raisedByGiantsAST(), "enchantment", "background")
	registerASTStaticEffects(gs, bg)

	oppCmdr := addBattlefield(gs, 1, "Other Commander", 3, 3, "creature")

	chars := GetEffectiveCharacteristics(gs, oppCmdr)
	if chars.Power != 3 || chars.Toughness != 3 {
		t.Fatalf("opponent's commander must be unaffected by your Background, got %d/%d", chars.Power, chars.Toughness)
	}
}

// (4) Additive "+X/+Y" form (future commander_anthem cards / boost clauses) is
// parsed as a boost, not a base set.
func TestBackground_CommanderAnthem_AdditiveBoost(t *testing.T) {
	gs := bgCommanderGame(t)
	gs.Seats[0].CommanderNames = []string{"Wilson, Refined Grizzly"}

	ast := &gameast.CardAST{
		Name: "Test Boost Background",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "commander_anthem",
				Args:    []interface{}{"+3/+3 and have flying"},
			}},
		},
	}
	bg := addBattlefieldWithAST(gs, 0, "Test Boost Background", 0, 0, ast, "enchantment", "background")
	registerASTStaticEffects(gs, bg)

	cmdr := addBattlefield(gs, 0, "Wilson, Refined Grizzly", 4, 4, "creature")
	chars := GetEffectiveCharacteristics(gs, cmdr)
	if chars.Power != 7 || chars.Toughness != 7 {
		t.Fatalf("additive +3/+3 on a 4/4 commander should be 7/7, got %d/%d", chars.Power, chars.Toughness)
	}
}
