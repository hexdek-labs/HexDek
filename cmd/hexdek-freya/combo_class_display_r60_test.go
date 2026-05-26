package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Pins the R60r2 combo-class surfacing work: human-readable labels,
// new Lockdown class, JSON field, text + markdown report tags, and
// the heuristic auto-classify backfill for non-curated combos.

// TestComboClassLabel_AllClassesMapped verifies every taxonomy constant
// (except Unknown/empty) has a human-readable label. Guards against
// adding a new ComboClass* constant without wiring its label, which
// would silently render blank in the report tag.
func TestComboClassLabel_AllClassesMapped(t *testing.T) {
	all := []string{
		ComboClassInfiniteMana,
		ComboClassInfiniteDamage,
		ComboClassInfiniteDrain,
		ComboClassInfiniteTokens,
		ComboClassInfiniteETB,
		ComboClassInfiniteMill,
		ComboClassLibraryExileWin,
		ComboClassStormFinisher,
		ComboClassETBPayoff,
		ComboClassETBDoubler,
		ComboClassBlinkEngine,
		ComboClassManaSink,
		ComboClassCombatFinisher,
		ComboClassLockdown,
	}
	for _, c := range all {
		if got := ComboClassLabel(c); got == "" {
			t.Errorf("ComboClassLabel(%q) is empty — every taxonomy entry needs a human label", c)
		}
	}
	// Unknown + empty must NOT produce a label so renderers can skip the tag.
	if got := ComboClassLabel(ComboClassUnknown); got != "" {
		t.Errorf("Unknown should produce empty label, got %q", got)
	}
	if got := ComboClassLabel(""); got != "" {
		t.Errorf("empty class should produce empty label, got %q", got)
	}
}

// TestComboClassLabel_PinnedDisplayNames verifies a few canonical
// labels so accidental "InfiniteDrain → Infinite drain" recasing is
// caught. The visible display names appear in deck reports humans read.
func TestComboClassLabel_PinnedDisplayNames(t *testing.T) {
	cases := map[string]string{
		ComboClassStormFinisher:   "Storm Finisher",
		ComboClassInfiniteMana:    "Infinite Mana",
		ComboClassInfiniteDrain:   "Infinite Drain",
		ComboClassInfiniteMill:    "Infinite Mill",
		ComboClassLockdown:        "Lockdown",
		ComboClassLibraryExileWin: "Library-Exile Win",
		ComboClassCombatFinisher:  "Combat Finisher",
	}
	for class, want := range cases {
		if got := ComboClassLabel(class); got != want {
			t.Errorf("ComboClassLabel(%q): want %q got %q", class, want, got)
		}
	}
}

// TestClassifyComboHeuristic_LockdownClass exercises the new lockdown
// classifier. Both phrase-based detection ("locks the game") and the
// curated card-name combos (Helm + RIP, Stasis + Solemnity) must hit.
func TestClassifyComboHeuristic_LockdownClass(t *testing.T) {
	cases := []struct {
		name string
		desc string
		want string
	}{
		{
			"Helm + RIP",
			"Helm of Obedience activated for X=1 with Rest in Peace exiles target opponent's library.",
			ComboClassLockdown,
		},
		{
			"Stasis lock",
			"Stasis + Solemnity locks opponents out of untap permanently.",
			ComboClassLockdown,
		},
		{
			"phrase-only lock",
			"This combo locks the game — opponents can't untap.",
			ComboClassLockdown,
		},
		{
			"Isochron + Dramatic",
			"Isochron Scepter + Dramatic Reversal generates infinite mana.",
			ComboClassInfiniteMana, // should NOT misclassify as lockdown
		},
	}
	for _, tc := range cases {
		got := ClassifyComboHeuristic(tc.name, tc.desc, nil)
		if got != tc.want {
			t.Errorf("ClassifyComboHeuristic(%q): want %q got %q", tc.name, tc.want, got)
		}
	}
}

// TestClassifyComboBucket_BackfillsEmpty verifies the backfill helper
// populates Class on heuristic combos (curated combos arrive with Class
// pre-set and must be left untouched).
func TestClassifyComboBucket_BackfillsEmpty(t *testing.T) {
	combos := []ComboResult{
		{
			Cards:       []string{"Thassa's Oracle", "Demonic Consultation"},
			Description: "Exile your library with Consultation, Oracle ETB wins with empty library.",
		},
		{
			Cards:       []string{"Walking Ballista", "Heliod, Sun-Crowned"},
			Description: "Lifelink Ballista pings, gains life, Heliod adds counter. Infinite damage.",
		},
		{
			// Pre-classed entry — must NOT be overwritten.
			Cards:       []string{"X"},
			Description: "anything",
			Class:       ComboClassInfiniteDrain,
		},
	}
	classifyComboBucket(combos)
	if combos[0].Class != ComboClassLibraryExileWin {
		t.Errorf("Thoracle/Consultation: want %q got %q",
			ComboClassLibraryExileWin, combos[0].Class)
	}
	if combos[1].Class != ComboClassInfiniteDamage {
		t.Errorf("Ballista/Heliod: want %q got %q",
			ComboClassInfiniteDamage, combos[1].Class)
	}
	if combos[2].Class != ComboClassInfiniteDrain {
		t.Errorf("pre-classed entry should be preserved, got %q", combos[2].Class)
	}
}

// TestComboSlice_IncludesClass: the JSON conversion must carry Class
// through to jsonCombo so downstream consumers (Heimdall, hat-strategy
// bridge, deck-detail UI) see the taxonomy.
func TestComboSlice_IncludesClass(t *testing.T) {
	combos := []ComboResult{
		{
			Cards:       []string{"Sanguine Bond", "Exquisite Blood"},
			LoopType:    "true_infinite",
			Class:       ComboClassInfiniteDrain,
			Description: "Infinite drain loop.",
			Confirmed:   true,
		},
	}
	out := comboSlice(combos)
	if len(out) != 1 {
		t.Fatalf("comboSlice: want 1 entry, got %d", len(out))
	}
	if out[0].Class != ComboClassInfiniteDrain {
		t.Errorf("jsonCombo.Class: want %q got %q",
			ComboClassInfiniteDrain, out[0].Class)
	}
}

// TestPrintText_SurfacesClassLabel asserts the text-format report
// emits a "[Storm Finisher]" / "[Infinite Drain]" tag next to each
// classed combo. The tag is what a human reader uses to triage —
// "which lines are wincons vs which are value engines."
func TestPrintText_SurfacesClassLabel(t *testing.T) {
	r := &FreyaReport{
		TrueInfinites: []ComboResult{{
			Cards:       []string{"Sanguine Bond", "Exquisite Blood"},
			LoopType:    "true_infinite",
			Class:       ComboClassInfiniteDrain,
			Description: "Infinite drain loop.",
			Confirmed:   true,
		}},
	}
	var buf bytes.Buffer
	PrintReport(&buf, r, "text")
	out := buf.String()
	if !strings.Contains(out, "[Infinite Drain]") {
		t.Errorf("text report missing [Infinite Drain] tag\n---\n%s", out)
	}
}

// TestPrintMarkdown_SurfacesClassLabel: same as the text variant,
// but pins the markdown italicized form _[Infinite Drain]_.
func TestPrintMarkdown_SurfacesClassLabel(t *testing.T) {
	r := &FreyaReport{
		TrueInfinites: []ComboResult{{
			Cards:       []string{"Sanguine Bond", "Exquisite Blood"},
			LoopType:    "true_infinite",
			Class:       ComboClassInfiniteDrain,
			Description: "Infinite drain loop.",
			Confirmed:   true,
		}},
	}
	var buf bytes.Buffer
	PrintReport(&buf, r, "markdown")
	out := buf.String()
	if !strings.Contains(out, "_[Infinite Drain]_") {
		t.Errorf("markdown report missing _[Infinite Drain]_ tag\n---\n%s", out)
	}
}

// TestPrintJSON_SerializesClass: the JSON output must contain
// "class":"infinite_drain" — proves the field carries through end
// to end. Uses the public PrintReport entry point so renderer
// integration is verified, not just the slice converter.
func TestPrintJSON_SerializesClass(t *testing.T) {
	r := &FreyaReport{
		TrueInfinites: []ComboResult{{
			Cards:       []string{"Sanguine Bond", "Exquisite Blood"},
			LoopType:    "true_infinite",
			Class:       ComboClassInfiniteDrain,
			Description: "Infinite drain loop.",
			Confirmed:   true,
		}},
	}
	var buf bytes.Buffer
	PrintReport(&buf, r, "json")
	out := buf.String()
	// Pretty-printed output uses `"class": "infinite_drain"` with a
	// space; check both compact + pretty forms.
	if !strings.Contains(out, `"class":"infinite_drain"`) &&
		!strings.Contains(out, `"class": "infinite_drain"`) {
		t.Errorf("JSON report missing class=infinite_drain\n---\n%s", out)
	}
	// Defensively re-parse the output: catches any future renderer
	// that emits malformed JSON when the class field is non-empty.
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Errorf("JSON output is not valid JSON: %v", err)
	}
}

// TestPrintText_NoClassLabelForUnknown: when Class is empty/unknown,
// the renderer must NOT emit an empty [ ] tag — verifies the
// "if label != \"\" { emit }" guard works.
func TestPrintText_NoClassLabelForUnknown(t *testing.T) {
	r := &FreyaReport{
		TrueInfinites: []ComboResult{{
			Cards:       []string{"Random A", "Random B"},
			LoopType:    "true_infinite",
			Class:       "", // empty
			Description: "Unknown combo.",
		}},
	}
	var buf bytes.Buffer
	PrintReport(&buf, r, "text")
	out := buf.String()
	if strings.Contains(out, "[]") || strings.Contains(out, "[ ]") {
		t.Errorf("empty class should produce no tag, got bracketed tag in:\n%s", out)
	}
}
