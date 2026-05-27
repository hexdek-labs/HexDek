package main

import (
	"testing"
)

// oracletext_tokenize_r60_test.go — regressions for the r60 unified
// tokenizing scanner (ScanOracle / ClauseCoOccurs / ClauseContains) and the
// graveyard-consume FP fix that motivated wiring it up.
//
// The pre-existing oracletext_test.go and oracletext_scan_test.go cover the
// 10 worst single-keyword leaks (storm / morph / transform / infect / mill /
// landfall + cascade / flashback / eternalize / modal-preamble). This file
// pins the next-worst class of leak: PAIRED substring co-occurrence that
// spans reminder text or unrelated clauses.

// -----------------------------------------------------------------------------
// ScanOracle structural contract
// -----------------------------------------------------------------------------

func TestScanOracle_NonModalCard(t *testing.T) {
	// Lightning Bolt — single clause, no modal, no reminder.
	scan := ScanOracle("Lightning Bolt deals 3 damage to any target.")
	if scan.Clean != "lightning bolt deals 3 damage to any target." {
		t.Errorf("Clean = %q", scan.Clean)
	}
	if len(scan.Clauses) != 1 {
		t.Fatalf("want 1 clause, got %d: %#v", len(scan.Clauses), scan.Clauses)
	}
	if scan.Clauses[0].Mode != -1 {
		t.Errorf("non-modal clause should have Mode=-1, got %d", scan.Clauses[0].Mode)
	}
}

func TestScanOracle_ReminderTextStripped(t *testing.T) {
	// Flashback reminder text must not survive into the clause list.
	ot := "Sacrifice a creature. Flashback {B} (You may cast this card from your graveyard for its flashback cost. Then exile it.)"
	scan := ScanOracle(ot)
	for _, c := range scan.Clauses {
		if containsAnyLiteral(c.Text, "from your graveyard", "exile it") {
			t.Errorf("reminder text survived into clause %q (Clean=%q)", c.Text, scan.Clean)
		}
	}
}

func TestScanOracle_ModalBulletsTagged(t *testing.T) {
	// Cryptic Command — 4 modes choose two; each bullet becomes its own
	// clause with Mode = 0..3 in declaration order.
	ot := "Choose two — • Counter target spell. • Return target permanent to its owner's hand. " +
		"• Tap all creatures your opponents control. • Draw a card."
	scan := ScanOracle(ot)
	// No preamble clauses (pure modal) — only bullets at Mode 0..3.
	wantModes := []int{0, 1, 2, 3}
	if len(scan.Clauses) != 4 {
		t.Fatalf("want 4 clauses, got %d: %#v", len(scan.Clauses), scan.Clauses)
	}
	for i, c := range scan.Clauses {
		if c.Mode != wantModes[i] {
			t.Errorf("clause %d Mode = %d, want %d (text=%q)", i, c.Mode, wantModes[i], c.Text)
		}
	}
	// Mode-2 clause should be the tap-all-creatures bullet; nothing about
	// it should touch the counter or draw bullets.
	mode2 := scan.Clauses[2].Text
	if !containsLiteral(mode2, "tap") {
		t.Errorf("mode 2 clause should be the tap bullet, got %q", mode2)
	}
}

func TestScanOracle_ModalWithPreamble(t *testing.T) {
	// Synthetic shape — preamble always applies, bullets are modal.
	ot := "When this creature enters, draw a card. Choose one — • Gain 3 life. • Lose 3 life."
	scan := ScanOracle(ot)
	// First clause is the preamble (Mode -1); next two are bullets (Mode 0, 1).
	if len(scan.Clauses) < 3 {
		t.Fatalf("want at least 3 clauses, got %d: %#v", len(scan.Clauses), scan.Clauses)
	}
	if scan.Clauses[0].Mode != -1 {
		t.Errorf("preamble clause should be Mode -1, got %d", scan.Clauses[0].Mode)
	}
	if scan.Clauses[1].Mode != 0 || scan.Clauses[2].Mode != 1 {
		t.Errorf("bullets should be Mode 0/1, got %d/%d", scan.Clauses[1].Mode, scan.Clauses[2].Mode)
	}
}

func TestScanOracle_EmptyInput(t *testing.T) {
	scan := ScanOracle("")
	if scan == nil {
		t.Fatal("ScanOracle must return non-nil even on empty input")
	}
	if scan.Clean != "" {
		t.Errorf("empty Clean expected, got %q", scan.Clean)
	}
	if len(scan.Clauses) != 0 {
		t.Errorf("empty Clauses expected, got %#v", scan.Clauses)
	}
}

// -----------------------------------------------------------------------------
// ClauseCoOccurs — paired-substring FP narrowing
// -----------------------------------------------------------------------------

func TestClauseCoOccurs_Basics(t *testing.T) {
	// "exile" + "from your graveyard" in one clause — true positive.
	scan := ScanOracle("Underworld Breach: exile three other cards from your graveyard.")
	if !ClauseCoOccurs(scan, "exile", "from your graveyard") {
		t.Error("ClauseCoOccurs should fire when both anchors live in the same clause")
	}
}

func TestClauseCoOccurs_AcrossClausesIsFalse(t *testing.T) {
	// "exile" in one clause, "from your graveyard" in a different one —
	// narrowing to single-clause kills the false positive.
	scan := ScanOracle("Exile target creature. Search for a card from your graveyard.")
	if ClauseCoOccurs(scan, "exile target creature", "from your graveyard") {
		t.Error("ClauseCoOccurs must NOT fire when anchors span separate clauses")
	}
}

func TestClauseCoOccurs_EmptyArgs(t *testing.T) {
	scan := ScanOracle("Lightning Bolt deals 3 damage.")
	if ClauseCoOccurs(scan, "", "damage") {
		t.Error("empty first arg should return false")
	}
	if ClauseCoOccurs(nil, "a", "b") {
		t.Error("nil scan should return false")
	}
}

// -----------------------------------------------------------------------------
// Flashback FP — the motivating real-card case for the migration.
// -----------------------------------------------------------------------------

// Cabal Therapy's flashback reminder contains both "exile" and "from your
// graveyard"; pre-fix analysis.go:372 used raw `ot`, so every flashback /
// encore / embalm / eternalize / aftermath card got Consumes=ResGraveyard
// from reminder text alone, wrongly closing card↔graveyard cycles in combo
// detection. The migration to otClean (this branch) drops the leak.
func TestClassifyCard_FlashbackNotGraveyardConsume(t *testing.T) {
	ot := "Name a nonland card. Target player reveals their hand and discards all cards with that name. " +
		"Flashback — Sacrifice a creature. (You may cast this card from your graveyard for its flashback cost. Then exile it.)"
	p := ClassifyCard("Cabal Therapy", ot, "Sorcery", "{B}", 1, "")
	for _, r := range p.Consumes {
		if r == ResGraveyard {
			t.Errorf("Cabal Therapy (flashback) tagged Consumes=ResGraveyard from reminder text. Consumes=%v", p.Consumes)
			return
		}
	}
}

func TestClassifyCard_EternalizeNotGraveyardConsume(t *testing.T) {
	// Champion of Wits — eternalize reminder also reads "Exile this card
	// from your graveyard". Same FP class as flashback.
	ot := "When this creature enters, draw two cards, then discard two cards. " +
		"Eternalize {5}{U}{U} ({5}{U}{U}, Exile this card from your graveyard: Create a token that's a copy of this card, " +
		"except it's a 4/4 black Zombie Human Wizard. Eternalize only as a sorcery.)"
	p := ClassifyCard("Champion of Wits", ot, "Creature — Human Wizard", "{2}{U}", 3, "1")
	for _, r := range p.Consumes {
		if r == ResGraveyard {
			t.Errorf("Champion of Wits (eternalize) tagged Consumes=ResGraveyard from reminder text. Consumes=%v", p.Consumes)
			return
		}
	}
}

// Negative-of-the-fix: a card whose REAL rules text (not reminder) says
// "exile … from your graveyard" must still get ResGraveyard tagged.
func TestClassifyCard_UnderworldBreachStillGraveyardConsume(t *testing.T) {
	ot := "Each nonland card in your graveyard has escape. The escape cost is equal to the card's mana cost plus exile three other cards from your graveyard. " +
		"(You may cast a card from your graveyard for its escape cost.) " +
		"At the beginning of the next end step, sacrifice this enchantment."
	p := ClassifyCard("Underworld Breach", ot, "Enchantment", "{1}{R}", 2, "")
	found := false
	for _, r := range p.Consumes {
		if r == ResGraveyard {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Underworld Breach lost Consumes=ResGraveyard after the otClean migration. Consumes=%v", p.Consumes)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func containsLiteral(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func containsAnyLiteral(haystack string, needles ...string) bool {
	for _, n := range needles {
		if containsLiteral(haystack, n) {
			return true
		}
	}
	return false
}
