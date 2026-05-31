package main

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// html_export — single-file HTML report for `--format html`.
//
// Design goals:
//
//   - Single self-contained file: inline CSS, no external JS, no
//     remote stylesheets. The output can be saved to disk, emailed,
//     or hosted at hexdek.dev/deck/{id} without any side-loading.
//   - Collapsible sections via native <details>/<summary> elements
//     (no JS dependency). The "Deck Profile" section is open by
//     default; everything else is collapsed so the page is scannable.
//   - Scryfall hyperlinks on every card name (reuses
//     ScryfallSearchURL from markdown_helpers.go for URL parity).
//   - Mana symbols rendered as small colored discs (W/U/B/R/G/C) via
//     CSS background colors + the letter — readable on any browser
//     without needing the Beleren / Mana font.
//   - Defensive HTML escaping on every user-provided value (deck
//     names, card names with apostrophes, descriptions) — the report
//     is hostable as-is.

// printHTML writes the full HTML report. Mirrors the section order of
// printText / printMarkdown but uses semantic HTML5 + inline CSS.
func printHTML(w io.Writer, r *FreyaReport) {
	fmt.Fprintf(w, "<!DOCTYPE html>\n")
	fmt.Fprintf(w, "<html lang=\"en\">\n")
	fmt.Fprintf(w, "<head>\n")
	fmt.Fprintf(w, "<meta charset=\"utf-8\">\n")
	fmt.Fprintf(w, "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(w, "<title>%s</title>\n", html.EscapeString(htmlReportTitle(r)))
	fmt.Fprintf(w, "<style>\n%s</style>\n", freyaHTMLStyle)
	fmt.Fprintf(w, "</head>\n")
	fmt.Fprintf(w, "<body>\n")
	fmt.Fprintf(w, "<main class=\"freya-report\">\n")

	renderHTMLHeader(w, r)
	renderHTMLProfileSection(w, r)
	renderHTMLCombosSection(w, r)
	renderHTMLFinishersSection(w, r)
	renderHTMLSynergiesSection(w, r)
	renderHTMLWinLinesSection(w, r)
	renderHTMLColorSection(w, r)
	renderHTMLManaCurveSection(w, r)
	renderHTMLTierListSection(w, r, LoadedTierList)
	renderHTMLFooter(w, r)

	fmt.Fprintf(w, "</main>\n")
	fmt.Fprintf(w, "</body>\n</html>\n")
}

func htmlReportTitle(r *FreyaReport) string {
	if r.DeckName != "" {
		return "Freya — " + r.DeckName
	}
	return "Freya Analysis"
}

// renderHTMLHeader writes the top banner: deck title, summary chips,
// gameplan blockquote. Designed to fit above-the-fold on a typical
// laptop screen so the reader sees the key signals immediately.
func renderHTMLHeader(w io.Writer, r *FreyaReport) {
	fmt.Fprintf(w, "<header class=\"deck-header\">\n")
	fmt.Fprintf(w, "  <h1>%s</h1>\n", html.EscapeString(htmlReportTitle(r)))
	if r.Commander != "" {
		fmt.Fprintf(w, "  <p class=\"commander\">Commander: %s</p>\n", cardLinkHTML(r.Commander))
	}

	dp := r.Profile
	fmt.Fprintf(w, "  <div class=\"badges\">\n")
	if dp != nil {
		if dp.PrimaryArchetype != "" {
			fmt.Fprintf(w, "    <span class=\"badge archetype\">%s</span>\n",
				html.EscapeString(dp.PrimaryArchetype))
		}
		if dp.Bracket > 0 {
			fmt.Fprintf(w, "    <span class=\"badge bracket\">B%d %s</span>\n",
				dp.Bracket, html.EscapeString(dp.BracketLabel))
		}
	}
	fmt.Fprintf(w, "    <span class=\"badge cards\">%d cards</span>\n", r.TotalCards)
	if len(r.ColorDemand) > 0 || (dp != nil && len(dp.ColorIdentity) > 0) {
		colors := []string{}
		if dp != nil && len(dp.ColorIdentity) > 0 {
			colors = dp.ColorIdentity
		} else {
			for _, c := range []string{"W", "U", "B", "R", "G"} {
				if r.ColorDemand[c] > 0 {
					colors = append(colors, c)
				}
			}
		}
		fmt.Fprintf(w, "    <span class=\"badge colors\">%s</span>\n", manaSymbolsHTML(colors))
	}
	fmt.Fprintf(w, "  </div>\n")

	if dp != nil && dp.GameplanSummary != "" {
		fmt.Fprintf(w, "  <blockquote class=\"gameplan-summary\">%s</blockquote>\n",
			html.EscapeString(dp.GameplanSummary))
	}
	fmt.Fprintf(w, "</header>\n")
}

func renderHTMLProfileSection(w io.Writer, r *FreyaReport) {
	dp := r.Profile
	if dp == nil {
		return
	}
	openSection(w, "Deck Profile", true)
	fmt.Fprintf(w, "<div class=\"profile-grid\">\n")
	if len(dp.Strengths) > 0 {
		fmt.Fprintf(w, "  <div class=\"strengths\">\n    <h3>Strengths</h3>\n    <ul>\n")
		for _, s := range dp.Strengths {
			fmt.Fprintf(w, "      <li>%s</li>\n", html.EscapeString(s))
		}
		fmt.Fprintf(w, "    </ul>\n  </div>\n")
	}
	if len(dp.Weaknesses) > 0 {
		fmt.Fprintf(w, "  <div class=\"weaknesses\">\n    <h3>Weaknesses</h3>\n    <ul>\n")
		for _, weak := range dp.Weaknesses {
			fmt.Fprintf(w, "      <li>%s</li>\n", html.EscapeString(weak))
		}
		fmt.Fprintf(w, "    </ul>\n  </div>\n")
	}
	fmt.Fprintf(w, "</div>\n")

	// Embedded turn-by-turn gameplan script (from PR #902).
	renderHTMLGameplanScript(w, dp.GameplanScript)

	closeSection(w)
}

func renderHTMLGameplanScript(w io.Writer, script *GameplanScript) {
	if script == nil {
		return
	}
	if len(script.TurnByTurn) > 0 {
		fmt.Fprintf(w, "<h3>Turn-by-Turn Sequence</h3>\n<ol class=\"turn-plan\">\n")
		for _, t := range script.TurnByTurn {
			fmt.Fprintf(w, "  <li><strong>T%d:</strong> %s",
				t.Turn, html.EscapeString(t.Action))
			if t.Note != "" {
				fmt.Fprintf(w, " <em>— %s</em>", html.EscapeString(t.Note))
			}
			fmt.Fprintf(w, "</li>\n")
		}
		fmt.Fprintf(w, "</ol>\n")
	}
	if len(script.DecisionPoints) > 0 {
		fmt.Fprintf(w, "<h3>Branching Decisions</h3>\n<dl class=\"decisions\">\n")
		for _, d := range script.DecisionPoints {
			fmt.Fprintf(w, "  <dt>IF %s</dt>\n", html.EscapeString(d.Trigger))
			fmt.Fprintf(w, "  <dd>THEN %s</dd>\n", html.EscapeString(d.Action))
		}
		fmt.Fprintf(w, "</dl>\n")
	}
	if len(script.DegradationPaths) > 0 {
		fmt.Fprintf(w, "<h3>Graceful Degradation</h3>\n<dl class=\"degradation\">\n")
		for _, d := range script.DegradationPaths {
			fmt.Fprintf(w, "  <dt>WHEN %s</dt>\n", html.EscapeString(d.Setback))
			fmt.Fprintf(w, "  <dd>%s</dd>\n", html.EscapeString(d.Recover))
		}
		fmt.Fprintf(w, "</dl>\n")
	}
}

func renderHTMLCombosSection(w io.Writer, r *FreyaReport) {
	infCount := len(r.TrueInfinites)
	detCount := len(r.Determined)
	if infCount == 0 && detCount == 0 {
		return
	}
	openSection(w, fmt.Sprintf("Combos (%d true infinite, %d determined)", infCount, detCount), false)

	if infCount > 0 {
		fmt.Fprintf(w, "<h3>True Infinites</h3>\n<ul class=\"combo-list\">\n")
		for _, c := range r.TrueInfinites {
			renderHTMLCombo(w, c)
		}
		fmt.Fprintf(w, "</ul>\n")
	}
	if detCount > 0 {
		fmt.Fprintf(w, "<h3>Determined Loops</h3>\n<ul class=\"combo-list\">\n")
		for _, c := range r.Determined {
			renderHTMLCombo(w, c)
		}
		fmt.Fprintf(w, "</ul>\n")
	}

	closeSection(w)
}

func renderHTMLCombo(w io.Writer, c ComboResult) {
	checkClass := "heuristic"
	checkGlyph := "🔍"
	if c.Confirmed {
		checkClass = "confirmed"
		checkGlyph = "✅"
	}
	fmt.Fprintf(w, "  <li class=\"combo %s\">\n", checkClass)
	fmt.Fprintf(w, "    <div class=\"combo-cards\"><span class=\"combo-flag\">%s</span> %s",
		checkGlyph, cardLinksHTML(c.Cards, " + "))
	if label := ComboClassLabel(c.Class); label != "" {
		fmt.Fprintf(w, " <span class=\"combo-class\">[%s]</span>", html.EscapeString(label))
	}
	fmt.Fprintf(w, "</div>\n")
	if c.Description != "" {
		// Mana symbols ({W}{U}{B}{R}{G}{C}) render as inline SVG via
		// RenderManaHTMLSafe — text chunks get HTML-escaped, SVG
		// chunks emit raw. The split-on-" | " has to happen on the
		// rendered output so the SVG fragments end up in the right half.
		parts := strings.SplitN(RenderManaHTMLSafe(c.Description), " | ", 2)
		fmt.Fprintf(w, "    <div class=\"combo-desc\">%s</div>\n", parts[0])
		if len(parts) > 1 {
			fmt.Fprintf(w, "    <div class=\"combo-outlets\"><strong>%s</strong></div>\n", parts[1])
		}
	}
	if c.NonDeterministic {
		fmt.Fprintf(w, "    <div class=\"combo-warn\">⚠️ non-deterministic (depends on random selection)</div>\n")
	}
	fmt.Fprintf(w, "  </li>\n")
}

func renderHTMLFinishersSection(w io.Writer, r *FreyaReport) {
	if len(r.Finishers) == 0 {
		return
	}
	openSection(w, fmt.Sprintf("Finishers (%d)", len(r.Finishers)), false)
	fmt.Fprintf(w, "<ul class=\"combo-list\">\n")
	for _, c := range r.Finishers {
		renderHTMLCombo(w, c)
	}
	fmt.Fprintf(w, "</ul>\n")
	closeSection(w)
}

func renderHTMLSynergiesSection(w io.Writer, r *FreyaReport) {
	if len(r.Synergies) == 0 {
		return
	}
	openSection(w, fmt.Sprintf("Synergies (%d)", len(r.Synergies)), false)
	fmt.Fprintf(w, "<ul class=\"combo-list synergies\">\n")
	for _, c := range r.Synergies {
		fmt.Fprintf(w, "  <li><strong>%s</strong>", cardLinksHTML(c.Cards, " + "))
		if c.Description != "" {
			fmt.Fprintf(w, " — %s", RenderManaHTMLSafe(c.Description))
		}
		fmt.Fprintf(w, "</li>\n")
	}
	fmt.Fprintf(w, "</ul>\n")
	closeSection(w)
}

func renderHTMLWinLinesSection(w io.Writer, r *FreyaReport) {
	if r.WinLines == nil || len(r.WinLines.WinLines) == 0 {
		return
	}
	openSection(w, fmt.Sprintf("Win Lines (%d)", len(r.WinLines.WinLines)), false)
	fmt.Fprintf(w, "<ul class=\"winline-list\">\n")
	for _, wl := range r.WinLines.WinLines {
		fmt.Fprintf(w, "  <li>")
		if wl.Tier != "" {
			fmt.Fprintf(w, "<span class=\"winline-tier tier-%s\">%s</span> ",
				strings.ToLower(html.EscapeString(wl.Tier)), html.EscapeString(wl.Tier))
		}
		fmt.Fprintf(w, "<strong>%s</strong>", cardLinksHTML(wl.Pieces, " + "))
		if wl.Type != "" {
			fmt.Fprintf(w, " <em>(%s)</em>", html.EscapeString(wl.Type))
		}
		if wl.Desc != "" {
			fmt.Fprintf(w, " — %s", html.EscapeString(wl.Desc))
		}
		fmt.Fprintf(w, "</li>\n")
	}
	fmt.Fprintf(w, "</ul>\n")
	closeSection(w)
}

func renderHTMLColorSection(w io.Writer, r *FreyaReport) {
	totalDemand := 0
	totalSupply := 0
	for _, v := range r.ColorDemand {
		totalDemand += v
	}
	for _, v := range r.ColorSupply {
		totalSupply += v
	}
	if totalDemand == 0 && totalSupply == 0 {
		return
	}
	openSection(w, "Color Balance", false)
	fmt.Fprintf(w, "<table class=\"color-table\">\n")
	fmt.Fprintf(w, "  <thead><tr><th>Color</th><th>Demand</th><th>Supply</th></tr></thead>\n")
	fmt.Fprintf(w, "  <tbody>\n")
	for _, c := range []string{"W", "U", "B", "R", "G", "C"} {
		if r.ColorDemand[c] == 0 && r.ColorSupply[c] == 0 {
			continue
		}
		dPct := 0.0
		sPct := 0.0
		if totalDemand > 0 {
			dPct = float64(r.ColorDemand[c]) / float64(totalDemand) * 100
		}
		if totalSupply > 0 {
			sPct = float64(r.ColorSupply[c]) / float64(totalSupply) * 100
		}
		fmt.Fprintf(w, "    <tr><td>%s</td><td>%.0f%%</td><td>%.0f%%</td></tr>\n",
			manaSymbolHTML(c), dPct, sPct)
	}
	fmt.Fprintf(w, "  </tbody>\n</table>\n")
	closeSection(w)
}

func renderHTMLManaCurveSection(w io.Writer, r *FreyaReport) {
	if r.NonlandCount == 0 {
		return
	}
	openSection(w, fmt.Sprintf("Mana Curve (avg %.1f, %s)", r.AvgCMC, r.CurveShape), false)
	maxCount := 0
	for _, count := range r.ManaCurve {
		if count > maxCount {
			maxCount = count
		}
	}
	fmt.Fprintf(w, "<div class=\"curve\">\n")
	for i, count := range r.ManaCurve {
		label := fmt.Sprintf("%d", i)
		if i == 7 {
			label = "7+"
		}
		barPct := 0
		if maxCount > 0 {
			barPct = count * 100 / maxCount
		}
		fmt.Fprintf(w, "  <div class=\"curve-row\"><span class=\"curve-label\">%s</span>"+
			"<span class=\"curve-bar\" style=\"width:%d%%\"></span>"+
			"<span class=\"curve-count\">%d</span></div>\n", label, barPct, count)
	}
	fmt.Fprintf(w, "</div>\n")
	fmt.Fprintf(w, "<p class=\"land-counts\">Lands: %d &middot; Nonlands: %d</p>\n", r.LandCount, r.NonlandCount)
	closeSection(w)
}

func renderHTMLFooter(w io.Writer, r *FreyaReport) {
	fmt.Fprintf(w, "<footer>\n")
	fmt.Fprintf(w, "  <p>Generated by hexdek-freya v%s</p>\n", html.EscapeString(FreyaVersion))
	fmt.Fprintf(w, "</footer>\n")
}

// openSection writes a <details>/<summary> opener. open=true makes the
// section expanded by default (used for the Deck Profile so the page
// loads with the most-important context visible).
func openSection(w io.Writer, title string, open bool) {
	attr := ""
	if open {
		attr = " open"
	}
	fmt.Fprintf(w, "<details%s>\n<summary><h2>%s</h2></summary>\n<div class=\"section-body\">\n",
		attr, html.EscapeString(title))
}

func closeSection(w io.Writer) {
	fmt.Fprintf(w, "</div>\n</details>\n")
}

// cardLinkHTML wraps a card name in an anchor pointing at its Scryfall
// search page. The exact-name search operator is reused from
// markdown_helpers.go's ScryfallSearchURL so both renderers agree on
// the URL contract. Empty names return empty string.
func cardLinkHTML(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return fmt.Sprintf(`<a class="card-link" href="%s" target="_blank" rel="noopener">%s</a>`,
		html.EscapeString(ScryfallSearchURL(name)), html.EscapeString(name))
}

// cardLinksHTML joins multiple card-link anchors with sep. Used for
// combo piece lists.
func cardLinksHTML(names []string, sep string) string {
	if len(names) == 0 {
		return ""
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if linked := cardLinkHTML(n); linked != "" {
			out = append(out, linked)
		}
	}
	return strings.Join(out, sep)
}

// manaSymbolHTML renders a single color letter as a small colored
// disc. The disc colors mirror the MTG mana symbol palette
// approximately (W = pale yellow, U = sky blue, B = greyish black,
// R = orange-red, G = pale green, C = light grey). The letter inside
// the disc keeps the symbol readable without a custom font.
func manaSymbolHTML(c string) string {
	letter := strings.ToUpper(strings.TrimSpace(c))
	switch letter {
	case "W", "U", "B", "R", "G", "C":
		return fmt.Sprintf(`<span class="mana-symbol mana-%s">%s</span>`,
			strings.ToLower(letter), letter)
	default:
		return html.EscapeString(c)
	}
}

// manaSymbolsHTML renders a sequence of color letters as adjacent
// discs. Used for the deck's color identity badge.
func manaSymbolsHTML(colors []string) string {
	var b strings.Builder
	for _, c := range colors {
		b.WriteString(manaSymbolHTML(c))
	}
	return b.String()
}

// freyaHTMLStyle is the inline CSS bundled into every report. Dark
// theme, monospaced body for the report sections, badge chips for the
// summary bar. Kept small enough to stay inline without ballooning the
// page size — a typical report adds ~5KB of style.
const freyaHTMLStyle = `
  :root {
    --bg: #0f1416;
    --fg: #e0e6e8;
    --muted: #889299;
    --accent: #6aaaef;
    --border: #2a3236;
    --confirmed: #4f9d6c;
    --heuristic: #b88a3a;
    --warn: #d97a4f;
    --tier-s: #d4af37;
    --tier-a: #7ca5d6;
    --tier-b: #8ebf95;
    --tier-c: #aaa78f;
    --tier-d: #8a8a8a;
  }
  * { box-sizing: border-box; }
  body {
    background: var(--bg);
    color: var(--fg);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    line-height: 1.5;
    margin: 0;
    padding: 0;
  }
  main.freya-report {
    max-width: 920px;
    margin: 0 auto;
    padding: 2rem 1.5rem 4rem;
  }
  header.deck-header {
    border-bottom: 1px solid var(--border);
    padding-bottom: 1.5rem;
    margin-bottom: 1.5rem;
  }
  h1 { font-size: 1.85rem; margin: 0 0 .25rem; }
  h2 { font-size: 1.15rem; margin: 0; display: inline; }
  h3 { font-size: 1rem; margin: 1rem 0 .35rem; color: var(--accent); }
  .commander { color: var(--muted); margin: .25rem 0 .9rem; }
  .badges { display: flex; flex-wrap: wrap; gap: .5rem; margin: .5rem 0; }
  .badge {
    display: inline-flex;
    align-items: center;
    padding: .15rem .6rem;
    background: #1a2125;
    border: 1px solid var(--border);
    border-radius: 999px;
    font-size: .85rem;
    color: var(--fg);
  }
  .badge.archetype { color: var(--accent); }
  .badge.bracket { color: var(--tier-s); }
  blockquote.gameplan-summary {
    border-left: 3px solid var(--accent);
    background: #131a1d;
    margin: .9rem 0 0;
    padding: .55rem .9rem;
    color: var(--fg);
    font-style: italic;
  }
  details {
    border: 1px solid var(--border);
    border-radius: 6px;
    margin: .55rem 0;
    background: #131a1d;
  }
  details > summary {
    cursor: pointer;
    padding: .55rem .9rem;
    list-style: none;
    user-select: none;
  }
  details > summary::-webkit-details-marker { display: none; }
  details > summary::before {
    content: "▸ ";
    color: var(--muted);
    margin-right: .35rem;
  }
  details[open] > summary::before { content: "▾ "; color: var(--accent); }
  .section-body { padding: 0 .9rem .9rem; }

  ul.combo-list { list-style: none; padding: 0; margin: 0; }
  ul.combo-list li.combo {
    padding: .55rem 0;
    border-top: 1px dashed #1f262a;
  }
  ul.combo-list li.combo:first-child { border-top: none; }
  .combo-cards { font-weight: 600; }
  .combo-flag { margin-right: .3rem; }
  .combo-cards .combo-class { color: var(--muted); font-weight: 400; font-size: .85em; }
  .combo-desc { color: var(--muted); margin: .15rem 0 0 1.4rem; font-size: .92em; }
  .combo-outlets { margin: .15rem 0 0 1.4rem; }
  .combo-warn { margin: .15rem 0 0 1.4rem; color: var(--warn); font-size: .85em; }
  li.confirmed .combo-flag { color: var(--confirmed); }
  li.heuristic .combo-flag { color: var(--heuristic); }

  ul.winline-list { list-style: none; padding: 0; margin: 0; }
  ul.winline-list li { padding: .35rem 0; border-top: 1px dashed #1f262a; }
  ul.winline-list li:first-child { border-top: none; }
  .winline-tier {
    display: inline-block;
    padding: 0 .35rem;
    border-radius: 3px;
    background: #1a2125;
    font-weight: 700;
    font-size: .8rem;
    margin-right: .3rem;
  }
  .winline-tier.tier-s { color: var(--tier-s); }
  .winline-tier.tier-a { color: var(--tier-a); }
  .winline-tier.tier-b { color: var(--tier-b); }
  .winline-tier.tier-c { color: var(--tier-c); }
  .winline-tier.tier-d { color: var(--tier-d); }

  .profile-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; }
  @media (max-width: 720px) { .profile-grid { grid-template-columns: 1fr; } }
  .profile-grid .strengths h3 { color: var(--confirmed); }
  .profile-grid .weaknesses h3 { color: var(--warn); }
  .profile-grid ul { margin: 0; padding-left: 1rem; }
  ol.turn-plan, dl.decisions, dl.degradation { margin: .25rem 0 .9rem; padding-left: 1.25rem; }
  dl.decisions dt, dl.degradation dt { font-weight: 600; margin-top: .35rem; }
  dl.decisions dd, dl.degradation dd { margin: 0 0 .15rem 1rem; color: var(--muted); }

  table.color-table {
    border-collapse: collapse;
    margin: .25rem 0;
  }
  table.color-table th, table.color-table td {
    padding: .25rem .65rem;
    text-align: left;
    border-bottom: 1px solid var(--border);
  }
  table.color-table th { color: var(--muted); font-weight: 500; }

  .mana-symbol {
    display: inline-block;
    width: 1.1em;
    height: 1.1em;
    border-radius: 50%;
    text-align: center;
    line-height: 1.1em;
    font-size: .78em;
    font-weight: 700;
    color: #000;
    margin: 0 .05em;
    border: 1px solid rgba(0,0,0,0.25);
  }
  .mana-w { background: #f8e7b9; }
  .mana-u { background: #b3ceea; }
  .mana-b { background: #a69f9d; color: #000; }
  .mana-r { background: #eb9f82; }
  .mana-g { background: #c4d3ca; }
  .mana-c { background: #d3d3d3; }

  .curve { font-family: monospace; }
  .curve-row {
    display: grid;
    grid-template-columns: 2rem 1fr 3rem;
    align-items: center;
    gap: .5rem;
    margin: .15rem 0;
  }
  .curve-label { color: var(--muted); }
  .curve-bar {
    background: var(--accent);
    height: .9rem;
    border-radius: 2px;
    min-width: 1px;
  }
  .curve-count { text-align: right; }
  .land-counts { color: var(--muted); }

  a.card-link {
    color: var(--accent);
    text-decoration: none;
    border-bottom: 1px dotted rgba(106, 170, 239, 0.4);
  }
  a.card-link:hover { border-bottom-style: solid; }

  footer { color: var(--muted); margin-top: 2rem; font-size: .85rem; }
`
