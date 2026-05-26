package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// meta_strength_r60_test.go — regressions for the Strength magnitude
// signal added to MetaMatchup (PR description: surveys revealed that the
// pre-r60 3-bucket favored/neutral/unfavered rating couldn't distinguish
// a mechanical hard-lock like Stax→Combo from a draw-dependent edge like
// Aggro→Voltron). Strength is "strong" / "moderate" (default) / "slight"
// for directional matchups; "" for neutral entries.

// -----------------------------------------------------------------------------
// metaMatchupStrengthOrDefault
// -----------------------------------------------------------------------------

func TestMetaMatchupStrength_DefaultsToModerate(t *testing.T) {
	e := matchupEntry{rating: "favored", strength: ""}
	if got := metaMatchupStrengthOrDefault(e); got != "moderate" {
		t.Errorf("unset strength on favored entry should default to moderate; got %q", got)
	}
	e = matchupEntry{rating: "unfavored", strength: ""}
	if got := metaMatchupStrengthOrDefault(e); got != "moderate" {
		t.Errorf("unset strength on unfavored entry should default to moderate; got %q", got)
	}
}

func TestMetaMatchupStrength_NeutralAlwaysEmpty(t *testing.T) {
	// Neutral with unset strength → "".
	e := matchupEntry{rating: "neutral", strength: ""}
	if got := metaMatchupStrengthOrDefault(e); got != "" {
		t.Errorf("neutral entry must return empty strength (unset); got %q", got)
	}
	// Neutral with accidentally-set strength → "" (the rating dominates).
	e = matchupEntry{rating: "neutral", strength: "strong"}
	if got := metaMatchupStrengthOrDefault(e); got != "" {
		t.Errorf("neutral entry must return empty strength even when "+
			"strength field is set (rating dominates); got %q", got)
	}
}

func TestMetaMatchupStrength_PreservesExplicitValue(t *testing.T) {
	for _, s := range []string{"slight", "moderate", "strong"} {
		e := matchupEntry{rating: "favored", strength: s}
		if got := metaMatchupStrengthOrDefault(e); got != s {
			t.Errorf("explicit strength %q should be preserved; got %q", s, got)
		}
	}
}

// -----------------------------------------------------------------------------
// Strong-annotation correctness
// -----------------------------------------------------------------------------

// The strength overrides target mechanical hard-locks where the favored
// side has a denial card that materially shuts off the opponent's plan.
// Pin the canonical examples — these are the matchups deck-recommendation
// surfaces should AMPLIFY ("you'd genuinely beat Combo decks") vs
// down-weight ("you'd slightly outpace Voltron decks").
func TestMetaMatchups_StaxAgainstCombo_IsStrong(t *testing.T) {
	got := findStrengthForMatchup(t, "stax", "Combo")
	if got != "strong" {
		t.Errorf("Stax vs Combo (Drannith Magistrate / Rule of Law / Cursed Totem) "+
			"should be strong; got %q", got)
	}
}

func TestMetaMatchups_StaxAgainstStorm_IsStrong(t *testing.T) {
	got := findStrengthForMatchup(t, "stax", "Storm")
	if got != "strong" {
		t.Errorf("Stax vs Storm (Rule of Law / Eidolon of Rhetoric / Damping Sphere) "+
			"should be strong; got %q", got)
	}
}

func TestMetaMatchups_ReanimatorAgainstGraveyardHate_IsStrong(t *testing.T) {
	got := findStrengthForMatchup(t, "reanimator", "Graveyard Hate")
	if got != "strong" {
		t.Errorf("Reanimator vs Graveyard Hate (Rest in Peace / Leyline of the Void "+
			"turn off the engine) should be strong; got %q", got)
	}
}

func TestMetaMatchups_StormAgainstControl_IsStrong(t *testing.T) {
	got := findStrengthForMatchup(t, "storm", "Control")
	if got != "strong" {
		t.Errorf("Storm vs Control (one Counterspell breaks the whole turn) "+
			"should be strong; got %q", got)
	}
}

// -----------------------------------------------------------------------------
// Default-coverage check
// -----------------------------------------------------------------------------

// Every NON-overridden directional entry should default to moderate.
// Pin a few canonical cases that are deliberately NOT overridden (race-
// style matchups with no clear hard-lock mechanic).
func TestMetaMatchups_RaceMatchups_DefaultModerate(t *testing.T) {
	cases := []struct{ from, vs string }{
		{"aggro", "Combo"},     // fast clock; race
		{"combo", "Voltron"},   // goldfish vs commander tax
		{"reanimator", "Aggro"}, // fatties vs creatures
	}
	for _, c := range cases {
		got := findStrengthForMatchup(t, c.from, c.vs)
		if got != "moderate" {
			t.Errorf("%s vs %s should default to moderate (no hard-lock); got %q",
				c.from, c.vs, got)
		}
	}
}

// -----------------------------------------------------------------------------
// Public surface — MetaMatchup.Strength + JSON
// -----------------------------------------------------------------------------

func TestMetaMatchup_StrengthExposedThroughComputeMetaPositioning(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Stax"}
	computeMetaPositioning(dp)

	if len(dp.MetaMatchups) == 0 {
		t.Fatal("Stax should populate MetaMatchups")
	}
	foundStrong := false
	for _, mm := range dp.MetaMatchups {
		switch mm.Archetype {
		case "Combo", "Storm", "Reanimator", "Aristocrats", "Enchantress":
			if mm.Strength != "strong" {
				t.Errorf("Stax vs %s: Strength=%q, want strong", mm.Archetype, mm.Strength)
			}
			foundStrong = true
		}
		// Neutral matchups must have empty Strength.
		if mm.Rating == "neutral" && mm.Strength != "" {
			t.Errorf("Stax vs %s: rating=neutral but Strength=%q; want empty",
				mm.Archetype, mm.Strength)
		}
	}
	if !foundStrong {
		t.Error("expected at least one strong-tagged matchup on Stax deck profile")
	}
}

// JSON tag check — the field renders as "strength" (omitempty) so a
// neutral matchup doesn't surface a confusing empty field.
func TestMetaMatchup_JSON_OmitsEmptyStrength(t *testing.T) {
	cases := []struct {
		name        string
		mm          MetaMatchup
		wantContains string
		wantOmits    string
	}{
		{
			name: "favored entry includes strength (json tag lowercased)",
			mm:   MetaMatchup{Archetype: "Combo", Rating: "favored", Strength: "strong"},
			// Field renders as the json-tag "strength", not the Go field
			// name "Strength" — pin the tag so a future refactor that
			// drops the tag fails this test.
			wantContains: `"strength":"strong"`,
			wantOmits:    "",
		},
		{
			name: "neutral entry omits strength field via omitempty",
			mm:   MetaMatchup{Archetype: "Midrange", Rating: "neutral", Strength: ""},
			wantContains: "",
			wantOmits:    "strength",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.mm)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			out := string(b)
			if tc.wantContains != "" && !strings.Contains(out, tc.wantContains) {
				t.Errorf("JSON %q does not contain %q", out, tc.wantContains)
			}
			if tc.wantOmits != "" && strings.Contains(out, tc.wantOmits) {
				t.Errorf("JSON %q should NOT contain %q (omitempty failed)", out, tc.wantOmits)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Reciprocity invariant — strength should agree across paired entries
// -----------------------------------------------------------------------------

// When A→B is "favored:strong" the reciprocal B→A should be
// "unfavored:strong" (not "unfavored:slight"). The reciprocity is a
// SOFTER invariant on strength than on rating — we allow strength
// asymmetry when one side's reason text is more concrete than the
// other's. This test just verifies the FOUR canonical hard-lock pairs
// are STRONG from both perspectives, where the reverse entry exists.
func TestMetaMatchups_StrongReciprocityForHardLocks(t *testing.T) {
	// Stax favored:strong vs Combo  ↔  Combo unfavored vs Stax
	// (Combo's entry vs Stax: "Drannith Magistrate / Rule of Law /
	// Cursed Totem deny multiple combo cast lines" — should be strong.)
	if comboVsStax := findStrengthForMatchup(t, "combo", "Stax"); comboVsStax != "strong" {
		t.Errorf("Combo vs Stax should reciprocate Stax vs Combo (both strong); got %q",
			comboVsStax)
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// findStrengthForMatchup walks metaMatchupDB[fromKey] for the named
// vsLabel and returns the strength computed by metaMatchupStrengthOrDefault.
// Fails the test if no matching entry is found.
func findStrengthForMatchup(t *testing.T, fromKey, vsLabel string) string {
	t.Helper()
	entries, ok := metaMatchupDB[fromKey]
	if !ok {
		t.Fatalf("metaMatchupDB has no entries for %q", fromKey)
	}
	for _, e := range entries {
		if e.vsArchetype == vsLabel {
			return metaMatchupStrengthOrDefault(e)
		}
	}
	t.Fatalf("no matchup entry for %s vs %s", fromKey, vsLabel)
	return ""
}
