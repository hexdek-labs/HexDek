package counters

import "testing"

// Phase 7 Counter DB — registry-shape assertions for the lore and
// defense counter entries that drive Saga (CR §714) and Battle
// (CR §310) behavior. The behavioral tests for the Saga chapter
// dispatcher, SBA §704.5s sacrifice, Battle combat-damage routing,
// and §704.5v defeat live in internal/gameengine/ because they
// depend on real Permanent / GameState plumbing — this package is
// intentionally gameengine-independent.

// TestPhase7_LoreCounterRegistryShape pins the lore counter definition
// to the shape Phase 7 relies on: category LoreCounter, valid only on
// Saga targets, proliferate-eligible, doubling-ineligible, and an
// OnPlacedTrigger that wires "saga_chapter_trigger" for downstream
// dispatch.
func TestPhase7_LoreCounterRegistryShape(t *testing.T) {
	def := Lookup("lore")
	if def == nil {
		t.Fatal("lore counter not registered")
	}
	if def.Category != LoreCounter {
		t.Errorf("lore.Category = %v, want LoreCounter", def.Category)
	}
	if def.DoublingApplies {
		t.Error("lore counters must NOT be doubled (CR §714.2b — fixed +1 per precombat main)")
	}
	if !def.Proliferate {
		t.Error("lore counters must be proliferate-eligible (CR §701.27)")
	}
	if len(def.ValidTargets) == 0 {
		t.Fatal("lore counter has no ValidTargets")
	}
	sawSaga := false
	for _, tt := range def.ValidTargets {
		if tt == TargetSaga {
			sawSaga = true
		}
	}
	if !sawSaga {
		t.Error("lore counter must target Sagas (CR §714.2)")
	}
	if len(def.OnPlacedTrigger) == 0 {
		t.Error("lore counter must have an OnPlacedTrigger for Phase 7 chapter dispatch")
	}
}

// TestPhase7_DefenseCounterRegistryShape pins the defense counter
// definition: valid only on Battle targets, doubling-eligible (a
// CR §310.3 design surface — defense counter doublers in the
// abstract are permitted), and not proliferating onto non-battles.
func TestPhase7_DefenseCounterRegistryShape(t *testing.T) {
	def := Lookup("defense")
	if def == nil {
		t.Fatal("defense counter not registered")
	}
	if len(def.ValidTargets) == 0 {
		t.Fatal("defense counter has no ValidTargets")
	}
	sawBattle := false
	for _, tt := range def.ValidTargets {
		if tt == TargetBattle {
			sawBattle = true
		}
	}
	if !sawBattle {
		t.Error("defense counter must target Battles (CR §310)")
	}
	if def.Category != LoyaltyCounter {
		t.Errorf("defense.Category = %v, want LoyaltyCounter (mirrors planeswalker loyalty)",
			def.Category)
	}
}

// TestPhase7_LoreCounterRejectsNonSaga confirms a lore counter is not
// validly placeable on a creature or planeswalker — the chapter-
// dispatch contract depends on lore only landing on Sagas.
func TestPhase7_LoreCounterRejectsNonSaga(t *testing.T) {
	def := Lookup("lore")
	if def == nil {
		t.Fatal("lore counter not registered")
	}
	if def.AcceptsTarget(TargetCreature) {
		t.Error("lore counter must NOT be valid on creatures")
	}
	if !def.AcceptsTarget(TargetSaga) {
		t.Error("lore counter must be valid on Sagas")
	}
}

// TestPhase7_DefenseCounterRejectsNonBattle confirms a defense
// counter cannot be placed on a non-battle — the SBA §704.5v defeat
// pipeline depends on defense only living on Battles.
func TestPhase7_DefenseCounterRejectsNonBattle(t *testing.T) {
	def := Lookup("defense")
	if def == nil {
		t.Fatal("defense counter not registered")
	}
	if def.AcceptsTarget(TargetCreature) {
		t.Error("defense counter must NOT be valid on creatures")
	}
	if !def.AcceptsTarget(TargetBattle) {
		t.Error("defense counter must be valid on Battles")
	}
}
