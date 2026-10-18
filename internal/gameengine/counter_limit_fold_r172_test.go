package gameengine

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// -----------------------------------------------------------------------------
// r172 — containsSubstrFold pre-check for parseCounterLimit.
//
// The production parseCounterLimit gained a non-allocating case-insensitive
// "contains" pre-check. These tests pin that the pre-check is BEHAVIOUR
// PRESERVING: the optimised parser must agree with the pre-optimisation
// parser on every input, and the predicate itself must agree with
// strings.Contains(strings.ToLower(h), needle) in BOTH directions.
// -----------------------------------------------------------------------------

// parseCounterLimitLegacy is a verbatim copy of parseCounterLimit as it stood
// at 989772e2, before the containsSubstrFold pre-check was added. It is the
// differential oracle. Do not "optimise" it.
func parseCounterLimitLegacy(raw string) (maxN int, counterKind string) {
	lower := strings.ToLower(raw)
	idx := strings.Index(lower, "can't have more than ")
	if idx < 0 {
		return -1, ""
	}
	rest := lower[idx+len("can't have more than "):]
	parts := strings.Fields(rest)
	if len(parts) < 2 {
		return -1, ""
	}
	n, ok := counterLimitWords[parts[0]]
	if !ok {
		n64, err := strconv.Atoi(parts[0])
		if err != nil {
			return -1, ""
		}
		n = n64
	}
	kind := parts[1]
	if strings.HasSuffix(kind, "counters") || strings.HasSuffix(kind, "counter") {
		return -1, ""
	}
	return n, kind
}

// containsFoldOracle is the allocating reference the pre-check must match.
func containsFoldOracle(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), needle)
}

// counterLimitCorpus is the existing TestParseCounterLimit table (sba_test.go)
// plus adversarial cases: mixed case, Unicode, the needle split across a
// rune boundary, empty string, needle at the very start and very end.
func counterLimitCorpus() []string {
	const N = "can't have more than "
	return []string{
		// --- the existing TestParseCounterLimit table, verbatim ---
		"Rasputin can't have more than seven dream counters on it",
		"This can't have more than 3 charge counters on it",
		"This can't have more than ten ice counters on it",
		"Flying",
		"",

		// --- case variations ---
		"RASPUTIN CAN'T HAVE MORE THAN SEVEN DREAM COUNTERS ON IT",
		"Rasputin CaN'T hAvE mOrE tHaN seven dream counters on it",
		"CAN'T HAVE MORE THAN 2 ICE COUNTERS ON IT",

		// --- needle at the very start / very end ---
		N,
		N + "four quest counters on it",
		"trailing text then " + N,
		strings.TrimSuffix(N, " "), // "can't have more than" — no trailing space
		"x" + N,
		N + "x",

		// --- near misses / the needle split across a boundary ---
		"can't have more  than seven dream counters on it", // double space
		"can'thave more than seven dream counters",
		"cant have more than seven dream counters on it", // no apostrophe
		"can’t have more than seven dream counters",      // typographic apostrophe
		"can't have more tha",
		"can't have more than",
		"can't have more than ",
		"can't have more than  ",
		"can't have more than seven",
		"can't have more than seven counters",
		"can't have more than seven counter",
		"can't have more than zero dream counters",
		"can't have more than -1 dream counters",
		"can't have more than 0 charge counters on it",
		"can't have more than 999 charge counters on it",

		// --- Unicode: runes whose lowercase IS ASCII, and general non-ASCII ---
		"Kelvin can't have more than two k counters on it", // U+212A KELVIN -> 'k'
		"İstanbul can't have more than two i counters",     // U+0130 -> 'i'
		"Élan can't have more than three élan counters on it",
		"日本語 can't have more than five mana counters on it",
		"can't have more than two über counters on it",
		"ß can't have more than two sharp counters on it",
		"ｃａｎ't have more than two wide counters", // fullwidth: must NOT match
		"KKK",
		"İİ",
		"caN’T HAVE MORE THAN two x counters",

		// --- invalid UTF-8 ---
		"\xff\xfe can't have more than two bad counters on it",
		"can't have more than two \xff\xfe counters on it",
		"\xff\xfe\xfd",
		"can'\xfft have more than two counters",

		// --- unrelated oracle text (the overwhelmingly common case) ---
		"Flying, vigilance, trample",
		"When this creature enters, draw a card.",
		"Enchant creature. Enchanted creature can't attack or block.",
		"This creature can't be blocked.",
		"Players can't gain life.",
		"Creatures you control get +1/+1.",
		"{T}: Add {G}.",
		strings.Repeat("Whenever this creature attacks, put a +1/+1 counter on it. ", 8),
	}
}

// TestR172_ParseCounterLimit_DifferentialVsLegacy runs BOTH the pre-check
// parser and the pre-optimisation parser over the whole corpus (existing
// table + adversarial cases) and asserts identical (int, string) results.
func TestR172_ParseCounterLimit_DifferentialVsLegacy(t *testing.T) {
	for _, raw := range counterLimitCorpus() {
		gotN, gotKind := parseCounterLimit(raw)
		wantN, wantKind := parseCounterLimitLegacy(raw)
		if gotN != wantN || gotKind != wantKind {
			t.Errorf("parseCounterLimit(%q) = (%d, %q); legacy = (%d, %q)",
				raw, gotN, gotKind, wantN, wantKind)
		}
	}
}

// TestR172_ContainsSubstrFold_BothDirections fails if the pre-check wrongly
// REJECTS a string whose lowered form contains the needle (false negative),
// and fails if it wrongly ACCEPTS one that does not (false positive).
func TestR172_ContainsSubstrFold_BothDirections(t *testing.T) {
	var sawTrue, sawFalse int
	for _, raw := range counterLimitCorpus() {
		got := containsSubstrFold(raw, counterLimitNeedle)
		want := containsFoldOracle(raw, counterLimitNeedle)
		if got && !want {
			t.Errorf("FALSE POSITIVE: containsSubstrFold(%q) = true, oracle = false", raw)
		}
		if !got && want {
			t.Errorf("FALSE NEGATIVE: containsSubstrFold(%q) = false, oracle = true", raw)
		}
		if want {
			sawTrue++
		} else {
			sawFalse++
		}
	}
	// Guard the guard: a corpus that is all-matching or all-non-matching
	// could not detect one of the two directions.
	if sawTrue < 5 {
		t.Fatalf("corpus has only %d matching cases; cannot prove the reject direction", sawTrue)
	}
	if sawFalse < 5 {
		t.Fatalf("corpus has only %d non-matching cases; cannot prove the accept direction", sawFalse)
	}
}

// TestR172_ContainsSubstrFold_NeedleAtEveryOffset inserts the needle at every
// byte offset of a carrier string (and in every case permutation of the first
// four characters) and asserts the pre-check finds it every time.
func TestR172_ContainsSubstrFold_NeedleAtEveryOffset(t *testing.T) {
	carrier := "abc éKİ XYZ can' have more than  日 zz"
	for i := 0; i <= len(carrier); i++ {
		for mask := 0; mask < 16; mask++ {
			n := []byte(counterLimitNeedle)
			for b := 0; b < 4; b++ {
				if mask&(1<<b) != 0 && 'a' <= n[b] && n[b] <= 'z' {
					n[b] -= 'a' - 'A'
				}
			}
			s := carrier[:i] + string(n) + carrier[i:]
			if !containsSubstrFold(s, counterLimitNeedle) {
				t.Fatalf("missed needle inserted at byte %d, mask %d: %q", i, mask, s)
			}
			if !containsFoldOracle(s, counterLimitNeedle) {
				t.Fatalf("oracle disagrees at byte %d, mask %d: %q", i, mask, s)
			}
		}
	}
}

// TestR172_ContainsSubstrFold_RandomDifferential fuzzes the predicate and the
// parser against their oracles over a biased random alphabet.
func TestR172_ContainsSubstrFold_RandomDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(1172))
	alphabet := []string{
		"c", "a", "n", "'", "t", " ", "h", "v", "e", "m", "o", "r",
		"C", "A", "N", "T", "H", "V", "E", "M", "O", "R",
		"can't ", "have ", "more ", "than ", "can't have more than ",
		"CAN'T HAVE MORE THAN ", "x", "K", "İ", "é", "日",
		"’", "\xff", "7", "seven", "dream", "counters",
	}
	for i := 0; i < 200000; i++ {
		var sb strings.Builder
		for j := rng.Intn(10); j >= 0; j-- {
			sb.WriteString(alphabet[rng.Intn(len(alphabet))])
		}
		s := sb.String()
		if got, want := containsSubstrFold(s, counterLimitNeedle), containsFoldOracle(s, counterLimitNeedle); got != want {
			t.Fatalf("predicate mismatch on %q: got %v want %v", s, got, want)
		}
		gotN, gotK := parseCounterLimit(s)
		wantN, wantK := parseCounterLimitLegacy(s)
		if gotN != wantN || gotK != wantK {
			t.Fatalf("parser mismatch on %q: got (%d,%q) want (%d,%q)", s, gotN, gotK, wantN, wantK)
		}
	}
}

// TestR172_ContainsSubstrFold_EdgeContracts pins the documented contract.
func TestR172_ContainsSubstrFold_EdgeContracts(t *testing.T) {
	if !containsSubstrFold("anything", "") {
		t.Error("empty needle must always match")
	}
	if !containsSubstrFold("", "") {
		t.Error("empty needle in empty haystack must match")
	}
	if containsSubstrFold("", counterLimitNeedle) {
		t.Error("empty haystack must not match a non-empty needle")
	}
	if containsSubstrFold("short", counterLimitNeedle) {
		t.Error("haystack shorter than needle must not match")
	}
	if !containsSubstrFold(counterLimitNeedle, counterLimitNeedle) {
		t.Error("exact-length haystack must match")
	}
	if !containsSubstrFold(strings.ToUpper(counterLimitNeedle), counterLimitNeedle) {
		t.Error("all-uppercase haystack must match")
	}
}

// -----------------------------------------------------------------------------
// Benchmarks: the matching case (rare) and the non-matching case (common).
// -----------------------------------------------------------------------------

var (
	benchMatch    = "Rasputin Dreamweaver can't have more than seven dream counters on it"
	benchNoMatch  = "Whenever this creature attacks, put a +1/+1 counter on it. Flying, vigilance."
	benchSinkInt  int
	benchSinkKind string
)

func BenchmarkParseCounterLimit_Match(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchSinkInt, benchSinkKind = parseCounterLimit(benchMatch)
	}
}

func BenchmarkParseCounterLimit_NoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchSinkInt, benchSinkKind = parseCounterLimit(benchNoMatch)
	}
}

func BenchmarkParseCounterLimitLegacy_Match(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchSinkInt, benchSinkKind = parseCounterLimitLegacy(benchMatch)
	}
}

func BenchmarkParseCounterLimitLegacy_NoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchSinkInt, benchSinkKind = parseCounterLimitLegacy(benchNoMatch)
	}
}

// TestR172_OnlyKelvinAndDotlessILowerToASCII exhaustively verifies, over the
// whole Unicode range, which non-ASCII runes unicode.ToLower maps into ASCII.
// containsSubstrFold's fast arm assumes an ASCII needle byte other than 'i'
// or 'k' can only be begun by an ASCII haystack byte; this test is what makes
// that assumption safe to rely on if anyone ever changes the needle. (The
// production code still handles the general case, so a future Unicode table
// change degrades performance, never correctness.)
func TestR172_OnlyKelvinAndDotlessILowerToASCII(t *testing.T) {
	got := map[rune]rune{}
	for r := rune(utf8.RuneSelf); r <= unicode.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue
		}
		if lr := unicode.ToLower(r); lr < utf8.RuneSelf {
			got[r] = lr
		}
	}
	want := map[rune]rune{0x0130: 'i', 0x212A: 'k'}
	if len(got) != len(want) {
		t.Fatalf("non-ASCII runes lowering to ASCII = %v, want %v", got, want)
	}
	for r, lr := range want {
		if got[r] != lr {
			t.Errorf("ToLower(%U) = %q, want %q", r, got[r], lr)
		}
	}
	// And the needle in production contains neither 'i' nor 'k' as its FIRST
	// byte, which is the only byte the fast arm special-cases.
	if counterLimitNeedle[0] == 'i' || counterLimitNeedle[0] == 'k' {
		t.Fatalf("needle now starts with %q; re-check containsSubstrFold's fast arm", counterLimitNeedle[0])
	}
}

var benchSinkBool bool

func BenchmarkContainsSubstrFold_NoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchSinkBool = containsSubstrFold(benchNoMatch, counterLimitNeedle)
	}
}

func BenchmarkContainsFoldOracle_NoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchSinkBool = containsFoldOracle(benchNoMatch, counterLimitNeedle)
	}
}
