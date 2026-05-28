package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSuggestedChanges_PreconQualityReview runs BuildSuggestedChanges
// against 5 known WotC precons and dumps a complete diagnostic per
// deck — input metrics + every Add / Cut / Swap with full reason.
// Driven by docs/freya-suggestions-quality-r60.md to produce the
// deck-by-deck quality review.
//
// Selected precons span archetype + era:
//   - animated_army (Bloomburrow, recent, Tokens)
//   - vampiric_bloodlust (C17, classic, Vampire Tribal)
//   - blast_from_the_past (Doctor Who, time-spells theme)
//   - tyranid_swarm (40K, swarm aggro)
//   - cabaretti_cacophony (Capenna, go-wide tokens)
//
// Test always passes — it's a diagnostic dump, not an assertion
// suite. The output is read by humans and translated into the
// quality-review doc. Skipped when oracle data absent.
func TestSuggestedChanges_PreconQualityReview(t *testing.T) {
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

	precons := []string{
		"../../data/decks/wizards/animated_army_bloomburrow_commander_precon_decklist.txt",
		"../../data/decks/wizards/vampiric_bloodlust_commander_2017_precon_decklist.txt",
		"../../data/decks/wizards/blast_from_the_past_doctor_who_commander_precon_decklist.txt",
		"../../data/decks/wizards/tyranid_swarm_warhammer_40_000_commander_precon_decklist.txt",
		"../../data/decks/wizards/cabaretti_cacophony_streets_of_new_capenna_commander_precon_decklist.txt",
	}

	t.Logf("=== SuggestedChanges Quality Review — 5 WotC Precons ===\n")

	for _, p := range precons {
		base := filepath.Base(p)
		t.Logf("\n========================================")
		t.Logf("DECK: %s", base)
		t.Logf("========================================")

		report, err := analyzeDeckFile(p, oracle, mechDB)
		if err != nil {
			t.Errorf("analyze %s: %v", base, err)
			continue
		}
		dp := BuildDeckProfile(report, oracle)

		t.Logf("Archetype:       %s", dp.PrimaryArchetype)
		t.Logf("Bracket:         B%d", dp.Bracket)
		t.Logf("Color Identity:  %s", strings.Join(dp.ColorIdentity, ""))
		t.Logf("LandCount:       %d (target %d)", dp.LandCount, dp.RecommendedLands)
		t.Logf("AvgCMC:          %.2f", dp.AvgCMC)
		t.Logf("ManaBase Grade:  %s", dp.ManaBaseGrade)
		t.Logf("Taplands:        %d", dp.TaplandCount)
		t.Logf("RampCount:       %d", dp.RampCount)
		t.Logf("DrawCount:       %d", dp.DrawCount)
		t.Logf("RemovalCount:    %d", report.RemovalCount)
		if report.Roles != nil {
			t.Logf("Counterspells:   %d", report.Roles.RoleCounts[RoleCounterspell])
			t.Logf("BoardWipes:      %d", report.Roles.RoleCounts[RoleBoardWipe])
		}
		t.Logf("NonLandTutors:   %d", report.NonLandTutorCount)
		t.Logf("CuttableCards:   %d entries", len(dp.CuttableCards))

		suggestions := BuildSuggestedChanges(dp, report)
		t.Logf("\n--- SUMMARY ---")
		t.Logf("%s", suggestions.Summary)
		t.Logf("Totals: %d adds / %d cuts / %d swaps",
			len(suggestions.Adds), len(suggestions.Cuts), len(suggestions.Swaps))

		if len(suggestions.Adds) > 0 {
			t.Logf("\n--- ADDS (sorted by priority) ---")
			for i, a := range suggestions.Adds {
				t.Logf("  [%d] P%d  %s  [%s]", i+1, a.Priority, a.CardName, a.Category)
				t.Logf("       → %s", a.Reason)
			}
		}
		if len(suggestions.Swaps) > 0 {
			t.Logf("\n--- SWAPS (sorted by priority) ---")
			for i, s := range suggestions.Swaps {
				t.Logf("  [%d] P%d  swap %s -> %s  [%s]",
					i+1, s.Priority, s.CutCardName, s.AddCardName, s.Category)
				t.Logf("       → %s", s.Reason)
			}
		}
		if len(suggestions.Cuts) > 0 {
			t.Logf("\n--- CUTS (top 5, sorted by priority) ---")
			limit := len(suggestions.Cuts)
			if limit > 5 {
				limit = 5
			}
			for i := 0; i < limit; i++ {
				c := suggestions.Cuts[i]
				t.Logf("  [%d] P%d  %s  [%s]", i+1, c.Priority, c.CardName, c.Category)
				t.Logf("       → %s", c.Reason)
			}
			if len(suggestions.Cuts) > 5 {
				t.Logf("  ... %d more cuts elided", len(suggestions.Cuts)-5)
			}
		}
	}
	t.Logf("\n=== END Quality Review ===")

	// Silence go-vet about unused import on fmt — used below.
	_ = fmt.Sprintf
}
