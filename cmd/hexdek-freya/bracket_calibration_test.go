package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// TestBracketCalibration runs Freya bracket estimation against the curated
// known-bracket deck set in data/decks/test/ (filenames encode the expected
// bracket as `..._b[1-5]_...txt`) and pins both per-deck ±1 accuracy and the
// overall exact-match rate. The corpus is the calibration anchor for the
// estimateBracket scoring formula in archetype.go — if you tune the formula,
// re-run this test and update the floor.
//
// Skipped when data/rules/oracle-cards.json is absent (gitignored 163MB blob).
func TestBracketCalibration(t *testing.T) {
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

	deckDir := "../../data/decks/test"
	matches, err := filepath.Glob(filepath.Join(deckDir, "*.txt"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) < 10 {
		t.Fatalf("calibration corpus shrunk to %d decks — need at least 10", len(matches))
	}

	re := regexp.MustCompile(`_b([1-5])_`)
	type result struct {
		name     string
		expected int
		got      int
	}
	var results []result
	exact := 0
	for _, deck := range matches {
		base := filepath.Base(deck)
		m := re.FindStringSubmatch(base)
		if len(m) != 2 {
			continue // skip files without bracket encoded
		}
		expected, _ := strconv.Atoi(m[1])

		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			t.Errorf("analyze %s: %v", base, err)
			continue
		}
		got := report.Archetype.Bracket
		results = append(results, result{base, expected, got})

		// Per-deck guard: never off by more than 1 bracket.
		if abs(got-expected) > 1 {
			t.Errorf("%s: expected B%d, got B%d (off by %d, max 1 allowed)",
				base, expected, got, abs(got-expected))
		}
		if got == expected {
			exact++
		}
	}

	// Overall accuracy floor — re-tune if scoring changes.
	const minExact = 14
	if exact < minExact {
		t.Errorf("calibration regression: %d/%d exact matches, need at least %d", exact, len(results), minExact)
	}

	t.Logf("bracket calibration: %d/%d exact across %d decks", exact, len(results), len(results))
	for _, r := range results {
		marker := "OK"
		if r.got != r.expected {
			marker = fmt.Sprintf("MISS (off %d)", r.got-r.expected)
		}
		t.Logf("  %-50s exp=B%d got=B%d  %s", r.name, r.expected, r.got, marker)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
