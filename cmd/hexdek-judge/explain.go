package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// hexdek-judge --explain <rule-section> — single-helper batch report
// over the comprehensive CR citation index built by PR #934 + #948.
//
// What it does: takes a rule slug (with or without the `§` prefix),
// looks it up in the citation index, and renders a man-page-style
// report that consolidates everything the judge knows about the
// rule:
//
//   - CR text         — the canonical Description + broader SectionTitle
//   - Codebase checks — probes that implement the rule + engine
//                       invariants that cite it (forward direction)
//   - Related rules   — CR sub-sections the rule interacts with
//                       (cross-references from relatedRules)
//   - Resolved-issue  — historical fix history from CLAUDE.md's
//     history          Resolved table (the HistoricalFixes attached
//                       in PR #948)
//
// Distinct from the `--interactive` REPL's `index <rule>` intent
// (which prints a compact 5-line summary suitable for a transcript)
// in that `--explain` produces a one-shot, pipeable, full report
// with section headings suitable for inclusion in postmortems, judge
// dispatch summaries, or rule-question docs. Same data, different
// rendering.
//
// Exit status mirrors the other --check-* probes: 0 on a known rule,
// 1 when the rule isn't in the index — so a CI hook can gate on
// "is this CR sub-section implemented?" without parsing the output.

// runExplain is the CLI entry point. Looks up the rule, renders the
// report to outPath (or stdout when empty), returns whether the rule
// was known.
func runExplain(rule, outPath string) (bool, error) {
	if strings.TrimSpace(rule) == "" {
		return false, fmt.Errorf("--explain requires a rule slug (e.g. --explain 704.5f or --explain §704.5f)")
	}
	idx := BuildCitationIndex()
	e := idx.LookupByRule(rule)

	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return false, fmt.Errorf("create %s: %w", outPath, err)
		}
		defer f.Close()
		w = f
	}

	if e == nil {
		fmt.Fprintf(w, "No index entry for §%s.\n",
			strings.TrimPrefix(strings.TrimSpace(rule), "§"))
		fmt.Fprintf(w, "\nThe citation index knows about %d CR sub-sections — run --citation-index to see the full list.\n",
			idx.Counts.TotalRules)
		return false, nil
	}

	renderExplainReport(w, e)
	return true, nil
}

// renderExplainReport writes the man-page-style report for one
// CitationIndexEntry. Sections are always printed in the same
// canonical order (CR text → codebase checks → related rules →
// resolved-issue history); empty sections render as "(none)" rather
// than being skipped, so the reader's eye finds every header in the
// same place regardless of which rule is being explained.
func renderExplainReport(w io.Writer, e *CitationIndexEntry) {
	const bar = "═══════════════════════════════════════════════════════════════"

	fmt.Fprintln(w, bar)
	fmt.Fprintf(w, "  CR §%s — %s\n", e.Rule, e.Description)
	fmt.Fprintf(w, "  Section: %s\n", e.SectionTitle)
	fmt.Fprintln(w, bar)
	fmt.Fprintln(w)

	// --- CODEBASE CHECKS section ---
	fmt.Fprintln(w, "CODEBASE CHECKS")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Probes that check this rule:")
	if len(e.CheckedBy) == 0 {
		fmt.Fprintln(w, "    (none — rule cited via engine invariant only, not by a judge probe)")
	} else {
		for _, p := range e.CheckedBy {
			fmt.Fprintf(w, "    - %s%s\n", p, probeLocationHint(p))
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  Engine invariants that cite this rule:")
	if len(e.RelatedInvariants) == 0 {
		fmt.Fprintln(w, "    (none — pure probe coverage)")
	} else {
		for _, inv := range e.RelatedInvariants {
			fmt.Fprintf(w, "    - %s\n", inv)
		}
	}
	fmt.Fprintln(w)

	// --- RELATED RULES section ---
	fmt.Fprintln(w, "RELATED RULES")
	fmt.Fprintln(w)
	if len(e.RelatedRules) == 0 {
		fmt.Fprintln(w, "  (no cross-references in the relatedRules table — see CR for adjacent sub-sections)")
	} else {
		fmt.Fprintln(w, "  This rule interacts with:")
		// Sort for stable rendering even though BuildCitationIndex
		// already sorts; defensive.
		related := append([]string(nil), e.RelatedRules...)
		sort.Strings(related)
		for _, r := range related {
			fmt.Fprintf(w, "    - §%s\n", r)
		}
	}
	fmt.Fprintln(w)

	// --- RESOLVED-ISSUE HISTORY section ---
	fmt.Fprintln(w, "RESOLVED-ISSUE HISTORY (CLAUDE.md)")
	fmt.Fprintln(w)
	if len(e.HistoricalFixes) == 0 {
		fmt.Fprintln(w, "  No historical fixes in CLAUDE.md cite this rule.")
		fmt.Fprintln(w, "  (Either the rule has never had a recorded engine defect, or the citation")
		fmt.Fprintln(w, "   uses a different slug shape than what the parser matched — run")
		fmt.Fprintln(w, "   --citation-index | jq .unmapped_claudemd to see slugs that didn't link.)")
	} else {
		fmt.Fprintf(w, "  %d historical fix(es) in CLAUDE.md cite this rule:\n", len(e.HistoricalFixes))
		fmt.Fprintln(w)
		for _, fx := range e.HistoricalFixes {
			fmt.Fprintf(w, "  %s  %s\n", fx.Date, fx.Source)
			// Wrap the summary at ~72 cols for readability.
			for _, line := range wrapWords(fx.IssueSummary, 72) {
				fmt.Fprintf(w, "    %s\n", line)
			}
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w, bar)
}

// probeLocationHint maps a probe name to a "(file)" hint so the
// reader can jump straight to the implementation. Best-effort —
// probes whose source file isn't a direct name → path mapping
// (e.g. "interactive_what_sbas" which is dispatched from
// interactive.go but implemented via sba_probe.go) get a composite
// hint.
//
// Sync invariant: matches the probeRules keys in citation_index.go.
// A test pin asserts every key has a hint.
var probeFileHints = map[string]string{
	"mana_cost_check":             "cmd/hexdek-judge/mana_cost_check.go",
	"commander_check":             "cmd/hexdek-judge/commander_check.go",
	"deck_construction_check":     "cmd/hexdek-judge/deck_construction_check.go",
	"sba_probe":                   "cmd/hexdek-judge/sba_probe.go",
	"loki_replay_analysis":        "cmd/hexdek-judge/loki_replay.go",
	"interactive_what_sbas":       "cmd/hexdek-judge/interactive.go → sba_probe.go",
	"interactive_is_combat_legal": "cmd/hexdek-judge/interactive.go",
}

func probeLocationHint(probe string) string {
	if hint, ok := probeFileHints[probe]; ok {
		return "   (" + hint + ")"
	}
	return ""
}

// wrapWords breaks `s` into lines that are each at most `width`
// characters long, splitting on word boundaries. Used to keep the
// resolved-issue summaries readable in the report — they can be up
// to ~120 chars in the raw CLAUDE.md bold-prefix form.
func wrapWords(s string, width int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	current := words[0]
	for _, w := range words[1:] {
		if len(current)+1+len(w) <= width {
			current += " " + w
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
