package deckparser

import (
	"bytes"
	"strings"
	"testing"
)

// suggest_r60_test.go — autosuggest unit tests.
//
// Verifies MetaDB.SuggestSimilarNames returns the closest Levenshtein
// match for common deckbuilder typo patterns (transpose, drop, insert,
// substitute) and pins the threshold semantics so future tightening
// doesn't silently break the "Did you mean X?" UI line.
//
// Casing is intentionally NOT a suggest-path case — normalizeName
// already lowercases + accent-folds before meta lookup, so
// "lightning bolt" resolves directly. The casing test below pins
// that contract: a casing-only mismatch must NOT reach the suggest
// path. Abbreviations are also OUT of scope — pure Levenshtein on
// "BoP" → "Birds of Paradise" is 14 edits, well past the threshold,
// and acronym matching is a different feature.

func suggestMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Lightning Bolt", "Sol Ring", "Counterspell", "Force of Will",
		"Birds of Paradise", "Llanowar Elves", "Wood Elves",
		"Cyclonic Rift", "Demonic Tutor", "Vampiric Tutor",
		"Path to Exile", "Swords to Plowshares",
		"Snapcaster Mage", "Tarmogoyf", "Goblin Guide",
		"Brainstorm", "Ponder", "Preordain", "Cultivate",
		"Forest", "Mountain", "Island", "Plains", "Swamp",
		"Atraxa, Praetors' Voice",
		// A few near-collisions so the suggester picks the right one.
		"Lightning Helix", // 6 edits from Bolt
		"Sol's Ring",      // 2 edits from Sol Ring (apostrophe variant)
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, Types: []string{"generic"}, CMC: 1,
		}
	}
	return meta
}

// TestSuggest_TypoSingleEdit — the most common deckbuilder typo: a
// single character missing / extra / swapped. Each input is exactly
// 1 Levenshtein edit from a real card name.
func TestSuggest_TypoSingleEdit(t *testing.T) {
	meta := suggestMeta()
	cases := []struct {
		typo string
		want string
		dist int
	}{
		{"Sol Rin", "Sol Ring", 1},          // missing trailing g
		{"Forst", "Forest", 1},              // missing e
		{"Counterspel", "Counterspell", 1},  // missing trailing l
		{"Cyconic Rift", "Cyclonic Rift", 1}, // missing l
		{"Mountian", "Mountain", 2},          // transpose (ai → ia) counts as 2 edits in classic Levenshtein
	}
	for _, tc := range cases {
		got := meta.SuggestSimilarNames(tc.typo, 3)
		if len(got) == 0 {
			t.Errorf("SuggestSimilarNames(%q): got 0 suggestions; want %q at dist %d", tc.typo, tc.want, tc.dist)
			continue
		}
		if got[0].Name != tc.want {
			t.Errorf("SuggestSimilarNames(%q): top suggestion = %q (dist %d), want %q (dist %d)",
				tc.typo, got[0].Name, got[0].Distance, tc.want, tc.dist)
		}
		if got[0].Distance != tc.dist {
			t.Errorf("SuggestSimilarNames(%q): top suggestion distance = %d, want %d",
				tc.typo, got[0].Distance, tc.dist)
		}
	}
}

// TestSuggest_TypoMultiEdit — the canonical "Lighting Blot" pattern
// from the task spec: 3 edits from "Lightning Bolt" (drop n, swap o↔l).
// Verifies the threshold accommodates 3-edit inputs at the longer
// end of the per-input scaling curve.
func TestSuggest_TypoMultiEdit(t *testing.T) {
	meta := suggestMeta()
	got := meta.SuggestSimilarNames("Lighting Blot", 3)
	if len(got) == 0 {
		t.Fatalf("SuggestSimilarNames(\"Lighting Blot\"): got 0 suggestions; want Lightning Bolt")
	}
	if got[0].Name != "Lightning Bolt" {
		t.Errorf("top suggestion = %q (dist %d), want %q", got[0].Name, got[0].Distance, "Lightning Bolt")
	}
	if got[0].Distance > 4 {
		t.Errorf("distance = %d, want ≤ 4 (canonical task-spec example)", got[0].Distance)
	}
}

// TestSuggest_CasingResolvesDirectly — a casing-only mismatch must
// resolve via the direct meta lookup (normalizeName lowercases both
// sides) and never reach the suggest path. Verified by parsing a
// deck and checking the line resolved cleanly, no suggestion needed.
func TestSuggest_CasingResolvesDirectly(t *testing.T) {
	meta := suggestMeta()
	text := `COMMANDER: Atraxa, Praetors' Voice
1 LIGHTNING BOLT
1 sol ring
1 Cyclonic RIFT
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if td.ParseReport.UnresolvedLines != 0 {
		t.Errorf("Unresolved = %d, want 0 (casing should resolve via normalizeName); details=%+v",
			td.ParseReport.UnresolvedLines, td.ParseReport.UnresolvedDetails)
	}
	if len(td.Library) != 3 {
		t.Errorf("Library: want 3 cards, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
}

// TestSuggest_ReturnsTopN — when multiple meta names cluster near the
// input, the suggester returns the top N in ascending-distance order.
// `Cyclonc Rif` is close to both `Cyclonic Rift` and (less close) to
// other words but only one match should win on distance.
func TestSuggest_ReturnsTopN(t *testing.T) {
	meta := suggestMeta()
	got := meta.SuggestSimilarNames("Solv Ring", 3)
	if len(got) == 0 {
		t.Fatalf("got 0 suggestions; want Sol Ring as top")
	}
	if got[0].Name != "Sol Ring" {
		t.Errorf("top suggestion = %q, want %q", got[0].Name, "Sol Ring")
	}
	// Sol's Ring is 2 edits from "Solv Ring" (drop v, insert '), should
	// also show up if within threshold.
	for _, s := range got {
		if s.Distance < 0 {
			t.Errorf("negative distance in %+v", s)
		}
	}
	// Suggestions are in ascending-distance order.
	for i := 1; i < len(got); i++ {
		if got[i].Distance < got[i-1].Distance {
			t.Errorf("suggestions not sorted: %+v", got)
		}
	}
}

// TestSuggest_NoMatchReturnsNil — gibberish input that's far from any
// meta entry returns nil (not an empty slice with garbage). 8+ edits
// from anything in suggestMeta.
func TestSuggest_NoMatchReturnsNil(t *testing.T) {
	meta := suggestMeta()
	got := meta.SuggestSimilarNames("Zyxqwerasdf Nonexistent Garbage", 3)
	if got != nil {
		t.Errorf("SuggestSimilarNames(gibberish): got %v, want nil", got)
	}
}

// TestSuggest_NilMetaReturnsNil — defensive nil-receiver handling so
// callers in the parse loop don't crash when meta is unavailable.
func TestSuggest_NilMetaReturnsNil(t *testing.T) {
	var meta *MetaDB
	if got := meta.SuggestSimilarNames("Sol Ring", 3); got != nil {
		t.Errorf("nil-receiver SuggestSimilarNames: got %v, want nil", got)
	}
}

// TestSuggest_ShortInputSkipped — inputs ≤ 3 chars produce no
// suggestions (too noisy — every 3-char input matches dozens of card
// names within 2 edits). Documents the suggestionThreshold's zero-
// return for short inputs.
func TestSuggest_ShortInputSkipped(t *testing.T) {
	meta := suggestMeta()
	for _, in := range []string{"X", "Bo", "Sol"} {
		if got := meta.SuggestSimilarNames(in, 3); got != nil {
			t.Errorf("SuggestSimilarNames(%q): got %v, want nil (too short to suggest)", in, got)
		}
	}
}

// TestSuggest_ExactMatchExcluded — if `input` normalizes to a name
// already in meta (which shouldn't happen because the caller probed
// meta.Get first), the suggester filters it out. Defensive — without
// this guard a duplicate-key meta or a case-folded re-probe could
// re-suggest the input as a "match".
func TestSuggest_ExactMatchExcluded(t *testing.T) {
	meta := suggestMeta()
	got := meta.SuggestSimilarNames("Sol Ring", 3)
	for _, s := range got {
		if normalizeName(s.Name) == normalizeName("Sol Ring") {
			t.Errorf("exact-match self-suggestion leaked: %+v", s)
		}
	}
}

// TestSuggest_LongInputRespectsThresholdCap — a 50-char input with
// many edits should NOT sweep the meta. The threshold caps at 4 even
// for long inputs.
func TestSuggest_LongInputRespectsThresholdCap(t *testing.T) {
	meta := suggestMeta()
	// 50-char input, 20+ edits from any meta entry.
	long := "ThisIsAVeryLongStringThatShouldNotMatchAnythingInMeta"
	if got := meta.SuggestSimilarNames(long, 3); got != nil {
		t.Errorf("long-gibberish suggest: got %v, want nil (threshold cap at 4 should keep nothing in range)", got)
	}
}

// TestParseReport_UnresolvedCarriesSuggestions — end-to-end through
// the parse loop. An unresolved card in a deck gets the Suggestions
// field populated on its UnresolvedLine.
func TestParseReport_UnresolvedCarriesSuggestions(t *testing.T) {
	meta := suggestMeta()
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Lighting Blot
1 Sol Rin
1 Forst
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if td.ParseReport.UnresolvedLines != 3 {
		t.Fatalf("Unresolved = %d, want 3", td.ParseReport.UnresolvedLines)
	}
	wantTop := map[string]string{
		"Lighting Blot": "Lightning Bolt",
		"Sol Rin":       "Sol Ring",
		"Forst":         "Forest",
	}
	for _, u := range td.ParseReport.UnresolvedDetails {
		if len(u.Suggestions) == 0 {
			t.Errorf("UnresolvedLine %+v: no suggestions populated", u)
			continue
		}
		want := wantTop[u.Name]
		if u.Suggestions[0].Name != want {
			t.Errorf("UnresolvedLine %q: top suggestion = %q, want %q",
				u.Name, u.Suggestions[0].Name, want)
		}
	}
}

// TestPrintReport_RendersDidYouMean — the report output includes the
// "Did you mean X? (N char edits from \"Y\")" line for every
// unresolved with at least one suggestion. Verifies the canonical
// phrasing from the task spec.
func TestPrintReport_RendersDidYouMean(t *testing.T) {
	meta := suggestMeta()
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Lighting Blot
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := td.PrintReport(&buf); err != nil {
		t.Fatalf("PrintReport: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Did you mean Lightning Bolt?") {
		t.Errorf("output missing canonical \"Did you mean Lightning Bolt?\" line; full output:\n%s", out)
	}
	if !strings.Contains(out, "char edits from \"Lighting Blot\"") {
		t.Errorf("output missing edit-distance phrasing; full output:\n%s", out)
	}
}

// TestPrintReport_SingularEditFormat — 1-edit suggestions render as
// "1 char edit" (singular), not "1 char edits". Small UX detail but
// pins the phrasing branch.
func TestPrintReport_SingularEditFormat(t *testing.T) {
	meta := suggestMeta()
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Rin
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	_ = td.PrintReport(&buf)
	if !strings.Contains(buf.String(), "1 char edit ") {
		t.Errorf("output missing singular \"1 char edit\" phrasing; got:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "1 char edits") {
		t.Errorf("output uses plural \"edits\" for distance=1; got:\n%s", buf.String())
	}
}

// TestLevenshteinBounded_EarlyExit — the bounded variant returns
// max+1 (not the true distance) when the running min crosses the
// threshold. Defends the perf optimization that skips far-apart
// pairs in O(min(M,N)) rather than O(M*N).
func TestLevenshteinBounded_EarlyExit(t *testing.T) {
	// "abc" vs "xyz123456789" — actual distance is 12, max=3 → early-exit returns 4.
	got := levenshteinBounded("abc", "xyz123456789", 3)
	if got <= 3 {
		t.Errorf("levenshteinBounded(\"abc\", \"xyz123456789\", 3) = %d, want > 3", got)
	}
	// Same pair with high max returns the true distance.
	got = levenshteinBounded("abc", "xyz123456789", 100)
	if got != 12 {
		t.Errorf("levenshteinBounded(\"abc\", \"xyz123456789\", 100) = %d, want 12", got)
	}
}

// TestLevenshteinBounded_UnicodeSafe — card names contain Æ, ö, û, é,
// etc. The bounded computation must use rune-counting (not byte-counting)
// so a 4-rune word doesn't read as 5-bytes. Verifies with the canonical
// Magic accent characters that appear in real card names.
func TestLevenshteinBounded_UnicodeSafe(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"Æther Vial", "Aether Vial", 2},   // Æ is 1 rune; "Aether" inserts both A and e = 2 edits
		{"Jötun Grunt", "Jotun Grunt", 1}, // 1 substitution (ö → o)
		{"Séance", "Seance", 1},            // 1 substitution
		{"Lim-Dûl's Vault", "Lim-Dul's Vault", 1},
	}
	for _, tc := range cases {
		got := levenshteinBounded(tc.a, tc.b, 5)
		if got != tc.want {
			t.Errorf("levenshteinBounded(%q, %q) = %d, want %d (rune count vs byte count)", tc.a, tc.b, got, tc.want)
		}
	}
}
