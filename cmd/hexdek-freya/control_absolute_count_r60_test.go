package main

import "testing"

// control_absolute_count_r60_test.go pins the absolute-count Control
// detection arm added to handle creature-light Talrand / Baral / Kess /
// Sen Triplets-style builds where a deep draw + ramp package dilutes
// each individual role ratio below 15% even though the deck packs
// 15+ interaction cards.
//
// Two-arm shape:
//
//	(1) Proportional: (Removal + BoardWipe + Counterspell) ≥ 0.15 AND Draw ≥ 0.10
//	(2) Absolute: interactionCount ≥ 15 AND creatureCount < 15
//
// Tests pin BOTH arms accept the right shapes, the conjunction within
// each arm holds, and creature-heavy Bant/Esper midrange isn't poached.

func findControlFingerprint(t *testing.T) archetypeFingerprint {
	t.Helper()
	for _, fp := range archetypeFingerprints {
		if fp.Name == "Control" {
			return fp
		}
	}
	t.Fatalf("Control fingerprint missing from archetypeFingerprints")
	return archetypeFingerprint{}
}

func TestControlRequire_ProportionalArmStillAccepted(t *testing.T) {
	fp := findControlFingerprint(t)
	// Canonical UW Control proportional shape: 16% interaction, 12% draw.
	ctx := &classifyContext{
		roleRatios: map[RoleTag]float64{
			RoleRemoval:      0.10,
			RoleBoardWipe:    0.03,
			RoleCounterspell: 0.04,
			RoleDraw:         0.12,
		},
	}
	if !fp.Require(ctx) {
		t.Fatalf("proportional arm interaction=0.17 draw=0.12 must still classify Control (legacy gate)")
	}
}

func TestControlRequire_AbsoluteCountArmAccepted(t *testing.T) {
	fp := findControlFingerprint(t)
	// Talrand / Baral / Kess shell: 18 interaction (Counterspell +
	// Swan Song + Negate + Path + Swords + Cyclonic Rift + Toxic Deluge +
	// Wrath of God + Damnation + Supreme Verdict + Force of Will +
	// Force of Negation + Mana Drain + Mana Leak + Dovin's Veto +
	// Render Silent + Disallow + Pongify), 8 creatures (commander + a
	// handful of value plays + Snapcaster Mage). Proportional gates fail
	// because the deep draw + ramp dilutes each role ratio below 15%.
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{}, // ratios all 0 — only absolute arm should fire
		interactionCount: 18,
		creatureCount:    8,
	}
	if !fp.Require(ctx) {
		t.Fatalf("absolute arm interaction=18 creatures=8 must classify Control")
	}
}

func TestControlRequire_AbsoluteCountExactThreshold(t *testing.T) {
	fp := findControlFingerprint(t)
	// At-threshold: 15 interaction, 14 creatures — both at the
	// boundary, must accept.
	ctx := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		interactionCount: 15,
		creatureCount:    14,
	}
	if !fp.Require(ctx) {
		t.Fatalf("at-threshold interaction=15 creatures=14 must classify Control")
	}

	// One below the interaction floor: must fail.
	low1 := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		interactionCount: 14,
		creatureCount:    14,
	}
	if fp.Require(low1) {
		t.Errorf("interaction=14 creatures=14 must NOT classify (below interaction floor)")
	}

	// At creature ceiling: 15 creatures — must fail (the gate is <15,
	// not ≤15; 15-creature decks are creature-leaning, not control).
	low2 := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		interactionCount: 18,
		creatureCount:    15,
	}
	if fp.Require(low2) {
		t.Errorf("interaction=18 creatures=15 must NOT classify (above creature ceiling)")
	}
}

func TestControlRequire_CreatureHeavyMidrangeRejected(t *testing.T) {
	fp := findControlFingerprint(t)
	// Cases that look interaction-adjacent but are creature-heavy
	// goodstuff midrange, not control. Each must be rejected by BOTH
	// arms.
	cases := []classifyContext{
		// Bant goodstuff: 16 interaction cards (lots of bounce + spot
		// removal), but 20 creatures. Proportional dilutes; absolute
		// fails the <15 creature ceiling.
		{
			roleRatios:       map[RoleTag]float64{},
			interactionCount: 16,
			creatureCount:    20,
		},
		// Esper midrange: 12 interaction, 16 creatures. Both arms fail.
		{
			roleRatios:       map[RoleTag]float64{},
			interactionCount: 12,
			creatureCount:    16,
		},
		// Proportional close-but-not-quite: 14% interaction, 11% draw
		// — fails the 15% interaction floor.
		{
			roleRatios: map[RoleTag]float64{
				RoleRemoval:      0.08,
				RoleBoardWipe:    0.03,
				RoleCounterspell: 0.03,
				RoleDraw:         0.11,
			},
		},
		// Proportional interaction OK but draw too thin: 17% interaction,
		// 8% draw.
		{
			roleRatios: map[RoleTag]float64{
				RoleRemoval:      0.10,
				RoleBoardWipe:    0.04,
				RoleCounterspell: 0.03,
				RoleDraw:         0.08,
			},
		},
	}
	for i, ctx := range cases {
		ctx := ctx
		if fp.Require(&ctx) {
			t.Errorf("case %d %+v should NOT classify Control (would poach midrange)", i, ctx)
		}
	}
}

// Either arm independently triggers classification — the test pins
// that the disjunction is a true OR, not an AND. A deck that passes
// the absolute arm with all role ratios zeroed must still classify;
// a deck that passes the proportional arm with creatureCount=20 and
// interactionCount=0 must also still classify.
func TestControlRequire_ArmsAreDisjunctive(t *testing.T) {
	fp := findControlFingerprint(t)
	// Absolute arm only — role ratios all zero.
	absOnly := &classifyContext{
		roleRatios:       map[RoleTag]float64{},
		interactionCount: 20,
		creatureCount:    10,
	}
	if !fp.Require(absOnly) {
		t.Errorf("absolute-arm-only ctx must classify (proves OR, not AND)")
	}

	// Proportional arm only — creatureCount well above the absolute
	// ceiling, interactionCount well below the absolute floor.
	propOnly := &classifyContext{
		roleRatios: map[RoleTag]float64{
			RoleRemoval:      0.12,
			RoleBoardWipe:    0.04,
			RoleCounterspell: 0.04,
			RoleDraw:         0.11,
		},
		interactionCount: 5,  // below absolute floor
		creatureCount:    20, // above absolute ceiling
	}
	if !fp.Require(propOnly) {
		t.Errorf("proportional-arm-only ctx must classify (proves OR, not AND)")
	}
}
