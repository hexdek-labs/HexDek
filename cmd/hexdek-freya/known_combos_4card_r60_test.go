package main

import (
	"strings"
	"testing"
)

// known_combos_4card_r60_test.go — regressions for the r60 4-card combo
// extension. Five canonical 4-card cEDH lines were added to KnownCombos
// (Citadel+Top+Birgi+Aetherflux, Twiddle Storm, Hermit Druid Dread
// Return, Worldgorger+Bombardment package, Necrotic Ooze+Devourer+
// Triskelion+Hermit Druid). The existing AnalyzeDeck lookup is arity-
// agnostic — adding 4-card entries to KnownCombos extends detection
// without detector-side plumbing.
//
// Each test below builds a minimal deck containing all 4 pieces and
// verifies the combo lands in the relevant ComboResult slice on
// FreyaReport (TrueInfinites for "true_infinite", Determined for
// "determined", Synergies for "synergy"). A negative-of-the-fix
// companion (missing one piece) confirms the prefix-pruning gate
// surfaces the near-miss as a ComboNote but NOT a confirmed combo.

// makeProfile constructs a minimal CardProfile for combo detection.
// Combo lookup only consults p.Name + a few flags, so we don't need
// full oracle text — the ClassifyCard path is exercised separately in
// oracletext_test.go and analysis sibling tests.
func makeProfile(name string) CardProfile {
	return CardProfile{Name: name}
}

// findComboByPieces returns true if any ComboResult on the report
// (TrueInfinites / Determined / Synergies) has a Cards slice that
// contains every piece in `wantPieces`. Order-insensitive set
// containment, since the lookup populates Cards in declaration order
// and tests assert by full piece-set.
//
// Pieces are passed as a slice (NOT a CSV string) because real card
// names contain commas ("Birgi, God of Storytelling"; "Zealous
// Conscripts" — no comma but ", " is a common separator).
func findComboByPieces(report *FreyaReport, wantPieces []string) bool {
	check := func(have []string) bool {
		for _, want := range wantPieces {
			found := false
			for _, h := range have {
				if h == want {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	for _, c := range report.TrueInfinites {
		if check(c.Cards) {
			return true
		}
	}
	for _, c := range report.Determined {
		if check(c.Cards) {
			return true
		}
	}
	for _, c := range report.Synergies {
		if check(c.Cards) {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// 1. Citadel + Top + Birgi + Aetherflux — Storm finisher
// -----------------------------------------------------------------------------

func TestKnownCombo_CitadelTopBirgiAetherflux(t *testing.T) {
	profiles := []CardProfile{
		makeProfile("Bolas's Citadel"),
		makeProfile("Sensei's Divining Top"),
		makeProfile("Birgi, God of Storytelling"),
		makeProfile("Aetherflux Reservoir"),
	}
	report := AnalyzeDeck(profiles, "test", "test.txt", "")
	want := []string{
		"Bolas's Citadel", "Sensei's Divining Top",
		"Birgi, God of Storytelling", "Aetherflux Reservoir",
	}
	if !findComboByPieces(report, want) {
		t.Errorf("4-card Citadel+Top+Birgi+Aetherflux not detected; "+
			"Determined=%d TrueInfinites=%d Synergies=%d",
			len(report.Determined), len(report.TrueInfinites), len(report.Synergies))
	}
}

// Missing one piece (Birgi) → should NOT confirm, but SHOULD surface
// as a near-miss ComboNote (3-of-4 = 75% ≥ threshold).
func TestKnownCombo_CitadelTopAetherflux_NearMissNoteOnly(t *testing.T) {
	profiles := []CardProfile{
		makeProfile("Bolas's Citadel"),
		makeProfile("Sensei's Divining Top"),
		makeProfile("Aetherflux Reservoir"),
		// Birgi missing
	}
	report := AnalyzeDeck(profiles, "test", "test.txt", "")
	want := []string{
		"Bolas's Citadel", "Sensei's Divining Top",
		"Birgi, God of Storytelling", "Aetherflux Reservoir",
	}
	if findComboByPieces(report, want) {
		t.Error("3-of-4 should NOT confirm the 4-card combo")
	}
	// Near-miss note should be present.
	foundNote := false
	for _, n := range report.ComboNotes {
		if strings.Contains(n, "Citadel + Top + Birgi + Aetherflux") &&
			strings.Contains(n, "missing Birgi") {
			foundNote = true
			break
		}
	}
	if !foundNote {
		t.Errorf("expected near-miss ComboNote for 3-of-4 Citadel line; got %d notes",
			len(report.ComboNotes))
	}
}

// 2-of-4 should NOT surface a ComboNote (prefix-pruning: 50% < 75%).
// Without this gate every EDH deck running 2 staples from each of 30+
// 4-card combos would emit dozens of low-signal near-miss notes.
func TestKnownCombo_CitadelAetherflux_BelowPrefixThresholdSuppressed(t *testing.T) {
	profiles := []CardProfile{
		makeProfile("Bolas's Citadel"),
		makeProfile("Aetherflux Reservoir"),
		// 2 of 4 pieces
	}
	report := AnalyzeDeck(profiles, "test", "test.txt", "")
	for _, n := range report.ComboNotes {
		if strings.Contains(n, "Citadel + Top + Birgi + Aetherflux") {
			t.Errorf("2-of-4 (50%%) should be suppressed by prefix-pruning gate; got note: %q", n)
		}
	}
}

// -----------------------------------------------------------------------------
// 2. Twiddle Storm
// -----------------------------------------------------------------------------

func TestKnownCombo_TwiddleStorm(t *testing.T) {
	profiles := []CardProfile{
		makeProfile("Bonus Round"),
		makeProfile("Twiddle"),
		makeProfile("High Tide"),
		makeProfile("Brain Freeze"),
	}
	report := AnalyzeDeck(profiles, "test", "test.txt", "")
	want := []string{"Bonus Round", "Twiddle", "High Tide", "Brain Freeze"}
	if !findComboByPieces(report, want) {
		t.Errorf("Twiddle Storm 4-card combo not detected")
	}
}

// -----------------------------------------------------------------------------
// 3. Hermit Druid + Narcomoeba + Dread Return + Thassa's Oracle
// -----------------------------------------------------------------------------

func TestKnownCombo_HermitDruidDreadReturn(t *testing.T) {
	profiles := []CardProfile{
		makeProfile("Hermit Druid"),
		makeProfile("Narcomoeba"),
		makeProfile("Dread Return"),
		makeProfile("Thassa's Oracle"),
	}
	report := AnalyzeDeck(profiles, "test", "test.txt", "")
	want := []string{"Hermit Druid", "Narcomoeba", "Dread Return", "Thassa's Oracle"}
	if !findComboByPieces(report, want) {
		t.Errorf("Hermit Druid Dread Return 4-card combo not detected")
	}
}

// -----------------------------------------------------------------------------
// 4. Worldgorger Dragon + Animate Dead + Goblin Bombardment + Bloodghast
// -----------------------------------------------------------------------------

func TestKnownCombo_WorldgorgerBombardment(t *testing.T) {
	profiles := []CardProfile{
		makeProfile("Worldgorger Dragon"),
		makeProfile("Animate Dead"),
		makeProfile("Goblin Bombardment"),
		makeProfile("Bloodghast"),
	}
	report := AnalyzeDeck(profiles, "test", "test.txt", "")
	want := []string{"Worldgorger Dragon", "Animate Dead", "Goblin Bombardment", "Bloodghast"}
	if !findComboByPieces(report, want) {
		t.Errorf("Worldgorger + Bombardment package 4-card combo not detected")
	}
}

// -----------------------------------------------------------------------------
// 5. Necrotic Ooze + Phyrexian Devourer + Triskelion + Hermit Druid
// -----------------------------------------------------------------------------

func TestKnownCombo_NecroticOozeDevourer(t *testing.T) {
	profiles := []CardProfile{
		makeProfile("Necrotic Ooze"),
		makeProfile("Phyrexian Devourer"),
		makeProfile("Triskelion"),
		makeProfile("Hermit Druid"),
	}
	report := AnalyzeDeck(profiles, "test", "test.txt", "")
	want := []string{"Necrotic Ooze", "Phyrexian Devourer", "Triskelion", "Hermit Druid"}
	if !findComboByPieces(report, want) {
		t.Errorf("Necrotic Ooze + Devourer + Triskelion + Hermit Druid 4-card combo not detected")
	}
}

// -----------------------------------------------------------------------------
// Database integrity
// -----------------------------------------------------------------------------

// TestKnownCombos_4CardEntriesHaveAllFields pins that every 4-card combo
// has the required fields populated — Name, Pieces (length 4), Type,
// Class, Description.
func TestKnownCombos_4CardEntriesHaveAllFields(t *testing.T) {
	count := 0
	for _, k := range KnownCombos {
		if len(k.Pieces) != 4 {
			continue
		}
		count++
		if k.Name == "" {
			t.Errorf("4-card combo at Pieces=%v missing Name", k.Pieces)
		}
		if k.Type == "" {
			t.Errorf("4-card combo %q missing Type", k.Name)
		}
		if k.Class == "" {
			t.Errorf("4-card combo %q missing Class", k.Name)
		}
		if k.Description == "" {
			t.Errorf("4-card combo %q missing Description", k.Name)
		}
	}
	if count < 5 {
		t.Errorf("expected ≥5 4-card combos in database, got %d", count)
	}
}

// TestKnownCombos_4CardPiecesUnique — the 4 pieces of every 4-card
// combo must be distinct (no duplicate piece in the same combo entry).
func TestKnownCombos_4CardPiecesUnique(t *testing.T) {
	for _, k := range KnownCombos {
		if len(k.Pieces) != 4 {
			continue
		}
		seen := map[string]bool{}
		for _, p := range k.Pieces {
			if seen[p] {
				t.Errorf("4-card combo %q has duplicate piece %q in Pieces list",
					k.Name, p)
			}
			seen[p] = true
		}
	}
}
