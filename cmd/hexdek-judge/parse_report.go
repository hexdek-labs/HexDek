package main

import (
	"fmt"
	"os"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
)

// runParseReport loads the AST corpus + Scryfall oracle (P/T supplement)
// and parses the deck at deckPath, then prints the deckparser's
// structured per-line resolution coverage report to stdout. Returns
// (true, nil) when every card-shaped line resolved (clean or fallback);
// (false, nil) when any line failed to resolve. Non-nil error only for
// I/O / corpus-load failures, never for parse-coverage findings.
//
// Surfaced via hexdek-judge --report-parse. Drives the deckbuilder UX
// where a freshly-imported deck file is parsed and a coverage summary
// is shown so the user can spot typos, renamed cards, and meta gaps
// before running any downstream tooling (tournament, valkyrie, freya).
func runParseReport(deckPath, astPath, oraclePath string) (bool, error) {
	corpus, err := astload.Load(astPath)
	if err != nil {
		// Best-effort: a missing AST corpus is non-fatal for the report
		// path — buildCard falls back to meta-only resolution. Warn and
		// proceed.
		fmt.Fprintf(os.Stderr, "warning: load AST corpus from %s: %v (continuing with meta-only resolution)\n", astPath, err)
	}
	meta, err := deckparser.LoadMetaFromJSONL(astPath)
	if err != nil {
		return false, fmt.Errorf("load meta from %s: %w", astPath, err)
	}
	if err := meta.SupplementWithOracleJSON(oraclePath); err != nil {
		// Oracle is for P/T supplementation, not name resolution — warn
		// but don't fail. Parse coverage doesn't depend on P/T.
		fmt.Fprintf(os.Stderr, "warning: supplement meta with Scryfall oracle from %s: %v\n", oraclePath, err)
	}
	td, err := deckparser.ParseDeckFile(deckPath, corpus, meta)
	if err != nil {
		// Parse error short-circuits before the report can be built.
		// Surface the error directly so the user knows the file is
		// structurally invalid (no commander when one was expected,
		// scanner failure, etc.).
		return false, fmt.Errorf("parse %s: %w", deckPath, err)
	}
	if err := td.PrintReport(os.Stdout); err != nil {
		return false, fmt.Errorf("print report: %w", err)
	}
	return td.ParseReport.UnresolvedLines == 0, nil
}
