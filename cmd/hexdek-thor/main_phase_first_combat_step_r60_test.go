package main

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestMainPhaseOrFirstCombat_StepIsBeginOfCombat pins the canonical
// "begin_of_combat" step name written by condScaffoldMainPhaseOrFirstCombat
// when the first_combat_phase subtype fires.
//
// Goldilocks R60 surfaced 2 TurnStructure invariant failures (Karlach,
// Fury of Avernus + Finest Hour) caused by the scaffold writing
// gs.Step="begin_combat", which is NOT in checkTurnStructure's allow-list
// for phase="combat". The canonical name per
// internal/gameengine/invariants.go:879 is "begin_of_combat" (or
// "beginning_of_combat" / "combat_start"); the sibling
// condScaffoldBeginningOfOrdinalStep handler at conditional_setup.go:4006
// already uses "begin_of_combat".
//
// This test pins both halves: the literal step name AND a full run of
// checkTurnStructure to prove the post-scaffold state is invariant-clean.
func TestMainPhaseOrFirstCombat_StepIsBeginOfCombat(t *testing.T) {
	gs := newTestGameState(2)
	cs := conditionScaffold{
		kind:    condScaffoldMainPhaseOrFirstCombat,
		subtype: "first_combat_phase",
	}
	// Trigger the apply path via a synthetic raw-text condition that maps
	// to first_combat_phase. (Detection routes "first combat phase" to the
	// first_combat_phase subtype.)
	cond := &gameast.Condition{
		Kind: "raw",
		Args: []interface{}{"if it's the first combat phase of the turn"},
	}
	_ = cs // referenced for documentation; apply runs through detect.
	applied := applyConditionScaffolding(gs, cond, nil)

	if applied.kind != condScaffoldMainPhaseOrFirstCombat {
		t.Fatalf("expected MainPhaseOrFirstCombat scaffold, got %v", applied.kind)
	}
	if applied.subtype != "first_combat_phase" {
		t.Fatalf("expected subtype=first_combat_phase, got %q", applied.subtype)
	}
	if gs.Phase != "combat" {
		t.Errorf("Phase: want %q, got %q", "combat", gs.Phase)
	}
	if gs.Step != "begin_of_combat" {
		t.Errorf("Step: want %q, got %q (the old 'begin_combat' typo trips TurnStructure)",
			"begin_of_combat", gs.Step)
	}
	if gs.Flags["first_combat_phase"] != 1 {
		t.Errorf("Flags[first_combat_phase] want 1, got %d", gs.Flags["first_combat_phase"])
	}

	// End-to-end pin: every invariant including TurnStructure must be
	// clean after the scaffold runs. This is the assertion that
	// previously failed on Karlach + Finest Hour during goldilocks R60.
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		if v.Name == "TurnStructure" {
			t.Fatalf("TurnStructure invariant tripped post-scaffold: %s", v.Message)
		}
		// Other invariants (e.g. ZoneCastGrantExpiry) may legitimately
		// fire on a freshly-constructed test state; we only care that
		// the scaffold's own state writes don't break TurnStructure.
	}
}

// TestMainPhaseOrFirstCombat_DefaultSubtypeIsMainPhase pins the
// default (non-first-combat) branch so the fix doesn't accidentally
// regress the main-phase path.
func TestMainPhaseOrFirstCombat_DefaultSubtypeIsMainPhase(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{
		Kind: "raw",
		Args: []interface{}{"if it's your main phase"},
	}
	applied := applyConditionScaffolding(gs, cond, nil)

	if applied.kind != condScaffoldMainPhaseOrFirstCombat {
		t.Fatalf("expected MainPhaseOrFirstCombat scaffold, got %v", applied.kind)
	}
	if gs.Phase != "precombat_main" {
		t.Errorf("Phase: want %q, got %q", "precombat_main", gs.Phase)
	}
	if gs.Step != "main" {
		t.Errorf("Step: want %q, got %q", "main", gs.Step)
	}
	if gs.Flags["main_phase"] != 1 {
		t.Errorf("Flags[main_phase] want 1, got %d", gs.Flags["main_phase"])
	}

	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		if v.Name == "TurnStructure" {
			t.Fatalf("TurnStructure invariant tripped post-scaffold: %s", v.Message)
		}
	}
}

// TestMainPhaseOrFirstCombat_StepNotInvariantAllowList is a defensive
// regression: if anyone reintroduces "begin_combat" (or any other
// non-allow-listed step) at conditional_setup.go:5023, the next thor
// build will fail this test BEFORE goldilocks surfaces it as 17+ card
// invariant violations.
func TestMainPhaseOrFirstCombat_StepNotInvariantAllowList(t *testing.T) {
	gs := newTestGameState(2)
	cond := &gameast.Condition{
		Kind: "raw",
		Args: []interface{}{"if it's the first combat phase of the turn"},
	}
	applyConditionScaffolding(gs, cond, nil)

	// The validator's allow-list for phase="combat" — kept in sync with
	// internal/gameengine/invariants.go:878-886 by reading the same
	// invariant gate that goldilocks runs.
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		if v.Name == "TurnStructure" && strings.Contains(v.Message, "step ") {
			t.Fatalf("scaffold wrote invariant-invalid step: %s", v.Message)
		}
	}
}
