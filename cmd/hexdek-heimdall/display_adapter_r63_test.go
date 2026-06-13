package main

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
)

// display_adapter_r63_test.go — round-2 step-4 pin: Heimdall's
// spectator violation display is a thin adapter over the canonical
// judge.ValidationViolation (gameengine.InvariantViolation is an alias
// of it). The Judge is the source of truth: the display renders the
// vocabulary's own String() form and NEVER re-logs through the router
// (origin logging happens inside RunAllInvariants).

func TestFormatViolation_RendersCanonicalVocabulary(t *testing.T) {
	// The function signature itself pins the one-vocabulary contract:
	// it takes gameengine.InvariantViolation, which must remain an
	// alias of judge.ValidationViolation.
	var v gameengine.InvariantViolation = judge.ValidationViolation{
		Surface:   judge.SurfaceInvariants,
		Name:      "ZoneConservation",
		Severity:  judge.SeverityCritical,
		Message:   "seat 3 census reads 4 cards low",
		Dimension: judge.DimensionConservation,
		Seat:      3,
	}

	out := formatViolation("after R7.2 SBA", v)

	// The body must be the canonical rendering — what every other
	// Judge consumer sees — not a hand-rolled Name/Message duplicate.
	if !strings.Contains(out, v.String()) {
		t.Fatalf("display must use the canonical String() rendering;\ngot: %s\nwant substring: %s", out, v.String())
	}
	for _, want := range []string{"ZoneConservation", "conservation", "after R7.2 SBA", "[critical]", "[seat 3]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("display missing %q:\n%s", want, out)
		}
	}
}

func TestFormatViolation_DoesNotReLog(t *testing.T) {
	var routed []judge.ValidationViolation
	unregister := judge.RegisterSink(func(v judge.ValidationViolation) {
		routed = append(routed, v)
	})
	defer unregister()

	formatViolation("ctx", judge.ValidationViolation{
		Surface: judge.SurfaceInvariants, Name: "X",
		Severity: judge.SeverityCritical, Message: "m",
	})

	if len(routed) != 0 {
		t.Fatalf("display adapter re-logged through the router (%d violations) — origin logging belongs to RunAllInvariants alone", len(routed))
	}
}
