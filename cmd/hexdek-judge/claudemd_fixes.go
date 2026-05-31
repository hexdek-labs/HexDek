package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

// hexdek-judge — CLAUDE.md ↔ citation-index cross-reference.
//
// PR #934 built a comprehensive CR citation index keying every rule
// the judge probes implement to the probes that check it + the
// engine invariants that cite it. That covered the FORWARD direction
// — "given this rule, what code checks it?" — but the HISTORICAL
// direction was a gap: the Resolved table in CLAUDE.md carries the
// canonical record of every engine bug fix that touched a CR
// section, but no programmatic link tied a citation entry back to
// the resolved-issue rows.
//
// This file closes the cross-reference. parseClaudemdResolvedFixes
// walks the CLAUDE.md Resolved table, extracts every CR citation
// from each row's Issue + Resolution columns, and yields a
// rule → []ClaudemdFix backlink. BuildCitationIndex now populates
// the new HistoricalFixes field on each CitationIndexEntry, so a
// `judge --citation-index` dump (or `judge --interactive`'s
// `index <rule>` lookup) surfaces every historical fix that touched
// the rule alongside the probe coverage.
//
// Use case: a tournament judge debugging a §704.5f hit can run
// `judge --interactive` → `index 704.5f` and immediately see (a) the
// probes that currently check the rule, (b) the engine invariants
// that emit citations to it, AND (c) the bug-fix history — Aziza
// fabrication closure, District Mascot 0/0 survival, ZoneCastGrant
// expiry sweep, etc. — every time the rule was tickled by a real
// engine defect, with the dispatch branch + one-line summary.
//
// Schema: rows look like
//
//   | YYYY-MM-DD | source-or-branch | **bold-issue-summary** … | Closed. resolution-detail … |
//
// where the date is the first pipe-separated cell, the source is the
// second (often a branch name backticked or a PR ref), the issue +
// resolution are the third and fourth. The parser extracts every
// `§\d+(\.\d+[a-z]?)?` substring from columns 3+4 — anything
// matching the canonical CR rule-slug shape becomes a backlink key.

// ClaudemdFix is one historical bug-fix backlink for a CR rule.
type ClaudemdFix struct {
	Date         string `json:"date"`           // "2026-05-30"
	Source       string `json:"source"`         // branch / PR ref / audit name from column 2
	IssueSummary string `json:"issue_summary"`  // first bolded sentence from the Issue column (truncated)
}

// claudemdRuleRE matches a CR rule-slug as it appears in CLAUDE.md.
// Two surface forms cover ~all citations:
//
//   - "CR §704.5a" / "CR §704.5" / "CR §122.1g"   (full form)
//   - "§704.5a" / "§122.1g"                        (bare § form)
//
// Both reduce to the canonical slug "704.5a" / "122.1g" / "704.5"
// after stripping the "CR " / "§" prefix. Rule numbers must be at
// least 3 digits — CR sections start at §100 (CR §1xx general,
// §2xx parts, §3xx card types, etc.); a "§2" or "§3.1" match in
// CLAUDE.md is almost always a doc-section reference (e.g.
// "docs/engine-event-registry.md §3.1 audit"), not a CR citation.
var claudemdRuleRE = regexp.MustCompile(`(?:CR\s+)?§(\d{3}(?:\.\d+[a-z]?)?)`)

// parseClaudemdResolvedFixes reads a CLAUDE.md file and returns the
// rule → []ClaudemdFix backlink map. Walks the file looking for the
// `### Resolved` section header, then parses every subsequent
// pipe-table row (one per line) until EOF or the next `###` heading.
//
// Each row contributes one fix per UNIQUE rule slug found in the
// Issue + Resolution cells — a row that cites §704.5a three times
// in different sentences contributes one entry to the §704.5a
// backlink, not three.
//
// Skips:
//   - Lines before `### Resolved`
//   - Markdown table header / separator rows (`| Date | Source | …`,
//     `|---|---|---|---|`)
//   - Lines after the next `### …` heading (no current second
//     section under Resolved; defensive against future structure
//     drift)
func parseClaudemdResolvedFixes(path string) (map[string][]ClaudemdFix, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CLAUDE.md: %w", err)
	}
	defer f.Close()
	return parseClaudemdResolvedFixesFromReader(f)
}

// parseClaudemdResolvedFixesFromReader is the io.Reader variant used
// by tests with synthetic input.
func parseClaudemdResolvedFixesFromReader(r io.Reader) (map[string][]ClaudemdFix, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	out := map[string][]ClaudemdFix{}
	inResolved := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !inResolved {
			if strings.HasPrefix(trimmed, "### Resolved") {
				inResolved = true
			}
			continue
		}
		// We're in the Resolved section now.
		if strings.HasPrefix(trimmed, "### ") {
			// New section started — stop.
			break
		}
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		// Skip header + separator rows.
		if strings.Contains(trimmed, "| Date |") || strings.Contains(trimmed, "|------|") ||
			strings.HasPrefix(trimmed, "|---") || strings.HasPrefix(trimmed, "| ---") {
			continue
		}
		row := parseClaudemdRow(line)
		if row == nil {
			continue
		}
		// Collect unique CR slugs across issue + resolution.
		seen := map[string]bool{}
		blob := row.Issue + " " + row.Resolution
		for _, m := range claudemdRuleRE.FindAllStringSubmatch(blob, -1) {
			slug := strings.ToLower(m[1])
			if seen[slug] {
				continue
			}
			seen[slug] = true
			out[slug] = append(out[slug], ClaudemdFix{
				Date:         row.Date,
				Source:       row.Source,
				IssueSummary: row.IssueSummary,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan CLAUDE.md: %w", err)
	}
	// Sort each rule's fixes by date desc, then by source for stability.
	for k, v := range out {
		sort.Slice(v, func(i, j int) bool {
			if v[i].Date != v[j].Date {
				return v[i].Date > v[j].Date
			}
			return v[i].Source < v[j].Source
		})
		out[k] = v
	}
	return out, nil
}

// resolvedRow holds the parsed columns of one Resolved-table row.
type resolvedRow struct {
	Date         string
	Source       string
	Issue        string
	Resolution   string
	IssueSummary string
}

// parseClaudemdRow splits a pipe-table row into its 4 columns and
// returns the resolvedRow. Returns nil for malformed rows (fewer than
// 4 columns or a non-date first cell).
func parseClaudemdRow(line string) *resolvedRow {
	// Trim outer pipes + whitespace, then split on " | " (the inner
	// markdown convention). The Issue + Resolution cells can contain
	// embedded backticks and dollar signs but no raw pipe.
	t := strings.TrimSpace(line)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, " | ")
	if len(parts) < 4 {
		return nil
	}
	date := strings.TrimSpace(parts[0])
	if !looksLikeDate(date) {
		return nil
	}
	row := &resolvedRow{
		Date:       date,
		Source:     strings.TrimSpace(parts[1]),
		Issue:      strings.TrimSpace(parts[2]),
		Resolution: strings.TrimSpace(strings.Join(parts[3:], " | ")),
	}
	row.IssueSummary = extractIssueSummary(row.Issue)
	return row
}

// looksLikeDate reports whether a string matches a YYYY-MM-DD shape.
var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func looksLikeDate(s string) bool {
	return dateRE.MatchString(s)
}

// extractIssueSummary pulls a compact one-line headline from the
// Issue cell. The Resolved table follows a consistent convention:
// each Issue cell opens with a **bold sentence** containing the
// headline, then continues with prose. The summary is the bold
// sentence (or, if no bold, the first 120 chars).
var boldRE = regexp.MustCompile(`\*\*([^*]+)\*\*`)

func extractIssueSummary(issue string) string {
	if m := boldRE.FindStringSubmatch(issue); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// No bold opener — truncate the prose at the first em-dash or 120 chars.
	s := strings.TrimSpace(issue)
	if i := strings.Index(s, " — "); i > 0 && i < 200 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

// MergeClaudemdFixesIntoIndex walks the citation index and attaches
// each rule's historical fixes from the parsed CLAUDE.md map. Rules
// in the map that aren't in the index are recorded in the returned
// `unmappedSlugs` slice so the caller can surface the gap — a CR
// citation in CLAUDE.md that the judge doesn't know about means
// either (a) the judge should add probe coverage for it, or (b) the
// citation in CLAUDE.md is informational-only.
func MergeClaudemdFixesIntoIndex(idx *CitationIndex, fixes map[string][]ClaudemdFix) (unmappedSlugs []string) {
	for rule, list := range fixes {
		e, ok := idx.Entries[rule]
		if !ok {
			unmappedSlugs = append(unmappedSlugs, rule)
			continue
		}
		e.HistoricalFixes = append(e.HistoricalFixes, list...)
	}
	sort.Strings(unmappedSlugs)
	return unmappedSlugs
}

// claudemdPath resolves the CLAUDE.md location for the citation
// index. Honors the CLAUDEMD_PATH env var (used by tests with
// synthetic fixtures) and falls back to the worktree-root CLAUDE.md.
func claudemdPath() string {
	if p := os.Getenv("CLAUDEMD_PATH"); p != "" {
		return p
	}
	return "CLAUDE.md"
}
