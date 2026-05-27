package main

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/astload"
)

// buildEmptyAbilityCorpus assembles an astload.Corpus from a JSONL
// fixture where each card has zero abilities and fully_parsed=true.
// Mirrors the canonical "parser dropped reminder text" output shape we
// want classify() to recognize as OK_VANILLA.
func buildEmptyAbilityCorpus(t *testing.T, names ...string) *astload.Corpus {
	t.Helper()
	var b strings.Builder
	for _, n := range names {
		// Minimal AST: name + empty abilities + fully_parsed=true.
		b.WriteString(`{"name":"`)
		b.WriteString(n)
		b.WriteString(`","oracle_text":"","type_line":"","ast":{"__ast_type__":"CardAST","name":"`)
		b.WriteString(n)
		b.WriteString(`","abilities":[],"fully_parsed":true,"parse_errors":[]}}` + "\n")
	}
	c, err := astload.LoadReader(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("LoadReader: %v", err)
	}
	return c
}

// TestIsReminderTextOnly_DualLands pins the canonical motivating case:
// the ten Revised dual lands all share the single-parenthetical reminder-
// text shape. Pre-fix every one classified EMPTY_AST because their type
// line is "Land — Island Swamp" (not "Basic Land"), so isBasicLand
// returned false and the vanilla check fell through. Post-fix the
// reminder-only shape is recognized directly.
func TestIsReminderTextOnly_DualLands(t *testing.T) {
	duals := []string{
		"({T}: Add {U} or {B}.)",
		"({T}: Add {U} or {R}.)",
		"({T}: Add {G} or {U}.)",
		"({T}: Add {B} or {G}.)",
		"({T}: Add {W} or {U}.)",
		"({T}: Add {W} or {B}.)",
		"({T}: Add {B} or {R}.)",
		"({T}: Add {R} or {G}.)",
		"({T}: Add {G} or {W}.)",
		"({T}: Add {R} or {W}.)",
	}
	for _, txt := range duals {
		if !isReminderTextOnly(txt) {
			t.Errorf("dual reminder text %q should be detected as reminder-text-only", txt)
		}
	}
}

// TestIsReminderTextOnly_Negatives defends against false positives. A
// card whose body has any rules text outside the parentheses (the
// keyword-with-reminder shape, the multi-paragraph shape, the
// closing-paren-mid-text shape) must NOT be flagged — the parser is
// expected to produce abilities for the non-parenthetical part, so 0
// abilities really does indicate a gap there.
func TestIsReminderTextOnly_Negatives(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"empty", ""},
		{"whitespace_only", "   "},
		{"plain_rules_text", "Flying"},
		{"keyword_with_reminder", "Flying (This creature can't be blocked except by creatures with flying or reach.)"},
		{"two_reminders_with_text_between", "({T}: Add {U}.) Some rules text. ({T}: Add {R}.)"},
		{"closes_then_reopens", "(reminder one) plain text (reminder two)"},
		{"unbalanced_open", "({T}: Add"},
		{"unbalanced_close_only", "{T}: Add)"},
		{"starts_with_paren_but_text_after_close", "({T}: Add {U} or {B}.) extra"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if isReminderTextOnly(tc.text) {
				t.Errorf("text %q should NOT be flagged reminder-text-only", tc.text)
			}
		})
	}
}

// TestIsReminderTextOnly_NestedParens ensures the depth tracker handles a
// reminder body that contains a nested parenthetical (rare in printed
// Magic text but possible — e.g., a parenthesis inside a reminder
// example). Depth must close to zero only at the very end.
func TestIsReminderTextOnly_NestedParens(t *testing.T) {
	// Synthetic: "(outer (inner) outer continues)" — one balanced group
	// from start to finish.
	if !isReminderTextOnly("(outer (inner) outer continues)") {
		t.Error("nested-balanced parenthetical should still classify as reminder-only")
	}
	// Negative: depth temporarily goes negative — malformed text.
	if isReminderTextOnly("(stuff)) trailing") {
		t.Error("text with unbalanced close should not classify as reminder-only")
	}
}

// TestClassify_DualLandIsOKVanilla wires the change end-to-end through
// classify(). Underground Sea (type line "Land — Island Swamp", oracle
// text "({T}: Add {U} or {B}.)", parser produces 0 abilities) must
// classify OK_VANILLA — not EMPTY_AST.
func TestClassify_DualLandIsOKVanilla(t *testing.T) {
	corpus := buildEmptyAbilityCorpus(t, "Underground Sea")
	e := oracleEntry{
		Name:       "Underground Sea",
		TypeLine:   "Land — Island Swamp",
		OracleText: "({T}: Add {U} or {B}.)",
	}
	got := classify(e, corpus)
	if got.Class != classOKVanilla {
		t.Errorf("Underground Sea: want classOKVanilla, got %v", got.Class)
	}
}

// TestClassify_RealEmptyASTStillFires defends against the fix
// over-firing: a card with NON-reminder oracle text and 0 abilities must
// still classify EMPTY_AST. Stone-Throwing Devils — oracle "First
// strike", 0 abilities in the AST corpus — is the canonical real-gap
// case from the r41 baseline.
func TestClassify_RealEmptyASTStillFires(t *testing.T) {
	corpus := buildEmptyAbilityCorpus(t, "Stone-Throwing Devils")
	e := oracleEntry{
		Name:       "Stone-Throwing Devils",
		TypeLine:   "Creature — Human Cleric",
		OracleText: "First strike",
	}
	got := classify(e, corpus)
	if got.Class != classEmptyAST {
		t.Errorf("Stone-Throwing Devils: want classEmptyAST (real gap), got %v", got.Class)
	}
}
