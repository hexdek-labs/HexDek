package main

import (
	"testing"
)

// TestKnownCombos_AllHaveClass guards against future additions to the
// KnownCombos slice forgetting the Class tag. Every curated entry must
// carry a non-empty Class — the hat affinity scorer relies on this
// invariant to keep its on-class/off-class judgement meaningful.
func TestKnownCombos_AllHaveClass(t *testing.T) {
	for i, kc := range KnownCombos {
		if kc.Class == "" {
			t.Errorf("KnownCombos[%d] %q missing Class tag", i, kc.Name)
		}
	}
}

// TestKnownCombos_ClassesAreFromHierarchy verifies that every Class
// value is one of the defined ComboClass* constants. Catches typos
// (ComboClassInfiniteToken vs ComboClassInfiniteTokens) at test time
// rather than at hat-eval time.
func TestKnownCombos_ClassesAreFromHierarchy(t *testing.T) {
	valid := map[string]bool{
		ComboClassInfiniteMana:    true,
		ComboClassInfiniteDamage:  true,
		ComboClassInfiniteDrain:   true,
		ComboClassInfiniteTokens:  true,
		ComboClassInfiniteETB:     true,
		ComboClassInfiniteMill:    true,
		ComboClassLibraryExileWin: true,
		ComboClassStormFinisher:   true,
		ComboClassETBPayoff:       true,
		ComboClassETBDoubler:      true,
		ComboClassBlinkEngine:     true,
		ComboClassManaSink:        true,
		ComboClassCombatFinisher:  true,
		ComboClassLockdown:        true,
		ComboClassUnknown:         true,
	}
	for _, kc := range KnownCombos {
		if !valid[kc.Class] {
			t.Errorf("KnownCombos %q has unknown Class %q", kc.Name, kc.Class)
		}
	}
}

// TestClassifyComboHeuristic_HitsExpectedClass exercises the heuristic
// classifier against representative shapes the Spellbook import surface
// is known to emit. Each case is a "name + description" pair drawn
// from realistic Spellbook entries; the expected class is the one a
// human curator would have picked.
func TestClassifyComboHeuristic_HitsExpectedClass(t *testing.T) {
	cases := []struct {
		name   string
		desc   string
		pieces []string
		want   string
	}{
		{
			"Thassa's Oracle + Demonic Consultation",
			"Cast Consultation, exile your library. Oracle ETB with empty library wins.",
			[]string{"Thassa's Oracle", "Demonic Consultation"},
			ComboClassLibraryExileWin,
		},
		{
			"Aetherflux Reservoir + Storm",
			"Each spell cast triggers Aetherflux. With high storm count, pay 50 life to deal 50.",
			[]string{"Aetherflux Reservoir"},
			ComboClassStormFinisher,
		},
		{
			"Sanguine Bond + Exquisite Blood",
			"Infinite drain loop: lifegain triggers drain, drain triggers lifegain.",
			[]string{"Sanguine Bond", "Exquisite Blood"},
			ComboClassInfiniteDrain,
		},
		{
			"Altar of Dementia + Karmic Guide",
			"Sacrifice Karmic Guide to mill an opponent for 3. With recursion, mill the entire library.",
			[]string{"Altar of Dementia", "Karmic Guide"},
			ComboClassInfiniteMill,
		},
		{
			"Kiki-Jiki + Conscripts",
			"Kiki creates a token copy. The copy untaps Kiki. Infinite hasty tokens.",
			[]string{"Kiki-Jiki, Mirror Breaker", "Zealous Conscripts"},
			ComboClassInfiniteTokens,
		},
		{
			"Walking Ballista + Heliod",
			"Lifelink Ballista pings for 1, gains life, Heliod adds counter. Infinite damage.",
			[]string{"Walking Ballista", "Heliod, Sun-Crowned"},
			ComboClassInfiniteDamage,
		},
		{
			"Basalt Monolith + Power Artifact",
			"Power Artifact reduces Monolith untap to 0. Tap for 3, untap for 0. Infinite mana.",
			[]string{"Basalt Monolith", "Power Artifact"},
			ComboClassInfiniteMana,
		},
		{
			"Purphoros + Tokens",
			"Every creature ETB deals 2 damage to each opponent.",
			[]string{"Purphoros, God of the Forge"},
			ComboClassETBPayoff,
		},
		{
			"Panharmonicon",
			"Panharmonicon doubles all artifact and creature ETB triggers.",
			[]string{"Panharmonicon"},
			ComboClassETBDoubler,
		},
		{
			"Conjurer's Closet Engine",
			"Conjurer's Closet blinks one creature every end step.",
			[]string{"Conjurer's Closet"},
			ComboClassBlinkEngine,
		},
		{
			"Staff of Domination + Infinite Mana",
			"Any mana sink: draw entire deck with Staff of Domination.",
			[]string{"Staff of Domination"},
			ComboClassManaSink,
		},
		{
			"Craterhoof Behemoth swing",
			"Craterhoof Behemoth pumps board for lethal trample swing.",
			[]string{"Craterhoof Behemoth"},
			ComboClassCombatFinisher,
		},
		{
			"Worldgorger Dragon + Animate Dead",
			"Animate Dead targets Worldgorger Dragon. Infinite ETB triggers and infinite mana via land untap.",
			[]string{"Worldgorger Dragon", "Animate Dead"},
			ComboClassInfiniteETB,
		},
		{
			"Some janky synergy",
			"Two cards do a thing.",
			[]string{"Random Card A", "Random Card B"},
			ComboClassUnknown,
		},
	}
	for _, tc := range cases {
		got := ClassifyComboHeuristic(tc.name, tc.desc, tc.pieces)
		if got != tc.want {
			t.Errorf("ClassifyComboHeuristic(%q): want %q got %q",
				tc.name, tc.want, got)
		}
	}
}

// TestClassifySpellbookImported_BackfillsEmptyOnly verifies that the
// import-side classifier respects pre-tagged Class values (a hypothetical
// future Spellbook payload that DOES include class hints should not be
// overwritten) and only fills empties.
func TestClassifySpellbookImported_BackfillsEmptyOnly(t *testing.T) {
	imports := []KnownCombo{
		{
			Name:        "Pre-tagged combo",
			Description: "Infinite mana — but tagged as drain in the import",
			Pieces:      []string{"Card A", "Card B"},
			Class:       ComboClassInfiniteDrain, // intentionally weird tag
		},
		{
			Name:        "Untagged Thoracle line",
			Description: "Cast Consultation, exile library, Oracle ETB wins",
			Pieces:      []string{"Thassa's Oracle", "Demonic Consultation"},
			Class:       "",
		},
	}
	ClassifySpellbookImported(imports)

	if imports[0].Class != ComboClassInfiniteDrain {
		t.Errorf("pre-tagged Class should be preserved, got %q", imports[0].Class)
	}
	if imports[1].Class != ComboClassLibraryExileWin {
		t.Errorf("untagged Thoracle line should classify as library_exile_win, got %q", imports[1].Class)
	}
}
