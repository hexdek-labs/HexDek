// hexdek-correctness — the Hex Judge correctness score (Tier 2).
//
// The first honest "how rules-correct is the engine" number: drives
// every available Judge dimension across a representative sample and
// reports the percent passing per dimension, aggregated into one
// top-line correctness %.
//
//	LEGALITY        % player actions legal      (chaos-game sweep,
//	                ride-along validator)
//	CONSERVATION    % games conserved by identity (chaos-game sweep,
//	                ZoneConservation/CardIdentity invariants + Feynman)
//	STATE-INTEGRITY % games internally consistent (chaos-game sweep,
//	                inline invariants + seat-outcome + Feynman; a
//	                panic fails the game)
//	PROGRESSION     % triggers firing correctly  (AST-corpus sweep)
//	OUTCOME         % effects resolving correctly (AST-corpus sweep)
//
// This is a REPORTING tool: it registers a judge.RegisterSink and
// counts the canonical ValidationViolation stream — it implements no
// checks of its own. Output: machine-readable JSON (CI-trackable) +
// a human markdown summary.
//
// Usage:
//
//	go run ./cmd/hexdek-correctness/ \
//	    --ast data/rules/ast_dataset.jsonl \
//	    --oracle data/rules/oracle-cards.json \
//	    --games 300 --seed 42 \
//	    --json correctness-score.json --md correctness-score.md
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func main() {
	var (
		astPath    = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL path")
		oraclePath = flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json path")
		games      = flag.Int("games", 300, "chaos games for the game-level dimensions (0 = skip game sweep)")
		seed       = flag.Int64("seed", 42, "master RNG seed for the game sweep")
		seats      = flag.Int("seats", 4, "seats per game")
		maxTurns   = flag.Int("max-turns", 60, "max turns per game")
		skipCorpus = flag.Bool("skip-corpus", false, "skip the AST-corpus sweep (outcome + progression)")
		tunedGames = flag.Int("tuned-games", 0, "ALSO run the game-level dimensions over real tuned decks with YggdrasilHat pilots (0 = skip)")
		deckDir    = flag.String("deck-dir", "data/decks", "Moxfield decklist pool for --tuned-games")
		jsonPath   = flag.String("json", "correctness-score.json", "machine-readable JSON output path")
		mdPath     = flag.String("md", "", "markdown summary output path (empty = stdout only)")
		livenessBudget = flag.Duration("liveness-budget", 60*time.Second, "per-game wall-clock watchdog for the LIVENESS dimension (0 = watchdog off; post-game liveness checks still run)")
	)
	flag.Parse()

	start := time.Now()
	corpus, err := astload.Load(*astPath)
	if err != nil {
		log.Fatalf("astload %s: %v", *astPath, err)
	}
	fmt.Fprintf(os.Stderr, "loaded AST corpus: %d cards (%.1fs)\n", len(corpus.Names()), time.Since(start).Seconds())

	var dims []DimensionScore
	var notes []string

	if !*skipCorpus {
		t := time.Now()
		outcomeScore := runOutcomePass(corpus)
		fmt.Fprintf(os.Stderr, "outcome sweep: %d effects, %d passed (%.1fs)\n",
			outcomeScore.Checked, outcomeScore.Passed, time.Since(t).Seconds())
		t = time.Now()
		progressionScore := runProgressionPass(corpus)
		fmt.Fprintf(os.Stderr, "progression sweep: %d triggers, %d passed (%.1fs)\n",
			progressionScore.Checked, progressionScore.Passed, time.Since(t).Seconds())
		dims = append(dims, outcomeScore, progressionScore)
		notes = append(notes,
			"OUTCOME and PROGRESSION cover the Judge's current in-scope vocabulary (phase-1..3 effect kinds and trigger shapes), not the full corpus — coverage widens as the Judge widens.",
		)
	} else {
		notes = append(notes, "corpus sweep skipped (--skip-corpus): outcome and progression not scored")
	}

	var gp *GamePassStats
	if *games > 0 {
		meta, err := deckparser.LoadMetaFromJSONL(*astPath)
		if err != nil {
			log.Fatalf("meta %s: %v", *astPath, err)
		}
		// P/T + MDFC back-face enrichment from the oracle corpus, same
		// as loki — without it the MetaDB carries no BackFace* data and
		// MDFC face-cleanup at battlefield entry is blind.
		if err := meta.SupplementWithOracleJSON(*oraclePath); err != nil {
			log.Fatalf("meta oracle supplement %s: %v", *oraclePath, err)
		}
		chaosCorpus, err := gameengine.LoadChaosCorpusFromOracleJSON(*oraclePath)
		if err != nil {
			log.Fatalf("oracle: %v", err)
		}
		t := time.Now()
		var gameDims []DimensionScore
		gameDims, gp = runGamePass(chaosCorpus, corpus, meta, gameConfig{
			Games: *games, Seats: *seats, MaxTurns: *maxTurns, Seed: *seed,
			LivenessBudget: *livenessBudget,
		})
		fmt.Fprintf(os.Stderr, "game sweep: %d games (%.1fs)\n", *games, time.Since(t).Seconds())
		dims = append(dims, gameDims...)
		notes = append(notes,
			"Game-level dimensions sample loki-style chaos games (random commander decks, GreedyHat pilots) — the same population as the loki violation baselines.",
			"CONSERVATION and STATE-INTEGRITY are per-game (a game passes when its whole violation stream for the dimension is clean); LEGALITY is per player action (cast/activate/attack/block).",
			"A panicking game fails STATE-INTEGRITY.",
		)
	} else {
		notes = append(notes, "game sweep skipped (--games 0): legality, conservation, and state-integrity not scored")
	}

	// Tuned-deck sweep (r63): the same game-level dimensions over the
	// REAL deck pool with mixed YggdrasilHat/GreedyHat tables — closes
	// the "chaos games, not tuned-deck play" caveat. Tuned rows are
	// "<dimension>_tuned" and participate in the topline mean.
	if *tunedGames > 0 {
		meta, err := deckparser.LoadMetaFromJSONL(*astPath)
		if err != nil {
			log.Fatalf("meta %s: %v", *astPath, err)
		}
		if err := meta.SupplementWithOracleJSON(*oraclePath); err != nil {
			log.Fatalf("meta oracle supplement %s: %v", *oraclePath, err)
		}
		pool := loadTunedDeckPool(*deckDir, corpus, meta)
		if len(pool) < *seats {
			log.Fatalf("tuned deck pool too small: %d decks in %s (need >= %d)", len(pool), *deckDir, *seats)
		}
		t := time.Now()
		tunedDims, tgp := runTunedGamePass(pool, corpus, gameConfig{
			Games: *tunedGames, Seats: *seats, MaxTurns: *maxTurns, Seed: *seed,
			LivenessBudget: *livenessBudget,
		})
		fmt.Fprintf(os.Stderr, "tuned sweep: %d games over %d decks (%.1fs)\n", *tunedGames, len(pool), time.Since(t).Seconds())
		dims = append(dims, tunedDims...)
		notes = append(notes,
			fmt.Sprintf("Tuned-deck sweep: %d games over the %d-deck Moxfield pool (%s), mixed tables (YggdrasilHat budget %d on even seats, GreedyHat on odd) with real London mulligans — real-play coverage for the game-level dimensions.", *tunedGames, len(pool), *deckDir, yggBudget),
			fmt.Sprintf("Tuned sweep crashes: %d (in %d games).", tgp.Crashes, tgp.CrashedGames),
		)
	}

	score := assembleScore(
		time.Now().UTC().Format(time.RFC3339),
		map[string]string{
			"ast":       *astPath,
			"oracle":    *oraclePath,
			"games":     fmt.Sprint(*games),
			"seed":      fmt.Sprint(*seed),
			"seats":     fmt.Sprint(*seats),
			"max_turns": fmt.Sprint(*maxTurns),
		},
		dims, gp, notes,
	)

	js, err := renderJSON(score)
	if err != nil {
		log.Fatalf("render json: %v", err)
	}
	if err := os.WriteFile(*jsonPath, js, 0644); err != nil {
		log.Fatalf("write %s: %v", *jsonPath, err)
	}
	md := renderMarkdown(score)
	if *mdPath != "" {
		if err := os.WriteFile(*mdPath, []byte(md), 0644); err != nil {
			log.Fatalf("write %s: %v", *mdPath, err)
		}
	}
	fmt.Print(md)
	fmt.Fprintf(os.Stderr, "\nwrote %s (total %.1fs)\n", *jsonPath, time.Since(start).Seconds())
}
