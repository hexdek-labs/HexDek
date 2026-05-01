// hexdek-muninn — Read and report on persistent Muninn memory files.
//
// Reads parser gaps, crash logs, and dead triggers accumulated across
// tournament runs and prints a human-readable summary.
//
// Usage:
//
//	hexdek-muninn [--gaps] [--crashes] [--triggers] [--all] [--top N] [--dir path]
//
// Flags:
//
//	--gaps       Show parser gaps only
//	--crashes    Show crash logs only
//	--triggers   Show dead triggers only
//	--all        Show all sections (default if no section flag specified)
//	--top N      Limit output to top N entries per section (default 20)
//	--dir path   Muninn data directory (default data/muninn)
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/hexdek/hexdek/internal/muninn"
)

func main() {
	var (
		showGaps        = flag.Bool("gaps", false, "show parser gaps only")
		showCrashes     = flag.Bool("crashes", false, "show crash logs only")
		showTriggers    = flag.Bool("triggers", false, "show dead triggers only")
		showConcessions = flag.Bool("concessions", false, "show concession diagnostics only")
		showAll         = flag.Bool("all", false, "show all sections (default)")
		topN            = flag.Int("top", 20, "limit output per section")
		dir             = flag.String("dir", "data/muninn", "muninn data directory")
	)
	flag.Parse()

	// Default to --all if no section flag specified.
	if !*showGaps && !*showCrashes && !*showTriggers && !*showConcessions {
		*showAll = true
	}

	fmt.Println("=== MUNINN MEMORY REPORT ===")
	fmt.Println()

	anyData := false

	if *showAll || *showGaps {
		ok, err := printGaps(*dir, *topN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading parser gaps: %v\n", err)
			os.Exit(1)
		}
		if ok {
			anyData = true
		}
	}

	if *showAll || *showCrashes {
		ok, err := printCrashes(*dir, *topN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading crash logs: %v\n", err)
			os.Exit(1)
		}
		if ok {
			anyData = true
		}
	}

	if *showAll || *showTriggers {
		ok, err := printTriggers(*dir, *topN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading dead triggers: %v\n", err)
			os.Exit(1)
		}
		if ok {
			anyData = true
		}
	}

	if *showAll || *showConcessions {
		ok, err := printConcessions(*dir, *topN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading concessions: %v\n", err)
			os.Exit(1)
		}
		if ok {
			anyData = true
		}
	}

	if !anyData {
		fmt.Println("No Muninn data found. Run a tournament with --audit and --analytics to populate.")
	}
}

func printGaps(dir string, topN int) (bool, error) {
	gaps, err := muninn.ReadParserGaps(dir)
	if err != nil {
		return false, err
	}
	if len(gaps) == 0 {
		fmt.Println("PARSER GAPS: (none)")
		fmt.Println()
		return false, nil
	}

	sorted := muninn.SortedParserGaps(gaps)
	limit := topN
	if limit > len(sorted) {
		limit = len(sorted)
	}

	fmt.Printf("PARSER GAPS (top %d by frequency, %d total):\n", limit, len(sorted))
	for i := 0; i < limit; i++ {
		g := sorted[i]
		snippet := g.Snippet
		if len(snippet) > 80 {
			snippet = snippet[:77] + "..."
		}
		firstDate := shortDate(g.FirstSeen)
		lastDate := shortDate(g.LastSeen)
		fmt.Printf("  %3d. %q  count=%d  first=%s  last=%s\n",
			i+1, snippet, g.Count, firstDate, lastDate)
	}
	if len(sorted) > limit {
		fmt.Printf("  ... and %d more\n", len(sorted)-limit)
	}
	fmt.Println()
	return true, nil
}

func printCrashes(dir string, topN int) (bool, error) {
	crashes, err := muninn.ReadCrashLogs(dir)
	if err != nil {
		return false, err
	}
	if len(crashes) == 0 {
		fmt.Println("RECURRING CRASHES: (none)")
		fmt.Println()
		return false, nil
	}

	sorted := muninn.SortedCrashLogs(crashes)
	limit := topN
	if limit > len(sorted) {
		limit = len(sorted)
	}

	fmt.Printf("RECURRING CRASHES (top %d by recency, %d total):\n", limit, len(sorted))
	for i := 0; i < limit; i++ {
		c := sorted[i]
		// Truncate stack trace to first line for summary.
		firstLine := c.StackTrace
		if idx := strings.Index(firstLine, "\n"); idx > 0 {
			firstLine = firstLine[:idx]
		}
		if len(firstLine) > 100 {
			firstLine = firstLine[:97] + "..."
		}
		decks := strings.Join(c.Decks, ", ")
		if len(decks) > 60 {
			decks = decks[:57] + "..."
		}
		fmt.Printf("  %3d. %s  decks=[%s]  turns=%d  seen=%s\n",
			i+1, firstLine, decks, c.TurnCount, shortDate(c.Timestamp))
	}
	if len(sorted) > limit {
		fmt.Printf("  ... and %d more\n", len(sorted)-limit)
	}
	fmt.Println()
	return true, nil
}

func printTriggers(dir string, topN int) (bool, error) {
	triggers, err := muninn.ReadDeadTriggers(dir)
	if err != nil {
		return false, err
	}
	if len(triggers) == 0 {
		fmt.Println("DEAD TRIGGERS: (none)")
		fmt.Println()
		return false, nil
	}

	sorted := muninn.SortedDeadTriggers(triggers)
	limit := topN
	if limit > len(sorted) {
		limit = len(sorted)
	}

	fmt.Printf("DEAD TRIGGERS (top %d by frequency, %d total):\n", limit, len(sorted))
	for i := 0; i < limit; i++ {
		dt := sorted[i]
		fmt.Printf("  %3d. trigger_name=%q card=%q  count=%d  games=%d  last=%s\n",
			i+1, dt.TriggerName, dt.CardName, dt.Count, dt.GamesSeen, shortDate(dt.LastSeen))
	}
	if len(sorted) > limit {
		fmt.Printf("  ... and %d more\n", len(sorted)-limit)
	}
	fmt.Println()
	return true, nil
}

func printConcessions(dir string, topN int) (bool, error) {
	records, err := muninn.ReadConcessions(dir)
	if err != nil {
		return false, err
	}
	if len(records) == 0 {
		fmt.Println("CONCESSIONS: (none)")
		fmt.Println()
		return false, nil
	}

	sorted := muninn.SortedConcessions(records)
	limit := topN
	if limit > len(sorted) {
		limit = len(sorted)
	}

	fmt.Printf("CONCESSIONS BY COMMANDER (top %d, %d total records):\n", limit, len(records))
	for i := 0; i < limit; i++ {
		cs := sorted[i]
		fmt.Printf("  %3d. %-40s  count=%d  avg_turn=%.1f  avg_life=%.1f\n",
			i+1, cs.Commander, cs.Count, cs.AvgTurn, cs.AvgLife)
	}
	if len(sorted) > limit {
		fmt.Printf("  ... and %d more commanders\n", len(sorted)-limit)
	}
	fmt.Println()
	return true, nil
}

// shortDate extracts YYYY-MM-DD from an RFC3339 timestamp.
func shortDate(rfc3339 string) string {
	if len(rfc3339) >= 10 {
		return rfc3339[:10]
	}
	return rfc3339
}
