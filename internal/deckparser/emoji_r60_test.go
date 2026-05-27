package deckparser

import (
	"strings"
	"testing"
)

// emoji_r60_test.go — emoji-decoration tolerance audit. Moxfield deck
// tags sometimes carry pictographic markers like 🔥 / ⚔️ / 💀 as a
// visual annotation. Pre-fix, every such line landed in Unresolved
// because the emoji bled into the meta lookup name. Fix peels leading
// + trailing runs of \p{So} (symbol-other) / \p{Sk} (symbol-modifier,
// e.g. skin-tone) / \p{Cf} (format chars including U+FE0F variation
// selector that follows emoji like ⚔️) plus whitespace from
// cleanCardName, and applies the same strip to COMMANDER: / PARTNER:
// directive names so `COMMANDER: Sigarda 🔥` resolves cleanly.
//
// Already-handled paths (emoji inside a hashtag, bracket-tag, or
// inline `//` comment) continue to work because their containing
// strippers consume the whole token before the emoji-strip would
// see it.

func emojiMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Sigarda, Host of Herons", "Atraxa, Praetors' Voice",
		"Sol Ring", "Cyclonic Rift", "Lim-Dûl's Vault",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary",
			Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_EmojiTrailingDecoration — pictographic markers
// after the card name resolve cleanly.
func TestParseDeckReader_EmojiTrailingDecoration(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring 🔥
1 Cyclonic Rift 🔥⚔️💀
2 Sol Ring 🌊
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, emojiMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 4 {
		t.Errorf("library: want 4 (1+1+2), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("expected zero unresolved, got %v", td.Unresolved)
	}
}

// TestParseDeckReader_EmojiLeadingDecoration — emoji before the card
// name. Less common but seen in some Moxfield tag styles.
func TestParseDeckReader_EmojiLeadingDecoration(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 🔥 Sol Ring
1 ⚔️ Cyclonic Rift
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, emojiMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2, got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_EmojiInCommanderDirective — `COMMANDER: <name>
// 🔥` strips the emoji from the directive so the commander resolves.
// Pre-fix this raised a "no commander found" error (and a probe
// dereference of the nil TournamentDeck panicked downstream).
func TestParseDeckReader_EmojiInCommanderDirective(t *testing.T) {
	text := `COMMANDER: Sigarda, Host of Herons 🔥
1 Sol Ring
1 Cyclonic Rift
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, emojiMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Fatalf("commander: want Sigarda, got %v", commanderNamesProbe(td))
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_EmojiVariationSelectorHandled — ⚔️ is the
// crossed-swords pictograph (U+2694) followed by U+FE0F variation
// selector. The strip must clear BOTH chars or the trailing selector
// leaves the name as e.g. `Sol Ring ️` which still misses meta.
func TestParseDeckReader_EmojiVariationSelectorHandled(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring ⚔️
1 Cyclonic Rift ⚔
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, emojiMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2, got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_EmojiCoexistsWithFoilMarker — `*F* 🔥` strips
// both, in either order, by the repeat-strip loop in cleanCardName.
func TestParseDeckReader_EmojiCoexistsWithFoilMarker(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring *F* 🔥
1 Cyclonic Rift 🔥 *F*
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, emojiMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2, got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_EmojiCoexistsWithBracketAndComment — `1 Sol Ring
// 🔥 [Lands]` peels both the bracket tag and the trailing emoji.
// `1 Sol Ring 🔥 // mvp` peels the inline comment AND then the emoji.
func TestParseDeckReader_EmojiCoexistsWithBracketAndComment(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring 🔥 [Lands]
1 Cyclonic Rift 🔥 // wincon
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, emojiMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2, got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestCleanCardName_EmojiPairs — helper-level pin so a future regex
// rewrite that breaks the trim stays caught.
func TestCleanCardName_EmojiPairs(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Sol Ring 🔥", "Sol Ring"},
		{"Sol Ring 🔥⚔️💀", "Sol Ring"},
		{"🔥 Sol Ring", "Sol Ring"},
		{"⚔️ Sol Ring 🔥", "Sol Ring"},
		{"Sol Ring 🔥 *F*", "Sol Ring"},
		{"Sol Ring", "Sol Ring"},
		// Accented letters MUST survive — they're \p{L}, not \p{So}.
		{"Lim-Dûl's Vault", "Lim-Dûl's Vault"},
		{"Lim-Dûl's Vault 🔥", "Lim-Dûl's Vault"},
		{"🔥 Lim-Dûl's Vault 🔥", "Lim-Dûl's Vault"},
	}
	for _, tc := range cases {
		got := cleanCardName(tc.in)
		if got != tc.want {
			t.Errorf("cleanCardName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestCleanCardName_EmojiNegativeOfFix_AccentsNotStripped — defensive
// regression: \p{L} letter-class chars (Lim-Dûl, Jötun, Æther) must
// NOT be touched by the emoji strip. The trim regex is scoped to
// \p{So}\p{Sk}\p{Cf} only.
func TestCleanCardName_EmojiNegativeOfFix_AccentsNotStripped(t *testing.T) {
	cases := []string{
		"Lim-Dûl's Vault", "Jötun Grunt", "Æther Vial",
		"Naga Vitalist", "Séance",
	}
	for _, in := range cases {
		got := cleanCardName(in)
		if got != in {
			t.Errorf("cleanCardName(%q) = %q — accent-letter character stripped (emoji regex too broad)", in, got)
		}
	}
}
