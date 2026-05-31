package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCardLinkHTML pins the anchor shape. Every card name must
// produce a target="_blank" rel="noopener" link to the Scryfall
// exact-name search URL — this is the contract a downstream consumer
// (the future hexdek.dev/deck/{id} host) depends on.
func TestCardLinkHTML(t *testing.T) {
	got := cardLinkHTML("Sol Ring")
	for _, want := range []string{
		`<a class="card-link"`,
		`href="https://scryfall.com/search?q=%21%22Sol+Ring%22"`,
		`target="_blank"`,
		`rel="noopener"`,
		`>Sol Ring</a>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("cardLinkHTML missing %q\n---\n%s", want, got)
		}
	}
	if cardLinkHTML("") != "" {
		t.Errorf("empty name should return empty, got %q", cardLinkHTML(""))
	}
}

// TestCardLinkHTML_EscapesHTMLEntities defends against an XSS-style
// card name (or just an apostrophe-containing name) leaking raw HTML
// into the output. "Yawgmoth's Will" must render with &#39; (or
// equivalent escape), not a literal apostrophe that closes an attr.
func TestCardLinkHTML_EscapesHTMLEntities(t *testing.T) {
	got := cardLinkHTML("Yawgmoth's Will")
	// The link TEXT must be HTML-escaped.
	if strings.Contains(got, ">Yawgmoth's Will<") {
		t.Errorf("apostrophe should be HTML-escaped in link text: %q", got)
	}
	// The href attribute uses URL-encoded form (%27 or no escape since
	// `'` is not a URL-encoded mandatory char), not bare quotes that
	// could break the attribute. Just assert the link renders without
	// an unexpected closing quote in the middle of the attribute.
	if strings.Count(got, `href="`) != 1 {
		t.Errorf("link should have exactly one href attribute open: %q", got)
	}
}

// TestCardLinksHTML_MultiCardJoin asserts the multi-card join format
// used by combo piece lists. Empty/whitespace entries are filtered
// out, not left as " + " stubs.
func TestCardLinksHTML_MultiCardJoin(t *testing.T) {
	got := cardLinksHTML([]string{"Thassa's Oracle", "Demonic Consultation"}, " + ")
	for _, want := range []string{
		`Thassa&#39;s Oracle`, // text-escaped
		`Demonic Consultation`,
		` + `,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("cardLinksHTML missing %q\n---\n%s", want, got)
		}
	}
	if cardLinksHTML(nil, " + ") != "" {
		t.Errorf("nil names should return empty")
	}
}

// TestManaSymbolHTML pins the disc rendering. Each WUBRGC letter
// must produce its color-class span + the visible letter.
func TestManaSymbolHTML(t *testing.T) {
	cases := []struct {
		in, wantClass, wantLetter string
	}{
		{"W", "mana-w", "W"},
		{"U", "mana-u", "U"},
		{"B", "mana-b", "B"},
		{"R", "mana-r", "R"},
		{"G", "mana-g", "G"},
		{"C", "mana-c", "C"},
		{"w", "mana-w", "W"}, // lowercase normalizes to uppercase letter
	}
	for _, c := range cases {
		got := manaSymbolHTML(c.in)
		if !strings.Contains(got, c.wantClass) {
			t.Errorf("manaSymbolHTML(%q) missing class %q in %q", c.in, c.wantClass, got)
		}
		if !strings.Contains(got, ">"+c.wantLetter+"<") {
			t.Errorf("manaSymbolHTML(%q) missing letter %q in %q", c.in, c.wantLetter, got)
		}
	}
	// Unknown letter falls back to HTML-escaped raw text.
	if got := manaSymbolHTML("X"); strings.Contains(got, "mana-x") {
		t.Errorf("unknown letter should not produce a mana-x class; got %q", got)
	}
}

// TestPrintHTML_Structure pins the top-level HTML5 envelope: doctype,
// inline <style>, <main>, footer with FreyaVersion. The envelope is
// what makes the output a valid standalone document; if any of these
// regress the file becomes invalid HTML.
func TestPrintHTML_Structure(t *testing.T) {
	r := &FreyaReport{
		DeckName:   "envelope-test",
		TotalCards: 100,
	}
	var buf bytes.Buffer
	printHTML(&buf, r)
	out := buf.String()
	for _, want := range []string{
		"<!DOCTYPE html>",
		`<html lang="en">`,
		`<meta charset="utf-8">`,
		`<meta name="viewport"`,
		"<title>Freya — envelope-test</title>",
		"<style>",
		"</style>",
		`<main class="freya-report">`,
		"</main>",
		"</html>",
		// Footer pin:
		"hexdek-freya v" + FreyaVersion,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML envelope missing %q", want)
		}
	}
}

// TestPrintHTML_ArchetypeDistinctDecks drives the full HTML renderer
// against three archetype-distinct synthetic reports (Combo / Aggro /
// Reanimator) and asserts each output contains:
//   - The compact header (title, archetype badge, gameplan blockquote)
//   - Card hyperlinks for the commander + combo / synergy pieces
//   - The collapsible <details> sections for combos / finishers /
//     synergies / win lines
//   - The embedded turn-by-turn gameplan script (PR #902)
//   - The archetype-distinctive turn content from the script
//
// Three archetypes is enough variety to stress the section dispatch:
// Combo populates True Infinites, Aggro populates Finishers, and
// Reanimator populates Synergies.
func TestPrintHTML_ArchetypeDistinctDecks(t *testing.T) {
	cases := []struct {
		name        string
		report      *FreyaReport
		mustContain []string
	}{
		{
			name: "Combo / cEDH",
			report: &FreyaReport{
				DeckName:   "kraum_tymna_cedh",
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
					GameplanSummary:  "Combo deck that wins via Thassa's Oracle line.",
					ColorIdentity:    []string{"U", "B"},
				},
			},
			mustContain: []string{
				`<title>Freya — kraum_tymna_cedh</title>`,
				`Kraum, Ludevic&#39;s Opus`,
				`class="badge archetype">Combo<`,
				`class="badge bracket">B5 cEDH<`,
				`Thassa&#39;s Oracle`,
				`Demonic Consultation`,
				`href="https://scryfall.com/search?q=`, // card link present
				`<details open>`,                       // Deck Profile open by default
				`Turn-by-Turn Sequence`,
				`Cast Kraum, Ludevic&#39;s Opus`,
				// Color identity badge: U + B
				`mana-u`,
				`mana-b`,
				// Combos section opens (not "open" — collapsed)
				`<summary><h2>Combos`,
				`combo confirmed`, // checkmark for KnownCombos entry
				`✅`,
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
					ColorIdentity:    []string{"R"},
				},
			},
			mustContain: []string{
				`<title>Freya — krenko_goblins</title>`,
				`Krenko, Mob Boss`,
				`class="badge archetype">Aggro<`,
				`class="badge bracket">B3 Upgraded<`,
				`Coat of Arms`,
				`<summary><h2>Finishers`,
				`Turn-by-Turn Sequence`,
				`1-drop creature`,
				`21 commander damage`,
				`mana-r`,
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
					GameplanSummary:  "Reanimator deck that wins via cheating big threats.",
					ColorIdentity:    []string{"B", "G"},
				},
			},
			mustContain: []string{
				`<title>Freya — meren_reanimator</title>`,
				`Meren of Clan Nel Toth`,
				`class="badge archetype">Reanimator<`,
				`Reassembling Skeleton`,
				`Phyrexian Altar`,
				`<summary><h2>Synergies`,
				`Turn-by-Turn Sequence`,
				`Reanimate spell`,
				`Branching Decisions`,
				`Graveyard hate on table`,
				`Graceful Degradation`,
				`mana-b`,
				`mana-g`,
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// Populate the gameplan script (the production pipeline
			// does this in BuildDeckProfile; for this test we invoke
			// the builder directly).
			c.report.Profile.GameplanScript = buildGameplanScript(c.report.Profile, c.report)

			var buf bytes.Buffer
			printHTML(&buf, c.report)
			out := buf.String()
			for _, want := range c.mustContain {
				if !strings.Contains(out, want) {
					t.Errorf("%s: HTML missing %q", c.name, want)
				}
			}
		})
	}
}

// TestPrintHTML_PartialReport pins the defensive path: a report with
// nil DeckProfile / nil WinLines must still render valid HTML (no
// crashes, no orphaned tags). Partial reports happen when analysis
// errors mid-pipeline; the HTML must still be hostable.
func TestPrintHTML_PartialReport(t *testing.T) {
	r := &FreyaReport{
		DeckName:   "partial",
		TotalCards: 50,
	}
	var buf bytes.Buffer
	printHTML(&buf, r)
	out := buf.String()
	// Envelope intact:
	if !strings.Contains(out, "<!DOCTYPE html>") || !strings.Contains(out, "</html>") {
		t.Error("partial report should still produce valid HTML envelope")
	}
	// Sections that depend on nil sub-reports should be skipped, not crash.
	if strings.Contains(out, "<details><summary><h2>Combos") {
		t.Error("empty combos should skip the section, not render an empty <details>")
	}
}

// TestOpenSection_OpenAttribute pins the open-by-default attribute
// on the section opener. The Deck Profile uses open=true so the
// archetype + gameplan are visible on page load; everything else
// uses open=false to keep the page scannable.
func TestOpenSection_OpenAttribute(t *testing.T) {
	var buf bytes.Buffer
	openSection(&buf, "Test Section", true)
	if !strings.Contains(buf.String(), "<details open>") {
		t.Errorf("open=true should emit <details open>: %q", buf.String())
	}

	buf.Reset()
	openSection(&buf, "Test Section", false)
	if strings.Contains(buf.String(), "<details open>") {
		t.Errorf("open=false should NOT emit <details open>: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "<details>") {
		t.Errorf("open=false should still emit <details>: %q", buf.String())
	}
}
