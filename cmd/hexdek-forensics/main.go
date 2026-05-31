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
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	replayPath := flag.String("replay", "", "path to Loki replay JSON (required)")
	flag.Parse()

	if *replayPath == "" {
		fmt.Fprintln(os.Stderr, "hexdek-forensics: --replay <path> is required")
		flag.Usage()
		os.Exit(2)
	}

	replay, err := LoadReplay(*replayPath)
	if err != nil {
		log.Fatalf("load replay: %v", err)
	}

	traces := AnalyzeReplay(replay)
	if len(traces) == 0 {
		fmt.Printf("no fabrication-arm violations in replay (game %d, seed %d, %d total violations, %d events)\n",
			replay.GameIdx, replay.Seed, len(replay.Violations), len(replay.Events))
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
