package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestCEDHPowerTier_PreconCorpusVerification runs the cEDH power-tier
// classifier against every imported WotC precon under data/decks/wizards/
// and asserts the classification is structurally plausible: unedited
// WotC precons are designed for the casual / upgraded-precon power band
// (B1-B3). Any precon classifying as B4 (High Power) or B5 (cEDH) is a
// classifier bug to surface — precons do not ship in tournament-cEDH
// shape.
//
// Failure mode: per-precon t.Errorf with the deck name, tier, label,
// and the signal breakdown that drove the call. Aggregate distribution
// logged regardless of pass/fail so the calibration shape is visible
// in the test output.
//
// Skipped when data/rules/oracle-cards.json is absent.
func TestCEDHPowerTier_PreconCorpusVerification(t *testing.T) {
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

	deckDir := "../../data/decks/wizards"
	matches, err := filepath.Glob(filepath.Join(deckDir, "*.txt"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) < 50 {
		t.Fatalf("precon corpus shrunk to %d decks — expected 87+", len(matches))
	}

	type preconResult struct {
		Name      string
		Tier      int
		Label     string
		Score     int
		Signals   []CEDHTierSignal
		Verdict   string
	}
	var results []preconResult
	tierDist := map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}
	overTierCount := 0

	for _, deck := range matches {
		base := filepath.Base(deck)
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			t.Errorf("analyze %s: %v", base, err)
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)
		results = append(results, preconResult{
			Name:    base,
			Tier:    tier.Tier,
			Label:   tier.Label,
			Score:   tier.Score,
			Signals: tier.Signals,
			Verdict: tier.Verdict,
		})
		tierDist[tier.Tier]++

		if tier.Tier > 3 {
			overTierCount++
			var sigStrs []string
			for _, s := range tier.Signals {
				sigStrs = append(sigStrs, fmt.Sprintf("%s T%d (%s)", s.Name, s.TierVote, s.Measurement))
			}
			t.Errorf("PRECON OVER-CLASSIFIED: %s → T%d %q (expected B1-B3)\n  signals: %s\n  verdict: %s",
				base, tier.Tier, tier.Label, strings.Join(sigStrs, "; "), tier.Verdict)
		}
	}

	t.Logf("=== Precon Power-Tier Verification ===")
	t.Logf("Total decks: %d", len(results))
	for _, tier := range []int{1, 2, 3, 4, 5} {
		label := cedhTierLabel(tier)
		count := tierDist[tier]
		pct := 0.0
		if len(results) > 0 {
			pct = 100.0 * float64(count) / float64(len(results))
		}
		t.Logf("  T%d %-16s %3d (%4.1f%%)", tier, label, count, pct)
	}
	t.Logf("Over-classification (T4+) count: %d", overTierCount)

	// Sort results by tier desc then name for the per-deck dump.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Tier != results[j].Tier {
			return results[i].Tier > results[j].Tier
		}
		return results[i].Name < results[j].Name
	})

	t.Logf("\n=== Per-deck breakdown (sorted by tier desc) ===")
	for _, r := range results {
		t.Logf("  T%d %-16s score=%2d  %s", r.Tier, r.Label, r.Score, r.Name)
	}
}

// TestCEDHPowerTier_PreconStatsDump (no-fail) computes per-signal
// distribution stats across the precon corpus — used for tuning the
// classifier if the verification test surfaces a systemic over-call.
func TestCEDHPowerTier_PreconStatsDump(t *testing.T) {
	if testing.Short() {
		t.Skip("stats dump skipped under -short")
	}
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, _ := loadOracle(oraclePath)
	mechDB, _ := BuildMechanicDB(oraclePath)
	matches, _ := filepath.Glob(filepath.Join("../../data/decks/wizards", "*.txt"))

	signalVotes := map[string]map[int]int{}
	for _, deck := range matches {
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)
		for _, s := range tier.Signals {
			if signalVotes[s.Name] == nil {
				signalVotes[s.Name] = map[int]int{}
			}
			signalVotes[s.Name][s.TierVote]++
		}
	}
	t.Logf("=== Per-signal vote distribution across %d precons ===", len(matches))
	for sigName, dist := range signalVotes {
		var line strings.Builder
		fmt.Fprintf(&line, "  %-22s ", sigName)
		for tv := 1; tv <= 5; tv++ {
			fmt.Fprintf(&line, "T%d=%-3d  ", tv, dist[tv])
		}
		t.Log(line.String())
	}
}
