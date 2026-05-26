// audit-engine-dead — Static analysis of internal/gameengine/ to
// surface dead-branch candidates without runtime instrumentation.
//
// Two categories audited (Phase 1D scope):
//
//	exported_but_test_only        Exported functions/methods declared
//	                              in non-test code whose only refs
//	                              outside the declaring file are in
//	                              test files (no production code calls).
//
//	unused_switch_case_literals   Switch case arms whose string
//	                              literal value appears nowhere else
//	                              in the codebase — strong signal of
//	                              an emitter that was removed without
//	                              the matching consumer.
//
// Findings-only — the tool never mutates source. Output is a markdown
// report at --out (default docs/audit-engine-dead-branches-r60.md).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	dir := flag.String("dir", "internal/gameengine", "package directory to audit")
	refScan := flag.String("ref-scan", "internal,cmd", "comma-separated extra directories whose .go files contribute references but NOT declarations. Default scans every callable surface in the module.")
	outPath := flag.String("out", "docs/audit-engine-dead-branches-r60.md", "markdown report output path")
	topN := flag.Int("top", 100, "examples per category in the report (counts include all findings)")
	flag.Parse()

	var refScanDirs []string
	for _, d := range strings.Split(*refScan, ",") {
		d = strings.TrimSpace(d)
		if d != "" {
			refScanDirs = append(refScanDirs, d)
		}
	}

	log.Printf("analyzing %s (ref-scan: %v) ...", *dir, refScanDirs)
	result, err := AnalyzePackageWithScope(*dir, refScanDirs)
	if err != nil {
		log.Fatalf("AnalyzePackage: %v", err)
	}
	log.Printf("  %d declarations / %d references", result.TotalDecls, result.TotalRefs)
	log.Printf("  exported test-only: %d", len(result.ExportedTestOnly))
	log.Printf("  unused switch cases: %d", len(result.UnusedSwitchCases))

	if err := writeReport(*outPath, *dir, result, *topN); err != nil {
		log.Fatalf("writeReport: %v", err)
	}
	log.Printf("wrote %s", *outPath)
}

func writeReport(path, dir string, r *AnalyzeResult, topN int) error {
	if dirOf := dirname(path); dirOf != "" && dirOf != "." {
		if err := os.MkdirAll(dirOf, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# Audit: Engine Dead Branches (R60 Phase 1D)\n\n")
	fmt.Fprintf(f, "Static analysis of `%s` to surface dead-branch candidates without runtime\n", dir)
	fmt.Fprintf(f, "instrumentation. **Findings-only — no source files were modified.**\n\n")

	fmt.Fprintf(f, "## Headline\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|---|---:|\n")
	fmt.Fprintf(f, "| Declarations scanned | %d |\n", r.TotalDecls)
	fmt.Fprintf(f, "| Identifier references counted | %d |\n", r.TotalRefs)
	fmt.Fprintf(f, "| `exported_but_test_only` findings | %d |\n", len(r.ExportedTestOnly))
	fmt.Fprintf(f, "| `unused_switch_case_literals` findings | %d |\n", len(r.UnusedSwitchCases))
	fmt.Fprintln(f)

	// Category 1: exported helpers called only from tests.
	fmt.Fprintf(f, "## `exported_but_test_only` — %d findings\n\n",
		len(r.ExportedTestOnly))
	fmt.Fprintf(f, "Exported function or method declared in non-test code whose only\n")
	fmt.Fprintf(f, "references outside the declaring file are in `_test.go` files.\n")
	fmt.Fprintf(f, "Strong signal that the API surface exists only to support tests —\n")
	fmt.Fprintf(f, "either the production caller was deleted (dead code) or the helper\n")
	fmt.Fprintf(f, "should be unexported and moved next to the consumer.\n\n")
	fmt.Fprintf(f, "_Note: methods are matched by simple name; if two unrelated types\n")
	fmt.Fprintf(f, "expose the same method name, references conflate. Verify before acting._\n\n")
	if len(r.ExportedTestOnly) == 0 {
		fmt.Fprintf(f, "_None detected._\n\n")
	} else {
		shown := len(r.ExportedTestOnly)
		if topN > 0 && topN < shown {
			shown = topN
		}
		fmt.Fprintf(f, "| # | Symbol | Receiver | File:Line |\n|---:|---|---|---|\n")
		for i := 0; i < shown; i++ {
			d := r.ExportedTestOnly[i]
			recv := d.Receiver
			if recv == "" {
				recv = "—"
			}
			fmt.Fprintf(f, "| %d | `%s` | `%s` | `%s:%d` |\n",
				i+1, d.Name, recv, d.File, d.Line)
		}
		if shown < len(r.ExportedTestOnly) {
			fmt.Fprintf(f, "\n_… %d more (run with `--top 0` to dump all)._\n",
				len(r.ExportedTestOnly)-shown)
		}
		fmt.Fprintln(f)
	}

	// Category 2: unused switch case literals.
	fmt.Fprintf(f, "## `unused_switch_case_literals` — %d findings\n\n",
		len(r.UnusedSwitchCases))
	fmt.Fprintf(f, "Switch case arm whose string-literal value appears nowhere else\n")
	fmt.Fprintf(f, "in the codebase (every other string literal in `%s` was searched).\n", dir)
	fmt.Fprintf(f, "Strong signal that the matching emitter was removed without the\n")
	fmt.Fprintf(f, "consumer; the case is unreachable.\n\n")
	fmt.Fprintf(f, "**False-positive sources** to verify before acting:\n")
	fmt.Fprintf(f, "- **Card-name switches** (`switch name { case \"Storm-Kiln Artist\": ... }`):\n")
	fmt.Fprintf(f, "  the literal is a card name compared against `p.Card.Name`, which the\n")
	fmt.Fprintf(f, "  engine reads from the JSON oracle dataset, not from Go source. Every\n")
	fmt.Fprintf(f, "  such case will appear here even when the per-card handler is live.\n")
	fmt.Fprintf(f, "  Verify by checking the switch's tag column: tags like `name`, `Card.Name`,\n")
	fmt.Fprintf(f, "  or `p.Card.Name` strongly suggest a data-driven dispatch, not a dead case.\n")
	fmt.Fprintf(f, "- **AST modification-kind switches** (`switch mod.ModKind { case \"tri_tribe_anthem\": ... }`):\n")
	fmt.Fprintf(f, "  the literal is emitted by the Python parser (`scripts/mtg_ast.py`),\n")
	fmt.Fprintf(f, "  not by Go code. Cross-check against the parser's emitter table.\n")
	fmt.Fprintf(f, "- **Runtime-constructed strings** (`fmt.Sprintf`, `event.Kind = base + \"x\"`)\n")
	fmt.Fprintf(f, "  can produce values this scan misses.\n")
	fmt.Fprintf(f, "- **References from outside the module** (data dumps, generated configs)\n")
	fmt.Fprintf(f, "  aren't scanned.\n\n")
	if len(r.UnusedSwitchCases) == 0 {
		fmt.Fprintf(f, "_None detected._\n\n")
	} else {
		// "By switch tag" triage summary. Most findings cluster into
		// a handful of switch-tag expressions; the reader can scan this
		// first and drill into the ones whose tag doesn't look data-
		// driven.
		tagCounts := map[string]int{}
		for _, c := range r.UnusedSwitchCases {
			tag := c.SwitchTag
			if tag == "" {
				tag = "(no tag)"
			}
			tagCounts[tag]++
		}
		type tc struct {
			Tag   string
			Count int
		}
		ranked := make([]tc, 0, len(tagCounts))
		for k, v := range tagCounts {
			ranked = append(ranked, tc{Tag: k, Count: v})
		}
		// Sort by count desc; ties by tag name asc for stability.
		for i := 0; i < len(ranked); i++ {
			for j := i + 1; j < len(ranked); j++ {
				if ranked[j].Count > ranked[i].Count ||
					(ranked[j].Count == ranked[i].Count && ranked[j].Tag < ranked[i].Tag) {
					ranked[i], ranked[j] = ranked[j], ranked[i]
				}
			}
		}
		fmt.Fprintf(f, "### By switch tag\n\n")
		fmt.Fprintf(f, "| Switch tag | Count | Likely interpretation |\n|---|---:|---|\n")
		for _, t := range ranked {
			fmt.Fprintf(f, "| `%s` | %d | %s |\n", escapeCell(t.Tag), t.Count, tagInterpretation(t.Tag))
		}
		fmt.Fprintln(f)

		fmt.Fprintf(f, "### Per-case detail\n\n")
		shown := len(r.UnusedSwitchCases)
		if topN > 0 && topN < shown {
			shown = topN
		}
		fmt.Fprintf(f, "| # | Literal value | Switched on | File:Line |\n|---:|---|---|---|\n")
		for i := 0; i < shown; i++ {
			c := r.UnusedSwitchCases[i]
			tag := c.SwitchTag
			if tag == "" {
				tag = "—"
			}
			fmt.Fprintf(f, "| %d | `%s` | `%s` | `%s:%d` |\n",
				i+1, escapeCell(c.Value), escapeCell(tag), c.File, c.Line)
		}
		if shown < len(r.UnusedSwitchCases) {
			fmt.Fprintf(f, "\n_… %d more (run with `--top 0` to dump all)._\n",
				len(r.UnusedSwitchCases)-shown)
		}
		fmt.Fprintln(f)
	}

	fmt.Fprintf(f, "## Methodology\n\n")
	fmt.Fprintf(f, "- `go/ast` parses every `.go` file under `%s`. Test files (`*_test.go`)\n", dir)
	fmt.Fprintf(f, "  participate in reference counting but their declarations are NOT\n")
	fmt.Fprintf(f, "  collected — dead-branch findings about test helpers aren't actionable.\n")
	fmt.Fprintf(f, "- References are collected as both `*ast.Ident` and `*ast.SelectorExpr.Sel`,\n")
	fmt.Fprintf(f, "  matching by simple name. Receiver-qualified disambiguation isn't\n")
	fmt.Fprintf(f, "  attempted; the over-counting bias is conservative (less likely to\n")
	fmt.Fprintf(f, "  flag a real dead branch).\n")
	fmt.Fprintf(f, "- Self-references (a declaration's name appearing in its own declaring\n")
	fmt.Fprintf(f, "  file) are filtered before classification.\n")
	fmt.Fprintf(f, "- Switch case arms are extracted via `*ast.SwitchStmt`. Type-switch\n")
	fmt.Fprintf(f, "  arms (which switch on types, not string literals) are out of scope.\n\n")

	fmt.Fprintf(f, "## Reproducing this report\n\n")
	fmt.Fprintf(f, "```\ngo run ./cmd/audit-engine-dead --dir %s --out %s --top %d\n```\n",
		dir, path, topN)
	return nil
}

// tagInterpretation returns a short hint about whether the switch
// tag is likely a high-signal dead-branch indicator or a known
// false-positive class (data-driven dispatch, AST-emitted enum).
// Used by the "By switch tag" triage table so readers can prioritise.
func tagInterpretation(tag string) string {
	low := strings.ToLower(tag)
	switch {
	case strings.Contains(low, "name") || strings.Contains(low, "displayname"):
		return "card-name dispatch — values come from JSON data, expected false positive"
	case strings.Contains(low, "modkind") || strings.Contains(low, "scalingkind") || strings.Contains(low, "base"):
		return "AST enum from `scripts/mtg_ast.py` — cross-check parser, expected false positive"
	case strings.Contains(low, "kind") || strings.Contains(low, "event"):
		return "engine event/kind — high-signal if no emitter found"
	case tag == "" || tag == "(no tag)":
		return "no tag (type switch / tagless switch — unusual)"
	}
	return "verify the emitter is genuinely absent"
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func dirname(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}
