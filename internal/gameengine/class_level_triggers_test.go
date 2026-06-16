package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// class_level_triggers_test.go — r63 Class-enchantment probe (CR §716.2c).
//
// Before this fix the production level-up path (the AST `level_marker` resolve
// handler) recorded the new level but fired NO "becomes level N" trigger, so
// every generic `class_becomes_level` payoff in the corpus was inert (and the 6
// per_card class_level_up consumers never fired in real games, since the only
// emitters of that trigger were the dead LevelUpClass / AdvanceClassLevel
// helpers). These pin the generic AST dispatch on the production path.

// classWithBecomesLevel builds a Class enchantment whose level-2 activated
// ability (level_marker 2) and "When this Class becomes level 2, draw N" AST
// trigger are present, mirroring the real Wizard/Cleric/Druid shape.
func classWithBecomesLevel(gs *GameState, seat int, name string, level int, drawN int) *Permanent {
	ast := &gameast.CardAST{Abilities: []gameast.Ability{
		// "When this Class becomes level N, draw drawN cards." (The level-N
		// activated ability's effect — a level_marker — is resolved directly
		// by the test via ResolveEffect to drive the production path.)
		&gameast.Triggered{
			Trigger: gameast.Trigger{Event: "class_becomes_level", Phase: itoaLevel(level)},
			Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: drawN}},
			Raw:     "when this class becomes level draw",
		},
	}}
	return addBattlefieldWithAST(gs, seat, name, 0, 0, ast, "enchantment", "class")
}

func itoaLevel(n int) string {
	switch n {
	case 2:
		return "2"
	case 3:
		return "3"
	default:
		return "1"
	}
}

// The generic path: a Class with NO per_card handler whose level-2 ability
// resolves must fire its AST "becomes level 2" trigger (here, draw 2) and sync
// the canonical class-level field.
func TestClassBecomesLevel_GenericASTTriggerFires(t *testing.T) {
	gs := newCombatGame(t)
	addLibrary(gs, 0, "A", "B", "C", "D")
	cls := classWithBecomesLevel(gs, 0, "Test Generic Class", 2, 2)

	startHand := len(gs.Seats[0].Hand)

	// Resolve the level-2 activated ability's effect (the production path).
	ResolveEffect(gs, cls, &gameast.ModificationEffect{ModKind: "level_marker", Args: []interface{}{2}})
	// Resolve the queued becomes-level trigger.
	DrainStack(gs)

	if cls.Counters["level"] != 2 {
		t.Fatalf("level counter = %d, want 2", cls.Counters["level"])
	}
	if cls.Flags["class_level"] != 2 {
		t.Fatalf("Flags[class_level] = %d, want 2 (canonical field must be synced)", cls.Flags["class_level"])
	}
	if got := len(gs.Seats[0].Hand) - startHand; got != 2 {
		t.Fatalf("becomes-level-2 trigger should draw 2; drew %d", got)
	}
}

// A 1→2 level-up must fire ONLY the becomes-level-2 trigger, never a spurious
// "becomes level 1" (a fresh Class enters at level 1 with Counters["level"]==0
// on this path; the previous level is floored at 1 per §716.1).
func TestClassBecomesLevel_NoSpuriousLevelOne(t *testing.T) {
	gs := newCombatGame(t)
	addLibrary(gs, 0, "A", "B", "C", "D")
	// Two triggers: one for level 1, one for level 2. Only level 2 should fire.
	ast := &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Triggered{
			Trigger: gameast.Trigger{Event: "class_becomes_level", Phase: "1"},
			Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 3}},
			Raw:     "becomes level 1 draw 3",
		},
		&gameast.Triggered{
			Trigger: gameast.Trigger{Event: "class_becomes_level", Phase: "2"},
			Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}},
			Raw:     "becomes level 2 draw 1",
		},
	}}
	cls := addBattlefieldWithAST(gs, 0, "Two Band Class", 0, 0, ast, "enchantment", "class")

	ResolveEffect(gs, cls, &gameast.ModificationEffect{ModKind: "level_marker", Args: []interface{}{2}})
	DrainStack(gs)

	// Only the level-2 trigger (draw 1) should have fired, not level-1 (draw 3).
	if got := len(gs.Seats[0].Hand); got != 1 {
		t.Fatalf("only becomes-level-2 should fire (draw 1); hand=%d (a draw-3 means a spurious level-1 fired)", got)
	}
}

// A non-class leveler (Level Up creature) must NOT route through the class
// dispatch — its level_marker still updates the level counter / bracket but
// fires no class_becomes_level trigger.
func TestClassBecomesLevel_NonClassUnaffected(t *testing.T) {
	gs := newCombatGame(t)
	ast := &gameast.CardAST{Abilities: []gameast.Ability{
		// A creature with a class_becomes_level-shaped trigger but NOT the
		// class subtype — must be ignored by the class dispatch.
		&gameast.Triggered{
			Trigger: gameast.Trigger{Event: "class_becomes_level", Phase: "2"},
			Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 5}},
			Raw:     "becomes level 2 draw 5",
		},
	}}
	addLibrary(gs, 0, "A", "B", "C", "D", "E")
	creature := addBattlefieldWithAST(gs, 0, "Not A Class", 2, 2, ast, "creature")

	ResolveEffect(gs, creature, &gameast.ModificationEffect{ModKind: "level_marker", Args: []interface{}{2}})
	DrainStack(gs)

	if got := len(gs.Seats[0].Hand); got != 0 {
		t.Fatalf("a non-class permanent must not fire class_becomes_level; drew %d", got)
	}
	if creature.Flags["class_level"] != 0 {
		t.Fatalf("a non-class permanent must not get a class_level flag, got %d", creature.Flags["class_level"])
	}
}
