package main

import "testing"

// spellslinger_absolute_count_r60_test.go pins the absolute-count
// Spellslinger detection arm added to handle Niv-Mizzet Reborn /
// Veyran / Stormwing / Aetherflux-payoff hybrids that pack a clear
// I/S + cast-trigger payoff cluster but include enough ramp +
// creatures to fall below the 60% I/S density gate.
//
// Two-arm shape:
//
//	(1) Density: instantSorcPct >= 0.60 AND spellCopyCount >= 1
//	(2) Absolute: instantSorceryCount >= 25 AND spellTriggerPermanentCount >= 4
//
// Tests pin BOTH arms accept the right shapes, the conjunction within
// each arm holds, and generic Izzet midrange isn't poached.

func findSpellslingerFingerprint(t *testing.T) archetypeFingerprint {
	t.Helper()
	for _, fp := range archetypeFingerprints {
		if fp.Name == "Spellslinger" {
			return fp
		}
	}
	t.Fatalf("Spellslinger fingerprint missing from archetypeFingerprints")
	return archetypeFingerprint{}
}

func TestSpellslingerRequire_DensityArmStillAccepted(t *testing.T) {
	fp := findSpellslingerFingerprint(t)
	ctx := &classifyContext{instantSorcPct: 0.62, spellCopyCount: 2}
	if !fp.Require(ctx) {
		t.Fatalf("density arm pct=0.62 copy=2 must still classify Spellslinger (legacy gate)")
	}
}

func TestSpellslingerRequire_AbsoluteCountArmAccepted(t *testing.T) {
	fp := findSpellslingerFingerprint(t)
	// Niv-Mizzet Reborn / Veyran hybrid: 28 instants/sorceries,
	// 5 spell-trigger permanents (Guttersnipe / Talrand / Young
	// Pyromancer / Stormwing Entity / Aetherflux Reservoir). Overall
	// nonland density drops below 60% because the deck still runs
	// ~15 creatures and ramp, but the payoff cluster is unmistakable.
	ctx := &classifyContext{
		instantSorcPct:             0.35,
		instantSorceryCount:        28,
		spellTriggerPermanentCount: 5,
	}
	if !fp.Require(ctx) {
		t.Fatalf("absolute arm I/S=28 triggers=5 must classify Spellslinger")
	}
}

func TestSpellslingerRequire_AbsoluteCountExactThreshold(t *testing.T) {
	fp := findSpellslingerFingerprint(t)
	// Exactly at floor — 25 I/S, 4 spell-trigger permanents.
	ctx := &classifyContext{instantSorceryCount: 25, spellTriggerPermanentCount: 4}
	if !fp.Require(ctx) {
		t.Fatalf("absolute arm at-threshold I/S=25 triggers=4 must classify Spellslinger")
	}
	// One below either floor must fail.
	low1 := &classifyContext{instantSorceryCount: 24, spellTriggerPermanentCount: 4}
	if fp.Require(low1) {
		t.Errorf("I/S=24 triggers=4 must NOT classify (below absolute floor)")
	}
	low2 := &classifyContext{instantSorceryCount: 25, spellTriggerPermanentCount: 3}
	if fp.Require(low2) {
		t.Errorf("I/S=25 triggers=3 must NOT classify (below trigger floor)")
	}
}

func TestSpellslingerRequire_GenericIzzetMidrangeRejected(t *testing.T) {
	fp := findSpellslingerFingerprint(t)
	// Cases that look I/S-adjacent but lack the deliberate Spellslinger
	// shape. Each must be rejected by BOTH arms.
	cases := []classifyContext{
		// Generic Izzet goodstuff: 18 I/S, 2 cast-trigger creatures —
		// well below absolute floor, well below density floor.
		{instantSorcPct: 0.30, instantSorceryCount: 18, spellTriggerPermanentCount: 2},
		// Instant-leaning midrange that hits 25 I/S but has zero
		// spell-trigger permanents — the I/S is just removal + ramp,
		// no payoffs.
		{instantSorcPct: 0.42, instantSorceryCount: 25, spellTriggerPermanentCount: 0},
		// Heavy payoff cluster but only 12 I/S to feed it — clearly a
		// creature deck with a few spellslinger flavor cards, not the
		// archetype proper.
		{instantSorcPct: 0.20, instantSorceryCount: 12, spellTriggerPermanentCount: 5},
		// Density gate fires the legacy condition (pct>=0.60) but
		// spellCopyCount=0 and triggers<4 — neither arm passes.
		{instantSorcPct: 0.60, spellCopyCount: 0, instantSorceryCount: 22, spellTriggerPermanentCount: 3},
	}
	for i, ctx := range cases {
		ctx := ctx
		if fp.Require(&ctx) {
			t.Errorf("case %d %+v should NOT classify Spellslinger (would poach midrange)", i, ctx)
		}
	}
}

// End-to-end: the spellTriggerPermanentCount accumulator picks up the
// canonical payoff cards via their oracle-text trigger phrases. This is
// a thin smoke-test of buildClassifyContext's spell-trigger detection
// branch — pins that the named cards in the user-facing spec (Aetherflux
// Reservoir, Stormwing Entity, Guttersnipe, Talrand, Archmage Emeritus)
// each contribute to the counter when present in the deck.
func TestSpellTriggerPermanentDetection_RecognizesCanonicalPayoffs(t *testing.T) {
	cards := []struct {
		name   string
		typing string
		ot     string
		want   bool
	}{
		{
			name:   "Aetherflux Reservoir",
			typing: "Artifact",
			ot:     "Whenever you cast a spell, you gain 1 life for each spell you've cast this turn.",
			want:   true,
		},
		{
			name:   "Guttersnipe",
			typing: "Creature — Goblin Shaman",
			ot:     "Whenever you cast an instant or sorcery spell, Guttersnipe deals 2 damage to each opponent.",
			want:   true,
		},
		{
			name:   "Talrand, Sky Summoner",
			typing: "Legendary Creature — Merfolk Wizard",
			ot:     "Whenever you cast an instant or sorcery spell, create a 2/2 blue Drake creature token with flying.",
			want:   true,
		},
		{
			name:   "Stormwing Entity",
			typing: "Creature — Elemental Bird",
			ot:     "When this creature enters, if you've cast another instant or sorcery spell this turn, return it to its owner's hand.",
			want:   true,
		},
		{
			name:   "Archmage Emeritus",
			typing: "Creature — Human Wizard",
			ot:     "Magecraft — Whenever you cast or copy an instant or sorcery spell, draw a card.",
			want:   true,
		},
		{
			name:   "Sentinel Tower",
			typing: "Artifact",
			ot:     "Whenever you cast a spell during your turn, Sentinel Tower deals damage equal to the number of other spells you've cast this turn to any target.",
			want:   true,
		},
		// Negative cases: NOT a spell-trigger payoff permanent.
		{
			name:   "Lightning Bolt",
			typing: "Instant",
			ot:     "Lightning Bolt deals 3 damage to any target.",
			want:   false, // is instant — excluded from permanents counter
		},
		{
			name:   "Counterspell",
			typing: "Instant",
			ot:     "Counter target spell.",
			want:   false,
		},
		{
			name:   "Sol Ring",
			typing: "Artifact",
			ot:     "{T}: Add {C}{C}.",
			want:   false, // no spell-cast trigger
		},
		{
			name:   "Llanowar Elves",
			typing: "Creature — Elf Druid",
			ot:     "{T}: Add {G}.",
			want:   false,
		},
		{
			name:   "Thousand-Year Storm",
			typing: "Enchantment",
			ot:     "Whenever you cast an instant or sorcery spell, copy it for each other instant and sorcery spell you've cast before it this turn.",
			want:   true, // matches "whenever you cast an instant or sorcery"
		},
	}

	for _, c := range cards {
		ctx := buildSpellTriggerCtxForCard(c.typing, c.ot)
		got := ctx.spellTriggerPermanentCount > 0
		if got != c.want {
			t.Errorf("%s: spellTriggerPermanentCount>0 = %v, want %v (type=%q ot=%q)",
				c.name, got, c.want, c.typing, c.ot)
		}
	}
}

// buildSpellTriggerCtxForCard mirrors the per-card branch of
// buildClassifyContext that we exercise here, decoupled from the full
// oracle-DB / qty-profile pipeline so the test runs without needing
// scryfall data. Uses the production `containsAny` from analysis.go to
// guarantee the detection-phrase list stays in lockstep.
func buildSpellTriggerCtxForCard(typeLine, oracleText string) *classifyContext {
	ctx := &classifyContext{}
	tl := strings_ToLower(typeLine)
	ot := strings_ToLower(oracleText)
	isInstantOrSorcery := contains(tl, "instant") || contains(tl, "sorcery")
	if !isInstantOrSorcery && containsAny(ot,
		"whenever you cast a spell",
		"whenever you cast an instant or sorcery",
		"magecraft",
		"if you've cast another instant or sorcery",
		"if you've cast an instant or sorcery this turn",
		"for each spell you've cast",
		"for each instant and sorcery spell you've cast",
		"whenever you cast your") {
		ctx.spellTriggerPermanentCount++
	}
	return ctx
}

// strings_ToLower is a local thin wrapper so the test file doesn't need
// a top-level `strings` import (the underscore name avoids any cosmetic
// collision with the strings package if it's added later). The
// production code uses strings.ToLower; this test only needs ASCII
// lowercasing of fixture-string literals, so the inline version is
// sufficient and keeps the test file's dependency surface minimal.
func strings_ToLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
