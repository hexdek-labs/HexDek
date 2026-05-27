package main

import (
	"strings"
	"testing"
)

// makeRock builds a CMC-2 mana-producing artifact profile for fast-mana
// tests. The Produces ResMana flag is the signal the production code
// keys on; CMC<=2 puts the card in the fast-mana eligibility window.
func makeRock(name string, cmc int) CardProfile {
	return CardProfile{
		Name:     name,
		CMC:      cmc,
		TypeLine: "Artifact",
		Produces: []ResourceType{ResMana},
	}
}

// TestFastMana_UntappedSolRingCounts pins the canonical baseline: Sol
// Ring (CMC 1, untapped, taps for {C}{C}) IS fast mana. Sanity check
// that the refactor didn't break the untapped path.
func TestFastMana_UntappedSolRingCounts(t *testing.T) {
	ctx := buildContextFor(
		[]CardProfile{makeRock("Sol Ring", 1)},
		map[string]string{"Sol Ring": "{t}: add {c}{c}."},
	)
	if ctx.fastManaCount != 1 {
		t.Errorf("Sol Ring should count as fast mana, got fastManaCount=%d", ctx.fastManaCount)
	}
	if ctx.tappedManaCount != 0 {
		t.Errorf("Sol Ring is NOT tapped-ETB, tappedManaCount=%d", ctx.tappedManaCount)
	}
}

// TestFastMana_ColdsteelHeartDoesNotCount is the canonical refinement:
// Coldsteel Heart (CMC 2, ETB tapped) was counted as fast mana under
// the old detection alongside Sol Ring. After the refinement it
// belongs in tappedManaCount, not fastManaCount, because it doesn't
// tap-for-mana the turn cast and so doesn't drive the T2/T3 wincon
// tempo that the B4 bracket signal measures.
func TestFastMana_ColdsteelHeartDoesNotCount(t *testing.T) {
	ctx := buildContextFor(
		[]CardProfile{makeRock("Coldsteel Heart", 2)},
		map[string]string{
			"Coldsteel Heart": "coldsteel heart enters tapped. choose a color when this enters. {t}: add one mana of the chosen color.",
		},
	)
	if ctx.fastManaCount != 0 {
		t.Errorf("Coldsteel Heart should NOT count as fast mana, got fastManaCount=%d", ctx.fastManaCount)
	}
	if ctx.tappedManaCount != 1 {
		t.Errorf("Coldsteel Heart should land in tappedManaCount, got %d", ctx.tappedManaCount)
	}
	if len(ctx.tappedManaNames) != 1 || ctx.tappedManaNames[0] != "Coldsteel Heart" {
		t.Errorf("tappedManaNames wrong: %v", ctx.tappedManaNames)
	}
}

// TestFastMana_DiamondCycleExcluded verifies the whole Diamond cycle
// (Charcoal / Marble / Sky / Fire / Moss) — all CMC-2 ETB-tapped
// off-color rocks — gets pulled out of fast-mana count via the curated
// name match. The curated set's job is to backstop oracle-less classify
// runs.
func TestFastMana_DiamondCycleExcluded(t *testing.T) {
	cards := []CardProfile{
		makeRock("Charcoal Diamond", 2),
		makeRock("Marble Diamond", 2),
		makeRock("Sky Diamond", 2),
		makeRock("Fire Diamond", 2),
		makeRock("Moss Diamond", 2),
	}
	// Intentionally empty oracle map — exercises the curated-set
	// fallback path (the whole point of having tappedETBRocks).
	ctx := buildContextFor(cards, map[string]string{})
	if ctx.fastManaCount != 0 {
		t.Errorf("Diamond cycle should be fully excluded via curated set, got fastManaCount=%d", ctx.fastManaCount)
	}
	if ctx.tappedManaCount != 5 {
		t.Errorf("Diamond cycle should all land in tappedManaCount, got %d", ctx.tappedManaCount)
	}
}

// TestFastMana_MixedDeckSplitsCorrectly verifies the partitioning on a
// realistic ramp suite: 4 untapped (Sol Ring, Arcane Signet, Mind
// Stone, Talisman) + 2 tapped (Coldsteel Heart, Star Compass) splits
// 4 fast / 2 tapped. The B4 fast-mana signal at the >=6 tier should
// not fire for this 6-rock suite (4 < 6) — the old detection would
// have lifted it to "Fast mana 6-9, +2 points" incorrectly. Mana
// Crypt is intentionally NOT in this fixture even though it'd be a
// canonical fast rock: it lives on commanderBannedList and is
// excluded by an earlier `continue` upstream of the fast-mana check.
func TestFastMana_MixedDeckSplitsCorrectly(t *testing.T) {
	cards := []CardProfile{
		makeRock("Sol Ring", 1),
		makeRock("Arcane Signet", 2),
		makeRock("Mind Stone", 2),
		makeRock("Talisman of Dominance", 2),
		makeRock("Coldsteel Heart", 2),
		makeRock("Star Compass", 2),
	}
	ot := map[string]string{
		"Sol Ring":              "{t}: add {c}{c}.",
		"Arcane Signet":         "{t}: add one mana of any color in your commander's color identity.",
		"Mind Stone":            "{t}: add {c}. {1}, {t}, sacrifice mind stone: draw a card.",
		"Talisman of Dominance": "{t}: add {c}. {t}: add {u} or {b}. talisman of dominance deals 1 damage to you.",
		"Coldsteel Heart":       "coldsteel heart enters tapped. choose a color when this enters. {t}: add one mana of the chosen color.",
		"Star Compass":          "star compass enters tapped. {t}: add one mana of any color that a basic land you control could produce.",
	}
	ctx := buildContextFor(cards, ot)

	if ctx.fastManaCount != 4 {
		t.Errorf("mixed deck: fastManaCount=%d, want 4 (Sol Ring, Signet, Mind Stone, Talisman)",
			ctx.fastManaCount)
	}
	if ctx.tappedManaCount != 2 {
		t.Errorf("mixed deck: tappedManaCount=%d, want 2 (Coldsteel, Star Compass)",
			ctx.tappedManaCount)
	}
}

// TestFastMana_OracleTextFallbackForUnknownRock verifies the oracle-
// text scan catches tapped-ETB rocks that aren't in the curated list.
// Future printings shouldn't need a curated-set update to be correctly
// classified — the "enters tapped" phrase carries the signal directly.
func TestFastMana_OracleTextFallbackForUnknownRock(t *testing.T) {
	ctx := buildContextFor(
		[]CardProfile{makeRock("Hypothetical Slow Rock", 2)},
		map[string]string{
			"Hypothetical Slow Rock": "this artifact enters the battlefield tapped. {t}: add one mana of any color.",
		},
	)
	if ctx.fastManaCount != 0 {
		t.Errorf("oracle-text 'enters the battlefield tapped' should exclude, got fastManaCount=%d", ctx.fastManaCount)
	}
	if ctx.tappedManaCount != 1 {
		t.Errorf("oracle-fallback didn't fire, tappedManaCount=%d", ctx.tappedManaCount)
	}
}

// TestFastMana_HighCMCRockUnaffected verifies that CMC-3+ rocks (which
// were already excluded from fastManaCount via the CMC<=2 gate) don't
// accidentally route into tappedManaCount either. The new counter is
// scoped to the same fast-mana eligibility window.
func TestFastMana_HighCMCRockUnaffected(t *testing.T) {
	ctx := buildContextFor(
		[]CardProfile{makeRock("Worn Powerstone", 3)},
		map[string]string{
			"Worn Powerstone": "worn powerstone enters tapped. {t}: add {c}{c}.",
		},
	)
	if ctx.fastManaCount != 0 {
		t.Errorf("CMC-3 rock must not count as fast mana, got %d", ctx.fastManaCount)
	}
	if ctx.tappedManaCount != 0 {
		t.Errorf("CMC-3 rock must not count as tapped-mana either (scoped to CMC<=2), got %d", ctx.tappedManaCount)
	}
}

// TestFastMana_LandsIgnored verifies lands are still skipped entirely
// (the loop's IsLand continue runs before fast-mana detection). A
// tapped dual land must not pollute tappedManaCount — that counter is
// specifically about non-land ramp.
func TestFastMana_LandsIgnored(t *testing.T) {
	land := CardProfile{
		Name:     "Tapped Dual Land",
		CMC:      0,
		IsLand:   true,
		TypeLine: "Land",
		Produces: []ResourceType{ResMana},
	}
	ctx := buildContextFor(
		[]CardProfile{land},
		map[string]string{"Tapped Dual Land": "this land enters tapped. {t}: add {u} or {b}."},
	)
	if ctx.fastManaCount != 0 || ctx.tappedManaCount != 0 {
		t.Errorf("land should be skipped entirely, got fast=%d tapped=%d",
			ctx.fastManaCount, ctx.tappedManaCount)
	}
}

// TestFastMana_IsTappedETBRockUnit pins the helper directly so future
// curation changes are easy to verify in isolation. Covers the
// curated-name path, both oracle-text phrasings, and the no-match
// baseline (Sol Ring oracle).
func TestFastMana_IsTappedETBRockUnit(t *testing.T) {
	cases := []struct {
		name string
		ot   string
		want bool
	}{
		{"Coldsteel Heart", "", true},                                                       // curated
		{"coldsteel heart", "", true},                                                       // case-insensitive curated
		{"Random Rock", "this artifact enters tapped. {t}: add {c}.", true},                 // modern oracle phrasing
		{"Random Rock", "this artifact enters the battlefield tapped. {t}: add {c}.", true}, // legacy phrasing
		{"Sol Ring", "{t}: add {c}{c}.", false},                                             // untapped baseline
		{"Mana Vault", "mana vault doesn't untap during your untap step. {t}: add {c}{c}{c}.", false}, // doesn't ETB tapped
	}
	for _, tc := range cases {
		if got := isTappedETBRock(tc.name, tc.ot); got != tc.want {
			t.Errorf("isTappedETBRock(%q, %q) = %v, want %v", tc.name, tc.ot, got, tc.want)
		}
	}
}

// TestFastMana_BracketRationaleNoteFires verifies that when
// tappedManaCount > 0, the bracket rationale picks up a Kind="note"
// signal explaining the exclusion so the deckbuilder sees why their 6
// rocks didn't trip the Fast-mana 6-9 tier.
func TestFastMana_BracketRationaleNoteFires(t *testing.T) {
	cards := []CardProfile{
		makeRock("Sol Ring", 1),
		makeRock("Arcane Signet", 2),
		makeRock("Coldsteel Heart", 2),
		makeRock("Star Compass", 2),
	}
	ot := map[string]string{
		"Sol Ring":        "{t}: add {c}{c}.",
		"Arcane Signet":   "{t}: add one mana of any color in your commander's color identity.",
		"Coldsteel Heart": "coldsteel heart enters tapped. {t}: add one mana of the chosen color.",
		"Star Compass":    "star compass enters tapped. {t}: add one mana of any color a basic land could produce.",
	}
	ctx := buildContextFor(cards, ot)
	_, _, rationale := estimateMeasuredBracket(ctx, &FreyaReport{}, "")

	var note *BracketSignal
	for i := range rationale.Signals {
		s := &rationale.Signals[i]
		if s.Name == "Fast mana" && s.Kind == "note" {
			note = s
			break
		}
	}
	if note == nil {
		t.Fatalf("expected Fast-mana exclusion note, got signals: %+v", rationale.Signals)
	}
	if !strings.Contains(note.Note, "Coldsteel Heart") || !strings.Contains(note.Note, "Star Compass") {
		t.Errorf("note should name the excluded rocks, got: %q", note.Note)
	}
	if !strings.Contains(note.Note, "2 tapped-ETB rock") {
		t.Errorf("note should report the count, got: %q", note.Note)
	}
}

// TestFastMana_NoNoteWithoutTappedRocks pins the inverse: a clean
// untapped-only ramp suite gets no note (the signal is only worth
// surfacing when there's something to explain).
func TestFastMana_NoNoteWithoutTappedRocks(t *testing.T) {
	cards := []CardProfile{
		makeRock("Sol Ring", 1),
		makeRock("Arcane Signet", 2),
	}
	ot := map[string]string{
		"Sol Ring":      "{t}: add {c}{c}.",
		"Arcane Signet": "{t}: add one mana of any color in your commander's color identity.",
	}
	ctx := buildContextFor(cards, ot)
	_, _, rationale := estimateMeasuredBracket(ctx, &FreyaReport{}, "")

	for _, s := range rationale.Signals {
		if s.Name == "Fast mana" && s.Kind == "note" {
			t.Errorf("untapped-only suite should NOT fire the exclusion note, got: %+v", s)
		}
	}
}
