package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spellbook_integration_r60_test.go — end-to-end wiring regressions.
//
// PR #546 shipped the Spellbook JSON import path (ParseSpellbookJSON,
// MergeKnownCombos, LoadSpellbookCache, FetchSpellbook) but only covered
// the parse/dedupe surface. These tests close the loop: a real Spellbook
// JSON sample is loaded, ImportedCombos is populated, a synthetic deck
// containing the combo pieces is run through AnalyzeDeck, and we assert
// the combo actually lands on the FreyaReport.
//
// Two paths are covered:
//   (1) curated-wins dedupe — Thassa's Oracle + Demonic Consultation is
//       already in KnownCombos; loading the Spellbook variant should NOT
//       create a second detection, and the curated entry remains source
//       of truth (carries hand-authored outlets/stops).
//   (2) Spellbook-only import — Painter's Servant + Grindstone is NOT in
//       curated; detection must come from the import path. This is the
//       canary for the wiring at analysis.go:1404 (allKnownCombos()).

// loadIntegrationSample parses the integration fixture and returns the
// imported combos. Fixture lives at testdata/spellbook_integration_sample.json.
func loadIntegrationSample(t *testing.T) []KnownCombo {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "spellbook_integration_sample.json"))
	if err != nil {
		t.Fatalf("read integration fixture: %v", err)
	}
	combos, _, err := ParseSpellbookJSON(data)
	if err != nil {
		t.Fatalf("parse integration fixture: %v", err)
	}
	if len(combos) < 2 {
		t.Fatalf("fixture should produce >=2 combos, got %d", len(combos))
	}
	return combos
}

// withImportedCombos swaps in the given imported combos for the duration
// of the test, restoring the original slice on cleanup. Tests run in
// parallel by default in Go's testing harness only when t.Parallel() is
// called explicitly; integration tests here are kept serial precisely
// because they mutate ImportedCombos (a package-level global).
func withImportedCombos(t *testing.T, imported []KnownCombo) {
	t.Helper()
	saved := ImportedCombos
	t.Cleanup(func() { ImportedCombos = saved })
	ImportedCombos = imported
}

// TestSpellbookIntegration_CuratedComboStillDetectsAfterImport verifies
// that loading a Spellbook variant which duplicates a curated entry does
// not regress detection. Thoracle + Demonic Consultation is curated; the
// imported version should be deduped, and a deck containing both pieces
// should still produce exactly ONE combo on the report.
func TestSpellbookIntegration_CuratedComboStillDetectsAfterImport(t *testing.T) {
	imported := loadIntegrationSample(t)
	withImportedCombos(t, imported)

	profiles := []CardProfile{
		makeProfile("Thassa's Oracle"),
		makeProfile("Demonic Consultation"),
	}
	report := AnalyzeDeck(profiles, "test-thoracle", "test.txt", "")

	want := []string{"Thassa's Oracle", "Demonic Consultation"}
	if !findComboByPieces(report, want) {
		t.Fatalf("Thoracle + Consultation not detected; "+
			"TrueInfinites=%d Determined=%d Synergies=%d",
			len(report.TrueInfinites), len(report.Determined), len(report.Synergies))
	}

	// Curated entry wins on dedup — should appear exactly once across all
	// buckets. Count matches by canonical key so we catch both spellings.
	wantKey := canonicalComboKey(want)
	matches := 0
	for _, bucket := range [][]ComboResult{report.TrueInfinites, report.Determined, report.Synergies} {
		for _, c := range bucket {
			if canonicalComboKey(c.Cards) == wantKey {
				matches++
			}
		}
	}
	if matches != 1 {
		t.Errorf("Thoracle + Consultation appears %d times across buckets; want exactly 1 (curated wins dedup)", matches)
	}
}

// TestSpellbookIntegration_NewSpellbookComboDetectedEndToEnd is the core
// wiring regression: Painter's Servant + Grindstone is NOT in curated
// (verified at parse time below), so detection comes entirely through
// the import path. If allKnownCombos() ever stops merging ImportedCombos
// into the lookup, or if analysis.go ever stops calling allKnownCombos(),
// this test fails — proving the wiring at analysis.go:1404 is load-bearing.
func TestSpellbookIntegration_NewSpellbookComboDetectedEndToEnd(t *testing.T) {
	imported := loadIntegrationSample(t)

	// Sanity check the fixture: Painter + Grindstone must NOT be in
	// curated, otherwise this test asserts curated detection instead of
	// import detection.
	painterKey := canonicalComboKey([]string{"Painter's Servant", "Grindstone"})
	for _, c := range KnownCombos {
		if canonicalComboKey(c.Pieces) == painterKey {
			t.Skip("curated set already contains Painter + Grindstone — fixture needs a different new combo")
		}
	}

	withImportedCombos(t, imported)

	// Sanity: the merged set must include the imported combo. If this
	// fails, the failure points at allKnownCombos() rather than the
	// detection loop.
	mergedHas := false
	for _, c := range allKnownCombos() {
		if canonicalComboKey(c.Pieces) == painterKey {
			mergedHas = true
			break
		}
	}
	if !mergedHas {
		t.Fatalf("allKnownCombos() did not include the imported Painter + Grindstone combo")
	}

	profiles := []CardProfile{
		makeProfile("Painter's Servant"),
		makeProfile("Grindstone"),
		// Pad with a third unrelated card so the deck profile isn't
		// degenerate — AnalyzeDeck's downstream phases (archetype,
		// roles, etc.) tolerate small decks but a single combo pair is
		// the absolute floor.
		makeProfile("Sol Ring"),
	}
	report := AnalyzeDeck(profiles, "test-painter", "test.txt", "")

	want := []string{"Painter's Servant", "Grindstone"}
	if !findComboByPieces(report, want) {
		t.Fatalf("imported Spellbook combo Painter + Grindstone not detected end-to-end; "+
			"TrueInfinites=%d Determined=%d Synergies=%d",
			len(report.TrueInfinites), len(report.Determined), len(report.Synergies))
	}

	// Type inference: Win-the-game produced → true_infinite bucket.
	foundInTrueInf := false
	for _, c := range report.TrueInfinites {
		if canonicalComboKey(c.Cards) == painterKey {
			foundInTrueInf = true
			if !c.Confirmed {
				t.Errorf("imported combo should be marked Confirmed=true (matched from KnownCombos database)")
			}
			break
		}
	}
	if !foundInTrueInf {
		t.Errorf("Painter + Grindstone should land in TrueInfinites (Spellbook 'Win the game' feature)")
	}
}

// TestSpellbookIntegration_CaseInsensitiveDeckMatch verifies the
// lookup-side case-insensitivity at analysis.go's deckCardNames map.
// Even if the user's decklist parser handed up a card name with
// non-canonical casing (lowercased, all-caps), the combo should still
// detect against the canonically-cased Spellbook piece name.
func TestSpellbookIntegration_CaseInsensitiveDeckMatch(t *testing.T) {
	imported := loadIntegrationSample(t)
	withImportedCombos(t, imported)

	profiles := []CardProfile{
		makeProfile("THASSA'S ORACLE"),
		makeProfile("demonic consultation"),
	}
	report := AnalyzeDeck(profiles, "test-case", "test.txt", "")

	// Detection uses canonical Spellbook/curated spelling on the report
	// even though the deck profile used different casing. findComboByPieces
	// scans every bucket (curated Thoracle+Consultation is classed as
	// "determined", not "true_infinite" — see known_combos.go:210).
	want := []string{"Thassa's Oracle", "Demonic Consultation"}
	if !findComboByPieces(report, want) {
		t.Errorf("case-insensitive deck lookup failed: combo not detected when deck profiles use non-canonical casing")
	}
}

// TestSpellbookIntegration_LoadSpellbookCacheThenAnalyze is the full
// loader → analyze loop: write the fixture to disk, LoadSpellbookCache
// reads it (mirroring main.go's startup path), populate ImportedCombos,
// then run AnalyzeDeck. Catches regressions in the file-I/O path that
// the in-memory tests above would miss.
func TestSpellbookIntegration_LoadSpellbookCacheThenAnalyze(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "spellbook_integration_sample.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cachePath := filepath.Join(dir, "spellbook.json")
	if err := os.WriteFile(cachePath, src, 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	combos, warnings, err := LoadSpellbookCache(cachePath)
	if err != nil {
		t.Fatalf("load cache: %v", err)
	}
	if len(combos) == 0 {
		t.Fatalf("empty combo load (warnings=%v)", warnings)
	}
	withImportedCombos(t, combos)

	profiles := []CardProfile{
		makeProfile("Painter's Servant"),
		makeProfile("Grindstone"),
		makeProfile("Sol Ring"),
	}
	report := AnalyzeDeck(profiles, "test-loader", "test.txt", "")

	want := []string{"Painter's Servant", "Grindstone"}
	if !findComboByPieces(report, want) {
		// Surface the actual report state in the failure message so
		// future debuggers can see whether the combo lookup or the
		// loader regressed.
		var have []string
		for _, c := range report.TrueInfinites {
			have = append(have, strings.Join(c.Cards, "+"))
		}
		t.Errorf("LoadSpellbookCache → AnalyzeDeck path failed to detect Painter + Grindstone; TrueInfinites had: %v", have)
	}
}
