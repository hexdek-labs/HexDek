// hexdek-muninn — Read and report on persistent Muninn memory files.
//
// Reads parser gaps, crash logs, and dead triggers accumulated across
// tournament runs and prints a human-readable summary. Also supports
// archiving dead-trigger entries for cards whose handlers have been
// upgraded since the records were captured.
//
// Usage:
//
//	hexdek-muninn [--gaps] [--crashes] [--triggers] [--all] [--top N] [--dir path]
//	hexdek-muninn --reconcile-fixed [--reconcile-cause "PR #35"] [--reconcile-from path]
//	hexdek-muninn --judge-triage [--judge-log path] [--dir path] [--since when]
//	              [--server-log path[,path…]] [--server-log-stamp when]
//
// Flags:
//
//	--gaps              Show parser gaps only
//	--crashes           Show crash logs only
//	--triggers          Show dead triggers only
//	--all               Show all sections (default if no section flag specified)
//	--top N             Limit output to top N entries per section (default 20)
//	--dir path          Muninn data directory (default data/muninn)
//	--reconcile-fixed   Archive dead-trigger entries for cards in the
//	                    EraPassFixedCards manifest (or --reconcile-from file).
//	--reconcile-cause   Cause string written into the archive (default "era unification 2026-05-09")
//	--reconcile-from    File of card names (one per line) to use instead of
//	                    the embedded EraPassFixedCards list.
//	--judge-triage      Triage the Hex Judge's live violation stream:
//	                    classify by dimension, dedupe by stable fingerprint,
//	                    and write judge-triage.md + judge-triage.json to --dir.
//	--judge-log path    Judge violation JSONL to triage
//	                    (default data/judge/grinder-violations.jsonl).
//	                    Rotated siblings (<path>.1, .2, …) fold in.
//	--since when        Only triage violations stamped at/after this
//	                    time (RFC3339 or YYYY-MM-DD). Scrape only-new
//	                    issues across days of a rolling bucket.
//	--server-log path   Comma-separated server.log capture(s) to ALSO
//	                    scrape for bracket-game feynman-check lines
//	                    (`feynman/RULE [seat N]: MESSAGE`), merged into
//	                    the same digest. This is where the live issues
//	                    land when the jsonl bucket is clean.
//	--server-log-stamp  Fallback timestamp for server.log feynman lines
//	                    that carry no per-line stamp (default = run time).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

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
		reconcileFixed  = flag.Bool("reconcile-fixed", false, "archive dead-trigger entries for fixed handlers")
		reconcileCause  = flag.String("reconcile-cause", "era unification 2026-05-09", "cause string written into the archive")
		reconcileFrom   = flag.String("reconcile-from", "", "file with card names (one per line) overriding the embedded list")
		// Silent-inert audit surface — automates the Genesis Chamber
		// detection pattern (Muninn parser-gap #41, 22,662 hits per
		// PR #815). Reads persisted dead-trigger records (handlers
		// that fired with TriggeredCount > 0 but produced no measurable
		// game state change), ranks by hit count + games seen, applies
		// severity tiers, and emits candidates for pipeline consumption.
		silentInertAudit       = flag.Bool("silent-inert-audit", false, "scan recent dead-trigger observation data and output silent-inert handler candidates ranked by severity")
		auditFormat            = flag.String("audit-format", "text", "output format for --silent-inert-audit: text / json / tsv")
		auditThresholdCritical = flag.Int("audit-threshold-critical", 1000, "hit-count floor for the 'critical' severity tier (Genesis Chamber was 22,662)")
		auditThresholdModerate = flag.Int("audit-threshold-moderate", 100, "hit-count floor for the 'moderate' severity tier")
		auditThresholdWatch    = flag.Int("audit-threshold-watch", 10, "hit-count floor for the 'watch' severity tier (below = 'trivial')")
		auditFailOnCritical    = flag.Bool("audit-fail-on-critical", false, "exit non-zero if any critical-tier candidates surface (for CI use)")
		// Judge triage clerk (r63): consume the Judge watchdog's live
		// violation stream + muninn's own archive, emit the organized
		// triage artifact for human review.
		judgeTriage    = flag.Bool("judge-triage", false, "triage the Judge violation stream into judge-triage.md/.json under --dir")
		judgeLog       = flag.String("judge-log", "data/judge/grinder-violations.jsonl", "judge violation JSONL to triage")
		judgeSince     = flag.String("since", "", "only triage violations stamped at/after this time (RFC3339 or YYYY-MM-DD); scrape only-new issues across days")
		judgeServerLog = flag.String("server-log", "", "comma-separated server.log capture(s) to also scrape for `feynman/RULE [seat N]: MESSAGE` bracket-game lines")
		judgeSrvStamp  = flag.String("server-log-stamp", "", "fallback timestamp (RFC3339 or YYYY-MM-DD) for server.log feynman lines with no per-line stamp; default = run time")
	)
	flag.Parse()

	if *judgeTriage {
		if err := runJudgeTriage(*judgeLog, *dir, *judgeSince, *judgeServerLog, *judgeSrvStamp); err != nil {
			fmt.Fprintf(os.Stderr, "judge-triage: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *silentInertAudit {
		opts := SilentInertAuditOpts{
			Dir:               *dir,
			Format:            *auditFormat,
			CriticalThreshold: *auditThresholdCritical,
			ModerateThreshold: *auditThresholdModerate,
			WatchThreshold:    *auditThresholdWatch,
			FailOnCritical:    *auditFailOnCritical,
		}
		exitCode, err := runSilentInertAudit(os.Stdout, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "silent-inert-audit: %v\n", err)
			os.Exit(1)
		}
		os.Exit(exitCode)
	}

	if *reconcileFixed {
		if err := runReconcile(*dir, *reconcileFrom, *reconcileCause); err != nil {
			fmt.Fprintf(os.Stderr, "reconcile: %v\n", err)
			os.Exit(1)
		}
		return
	}

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

func runJudgeTriage(logPath, dir, since, serverLog, srvStamp string) error {
	var serverLogs []string
	for _, p := range strings.Split(serverLog, ",") {
		if p = strings.TrimSpace(p); p != "" {
			serverLogs = append(serverLogs, p)
		}
	}
	t, err := muninn.TriageJudgeLogWithOptions(muninn.TriageOptions{
		LogPath:        logPath,
		MuninnDir:      dir,
		Since:          since,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		IncludeRotated: true,
		ServerLogPaths: serverLogs,
		ServerLogStamp: srvStamp,
	})
	if err != nil {
		return err
	}
	fmt.Println("=== MUNINN JUDGE TRIAGE ===")
	fmt.Printf("source:   %s\n", logPath)
	if len(serverLogs) > 0 {
		fmt.Printf("server.log: %s (%d feynman line(s) parsed)\n", strings.Join(serverLogs, ", "), t.ServerLogParsed)
	}
	if len(t.Sources) > 1 {
		fmt.Printf("sources:  %d file(s) scanned\n", len(t.Sources))
	}
	if t.Since != "" {
		fmt.Printf("since:    %s (%d older record(s) excluded)\n", t.Since, t.SinceFiltered)
	}
	fmt.Printf("records:  %d (archive folded: %d, malformed skipped: %d)\n",
		t.TotalRecords, t.ArchiveFolded, t.Malformed)
	if t.TotalRecords == 0 {
		fmt.Println("\nNothing to triage — no artifacts written.")
		return nil
	}
	fmt.Printf("clusters: %d\n", len(t.Clusters))
	for _, dim := range []string{"legality", "conservation", "state_integrity", "progression", "outcome", "liveness"} {
		if n := t.ByDimension[dim]; n > 0 {
			fmt.Printf("  %-16s %d\n", dim, n)
		}
	}
	mdPath, jsonPath, err := muninn.WriteJudgeTriage(dir, t)
	if err != nil {
		return err
	}
	fmt.Printf("\nwrote %s\nwrote %s\n", mdPath, jsonPath)
	return nil
}

func runReconcile(dir, fromFile, cause string) error {
	cards := muninn.EraPassFixedCards
	if fromFile != "" {
		loaded, err := loadCardList(fromFile)
		if err != nil {
			return fmt.Errorf("load %s: %w", fromFile, err)
		}
		cards = loaded
	}

	fmt.Println("=== MUNINN RECONCILE (dead triggers) ===")
	fmt.Printf("dir:    %s\n", dir)
	fmt.Printf("cards:  %d\n", len(cards))
	fmt.Printf("cause:  %q\n", cause)
	fmt.Println()

	res, err := muninn.ArchiveFixedCards(dir, cards, cause)
	if err != nil {
		return err
	}

	fmt.Printf("dead_triggers.json before: %d\n", res.DeadTriggersBefore)
	fmt.Printf("dead_triggers.json after:  %d\n", res.DeadTriggersAfter)
	fmt.Printf("archived:                  %d\n", res.DeadTriggersArchived)
	if len(res.UnmatchedCards) > 0 {
		fmt.Printf("\nNo live records for %d cards (already-clean handlers — expected):\n", len(res.UnmatchedCards))
		for _, n := range res.UnmatchedCards {
			fmt.Printf("  - %s\n", n)
		}
	}
	if res.DeadTriggersArchived == 0 && res.DeadTriggersBefore == 0 {
		fmt.Println("\nNo dead_triggers.json present — nothing to do.")
	}
	return nil
}

func loadCardList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var names []string
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	if err := scan.Err(); err != nil {
		return nil, err
	}
	return names, nil
}
