package main

import "testing"

// aristocrats_broaden_r60_test.go — R60-13 pins the broadened
// Aristocrats fingerprint Require predicate. The legacy AND
// (sacrificeCount>=5 AND deathTriggers>=3) missed two real
// shapes that landed on the Midrange fallback:
//   - drain-payoff-heavy (few outlets, many Blood Artist-style
//     death-trigger payoffs)
//   - persist / GY-recursion shells (Reassembling Skeleton +
//     Phyrexian Altar where GY bodies fuel the outlet)
// The disjunction must accept both new shapes AND keep the
// strict shape, AND must NOT poach generic creature midrange.

func findAristocratsFingerprint(t *testing.T) archetypeFingerprint {
	t.Helper()
	for _, fp := range archetypeFingerprints {
		if fp.Name == "Aristocrats" {
			return fp
		}
	}
	t.Fatalf("Aristocrats fingerprint missing from archetypeFingerprints")
	return archetypeFingerprint{}
}

func TestAristocratsRequire_StrictShapeStillAccepted(t *testing.T) {
	fp := findAristocratsFingerprint(t)
	ctx := &classifyContext{sacrificeCount: 5, deathTriggers: 3}
	if !fp.Require(ctx) {
		t.Fatalf("strict shape sac=5 deaths=3 must still classify Aristocrats")
	}
}

func TestAristocratsRequire_DrainPayoffShapeAccepted(t *testing.T) {
	fp := findAristocratsFingerprint(t)
	// Elenda / Teysa / Marchesa shells: light outlets, heavy payoff cluster.
	ctx := &classifyContext{sacrificeCount: 2, deathTriggers: 6}
	if !fp.Require(ctx) {
		t.Fatalf("drain-payoff shape sac=2 deaths=6 must classify Aristocrats")
	}
}

func TestAristocratsRequire_PersistRecursionShapeAccepted(t *testing.T) {
	fp := findAristocratsFingerprint(t)
	// Reassembling Skeleton / Bloodghast / Gravecrawler shells.
	ctx := &classifyContext{sacrificeCount: 3, deathTriggers: 3, graveyardCount: 4}
	if !fp.Require(ctx) {
		t.Fatalf("persist-recursion shape sac=3 deaths=3 gy=4 must classify Aristocrats")
	}
}

func TestAristocratsRequire_GenericMidrangeRejected(t *testing.T) {
	fp := findAristocratsFingerprint(t)
	// A midrange creature deck with one Viscera Seer and a few dies
	// triggers must NOT poach into Aristocrats.
	cases := []classifyContext{
		{sacrificeCount: 1, deathTriggers: 2},                        // too thin everywhere
		{sacrificeCount: 4, deathTriggers: 2},                        // fails strict, fails drain (deaths<6), persist (deaths<3 false here but gy=0)
		{sacrificeCount: 2, deathTriggers: 5},                        // fails drain (deaths<6)
		{sacrificeCount: 3, deathTriggers: 3, graveyardCount: 3},     // persist gated by gy>=4
		{sacrificeCount: 0, deathTriggers: 8},                        // no outlet at all
	}
	for i, ctx := range cases {
		ctx := ctx
		if fp.Require(&ctx) {
			t.Errorf("case %d %+v should NOT classify Aristocrats (would poach midrange)", i, ctx)
		}
	}
}
