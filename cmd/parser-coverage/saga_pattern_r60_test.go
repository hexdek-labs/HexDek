package main

import "testing"

// findFailurePattern looks up a failurePatterns entry by name.
func findFailurePattern(t *testing.T, name string) func(text, typeLine string) bool {
	t.Helper()
	for _, p := range failurePatterns {
		if p.name == name {
			return p.match
		}
	}
	t.Fatalf("failure pattern %q not registered in failurePatterns", name)
	return nil
}

// TestSagaOrClass_TypeLineSubtypeFires pins the new type-line path on
// the saga_or_class bucket. A Saga that empty-ASTs upstream of its
// chapter parsing would have minimal oracle text, but its type_line
// always carries the "— Saga" subtype. The matcher now recognizes
// that signal, so such failures bucket correctly instead of falling
// through to "other".
func TestSagaOrClass_TypeLineSubtypeFires(t *testing.T) {
	match := findFailurePattern(t, "saga_or_class")
	cases := []struct {
		name     string
		oracle   string
		typeLine string
	}{
		{"canonical_saga", "", "Enchantment — Saga"},
		{"saga_creature", "", "Enchantment Creature — Saga Knight"},
		{"saga_land", "", "Enchantment Land — Urza's Saga"},
		{"dfc_back_face_saga", "", "Legendary Creature — Human Noble Knight // Legendary Enchantment Creature — Saga Dragon"},
		{"canonical_class", "", "Enchantment — Class"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !match(tc.oracle, tc.typeLine) {
				t.Errorf("saga_or_class should match type_line=%q with empty oracle", tc.typeLine)
			}
		})
	}
}

// TestSagaOrClass_OracleFallbackPreserved defends the legacy oracle-
// text path. Pre-fix, the matcher relied solely on these substrings
// to detect chapter markers; the type-line check is additive, not
// replacing.
func TestSagaOrClass_OracleFallbackPreserved(t *testing.T) {
	match := findFailurePattern(t, "saga_or_class")
	cases := []struct {
		oracle, typeLine string
	}{
		{"I — Create a 1/1 Spirit token.", ""},
		{"II — Draw a card.", ""},
		{"Exile this Saga.", ""},
		{"Whenever this Class becomes a higher level, draw a card.", ""},
	}
	for i, tc := range cases {
		if !match(tc.oracle, tc.typeLine) {
			t.Errorf("case %d: saga_or_class should still match oracle=%q via legacy fallback", i, tc.oracle)
		}
	}
}

// TestSagaOrClass_RejectsUnrelated defends against the type-line
// check picking up cards whose names happen to contain "saga" or
// "class" by accident — type lines don't include card names, so a
// hypothetical card named "Saga of X" wouldn't match unless its
// type line included the subtype.
func TestSagaOrClass_RejectsUnrelated(t *testing.T) {
	match := findFailurePattern(t, "saga_or_class")
	cases := []struct {
		oracle, typeLine string
	}{
		{"Lightning Bolt deals 3 damage to any target.", "Instant"},
		{"Vanilla creature stat-line text.", "Creature — Bear"},
		{"({T}: Add {R} or {G}.)", "Land — Mountain Forest"},
	}
	for i, tc := range cases {
		if match(tc.oracle, tc.typeLine) {
			t.Errorf("case %d: saga_or_class must NOT match oracle=%q tl=%q", i, tc.oracle, tc.typeLine)
		}
	}
}

// TestClassifyPattern_PropagatesTypeLine pins the load-bearing
// behavioral fix in classifyPattern. Pre-fix it shadowed tl to "",
// so every type-line-only matcher (battle, emblem_or_planeswalker,
// and the new saga_or_class subtype path) silently never fired. This
// test asserts the propagation end-to-end via a Saga with empty
// oracle text — the only signal is on the type line.
func TestClassifyPattern_PropagatesTypeLine(t *testing.T) {
	r := result{
		Name:       "Test Saga",
		Class:      classEmptyAST,
		OracleText: "",
		TypeLine:   "Enchantment — Saga",
	}
	got := classifyPattern(r)
	if got != "saga_or_class" {
		t.Errorf("classifyPattern: want \"saga_or_class\" (via type-line propagation), got %q", got)
	}
}

// TestClassifyPattern_BattleNowFires is the sibling fix-confirmation:
// the "battle" failure-pattern bucket matches only on type_line, so
// it was completely dead pre-fix. Confirm it fires now.
func TestClassifyPattern_BattleNowFires(t *testing.T) {
	r := result{
		Name:       "Hypothetical Battle Failure",
		Class:      classEmptyAST,
		OracleText: "",
		TypeLine:   "Battle — Siege",
	}
	got := classifyPattern(r)
	if got != "battle" {
		t.Errorf("classifyPattern: want \"battle\" (via type-line propagation), got %q", got)
	}
}

// TestSagaMatcher_TypeLineCanonical sanity-pins the by-mechanic
// matcher (separate from the failure-pattern bucket). The Saga
// mechanic-coverage matcher is typeLineMatcher("saga"), which catches
// every saga subtype including DFC back-face Sagas where the
// top-level Scryfall type_line includes both faces joined by " // ".
func TestSagaMatcher_TypeLineCanonical(t *testing.T) {
	match := findMatcher(t, "saga")
	cases := []result{
		{TypeLine: "Enchantment — Saga"},
		{TypeLine: "Enchantment Creature — Saga Knight"},
		{TypeLine: "Legendary Creature — Phyrexian Praetor // Enchantment — Saga"},
	}
	for _, r := range cases {
		if !match(r) {
			t.Errorf("saga mechanic matcher must match TypeLine=%q", r.TypeLine)
		}
	}
}
