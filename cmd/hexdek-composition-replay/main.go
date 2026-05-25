// hexdek-composition-replay loads a finished game's composition-prior
// monitoring record (PR #420 effects persisted to disk by PR #422)
// and prints a debug view comparing what the prior PREDICTED for each
// seat against what actually happened.
//
// Usage:
//
//	hexdek-composition-replay 1234567890           # RNG seed
//	hexdek-composition-replay -gameid 42           # showmatch_game.game_id (looks up RNG seed)
//	hexdek-composition-replay -data data/ 9999     # custom data dir
//
// The output shows per-seat: archetype, predicted winrate, prior
// confidence, μ offset applied, vanilla baseline shadow comparison,
// and a marker for the actual winner. Designed for hand-tuning the
// CompositionUpdateConfig knobs (Weight, MuOffsetScale) by eyeballing
// where the prior over- or under-shoots.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"

	_ "modernc.org/sqlite"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

func main() {
	var (
		dataDir = flag.String("data", "data", "data directory (Heimdall reads heimdall/composition_prior/{seed}.json from here)")
		dbPath  = flag.String("db", "data/hexdek.db", "SQLite DB path (used only when -gameid is set)")
		gameID  = flag.Int64("gameid", 0, "showmatch_game.game_id (looks up RNG seed in DB) — alternative to passing seed positionally")
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: hexdek-composition-replay [-data DIR] [-db PATH] {seed | -gameid ID}")
		flag.PrintDefaults()
	}
	flag.Parse()

	var rngSeed int64
	switch {
	case *gameID > 0:
		var err error
		rngSeed, err = lookupRNGSeed(*dbPath, *gameID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "lookup game_id %d: %v\n", *gameID, err)
			os.Exit(1)
		}
	case flag.NArg() == 1:
		var err error
		rngSeed, err = strconv.ParseInt(flag.Arg(0), 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid RNG seed %q: %v\n", flag.Arg(0), err)
			os.Exit(1)
		}
	default:
		flag.Usage()
		os.Exit(2)
	}

	rec, err := heimdall.LoadCompositionPriorRecord(*dataDir, rngSeed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no composition-prior record for seed %d in %s: %v\n",
			rngSeed, heimdall.CompositionPriorLogDir(*dataDir), err)
		os.Exit(1)
	}

	printReplay(rec)
}

// lookupRNGSeed resolves a friendly showmatch_game.game_id to its RNG
// seed (which is the on-disk file naming key for replay records).
func lookupRNGSeed(dbPath string, gameID int64) (int64, error) {
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)")
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	g, err := db.LoadGameByID(context.Background(), conn, gameID)
	if err != nil {
		return 0, err
	}
	if g.Seed == 0 {
		return 0, fmt.Errorf("game_id %d has no recorded RNG seed", gameID)
	}
	return g.Seed, nil
}

// printReplay emits the human-readable comparison table.
func printReplay(rec *heimdall.CompositionPriorReplayRecord) {
	fmt.Printf("Game RNG seed: %d\n", rec.GameSeed.RNGSeed)
	fmt.Printf("Winner seat:   %d\n", rec.GameSeed.Winner)
	fmt.Printf("Turns:         %d\n", rec.GameSeed.Turns)
	fmt.Printf("Kill method:   %s\n", rec.GameSeed.KillMethod)
	fmt.Println()

	if len(rec.Effects) == 0 {
		fmt.Println("(no composition prior effects recorded)")
		return
	}

	fmt.Printf("%-4s %-8s %-18s %12s %12s %12s %12s %12s %s\n",
		"seat", "result", "archetype", "predicted%", "confidence", "offset_μ", "Δ_vs_vanilla", "|Δ_vs_van|", "interpretation")
	fmt.Println(repeat("-", 132))

	// Sort by seat for deterministic output.
	totalAbsDelta := 0.0
	for _, e := range rec.Effects {
		result := "—"
		if e.Seat == rec.GameSeed.Winner {
			result = "WINNER"
		}
		interp := interpretEffect(e, e.Seat == rec.GameSeed.Winner)
		fmt.Printf("%-4d %-8s %-18s %11.1f%% %12.3f %12.3f %12.3f %12.3f %s\n",
			e.Seat, result, e.Archetype,
			e.ExpectedWinrate*100, e.Confidence, e.Offset,
			e.MuDeltaVsBaseline, math.Abs(e.MuDeltaVsBaseline),
			interp)
		totalAbsDelta += math.Abs(e.MuDeltaVsBaseline)
	}

	fmt.Println(repeat("-", 132))
	fmt.Printf("Sum |Δ_vs_vanilla|: %.3f μ-points total redistribution\n", totalAbsDelta)
	fmt.Printf("Avg |Δ| per seat:   %.3f\n", totalAbsDelta/float64(len(rec.Effects)))
}

// interpretEffect produces a short English phrase describing what the
// prior did to this seat in this game.
func interpretEffect(e heimdall.CompositionPriorEffect, isWinner bool) string {
	if e.Confidence < 0.05 {
		return "cold-start (no effect)"
	}
	favored := e.ExpectedWinrate > 0.25+0.05
	disfavored := e.ExpectedWinrate < 0.25-0.05
	switch {
	case isWinner && favored:
		return "expected win → dampened μ gain"
	case isWinner && disfavored:
		return "upset win → amplified μ gain"
	case !isWinner && favored:
		return "expected better → amplified μ loss"
	case !isWinner && disfavored:
		return "expected loss → dampened μ loss"
	default:
		return "near-neutral expectation"
	}
}

func repeat(s string, n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = s[0]
	}
	return string(out)
}
