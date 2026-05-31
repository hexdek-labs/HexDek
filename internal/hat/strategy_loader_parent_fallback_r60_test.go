package hat

import (
	"os"
	"path/filepath"
	"testing"
)

// r60-cedh-bottleneck (docs/hat-bottleneck-r60.md): LoadStrategyFromFreya
// must find strategy.json / _freya.json files staged at the grandparent's
// freya/ directory, not just the deck's immediate parent. This is the
// canonical gauntlet staging layout:
//
//   <root>/
//     freya/<deck>.strategy.json
//     batch_a/<deck>.txt
//     batch_b/<deck>.txt
//
// Pre-fix, the loader looked only at <root>/batch_a/freya/<deck>.strategy.json
// (which doesn't exist), returned nil, and the hat ran without any
// Freya intelligence. This silently disabled every architectural change
// in PRs #793 + #826 + #848 that depended on Strategy.ComboPieces being
// populated — the entire investigation chain operated on profile=nil.

const fixtureStrategyJSON = `{
  "archetype": "combo",
  "bracket": 5,
  "win_lines": [
    {
      "pieces": ["PieceA", "PieceB"],
      "type": "determined",
      "class": "library_exile_win"
    }
  ],
  "tutor_targets": ["PieceA", "PieceB"]
}`

// TestLoadStrategyFromFreya_DeckLocalStillWorks pins the pre-existing
// behavior: a strategy.json at <deckdir>/freya/<base>.strategy.json
// loads correctly. The new parent-dir fallback must not regress this.
func TestLoadStrategyFromFreya_DeckLocalStillWorks(t *testing.T) {
	tmp := t.TempDir()
	deckPath := filepath.Join(tmp, "francisco_b5.txt")
	if err := os.WriteFile(deckPath, []byte("// dummy deck\n"), 0644); err != nil {
		t.Fatal(err)
	}
	freyaDir := filepath.Join(tmp, "freya")
	if err := os.MkdirAll(freyaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(freyaDir, "francisco_b5.strategy.json"), []byte(fixtureStrategyJSON), 0644); err != nil {
		t.Fatal(err)
	}

	sp := LoadStrategyFromFreya(deckPath)
	if sp == nil {
		t.Fatal("deck-local freya strategy should have loaded")
	}
	if sp.Archetype != "combo" {
		t.Errorf("archetype = %q, want combo", sp.Archetype)
	}
	if len(sp.ComboPieces) != 1 {
		t.Errorf("ComboPieces len = %d, want 1", len(sp.ComboPieces))
	}
}

// TestLoadStrategyFromFreya_ParentDirFallback pins the new behavior:
// a strategy.json at <root>/freya/ is found when the deck is staged
// at <root>/batch_a/. This is the canonical cEDH gauntlet layout the
// pre-fix loader missed.
func TestLoadStrategyFromFreya_ParentDirFallback(t *testing.T) {
	tmp := t.TempDir()
	batchDir := filepath.Join(tmp, "batch_a")
	freyaDir := filepath.Join(tmp, "freya")
	if err := os.MkdirAll(batchDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(freyaDir, 0755); err != nil {
		t.Fatal(err)
	}

	deckPath := filepath.Join(batchDir, "francisco_b5.txt")
	if err := os.WriteFile(deckPath, []byte("// dummy deck\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Strategy at PARENT's freya/ (one level above batch_a).
	if err := os.WriteFile(filepath.Join(freyaDir, "francisco_b5.strategy.json"), []byte(fixtureStrategyJSON), 0644); err != nil {
		t.Fatal(err)
	}

	sp := LoadStrategyFromFreya(deckPath)
	if sp == nil {
		t.Fatal("parent-dir freya strategy should have loaded; the four-PR cedh investigation depended on this path")
	}
	if sp.Archetype != "combo" {
		t.Errorf("archetype = %q, want combo (loaded from grandparent's freya/)", sp.Archetype)
	}
	if len(sp.ComboPieces) != 1 {
		t.Errorf("ComboPieces len = %d, want 1 (loaded from grandparent's freya/)", len(sp.ComboPieces))
	}
}

// TestLoadStrategyFromFreya_DeckLocalPreferredOverParent pins the
// search-order contract: when BOTH the deck-local and parent-dir
// strategy.json exist, the deck-local one wins. This protects the
// per-batch-override scenario where a gauntlet wants to ship a
// tuned variant for one batch without disturbing the global pool's
// analysis.
func TestLoadStrategyFromFreya_DeckLocalPreferredOverParent(t *testing.T) {
	tmp := t.TempDir()
	batchDir := filepath.Join(tmp, "batch_a")
	batchFreyaDir := filepath.Join(batchDir, "freya")
	parentFreyaDir := filepath.Join(tmp, "freya")
	for _, d := range []string{batchDir, batchFreyaDir, parentFreyaDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	deckPath := filepath.Join(batchDir, "francisco_b5.txt")
	if err := os.WriteFile(deckPath, []byte("// dummy deck\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Two strategy files: deck-local says "combo", parent says "control".
	localJSON := `{"archetype": "combo", "win_lines": [{"pieces":["X","Y"],"type":"determined"}]}`
	parentJSON := `{"archetype": "control", "win_lines": []}`
	if err := os.WriteFile(filepath.Join(batchFreyaDir, "francisco_b5.strategy.json"), []byte(localJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parentFreyaDir, "francisco_b5.strategy.json"), []byte(parentJSON), 0644); err != nil {
		t.Fatal(err)
	}

	sp := LoadStrategyFromFreya(deckPath)
	if sp == nil {
		t.Fatal("strategy should have loaded")
	}
	if sp.Archetype != "combo" {
		t.Errorf("deck-local should win over parent-dir; got archetype=%q want combo", sp.Archetype)
	}
}

// TestLoadStrategyFromFreya_NilWhenNothingPresent pins the graceful-
// degradation contract: when neither deck-local nor parent-dir strategy
// is present, the loader returns nil and the hat falls back to the
// no-strategy default. The four-PR investigation's WARNING log lines
// trigger off this nil.
func TestLoadStrategyFromFreya_NilWhenNothingPresent(t *testing.T) {
	tmp := t.TempDir()
	deckPath := filepath.Join(tmp, "no_freya.txt")
	if err := os.WriteFile(deckPath, []byte("// dummy\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if sp := LoadStrategyFromFreya(deckPath); sp != nil {
		t.Errorf("expected nil when no freya analysis present; got %+v", sp)
	}
}
