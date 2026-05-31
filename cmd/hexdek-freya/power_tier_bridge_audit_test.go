package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPowerTierBridge_NoContradictionOnCalibration is the
// consistency regression test for the MeasuredBracket /
// CEDHPowerTier bridge. The bridge tests use a Δ≥2 contradiction
// heuristic to catch tuning regressions where one surface drifts
// significantly from the other.
//
// Post-r60-power-bracket-reconciliation (2026-05-30):
//
//	calibration test corpus: 1 / 16 contradictions (6.3%) — voja
//	WotC precon corpus:      3 / 87 contradictions (3.4%)
//
// Per-deck verdict from the audit. Each was triaged as bracket
// wrong / power-tier wrong / threshold wrong:
//
//  1. voja_wolf_elf_tribal_b4 (calibration, Δ=2 B4/T2) —
//     THRESHOLD wrong for this case. Voja is correctly B4 by
//     MeasuredBracket (tribal-voltron tempo: 8 fast mana + 4
//     finishers + 1 GC + 5% tutor) AND correctly T2 by
//     CEDHPowerTier (no free interaction, no 12% tutor density,
//     no GC density — not cEDH-shaped). Both surfaces are
//     correct on their own axes; the Δ≥2 heuristic doesn't
//     hold for legitimate high-tempo non-cEDH builds.
//
//  2. eternal_bargain_2013 (Δ=3 B4/T1) — THRESHOLD wrong.
//     Vizkopa Guildmage + Sanguine Bond is a real
//     ComboClassInfiniteDrain categorical-win combo, so B4 per
//     WotC carveout ("winning 2-card combo IS a B4 marker
//     regardless of other signals"). T1 also correct: precon
//     shell with 0 GCs, no tutoring, no interaction. Framework
//     split — bracket measures slot-table fit, PowerTier
//     measures cEDH race-readiness; they naturally diverge for
//     combo precons.
//
//  3. desert_bloom_OOT (Δ=2 B4/T2) — THRESHOLD wrong. Titania,
//     Protector of Argoth + Sand Scout is a real
//     ComboClassInfiniteTokens combo with named outlets in the
//     deck (Turntimber Sower, Ramunap Ruins, etc.), so B4 per
//     carveout. T2 also correct: precon shell with 0 GCs, 8%
//     tutors (the Outlaws precon's natural baseline, not cEDH
//     density). Same framework split as eternal_bargain.
//
//  4. family_matters_BLB (Δ=2 B3/T1) — THRESHOLD wrong.
//     Restoration Angel + Junk Winder is a heuristic-detected
//     graveyard-loop pair (2 such pairs total → +3 raw score).
//     B3 raw 5 reflects the genuine "upgraded precon" shape
//     (8 fast mana + heuristic combo lines + 4% tutor). T1
//     correct because the deck has no GC / interaction package
//     / tutor depth to ENABLE the combo lines. Same framework
//     split — graveyard-loop signal credits combo PRESENCE,
//     PowerTier requires combo SUPPORT.
//
//  CLOSED in this audit (was on the contradiction list pre-fix):
//
//  - mirror_mastery_2011 (was Δ=2 B4/T2) — BRACKET WRONG. MLD
//    floor was firing on Avatar of Fury (an 8/8 flying creature
//    with a red-spell-damage trigger, NOT mass land destruction).
//    Two sibling false-positives also pruned: "argothian wurm"
//    (single-land ETB edict, not mass-reset) and "plague of
//    vermin" (token creation, not land destruction). All three
//    entries removed from mldList in archetype.go; mirror_mastery
//    now correctly reads B2 and the contradiction closes.
//
// Frameworks disagree LEGITIMATELY on the remaining 4 (1
// calibration + 3 precons): WotC's bracket framework says "if
// the deck has a winning combo it's B4 regardless of support";
// cEDH-shape framework says "without tutors / interaction / GCs
// to ENABLE the combo, the deck plays as casual." Both are
// defensible. The test floor pins ≤1 on calibration + ≤3 on
// precons to catch tuning regressions while accepting the
// framework split.
//
// Skipped when oracle data absent.
func TestPowerTierBridge_NoContradictionOnCalibration(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, _ := loadOracle(oraclePath)
	mechDB, _ := BuildMechanicDB(oraclePath)

	// Calibration test corpus — pins ≤1 contradiction. See per-deck
	// triage in the function docstring above for the voja B4/T2 split
	// rationale. Future bracket or PowerTier tuning that lifts ANOTHER
	// calibration deck would trip this floor — keep the threshold at 1
	// unless adding new legitimate B4-but-not-cEDH calibration entries.
	testMatches, _ := filepath.Glob("../../data/decks/test/*.txt")
	testContradictions := countTierBridgeContradictions(t, oracle, mechDB, testMatches)
	if testContradictions > 1 {
		t.Errorf("calibration corpus regression: want ≤1 contradiction (voja B4/T2 split), got %d", testContradictions)
	}

	// WotC precon corpus — pins ≤3 contradictions after the r60
	// power-bracket reconciliation. The mirror_mastery_2011 case was
	// the only true bug in the residual list (MLD floor false-positive
	// on Avatar of Fury); fixing the mldList closed it. The remaining
	// 3 (eternal_bargain_2013, desert_bloom_OOT, family_matters_BLB)
	// are all WotC-vs-cEDH framework splits — see per-deck triage in
	// the function docstring above. Ratchet down further only if a
	// future PR adds real PowerTier credit for combo presence (would
	// lift T1-T2 precons to T2-T3, narrowing Δ below 2).
	wizMatches, _ := filepath.Glob("../../data/decks/wizards/*.txt")
	wizContradictions := countTierBridgeContradictions(t, oracle, mechDB, wizMatches)
	if wizContradictions > 3 {
		t.Errorf("precon corpus regression: want ≤3 contradictions, got %d", wizContradictions)
	}

	t.Logf("MeasuredBracket / PowerTier consistency:")
	t.Logf("  calibration corpus: %d / %d contradictions (Δ≥2)", testContradictions, len(testMatches))
	t.Logf("  WotC precon corpus: %d / %d contradictions (Δ≥2)", wizContradictions, len(wizMatches))
}

// countTierBridgeContradictions runs both classifiers on each deck
// in `deckPaths` and returns the count where |MeasuredBracket -
// PowerTier| ≥ 2.
func countTierBridgeContradictions(t *testing.T, oracle *oracleDB, mechDB *MechanicDB, deckPaths []string) int {
	t.Helper()
	contradictions := 0
	for _, deck := range deckPaths {
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)
		spread := dp.MeasuredBracket - tier.Tier
		if spread < 0 {
			spread = -spread
		}
		if spread >= 2 {
			contradictions++
			t.Logf("  Δ=%d: B%d / T%d  %s",
				spread, dp.MeasuredBracket, tier.Tier, filepath.Base(deck))
		}
	}
	return contradictions
}

// TestPowerTierBridge_CalibrationBracketsPreserved verifies the
// B4 confirmation gate doesn't regress the existing bracket
// calibration. All 16 calibration-corpus decks must still land
// within ±1 of their filename-encoded expected bracket. This
// guards against the B4 gate over-correcting on legitimate B4
// nightmare-builds (yarus / riku / narset).
func TestPowerTierBridge_CalibrationBracketsPreserved(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, _ := loadOracle(oraclePath)
	mechDB, _ := BuildMechanicDB(oraclePath)
	matches, _ := filepath.Glob("../../data/decks/test/*.txt")

	for _, deck := range matches {
		base := filepath.Base(deck)
		var expected int
		// Filename encodes expected bracket as _b[1-5]_
		for _, b := range []int{1, 2, 3, 4, 5} {
			if strings.Contains(base, fmt.Sprintf("_b%d_", b)) {
				expected = b
				break
			}
		}
		if expected == 0 {
			continue
		}
		report, _ := analyzeDeckFile(deck, oracle, mechDB)
		dp := BuildDeckProfile(report, oracle)
		got := dp.MeasuredBracket
		delta := got - expected
		if delta < 0 {
			delta = -delta
		}
		if delta > 1 {
			t.Errorf("%s: expected B%d, got B%d (Δ=%d > 1 — B4 gate may be over-correcting)",
				base, expected, got, delta)
		}
	}
}

// TestPowerTierBridge_Audit dumps both surfaces — MeasuredBracket
// (estimateMeasuredBracket) and PowerTier (ClassifyCEDHPowerTier) —
// for the 16-deck calibration corpus plus the 87 precons. Prints any
// divergence ≥ 2 tiers, which is the "contradiction" threshold per
// PR #724.
//
// Always passes — diagnostic dump only. The consistency assertion
// lives in TestPowerTierBridge_NoContradictionOnCalibration.
func TestPowerTierBridge_Audit(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, _ := loadOracle(oraclePath)
	mechDB, _ := BuildMechanicDB(oraclePath)

	dirs := []string{
		"../../data/decks/test",
		"../../data/decks/wizards",
	}
	contradictionsByDir := map[string]int{}
	totalByDir := map[string]int{}

	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.txt"))
		t.Logf("\n=== %s (%d decks) ===", dir, len(matches))
		for _, deck := range matches {
			base := filepath.Base(deck)
			report, err := analyzeDeckFile(deck, oracle, mechDB)
			if err != nil {
				continue
			}
			dp := BuildDeckProfile(report, oracle)
			tier := ClassifyCEDHPowerTier(dp, report)
			totalByDir[dir]++

			spread := dp.MeasuredBracket - tier.Tier
			if spread < 0 {
				spread = -spread
			}
			marker := "OK"
			if spread >= 2 {
				marker = fmt.Sprintf("CONTRADICTION (Δ=%d)", spread)
				contradictionsByDir[dir]++
			} else if spread == 1 {
				marker = "minor"
			}
			t.Logf("  B%d / T%d  %-20s  %s", dp.MeasuredBracket, tier.Tier, marker, base)
			if spread >= 2 {
				// Show discriminator data + rationale to diagnose
				ic := 0
				if report.Roles != nil {
					ic = report.RemovalCount + report.Roles.RoleCounts[RoleCounterspell] + report.Roles.RoleCounts[RoleBoardWipe]
				}
				t.Logf("       discriminators: GC=%d, tutors=%d, interaction=%d",
					dp.GameChangerCount, report.NonLandTutorCount, ic)
				if dp.BracketRationale != nil {
					t.Logf("       raw_score=%d, final_bracket=B%d",
						dp.BracketRationale.RawScore, dp.BracketRationale.FinalBracket)
					for _, s := range dp.BracketRationale.Signals {
						if s.Kind == "score" || s.Kind == "gate" {
							t.Logf("         [%s] %s: %s (+%d)", s.Kind, s.Name, s.Measurement, s.Contribution)
						}
					}
				}
			}
		}
	}
	t.Logf("\n=== Summary ===")
	for _, dir := range dirs {
		t.Logf("  %s: %d / %d contradictions (Δ≥2)",
			filepath.Base(dir), contradictionsByDir[dir], totalByDir[dir])
	}
}
