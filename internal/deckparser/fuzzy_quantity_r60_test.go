package deckparser

import (
	"strings"
	"testing"
)

// fuzzy_quantity_r60_test.go — fuzzy quantity-syntax normalization.
//
// Pre-fix only the canonical leading-digit form (`4 Card` / `4x Card`)
// resolved. Alternate quantity syntaxes that show up in real exports
// silently degraded:
//
//   - `x4 Lightning Bolt` — leading `x` prefix (Aetherhub list-view
//     paste). Fell into the fallback as qty=1 name=`x4 Lightning Bolt`
//     and missed meta.
//   - `Lightning Bolt x4` — trailing `xN` (some Tappedout views, some
//     hand-typed lists). Same fallback failure.
//   - `Lightning Bolt (4)` — trailing parens-qty (Deckbox alternative
//     export, some Archidekt views). Even WORSE: the set-parens strip
//     ate `(4)` so the line parsed as qty=1, ONE copy of Lightning
//     Bolt instead of FOUR. Silent count loss.
//
// New normalizeFuzzyQuantity helper translates each alternate to the
// canonical leading form before set-parens stripping eats `(4)`.

func fuzzyMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice",
		"Lightning Bolt", "Sol Ring", "Counterspell",
		"Goblin Guide", "Snapcaster Mage", "Force of Will",
		"Brainstorm", "Ponder", "Preordain", "Path to Exile",
		"Searing Blaze", "Lava Spike", "Daze",
		"Birds of Paradise", "Llanowar Elves",
		"Forest", "Mountain", "Island", "Plains", "Swamp",
		// A defensive collision-bait card whose name contains an
		// inner `x` — must NOT be misparsed by the trailing-x regex.
		"Naya Hexproof",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, Types: []string{"generic"}, CMC: 1,
		}
	}
	return meta
}

// TestNormalizeFuzzyQuantity_AllShapes — the canonical transformation
// table. Each fuzzy shape rewrites to the leading-digit form; the
// canonical `4 Card` / `4x Card` forms pass through unchanged.
func TestNormalizeFuzzyQuantity_AllShapes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Canonical forms — no-op.
		{"4 Lightning Bolt", "4 Lightning Bolt"},
		{"4x Lightning Bolt", "4x Lightning Bolt"},
		{"1 Sol Ring", "1 Sol Ring"},
		// Fuzzy forms — rewrite to leading-digit.
		{"x4 Lightning Bolt", "4 Lightning Bolt"},
		{"X4 Lightning Bolt", "4 Lightning Bolt"},
		{"Lightning Bolt x4", "4 Lightning Bolt"},
		{"Lightning Bolt X4", "4 Lightning Bolt"},
		{"Lightning Bolt (4)", "4 Lightning Bolt"},
		{"Sol Ring (1)", "1 Sol Ring"},
		// Multi-digit quantities.
		{"x12 Forest", "12 Forest"},
		{"Forest x12", "12 Forest"},
		{"Forest (12)", "12 Forest"},
		// Defensive: card with inner `x` and NO trailing qty — no
		// rewrite (no trailing `xN` token to match).
		{"Naya Hexproof", "Naya Hexproof"},
		// Defensive: trailing set-parens with letters — no rewrite
		// (parens content isn't all-digits, set-parens strip handles it).
		{"Lightning Bolt (M11)", "Lightning Bolt (M11)"},
		// Empty line — defensive.
		{"", ""},
	}
	for _, tc := range cases {
		got := normalizeFuzzyQuantity(tc.in)
		if got != tc.want {
			t.Errorf("normalizeFuzzyQuantity(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestParseDeckReader_FuzzyQty_LeadingX — `x4 Lightning Bolt`
// end-to-end through the parser. Library should have 4 copies, not 1
// (the pre-fix fallback path would have resolved 0 with name=`x4
// Lightning Bolt`).
func TestParseDeckReader_FuzzyQty_LeadingX(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
x4 Lightning Bolt
x4 Goblin Guide
X1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, fuzzyMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 9 {
		t.Errorf("Library: want 9 (4+4+1), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_FuzzyQty_TrailingX — `Lightning Bolt x4`
// end-to-end. Same expected result as leading-x.
func TestParseDeckReader_FuzzyQty_TrailingX(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
Lightning Bolt x4
Goblin Guide x4
Sol Ring x1
Counterspell X2
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, fuzzyMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 11 {
		t.Errorf("Library: want 11 (4+4+1+2), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_FuzzyQty_TrailingParens — `Lightning Bolt (4)`.
// This is the highest-impact fix: pre-fix the set-parens strip ate
// the `(4)` and the card resolved with qty=1 (silent count loss).
// Verify the quantity is preserved.
func TestParseDeckReader_FuzzyQty_TrailingParens(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
Lightning Bolt (4)
Goblin Guide (4)
Sol Ring (1)
Forest (12)
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, fuzzyMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 21 {
		t.Errorf("Library: want 21 (4+4+1+12), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	// Sanity-check the multiplicity per name.
	counts := map[string]int{}
	for _, c := range td.Library {
		counts[c.Name]++
	}
	wantCounts := map[string]int{
		"Lightning Bolt": 4, "Goblin Guide": 4, "Sol Ring": 1, "Forest": 12,
	}
	for name, want := range wantCounts {
		if counts[name] != want {
			t.Errorf("count[%s] = %d, want %d", name, counts[name], want)
		}
	}
}

// TestParseDeckReader_FuzzyQty_AllShapesMixed — a single deck mixing
// every quantity syntax (canonical + fuzzy x3). Verifies the
// normalize step doesn't interfere with the canonical paths.
func TestParseDeckReader_FuzzyQty_AllShapesMixed(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
4 Lightning Bolt
4x Goblin Guide
x4 Snapcaster Mage
Brainstorm x4
Ponder (4)
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, fuzzyMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 21 {
		t.Errorf("Library: want 21 (4×5 + 1), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_FuzzyQty_DoesNotEatSetCode — `Lightning Bolt
// (M11)` is a set-code annotation, NOT a quantity. The fuzzy parens
// regex requires digit-only parens content; alphabetic content
// (`M11` has letter `M`) falls through to the existing set-parens
// strip path. Without this guard `(M11)` would be misinterpreted as
// qty.
func TestParseDeckReader_FuzzyQty_DoesNotEatSetCode(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Lightning Bolt (M11) 146
1 Sol Ring (CMR) 254
1 Counterspell (KHM) 56
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, fuzzyMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 3 {
		t.Errorf("Library: want 3 (set codes stripped, qty=1 each), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_FuzzyQty_InnerXNotEaten — a card name with an
// inner `x` (e.g. `Naya Hexproof`) must not be misparsed by the
// trailing-x regex. The regex requires whitespace before `[xX]` so
// only an isolated trailing token matches.
func TestParseDeckReader_FuzzyQty_InnerXNotEaten(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Naya Hexproof
1 Lightning Bolt
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, fuzzyMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 2 {
		t.Errorf("Library: want 2, got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	// Specifically verify Naya Hexproof survived.
	found := false
	for _, c := range td.Library {
		if c.Name == "Naya Hexproof" {
			found = true
		}
	}
	if !found {
		t.Errorf("Naya Hexproof missing from library; got %v", libraryNames(td.Library))
	}
}

// TestParseDeckReader_FuzzyQty_CombinesWithBullets — Tappedout-style
// bullet prefix + trailing-x quantity. The bullet strip happens
// earlier in the pipeline; the fuzzy normalize then sees the un-
// bulleted line and rewrites the trailing `x4` to the leading form.
func TestParseDeckReader_FuzzyQty_CombinesWithBullets(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
* Lightning Bolt x4
- Goblin Guide x4
• Sol Ring (1)
* Forest x12
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, fuzzyMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 21 {
		t.Errorf("Library: want 21 (4+4+1+12), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestNormalizeFuzzyQuantity_LeadingDigitNoOp — when the line ALREADY
// starts with a digit, normalization is a no-op. Defends the contract
// that the canonical `4 Card` path stays exactly as deckLineRE expects
// it (the fuzzy normalize never rewrites a digit-led line).
func TestNormalizeFuzzyQuantity_LeadingDigitNoOp(t *testing.T) {
	cases := []string{
		"1 Sol Ring",
		"4 Lightning Bolt",
		"4x Goblin Guide",
		"12 Forest",
		"  4 Snapcaster Mage", // leading whitespace tolerated
	}
	for _, in := range cases {
		got := normalizeFuzzyQuantity(in)
		if got != in {
			t.Errorf("normalizeFuzzyQuantity(%q) = %q, want unchanged", in, got)
		}
	}
}
