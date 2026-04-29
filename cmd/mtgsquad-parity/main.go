// mtgsquad-parity — Go ↔ Python engine parity verifier.
//
// Runs N games in the Go engine, optionally re-runs them in the Python
// reference via scripts/parity_harness.py, and reports divergences to
// stdout + a markdown file.
//
// Usage:
//
//	mtgsquad-parity \
//	    --decks path1,path2,path3,path4 \
//	    --games 10 \
//	    --seed 42 \
//	    [--seats 4] \
//	    [--python-harness scripts/parity_harness.py] \
//	    [--python-bin python3] \
//	    [--report data/rules/GO_PYTHON_PARITY_REPORT.md]
//
// If --python-harness is omitted the tool still runs — it records the Go
// outcomes but skips diffing. That's useful in CI when Python isn't
// available; the report surfaces "python_available: false" so no one
// mistakes a skip for a pass.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hexdek/hexdek/internal/paritycheck"
)

func main() {
	var (
		decksArg      = flag.String("decks", "", "comma-separated deck paths (required)")
		games         = flag.Int("games", 5, "number of games to run")
		seed          = flag.Int64("seed", 42, "base seed")
		seats         = flag.Int("seats", 4, "seats per game")
		astPath       = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL path")
		oraclePath    = flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json path")
		pythonHarness = flag.String("python-harness", "scripts/parity_harness.py", "Python harness script path")
		pythonBin     = flag.String("python-bin", "python3", "Python interpreter binary")
		reportPath    = flag.String("report", "data/rules/GO_PYTHON_PARITY_REPORT.md", "markdown report output path")
		printJSON     = flag.Bool("json", false, "print the raw report JSON to stdout")
	)
	flag.Parse()

	if *decksArg == "" {
		log.Fatal("--decks is required (comma-separated paths)")
	}
	deckPaths := strings.Split(*decksArg, ",")
	for i, p := range deckPaths {
		deckPaths[i] = strings.TrimSpace(p)
	}
	if len(deckPaths) < *seats {
		log.Fatalf("need at least %d decks for %d seats (got %d)", *seats, *seats, len(deckPaths))
	}

	cfg := paritycheck.Config{
		DeckPaths:         deckPaths,
		NSeats:            *seats,
		NGames:            *games,
		BaseSeed:          *seed,
		AstPath:           *astPath,
		OraclePath:        *oraclePath,
		PythonHarnessPath: *pythonHarness,
		PythonBin:         *pythonBin,
		ReportPath:        *reportPath,
	}
	report, err := paritycheck.Run(cfg)
	if err != nil {
		log.Fatalf("parity run: %v", err)
	}

	if *printJSON {
		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("── Parity Report ──")
	fmt.Printf("  games:               %d\n", report.Games)
	fmt.Printf("  seats:               %d\n", report.NSeats)
	fmt.Printf("  base seed:           %d\n", report.BaseSeed)
	fmt.Printf("  python available:    %v\n", report.PythonAvailable)
	if report.PythonAvailable {
		fmt.Printf("  outcome matches:     %d / %d (%.1f%%)\n",
			report.OutcomeMatches, report.Games,
			100*float64(report.OutcomeMatches)/max1(float64(report.Games)))
		fmt.Printf("  event stream match:  %d / %d (%.1f%%)\n",
			report.EventStreamMatches, report.Games,
			100*float64(report.EventStreamMatches)/max1(float64(report.Games)))
	}
	if len(report.CategoryCounts) > 0 {
		fmt.Println()
		fmt.Println("  Divergence categories:")
		for k, v := range report.CategoryCounts {
			fmt.Printf("    %s: %d\n", k, v)
		}
	}
	if *reportPath != "" {
		fmt.Printf("\n  Report: %s\n", *reportPath)
	}
	os.Exit(0)
}

func max1(v float64) float64 {
	if v < 1 {
		return 1
	}
	return v
}
