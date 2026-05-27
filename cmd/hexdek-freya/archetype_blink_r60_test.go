package main

import (
	"testing"
)

// archetype_blink_r60_test.go — pins the r60 Blink / Flicker archetype gate.
//
// Audit verdict (pre-r60): the Blink fingerprint existed in
// archetypeFingerprints but required only `blinkCount >= 6`, which over-
// classified control decks that ran a handful of bounce/exile-and-return
// effects (Ghostly Flicker + Cyclonic Rift + Capsize) without the
// ETB-payoff density that makes blink actually function as a strategy.
//
// r60 gate is two-pronged:
//   1. blinkCount >= 5 (Conjurer's Closet, Cloudshift, Ghostly Flicker,
//      Eldrazi Displacer, Restoration Angel, Brago, Aminatou, Yorion,
//      Deadeye Navigator, Thassa Deep-Dwelling, Ephemerate, ...)
//   2. etbValueCreatureCount >= 8 (Mulldrifter, Reclamation Sage,
//      Eternal Witness, Wood Elves, Sun Titan, Cavalier of Gales,
//      Solemn Simulacrum, Mnemonic Wall, ...)
//
// Both prongs are load-bearing: tests pin failure when EITHER is short.

// -----------------------------------------------------------------------------
// 1. Counters — buildClassifyContext correctly increments both counters
// -----------------------------------------------------------------------------

func TestBuildClassifyContext_BlinkCounters(t *testing.T) {
	profiles := []CardProfile{
		// 5 blink-effect cards (IsBlinker via the analysis-layer detection)
		{Name: "Brago, King Eternal", TypeLine: "Legendary Creature — Human Soldier", IsBlinker: true},
		{Name: "Conjurer's Closet", TypeLine: "Artifact", IsBlinker: true},
		{Name: "Cloudshift", TypeLine: "Instant", IsBlinker: true},
		{Name: "Ghostly Flicker", TypeLine: "Instant", IsBlinker: true},
		{Name: "Eldrazi Displacer", TypeLine: "Creature — Eldrazi", IsBlinker: true},
		// 8 ETB-value CREATURES (HasValueETB + creature type line)
		{Name: "Mulldrifter", TypeLine: "Creature — Elemental", HasValueETB: true},
		{Name: "Reclamation Sage", TypeLine: "Creature — Elf Shaman", HasValueETB: true},
		{Name: "Eternal Witness", TypeLine: "Creature — Human Shaman", HasValueETB: true},
		{Name: "Wood Elves", TypeLine: "Creature — Elf Scout", HasValueETB: true},
		{Name: "Sun Titan", TypeLine: "Creature — Giant", HasValueETB: true},
		{Name: "Solemn Simulacrum", TypeLine: "Artifact Creature — Golem", HasValueETB: true},
		{Name: "Cavalier of Gales", TypeLine: "Creature — Elemental Knight", HasValueETB: true},
		{Name: "Mnemonic Wall", TypeLine: "Creature — Wall", HasValueETB: true},
		// Negative control: HasValueETB but NOT a creature must NOT count.
		{Name: "Smothering Tithe", TypeLine: "Enchantment", HasValueETB: true},
		// Negative control: creature but no ETB value must NOT count.
		{Name: "Grizzly Bears", TypeLine: "Creature — Bear"},
	}
	ctx := buildContextFor(profiles, nil)
	if ctx.blinkCount != 5 {
		t.Errorf("blinkCount = %d, want 5 (Brago + Closet + Cloudshift + Flicker + Displacer)", ctx.blinkCount)
	}
	if ctx.etbValueCreatureCount != 8 {
		t.Errorf("etbValueCreatureCount = %d, want 8 (Mulldrifter through Mnemonic Wall; Smothering Tithe + Grizzly Bears must NOT count)",
			ctx.etbValueCreatureCount)
	}
}

func TestBuildClassifyContext_BlinkCounters_NonCreatureETBExcluded(t *testing.T) {
	// Confirm Solemn Simulacrum DOES count (artifact CREATURE — the type
	// line contains "creature") even though it's also an artifact, but
	// pure non-creature ETB cards (Smothering Tithe, Phyrexian Arena)
	// do NOT count toward etbValueCreatureCount.
	profiles := []CardProfile{
		{Name: "Solemn Simulacrum", TypeLine: "Artifact Creature — Golem", HasValueETB: true},
		{Name: "Smothering Tithe", TypeLine: "Enchantment", HasValueETB: true},
		{Name: "Phyrexian Arena", TypeLine: "Enchantment", HasValueETB: true},
	}
	ctx := buildContextFor(profiles, nil)
	if ctx.etbValueCreatureCount != 1 {
		t.Errorf("etbValueCreatureCount = %d, want 1 (only Solemn Simulacrum is a creature)", ctx.etbValueCreatureCount)
	}
}

func TestBuildClassifyContext_BlinkOracleTextFallback(t *testing.T) {
	// Cards without IsBlinker pre-set still get counted if oracle text
	// matches the legacy patterns. Pin the oracle-text fallback so a
	// future refactor doesn't accidentally drop the secondary detection.
	profiles := []CardProfile{
		{Name: "Custom Flicker Spell", TypeLine: "Instant"},
		{Name: "Custom Exile-Return", TypeLine: "Instant"},
	}
	oracleText := map[string]string{
		"Custom Flicker Spell": "flicker target creature.",
		"Custom Exile-Return":  "exile target creature you control, then return it to the battlefield under its owner's control.",
	}
	ctx := buildContextFor(profiles, oracleText)
	if ctx.blinkCount != 2 {
		t.Errorf("blinkCount = %d, want 2 via oracle-text fallback", ctx.blinkCount)
	}
}

// -----------------------------------------------------------------------------
// 2. Fingerprint gate — both prongs are load-bearing
// -----------------------------------------------------------------------------

// makeBragoShapeFixture builds a deck shape that should classify as
// Blink: 5+ blink effects + 8+ ETB-value creatures, padded to a
// realistic 100-card profile.
func makeBragoShapeFixture() (profiles []CardProfile, roleCounts map[RoleTag]int, totalCards int) {
	profiles = []CardProfile{
		// 5 blink effects
		{Name: "Brago, King Eternal", TypeLine: "Legendary Creature — Human Soldier", IsBlinker: true},
		{Name: "Conjurer's Closet", TypeLine: "Artifact", IsBlinker: true},
		{Name: "Cloudshift", TypeLine: "Instant", IsBlinker: true},
		{Name: "Ghostly Flicker", TypeLine: "Instant", IsBlinker: true},
		{Name: "Eldrazi Displacer", TypeLine: "Creature — Eldrazi", IsBlinker: true},
		// 8 ETB-value creatures
		{Name: "Mulldrifter", TypeLine: "Creature — Elemental", HasValueETB: true},
		{Name: "Reclamation Sage", TypeLine: "Creature — Elf Shaman", HasValueETB: true},
		{Name: "Eternal Witness", TypeLine: "Creature — Human Shaman", HasValueETB: true},
		{Name: "Wood Elves", TypeLine: "Creature — Elf Scout", HasValueETB: true},
		{Name: "Sun Titan", TypeLine: "Creature — Giant", HasValueETB: true},
		{Name: "Solemn Simulacrum", TypeLine: "Artifact Creature — Golem", HasValueETB: true},
		{Name: "Cavalier of Gales", TypeLine: "Creature — Elemental Knight", HasValueETB: true},
		{Name: "Mnemonic Wall", TypeLine: "Creature — Wall", HasValueETB: true},
	}
	roleCounts = map[RoleTag]int{
		RoleThreat:  10,
		RoleDraw:    10,
		RoleRamp:    8,
		RoleRemoval: 6,
	}
	totalCards = 100
	return
}

func TestClassifyArchetype_BragoShape_RoutesToBlink(t *testing.T) {
	profiles, roleCounts, totalCards := makeBragoShapeFixture()
	ac := classifyFixture(profiles, nil, roleCounts, totalCards)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary != "Blink" {
		t.Errorf("Primary = %q, want %q (5 blinks + 8 ETB creatures should win the fingerprint)", ac.Primary, "Blink")
	}
}

func TestClassifyArchetype_FewBlinks_FallsOffBlink(t *testing.T) {
	// Counterfactual: 8 ETB-value creatures but only 3 blink effects —
	// blinkCount < 5 fails the Require gate. Must NOT classify as Blink.
	profiles, roleCounts, totalCards := makeBragoShapeFixture()
	// Strip 2 blink effects (Eldrazi Displacer is also a creature, so
	// it stays — but the 2 instants go).
	stripped := profiles[:0]
	dropped := 0
	for _, p := range profiles {
		if dropped < 2 && (p.Name == "Cloudshift" || p.Name == "Ghostly Flicker") {
			dropped++
			continue
		}
		stripped = append(stripped, p)
	}
	ac := classifyFixture(stripped, nil, roleCounts, totalCards)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Blink" {
		t.Errorf("Primary = %q — Blink fingerprint must NOT match with only 3 blink effects (< 5 threshold)", ac.Primary)
	}
}

func TestClassifyArchetype_FewETBCreatures_FallsOffBlink(t *testing.T) {
	// Counterfactual: 5 blink effects but only 5 ETB-value creatures —
	// etbValueCreatureCount < 8 fails the Require gate. This is the
	// "control deck with some bounce" shape that pre-r60 over-classified.
	profiles, roleCounts, totalCards := makeBragoShapeFixture()
	stripped := profiles[:0]
	dropped := 0
	for _, p := range profiles {
		if dropped < 3 && p.HasValueETB && (p.Name == "Cavalier of Gales" || p.Name == "Mnemonic Wall" || p.Name == "Solemn Simulacrum") {
			dropped++
			continue
		}
		stripped = append(stripped, p)
	}
	ac := classifyFixture(stripped, nil, roleCounts, totalCards)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Blink" {
		t.Errorf("Primary = %q — Blink fingerprint must NOT match with only 5 ETB-value creatures (< 8 threshold)", ac.Primary)
	}
}

func TestClassifyArchetype_NeitherProng_FallsOffBlink(t *testing.T) {
	// Counterfactual: 3 blink effects + 4 ETB creatures — both prongs
	// fail. Pure control deck shape.
	profiles := []CardProfile{
		{Name: "Brago, King Eternal", TypeLine: "Legendary Creature — Human Soldier", IsBlinker: true},
		{Name: "Cloudshift", TypeLine: "Instant", IsBlinker: true},
		{Name: "Ghostly Flicker", TypeLine: "Instant", IsBlinker: true},
		{Name: "Mulldrifter", TypeLine: "Creature — Elemental", HasValueETB: true},
		{Name: "Reclamation Sage", TypeLine: "Creature — Elf Shaman", HasValueETB: true},
		{Name: "Eternal Witness", TypeLine: "Creature — Human Shaman", HasValueETB: true},
		{Name: "Solemn Simulacrum", TypeLine: "Artifact Creature — Golem", HasValueETB: true},
	}
	roleCounts := map[RoleTag]int{
		RoleThreat:  4,
		RoleDraw:    12,
		RoleRamp:    8,
		RoleRemoval: 8,
		RoleCounterspell: 8,
	}
	ac := classifyFixture(profiles, nil, roleCounts, 100)
	if ac == nil {
		t.Fatal("ClassifyArchetype returned nil")
	}
	if ac.Primary == "Blink" {
		t.Errorf("Primary = %q — Blink fingerprint must NOT match with 3 blinks + 4 ETB creatures (both prongs short)", ac.Primary)
	}
}

// -----------------------------------------------------------------------------
// 3. Archetype definition surface — Brago / Yorion / Aminatou in KeyCards
// -----------------------------------------------------------------------------

func TestArchetypes_BlinkFlickerKeyCards_IncludesCanonicalCommanders(t *testing.T) {
	var blinkDef *ArchetypeDef
	for i := range Archetypes {
		if Archetypes[i].Name == "Blink / Flicker" {
			blinkDef = &Archetypes[i]
			break
		}
	}
	if blinkDef == nil {
		t.Fatal("Blink / Flicker archetype missing from Archetypes")
	}
	want := []string{"Brago, King Eternal", "Yorion, Sky Nomad", "Aminatou, the Fateshifter", "Eldrazi Displacer", "Cloudshift"}
	for _, w := range want {
		found := false
		for _, k := range blinkDef.KeyCards {
			if k == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Blink / Flicker KeyCards must include %q, got %v", w, blinkDef.KeyCards)
		}
	}
}
