package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPowerTierDistributionCalibration runs computeCardPower across the
// curated known-bracket deck set and asserts the S/A/B/C/D distribution
// stays within the calibration target bands (the standard tier-grading
// shape: roughly normal peaked at B with thin tails). Pins the
// recalibrated 70/55/38/25 thresholds against the corpus baseline
// established in PR #341 — if a future tweak to computeCardPower or
// PowerTierFor shifts the distribution outside the bands, this test
// flags it as a calibration regression so the thresholds get retuned
// alongside the scoring change.
//
// Target bands (with ±5pp tolerance):
//
//	S 5-12%   A 14-24%   B 30-45%   C 23-35%   D 5-15%
//
// Skipped when data/rules/oracle-cards.json is absent (gitignored).
func TestPowerTierDistributionCalibration(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available at %s", oraclePath)
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
		t.Fatalf("calibration corpus shrunk to %d decks", len(matches))
	}

	var reports []*FreyaReport
	for _, deck := range matches {
		r, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			t.Errorf("analyze %s: %v", filepath.Base(deck), err)
			continue
		}
		reports = append(reports, r)
	}

	agg := ComputePowerTierAggregate(reports)
	if agg.TotalCards == 0 {
		t.Fatalf("aggregate produced 0 cards")
	}

	bands := []struct {
		tier       string
		minPct     float64
		maxPct     float64
		commentary string
	}{
		{"S", 0.03, 0.15, "elite cards should be rare but present"},
		{"A", 0.10, 0.28, "strong supports"},
		{"B", 0.25, 0.50, "solid utility — the peak tier"},
		{"C", 0.18, 0.40, "situational filler"},
		{"D", 0.03, 0.20, "obvious cut candidates"},
	}
	for _, b := range bands {
		got := agg.TierPercents[b.tier]
		if got < b.minPct || got > b.maxPct {
			t.Errorf("tier %s distribution out of calibration band: got %.1f%%, want [%.1f%%, %.1f%%] (%s)",
				b.tier, got*100, b.minPct*100, b.maxPct*100, b.commentary)
		}
	}

	// Tiers must sum to 100% (defensive — catches map-key drift).
	total := 0.0
	for _, b := range bands {
		total += agg.TierPercents[b.tier]
	}
	if total < 0.99 || total > 1.01 {
		t.Errorf("tier percents should sum to ~1.0, got %.3f", total)
	}

	// Log the actual distribution so the calibration baseline is
	// visible in test output without re-running the aggregate tool.
	t.Logf("calibration: %d decks / %d cards   mean=%.1f median=%d",
		agg.DeckCount, agg.TotalCards, agg.MeanPower, agg.MedianPower)
	for _, tier := range PowerTierOrder {
		t.Logf("  %s: %5d cards (%5.1f%%)", tier,
			agg.TierCounts[tier], agg.TierPercents[tier]*100)
	}
}
