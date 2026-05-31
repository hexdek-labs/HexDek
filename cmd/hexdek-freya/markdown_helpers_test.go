package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestScryfallSearchURL pins the URL shape — exact-name matching via
// the `!"..."` Scryfall operator with URL-encoded quotes. The link
// must resolve to a single-card page on Scryfall when the name is
// canonical (Sol Ring → Sol Ring's page).
func TestScryfallSearchURL(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		// Standard card name — spaces in the search query become +
		// when url.QueryEscape encodes them.
		{"Sol Ring", "https://scryfall.com/search?q=%21%22Sol+Ring%22"},
		// Punctuation in card names (apostrophes, commas) must be
		// encoded — these are the standard breakage points.
		{"Demonic Tutor", "https://scryfall.com/search?q=%21%22Demonic+Tutor%22"},
		{"Razaketh, the Foulblooded", "https://scryfall.com/search?q=%21%22Razaketh%2C+the+Foulblooded%22"},
		// Edge: empty / whitespace-only name returns empty so the
		// caller can fall back without rendering a broken link.
		{"", ""},
		{"   ", ""},
	}
	for _, c := range cases {
		got := ScryfallSearchURL(c.name)
		if got != c.want {
			t.Errorf("ScryfallSearchURL(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestScryfallLink wraps the URL in markdown link syntax. Discord +
// GitHub + Reddit all render `[Sol Ring](URL)` correctly; the test
// asserts the exact format so a render-pipeline regression surfaces
// here, not in the eyes of someone pasting the report into a chat.
func TestScryfallLink(t *testing.T) {
	got := scryfallLink("Sol Ring")
	want := "[Sol Ring](https://scryfall.com/search?q=%21%22Sol+Ring%22)"
	if got != want {
		t.Errorf("scryfallLink = %q, want %q", got, want)
	}
	if scryfallLink("") != "" {
		t.Errorf("scryfallLink on empty should be empty, got %q", scryfallLink(""))
	}
}

// TestScryfallLinks pins the "join multiple cards with sep" helper.
// Used for combo piece lists, where the report renders
// `[A](…) + [B](…) + [C](…)`. Empty entries must NOT produce
// stray "+ " stubs.
func TestScryfallLinks(t *testing.T) {
	got := scryfallLinks([]string{"Heliod, Sun-Crowned", "Walking Ballista"}, " + ")
	for _, want := range []string{
		"[Heliod, Sun-Crowned](https://scryfall.com/",
		"[Walking Ballista](https://scryfall.com/",
		" + ",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("scryfallLinks output missing %q\n---\n%s", want, got)
		}
	}
	// Empty / whitespace entries are filtered out, not left as " + ".
	emptyJoined := scryfallLinks([]string{"", "  ", "Sol Ring"}, " + ")
	if strings.Contains(emptyJoined, " + ") {
		t.Errorf("empty entries should not produce a stray '+'; got %q", emptyJoined)
	}
}

// TestPrintMarkdownSummaryHeader_FullProfile pins the TL;DR header
// when a DeckProfile is populated. Discord renders the first 8-12
// lines as a preview, so the header must surface archetype + bracket
// + win method + the gameplan one-liner without scrolling.
func TestPrintMarkdownSummaryHeader_FullProfile(t *testing.T) {
	r := &FreyaReport{
		DeckName:   "azula",
		Commander:  "Yshtola, Night's Blessed",
		TotalCards: 100,
		Profile: &DeckProfile{
			PrimaryArchetype: "Combo",
			Bracket:          5,
			BracketLabel:     "cEDH",
			PrimaryWinLine:   "Thassa's Oracle + Demonic Consultation",
			GameplanSummary:  "Combo deck that wins via Thassa's Oracle + Demonic Consultation.",
		},
	}
	var buf bytes.Buffer
	printMarkdownSummaryHeader(&buf, r)
	out := buf.String()
	for _, want := range []string{
		"# Freya — azula",
		"Yshtola, Night's Blessed",
		"scryfall.com/search?q=",       // commander link present
		"| Archetype | Bracket | Cards | Win method |",
		"| Combo | B5 (cEDH) | 100 |",
		"Thassa's Oracle + Demonic Consultation",
		"> Combo deck that wins via",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary header missing %q\n---\n%s", want, out)
		}
	}
}

// TestPrintMarkdownSummaryHeader_NoProfile pins the defensive
// fallback. Reports without a DeckProfile (e.g. analysis errors that
// surface a partial report) must still render a usable header rather
// than dropping the TL;DR block entirely.
func TestPrintMarkdownSummaryHeader_NoProfile(t *testing.T) {
	r := &FreyaReport{
		DeckName:   "partial-deck",
		Commander:  "Atraxa",
		TotalCards: 100,
	}
	var buf bytes.Buffer
	printMarkdownSummaryHeader(&buf, r)
	out := buf.String()
	for _, want := range []string{
		"# Freya — partial-deck",
		"Atraxa",
		"**Cards:** 100",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fallback header missing %q\n---\n%s", want, out)
		}
	}
	// And it MUST NOT include the table (no profile to populate it).
	if strings.Contains(out, "| Archetype") {
		t.Errorf("fallback header should skip the table; got:\n%s", out)
	}
}

// TestPrintGameplanScriptMarkdown_AllSections covers the
// turn-by-turn + branching + degradation rendering. All three
// section headers must appear, and the IF/THEN/WHEN flow markers
// must be present so a reader can find the structure visually.
func TestPrintGameplanScriptMarkdown_AllSections(t *testing.T) {
	script := buildGameplanScript(&DeckProfile{
		PrimaryArchetype: "Reanimator",
	}, &FreyaReport{Commander: "Meren of Clan Nel Toth"})
	if script == nil {
		t.Fatal("setup: buildGameplanScript returned nil")
	}

	var buf bytes.Buffer
	printGameplanScriptMarkdown(&buf, script)
	out := buf.String()

	for _, want := range []string{
		"### Turn-by-Turn Sequence",
		"- **T1:**",
		"- **T3:**",
		"### Branching Decisions",
		"- **IF**",
		"- **THEN**",
		"### Graceful Degradation",
		"- **WHEN**",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("gameplan markdown missing %q\n---\n%s", want, out)
		}
	}
}

// TestPrintGameplanScriptMarkdown_NilSilent pins the defensive
// entry — a nil script must NOT write anything (caller can invoke
// unconditionally in the wider markdown pipeline).
func TestPrintGameplanScriptMarkdown_NilSilent(t *testing.T) {
	var buf bytes.Buffer
	printGameplanScriptMarkdown(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("nil script should write nothing; got %q", buf.String())
	}
}

// TestPrintMarkdown_ArchetypeDistinctDecks runs the full markdown
// renderer against 3 archetype-distinct synthetic FreyaReports and
// asserts each output contains:
//   - The compact summary table (TL;DR header)
//   - Scryfall hyperlinks for commander + named cards
//   - The archetype-keyed gameplan script section
//   - The expected combo / win-line content
//
// Combo / Aggro / Reanimator are the three canonical shapes that
// stress-test different report sections (combo-heavy / threat-heavy /
// graveyard-heavy).
func TestPrintMarkdown_ArchetypeDistinctDecks(t *testing.T) {
	cases := []struct {
		name        string
		report      *FreyaReport
		mustContain []string
	}{
		{
			name: "Combo / cEDH",
			report: &FreyaReport{
				DeckName:   "kraum_tymna",
				Commander:  "Kraum, Ludevic's Opus",
				TotalCards: 100,
				TrueInfinites: []ComboResult{
					{Cards: []string{"Thassa's Oracle", "Demonic Consultation"},
						LoopType:    "infinite",
						Description: "exile library then ETB win",
						Confirmed:   true},
				},
				Profile: &DeckProfile{
					Commander:        "Kraum, Ludevic's Opus",
					PrimaryArchetype: "Combo",
					Bracket:          5,
					BracketLabel:     "cEDH",
					PrimaryWinLine:   "Thassa's Oracle + Demonic Consultation",
					GameplanSummary:  "Combo deck that wins via Thassa's Oracle.",
				},
			},
			mustContain: []string{
				"# Freya — kraum_tymna",
				"| Combo | B5 (cEDH) |",
				"[Kraum, Ludevic's Opus](https://scryfall.com/",
				"[Thassa's Oracle](https://scryfall.com/",
				"[Demonic Consultation](https://scryfall.com/",
				"### Turn-by-Turn Sequence",
				"Cast Kraum, Ludevic's Opus",
				"### Graceful Degradation",
			},
		},
		{
			name: "Aggro / Goblins",
			report: &FreyaReport{
				DeckName:   "krenko_goblins",
				Commander:  "Krenko, Mob Boss",
				TotalCards: 100,
				Finishers: []ComboResult{
					{Cards: []string{"Krenko, Mob Boss", "Coat of Arms"},
						Description: "go-wide aggro lethal"},
				},
				Profile: &DeckProfile{
					Commander:        "Krenko, Mob Boss",
					PrimaryArchetype: "Aggro",
					Bracket:          3,
					BracketLabel:     "Upgraded",
					PrimaryWinLine:   "wide goblin board",
					GameplanSummary:  "Aggro deck that wins via combat damage.",
				},
			},
			mustContain: []string{
				"# Freya — krenko_goblins",
				"| Aggro | B3 (Upgraded) |",
				"[Krenko, Mob Boss](https://scryfall.com/",
				"[Coat of Arms](https://scryfall.com/",
				"### Turn-by-Turn Sequence",
				"1-drop creature",
				"21 commander damage",
			},
		},
		{
			name: "Reanimator / Meren",
			report: &FreyaReport{
				DeckName:   "meren_reanimator",
				Commander:  "Meren of Clan Nel Toth",
				TotalCards: 100,
				Synergies: []ComboResult{
					{Cards: []string{"Meren of Clan Nel Toth", "Reassembling Skeleton", "Phyrexian Altar"},
						Description: "recurring sac fuel for experience counters"},
				},
				Profile: &DeckProfile{
					Commander:        "Meren of Clan Nel Toth",
					PrimaryArchetype: "Reanimator",
					Bracket:          4,
					BracketLabel:     "Optimized",
					PrimaryWinLine:   "Razaketh + Animate Dead",
					GameplanSummary:  "Reanimator deck that wins via cheating big threats into play.",
				},
			},
			mustContain: []string{
				"# Freya — meren_reanimator",
				"| Reanimator | B4 (Optimized) |",
				"[Meren of Clan Nel Toth](https://scryfall.com/",
				"[Reassembling Skeleton](https://scryfall.com/",
				"[Phyrexian Altar](https://scryfall.com/",
				"### Turn-by-Turn Sequence",
				"Reanimate spell",
				"### Branching Decisions",
				"Graveyard hate on table",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// buildGameplanScript needs to run for the script
			// content to appear in markdown. The full pipeline
			// (BuildDeckProfile) is expensive; for this test we
			// invoke just the script builder directly.
			c.report.Profile.GameplanScript = buildGameplanScript(c.report.Profile, c.report)

			var buf bytes.Buffer
			printMarkdown(&buf, c.report)
			out := buf.String()
			for _, want := range c.mustContain {
				if !strings.Contains(out, want) {
					t.Errorf("%s: output missing %q", c.name, want)
				}
			}
		})
	}
}
