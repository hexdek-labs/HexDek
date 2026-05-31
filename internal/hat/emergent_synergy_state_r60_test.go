package hat

import (
	"testing"
)

// TestEmergentSynergyBump pins the per-state EmergentSynergies bump
// added in the wave-2 freya-hat integration audit (2026-05-30).
// Companion to applyEmergentSynergyBoost in strategy_loader.go which
// applies a load-time global Weights.ComboProximity multiplier to
// every state regardless of synergy-piece visibility. The per-state
// bump only fires when ALL pieces of a synergy are visible in the
// `available` map (hand / battlefield / recursable-graveyard).
func TestEmergentSynergyBump(t *testing.T) {
	cases := []struct {
		name      string
		sp        *StrategyProfile
		available map[string]float64
		want      float64
	}{
		{
			name:      "nil sp -> 0",
			sp:        nil,
			available: map[string]float64{"A": 1.0},
			want:      0,
		},
		{
			name:      "nil available -> 0",
			sp:        &StrategyProfile{EmergentSynergies: []EmergentSynergy{{Cards: []string{"A"}, Tier: 2}}},
			available: nil,
			want:      0,
		},
		{
			name:      "empty synergies -> 0",
			sp:        &StrategyProfile{EmergentSynergies: nil},
			available: map[string]float64{"A": 1.0},
			want:      0,
		},
		{
			name: "single tier-2 with all pieces visible -> +0.04",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: []string{"A", "B"}, Tier: 2},
				},
			},
			available: map[string]float64{"A": 1.0, "B": 1.0},
			want:      0.04,
		},
		{
			name: "single tier-3 with all pieces visible -> +0.08",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: []string{"A", "B"}, Tier: 3},
				},
			},
			available: map[string]float64{"A": 1.0, "B": 1.0},
			want:      0.08,
		},
		{
			name: "tier-2 with one piece missing -> 0 (not partial-credit)",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: []string{"A", "B"}, Tier: 2},
				},
			},
			available: map[string]float64{"A": 1.0}, // B missing
			want:      0,
		},
		{
			name: "tier-2 piece in graveyard (weight 0.9) counts as visible",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: []string{"A", "B"}, Tier: 2},
				},
			},
			available: map[string]float64{"A": 1.0, "B": 0.9},
			want:      0.04,
		},
		{
			name: "tier-1 or tier-0 synergy -> 0 (only tier-2/3 count)",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: []string{"A", "B"}, Tier: 1},
					{Cards: []string{"C", "D"}, Tier: 0},
				},
			},
			available: map[string]float64{"A": 1.0, "B": 1.0, "C": 1.0, "D": 1.0},
			want:      0,
		},
		{
			name: "three visible tier-3 synergies sum to +0.24, capped at +0.20",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: []string{"A", "B"}, Tier: 3},
					{Cards: []string{"C", "D"}, Tier: 3},
					{Cards: []string{"E", "F"}, Tier: 3},
				},
			},
			available: map[string]float64{
				"A": 1.0, "B": 1.0, "C": 1.0, "D": 1.0, "E": 1.0, "F": 1.0,
			},
			want: 0.20, // capped from 0.24
		},
		{
			name: "mixed tier-2 + tier-3 sums correctly",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: []string{"A", "B"}, Tier: 2},
					{Cards: []string{"C", "D"}, Tier: 3},
				},
			},
			available: map[string]float64{"A": 1.0, "B": 1.0, "C": 1.0, "D": 1.0},
			want:      0.12, // 0.04 + 0.08
		},
		{
			name: "empty Cards on synergy -> skipped (defensive)",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: nil, Tier: 3},
					{Cards: []string{"A", "B"}, Tier: 2},
				},
			},
			available: map[string]float64{"A": 1.0, "B": 1.0},
			want:      0.04,
		},
		{
			name: "synergy with zero-weight piece -> not visible",
			sp: &StrategyProfile{
				EmergentSynergies: []EmergentSynergy{
					{Cards: []string{"A", "B"}, Tier: 2},
				},
			},
			available: map[string]float64{"A": 1.0, "B": 0.0},
			want:      0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := emergentSynergyBump(tc.sp, tc.available)
			if got != tc.want {
				t.Errorf("emergentSynergyBump: want %v, got %v", tc.want, got)
			}
		})
	}
}

// TestEmergentSynergyBump_VisibleStateScoresHigher pins the
// motivating-case invariant: given the same StrategyProfile, a state
// where the synergy pieces are VISIBLE must score higher than a state
// where they're not. Confirms the per-state lift actually
// differentiates between equivalent positions distinguished only by
// synergy-piece visibility — without this, the load-time global bump
// would apply equally to both states, masking the visibility signal.
func TestEmergentSynergyBump_VisibleStateScoresHigher(t *testing.T) {
	sp := &StrategyProfile{
		EmergentSynergies: []EmergentSynergy{
			{Cards: []string{"Cultivate", "Burgeoning"}, Tier: 3, AvgImpact: 0.6},
		},
	}
	visibleState := map[string]float64{
		"Cultivate":  1.0,
		"Burgeoning": 1.0,
	}
	hiddenState := map[string]float64{
		"Cultivate": 1.0,
		// Burgeoning missing — synergy not assemblable from this state.
	}
	noPiecesState := map[string]float64{}

	visibleBump := emergentSynergyBump(sp, visibleState)
	hiddenBump := emergentSynergyBump(sp, hiddenState)
	noBump := emergentSynergyBump(sp, noPiecesState)

	if visibleBump <= hiddenBump {
		t.Errorf("visible state should score higher than hidden: visible=%v, hidden=%v",
			visibleBump, hiddenBump)
	}
	if visibleBump <= noBump {
		t.Errorf("visible state should score higher than empty: visible=%v, empty=%v",
			visibleBump, noBump)
	}
	if hiddenBump != noBump {
		t.Errorf("hidden (partial-piece) and empty state should both be 0: hidden=%v, empty=%v",
			hiddenBump, noBump)
	}
}

// TestEmergentSynergyBump_RecursableGraveyardCounted pins that the
// per-state bump accepts graveyard pieces when the recursable-
// graveyard weight (0.9) is in play. This mirrors the scoreCombo
// upstream behavior — graveyard pieces are real reach when a
// recursion engine is on the battlefield (Underworld Breach,
// Yawgmoth's Will, Muldrotha, etc.), and the synergy bump should
// credit them the same way.
func TestEmergentSynergyBump_RecursableGraveyardCounted(t *testing.T) {
	sp := &StrategyProfile{
		EmergentSynergies: []EmergentSynergy{
			{Cards: []string{"Lightning Bolt", "Snapcaster Mage"}, Tier: 3},
		},
	}
	// All pieces with the recursable-graveyard weight (0.9):
	gyState := map[string]float64{
		"Lightning Bolt":  0.9,
		"Snapcaster Mage": 0.9,
	}
	// One piece at full weight, one in non-recursable graveyard (0.5):
	mixedState := map[string]float64{
		"Lightning Bolt":  1.0,
		"Snapcaster Mage": 0.5, // also positive — still counts
	}

	if got := emergentSynergyBump(sp, gyState); got != 0.08 {
		t.Errorf("recursable-graveyard state: want +0.08, got %v", got)
	}
	if got := emergentSynergyBump(sp, mixedState); got != 0.08 {
		t.Errorf("mixed-weight state: want +0.08, got %v", got)
	}
}
