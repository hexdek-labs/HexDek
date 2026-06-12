package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/judge"
)

// legality_dimension_r63_test.go — LEGALITY fold: the ride-along
// validator's canonical emissions carry Dimension=legality, so the
// Judge's per-dimension view sees both halves (deck construction in
// internal/judge/legality.go + in-game action legality here) under one
// tag.
func TestLegalityViolation_CanonicalCarriesDimension(t *testing.T) {
	v := LegalityViolation{
		Seed: 42, Turn: 3, Seat: 1,
		Action: "cast:Lightning Bolt", Rule: "307.1",
		Detail: "sorcery-speed cast during opponent's turn",
	}
	c := v.Canonical()
	if c.Dimension != judge.DimensionLegality {
		t.Fatalf("Dimension = %q, want %q", c.Dimension, judge.DimensionLegality)
	}
	if c.Surface != judge.SurfaceLegality || c.Name != "307.1" {
		t.Fatalf("Surface/Name = %q/%q", c.Surface, c.Name)
	}
}

// TestLegalityValidator_RecordRoutesTaggedViolation — end-to-end: a
// validator record() lands in a registered Judge sink with the
// dimension tag intact.
func TestLegalityValidator_RecordRoutesTaggedViolation(t *testing.T) {
	var got []judge.ValidationViolation
	unreg := judge.RegisterSink(func(v judge.ValidationViolation) {
		got = append(got, v)
	})
	defer unreg()

	v := NewLegalityValidator(99)
	v.MaxViolations = 10
	v.record(nil, LegalityViolation{
		Turn: 5, Seat: 2, Action: "activate:Test#0",
		Rule: "605.3a", Detail: "mana ability put on the stack",
	})

	if len(got) != 1 {
		t.Fatalf("sink saw %d violations, want 1", len(got))
	}
	if got[0].Dimension != judge.DimensionLegality {
		t.Fatalf("routed Dimension = %q, want legality", got[0].Dimension)
	}
}
