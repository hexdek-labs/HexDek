package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureSpellbookJSON is a hand-built sample of the Commander Spellbook
// variants payload shape. Includes:
//   - one entry that duplicates an existing curated combo (Exquisite Blood +
//     Sanguine Bond) — must be dropped by MergeKnownCombos
//   - one genuinely new combo (Heliod + Grindstone) — must import
//   - one variant in DRAFT status — must be skipped by the status filter
//   - one variant with a "Win the game" feature — type must infer to true_infinite
//   - one variant with only "Infinite mana" — type must infer to true_infinite
//   - one variant producing a plain feature with no "infinite"/"win" — determined
//   - one variant with <2 cards — must be warned and skipped
//   - one variant duplicating another inside the import — intra-file dedupe
const fixtureSpellbookJSON = `{
  "variants": [
    {
      "id": "spbk-blood-bond",
      "uses": [
        {"card": {"name": "Exquisite Blood"}, "quantity": 1},
        {"card": {"name": "Sanguine Bond"}, "quantity": 1}
      ],
      "produces": [{"feature": {"name": "Win the game"}}],
      "description": "Standard drain loop.",
      "identity": "WB",
      "status": "OK"
    },
    {
      "id": "spbk-painter-grindstone",
      "uses": [
        {"card": {"name": "Painter's Servant"}, "quantity": 1},
        {"card": {"name": "Grindstone"}, "quantity": 1}
      ],
      "produces": [
        {"feature": {"name": "Infinite damage"}},
        {"feature": {"name": "Win the game"}}
      ],
      "description": "Give Ballista lifelink, +1/+1 counter loop.",
      "identity": "W",
      "status": "OK"
    },
    {
      "id": "spbk-draft-skip",
      "uses": [
        {"card": {"name": "Some Card A"}, "quantity": 1},
        {"card": {"name": "Some Card B"}, "quantity": 1}
      ],
      "produces": [{"feature": {"name": "Win the game"}}],
      "description": "Not yet reviewed.",
      "status": "DRAFT"
    },
    {
      "id": "spbk-infinite-mana",
      "uses": [
        {"card": {"name": "Basalt Monolith"}, "quantity": 1},
        {"card": {"name": "Rings of Brighthearth"}, "quantity": 1}
      ],
      "produces": [{"feature": {"name": "Infinite colorless mana"}}],
      "description": "Untap loop for mana.",
      "identity": "C",
      "status": "OK"
    },
    {
      "id": "spbk-determined",
      "uses": [
        {"card": {"name": "Card X"}, "quantity": 1},
        {"card": {"name": "Card Y"}, "quantity": 1}
      ],
      "produces": [{"feature": {"name": "Card draw"}}],
      "description": "Optional card draw engine.",
      "status": "OK"
    },
    {
      "id": "spbk-too-small",
      "uses": [{"card": {"name": "Lone Card"}, "quantity": 1}],
      "produces": [{"feature": {"name": "Win the game"}}],
      "status": "OK"
    },
    {
      "id": "spbk-painter-grindstone-dup",
      "uses": [
        {"card": {"name": "Grindstone"}, "quantity": 1},
        {"card": {"name": "Painter's Servant"}, "quantity": 1}
      ],
      "produces": [{"feature": {"name": "Infinite damage"}}],
      "description": "Same combo, different prerequisite phrasing.",
      "status": "OK"
    }
  ]
}`

func TestParseSpellbookJSON_BasicShape(t *testing.T) {
	combos, warnings, err := ParseSpellbookJSON([]byte(fixtureSpellbookJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// 7 input variants:
	//   blood-bond, heliod-ballista, infinite-mana, determined → kept (4)
	//   draft-skip → skipped by status filter (no warning, just dropped)
	//   too-small → warned + skipped
	//   heliod-ballista-dup → intra-file dedupe drop
	if got, want := len(combos), 4; got != want {
		t.Errorf("combo count: got %d, want %d", got, want)
		for _, c := range combos {
			t.Logf("  parsed: %s (%v)", c.Name, c.Pieces)
		}
	}
	if len(warnings) < 1 {
		t.Errorf("expected at least 1 warning for too-small variant, got %d", len(warnings))
	}
	// Spot-check the too-small warning surfaces the ID so operators can find it.
	foundTooSmall := false
	for _, w := range warnings {
		if strings.Contains(w, "spbk-too-small") {
			foundTooSmall = true
		}
	}
	if !foundTooSmall {
		t.Errorf("expected warning mentioning spbk-too-small, got: %v", warnings)
	}
}

func TestParseSpellbookJSON_TypeInference(t *testing.T) {
	combos, _, err := ParseSpellbookJSON([]byte(fixtureSpellbookJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantType := map[string]string{
		"spbk-blood-bond":     "true_infinite",
		"spbk-painter-grindstone": "true_infinite",
		"spbk-infinite-mana":  "true_infinite",
		"spbk-determined":     "determined",
	}
	for _, c := range combos {
		for wantID, wantT := range wantType {
			if strings.Contains(c.Description, wantID) {
				if c.Type != wantT {
					t.Errorf("%s: type=%q, want %q", wantID, c.Type, wantT)
				}
			}
		}
	}
}

func TestParseSpellbookJSON_StatusFilter(t *testing.T) {
	combos, _, err := ParseSpellbookJSON([]byte(fixtureSpellbookJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, c := range combos {
		if strings.Contains(c.Description, "spbk-draft-skip") {
			t.Errorf("DRAFT-status variant leaked into import: %+v", c)
		}
	}
}

func TestCanonicalComboKey_OrderIndependent(t *testing.T) {
	k1 := canonicalComboKey([]string{"Painter's Servant", "Grindstone"})
	k2 := canonicalComboKey([]string{"Grindstone", "Painter's Servant"})
	k3 := canonicalComboKey([]string{"  grindstone  ", "PAINTER'S SERVANT"})
	if k1 != k2 {
		t.Errorf("ordering matters: %q vs %q", k1, k2)
	}
	if k1 != k3 {
		t.Errorf("normalization fails: %q vs %q", k1, k3)
	}
	if !strings.Contains(k1, "|") {
		t.Errorf("key %q missing pipe separator", k1)
	}
}

func TestMergeKnownCombos_CuratedWinsOnConflict(t *testing.T) {
	curated := []KnownCombo{
		{Name: "Curated A", Pieces: []string{"Card 1", "Card 2"}, Type: "true_infinite"},
		{Name: "Curated B", Pieces: []string{"Card 3", "Card 4"}, Type: "determined"},
	}
	imported := []KnownCombo{
		// Should be dropped — same pieces as Curated A (reversed order).
		{Name: "Spellbook A dup", Pieces: []string{"Card 2", "Card 1"}, Type: "determined"},
		// Should be kept — new pieces.
		{Name: "Spellbook C", Pieces: []string{"Card 5", "Card 6"}, Type: "true_infinite"},
		// Should be dropped — duplicates the just-added Spellbook C.
		{Name: "Spellbook C dup", Pieces: []string{"Card 6", "Card 5"}, Type: "determined"},
	}

	merged, dropped := MergeKnownCombos(curated, imported)

	if got, want := len(merged), 3; got != want {
		t.Errorf("merged length: got %d, want %d", got, want)
		for _, m := range merged {
			t.Logf("  merged: %s", m.Name)
		}
	}
	if got, want := len(dropped), 2; got != want {
		t.Errorf("dropped length: got %d, want %d", got, want)
	}

	// Curated A must remain (not the Spellbook duplicate).
	for _, m := range merged {
		if canonicalComboKey(m.Pieces) == canonicalComboKey([]string{"Card 1", "Card 2"}) {
			if m.Name != "Curated A" {
				t.Errorf("curated entry was overwritten by import: got %q", m.Name)
			}
		}
	}
}

func TestMergeKnownCombos_DedupesAgainstRealCurated(t *testing.T) {
	combos, _, err := ParseSpellbookJSON([]byte(fixtureSpellbookJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	merged, dropped := MergeKnownCombos(KnownCombos, combos)

	// Exquisite Blood + Sanguine Bond is in the curated set already
	// (known_combos.go), so the Spellbook entry must be dropped.
	wantDropKey := canonicalComboKey([]string{"Exquisite Blood", "Sanguine Bond"})
	foundDrop := false
	for _, d := range dropped {
		if canonicalComboKey(d.Pieces) == wantDropKey {
			foundDrop = true
		}
	}
	if !foundDrop {
		t.Errorf("expected Exquisite Blood + Sanguine Bond Spellbook entry to be dropped; dropped=%v", dropped)
	}

	// Merged list must contain exactly one entry for that combo (the curated one).
	count := 0
	for _, m := range merged {
		if canonicalComboKey(m.Pieces) == wantDropKey {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Exquisite Blood + Sanguine Bond appears %d times in merged; want 1", count)
	}

	// And Painter + Grindstone must NOT be in curated already (sanity check the
	// fixture is exercising the "new combo" import path).
	wantImportKey := canonicalComboKey([]string{"Painter's Servant", "Grindstone"})
	curatedHasHeliod := false
	for _, c := range KnownCombos {
		if canonicalComboKey(c.Pieces) == wantImportKey {
			curatedHasHeliod = true
		}
	}
	if curatedHasHeliod {
		t.Skip("curated set already contains Painter + Grindstone — fixture needs an alternate new combo")
	}
	mergedHasHeliod := false
	for _, m := range merged {
		if canonicalComboKey(m.Pieces) == wantImportKey {
			mergedHasHeliod = true
		}
	}
	if !mergedHasHeliod {
		t.Errorf("expected Painter + Grindstone to be imported into merged set")
	}
}

func TestLoadSpellbookCache_MissingFileReturnsNil(t *testing.T) {
	combos, warnings, err := LoadSpellbookCache("/nonexistent/spellbook-does-not-exist.json")
	if err != nil {
		t.Errorf("missing file should not error, got: %v", err)
	}
	if combos != nil {
		t.Errorf("missing file should return nil combos, got %d", len(combos))
	}
	if warnings != nil {
		t.Errorf("missing file should return nil warnings, got %d", len(warnings))
	}
}

func TestLoadSpellbookCache_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spellbook.json")
	if err := os.WriteFile(path, []byte(fixtureSpellbookJSON), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	combos, _, err := LoadSpellbookCache(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(combos) != 4 {
		t.Errorf("round-trip combo count: got %d, want 4", len(combos))
	}
}

func TestParseSpellbookJSON_BadJSONErrors(t *testing.T) {
	if _, _, err := ParseSpellbookJSON([]byte("not json")); err == nil {
		t.Error("expected error on garbage input")
	}
	if _, _, err := ParseSpellbookJSON([]byte(`{"variants": "wrong type"}`)); err == nil {
		t.Error("expected error on schema mismatch")
	}
}

func TestAllKnownCombos_FallsBackToCuratedWhenNoImport(t *testing.T) {
	saved := ImportedCombos
	ImportedCombos = nil
	defer func() { ImportedCombos = saved }()

	got := allKnownCombos()
	if len(got) != len(KnownCombos) {
		t.Errorf("expected curated-only length %d, got %d", len(KnownCombos), len(got))
	}
}

func TestAllKnownCombos_MergesImportedAndDedupes(t *testing.T) {
	saved := ImportedCombos
	defer func() { ImportedCombos = saved }()

	// Import one duplicate and one new combo.
	ImportedCombos = []KnownCombo{
		{Name: "spbk-dup", Pieces: []string{"Exquisite Blood", "Sanguine Bond"}, Type: "true_infinite"},
		{Name: "spbk-new", Pieces: []string{"Aaa New A", "Bbb New B"}, Type: "determined"},
	}
	got := allKnownCombos()
	if len(got) != len(KnownCombos)+1 {
		t.Errorf("merged length: got %d, want %d (curated + 1 new)", len(got), len(KnownCombos)+1)
	}
	// Verify the new combo is in there and the dup didn't leak.
	wantDupKey := canonicalComboKey([]string{"Exquisite Blood", "Sanguine Bond"})
	dupCount := 0
	for _, c := range got {
		if canonicalComboKey(c.Pieces) == wantDupKey {
			dupCount++
		}
	}
	if dupCount != 1 {
		t.Errorf("dup appeared %d times, want 1", dupCount)
	}
}
