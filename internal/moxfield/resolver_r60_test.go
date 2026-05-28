package moxfield

import (
	"reflect"
	"testing"
)

// resolver_r60_test pins the pure-logic helpers in resolver.go:
// splitTypeLine, fields, extractColors. The full Resolve() pipeline
// requires a DB+oracle and is exercised separately by the moxfield
// integration tests; these unit tests close the 0% coverage gap on
// the three pure helpers without dragging in test infrastructure.

// TestSplitTypeLine_HandlesBothSeparators verifies the function
// normalizes both em-dash (Scryfall's canonical separator) and ASCII
// hyphen (legacy / hand-curated decks). Same input under both
// separators must produce the same types + subtypes split.
func TestSplitTypeLine_HandlesBothSeparators(t *testing.T) {
	cases := []struct {
		input         string
		wantTypes     []string
		wantSubtypes  []string
	}{
		{"Legendary Creature — Human Wizard", []string{"Legendary", "Creature"}, []string{"Human", "Wizard"}},
		{"Legendary Creature - Human Wizard", []string{"Legendary", "Creature"}, []string{"Human", "Wizard"}}, // ASCII variant
		{"Instant", []string{"Instant"}, nil},                                                                   // no subtype
		{"Artifact — Equipment", []string{"Artifact"}, []string{"Equipment"}},
		{"", nil, nil}, // empty input
	}
	for _, tc := range cases {
		gotTypes, gotSubtypes := splitTypeLine(tc.input)
		if !slicesEqual(gotTypes, tc.wantTypes) {
			t.Errorf("splitTypeLine(%q) types = %v, want %v", tc.input, gotTypes, tc.wantTypes)
		}
		if !slicesEqual(gotSubtypes, tc.wantSubtypes) {
			t.Errorf("splitTypeLine(%q) subtypes = %v, want %v", tc.input, gotSubtypes, tc.wantSubtypes)
		}
	}
}

// TestFields_TrimsAndDropsEmpties verifies that fields() splits on
// whitespace, trims, and drops empty tokens — the no-subtype path
// (splitTypeLine returns nil) and the empty-input cases below all rely
// on fields() returning [] not nil-with-empty-string entries.
func TestFields_TrimsAndDropsEmpties(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"", []string{}},
		{"   ", []string{}},
		{"Creature", []string{"Creature"}},
		{"  Legendary    Creature  ", []string{"Legendary", "Creature"}},
		{"\tHuman\tWizard\n", []string{"Human", "Wizard"}},
	}
	for _, tc := range cases {
		got := fields(tc.input)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("fields(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestExtractColors_BasicAndHybrid pins the canonical mana-cost parse:
// plain WUBRG symbols, hybrid pips (W/U → both), generic/X/numeric
// symbols ignored, deduped output preserving first-seen order.
func TestExtractColors_BasicAndHybrid(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"{2}{U}{B}", []string{"U", "B"}},
		{"{W}{W}", []string{"W"}},                  // dedup
		{"{W/U}{B/R}", []string{"W", "U", "B", "R"}}, // hybrid expansion
		{"{X}{1}{R}", []string{"R"}},                // numeric/X ignored
		{"{2}", nil},                                // colorless
		{"", nil},                                   // empty
		{"{u}{b}", []string{"U", "B"}},              // lowercase normalized
		{"{C}", nil},                                // colorless mana symbol skipped
	}
	for _, tc := range cases {
		got := extractColors(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("extractColors(%q) len = %d, want %d (got %v)", tc.input, len(got), len(tc.want), got)
			continue
		}
		// Order matters for first-seen semantics in the production path.
		if !slicesEqual(got, tc.want) {
			t.Errorf("extractColors(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

// TestExtractColors_PreservesFirstSeenOrder pins the ordering contract
// — the slice carries colors in the order they first appear in the
// mana cost (not WUBRG-canonical), which downstream display layers
// rely on for "first color" semantics.
func TestExtractColors_PreservesFirstSeenOrder(t *testing.T) {
	got := extractColors("{R}{G}{W}")
	want := []string{"R", "G", "W"}
	if !slicesEqual(got, want) {
		t.Errorf("extractColors first-seen order broken: got %v, want %v", got, want)
	}
}

// slicesEqual is a small helper that treats nil and empty as equal,
// matching the behavior the test fixtures expect.
func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestResolve_NilInputErrors verifies the public Resolve API rejects a
// nil ParsedDecklist with a clear error rather than panicking. Hits
// the first guard branch of Resolve, which the audit flagged as never
// exercised. (The full happy path needs a DB + oracle and is covered
// elsewhere.)
func TestResolve_NilInputErrors(t *testing.T) {
	res, err := Resolve(nil, nil, nil, "", "", "")
	if err == nil {
		t.Fatalf("expected error for nil parsed decklist")
	}
	if res != nil {
		t.Errorf("expected nil ResolveResult on error, got %+v", res)
	}
}

// TestResolve_MissingCommanderErrors verifies the second guard: a
// non-nil parsed decklist with no commander entries (and no
// commanderName override) returns the "no commander found" error.
func TestResolve_MissingCommanderErrors(t *testing.T) {
	res, err := Resolve(nil, nil, &ParsedDecklist{}, "", "", "")
	if err == nil {
		t.Fatalf("expected error for empty commander")
	}
	if res != nil {
		t.Errorf("expected nil ResolveResult on error, got %+v", res)
	}
}

