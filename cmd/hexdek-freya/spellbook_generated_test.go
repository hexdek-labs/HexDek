package main

import (
	"strings"
	"testing"
)

// TestGeneratedSpellbookCombos_NonEmpty defends against an accidental
// empty regeneration. The generator's guard would have refused the
// initial run, but a future operator running with a stripped seed
// JSON should still get caught at test time.
func TestGeneratedSpellbookCombos_NonEmpty(t *testing.T) {
	if len(generatedSpellbookCombos) == 0 {
		t.Fatal("generatedSpellbookCombos is empty — regenerate via go run ./cmd/hexdek-freya/gen_spellbook")
	}
	// Sanity: every generated entry must carry at least 2 pieces and a
	// non-empty name. Spellbook variants with <2 cards are filtered at
	// parse time; if any leak through, the import pipeline is broken.
	for _, c := range generatedSpellbookCombos {
		if c.Name == "" {
			t.Errorf("generated combo with empty Name: %#v", c)
		}
		if len(c.Pieces) < 2 {
			t.Errorf("combo %q has %d pieces, need >=2", c.Name, len(c.Pieces))
		}
	}
}

// TestGeneratedSpellbookCombos_KnownCombosRoundTrip pins five specific
// real Commander Spellbook combos that must survive the JSON → parse →
// dedupe → generate → load round-trip. These are the contract for
// "the import pipeline actually works on real data" — drop any one of
// them and either the seed regressed or the generator broke.
func TestGeneratedSpellbookCombos_KnownCombosRoundTrip(t *testing.T) {
	type expectation struct {
		pieces      []string
		typeStr     string
		descContain string
	}
	want := map[string]expectation{
		"Devoted Druid + Vizier of Remedies": {
			pieces:      []string{"Devoted Druid", "Vizier of Remedies"},
			typeStr:     "true_infinite",
			descContain: "Infinite green mana",
		},
		"Helm of Obedience + Rest in Peace": {
			pieces:      []string{"Helm of Obedience", "Rest in Peace"},
			typeStr:     "true_infinite",
			descContain: "Win the game",
		},
		"Niv-Mizzet, Parun + Curiosity": {
			pieces:      []string{"Niv-Mizzet, Parun", "Curiosity"},
			typeStr:     "true_infinite",
			descContain: "Win the game",
		},
		"Pili-Pala + Grand Architect": {
			pieces:      []string{"Pili-Pala", "Grand Architect"},
			typeStr:     "true_infinite",
			descContain: "Infinite colored mana",
		},
		"Power Artifact + Basalt Monolith": {
			pieces:      []string{"Power Artifact", "Basalt Monolith"},
			typeStr:     "true_infinite",
			descContain: "Infinite colorless mana",
		},
	}

	byName := map[string]KnownCombo{}
	for _, c := range generatedSpellbookCombos {
		byName[c.Name] = c
	}

	for name, exp := range want {
		got, ok := byName[name]
		if !ok {
			t.Errorf("expected generated combo %q missing — regenerate seed or check gen_spellbook output", name)
			continue
		}
		if !sameNameSet(got.Pieces, exp.pieces) {
			t.Errorf("%s: pieces mismatch — got %v, want %v", name, got.Pieces, exp.pieces)
		}
		if got.Type != exp.typeStr {
			t.Errorf("%s: Type=%q want %q", name, got.Type, exp.typeStr)
		}
		if !got.Mandatory {
			t.Errorf("%s: Mandatory=false — Spellbook variants always import as mandatory", name)
		}
		if !strings.Contains(got.Description, exp.descContain) {
			t.Errorf("%s: description missing %q — got %q", name, exp.descContain, got.Description)
		}
	}
}

// TestGeneratedSpellbookCombos_CuratedDedup verifies that combos already
// present in the hand-curated KnownCombos slice are dropped at
// allKnownCombos() merge time even when the generator emitted them.
// The seed deliberately includes Mikaeus+Triskelion and Heliod+Ballista
// (both curated) so we can observe the merge behavior end-to-end.
func TestGeneratedSpellbookCombos_CuratedDedup(t *testing.T) {
	// Pre-condition: the generator emitted the duplicates (proves the
	// curated set isn't filtered at gen time — that's a runtime concern).
	if !containsByKey(generatedSpellbookCombos, []string{"Mikaeus, the Unhallowed", "Triskelion"}) {
		t.Fatal("seed regression: Mikaeus+Triskelion should be present in generated set to exercise the runtime dedup")
	}
	if !containsByKey(generatedSpellbookCombos, []string{"Walking Ballista", "Heliod, Sun-Crowned"}) {
		t.Fatal("seed regression: Heliod+Ballista should be present in generated set to exercise the runtime dedup")
	}

	// Snapshot the global to isolate from any test that mutated it.
	saved := ImportedCombos
	t.Cleanup(func() { ImportedCombos = saved })
	ImportedCombos = nil

	merged := allKnownCombos()

	// Post-condition: each curated card-set must appear EXACTLY ONCE.
	mikaeus := countByKey(merged, []string{"Mikaeus, the Unhallowed", "Triskelion"})
	if mikaeus != 1 {
		t.Errorf("Mikaeus+Triskelion appears %d times in merged set, want 1 (curated entry must win)", mikaeus)
	}
	heliod := countByKey(merged, []string{"Walking Ballista", "Heliod, Sun-Crowned"})
	if heliod != 1 {
		t.Errorf("Heliod+Ballista appears %d times in merged set, want 1 (curated entry must win)", heliod)
	}

	// Curated entries must be the ones that survived (they carry the
	// hand-authored Outlets/Stops that imports lack).
	for _, c := range merged {
		if sameNameSet(c.Pieces, []string{"Mikaeus, the Unhallowed", "Triskelion"}) {
			if len(c.Stops) == 0 {
				t.Errorf("merged Mikaeus+Triskelion lost curated Stops — import overwrote curated")
			}
		}
	}
}

// TestGeneratedSpellbookCombos_AllKnownCombosIncludes verifies that
// generated entries (those NOT colliding with curated) actually appear
// in allKnownCombos() output. Without this, the generator could ship
// data that's silently invisible to downstream Freya consumers.
func TestGeneratedSpellbookCombos_AllKnownCombosIncludes(t *testing.T) {
	saved := ImportedCombos
	t.Cleanup(func() { ImportedCombos = saved })
	ImportedCombos = nil

	merged := allKnownCombos()
	mergedKeys := map[string]bool{}
	for _, c := range merged {
		mergedKeys[canonicalComboKey(c.Pieces)] = true
	}

	checks := [][]string{
		{"Devoted Druid", "Vizier of Remedies"},
		{"Helm of Obedience", "Rest in Peace"},
		{"Niv-Mizzet, Parun", "Curiosity"},
		{"Pili-Pala", "Grand Architect"},
		{"Power Artifact", "Basalt Monolith"},
	}
	for _, pieces := range checks {
		if !mergedKeys[canonicalComboKey(pieces)] {
			t.Errorf("expected generated combo %v missing from allKnownCombos()", pieces)
		}
	}
}

// TestGeneratedSpellbookCombos_ClassBackfilled verifies the runtime
// classifier fires on generated entries — the generator leaves Class
// empty (it doesn't have access to ClassifyComboHeuristic), so
// allKnownCombos() must run ClassifySpellbookImported on the
// generated slice. Drain-class combos in the seed should land as
// infinite_drain after merge.
func TestGeneratedSpellbookCombos_ClassBackfilled(t *testing.T) {
	saved := ImportedCombos
	t.Cleanup(func() { ImportedCombos = saved })
	ImportedCombos = nil

	_ = allKnownCombos() // mutates generatedSpellbookCombos in place

	// Pili-Pala + Grand Architect produces infinite mana — should land
	// on the mana class after classification.
	for _, c := range generatedSpellbookCombos {
		if !sameNameSet(c.Pieces, []string{"Pili-Pala", "Grand Architect"}) {
			continue
		}
		if c.Class == "" {
			t.Errorf("Pili-Pala + Grand Architect: Class=\"\" after allKnownCombos() — classifier never ran")
		}
		return
	}
	t.Fatal("Pili-Pala + Grand Architect not found in generated set")
}

// --- helpers ---

func sameNameSet(a, b []string) bool {
	return canonicalComboKey(a) == canonicalComboKey(b)
}

func containsByKey(combos []KnownCombo, pieces []string) bool {
	key := canonicalComboKey(pieces)
	for _, c := range combos {
		if canonicalComboKey(c.Pieces) == key {
			return true
		}
	}
	return false
}

func countByKey(combos []KnownCombo, pieces []string) int {
	key := canonicalComboKey(pieces)
	n := 0
	for _, c := range combos {
		if canonicalComboKey(c.Pieces) == key {
			n++
		}
	}
	return n
}
