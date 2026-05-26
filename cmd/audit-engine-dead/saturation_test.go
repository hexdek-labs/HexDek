package main

import "testing"

// R60 Phase 1D residue fix #5 — saturation tests.
//
// After PRs #486 / #491 / #494 / #516, every finding in the
// unused_switch_case_literals audit is classified as an expected
// false positive (card-name JSON dispatch or AST-enum from
// scripts/mtg_ast.py). The audit-tool's `classifyCase` rule formalises
// that conclusion and emits a saturation summary at the top of the
// report. These tests pin the rule so future heuristic edits can't
// silently widen the exemption or drop a documented arm.

func TestClassifyTag_CardNameBucket(t *testing.T) {
	cases := []string{
		"name",
		"p.Card.Name",
		"perm.Card.DisplayName",
		"p.Card.DisplayName(…)", // the formatted form the audit emits for method calls
	}
	for _, tag := range cases {
		if got := classifyTag(tag); got != "card-name" {
			t.Errorf("classifyTag(%q) = %q; want card-name", tag, got)
		}
	}
}

func TestClassifyTag_ASTEnumBucket(t *testing.T) {
	cases := []string{
		"e.ModKind",
		"mod.ModKind",
		"st.Modification.ModKind",
		"sa.ScalingKind",
		"f.Base",
		"base",
		"e.Query.Base",
		"e.Actor",
		"f.Quantifier",
		"t.Controller",
		"exLow",
		"prefix",
		"f.Extra",
	}
	for _, tag := range cases {
		if got := classifyTag(tag); got != "ast-enum" {
			t.Errorf("classifyTag(%q) = %q; want ast-enum", tag, got)
		}
	}
}

func TestClassifyTag_HighSignalFallthrough(t *testing.T) {
	// Tags that don't match either substring set must fall through to
	// "high-signal" so a genuinely new dead-branch finding shows up in
	// the saturation summary.
	cases := []string{
		"event.Kind",       // engine event kind — note `kind` is matched by tagInterpretation's "kind/event" arm, but NOT by classifyCase (which only matches "modkind"/"scalingkind"). High-signal.
		"someUnknownLocal", // genuinely unclassified
		"t",                // hashaton's pip-stripping loop variable — classifies as high-signal at the TAG level; the per-arm `pip:C` exception fires only at classifyCase().
	}
	for _, tag := range cases {
		if got := classifyTag(tag); got != "high-signal" {
			t.Errorf("classifyTag(%q) = %q; want high-signal", tag, got)
		}
	}
}

func TestClassifyCase_DocumentedPipExceptionTakesPrecedenceOverHighSignal(t *testing.T) {
	// The `pip:C` arm on hashaton's `for _, t := range token.Types`
	// loop has tag `t` (high-signal at the tag level) but a documented
	// arm-value exception. classifyCase must route it to
	// "high-signal-documented", NOT "high-signal".
	c := CaseLiteral{Value: "pip:C", SwitchTag: "t"}
	if got := classifyCase(c); got != "high-signal-documented" {
		t.Errorf("classifyCase({tag=t, value=pip:C}) = %q; want high-signal-documented", got)
	}
	// A different arm value with the same tag is NOT covered by the
	// exception — must remain high-signal so a new dead arm surfaces.
	c2 := CaseLiteral{Value: "pip:Y", SwitchTag: "t"}
	if got := classifyCase(c2); got != "high-signal" {
		t.Errorf("classifyCase({tag=t, value=pip:Y}) = %q; want high-signal (only pip:C is the documented exception)", got)
	}
}

func TestDocumentedHighSignalArms_PRReferences(t *testing.T) {
	// Every documented exception MUST cite the PR it landed in so the
	// investigation trail is preserved. Pin the map to ensure new
	// entries follow the convention.
	if len(documentedHighSignalArms) == 0 {
		t.Skip("no documented exceptions; nothing to pin")
	}
	for key, ref := range documentedHighSignalArms {
		if ref == "" {
			t.Errorf("documented arm %v has empty PR reference", key)
		}
	}
}
