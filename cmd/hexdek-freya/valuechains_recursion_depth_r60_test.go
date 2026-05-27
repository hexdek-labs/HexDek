package main

import "testing"

// MaxValueChainRecursionDepth must rank infinite > deep > shallow > none
// so the strongest chain in the deck drives the GraveyardValue boost,
// even when shallower chains coexist.
func TestMaxValueChainRecursionDepth_RanksStrongestSignal(t *testing.T) {
	cases := []struct {
		name   string
		chains []ValueChain
		want   string
	}{
		{"empty", nil, "none"},
		{"single none", []ValueChain{{RecursionDepth: "none"}}, "none"},
		{"single shallow", []ValueChain{{RecursionDepth: "shallow"}}, "shallow"},
		{"single deep", []ValueChain{{RecursionDepth: "deep"}}, "deep"},
		{"single infinite", []ValueChain{{RecursionDepth: "infinite"}}, "infinite"},
		{"mixed deep+shallow picks deep", []ValueChain{
			{RecursionDepth: "shallow"}, {RecursionDepth: "deep"},
		}, "deep"},
		{"mixed infinite+deep picks infinite", []ValueChain{
			{RecursionDepth: "deep"}, {RecursionDepth: "infinite"}, {RecursionDepth: "shallow"},
		}, "infinite"},
		{"unknown tag treated as none", []ValueChain{{RecursionDepth: "weird"}}, "none"},
	}
	for _, tc := range cases {
		got := MaxValueChainRecursionDepth(tc.chains)
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// ComputeEvalWeights stacks the recursion-depth boost on top of the raw
// IsRecursion count signal. A deck with both a literal loop chain AND 5
// recursion bodies should get the full +0.4 (count) + +0.5 (infinite)
// graveyard boost on top of its archetype baseline.
func TestComputeEvalWeights_RecursionDepthStacksWithCountSignal(t *testing.T) {
	baseProfile := func(arch string) *DeckProfile {
		return &DeckProfile{
			PrimaryArchetype: arch,
			RampCount:        8,
			WinLineCount:     1,
		}
	}
	baseReport := func(recursionBodies int, chainDepth string) *FreyaReport {
		r := &FreyaReport{
			NonLandTutorCount: 2,
			RemovalCount:      8,
			Roles:             &RoleAnalysis{RoleCounts: map[RoleTag]int{}},
		}
		for i := 0; i < recursionBodies; i++ {
			r.Profiles = append(r.Profiles, CardProfile{IsRecursion: true})
		}
		if chainDepth != "" {
			r.ValueChains = []ValueChain{{Name: "test", RecursionDepth: chainDepth}}
		}
		return r
	}

	dp := baseProfile("midrange")

	// Baseline: midrange archetype, 0 recursion bodies, 0 chains.
	wBase := ComputeEvalWeights(dp, baseReport(0, ""))
	// 5 bodies, no chain → +0.4.
	wCount := ComputeEvalWeights(dp, baseReport(5, ""))
	if got := wCount.GraveyardValue - wBase.GraveyardValue; got < 0.39 || got > 0.41 {
		t.Errorf("recursion-count boost (5 bodies, no chain): got +%.3f, want +0.4", got)
	}
	// 0 bodies, infinite chain → +0.5.
	wInf := ComputeEvalWeights(dp, baseReport(0, "infinite"))
	if got := wInf.GraveyardValue - wBase.GraveyardValue; got < 0.49 || got > 0.51 {
		t.Errorf("recursion-depth boost (0 bodies, infinite chain): got +%.3f, want +0.5", got)
	}
	// 5 bodies + infinite chain → +0.9.
	wBoth := ComputeEvalWeights(dp, baseReport(5, "infinite"))
	if got := wBoth.GraveyardValue - wBase.GraveyardValue; got < 0.89 || got > 0.91 {
		t.Errorf("count + depth stack (5 bodies, infinite chain): got +%.3f, want +0.9", got)
	}
	// Deep chain alone: +0.25.
	wDeep := ComputeEvalWeights(dp, baseReport(0, "deep"))
	if got := wDeep.GraveyardValue - wBase.GraveyardValue; got < 0.24 || got > 0.26 {
		t.Errorf("recursion-depth boost (deep chain alone): got +%.3f, want +0.25", got)
	}
	// Shallow chain alone: +0.1.
	wShallow := ComputeEvalWeights(dp, baseReport(0, "shallow"))
	if got := wShallow.GraveyardValue - wBase.GraveyardValue; got < 0.09 || got > 0.11 {
		t.Errorf("recursion-depth boost (shallow chain alone): got +%.3f, want +0.1", got)
	}
	// "none" tag must NOT boost — defends against accidental float
	// drift in switch-case ordering.
	wNone := ComputeEvalWeights(dp, baseReport(0, "none"))
	if got := wNone.GraveyardValue - wBase.GraveyardValue; got > 0.001 {
		t.Errorf("recursion-depth 'none' must not boost: got +%.3f", got)
	}
}
