package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Generic "anthem" scaffold-kind coverage (dev/scaffold-kind-coverage-r63):
// "creatures [you control | opponents | all] get +X/+Y [until end of turn]".
// Before this handler the kind fell through inert in both dispatch sites.

func anthemTestGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Turn = 1
	gs.Active = 0
	return gs
}

func anthemSpellSrc(seat int) *Permanent {
	return &Permanent{
		Card:       &Card{Name: "Anthem Spell", Owner: seat},
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
	}
}

// Spell anthem, "creatures you control", until end of turn.
func TestAnthem_SpellBuffsYourCreatures(t *testing.T) {
	gs := anthemTestGame(t)
	mine := addKWCombatBattlefield(gs, 0, "Mine", 2, 2, "creature")
	theirs := addKWCombatBattlefield(gs, 1, "Theirs", 2, 2, "creature")

	e := &gameast.ModificationEffect{ModKind: "anthem", Args: []interface{}{1, 1, "until_eot"}}
	resolveModificationEffect(gs, anthemSpellSrc(0), e)

	if mine.Power() != 3 || mine.Toughness() != 3 {
		t.Errorf("your creature should be +1/+1 → 3/3, got %d/%d", mine.Power(), mine.Toughness())
	}
	if theirs.Power() != 2 || theirs.Toughness() != 2 {
		t.Errorf("opponent's creature must be unaffected by a 'you control' anthem, got %d/%d", theirs.Power(), theirs.Toughness())
	}
	if !hasEventKind(gs, "anthem") {
		t.Error("expected an anthem event")
	}
}

// Spell anthem, "opponents", negative P/T (e.g. -2/0 to opponents' creatures).
func TestAnthem_SpellDebuffsOpponents(t *testing.T) {
	gs := anthemTestGame(t)
	mine := addKWCombatBattlefield(gs, 0, "Mine", 2, 2, "creature")
	theirs := addKWCombatBattlefield(gs, 1, "Theirs", 3, 3, "creature")

	e := &gameast.ModificationEffect{ModKind: "anthem", Args: []interface{}{-2, 0, "opponents", "until_eot"}}
	resolveModificationEffect(gs, anthemSpellSrc(0), e)

	if theirs.Power() != 1 {
		t.Errorf("opponent creature should be -2/0 → power 1, got %d", theirs.Power())
	}
	if mine.Power() != 2 {
		t.Errorf("your creature must be unaffected by an 'opponents' anthem, got power %d", mine.Power())
	}
}

// Spell anthem, "all" creatures.
func TestAnthem_SpellAllCreatures(t *testing.T) {
	gs := anthemTestGame(t)
	mine := addKWCombatBattlefield(gs, 0, "Mine", 2, 2, "creature")
	theirs := addKWCombatBattlefield(gs, 1, "Theirs", 2, 2, "creature")

	e := &gameast.ModificationEffect{ModKind: "anthem", Args: []interface{}{1, 1, "all", "until_eot"}}
	resolveModificationEffect(gs, anthemSpellSrc(0), e)

	if mine.Power() != 3 || theirs.Power() != 3 {
		t.Errorf("'all' anthem should buff both: mine=%d theirs=%d (want 3/3)", mine.Power(), theirs.Power())
	}
}

// Static anthem on a permanent, exercised through the REAL dispatch
// (RegisterContinuousEffectsForPermanent → registerASTStaticEffects → the
// new "anthem" case): continuous +X/+Y to your creatures.
func TestAnthem_StaticContinuousBuff(t *testing.T) {
	gs := newFixtureGame(t)
	ast := &gameast.CardAST{
		Name: "Generic Anthem",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "anthem", Args: []interface{}{1, 1}, Layer: "7c",
			}},
		},
	}
	source := addBattlefieldWithAST(gs, 0, "Generic Anthem", 0, 0, ast, "enchantment")
	RegisterContinuousEffectsForPermanent(gs, source)

	mine := addBattlefield(gs, 0, "Mine", 2, 2, "creature")
	theirs := addBattlefield(gs, 1, "Theirs", 2, 2, "creature")

	mc := GetEffectiveCharacteristics(gs, mine)
	if mc.Power != 3 || mc.Toughness != 3 {
		t.Errorf("static anthem should continuously buff your creature to 3/3, got %d/%d", mc.Power, mc.Toughness)
	}
	tc := GetEffectiveCharacteristics(gs, theirs)
	if tc.Power != 2 {
		t.Errorf("opponent creature must be unaffected, got power %d", tc.Power)
	}
}

// Static anthem with the "other" scope excludes the source creature itself.
func TestAnthem_StaticOtherExcludesSelf(t *testing.T) {
	gs := newFixtureGame(t)
	ast := &gameast.CardAST{
		Name: "Anthem Lord",
		Abilities: []gameast.Ability{
			&gameast.Static{Modification: &gameast.Modification{
				ModKind: "anthem", Args: []interface{}{1, 1, "other"}, Layer: "7c",
			}},
		},
	}
	lord := addBattlefieldWithAST(gs, 0, "Anthem Lord", 2, 2, ast, "creature")
	RegisterContinuousEffectsForPermanent(gs, lord)
	buddy := addBattlefield(gs, 0, "Buddy", 2, 2, "creature")

	bc := GetEffectiveCharacteristics(gs, buddy)
	if bc.Power != 3 {
		t.Errorf("other creature should get +1/+1 → 3, got %d", bc.Power)
	}
	lc := GetEffectiveCharacteristics(gs, lord)
	if lc.Power != 2 {
		t.Errorf("'other' anthem must NOT buff the source itself, got %d", lc.Power)
	}
}
