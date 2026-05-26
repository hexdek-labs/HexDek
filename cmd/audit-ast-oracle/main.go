// audit-ast-oracle — Cross-references every AST-dataset entry against
// the current Scryfall oracle corpus and emits a markdown report of
// discrepancies.
//
// Categories audited:
//
//	missing_in_oracle           AST entry whose card name no longer
//	                            appears in oracle-cards.json
//	oracle_text_drift           AST's cached oracle_text disagrees with
//	                            the current oracle text (post-WS norm)
//	type_line_drift             type_line mismatch
//	cmc_drift                   numeric mana value mismatch
//	mana_cost_drift             mana_cost string mismatch
//	ast_keyword_hallucination   AST has Keyword.name nodes that don't
//	                            appear as substrings of the current
//	                            oracle text — the parser produced an
//	                            ability reference the printed card
//	                            doesn't actually have
//
// Findings-only — the tool NEVER mutates the data files. Output is a
// markdown report at --out (default docs/audit-ast-vs-oracle-r60.md)
// with per-category counts plus top-N examples per category.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

func main() {
	astPath := flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL")
	oraclePath := flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards JSON")
	outPath := flag.String("out", "docs/audit-ast-vs-oracle-r60.md", "markdown report output path")
	topN := flag.Int("top", 50, "examples per category in the report (counts include all findings)")
	flag.Parse()

	log.Printf("loading oracle corpus from %s ...", *oraclePath)
	oracle, err := loadOracle(*oraclePath)
	if err != nil {
		log.Fatalf("loadOracle: %v", err)
	}
	log.Printf("  loaded %d unique oracle entries", len(oracle))

	log.Printf("auditing AST dataset at %s ...", *astPath)
	findings, scanned, err := auditAST(*astPath, oracle)
	if err != nil {
		log.Fatalf("auditAST: %v", err)
	}
	log.Printf("  scanned %d AST entries, found %d discrepancies", scanned, len(findings))

	if err := writeReport(*outPath, findings, scanned, len(oracle), *topN); err != nil {
		log.Fatalf("writeReport: %v", err)
	}
	log.Printf("wrote %s", *outPath)
	summary := summarizeCounts(findings)
	log.Printf("  summary: %s", summary)
}

// loadOracle reads oracle-cards.json into a name→entry map.
//
// Tokens and Un-sets are filtered out before deduping: Scryfall's bulk
// dump includes token-shaped entries that share names with real cards
// (e.g., "Adorned Pouncer" the creature vs "Adorned Pouncer" the
// double-strike token in Hour of Devastation Tokens). The token entry's
// oracle text is stripped of set-specific abilities like Eternalize,
// which would otherwise corrupt the keyword-hallucination check —
// asserting that the parser hallucinated "eternalize" when in fact
// the parser was correct and we were comparing against the wrong entry.
func loadOracle(path string) (map[string]oracleEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var raw []oracleEntry
	dec := json.NewDecoder(f)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	unSets := map[string]bool{
		"Unstable":     true,
		"Unhinged":     true,
		"Unglued":      true,
		"Unsanctioned": true,
		"Unfinity":     true,
	}
	out := make(map[string]oracleEntry, len(raw))
	for _, e := range raw {
		if e.Name == "" || e.Layout == "token" || unSets[e.SetName] {
			continue
		}
		if _, ok := out[e.Name]; !ok {
			out[e.Name] = e
		}
	}
	return out, nil
}

// auditAST streams the AST dataset line-by-line, running every checker
// against each entry's matching oracle entry (if any). Returns the
// flat findings slice plus the number of AST entries scanned.
func auditAST(path string, oracle map[string]oracleEntry) ([]Discrepancy, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	// AST dataset entries can be long (multi-paragraph oracle text).
	// 1MB buffer is overkill but bounded and cheap.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var findings []Discrepancy
	scanned := 0
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry astEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			log.Printf("  line %d: decode error: %v (skipped)", lineNum, err)
			continue
		}
		scanned++
		findings = append(findings, auditEntry(entry, oracle)...)
	}
	if err := scanner.Err(); err != nil {
		return nil, scanned, err
	}
	return findings, scanned, nil
}

// auditEntry runs every checker against one AST entry. Returns all
// findings produced (a single entry can produce up to N findings —
// one per category).
func auditEntry(a astEntry, oracle map[string]oracleEntry) []Discrepancy {
	o, ok := oracle[a.Name]
	if !ok {
		return []Discrepancy{{
			Name:     a.Name,
			Category: CatMissingInOracle,
			Details:  "AST entry has no matching oracle-corpus card",
		}}
	}
	var out []Discrepancy
	if d := checkOracleTextDrift(a, o); d != nil {
		out = append(out, *d)
	}
	if d := checkTypeLineDrift(a, o); d != nil {
		out = append(out, *d)
	}
	if d := checkCMCDrift(a, o); d != nil {
		out = append(out, *d)
	}
	if d := checkManaCostDrift(a, o); d != nil {
		out = append(out, *d)
	}
	if d := checkASTHallucination(a, canonicalOracleText(o)); d != nil {
		out = append(out, *d)
	}
	return out
}

// summarizeCounts builds the one-line stderr summary that mirrors the
// "Headline" section of the markdown report.
func summarizeCounts(findings []Discrepancy) string {
	counts := map[string]int{}
	for _, d := range findings {
		counts[d.Category]++
	}
	parts := make([]string, 0, len(CategoryOrder))
	for _, c := range CategoryOrder {
		parts = append(parts, fmt.Sprintf("%s=%d", c, counts[c]))
	}
	return strings.Join(parts, ", ")
}

// writeReport renders the markdown discrepancy report. The shape is
// stable: headline counts → per-category sections in CategoryOrder →
// reading guide footer. Each section truncates to topN examples; the
// per-section count reports the FULL tally.
func writeReport(path string, findings []Discrepancy, scanned, oracleCount, topN int) error {
	if dir := dirOf(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	counts := map[string][]Discrepancy{}
	for _, d := range findings {
		counts[d.Category] = append(counts[d.Category], d)
	}
	for cat := range counts {
		sort.Slice(counts[cat], func(i, j int) bool {
			return counts[cat][i].Name < counts[cat][j].Name
		})
	}

	fmt.Fprintf(f, "# Audit: AST Dataset vs Scryfall Oracle Corpus (R60)\n\n")
	fmt.Fprintf(f, "Cross-references every entry in `data/rules/ast_dataset.jsonl` against\n")
	fmt.Fprintf(f, "`data/rules/oracle-cards.json` and reports per-category discrepancies.\n")
	fmt.Fprintf(f, "**Findings-only — no data files were modified.**\n\n")

	fmt.Fprintf(f, "## Headline\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(f, "| AST entries scanned | %d |\n", scanned)
	fmt.Fprintf(f, "| Oracle entries indexed | %d |\n", oracleCount)
	fmt.Fprintf(f, "| Total discrepancies | %d |\n", len(findings))
	for _, cat := range CategoryOrder {
		fmt.Fprintf(f, "| `%s` | %d |\n", cat, len(counts[cat]))
	}
	fmt.Fprintln(f)

	fmt.Fprintf(f, "## Categories\n\n")
	for _, cat := range CategoryOrder {
		writeCategorySection(f, cat, counts[cat], topN)
	}

	fmt.Fprintf(f, "## Reading this report\n\n")
	fmt.Fprintf(f, "- **`missing_in_oracle`** — the AST dataset contains a card name with no\n")
	fmt.Fprintf(f, "  matching entry in `oracle-cards.json`. Usually a renamed Scryfall\n")
	fmt.Fprintf(f, "  entry (split/adventure faces, errata names) or a card removed from\n")
	fmt.Fprintf(f, "  the corpus. Action: re-fetch oracle-cards.json then re-audit before\n")
	fmt.Fprintf(f, "  re-ingesting the AST dataset.\n")
	fmt.Fprintf(f, "- **`oracle_text_drift`** — the AST entry's cached oracle text disagrees\n")
	fmt.Fprintf(f, "  with the current Scryfall text (after whitespace normalization). The\n")
	fmt.Fprintf(f, "  snippet shows the first byte where the strings diverge — useful for\n")
	fmt.Fprintf(f, "  spotting reminder-text revisions vs substantive errata.\n")
	fmt.Fprintf(f, "- **`type_line_drift` / `cmc_drift` / `mana_cost_drift`** — metadata\n")
	fmt.Fprintf(f, "  drift, usually a Scryfall-side normalization (case, em-dash) but\n")
	fmt.Fprintf(f, "  occasionally a real type-line addition (e.g., Battle subtype) that\n")
	fmt.Fprintf(f, "  the AST has not re-ingested.\n")
	fmt.Fprintf(f, "- **`ast_keyword_hallucination`** — the AST contains `Keyword` nodes\n")
	fmt.Fprintf(f, "  whose `name` field does not appear as a substring in the current\n")
	fmt.Fprintf(f, "  oracle text. Strongest signal of parser error: an ability was\n")
	fmt.Fprintf(f, "  asserted that the printed card does not have. Common false-positive\n")
	fmt.Fprintf(f, "  sources are filtered via the parser-internal alias skip list.\n\n")

	fmt.Fprintf(f, "## Reproducing this report\n\n")
	fmt.Fprintf(f, "```\ngo run ./cmd/audit-ast-oracle --out %s --top %d\n```\n",
		path, topN)
	return nil
}

func writeCategorySection(f *os.File, cat string, entries []Discrepancy, topN int) {
	fmt.Fprintf(f, "### `%s` — %d findings\n\n", cat, len(entries))
	if len(entries) == 0 {
		fmt.Fprintf(f, "_None detected._\n\n")
		return
	}
	shown := len(entries)
	if topN > 0 && topN < shown {
		shown = topN
	}
	fmt.Fprintf(f, "| # | Card | Detail |\n|---:|---|---|\n")
	for i := 0; i < shown; i++ {
		e := entries[i]
		fmt.Fprintf(f, "| %d | %s | %s |\n",
			i+1, escapeCell(e.Name), escapeCell(e.Details))
	}
	if shown < len(entries) {
		fmt.Fprintf(f, "\n_… %d more (run with `--top 0` to dump all)._\n", len(entries)-shown)
	}
	fmt.Fprintln(f)
}

// escapeCell guards markdown table cells against pipes (cell breaks)
// and embedded newlines (row breaks). Backticks are preserved since
// the Details snippets often contain valid mana / type-line fragments
// we want rendered as-is.
func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func dirOf(path string) string {
	// Lightweight filepath.Dir replacement — keeps the import surface
	// minimal and the tool standalone.
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}
