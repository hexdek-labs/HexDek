package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWizardsPreconsAreDeclaredBracket2 pins the rubber-stamp override:
// every deck file under data/decks/wizards/ (unedited WotC precons) must
// emit dp.Bracket == 2 regardless of what Freya measures. The measured
// bracket may diverge (some precons play hotter than their stated B2
// intent — see docs/precon-vibes-baseline-r60.md) but the declared
// identity always reads B2 because that is WotC's product intent.
//
// Skipped when data/rules/oracle-cards.json is absent (gitignored
// 163MB blob; CI fetches via scripts/fetch-oracle.sh).
func TestWizardsPreconsAreDeclaredBracket2(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available at %s — run scripts/fetch-oracle.sh", oraclePath)
	}

	oracle, err := loadOracle(oraclePath)
	if err != nil {
		t.Fatalf("load oracle: %v", err)
	}
	mechDB, err := BuildMechanicDB(oraclePath)
	if err != nil {
		t.Fatalf("build mechanic db: %v", err)
	}

	wizardsDir := "../../data/decks/wizards"
	matches, err := filepath.Glob(filepath.Join(wizardsDir, "*.txt"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no wizards precon decks found at %s", wizardsDir)
	}

	for _, deck := range matches {
		base := filepath.Base(deck)
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			t.Errorf("analyze %s: %v", base, err)
			continue
		}
		if report.Profile == nil {
			t.Errorf("%s: nil profile", base)
			continue
		}
		if report.Profile.Bracket != 2 {
			t.Errorf("%s: declared bracket = %d, want 2 (wizards/ auto-stamp)",
				base, report.Profile.Bracket)
		}
		// Sanity: MeasuredBracket should still be populated (any of 1-5).
		if mb := report.Profile.MeasuredBracket; mb < 1 || mb > 5 {
			t.Errorf("%s: measured bracket = %d, want 1-5", base, mb)
		}
	}

	t.Logf("verified %d wizards/ precon decks all declare B2", len(matches))
}

// TestIsWizardsPrecon pins the path-matching helper so future deck-tree
// reorganizations don't silently break the auto-stamp.
func TestIsWizardsPrecon(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"data/decks/wizards/foo.txt", true},
		{"/abs/data/decks/wizards/bar.txt", true},
		{"../data/decks/wizards/baz.txt", true},
		{"data/decks/test/foo.txt", false},
		{"data/decks/lyon/bar.txt", false},
		{"", false},
		{"wizards.txt", false},
		// Defensive: match on /decks/wizards/ even outside the canonical
		// data/ prefix, since some callers normalize paths differently.
		{"some/other/decks/wizards/foo.txt", true},
	}
	for _, c := range cases {
		if got := isWizardsPrecon(c.path); got != c.want {
			t.Errorf("isWizardsPrecon(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
