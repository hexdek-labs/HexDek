package main

import (
	"strings"
	"testing"
)

// TestTagInterpretation_ASTEnumBucketCoverage pins the substrings
// that route switch-tag findings into the "AST enum from
// scripts/mtg_ast.py" expected-false-positive bucket. Each addition
// to the bucket is sourced from a Phase 1D-residue investigation:
//
//   - modkind / scalingkind / base: initial PR #478
//   - actor / quantifier:           PR #486 (residue #1)
//   - controller:                   this PR (residue #2)
//
// If a future cleanup deletes one of these substrings from
// tagInterpretation, real engine arms keyed on the corresponding
// gameast.* field would re-surface as "verify the emitter is
// genuinely absent" — wasting investigator time on a known false-
// positive class. This test catches that.
func TestTagInterpretation_ASTEnumBucketCoverage(t *testing.T) {
	expectedASTLikeTags := []string{
		"e.ModKind",
		"mod.ModKind",
		"st.Modification.ModKind",
		"sa.ScalingKind",
		"f.Base",
		"e.Query.Base",
		"base",
		"e.Actor",
		"f.Quantifier",
		"t.Controller",   // R60 Phase 1D-residue #2
		"e.Controller",   // gameast.Effect.Controller, same family
		"trg.Controller", // hypothetical local-var name
	}
	for _, tag := range expectedASTLikeTags {
		got := tagInterpretation(tag)
		if !strings.Contains(got, "AST enum") {
			t.Errorf("tag %q should classify as AST-enum false positive; got %q", tag, got)
		}
	}
}

func TestTagInterpretation_HighSignalTagsNotMisclassified(t *testing.T) {
	// Genuinely high-signal tags must NOT route to either of the
	// expected-false-positive buckets. If a future maintainer broadens
	// the substring matches too aggressively, this catches the
	// regression.
	expectedHighSignal := []string{
		"event.Kind",   // engine event kind — high signal
		"evt.EventKind",
		"someOtherTag", // unrelated — falls through to "verify the emitter"
	}
	for _, tag := range expectedHighSignal {
		got := tagInterpretation(tag)
		if strings.Contains(got, "expected false positive") {
			t.Errorf("tag %q must NOT be classified as false positive; got %q", tag, got)
		}
	}
}
