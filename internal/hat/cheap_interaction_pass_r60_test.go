package hat

import (
	"testing"
)

// TestCheapInteractionPassAdjust pins the new CheapInteraction-driven
// passBoost adjustment introduced in the freya-hat integration audit
// (2026-05-30). The audit found that StrategyProfile.CheapInteraction
// was loaded by strategy_loader.go but consumed by no decision code,
// despite the field's docstring claiming it should drive hold-mana
// behavior. The helper cheapInteractionPassAdjust closes that gap and
// is integrated into the priority-round passBoost calculation in
// yggdrasil.go.
func TestCheapInteractionPassAdjust(t *testing.T) {
	cases := []struct {
		name string
		sp   *StrategyProfile
		want float64
	}{
		{
			name: "nil strategy -> 0 (legacy / no-strategy hat)",
			sp:   nil,
			want: 0,
		},
		{
			name: "0 cheap interaction -> -0.15 (deck cannot interact)",
			sp:   &StrategyProfile{CheapInteraction: 0},
			want: -0.15,
		},
		{
			name: "1 cheap interaction -> 0 (below 'stocked package' threshold)",
			sp:   &StrategyProfile{CheapInteraction: 1},
			want: 0,
		},
		{
			name: "3 cheap interaction -> 0 (still below 4-piece threshold)",
			sp:   &StrategyProfile{CheapInteraction: 3},
			want: 0,
		},
		{
			name: "4 cheap interaction -> +0.10 (stocked interaction package)",
			sp:   &StrategyProfile{CheapInteraction: 4},
			want: 0.10,
		},
		{
			name: "8 cheap interaction -> +0.10 (cap at the threshold)",
			sp:   &StrategyProfile{CheapInteraction: 8},
			want: 0.10,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cheapInteractionPassAdjust(tc.sp)
			if got != tc.want {
				t.Errorf("cheapInteractionPassAdjust(%+v): want %v, got %v",
					tc.sp, tc.want, got)
			}
		})
	}
}

// TestCheapInteractionPassAdjust_OverridesArchetypeStance verifies the
// motivating use-case: a Control archetype labeled deck shipped with
// 0 cheap interaction (an unusual but possible minimally-rebuilt
// precon) sees its passBoost reduced. Composed with the archetype-
// based +0.30 boost the integration site applies, the combined
// passBoost is +0.15 — markedly less passy than a normally-
// configured Control deck (+0.30) but still positive, reflecting
// that the deck WANTS to hold mana even though it lacks the cards
// to do so productively. Tests the helper composes additively with
// the rest of the passBoost stack, not the integration site directly
// (game-state-driven path is exercised in the broader hat suite).
func TestCheapInteractionPassAdjust_OverridesArchetypeStance(t *testing.T) {
	// Simulate the integration site composition: archetype-based
	// baseline + CheapInteraction adjustment.
	controlBaseBoost := 0.30 // matches ArchetypeControl case in yggdrasil.go
	cases := []struct {
		name        string
		cheapInt    int
		wantBoost   float64
		description string
	}{
		{
			name:        "Control deck with normal interaction (no adjust)",
			cheapInt:    3,
			wantBoost:   0.30,
			description: "1-3 cheap interaction is neutral — archetype stance stands",
		},
		{
			name:        "Control deck with stocked interaction (lifts further)",
			cheapInt:    6,
			wantBoost:   0.40,
			description: "4+ cheap interaction reinforces the hold-mana stance (+0.10)",
		},
		{
			name:        "Control deck with no interaction (gets damped)",
			cheapInt:    0,
			wantBoost:   0.15,
			description: "0 cheap interaction veto: nothing to hold mana for, archetype stance halved (-0.15)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sp := &StrategyProfile{
				Archetype:        "control",
				CheapInteraction: tc.cheapInt,
			}
			composed := controlBaseBoost + cheapInteractionPassAdjust(sp)
			if composed != tc.wantBoost {
				t.Errorf("composed passBoost for %s: want %v, got %v (%s)",
					tc.name, tc.wantBoost, composed, tc.description)
			}
		})
	}
}

// TestCheapInteractionPassAdjust_LiftsMidrangeAggro pins the second
// half of the motivating case: a Midrange or Aggro deck (archetypes
// NOT in the priority-round archetype switch, so baseline passBoost
// of 0) with a stocked cheap-interaction package gets a small hold-
// mana boost. Without the adjustment, these decks would never lean
// toward passing even when they have 4+ cheap counterspells / removal
// to deploy on opp turns.
func TestCheapInteractionPassAdjust_LiftsMidrangeAggro(t *testing.T) {
	midrangeBaseBoost := 0.0 // Midrange not in the archetype switch
	cases := []struct {
		name      string
		archetype string
		cheapInt  int
		want      float64
	}{
		{
			name:      "Midrange with 0 cheap interaction stays at baseline minus damping",
			archetype: "midrange",
			cheapInt:  0,
			want:      -0.15,
		},
		{
			name:      "Midrange with 4+ cheap interaction gets a hold-mana lift",
			archetype: "midrange",
			cheapInt:  5,
			want:      0.10,
		},
		{
			name:      "Aggro with 4+ cheap interaction lifts too (no archetype gate)",
			archetype: "aggro",
			cheapInt:  4,
			want:      0.10,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sp := &StrategyProfile{
				Archetype:        tc.archetype,
				CheapInteraction: tc.cheapInt,
			}
			composed := midrangeBaseBoost + cheapInteractionPassAdjust(sp)
			if composed != tc.want {
				t.Errorf("composed passBoost: want %v, got %v", tc.want, composed)
			}
		})
	}
}
