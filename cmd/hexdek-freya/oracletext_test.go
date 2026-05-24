package main

import (
	"strings"
	"testing"
)

// TestStripReminder_Basics covers the four shapes the stripper has to
// handle: no parens (passthrough), single-paren reminder (the 99% case),
// multi-paren (a card with several keyword reminders), and pathological
// inputs (empty, mismatched, nested) that come from data quirks.
func TestStripReminder_Basics(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no parens passthrough",
			in:   "Lightning Bolt deals 3 damage to any target.",
			want: "Lightning Bolt deals 3 damage to any target.",
		},
		{
			name: "single reminder stripped",
			in:   "Flying (This creature can't be blocked except by creatures with flying or reach.)",
			want: "Flying",
		},
		{
			name: "multiple reminders stripped",
			in:   "Flying (...flying or reach.) Trample (...over.) Vigilance (...not attack.)",
			want: "Flying Trample Vigilance",
		},
		{
			name: "reminder mid-sentence preserves prose",
			in:   "Cycling {2} ({2}, Discard this card: Draw a card.) When you cycle this card, draw a card.",
			want: "Cycling {2} When you cycle this card, draw a card.",
		},
		{
			name: "empty parens collapse cleanly",
			in:   "Vigilance () Lifelink",
			want: "Vigilance Lifelink",
		},
		{
			name: "unclosed paren swallows tail",
			in:   "Flying (oops never closes",
			want: "Flying",
		},
		{
			name: "stray close paren tolerated",
			in:   "Flying ) Trample",
			want: "Flying Trample",
		},
		{
			name: "nested parens fully stripped",
			in:   "Outer (middle (inner) text) tail",
			want: "Outer tail",
		},
		{
			name: "mana symbols in braces unaffected",
			in:   "{T}: Add {W}{U}.",
			want: "{T}: Add {W}{U}.",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := stripReminder(tc.in)
			if got != tc.want {
				t.Errorf("stripReminder(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSplitModes_RealCards pins the bullet extractor against canonical
// modal printings. The preamble (slice index 0) is the always-applied
// clause; subsequent indices are the modes.
func TestSplitModes_RealCards(t *testing.T) {
	cases := []struct {
		name      string
		ot        string
		want      []string
	}{
		{
			name: "Cryptic Command — 4 modes choose two",
			ot: "Choose two — • Counter target spell. • Return target permanent to its owner's hand. " +
				"• Tap all creatures your opponents control. • Draw a card.",
			want: []string{
				"",
				"Counter target spell.",
				"Return target permanent to its owner's hand.",
				"Tap all creatures your opponents control.",
				"Draw a card.",
			},
		},
		{
			name: "Boros Charm-style — 3 modes choose one",
			ot: "Choose one — • Target player gains protection from creatures this turn. " +
				"• Permanents you control are indestructible this turn. " +
				"• Target creature you control gets +2/+0 and gains double strike until end of turn.",
			want: []string{
				"",
				"Target player gains protection from creatures this turn.",
				"Permanents you control are indestructible this turn.",
				"Target creature you control gets +2/+0 and gains double strike until end of turn.",
			},
		},
		{
			name: "Non-modal card returns nil",
			ot:   "Lightning Bolt deals 3 damage to any target.",
			want: nil,
		},
		{
			name: "Choose one or more",
			ot:   "Choose one or more — • Destroy target artifact. • Destroy target enchantment. • Destroy target land.",
			want: []string{
				"",
				"Destroy target artifact.",
				"Destroy target enchantment.",
				"Destroy target land.",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := splitModes(tc.ot)
			if !equalStrSlice(got, tc.want) {
				t.Errorf("splitModes(%q)\n  got:  %#v\n  want: %#v", tc.ot, got, tc.want)
			}
		})
	}
}

// TestHasKeyword_WordBoundary verifies that hasKeyword distinguishes
// keywords from substrings — "cast" should not match "broadcast", and
// "land" should not match "highlander".
func TestHasKeyword_WordBoundary(t *testing.T) {
	cases := []struct {
		ot, kw string
		want   bool
	}{
		{"You may cast it from exile.", "cast", true},
		{"This spell can't be broadcast.", "cast", false},
		{"Search your library for a land card.", "land", true},
		{"A highlander deck plays one of each card.", "land", false},
		{"", "cast", false},
		{"cast", "", false},
	}
	for _, tc := range cases {
		got := hasKeyword(tc.ot, tc.kw)
		if got != tc.want {
			t.Errorf("hasKeyword(%q, %q) = %v, want %v", tc.ot, tc.kw, got, tc.want)
		}
	}
}

// --- Integration: regression tests for the analysis sites that now use
// otClean. These confirm the substring leak is fixed without losing the
// real positive detection.

func TestClassifyCard_CascadeNotEmptiesLibrary(t *testing.T) {
	// Bituminous Blast — the worst offender. Pre-fix this card was
	// flagged EmptiesLibrary by the cascade reminder text alone.
	ot := "Cascade (When you cast this spell, exile cards from the top of your library until you exile a nonland card that costs less. " +
		"You may cast it without paying its mana cost. Put the exiled cards on the bottom of your library in a random order.) " +
		"Bituminous Blast deals 4 damage to target creature or planeswalker."
	p := ClassifyCard("Bituminous Blast", ot, "Instant", "{2}{B}{R}", 4, "")
	if p.EmptiesLibrary {
		t.Error("Bituminous Blast (cascade) was flagged EmptiesLibrary — cascade reminder text leaked through stripReminder")
	}
}

func TestClassifyCard_FlashbackNotGraveyardCurate(t *testing.T) {
	// Cabal Therapy — flashback creature. Pre-fix the flashback
	// reminder ("Exile this card from your graveyard") matched the
	// graveyard_curate substring check.
	ot := "Name a nonland card. Target player reveals their hand and discards all cards with that name. " +
		"Flashback — Sacrifice a creature. (You may cast this card from your graveyard for its flashback cost. Then exile it.)"
	p := ClassifyCard("Cabal Therapy", ot, "Sorcery", "{B}", 1, "")
	for _, e := range p.Effects {
		if e == "graveyard_curate" {
			t.Error("Cabal Therapy (flashback) was flagged graveyard_curate — flashback reminder leaked through")
			break
		}
	}
}

func TestClassifyCard_EternalizeNotGraveyardCurate(t *testing.T) {
	// Eternal Scourge has eternalize-style "from anywhere" but the
	// common-case eternalize creature is Champion of Wits. Use it.
	ot := "When this creature enters, draw two cards, then discard two cards. " +
		"Eternalize {5}{U}{U} ({5}{U}{U}, Exile this card from your graveyard: Create a token that's a copy of this card, " +
		"except it's a 4/4 black Zombie Human Wizard. Eternalize only as a sorcery.)"
	p := ClassifyCard("Champion of Wits", ot, "Creature — Human Wizard", "{2}{U}", 3, "1")
	for _, e := range p.Effects {
		if e == "graveyard_curate" {
			t.Errorf("Champion of Wits (eternalize) was flagged graveyard_curate — eternalize reminder leaked through. Effects=%v", p.Effects)
			break
		}
	}
}

func TestClassifyCard_DemonicConsultationStillEmptiesLibrary(t *testing.T) {
	// Negative-of-the-fix: stripping reminder text must NOT regress the
	// real cEDH finisher detection. Demonic Consultation's empty-library
	// effect is in its main rules text, not reminder text.
	ot := "Name a card. Exile the top six cards of your library, then reveal cards from the top of your library " +
		"until you reveal the named card. Put that card into your hand and exile all other cards revealed this way."
	p := ClassifyCard("Demonic Consultation", ot, "Instant", "{B}", 1, "")
	if !p.EmptiesLibrary {
		t.Error("Demonic Consultation lost its EmptiesLibrary flag after the reminder-stripping rewrite")
	}
}

func TestClassifyCard_UnderworldBreachStillGraveyardCurate(t *testing.T) {
	// Negative-of-the-fix: a card whose REAL rules text says "exile ...
	// from your graveyard" must still get the graveyard_curate effect.
	// Underworld Breach grants escape with a fixed exile cost as part
	// of its main text — the parenthesized escape reminder is extra,
	// not load-bearing.
	ot := "Each nonland card in your graveyard has escape. The escape cost is equal to the card's mana cost plus exile three other cards from your graveyard. " +
		"(You may cast a card from your graveyard for its escape cost.) " +
		"At the beginning of the next end step, sacrifice this enchantment."
	p := ClassifyCard("Underworld Breach", ot, "Enchantment", "{1}{R}", 2, "")
	found := false
	for _, e := range p.Effects {
		if e == "graveyard_curate" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Underworld Breach lost its graveyard_curate effect after the reminder-stripping rewrite. Effects=%v", p.Effects)
	}
}

// --- helpers

func equalStrSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimSpace(a[i]) != strings.TrimSpace(b[i]) {
			return false
		}
	}
	return true
}
