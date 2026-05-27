package main

import (
	"strings"
	"testing"
)

// R60 gap fills for the meta-positioning matrix. The matrix-coverage
// audit at PR #560 found exactly 2 bidirectional gaps (4 entries) plus
// one impactful meta-axis entry (storm vs Graveyard Hate). These tests
// pin each gap's rating, reasoning content, and reciprocal symmetry so
// future DB edits can't silently drop them.

// findMatchup is a small lookup helper that returns the MetaMatchup for
// a given opponent label, or nil when absent. Tests use it to assert
// both presence AND content of the new entries.
func findMatchup(matchups []MetaMatchup, vsArch string) *MetaMatchup {
	for i := range matchups {
		if matchups[i].Archetype == vsArch {
			return &matchups[i]
		}
	}
	return nil
}

// Gap 1: control → Enchantress (favored). Pre-r60 the row had 8
// matchups but no Enchantress entry — control decks had a blind spot
// reading their position against an enchantment engine.
func TestMetaPositioning_ControlVsEnchantress_R60(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Control"}
	computeMetaPositioning(dp)
	hit := findMatchup(dp.MetaMatchups, "Enchantress")
	if hit == nil {
		t.Fatal("control deck missing Enchantress matchup (r60 gap fill regressed)")
	}
	if hit.Rating != "favored" {
		t.Errorf("control vs Enchantress: want favored, got %q", hit.Rating)
	}
	// Reasoning must name a concrete mechanic — vague hedges like
	// "control beats enchantress" would pass the rating check but
	// fail the user-facing utility check. Pin canonical cards.
	for _, needle := range []string{"counterspell", "argothian enchantress", "sythis"} {
		if !strings.Contains(strings.ToLower(hit.Reason), needle) {
			t.Errorf("control vs Enchantress reason must name %q, got %q", needle, hit.Reason)
		}
	}
}

// Gap 2: aristocrats → Enchantress (favored). Drain math through
// Blood Artist / Zulaport bypasses pillow fort entirely — a structural
// asymmetry the matrix was missing pre-r60.
func TestMetaPositioning_AristocratsVsEnchantress_R60(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Aristocrats"}
	computeMetaPositioning(dp)
	hit := findMatchup(dp.MetaMatchups, "Enchantress")
	if hit == nil {
		t.Fatal("aristocrats deck missing Enchantress matchup (r60 gap fill regressed)")
	}
	if hit.Rating != "favored" {
		t.Errorf("aristocrats vs Enchantress: want favored, got %q", hit.Rating)
	}
	for _, needle := range []string{"blood artist", "ghostly prison", "pillow fort"} {
		if !strings.Contains(strings.ToLower(hit.Reason), needle) {
			t.Errorf("aristocrats vs Enchantress reason must name %q, got %q", needle, hit.Reason)
		}
	}
}

// Gap 3: enchantress → Control (unfavored). The reciprocal of gap 1.
// Reciprocity invariant alone would catch a missing entry, but doesn't
// validate the reason text — pin the canonical mechanic names here.
func TestMetaPositioning_EnchantressVsControl_R60(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Enchantress"}
	computeMetaPositioning(dp)
	hit := findMatchup(dp.MetaMatchups, "Control")
	if hit == nil {
		t.Fatal("enchantress deck missing Control matchup (r60 gap fill regressed)")
	}
	if hit.Rating != "unfavored" {
		t.Errorf("enchantress vs Control: want unfavored (reciprocal of control favored), got %q", hit.Rating)
	}
	for _, needle := range []string{"counterspell", "argothian enchantress"} {
		if !strings.Contains(strings.ToLower(hit.Reason), needle) {
			t.Errorf("enchantress vs Control reason must name %q, got %q", needle, hit.Reason)
		}
	}
}

// Gap 4: enchantress → Aristocrats (unfavored). The reciprocal of gap
// 2. Same content-validity discipline.
func TestMetaPositioning_EnchantressVsAristocrats_R60(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Enchantress"}
	computeMetaPositioning(dp)
	hit := findMatchup(dp.MetaMatchups, "Aristocrats")
	if hit == nil {
		t.Fatal("enchantress deck missing Aristocrats matchup (r60 gap fill regressed)")
	}
	if hit.Rating != "unfavored" {
		t.Errorf("enchantress vs Aristocrats: want unfavored, got %q", hit.Rating)
	}
	for _, needle := range []string{"blood artist", "pillow fort"} {
		if !strings.Contains(strings.ToLower(hit.Reason), needle) {
			t.Errorf("enchantress vs Aristocrats reason must name %q, got %q", needle, hit.Reason)
		}
	}
}

// Gap 5: storm → Graveyard Hate (unfavored). Meta-axis entry (no
// reciprocal expected per archetypeLabelToKey). Storm decks pivoting
// through Underworld Breach / Past in Flames / Yawgmoth's Will lose
// the pivot piece to RIP / Leyline — the matrix was missing this even
// though the equivalent reanimator + aristocrats entries existed.
func TestMetaPositioning_StormVsGraveyardHate_R60(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Storm"}
	computeMetaPositioning(dp)
	hit := findMatchup(dp.MetaMatchups, "Graveyard Hate")
	if hit == nil {
		t.Fatal("storm deck missing Graveyard Hate matchup (r60 gap fill regressed)")
	}
	if hit.Rating != "unfavored" {
		t.Errorf("storm vs Graveyard Hate: want unfavored, got %q", hit.Rating)
	}
	for _, needle := range []string{"underworld breach", "rest in peace", "leyline"} {
		if !strings.Contains(strings.ToLower(hit.Reason), needle) {
			t.Errorf("storm vs Graveyard Hate reason must name %q, got %q", needle, hit.Reason)
		}
	}
}

// Matrix-completeness check: with the r60 fills in place, every pair
// in the canonical 10-archetype set must have a bidirectional entry.
// Pins the "no blank cells" invariant so a future row addition that
// drops half a reciprocal pair fails this test in addition to the
// existing reciprocity check.
func TestMetaMatchups_NoBlankCellsBetweenCanonicalArchetypes(t *testing.T) {
	canonical := []string{
		"aggro", "combo", "control", "aristocrats", "voltron",
		"stax", "reanimator", "storm", "enchantress", "midrange",
	}
	has := map[string]map[string]bool{}
	for archA, matchups := range metaMatchupDB {
		has[archA] = map[string]bool{}
		for _, m := range matchups {
			if k, ok := archetypeLabelToKey[m.vsArchetype]; ok {
				has[archA][k] = true
			}
		}
	}
	var gaps []string
	for _, a := range canonical {
		for _, b := range canonical {
			if a == b {
				continue
			}
			if !has[a][b] {
				gaps = append(gaps, a+"→"+b)
			}
		}
	}
	if len(gaps) > 0 {
		t.Errorf("matrix has %d blank cells between canonical archetypes: %v", len(gaps), gaps)
	}
}

// New entries default to "moderate" strength (none of the r60 fills
// are mechanical hard-locks). Pins this so a future override-table
// edit that accidentally promotes them to "strong" gets caught.
func TestMetaPositioning_R60FillsDefaultModerateStrength(t *testing.T) {
	cases := []struct {
		archetype string
		vsArch    string
	}{
		{"Control", "Enchantress"},
		{"Aristocrats", "Enchantress"},
		{"Enchantress", "Control"},
		{"Enchantress", "Aristocrats"},
	}
	for _, tc := range cases {
		dp := &DeckProfile{PrimaryArchetype: tc.archetype}
		computeMetaPositioning(dp)
		hit := findMatchup(dp.MetaMatchups, tc.vsArch)
		if hit == nil {
			t.Errorf("%s vs %s: matchup not found", tc.archetype, tc.vsArch)
			continue
		}
		if hit.Strength != "moderate" {
			t.Errorf("%s vs %s: want strength 'moderate' (r60 fills aren't hard-locks), got %q",
				tc.archetype, tc.vsArch, hit.Strength)
		}
	}
}

// MetaStrongAgainst (reverse-lookup pass) must surface the new
// enchantress unfavored entries from the opponent's perspective.
// A control deck asking "what do I beat?" should now include
// Enchantress; same for aristocrats. Defends against the case where
// the forward entry exists but the reverse-lookup logic missed it.
func TestMetaStrongAgainst_PicksUpR60EnchantressGapFills(t *testing.T) {
	control := MetaStrongAgainst("Control")
	if findStrongAgainst(control, "Enchantress") == nil {
		t.Error("control's StrongAgainst missing Enchantress (forward entry should surface here)")
	}
	aristocrats := MetaStrongAgainst("Aristocrats")
	if findStrongAgainst(aristocrats, "Enchantress") == nil {
		t.Error("aristocrats' StrongAgainst missing Enchantress (forward entry should surface here)")
	}
}

func findStrongAgainst(adv []MetaAdvantage, name string) *MetaAdvantage {
	for i := range adv {
		if adv[i].Archetype == name {
			return &adv[i]
		}
	}
	return nil
}
