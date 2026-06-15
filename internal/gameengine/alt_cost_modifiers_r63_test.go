package gameengine

import "testing"

// TestEffectiveAlternativeCost_TaxApplies pins CR §118.9 / §601.2f: cost
// INCREASES (Thalia, Sphere of Resistance) apply to an alternative cost too.
// Before r63 the canonical cast path replaced the modifier-adjusted base cost
// with the raw alternative cost, so an overloaded/surged/spectacle/flashback
// spell dodged every tax and reducer.
func TestEffectiveAlternativeCost_TaxApplies(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Sphere of Resistance: all spells cost {1} more.
	addPerm(gs, 1, "Sphere of Resistance", []string{"artifact"})

	spell := &Card{Name: "Overloaded Bolt", Types: []string{"instant"}, ManaCostString: "{1}{R}"}
	// Raw alternative (overload) cost of 4 → taxed to 5.
	got := EffectiveAlternativeCost(gs, spell, 0, 4, CastContext{})
	if got != 5 {
		t.Fatalf("Sphere of Resistance must tax the alternative cost: want 5, got %d", got)
	}
}

// TestEffectiveAlternativeCost_ReductionApplies pins that cost REDUCTIONS
// (Helm of Awakening: spells cost {1} less) apply to an alternative cost.
func TestEffectiveAlternativeCost_ReductionApplies(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	addPerm(gs, 0, "Helm of Awakening", []string{"artifact"})

	spell := &Card{Name: "Overloaded Wrath", Types: []string{"sorcery"}, ManaCostString: "{3}{R}"}
	// Raw alternative cost 6 → reduced by 1 → 5.
	got := EffectiveAlternativeCost(gs, spell, 0, 6, CastContext{})
	if got != 5 {
		t.Fatalf("Helm of Awakening must reduce the alternative cost: want 5, got %d", got)
	}
}

// TestEffectiveAlternativeCost_NoModifiersUnchanged pins that with no cost
// modifiers present the alternative cost is returned unchanged, and that a
// zero / parser-missing alternative cost is never taxed up from nothing.
func TestEffectiveAlternativeCost_NoModifiersUnchanged(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	spell := &Card{Name: "Plain Spell", Types: []string{"instant"}, ManaCostString: "{2}{U}"}
	if got := EffectiveAlternativeCost(gs, spell, 0, 4, CastContext{}); got != 4 {
		t.Fatalf("no modifiers: alt cost should be unchanged, want 4, got %d", got)
	}

	// Tax present but rawAlt is 0 (parser-missing): must stay 0, not become 1.
	addPerm(gs, 1, "Sphere of Resistance", []string{"artifact"})
	if got := EffectiveAlternativeCost(gs, spell, 0, 0, CastContext{}); got != 0 {
		t.Fatalf("zero/parser-missing alt cost must not be taxed up, want 0, got %d", got)
	}
}

// TestEffectiveAlternativeCost_FlooredAtZero pins that a reduction larger than
// the alternative cost floors at 0 (not negative).
func TestEffectiveAlternativeCost_FlooredAtZero(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	addPerm(gs, 0, "Helm of Awakening", []string{"artifact"})
	spell := &Card{Name: "Cheap Overload", Types: []string{"instant"}, ManaCostString: "{R}"}
	// Raw alt cost 1, reduced by 1 → floored at 0 (not -? / not below 0).
	if got := EffectiveAlternativeCost(gs, spell, 0, 1, CastContext{}); got != 0 {
		t.Fatalf("reduction must floor at 0, want 0, got %d", got)
	}
}
