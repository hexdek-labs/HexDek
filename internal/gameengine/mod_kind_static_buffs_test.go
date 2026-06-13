package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// mod_kind_static_buffs_test.go — regression pins for the generic
// self_buff static handler (worker hex-dev-5).

// self_buff with integer args and an EMPTY layer tag (the real dataset
// shape) must still register a continuous +X/+Y on the source itself —
// exercising the staticKindAllowedLayerless gate relaxation.
func TestSelfBuff_StaticContinuousLayerless(t *testing.T) {
	gs := newFixtureGame(t)
	ast := &gameast.CardAST{
		Name: "Isleback Spawn",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "self_buff", Args: []interface{}{4, 8}, Layer: "",
			}},
		},
	}
	// Base 0/0 so the buff is the whole P/T.
	spawn := addBattlefieldWithAST(gs, 0, "Isleback Spawn", 0, 0, ast, "creature")
	RegisterContinuousEffectsForPermanent(gs, spawn)

	c := GetEffectiveCharacteristics(gs, spawn)
	if c.Power != 4 || c.Toughness != 8 {
		t.Errorf("self_buff should make this creature 4/8, got %d/%d", c.Power, c.Toughness)
	}
}

// The buff is scoped to the source only — another creature you control is
// unaffected.
func TestSelfBuff_DoesNotBuffOthers(t *testing.T) {
	gs := newFixtureGame(t)
	ast := &gameast.CardAST{
		Name: "Sedge Sliver",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "self_buff", Args: []interface{}{1, 1}, Layer: "",
			}},
		},
	}
	sliver := addBattlefieldWithAST(gs, 0, "Sedge Sliver", 2, 2, ast, "creature")
	RegisterContinuousEffectsForPermanent(gs, sliver)
	buddy := addBattlefield(gs, 0, "Buddy", 2, 2, "creature")

	sc := GetEffectiveCharacteristics(gs, sliver)
	if sc.Power != 3 || sc.Toughness != 3 {
		t.Errorf("self_buff source should be 3/3, got %d/%d", sc.Power, sc.Toughness)
	}
	bc := GetEffectiveCharacteristics(gs, buddy)
	if bc.Power != 2 {
		t.Errorf("self_buff must NOT touch another creature, got power %d", bc.Power)
	}
}

// The "+N/+0 for each …" scaling shape (raw string arg) is a no-op here,
// not a panic or a bogus +0/+0 that would mask the gap.
func TestSelfBuff_ForEachStringIsNoOp(t *testing.T) {
	gs := newFixtureGame(t)
	ast := &gameast.CardAST{
		Name: "Nim Lasher",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "self_buff",
				Args:    []interface{}{"this creature gets +1/+0 for each equipped"},
				Layer:   "",
			}},
		},
	}
	lasher := addBattlefieldWithAST(gs, 0, "Nim Lasher", 1, 1, ast, "creature")
	RegisterContinuousEffectsForPermanent(gs, lasher)

	c := GetEffectiveCharacteristics(gs, lasher)
	if c.Power != 1 || c.Toughness != 1 {
		t.Errorf("scaling self_buff should be a no-op (stay 1/1), got %d/%d", c.Power, c.Toughness)
	}
}
