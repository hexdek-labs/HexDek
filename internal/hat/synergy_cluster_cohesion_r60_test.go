package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// TestSynergyClusterCohesionBoost_NilSafety pins zero-return on every
// degenerate input shape. Defends against panics on legacy / no-strategy
// hat constructors.
func TestSynergyClusterCohesionBoost_NilSafety(t *testing.T) {
	gs := newTestGame(t, 2)
	card := newTestCardMinimal("Anointed Procession", []string{"enchantment"}, 4, nil)

	cases := []struct {
		name string
		h    *YggdrasilHat
		gs   *gameengine.GameState
		c    *gameengine.Card
	}{
		{"nil hat", nil, gs, card},
		{"nil strategy", &YggdrasilHat{}, gs, card},
		{"empty clusters", &YggdrasilHat{Strategy: &StrategyProfile{}}, gs, card},
		{
			"nil gs",
			&YggdrasilHat{Strategy: &StrategyProfile{
				SynergyClusters: []SynergyCluster{{Members: []string{"X"}}},
			}},
			nil,
			card,
		},
		{
			"nil card",
			&YggdrasilHat{Strategy: &StrategyProfile{
				SynergyClusters: []SynergyCluster{{Members: []string{"X"}}},
			}},
			gs,
			nil,
		},
		{
			"seatIdx out of range",
			&YggdrasilHat{Strategy: &StrategyProfile{
				SynergyClusters: []SynergyCluster{{Members: []string{"X"}}},
			}},
			gs,
			card,
		},
	}
	// All but the last use seatIdx 0 — last uses 99 to hit the out-of-range guard.
	for _, tc := range cases[:len(cases)-1] {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.h.synergyClusterCohesionBoost(tc.gs, 0, tc.c); got != 0 {
				t.Errorf("%s: want 0, got %v", tc.name, got)
			}
		})
	}
	last := cases[len(cases)-1]
	t.Run(last.name, func(t *testing.T) {
		if got := last.h.synergyClusterCohesionBoost(last.gs, 99, last.c); got != 0 {
			t.Errorf("out-of-range seatIdx: want 0, got %v", got)
		}
	})
}

// TestSynergyClusterCohesionBoost_RequiresTwoActiveOthers pins the
// activation gate: a candidate that's a cluster member with ZERO or ONE
// other members on the battlefield gets NO boost. The cluster must have
// ≥2 other members already in play to qualify as "active". The
// "OTHERS" guard means the candidate itself doesn't count toward
// activation (a creature about to be cast can't be its own cohesion
// partner).
func TestSynergyClusterCohesionBoost_RequiresTwoActiveOthers(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	candidate := newTestCardMinimal("Mascot Token", []string{"creature"}, 1, nil)

	h := &YggdrasilHat{
		Strategy: &StrategyProfile{
			SynergyClusters: []SynergyCluster{{
				Name:    "Tokens",
				Members: []string{"Mascot Token", "Anointed Procession", "Doubling Season", "Cathars' Crusade"},
			}},
		},
	}

	// 0 other cluster members on battlefield -> no boost
	if got := h.synergyClusterCohesionBoost(gs, 0, candidate); got != 0 {
		t.Errorf("zero-others: want 0, got %v", got)
	}

	// 1 other cluster member on battlefield -> still no boost (need ≥2)
	newTestPermanent(seat, newTestCardMinimal("Anointed Procession", []string{"enchantment"}, 4, nil), 0, 0)
	if got := h.synergyClusterCohesionBoost(gs, 0, candidate); got != 0 {
		t.Errorf("one-other: want 0, got %v", got)
	}

	// 2 other cluster members on battlefield -> boost activates
	newTestPermanent(seat, newTestCardMinimal("Doubling Season", []string{"enchantment"}, 5, nil), 0, 0)
	if got := h.synergyClusterCohesionBoost(gs, 0, candidate); got != 0.05 {
		t.Errorf("two-others: want +0.05, got %v", got)
	}
}

// TestSynergyClusterCohesionBoost_HighDensityClusterBoostsMore verifies
// that Freya's HighDensity flag (cluster is a "real subsystem of the
// gameplan" per the MemberCount ≥ 5 threshold) maps to a 2x boost
// magnitude — +0.10 vs +0.05 for regular clusters. The hat should weight
// load-bearing engines more heavily than incidental thematic overlap.
func TestSynergyClusterCohesionBoost_HighDensityClusterBoostsMore(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	candidate := newTestCardMinimal("Hardened Scales", []string{"enchantment"}, 1, nil)

	newTestPermanent(seat, newTestCardMinimal("Cathars' Crusade", []string{"enchantment"}, 5, nil), 0, 0)
	newTestPermanent(seat, newTestCardMinimal("Walking Ballista", []string{"creature"}, 0, nil), 0, 0)

	regular := &YggdrasilHat{
		Strategy: &StrategyProfile{
			SynergyClusters: []SynergyCluster{{
				Name:        "+1/+1 Counters",
				Members:     []string{"Hardened Scales", "Cathars' Crusade", "Walking Ballista"},
				HighDensity: false,
			}},
		},
	}
	highDensity := &YggdrasilHat{
		Strategy: &StrategyProfile{
			SynergyClusters: []SynergyCluster{{
				Name:        "+1/+1 Counters",
				Members:     []string{"Hardened Scales", "Cathars' Crusade", "Walking Ballista"},
				HighDensity: true,
			}},
		},
	}

	gotReg := regular.synergyClusterCohesionBoost(gs, 0, candidate)
	gotHigh := highDensity.synergyClusterCohesionBoost(gs, 0, candidate)

	if gotReg != 0.05 {
		t.Errorf("regular cluster: want +0.05, got %v", gotReg)
	}
	if gotHigh != 0.10 {
		t.Errorf("high-density cluster: want +0.10, got %v", gotHigh)
	}
	if gotHigh <= gotReg {
		t.Errorf("HighDensity should produce LARGER boost: regular=%v, high=%v",
			gotReg, gotHigh)
	}
}

// TestSynergyClusterCohesionBoost_CapsAt025 pins the cumulative cap.
// Multiple active clusters all containing the candidate sum, but never
// exceed +0.25 — keeps the synergy bonus from dwarfing the star-card
// (+0.15) and cuttable-card (-0.10) adjustments that share the
// cardHeuristic budget.
func TestSynergyClusterCohesionBoost_CapsAt025(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	candidate := newTestCardMinimal("Bloodghast", []string{"creature"}, 2, nil)

	// Stage 6 battlefield permanents so 3 clusters can each have ≥2
	// active others.
	for _, name := range []string{"Bloodartist", "Zulaport Cutthroat", "Cruel Celebrant",
		"Sun Titan", "Reveillark", "Karmic Guide"} {
		newTestPermanent(seat, newTestCardMinimal(name, []string{"creature"}, 2, nil), 0, 0)
	}

	h := &YggdrasilHat{
		Strategy: &StrategyProfile{
			SynergyClusters: []SynergyCluster{
				{
					Name:        "Aristocrats",
					Members:     []string{"Bloodghast", "Bloodartist", "Zulaport Cutthroat", "Cruel Celebrant"},
					HighDensity: true, // +0.10
				},
				{
					Name:        "Recursion",
					Members:     []string{"Bloodghast", "Sun Titan", "Reveillark", "Karmic Guide"},
					HighDensity: true, // +0.10
				},
				{
					Name:        "ETB Value",
					Members:     []string{"Bloodghast", "Reveillark", "Karmic Guide", "Sun Titan"},
					HighDensity: true, // +0.10
				},
			},
		},
	}
	// Three HighDensity clusters at +0.10 each = +0.30 uncapped.
	// Should clamp to +0.25.
	got := h.synergyClusterCohesionBoost(gs, 0, candidate)
	if got != 0.25 {
		t.Errorf("three-cluster stack: want cap at +0.25, got %v", got)
	}
}

// TestSynergyClusterCohesionBoost_NonMemberCardNoBoost pins that
// non-member cards get no boost even when clusters are active. The
// helper is membership-gated — only cards on the cluster member list
// benefit from cohesion.
func TestSynergyClusterCohesionBoost_NonMemberCardNoBoost(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	newTestPermanent(seat, newTestCardMinimal("Anointed Procession", []string{"enchantment"}, 4, nil), 0, 0)
	newTestPermanent(seat, newTestCardMinimal("Doubling Season", []string{"enchantment"}, 5, nil), 0, 0)

	// Candidate is NOT in the cluster.
	candidate := newTestCardMinimal("Counterspell", []string{"instant"}, 2, nil)
	h := &YggdrasilHat{
		Strategy: &StrategyProfile{
			SynergyClusters: []SynergyCluster{{
				Name:    "Tokens",
				Members: []string{"Mascot Token", "Anointed Procession", "Doubling Season"},
			}},
		},
	}
	if got := h.synergyClusterCohesionBoost(gs, 0, candidate); got != 0 {
		t.Errorf("non-member candidate: want 0, got %v", got)
	}
}

// TestSynergyClusterCohesionBoost_CaseInsensitive verifies that
// membership/battlefield matching is case-insensitive. Card names from
// Freya might be Title Case while engine names could vary; the
// strings.ToLower normalization should make both work.
func TestSynergyClusterCohesionBoost_CaseInsensitive(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	newTestPermanent(seat, newTestCardMinimal("anointed procession", []string{"enchantment"}, 4, nil), 0, 0)
	newTestPermanent(seat, newTestCardMinimal("DOUBLING SEASON", []string{"enchantment"}, 5, nil), 0, 0)
	candidate := newTestCardMinimal("Mascot Token", []string{"creature"}, 1, nil)
	h := &YggdrasilHat{
		Strategy: &StrategyProfile{
			SynergyClusters: []SynergyCluster{{
				Name:    "Tokens",
				Members: []string{"Mascot Token", "Anointed Procession", "Doubling Season"},
			}},
		},
	}
	if got := h.synergyClusterCohesionBoost(gs, 0, candidate); got != 0.05 {
		t.Errorf("case-insensitive match: want +0.05, got %v", got)
	}
}

// TestSynergyClusterCohesionBoost_CandidateDoesNotSelfActivate is the
// load-bearing semantic: a candidate that's ALSO on the battlefield
// (rare — copy effects / token returns) should not count itself as one
// of the "≥2 active others" required for activation. The "OTHERS" gate
// in the helper prevents a single cluster member from self-activating.
func TestSynergyClusterCohesionBoost_CandidateDoesNotSelfActivate(t *testing.T) {
	gs := newTestGame(t, 2)
	seat := gs.Seats[0]
	candidate := newTestCardMinimal("Mascot Token", []string{"creature"}, 1, nil)
	// Candidate is also already on battlefield (copy / token return scenario).
	newTestPermanent(seat, newTestCardMinimal("Mascot Token", []string{"creature"}, 1, nil), 0, 0)
	// One OTHER cluster member also on battlefield.
	newTestPermanent(seat, newTestCardMinimal("Anointed Procession", []string{"enchantment"}, 4, nil), 0, 0)

	h := &YggdrasilHat{
		Strategy: &StrategyProfile{
			SynergyClusters: []SynergyCluster{{
				Name:    "Tokens",
				Members: []string{"Mascot Token", "Anointed Procession", "Doubling Season"},
			}},
		},
	}
	// candidate + 1 other = 1 active other (candidate's own copy
	// excluded by the "OTHERS" guard). Below the ≥2 threshold → no boost.
	if got := h.synergyClusterCohesionBoost(gs, 0, candidate); got != 0 {
		t.Errorf("candidate-self-on-bf with 1 other: want 0 (candidate excluded), got %v", got)
	}
}

// TestBuildFromStrategyJSON_SynergyClustersPlumbed pins JSON wire
// round-trip — strategyFileJSON's synergy_clusters array populates
// StrategyProfile.SynergyClusters with members + HighDensity preserved.
// Defends against the load-side dropping the new field silently.
func TestBuildFromStrategyJSON_SynergyClustersPlumbed(t *testing.T) {
	sj := &strategyFileJSON{
		Archetype: "tokens",
		SynergyClusters: []freyaSynergyCluster{
			{
				Name:        "Tokens",
				Theme:       "tokens",
				Members:     []string{"Anointed Procession", "Doubling Season", "Mascot Token"},
				HighDensity: true,
			},
			{
				Name:    "Lifegain",
				Theme:   "lifegain",
				Members: []string{"Bloodartist", "Soul Warden"},
				// HighDensity unset (false)
			},
			{
				// Empty members — should be skipped at load time.
				Name:    "Empty",
				Members: nil,
			},
		},
	}
	sp := buildFromStrategyJSON(sj)
	if sp == nil {
		t.Fatal("expected StrategyProfile")
	}
	if len(sp.SynergyClusters) != 2 {
		t.Fatalf("want 2 clusters (empty skipped), got %d", len(sp.SynergyClusters))
	}
	if sp.SynergyClusters[0].Name != "Tokens" {
		t.Errorf("cluster[0].Name: want Tokens, got %q", sp.SynergyClusters[0].Name)
	}
	if !sp.SynergyClusters[0].HighDensity {
		t.Error("cluster[0] should be HighDensity")
	}
	if len(sp.SynergyClusters[0].Members) != 3 {
		t.Errorf("cluster[0] members: want 3, got %d", len(sp.SynergyClusters[0].Members))
	}
	if sp.SynergyClusters[1].HighDensity {
		t.Error("cluster[1] should NOT be HighDensity")
	}
}
