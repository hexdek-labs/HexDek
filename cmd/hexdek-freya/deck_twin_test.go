package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkfp is a test-helper constructor for DeckFingerprint — fewer
// fields than the production BuildDeckFingerprint so the synthetic
// tests stay focused on the twin-suggestion logic, not the
// fingerprint extraction.
func mkfp(name, commander string, nonland []string, isPrecon bool) *DeckFingerprint {
	cards := make(map[string]bool, len(nonland))
	for _, c := range nonland {
		cards[strings.ToLower(strings.TrimSpace(c))] = true
	}
	return &DeckFingerprint{
		Name:         name,
		Commander:    commander,
		NonlandCards: cards,
		IsPrecon:     isPrecon,
	}
}

// TestBuildDeckTwinSuggestion_NoTwinForNovelCommander verifies
// the "no precon ships this commander" path — Narrative carries a
// "no stock-precon ancestor" message rather than crashing.
func TestBuildDeckTwinSuggestion_NoTwinForNovelCommander(t *testing.T) {
	target := mkfp("user_deck", "Custom Homebrew Commander",
		[]string{"Sol Ring", "Counterspell"}, false)
	precons := []*DeckFingerprint{
		mkfp("blessed_sanctuary_precon_decklist", "Sigarda, Font of Blessings",
			[]string{"Sol Ring", "Path to Exile"}, true),
	}
	got := BuildDeckTwinSuggestion(target, precons)
	if got == nil {
		t.Fatal("nil result")
	}
	if got.HasTwin {
		t.Errorf("HasTwin: want false (no matching commander), got true")
	}
	if !strings.Contains(got.Narrative, "no stock-precon") {
		t.Errorf("narrative missing 'no stock-precon' callout: %q", got.Narrative)
	}
	if !strings.Contains(got.Narrative, "Custom Homebrew Commander") {
		t.Errorf("narrative missing commander name: %q", got.Narrative)
	}
}

// TestBuildDeckTwinSuggestion_IdenticalDeckReturnsNoUpgrades
// verifies that a fingerprint identical to a precon's nonlands
// reports 100% similarity + 0 added / 0 removed + the "identical"
// narrative line.
func TestBuildDeckTwinSuggestion_IdenticalDeckReturnsNoUpgrades(t *testing.T) {
	cards := []string{"Sol Ring", "Counterspell", "Swords to Plowshares", "Rhystic Study"}
	target := mkfp("stock_clone", "Sigarda, Font of Blessings", cards, false)
	precon := mkfp("blessed_sanctuary_innistrad_crimson_vow_commander_precon_decklist",
		"Sigarda, Font of Blessings", cards, true)
	got := BuildDeckTwinSuggestion(target, []*DeckFingerprint{precon})
	if !got.HasTwin {
		t.Fatal("want HasTwin=true")
	}
	if got.AddedCount != 0 {
		t.Errorf("AddedCount: want 0, got %d (Added=%v)", got.AddedCount, got.Added)
	}
	if got.RemovedCount != 0 {
		t.Errorf("RemovedCount: want 0, got %d (Removed=%v)", got.RemovedCount, got.Removed)
	}
	if got.SimilarityPercent != 100.0 {
		t.Errorf("SimilarityPercent: want 100, got %.1f", got.SimilarityPercent)
	}
	if got.DeltaPercent != 0.0 {
		t.Errorf("DeltaPercent: want 0, got %.1f", got.DeltaPercent)
	}
	if !strings.Contains(got.Narrative, "identical") {
		t.Errorf("narrative missing 'identical' wording: %q", got.Narrative)
	}
	if !strings.Contains(got.Narrative, "100%") {
		t.Errorf("narrative missing '100%%': %q", got.Narrative)
	}
}

// TestBuildDeckTwinSuggestion_PartialOverlapSurfacesDelta verifies
// the canonical happy-path scenario from the user's spec: a deck
// that's mostly the precon with a handful of upgrades produces a
// percentage + delta lists in the narrative.
func TestBuildDeckTwinSuggestion_PartialOverlapSurfacesDelta(t *testing.T) {
	precon := mkfp("blessed_sanctuary_innistrad_crimson_vow_commander_precon_decklist",
		"Sigarda, Font of Blessings",
		[]string{
			// 8 stock-precon nonlands
			"Sol Ring", "Open the Armory", "Boon Reflection",
			"Heliod's Pilgrim", "Welcoming Vampire", "Beast Within",
			"Path to Exile", "Sigarda's Splendor",
		}, true)
	// User's deck shares 5 of the precon's 8 + adds 3 new cards.
	// 5 shared out of (5 shared + 3 added) = 8 nonlands in user deck;
	// shared / target = 5/8 = 62.5% similarity, 37.5% delta.
	target := mkfp("alice_sigarda_upgrade", "Sigarda, Font of Blessings",
		[]string{
			// Shared with precon
			"Sol Ring", "Open the Armory", "Heliod's Pilgrim",
			"Beast Within", "Path to Exile",
			// User-added upgrades
			"Smothering Tithe", "Land Tax", "Heliod's Intervention",
		}, false)
	got := BuildDeckTwinSuggestion(target, []*DeckFingerprint{precon})
	if !got.HasTwin {
		t.Fatal("want HasTwin=true")
	}
	if got.SharedCount != 5 {
		t.Errorf("SharedCount: want 5, got %d", got.SharedCount)
	}
	if got.AddedCount != 3 {
		t.Errorf("AddedCount: want 3, got %d (Added=%v)", got.AddedCount, got.Added)
	}
	if got.RemovedCount != 3 {
		t.Errorf("RemovedCount: want 3, got %d (Removed=%v)", got.RemovedCount, got.Removed)
	}
	if got.SimilarityPercent < 62.0 || got.SimilarityPercent > 63.0 {
		t.Errorf("SimilarityPercent: want ~62.5, got %.2f", got.SimilarityPercent)
	}
	if got.DeltaPercent < 37.0 || got.DeltaPercent > 38.0 {
		t.Errorf("DeltaPercent: want ~37.5, got %.2f", got.DeltaPercent)
	}

	// Added list should include the upgrades (lowercase keys).
	addedSet := map[string]bool{}
	for _, c := range got.Added {
		addedSet[c] = true
	}
	for _, want := range []string{"smothering tithe", "land tax", "heliod's intervention"} {
		if !addedSet[want] {
			t.Errorf("Added missing %q: got %v", want, got.Added)
		}
	}
	// Removed should include precon-only cards.
	removedSet := map[string]bool{}
	for _, c := range got.Removed {
		removedSet[c] = true
	}
	for _, want := range []string{"boon reflection", "welcoming vampire", "sigarda's splendor"} {
		if !removedSet[want] {
			t.Errorf("Removed missing %q: got %v", want, got.Removed)
		}
	}

	// Narrative format check — should contain the percentage and
	// at least one card name from each side (title-cased).
	if !strings.Contains(got.Narrative, "63%") && !strings.Contains(got.Narrative, "62%") {
		t.Errorf("narrative missing similarity percent: %q", got.Narrative)
	}
	if !strings.Contains(got.Narrative, "Smothering Tithe") && !strings.Contains(got.Narrative, "Land Tax") {
		t.Errorf("narrative missing added card sample: %q", got.Narrative)
	}
	if !strings.Contains(got.Narrative, "added") || !strings.Contains(got.Narrative, "removed") {
		t.Errorf("narrative missing add/remove framing: %q", got.Narrative)
	}
}

// TestBuildDeckTwinSuggestion_NarrativeCapsCardLists verifies the
// 5-card cap on inline card-name samples — narrative stays readable
// even on a heavy-delta deck.
func TestBuildDeckTwinSuggestion_NarrativeCapsCardLists(t *testing.T) {
	precon := mkfp("foo_precon_decklist", "Test Commander",
		[]string{"Card_P1"}, true)
	added := []string{}
	for i := 0; i < 12; i++ {
		added = append(added, "Added_Card_"+string(rune('A'+i)))
	}
	target := mkfp("user_deck", "Test Commander",
		append([]string{"Card_P1"}, added...), false)
	got := BuildDeckTwinSuggestion(target, []*DeckFingerprint{precon})
	if got.AddedCount != 12 {
		t.Errorf("AddedCount: want 12, got %d", got.AddedCount)
	}
	// Narrative should show ... and N more
	if !strings.Contains(got.Narrative, "and 7 more") {
		t.Errorf("narrative missing 'and 7 more' overflow: %q", got.Narrative)
	}
}

// TestBuildDeckTwinSuggestion_PrefersHighestOverlapPreconWhenMultipleShipCommander
// verifies the multi-precon-per-commander disambiguation: when more
// than one precon ships the same commander (e.g. Atraxa across
// C16 + ONE precon variants), the highest-shared-card-count match
// wins.
func TestBuildDeckTwinSuggestion_PrefersHighestOverlapPreconWhenMultipleShipCommander(t *testing.T) {
	preconA := mkfp("variant_a_precon_decklist", "Atraxa, Praetors' Voice",
		[]string{"Atraxa Core 1", "Atraxa Core 2"}, true)
	preconB := mkfp("variant_b_precon_decklist", "Atraxa, Praetors' Voice",
		[]string{"Atraxa Core 1", "Atraxa Core 2", "Atraxa Core 3", "Atraxa Core 4"},
		true)
	// User's deck overlaps more with variant B (3 shared) than
	// variant A (1 shared).
	target := mkfp("user_atraxa", "Atraxa, Praetors' Voice",
		[]string{"Atraxa Core 1", "Atraxa Core 3", "Atraxa Core 4", "User Upgrade"},
		false)
	got := BuildDeckTwinSuggestion(target, []*DeckFingerprint{preconA, preconB})
	if !got.HasTwin {
		t.Fatal("nil twin")
	}
	if got.PreconName != "variant_b_precon_decklist" {
		t.Errorf("want variant_b (higher overlap), got %q", got.PreconName)
	}
	if got.SharedCount != 3 {
		t.Errorf("SharedCount: want 3, got %d", got.SharedCount)
	}
}

// TestBuildDeckTwinSuggestion_SkipsNonPreconEntries verifies that
// only IsPrecon fingerprints are eligible matches — a user-built
// deck named to look like a precon isn't a valid twin candidate.
func TestBuildDeckTwinSuggestion_SkipsNonPreconEntries(t *testing.T) {
	fake := mkfp("fake_precon_decklist", "Sigarda, Font of Blessings",
		[]string{"Sol Ring", "Path to Exile"}, false /* IsPrecon=false */)
	target := mkfp("user_deck", "Sigarda, Font of Blessings",
		[]string{"Sol Ring"}, false)
	got := BuildDeckTwinSuggestion(target, []*DeckFingerprint{fake})
	if got.HasTwin {
		t.Errorf("HasTwin: want false (no real precon), got true (matched %q)", got.PreconName)
	}
}

// TestBuildDeckTwinSuggestion_NilTargetDoesNotPanic verifies
// defensive handling of nil input.
func TestBuildDeckTwinSuggestion_NilTargetDoesNotPanic(t *testing.T) {
	got := BuildDeckTwinSuggestion(nil, nil)
	if got == nil {
		t.Fatal("want non-nil result, got nil")
	}
	if got.HasTwin {
		t.Errorf("HasTwin on nil input: want false, got true")
	}
}

// TestPreconDisplayName_StripsSuffixAndTitleCases verifies the
// name-rendering helper.
func TestPreconDisplayName_StripsSuffixAndTitleCases(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{
			"blessed_sanctuary_innistrad_crimson_vow_commander_precon_decklist.txt",
			"Blessed Sanctuary Innistrad Crimson Vow",
		},
		{
			"foo_bar_precon_decklist",
			"Foo Bar",
		},
		{
			"",
			"precon",
		},
		{
			"already_named_precon",
			"Already Named Precon",
		},
	}
	for _, c := range cases {
		got := preconDisplayName(c.in)
		if got != c.want {
			t.Errorf("preconDisplayName(%q): want %q, got %q", c.in, c.want, got)
		}
	}
}

// TestBuildDeckTwinSuggestion_EndToEnd_AnimatedArmyVsItself runs
// the deck-twin lookup against the real precon corpus using one of
// the imported precon decks as the user deck. Self-match is
// suppressed by FindCanonicalPreconOverlap, so the test confirms
// either a near-match-to-different-precon-with-same-commander
// case OR the no-twin path. Skipped when oracle data absent.
func TestBuildDeckTwinSuggestion_EndToEnd_AnimatedArmyVsItself(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, err := loadOracle(oraclePath)
	if err != nil {
		t.Fatalf("load oracle: %v", err)
	}
	mechDB, err := BuildMechanicDB(oraclePath)
	if err != nil {
		t.Fatalf("build mechanic db: %v", err)
	}

	// Build the precon corpus fingerprints (the same set
	// main.go's --all-decks path uses).
	preconDir := "../../data/decks/wizards"
	preconFiles, _ := filepath.Glob(filepath.Join(preconDir, "*.txt"))
	var precons []*DeckFingerprint
	for _, p := range preconFiles {
		report, err := analyzeDeckFile(p, oracle, mechDB)
		if err != nil {
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		fp := BuildDeckFingerprint(dp, report)
		precons = append(precons, fp)
	}

	// Pick a target deck — animated_army_bloomburrow.
	targetPath := filepath.Join(preconDir, "animated_army_bloomburrow_commander_precon_decklist.txt")
	targetReport, err := analyzeDeckFile(targetPath, oracle, mechDB)
	if err != nil {
		t.Fatalf("analyze target: %v", err)
	}
	targetDP := BuildDeckProfile(targetReport, oracle)
	targetFP := BuildDeckFingerprint(targetDP, targetReport)

	twin := BuildDeckTwinSuggestion(targetFP, precons)
	t.Logf("Twin verdict: %s", twin.Narrative)
	if twin == nil {
		t.Fatal("nil twin")
	}
	// Self-match is suppressed; either there's a different precon
	// sharing the commander (unlikely for Bloomburrow's unique
	// commander) or HasTwin=false. Both are valid; just verify
	// the function completed without panic + emitted a narrative.
	if twin.Narrative == "" {
		t.Error("empty narrative")
	}
}
