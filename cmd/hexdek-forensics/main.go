// hexdek-forensics — InstanceID mint-bypass tracer for Loki replay JSONs.
//
// Loki's ZoneConservation invariant flags two distinct InstanceID bugs:
//
//  1. **Fabrication** — an ID is present in some zone but was never
//     minted (or was ceased and re-added). The dominant Loki r60
//     residual cluster: 80+ hits across `h1OGVR200096` (game 411 / 46
//     hits) + `h1OGVR200056` (game 2762 / 34 hits) + sibling IDs.
//     Almost always traces to a struct-literal `Card{...}` or a clone /
//     copy helper that bypassed MintOGInstanceID / MintCopyInstanceID.
//
//  2. **Disappearance** — an ID was minted, isn't ceased, but is absent
//     from every zone. Handled by separate workflows (Phase E sweep).
//
// This tool focuses on #1. It consumes a Loki replay JSON (schema in
// replay.go), regex-extracts the fabricated ID from each
// ZoneConservation message, decodes the ID per the §3 layout, walks
// the event log for the first reference to either the ID directly or
// the *Card's display name, and prints a unified trace per hit so the
// bisect collapses from "spelunk 5k-game event logs" to "read the
// first event after the mint-bypass site."
//
// Usage:
//
//	hexdek-forensics --replay /path/to/replay-game-411.json
//
// Output: one block per fabrication-arm violation, with the decoded
// InstanceID + first matching event. Non-fabrication violations are
// skipped (they're not mint-bypasses).
//
// Producer integration with Loki is TODO — today's flow is hand-rolled
// or fixture replays. Once Loki ships a `--replay-game-out` flag, the
// workflow becomes: `loki --games 412 --seed 42 --replay-game-out
// /tmp/g411.json --replay-game-idx 411 && hexdek-forensics --replay
// /tmp/g411.json`.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	replayPath := flag.String("replay", "", "single Loki replay JSON path (one-shot trace mode)")
	patternsFlag := flag.Bool("patterns", false, "with --replay: group fabrications by mint-bypass code path (rank-ordered cluster table)")
	watchPath := flag.String("watch", "", "tail a JSONL replay stream (one Replay per line); accumulates pattern aggregate across replays; mutually exclusive with --replay")
	watchFromStart := flag.Bool("watch-from-start", false, "with --watch: start from beginning of file instead of tailing from end")
	pollMs := flag.Int("poll-ms", 500, "with --watch: file-poll interval in milliseconds")
	maxLines := flag.Int("max-lines", 0, "with --watch: stop after processing N replays (0 = unbounded)")
	flag.Parse()

	if (*replayPath == "" && *watchPath == "") || (*replayPath != "" && *watchPath != "") {
		fmt.Fprintln(os.Stderr, "hexdek-forensics: exactly one of --replay <path> or --watch <path> is required")
		flag.Usage()
		os.Exit(2)
	}

	if *watchPath != "" {
		runWatch(*watchPath, *watchFromStart, time.Duration(*pollMs)*time.Millisecond, *maxLines)
		return
	}

	runReplay(*replayPath, *patternsFlag)
}

// runReplay is the one-shot mode: load a single replay, print
// either the per-violation trace blocks (default) or the pattern
// aggregate table (--patterns).
func runReplay(path string, patternsMode bool) {
	replay, err := LoadReplay(path)
	if err != nil {
		log.Fatalf("load replay: %v", err)
	}

	traces := AnalyzeReplay(replay)
	if len(traces) == 0 {
		fmt.Printf("no fabrication-arm violations in replay (game %d, seed %d, %d total violations, %d events)\n",
			replay.GameIdx, replay.Seed, len(replay.Violations), len(replay.Events))
		return
	}

	if patternsMode {
		agg := NewPatternAggregate()
		agg.MergeTraces(traces)
		agg.ReplaysSeen = 1
		fmt.Print(RenderPatternSummary(agg))
		return
	}

	fmt.Printf("Forensics trace — game %d / seed %d / %d events / %d total violations / %d fabrication hits\n",
		replay.GameIdx, replay.Seed, len(replay.Events), len(replay.Violations), len(traces))
	fmt.Println()
	for _, t := range traces {
		fmt.Print(RenderTrace(t))
		fmt.Println()
	}
}

// runWatch is the streaming mode: tail a JSONL replay file and
// re-render the pattern aggregate after each new replay arrives.
// Ctrl-C cleanly exits.
func runWatch(path string, fromStart bool, pollInterval time.Duration, maxLines int) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	agg := NewPatternAggregate()
	opts := WatchOptions{PollInterval: pollInterval, MaxLines: maxLines}

	onReplay := func(r *Replay) {
		agg.MergeReplay(r)
		// Re-render after each merge. ANSI clear-screen + cursor-home
		// would be nicer for live updates but keeping it simple /
		// pipe-friendly (cat the output if you want history) — just
		// print the latest summary with a separator.
		fmt.Println("───────────────────────────────────────────────")
		fmt.Printf("watch update — replay #%d ingested (game_idx=%d seed=%d)\n",
			agg.ReplaysSeen, r.GameIdx, r.Seed)
		fmt.Println()
		summary := RenderPatternSummary(agg)
		if summary == "" {
			fmt.Println("(no fabrication clusters yet across observed replays)")
		} else {
			fmt.Print(summary)
		}
	}
	onError := func(err error) {
		fmt.Fprintf(os.Stderr, "watch: %v\n", err)
	}

	var (
		processed int
		err       error
	)
	if fromStart {
		processed, err = WatchJSONLFromStart(ctx, path, opts, onReplay, onError)
	} else {
		processed, err = WatchJSONL(ctx, path, opts, onReplay, onError)
	}
	if err != nil {
		log.Fatalf("watch %q: %v", path, err)
	}
	fmt.Fprintf(os.Stderr, "watch done — processed %d replay(s)\n", processed)
}
