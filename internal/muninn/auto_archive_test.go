package muninn

import (
	"testing"

	"github.com/hexdek/hexdek/internal/judge"
)

// r63 Judge fold: AutoArchiveViolation takes canonical judge violations
// (the stringify-then-reparse path and muninn's InvariantViolation
// vocabulary copy are deleted; ArchivedViolation is the storage row).

func TestAutoArchiveViolation_AppendsCanonical(t *testing.T) {
	dir := t.TempDir()
	deckKeys := [4]string{"alpha", "beta", "gamma", "delta"}
	violations := []judge.ValidationViolation{
		{Surface: judge.SurfaceFeynman, Dimension: judge.DimensionStateIntegrity,
			Name: "704.5a", Severity: judge.SeverityCritical, Seat: 1,
			Message: "seat 1 has -3 life but is not marked lost"},
		{Surface: judge.SurfaceInvariants, Dimension: judge.DimensionConservation,
			Name: "ZoneConservation", Severity: judge.SeverityCritical,
			Message: "card disappeared"},
	}

	if err := AutoArchiveViolation(dir, 1234, deckKeys, violations); err != nil {
		t.Fatalf("AutoArchiveViolation: %v", err)
	}

	got, err := ReadInvariantViolations(dir)
	if err != nil {
		t.Fatalf("ReadInvariantViolations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
	if got[0].GameSeed != 1234 || got[0].DeckKeys != deckKeys {
		t.Errorf("record 0 metadata wrong: %+v", got[0])
	}
	if got[0].ViolationType != "704.5a" {
		t.Errorf("record 0: ViolationType=%q, want 704.5a (canonical Name, no string parsing)", got[0].ViolationType)
	}
	if got[0].Message != violations[0].String() {
		t.Errorf("record 0: Message=%q, want canonical rendering %q", got[0].Message, violations[0].String())
	}
	if got[0].Timestamp == "" {
		t.Error("record 0: Timestamp is empty")
	}
	if got[1].ViolationType != "ZoneConservation" {
		t.Errorf("record 1: ViolationType=%q, want ZoneConservation", got[1].ViolationType)
	}
}

func TestAutoArchiveViolation_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	deckA := [4]string{"a", "b", "c", "d"}
	deckB := [4]string{"e", "f", "g", "h"}

	v1 := []judge.ValidationViolation{{Name: "704.5f", Severity: judge.SeverityCritical, Message: "toughness 0"}}
	v2 := []judge.ValidationViolation{{Name: "704.5c", Severity: judge.SeverityCritical, Message: "poison 12"}}

	if err := AutoArchiveViolation(dir, 1, deckA, v1); err != nil {
		t.Fatalf("first archive: %v", err)
	}
	if err := AutoArchiveViolation(dir, 2, deckB, v2); err != nil {
		t.Fatalf("second archive: %v", err)
	}

	got, err := ReadInvariantViolations(dir)
	if err != nil {
		t.Fatalf("ReadInvariantViolations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records after two appends, got %d", len(got))
	}
	if got[0].GameSeed != 1 || got[1].GameSeed != 2 {
		t.Errorf("seed order wrong: %d, %d", got[0].GameSeed, got[1].GameSeed)
	}
}

func TestAutoArchiveViolation_EmptyIsNoop(t *testing.T) {
	dir := t.TempDir()
	deck := [4]string{}

	if err := AutoArchiveViolation(dir, 99, deck, nil); err != nil {
		t.Fatalf("nil slice: %v", err)
	}
	if err := AutoArchiveViolation(dir, 99, deck, []judge.ValidationViolation{}); err != nil {
		t.Fatalf("empty slice: %v", err)
	}

	got, err := ReadInvariantViolations(dir)
	if err != nil {
		t.Fatalf("ReadInvariantViolations: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no records, got %d", len(got))
	}
}
