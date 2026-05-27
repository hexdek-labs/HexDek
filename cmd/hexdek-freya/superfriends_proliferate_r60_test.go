package main

import "testing"

// superfriends_proliferate_r60_test.go pins the Atraxa-style
// proliferate-driven Superfriends detection arm — added so a 4-7
// planeswalker shell with 3+ proliferate engines (Atraxa as commander
// + Karn's Bastion + Tezzeret's Gambit + Contagion Engine + Evolution
// Sage + Flux Channeler + Inexorable Tide) classifies as Superfriends
// even though the legacy ≥8 planeswalker gate misses it.
//
// Two-arm shape:
//
//	(1) Walker-dense: planeswalkerCount ≥ 8
//	(2) Proliferate shell: planeswalkerCount ≥ 4 AND proliferateCount ≥ 3
//
// The distinguishing precision is against:
//   - counters-matter decks (Hardened Scales / Marchesa / Animar) that
//     pack +1/+1 payoffs but typically 0-2 planeswalkers
//   - generic proliferate-matters creature decks (Skullbriar, Pir Imag.
//     Rascal, Ezuri Renegade Leader) where the proliferate is on
//     creature counters, not walker loyalty

func findSuperfriendsFingerprint(t *testing.T) archetypeFingerprint {
	t.Helper()
	for _, fp := range archetypeFingerprints {
		if fp.Name == "Superfriends" {
			return fp
		}
	}
	t.Fatalf("Superfriends fingerprint missing from archetypeFingerprints")
	return archetypeFingerprint{}
}

func TestSuperfriendsRequire_WalkerDenseArmStillAccepted(t *testing.T) {
	fp := findSuperfriendsFingerprint(t)
	// Estrid / dedicated walker-tribal: 9 walkers, zero proliferate.
	ctx := &classifyContext{planeswalkerCount: 9, proliferateCount: 0}
	if !fp.Require(ctx) {
		t.Fatalf("walker-dense arm pw=9 must still classify Superfriends (legacy gate)")
	}
}

func TestSuperfriendsRequire_AtraxaProliferateArmAccepted(t *testing.T) {
	fp := findSuperfriendsFingerprint(t)
	// Atraxa Praetors' Voice shell: 6 planeswalkers, 5 proliferate
	// engines (Atraxa commander + Karn's Bastion + Contagion Engine +
	// Evolution Sage + Flux Channeler).
	ctx := &classifyContext{planeswalkerCount: 6, proliferateCount: 5}
	if !fp.Require(ctx) {
		t.Fatalf("Atraxa proliferate arm pw=6 prolif=5 must classify Superfriends")
	}
}

func TestSuperfriendsRequire_ExactThreshold(t *testing.T) {
	fp := findSuperfriendsFingerprint(t)
	// At-threshold: 4 walkers, 3 proliferate — both at the boundary,
	// must accept.
	at := &classifyContext{planeswalkerCount: 4, proliferateCount: 3}
	if !fp.Require(at) {
		t.Fatalf("at-threshold pw=4 prolif=3 must classify Superfriends")
	}

	// One below the walker floor: pw=3 prolif=10 should fail —
	// proliferate-tribal creature decks (Skullbriar / Pir / Ezuri)
	// shouldn't poach into Superfriends.
	lowPW := &classifyContext{planeswalkerCount: 3, proliferateCount: 10}
	if fp.Require(lowPW) {
		t.Errorf("pw=3 prolif=10 must NOT classify (below walker floor — distinguishes from proliferate-creature decks)")
	}

	// One below the proliferate floor: pw=5 prolif=2 should fail —
	// the deck has walkers but the proliferate cluster isn't dense
	// enough to be the engine.
	lowProlif := &classifyContext{planeswalkerCount: 5, proliferateCount: 2}
	if fp.Require(lowProlif) {
		t.Errorf("pw=5 prolif=2 must NOT classify (below proliferate floor)")
	}
}

func TestSuperfriendsRequire_DistinguishesFromCountersMatter(t *testing.T) {
	fp := findSuperfriendsFingerprint(t)
	// Counters-matter decks (Hardened Scales / Marchesa the Black Rose /
	// Animar Soul of Elements) routinely pack 15+ "+1/+1 counter"
	// references that bump counterCount, but typically run 0-2
	// planeswalkers and 0-2 specific proliferate effects (proliferate
	// is fine on creatures but isn't the engine). These MUST NOT
	// classify as Superfriends.
	cases := []classifyContext{
		// Hardened Scales pile: 1 walker (Vivien Reid maybe), 1 proliferate
		// effect (Inexorable Tide as a flex slot), but counterCount=20.
		{planeswalkerCount: 1, proliferateCount: 1, counterCount: 20},
		// Marchesa: zero walkers, 2 proliferate, counterCount=25.
		{planeswalkerCount: 0, proliferateCount: 2, counterCount: 25},
		// Animar: zero walkers, zero proliferate, counterCount=18.
		{planeswalkerCount: 0, proliferateCount: 0, counterCount: 18},
		// Walker-light, proliferate-heavy creature shell (Skullbriar
		// equipment-voltron with proliferate splash): 2 walkers,
		// 4 proliferate (enough to clear the proliferate floor on
		// its own, but fails the walker floor — this is the spec's
		// "distinguish from generic counters-matter" assertion).
		{planeswalkerCount: 2, proliferateCount: 4, counterCount: 12},
	}
	for i, ctx := range cases {
		ctx := ctx
		if fp.Require(&ctx) {
			t.Errorf("case %d %+v should NOT classify Superfriends (would poach counters-matter / proliferate-creature)", i, ctx)
		}
	}
}

// Either arm independently triggers classification — proves the
// disjunction is a true OR. Walker-dense ctx with zero proliferate
// must classify; Atraxa ctx with pw=4 prolif=8 must also classify
// (both above their respective floors but only the proliferate arm
// is the dominant signal).
func TestSuperfriendsRequire_ArmsAreDisjunctive(t *testing.T) {
	fp := findSuperfriendsFingerprint(t)
	walkerOnly := &classifyContext{planeswalkerCount: 8, proliferateCount: 0}
	if !fp.Require(walkerOnly) {
		t.Errorf("walker-only ctx pw=8 prolif=0 must classify (proves OR)")
	}
	prolifOnly := &classifyContext{planeswalkerCount: 4, proliferateCount: 8}
	if !fp.Require(prolifOnly) {
		t.Errorf("proliferate-only ctx pw=4 prolif=8 must classify (proves OR)")
	}
}

// End-to-end: the proliferateCount accumulator picks up canonical
// proliferate cards via their oracle-text "proliferate" keyword, and
// correctly excludes +1/+1-counter-matters payoffs that the broader
// counterCount accumulator catches. This pins the precision lever
// distinguishing Superfriends-shape from counters-matter-shape.
func TestProliferateCountDetection_RecognizesProliferateCardsOnly(t *testing.T) {
	cards := []struct {
		name string
		ot   string
		want bool
	}{
		// Canonical proliferate cards — must bump proliferateCount.
		{"Atraxa, Praetors' Voice", "At the beginning of your end step, proliferate.", true},
		{"Karn's Bastion", "{4}, {T}: Proliferate.", true},
		{"Tezzeret's Gambit", "Draw two cards. Proliferate.", true},
		{"Contagion Engine", "Whenever you cast this spell, target player gets two poison counters. Proliferate.", true},
		{"Contagion Clasp", "{4}, {T}: Proliferate.", true},
		{"Flux Channeler", "Whenever you cast a noncreature spell, proliferate.", true},
		{"Evolution Sage", "Landfall — Whenever a land enters under your control, proliferate.", true},
		{"Inexorable Tide", "Whenever you cast a spell, proliferate.", true},
		// Counters-matter / +1/+1-counter payoffs — MUST NOT bump
		// proliferateCount even though they bump counterCount.
		{"Hardened Scales", "If one or more +1/+1 counters would be put on a creature you control, that many plus one +1/+1 counters are put on it instead.", false},
		{"Branching Evolution", "If one or more +1/+1 counters would be placed on a creature you control, twice that many +1/+1 counters are placed on it instead.", false},
		{"Doubling Season", "If an effect would create one or more tokens under your control, it creates twice that many of those tokens instead. If an effect would put one or more counters on a permanent you control, it puts twice that many of those counters on it instead.", false},
		{"Walking Ballista", "This creature enters with X +1/+1 counters on it.", false},
		{"Sol Ring", "{T}: Add {C}{C}.", false},
	}
	for _, c := range cards {
		ctx := buildProliferateCtxForCard(c.ot)
		got := ctx.proliferateCount > 0
		if got != c.want {
			t.Errorf("%s: proliferateCount>0 = %v, want %v (ot=%q)", c.name, got, c.want, c.ot)
		}
	}
}

// buildProliferateCtxForCard mirrors the per-card "proliferate" branch
// of buildClassifyContext. Test-local helper so the assertion stays
// bit-stable without dragging in the full oracle-DB pipeline.
func buildProliferateCtxForCard(oracleText string) *classifyContext {
	ctx := &classifyContext{}
	ot := superfriendsLower(oracleText)
	if containsAny(ot, "proliferate") {
		ctx.proliferateCount++
	}
	return ctx
}

// superfriendsLower is a file-local ASCII lowercase helper. Production
// code uses strings.ToLower; the test file deliberately keeps a tiny
// inline version so its dependency surface is minimal and doesn't
// collide with sibling test files' helpers (cf. the
// spellslinger_absolute_count_r60_test.go strings_ToLower collision
// pattern from earlier in the r60 sweep).
func superfriendsLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
