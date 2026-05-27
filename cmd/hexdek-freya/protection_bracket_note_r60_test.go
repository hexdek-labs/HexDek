package main

import (
	"strings"
	"testing"
)

// findVulnerabilityNote returns the BracketSignal with Name="Vulnerability
// cap" (and nil if absent). Test helper.
func findVulnerabilityNote(br *BracketRationale) *BracketSignal {
	if br == nil {
		return nil
	}
	for i := range br.Signals {
		if br.Signals[i].Name == "Vulnerability cap" {
			return &br.Signals[i]
		}
	}
	return nil
}

// TestAppendVulnerabilityBracketNote_B4WithVulnerableComboFires verifies
// the canonical case: a B4 deck with one unprotected combo piece gets
// the soft "vulnerability cap" note appended. Bracket itself MUST stay
// at B4 — this is informational only, not a real cap.
func TestAppendVulnerabilityBracketNote_B4WithVulnerableComboFires(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket: 4,
		VulnerableComboPieces: []VulnerableComboPiece{
			{Name: "Thassa's Oracle", CMC: 2, Reason: "combo piece — no built-in protection"},
		},
		BracketRationale: &BracketRationale{
			FinalBracket: 4,
			FinalLabel:   "Optimized",
			RawScore:     8,
			Signals: []BracketSignal{
				{Name: "Game Changers", Kind: "score", Contribution: 4},
			},
		},
	}
	appendVulnerabilityBracketNote(dp)

	note := findVulnerabilityNote(dp.BracketRationale)
	if note == nil {
		t.Fatalf("expected Vulnerability cap note, got signals: %+v", dp.BracketRationale.Signals)
	}
	if note.Kind != "note" {
		t.Errorf("Kind = %q, want %q (must NOT be ceiling/floor/gate — gentle note only)", note.Kind, "note")
	}
	if !strings.Contains(note.Note, "Bracket 3") {
		t.Errorf("note should reference plays-closer-to-Bracket-3 intent, got: %q", note.Note)
	}
	if !strings.Contains(note.Note, "Thassa's Oracle") {
		t.Errorf("note should name the vulnerable piece, got: %q", note.Note)
	}
	if dp.BracketRationale.FinalBracket != 4 || dp.BracketRationale.FinalLabel != "Optimized" {
		t.Errorf("bracket itself must NOT change (got B%d %q) — note is informational only",
			dp.BracketRationale.FinalBracket, dp.BracketRationale.FinalLabel)
	}
}

// TestAppendVulnerabilityBracketNote_B5WithVulnerableComboFires verifies
// cEDH-level decks (B5) with unprotected combo pieces also get the note.
// B5 cEDH decks usually pair their combos with free interaction, but the
// note fires regardless — it's about the combo piece's per-card surface
// area, not about counterspell backup elsewhere.
func TestAppendVulnerabilityBracketNote_B5WithVulnerableComboFires(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket: 5,
		VulnerableComboPieces: []VulnerableComboPiece{
			{Name: "Thassa's Oracle", CMC: 2},
			{Name: "Demonic Consultation", CMC: 1},
		},
		BracketRationale: &BracketRationale{FinalBracket: 5, FinalLabel: "cEDH"},
	}
	appendVulnerabilityBracketNote(dp)
	if findVulnerabilityNote(dp.BracketRationale) == nil {
		t.Fatalf("B5 deck with 2 vulnerable combo pieces should fire the note")
	}
	if dp.BracketRationale.FinalBracket != 5 {
		t.Errorf("B5 bracket must stay B5; got B%d", dp.BracketRationale.FinalBracket)
	}
}

// TestAppendVulnerabilityBracketNote_B3DoesNotFire verifies the gate at
// MeasuredBracket > 3: a B3 deck with vulnerable combo pieces does NOT
// get the note. The note's purpose is "your B4/B5 deck plays softer than
// the bracket suggests" — at B3 there's nowhere to gently push.
func TestAppendVulnerabilityBracketNote_B3DoesNotFire(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket: 3,
		VulnerableComboPieces: []VulnerableComboPiece{
			{Name: "Combo Piece", CMC: 2},
		},
		BracketRationale: &BracketRationale{FinalBracket: 3, FinalLabel: "Upgraded"},
	}
	appendVulnerabilityBracketNote(dp)
	if findVulnerabilityNote(dp.BracketRationale) != nil {
		t.Errorf("B3 deck should NOT fire the vulnerability note (gate is strictly > 3)")
	}
}

// TestAppendVulnerabilityBracketNote_B2DoesNotFire pins the lower-bracket
// no-op explicitly so a future refactor can't silently let casual decks
// pick up the note.
func TestAppendVulnerabilityBracketNote_B2DoesNotFire(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket: 2,
		VulnerableComboPieces: []VulnerableComboPiece{
			{Name: "Combo Piece", CMC: 2},
		},
		BracketRationale: &BracketRationale{FinalBracket: 2, FinalLabel: "Core"},
	}
	appendVulnerabilityBracketNote(dp)
	if findVulnerabilityNote(dp.BracketRationale) != nil {
		t.Errorf("B2 deck should NOT fire the vulnerability note")
	}
}

// TestAppendVulnerabilityBracketNote_NoVulnerableComboDoesNotFire
// verifies a B4 deck whose combo pieces ALL have built-in protection
// gets no note. The protection-density work already counts them as
// Protected; the rationale doesn't need a "you're fine" note.
func TestAppendVulnerabilityBracketNote_NoVulnerableComboDoesNotFire(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket:       4,
		VulnerableComboPieces: nil,
		BracketRationale:      &BracketRationale{FinalBracket: 4, FinalLabel: "Optimized"},
	}
	appendVulnerabilityBracketNote(dp)
	if findVulnerabilityNote(dp.BracketRationale) != nil {
		t.Errorf("deck with zero vulnerable combo pieces should NOT fire the note")
	}
}

// TestAppendVulnerabilityBracketNote_NilRationaleSafe pins the defensive
// nil-guard. A B4 deck whose archetype classifier didn't populate a
// rationale must not panic.
func TestAppendVulnerabilityBracketNote_NilRationaleSafe(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket: 4,
		VulnerableComboPieces: []VulnerableComboPiece{
			{Name: "Combo Piece", CMC: 2},
		},
		BracketRationale: nil,
	}
	// Must not panic.
	appendVulnerabilityBracketNote(dp)
}

// TestAppendVulnerabilityBracketNote_EvidenceCappedAtThree verifies the
// in-note name list shows the first 3 vulnerable pieces (the highest-
// CMC ones, since computeProtectionDensity sorts CMC-desc upstream) and
// then a "+N more" suffix for the remainder. Keeps the note scannable.
func TestAppendVulnerabilityBracketNote_EvidenceCappedAtThree(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket: 4,
		VulnerableComboPieces: []VulnerableComboPiece{
			{Name: "Big Bomb", CMC: 8},
			{Name: "Mid Combo", CMC: 4},
			{Name: "Cheap Combo A", CMC: 2},
			{Name: "Cheap Combo B", CMC: 2},
			{Name: "Cheap Combo C", CMC: 1},
		},
		BracketRationale: &BracketRationale{FinalBracket: 4, FinalLabel: "Optimized"},
	}
	appendVulnerabilityBracketNote(dp)

	note := findVulnerabilityNote(dp.BracketRationale)
	if note == nil {
		t.Fatalf("expected vulnerability note")
	}
	if len(note.Evidence) != 3 {
		t.Errorf("Evidence length = %d, want 3 (cap)", len(note.Evidence))
	}
	if !strings.Contains(note.Note, "Big Bomb") {
		t.Errorf("note should mention Big Bomb (top of list), got: %q", note.Note)
	}
	if !strings.Contains(note.Note, "+2 more") {
		t.Errorf("note should mention remaining count as '+2 more', got: %q", note.Note)
	}
	// Singular/plural correctness with 5 pieces.
	if !strings.Contains(note.Note, "5 combo pieces") {
		t.Errorf("note should pluralize correctly, got: %q", note.Note)
	}
}

// TestAppendVulnerabilityBracketNote_SingularForOnePiece verifies the
// grammar branch for the 1-piece case.
func TestAppendVulnerabilityBracketNote_SingularForOnePiece(t *testing.T) {
	dp := &DeckProfile{
		MeasuredBracket: 4,
		VulnerableComboPieces: []VulnerableComboPiece{
			{Name: "Lone Combo", CMC: 3},
		},
		BracketRationale: &BracketRationale{FinalBracket: 4, FinalLabel: "Optimized"},
	}
	appendVulnerabilityBracketNote(dp)
	note := findVulnerabilityNote(dp.BracketRationale)
	if note == nil {
		t.Fatalf("expected vulnerability note")
	}
	if !strings.Contains(note.Note, "1 combo piece lacks") {
		t.Errorf("singular grammar wrong, got: %q", note.Note)
	}
	if strings.Contains(note.Note, "+0 more") {
		t.Errorf("should NOT have '+0 more' suffix for single-piece case, got: %q", note.Note)
	}
}
