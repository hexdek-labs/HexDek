package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
)

// ---------------------------------------------------------------------------
// Tier list HTML rendering — collapsible <details>/<summary> section in
// the per-deck Freya HTML report (--format html) showing the cards the
// corpus-derived tier list recommends for the deck's primary archetype.
//
// Loads a pre-computed TierListExport JSON via --tier-list <path>
// (produced by --all-decks --tier-list-out from PR #959). The HTML
// renderer looks up the deck's primary archetype in the loaded export
// and surfaces matching tier-list entries; archetypes absent from the
// export render no section (silent skip — the corpus didn't cover
// that archetype).
//
// Wiring follows the FocusMode precedent (report.go:40): a single
// package-level LoadedTierList variable, set once from main(), read by
// the renderer. Avoids threading an extra argument through the
// PrintReport / printHTML signature.
// ---------------------------------------------------------------------------

// LoadedTierList holds the in-memory parse of the tier-list JSON
// loaded via the --tier-list CLI flag. nil = no flag passed, no
// section rendered. Set from main() before PrintReport.
var LoadedTierList *TierListExport

// LoadTierList parses a TierListExport JSON file from path. On success
// sets the package-level LoadedTierList. Returns the parse error
// without mutating LoadedTierList on failure so callers can log + fall
// back to rendering without the section.
func LoadTierList(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read tier list %s: %w", path, err)
	}
	var tl TierListExport
	if err := json.Unmarshal(data, &tl); err != nil {
		return fmt.Errorf("parse tier list %s: %w", path, err)
	}
	LoadedTierList = &tl
	return nil
}

// renderHTMLTierListSection emits a collapsible <details> block listing
// the top tier-list cards for the report's primary archetype. No-op
// when:
//   - tl is nil (no --tier-list flag passed)
//   - the report has no primary archetype identified
//   - the archetype isn't in the loaded export (corpus didn't cover it)
//   - the matching ArchetypeTierList has zero top cards
//
// Section is collapsed by default (open=false) since it's
// supplementary — readers expand it intentionally when they want the
// "what should I auto-include" view.
func renderHTMLTierListSection(w io.Writer, r *FreyaReport, tl *TierListExport) {
	if tl == nil || r == nil || r.Profile == nil {
		return
	}
	arch := r.Profile.PrimaryArchetype
	if arch == "" {
		return
	}
	var match *ArchetypeTierList
	for i := range tl.Archetypes {
		if tl.Archetypes[i].Archetype == arch {
			match = &tl.Archetypes[i]
			break
		}
	}
	if match == nil || len(match.TopCards) == 0 {
		return
	}

	title := fmt.Sprintf("Auto-includes for %s (corpus-derived tier list)", arch)
	openSection(w, title, false)
	fmt.Fprintf(w,
		"<p class=\"tier-list-meta\">Ranked across %d %s decks (avg bracket %.1f). "+
			"Score = inclusion rate × (1 + win-impact); win-impact is the bracket "+
			"delta between decks containing the card and decks not.</p>\n",
		match.DeckCount, html.EscapeString(arch), match.AvgBracket)
	fmt.Fprintf(w, "<table class=\"tier-list-table\">\n")
	fmt.Fprintf(w, "  <thead><tr>"+
		"<th>#</th>"+
		"<th>Card</th>"+
		"<th>In</th>"+
		"<th>Rate</th>"+
		"<th>Avg B (with)</th>"+
		"<th>Avg B (without)</th>"+
		"<th>Win impact</th>"+
		"<th>Score</th>"+
		"</tr></thead>\n")
	fmt.Fprintf(w, "  <tbody>\n")
	for i, c := range match.TopCards {
		fmt.Fprintf(w,
			"    <tr><td>%d</td><td>%s</td>"+
				"<td>%d/%d</td><td>%.0f%%</td>"+
				"<td>%.2f</td><td>%.2f</td>"+
				"<td>%+.2f</td><td>%.2f</td></tr>\n",
			i+1, cardLinkHTML(c.Name),
			c.InclusionCount, match.DeckCount, 100*c.InclusionRate,
			c.AvgBracketWith, c.AvgBracketWithout,
			c.WinImpact, c.TierScore)
	}
	fmt.Fprintf(w, "  </tbody>\n</table>\n")
	closeSection(w)
}
