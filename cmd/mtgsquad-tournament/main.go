// mtgsquad-tournament — Phase 11 parallel tournament runner CLI.
//
// Runs N games of Magic: The Gathering Commander across a goroutine
// pool, reports winrate by commander, and optionally writes a Markdown
// report. Semantic peer of scripts/gauntlet_poker.py; the Go engine is
// where the 50k-games-per-second performance thesis ships.
//
// Usage:
//
//	mtgsquad-tournament \
//	    --decks data/decks/cage_match                          # directory of .txt decks
//	    --decks deck1.txt,deck2.txt,deck3.txt,deck4.txt        # or comma-separated paths
//	    --moxfield url1,url2,...                                # fetch decks from Moxfield URLs
//	    --games 1000 \
//	    --seed 42 \
//	    [--seats 4] \
//	    [--workers 0]         # 0 = runtime.NumCPU()
//	    [--hat greedy|poker]  # uniform for every seat (default greedy)
//	    [--matchup]           # show full matchup matrix
//	    [--audit]             # capture event stream for rule auditor
//	    [--report path.md]    # write Markdown summary
//	    [--commander]         # CR §903 mode (default true)
//	    [--ast path]          # override AST dataset path
//
// Default AST dataset path: data/rules/ast_dataset.jsonl.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	rpprof "runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	moxfieldfetch "github.com/hexdek/hexdek/internal/moxfield"
	"github.com/hexdek/hexdek/internal/tournament"
)

// checkFreyaData returns true if the deck has Freya analysis data available.
func checkFreyaData(deckPath string) bool {
	dir := filepath.Dir(deckPath)
	base := filepath.Base(deckPath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	freyaPath := filepath.Join(dir, "freya", base+"_freya.md")
	_, err := os.Stat(freyaPath)
	return err == nil
}

func main() {
	var (
		decksArg    = flag.String("decks", "", "comma-separated deck paths OR a directory of .txt files")
		moxfieldArg = flag.String("moxfield", "", "comma-separated Moxfield URLs to fetch and use as decks")
		games       = flag.Int("games", 100, "number of games to simulate")
		seed        = flag.Int64("seed", 42, "master RNG seed")
		seats       = flag.Int("seats", 4, "seats per game")
		workers     = flag.Int("workers", 0, "goroutine pool size (0 = NumCPU)")
		hatKind     = flag.String("hat", "yggdrasil", "hat policy: yggdrasil (default) | greedy | poker | octo (deprecated)")
		hatBudget     = flag.Int("hat-budget", 50, "yggdrasil budget: 0=heuristic, 1-199=evaluator-guided, 200+=rollout")
		hatTurnBudget = flag.Int("turn-budget", 0, "per-turn eval budget (0=legacy per-action, 100=recommended). Hat allocates evals across decisions within a turn.")
		hatNoise      = flag.Float64("hat-noise", 0.2, "gaussian σ on targeting scores (0=deterministic, 0.2=default human-scale fuzz)")
		matchup     = flag.Bool("matchup", false, "show full matchup matrix in output")
		audit       = flag.Bool("audit", false, "capture full event stream")
		reportPath  = flag.String("report", "data/rules/go_tournament_report.md", "markdown report output path")
		commander   = flag.Bool("commander", true, "enable commander format (CR §903)")
		astPath     = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL path")
		oraclePath  = flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json for P/T supplement")
		maxTurns    = flag.Int("max-turns", 0, "max turns per game (0 = default 80)")
		gameTimeout = flag.Int("game-timeout", 0, "per-game wall-clock timeout in seconds (0 = default 180)")
		roundRobin  = flag.Bool("round-robin", false, "round-robin: every C(N,seats) combination plays --games games")
		poolMode    = flag.Bool("pool", false, "pool mode: each game picks --seats random decks from the full pool (bug hunting)")
		lazyPool    = flag.Bool("lazy-pool", false, "lazy pool: like --pool but loads decks on demand (low memory for large pools)")
		pprofFlag     = flag.Bool("pprof", false, "enable pprof profiling (HTTP :6060 + heap dump after first game)")
		progressEvery = flag.Int("progress-every", 0, "log progress every N games (0 = auto)")
	)
	flag.Parse()

	// Start pprof HTTP server if enabled.
	if *pprofFlag {
		go func() {
			log.Println("pprof server listening on :6060")
			log.Println(http.ListenAndServe(":6060", nil))
		}()
	}

	if *decksArg == "" && *moxfieldArg == "" {
		log.Fatal("--decks or --moxfield is required")
	}

	// Fetch Moxfield decks if provided, save to data/decks/imported/.
	var moxfieldPaths []string
	if *moxfieldArg != "" {
		urls := strings.Split(*moxfieldArg, ",")
		for _, url := range urls {
			url = strings.TrimSpace(url)
			if url == "" {
				continue
			}
			log.Printf("fetching Moxfield deck: %s", url)
			text, err := moxfieldfetch.FetchDeck(url)
			if err != nil {
				log.Fatalf("moxfield fetch %s: %v", url, err)
			}
			// Save to data/decks/imported/{id}.txt
			deckID := moxfieldfetch.ExtractDeckID(url)
			if deckID == "" {
				deckID = fmt.Sprintf("mox_%d", time.Now().UnixNano())
			}
			importDir := "data/decks/imported"
			if err := os.MkdirAll(importDir, 0755); err != nil {
				log.Fatalf("mkdir %s: %v", importDir, err)
			}
			outPath := filepath.Join(importDir, deckID+".txt")
			if err := os.WriteFile(outPath, []byte(text), 0644); err != nil {
				log.Fatalf("write %s: %v", outPath, err)
			}
			log.Printf("  saved to %s", outPath)
			moxfieldPaths = append(moxfieldPaths, outPath)
		}
	}

	// Resolve deck paths: combine file decks + moxfield imports.
	var deckPaths []string
	if *decksArg != "" {
		resolved, err := resolveDeckPaths(*decksArg)
		if err != nil {
			log.Fatalf("resolve decks: %v", err)
		}
		deckPaths = append(deckPaths, resolved...)
	}
	deckPaths = append(deckPaths, moxfieldPaths...)
	if len(deckPaths) < *seats {
		log.Fatalf("need at least %d decks for %d seats (got %d)", *seats, *seats, len(deckPaths))
	}

	log.Printf("mtgsquad-tournament starting")
	log.Printf("  decks:      %d files", len(deckPaths))
	if len(deckPaths) <= 50 {
		for _, p := range deckPaths {
			log.Printf("              %s", p)
		}
	} else {
		log.Printf("              (%d paths, suppressing individual listing)", len(deckPaths))
	}
	log.Printf("  games:      %d", *games)
	log.Printf("  seed:       %d", *seed)
	log.Printf("  seats:      %d", *seats)
	log.Printf("  workers:    %d", *workers)
	log.Printf("  hat:        %s (budget=%d, turn-budget=%d, noise=%.2f)", *hatKind, *hatBudget, *hatTurnBudget, *hatNoise)
	log.Printf("  matchup:    %v", *matchup)
	log.Printf("  commander:  %v", *commander)
	log.Printf("  audit:      %v", *audit)

	// Load AST corpus + meta.
	log.Printf("loading AST corpus from %s ...", *astPath)
	t0 := time.Now()
	corpus, err := astload.Load(*astPath)
	if err != nil {
		log.Fatalf("astload: %v", err)
	}
	log.Printf("  %d cards in %s (warnings: %d)",
		corpus.Count(), time.Since(t0), len(corpus.ParseWarnings))
	meta, err := deckparser.LoadMetaFromJSONL(*astPath)
	if err != nil {
		log.Fatalf("deckparser meta: %v", err)
	}
	log.Printf("  %d card-meta entries", meta.Count())
	if *oraclePath != "" {
		if err := meta.SupplementWithOracleJSON(*oraclePath); err != nil {
			log.Printf("  oracle P/T supplement: %v (continuing without)", err)
		} else {
			log.Printf("  oracle P/T supplement: applied from %s", *oraclePath)
		}
	}

	// Write baseline heap profile after AST loading (before deck parse).
	if *pprofFlag {
		runtime.GC()
		if f, err := os.Create("/tmp/mtgsquad_heap_baseline.prof"); err == nil {
			rpprof.WriteHeapProfile(f)
			f.Close()
			log.Println("baseline heap profile written to /tmp/mtgsquad_heap_baseline.prof")
		}
	}

	// Parse decks (skip for lazy-pool — they load on demand).
	var decks []*deckparser.TournamentDeck
	if !*lazyPool {
		for _, p := range deckPaths {
			d, perr := deckparser.ParseDeckFile(p, corpus, meta)
			if perr != nil {
				log.Fatalf("parse %s: %v", p, perr)
			}
			log.Printf("  deck %q — commander %q, lib %d, unresolved %d",
				p, d.CommanderName, len(d.Library), len(d.Unresolved))
			decks = append(decks, d)
		}
	} else {
		log.Printf("  lazy-pool: %d deck paths registered (loading on demand)", len(deckPaths))
	}

	// Check for Freya analysis data — warn if missing (non-blocking).
	if !*lazyPool {
		for _, p := range deckPaths {
			if !checkFreyaData(p) {
				log.Printf("  WARNING: deck %s has no Freya analysis — Hat will play without strategy intelligence", filepath.Base(p))
			}
		}
	}

	// Hat factory — per-seat when poker hats need Freya strategy data.
	var hatFactories []tournament.HatFactory
	switch strings.ToLower(*hatKind) {
	case "greedy":
		hatFactories = []tournament.HatFactory{
			func() gameengine.Hat { return &hat.GreedyHat{} },
		}
	case "poker":
		// Build per-seat factories so each seat's PokerHat carries its
		// own StrategyProfile from Freya analysis (if available).
		hatFactories = make([]tournament.HatFactory, len(deckPaths))
		for i, p := range deckPaths {
			profile := hat.LoadStrategyFromFreya(p) // may be nil
			if profile != nil {
				log.Printf("  deck %s: loaded Freya strategy (archetype=%s, combos=%d, tutor_targets=%d)",
					filepath.Base(p), profile.Archetype, len(profile.ComboPieces), len(profile.TutorTargets))
			}
			// Capture loop variable for closure.
			prof := profile
			hatFactories[i] = func() gameengine.Hat {
				if prof != nil {
					return hat.NewPokerHatWithStrategy(prof)
				}
				return hat.NewPokerHat()
			}
		}
	case "octo":
		hatFactories = []tournament.HatFactory{
			func() gameengine.Hat { return &hat.OctoHat{} },
		}
	case "yggdrasil":
		hatFactories = make([]tournament.HatFactory, len(deckPaths))
		for i, p := range deckPaths {
			profile := hat.LoadStrategyFromFreya(p)
			if profile != nil {
				log.Printf("  deck %s: loaded Freya strategy (archetype=%s, combos=%d, tutor_targets=%d)",
					filepath.Base(p), profile.Archetype, len(profile.ComboPieces), len(profile.TutorTargets))
			}
			prof := profile
			b := *hatBudget
			tb := *hatTurnBudget
			n := *hatNoise
			hatFactories[i] = func() gameengine.Hat {
				yh := hat.NewYggdrasilHatWithNoise(prof, b, n)
				yh.TurnBudget = tb
				return yh
			}
		}
	default:
		log.Fatalf("unknown hat %q (want greedy|poker|octo|yggdrasil)", *hatKind)
	}

	if *roundRobin {
		rrCfg := tournament.RoundRobinConfig{
			Decks:            decks,
			NSeats:           *seats,
			GamesPerPod:      *games,
			Seed:             *seed,
			HatFactories:     hatFactories,
			Workers:          *workers,
			CommanderMode:    *commander,
			AuditEnabled:     *audit,
			AnalyticsEnabled: false,
			ReportPath:       *reportPath,
			MaxTurnsPerGame:  *maxTurns,
			GameTimeout:      time.Duration(*gameTimeout) * time.Second,
		}
		result, err := tournament.RunRoundRobin(rrCfg)
		if err != nil {
			log.Fatalf("round-robin: %v", err)
		}
		result.PrintDashboard(*matchup)
		if len(result.CrashLogs) > 0 {
			fmt.Printf("\n=== CRASH LOGS (%d) ===\n", len(result.CrashLogs))
			for i, cl := range result.CrashLogs {
				fmt.Printf("\n--- Crash %d ---\n%s\n", i+1, cl)
			}
		}
		if *reportPath != "" {
			fmt.Printf("\nReport written to %s\n", *reportPath)
		}
		os.Exit(0)
	}

	cfg := tournament.TournamentConfig{
		Decks:           decks,
		NSeats:          *seats,
		NGames:          *games,
		Seed:            *seed,
		HatFactories:    hatFactories,
		Workers:         *workers,
		CommanderMode:   *commander,
		AuditEnabled:    *audit,
		ReportPath:      *reportPath,
		MaxTurnsPerGame: *maxTurns,
		GameTimeout:     time.Duration(*gameTimeout) * time.Second,
		PoolMode:        *poolMode,
		LazyPool:        *lazyPool,
		DeckPaths:       deckPaths,
		Corpus:          corpus,
		Meta:            meta,
		PprofEnabled:     *pprofFlag,
		ProgressLogEvery: *progressEvery,
	}

	result, err := tournament.Run(cfg)
	if err != nil {
		log.Fatalf("tournament run: %v", err)
	}

	// Print the full dashboard.
	result.PrintDashboard(*matchup)
	if len(result.CrashLogs) > 0 {
		fmt.Printf("\n=== CRASH LOGS (%d) ===\n", len(result.CrashLogs))
		for i, cl := range result.CrashLogs {
			fmt.Printf("\n--- Crash %d ---\n%s\n", i+1, cl)
		}
	}

	if *reportPath != "" {
		fmt.Printf("\nReport written to %s\n", *reportPath)
	}
	os.Exit(0)
}

// resolveDeckPaths handles two forms of --decks:
//   - A directory path: scans recursively for all .txt files, sorted alphabetically.
//   - A comma-separated list of file paths.
func resolveDeckPaths(arg string) ([]string, error) {
	// Check if arg is a directory.
	info, err := os.Stat(arg)
	if err == nil && info.IsDir() {
		return scanDeckDir(arg)
	}
	// Comma-separated list.
	parts := strings.Split(arg, ",")
	paths := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no deck paths found in %q", arg)
	}
	return paths, nil
}

// scanDeckDir finds all .txt files in dir (recursively) and returns
// them sorted alphabetically.
func scanDeckDir(dir string) ([]string, error) {
	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".txt") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .txt deck files found in %s", dir)
	}
	sort.Strings(paths)
	return paths, nil
}
