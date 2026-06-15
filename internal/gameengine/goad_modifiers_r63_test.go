package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// TestGoad_GenericResolvePath_StampsGoaderAndExpiry pins the r63 headline fix:
// the generic AST goad resolution (ModKind "goad") now routes through
// GoadCreature, so the goad carries the goader's seat (CR §701.39 "attack a
// player other than you") AND an expiry ("until your next turn"). Before the
// fix it set a bare Flags["goaded"]=1 — goader-less and permanent.
func TestGoad_GenericResolvePath_StampsGoaderAndExpiry(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Turn = 10
	src := addBattlefield(gs, 0, "Disrupt Decorum", 0, 0, "sorcery")
	target := addBattlefield(gs, 1, "Bear", 2, 2, "creature")

	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "goad",
		Args:    []interface{}{"target_creature_opponent"},
	})

	if _, ok := target.Flags["goaded_until_turn"]; !ok {
		t.Fatal("generic goad must set an expiry (goaded_until_turn) — was permanent before r63")
	}
	if target.Flags["goaded_until_turn"] <= gs.Turn {
		t.Fatalf("goad expiry must be in the future, got %d at turn %d", target.Flags["goaded_until_turn"], gs.Turn)
	}
	if target.Flags["goaded_by_seat"] != 0 {
		t.Fatalf("generic goad must record the goader seat (0), got %d", target.Flags["goaded_by_seat"])
	}
	// The goader (seat 0) must be a forbidden defender per §701.39.
	if !CannotAttackGoader(gs, target, 0) {
		t.Fatal("the goader (seat 0) must be a forbidden attack target")
	}
}

// TestGoad_MultiGoader_FiltersAllGoaders pins CR §701.39b: a creature goaded
// by multiple players must attack a player other than ANY of them if able.
func TestGoad_MultiGoader_FiltersAllGoaders(t *testing.T) {
	gs := newGoadGame(t, 4)
	creature := putGoadBattlefield(gs, 0, newGoadCreature("Wurm", 0))

	GoadCreature(gs, 1, creature)
	GoadCreature(gs, 2, creature)

	gd := activeGoaders(creature, gs.Turn)
	if len(gd) != 2 {
		t.Fatalf("both goaders should be active, got %v", gd)
	}
	// Defender pool [1,2,3] → only seat 3 is a non-goader.
	got := filterGoaderFromDefenders(gs, creature, []int{1, 2, 3})
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("multi-goader must leave only the non-goader (seat 3), got %v", got)
	}
	if !CannotAttackGoader(gs, creature, 1) || !CannotAttackGoader(gs, creature, 2) {
		t.Fatal("both goaders (1 and 2) must be forbidden defenders")
	}
}

// TestGoad_MultiGoader_IndependentExpiry pins that each goad wears off on its
// own goader's next turn: an expired goad stops restricting while a still-live
// one keeps restricting.
func TestGoad_MultiGoader_IndependentExpiry(t *testing.T) {
	gs := newGoadGame(t, 4)
	creature := putGoadBattlefield(gs, 0, newGoadCreature("Wurm", 0))
	// Seat 1's goad expires at turn 12; seat 2's at turn 15.
	creature.Flags = map[string]int{
		"goad_from_1":       12,
		"goad_from_2":       15,
		"goaded_by_seat":    2,
		"goaded_until_turn": 15,
		"goaded":            1,
	}
	gs.Turn = 13 // seat 1's goad has expired; seat 2's is still active.

	gd := activeGoaders(creature, gs.Turn)
	if len(gd) != 1 || gd[0] != 2 {
		t.Fatalf("only seat 2's goad should still be active at turn 13, got %v", gd)
	}
	// Seat 1 is attackable again; seat 2 is still forbidden.
	got := filterGoaderFromDefenders(gs, creature, []int{1, 2, 3})
	if len(got) != 2 {
		t.Fatalf("only seat 2 should be filtered, got %v", got)
	}
	for _, d := range got {
		if d == 2 {
			t.Fatalf("seat 2 (still goading) must be filtered out, got %v", got)
		}
	}
}

// TestGoad_IfAbleEscape_AllOppsAreGoaders pins the "if able" escape: when every
// living opponent goaded the creature, it may attack one of them after all.
func TestGoad_IfAbleEscape_AllOppsAreGoaders(t *testing.T) {
	gs := newGoadGame(t, 4)
	creature := putGoadBattlefield(gs, 0, newGoadCreature("Wurm", 0))
	GoadCreature(gs, 1, creature)
	GoadCreature(gs, 2, creature)

	// Only seats 1 and 2 are living opponents — both goaders. Filtering would
	// empty the pool, so the escape returns the unfiltered list.
	got := filterGoaderFromDefenders(gs, creature, []int{1, 2})
	if len(got) != 2 {
		t.Fatalf("if-able escape: with only goaders as opponents, must return all, got %v", got)
	}
}

// TestGoad_EnforceMustAttack_OnlyForcesAble pins property (e): a goaded
// creature that CAN attack (in the legal pool) is force-added, but a goaded
// creature that CAN'T attack (absent from legal) is not forced.
func TestGoad_EnforceMustAttack_OnlyForcesAble(t *testing.T) {
	gs := newGoadGame(t, 4)
	gs.Active = 0
	able := putGoadBattlefield(gs, 0, newGoadCreature("Able", 0))
	unable := putGoadBattlefield(gs, 0, newGoadCreature("Unable", 0))
	GoadCreature(gs, 1, able)
	GoadCreature(gs, 1, unable)

	// legal contains only `able` (e.g. `unable` is tapped / has defender /
	// summoning-sick, so canAttack excluded it).
	out := enforceGoadMustAttack(gs, nil, []*Permanent{able})
	if len(out) != 1 || out[0] != able {
		t.Fatalf("able goaded creature must be force-added, got %v", out)
	}
	for _, p := range out {
		if p == unable {
			t.Fatal("a goaded creature that can't attack (absent from legal) must NOT be forced")
		}
	}
}
