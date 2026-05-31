package main

import (
	"fmt"
	"io"
	"net/url"
	"strings"
)

// markdown_helpers — small render helpers for the markdown report path.
//
// Goals:
//
//   - Card names are clickable Scryfall links so a Discord / forum
//     reader can hover or tap to see the full card text.
//   - Mana symbols stay as `{W}{U}` text — Discord doesn't render
//     custom MTG icons inline, but the brace notation is universally
//     readable and what every Magic-literate reader already expects.
//   - Tables are used only where they materially help (color balance,
//     turn-by-turn). Discord renders tables as plain monospace text,
//     so we keep tables narrow and avoid them in places where a bullet
//     list reads just as well.
//   - The structured GameplanScript (PR #902) gets a markdown section
//     that mirrors the text-report layout: turn-by-turn list,
//     branching decisions, graceful degradation paths.

// ScryfallSearchURL builds the canonical exact-name search URL for a
// card. The `!"..."` operator forces an exact match on Scryfall, so
// the resulting page lands on a single card (rather than a fuzzy
// match list) when the name resolves.
//
// Returns empty string for empty input — callers should fall back to
// the bare name string in that case.
func ScryfallSearchURL(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	q := url.QueryEscape(`!"` + name + `"`)
	return "https://scryfall.com/search?q=" + q
}

// scryfallLink wraps a card name in a clickable markdown link. Empty
// name → empty string. Discord renders this as a hyperlink with a
// preview card embed (when the URL is on its own line); GitHub /
// Reddit / forums render it as inline text+link.
func scryfallLink(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return fmt.Sprintf("[%s](%s)", name, ScryfallSearchURL(name))
}

// scryfallLinks applies scryfallLink to each entry and joins with sep.
// Used for combo piece lists ("A + B + C" → "[A](…) + [B](…) + [C](…)").
// Empty slice → empty string.
func scryfallLinks(names []string, sep string) string {
	if len(names) == 0 {
		return ""
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if linked := scryfallLink(n); linked != "" {
			out = append(out, linked)
		}
	}
	return strings.Join(out, sep)
}

// printMarkdownSummaryHeader writes a compact "TL;DR" block at the top
// of the markdown report. Designed for Discord — the first 8-12 lines
// of a Freya report are what someone sees in a chat preview, and
// having the archetype + bracket + win method + key cards visible
// without scrolling makes the report scannable.
//
// Falls back to a minimal header (deck name, card count) when the
// DeckProfile isn't populated.
func printMarkdownSummaryHeader(w io.Writer, r *FreyaReport) {
	fmt.Fprintf(w, "# %s\n\n", reportTitle(r))
	if r.Commander != "" {
		fmt.Fprintf(w, "**Commander:** %s\n\n", scryfallLink(r.Commander))
	}

	dp := r.Profile
	if dp == nil {
		fmt.Fprintf(w, "**Cards:** %d\n\n", r.TotalCards)
		return
	}

	// Compact summary table — single row, 4 columns. Discord renders
	// this as monospace plain text but the column shape stays
	// readable because the headers are short. GitHub / forums render
	// it as a real HTML table.
	fmt.Fprintf(w, "| Archetype | Bracket | Cards | Win method |\n")
	fmt.Fprintf(w, "|-----------|---------|-------|-----------|\n")
	winMethod := dp.PrimaryWinLine
	if winMethod == "" {
		winMethod = "—"
	}
	bracket := fmt.Sprintf("B%d (%s)", dp.Bracket, dp.BracketLabel)
	fmt.Fprintf(w, "| %s | %s | %d | %s |\n\n",
		dp.PrimaryArchetype, bracket, r.TotalCards, winMethod)

	if dp.GameplanSummary != "" {
		fmt.Fprintf(w, "> %s\n\n", dp.GameplanSummary)
	}
}

func reportTitle(r *FreyaReport) string {
	if r.DeckName != "" {
		return "Freya — " + r.DeckName
	}
	return "Freya Analysis"
}

// printGameplanScriptMarkdown writes the structured GameplanScript as
// markdown. Mirrors the text-report layout (turn-by-turn list +
// IF/THEN decisions + WHEN/recover degradation paths) but uses
// markdown emphasis instead of indentation so it reads cleanly when
// pasted into a Discord channel.
//
// Returns silently when script is nil (so the caller can invoke
// unconditionally).
func printGameplanScriptMarkdown(w io.Writer, script *GameplanScript) {
	if script == nil {
		return
	}
	if len(script.TurnByTurn) > 0 {
		fmt.Fprintf(w, "### Turn-by-Turn Sequence\n\n")
		for _, t := range script.TurnByTurn {
			fmt.Fprintf(w, "- **T%d:** %s", t.Turn, t.Action)
			if t.Note != "" {
				fmt.Fprintf(w, " — _%s_", t.Note)
			}
			fmt.Fprintf(w, "\n")
		}
		fmt.Fprintf(w, "\n")
	}
	if len(script.DecisionPoints) > 0 {
		fmt.Fprintf(w, "### Branching Decisions\n\n")
		for _, d := range script.DecisionPoints {
			fmt.Fprintf(w, "- **IF** %s\n", d.Trigger)
			fmt.Fprintf(w, "  - **THEN** %s\n", d.Action)
		}
		fmt.Fprintf(w, "\n")
	}
	if len(script.DegradationPaths) > 0 {
		fmt.Fprintf(w, "### Graceful Degradation\n\n")
		for _, d := range script.DegradationPaths {
			fmt.Fprintf(w, "- **WHEN** %s\n", d.Setback)
			fmt.Fprintf(w, "  - %s\n", d.Recover)
		}
		fmt.Fprintf(w, "\n")
	}
}
