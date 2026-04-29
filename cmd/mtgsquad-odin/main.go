// mtgsquad-fuzz — Property-based fuzzer for the Go rules engine.
//
// Runs N random games with full invariant checking after every game
// action (cast, resolve, combat, SBA, trigger). On violation: logs the
// full game state + event history + invariant name + game ID, then
// continues to the next game. Designed to run overnight on DARKSTAR.
//
// Usage:
//
//	go run ./cmd/mtgsquad-fuzz/ \
//	    --games 10000 \
//	    --seed 42 \
//	    --decks data/decks/cage_match/ \
//	    --report data/rules/FUZZ_REPORT.md
//
// The fuzzer mirrors the tournament runner's game setup but wraps
// every GameState mutation point with invariant checks. Violations
// are collected per-game and written to a markdown report at the end.
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

// violation records a single invariant violation in a game.
type violation struct {
	GameIdx       int
	GameSeed      int64
	InvariantName string
	Message       string
	Turn          int
	Phase         string
	Step          string
	EventCount    int
	StateSummary  string
	RecentEvents  []string
}

// gameResult is the per-game outcome from a fuzz run.
type gameResult struct {
	GameIdx    int
	Violations []violation
	Turns      int
	Crashed    bool
	CrashErr   string
}

func main() {
	var (
		gamesFlag  = flag.Int("games", 1000, "number of games to fuzz")
		seedFlag   = flag.Int64("seed", 42, "master RNG seed")
		decksFlag  = flag.String("decks", "data/decks/cage_match/", "directory or comma-separated deck paths")
		reportFlag = flag.String("report", "data/rules/FUZZ_REPORT.md", "markdown report output path")
		seatsFlag  = flag.Int("seats", 4, "seats per game")
		workersFlg = flag.Int("workers", 0, "worker goroutines (0 = NumCPU)")
		maxTurns   = flag.Int("max-turns", 80, "max turns per game")
		astPath    = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL path")
		oraclePath = flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json path")
	)
	flag.Parse()

	workers := *workersFlg
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	log.Printf("mtgsquad-fuzz starting")
	log.Printf("  games:    %d", *gamesFlag)
	log.Printf("  seed:     %d", *seedFlag)
	log.Printf("  seats:    %d", *seatsFlag)
	log.Printf("  workers:  %d", workers)
	log.Printf("  decks:    %s", *decksFlag)
	log.Printf("  report:   %s", *reportFlag)

	// Load AST corpus + meta.
	log.Printf("loading AST corpus from %s ...", *astPath)
	t0 := time.Now()
	corpus, err := astload.Load(*astPath)
	if err != nil {
		log.Fatalf("astload: %v", err)
	}
	log.Printf("  %d cards in %s", corpus.Count(), time.Since(t0))
	meta, err := deckparser.LoadMetaFromJSONL(*astPath)
	if err != nil {
		log.Fatalf("deckparser meta: %v", err)
	}
	log.Printf("  %d card-meta entries", meta.Count())
	if *oraclePath != "" {
		if err := meta.SupplementWithOracleJSON(*oraclePath); err != nil {
			log.Printf("  oracle supplement: %v (continuing without)", err)
		}
	}

	// Load decks.
	deckPaths := resolveDeckPaths(*decksFlag)
	if len(deckPaths) < *seatsFlag {
		log.Fatalf("need at least %d decks for %d seats (found %d)", *seatsFlag, *seatsFlag, len(deckPaths))
	}
	log.Printf("  loading %d decks ...", len(deckPaths))
	decks := make([]*deckparser.TournamentDeck, len(deckPaths))
	for i, p := range deckPaths {
		d, err := deckparser.ParseDeckFile(p, corpus, meta)
		if err != nil {
			log.Fatalf("deck %s: %v", p, err)
		}
		decks[i] = d
		log.Printf("    [%d] %s (%d cards, %d unresolved)",
			i, d.CommanderName, len(d.Library)+len(d.CommanderCards), len(d.Unresolved))
	}

	// Run fuzz games.
	nSeats := *seatsFlag
	nGames := *gamesFlag
	start := time.Now()

	seeds := make(chan int, workers*4)
	results := make(chan gameResult, workers*4)
	var completed int64

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for gameIdx := range seeds {
				result := fuzzOneGame(gameIdx, decks, nSeats, *seedFlag, *maxTurns)
				results <- result
				done := atomic.AddInt64(&completed, 1)
				if done%500 == 0 {
					gps := float64(done) / time.Since(start).Seconds()
					fmt.Fprintf(os.Stderr, "  fuzz: %d/%d games (%.0f g/s)\n", done, nGames, gps)
				}
			}
		}()
	}

	go func() {
		for i := 0; i < nGames; i++ {
			seeds <- i
		}
		close(seeds)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate results.
	var allViolations []violation
	totalCrashes := 0
	totalGamesWithViolations := 0
	violationsByName := map[string]int{}
	totalTurns := 0

	for r := range results {
		totalTurns += r.Turns
		if r.Crashed {
			totalCrashes++
		}
		if len(r.Violations) > 0 {
			totalGamesWithViolations++
			allViolations = append(allViolations, r.Violations...)
			for _, v := range r.Violations {
				violationsByName[v.InvariantName]++
			}
		}
	}

	elapsed := time.Since(start)
	gps := float64(nGames) / elapsed.Seconds()

	log.Printf("")
	log.Printf("=== FUZZ COMPLETE ===")
	log.Printf("  games:      %d", nGames)
	log.Printf("  duration:   %s", elapsed.Round(time.Millisecond))
	log.Printf("  throughput: %.0f games/sec", gps)
	log.Printf("  crashes:    %d", totalCrashes)
	log.Printf("  games with violations: %d", totalGamesWithViolations)
	log.Printf("  total violations:      %d", len(allViolations))
	for name, count := range violationsByName {
		log.Printf("    %-30s %d", name, count)
	}

	// Write report.
	if *reportFlag != "" {
		writeReport(*reportFlag, nGames, *seedFlag, nSeats, elapsed, gps,
			totalCrashes, totalGamesWithViolations, allViolations, violationsByName)
		log.Printf("report written to %s", *reportFlag)
	}

	if len(allViolations) > 0 {
		os.Exit(1)
	}
}

func fuzzOneGame(gameIdx int, decks []*deckparser.TournamentDeck,
	nSeats int, masterSeed int64, maxTurns int) (result gameResult) {

	result.GameIdx = gameIdx

	defer func() {
		if r := recover(); r != nil {
			result.Crashed = true
			result.CrashErr = fmt.Sprintf("panic: %v", r)
		}
	}()

	gameSeed := masterSeed + int64(gameIdx)*1000 + 1
	rng := rand.New(rand.NewSource(gameSeed))
	gs := gameengine.NewGameState(nSeats, rng, nil)

	// Rotate decks.
	rot := gameIdx % nSeats
	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		orig := (i + rot) % nSeats
		tpl := decks[orig%len(decks)]
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		rng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		commanderDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: cmdrs,
			Library:        lib,
		}
	}

	gameengine.SetupCommanderGame(gs, commanderDecks)

	// Attach hats.
	for i := 0; i < nSeats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}

	// Opening hands.
	for i := 0; i < nSeats; i++ {
		for j := 0; j < 7; j++ {
			if len(gs.Seats[i].Library) == 0 {
				break
			}
			c := gs.Seats[i].Library[0]
			gs.Seats[i].Library = gs.Seats[i].Library[1:]
			gs.Seats[i].Hand = append(gs.Seats[i].Hand, c)
		}
	}

	gs.Active = rng.Intn(nSeats)
	gs.Turn = 1

	// Run invariants on initial state.
	checkInvariants(gs, gameIdx, gameSeed, &result)

	// Turn loop with invariant checking.
	for turn := 1; turn <= maxTurns; turn++ {
		gs.Turn = turn
		tournament.TakeTurn(gs)
		checkInvariants(gs, gameIdx, gameSeed, &result)

		gameengine.StateBasedActions(gs)
		checkInvariants(gs, gameIdx, gameSeed, &result)

		if gs.CheckEnd() {
			break
		}
		gs.Active = nextLivingSeat(gs)
	}

	result.Turns = gs.Turn
	return result
}

func checkInvariants(gs *gameengine.GameState, gameIdx int, gameSeed int64, result *gameResult) {
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		viol := violation{
			GameIdx:       gameIdx,
			GameSeed:      gameSeed,
			InvariantName: v.Name,
			Message:       v.Message,
			Turn:          gs.Turn,
			Phase:         gs.Phase,
			Step:          gs.Step,
			EventCount:    len(gs.EventLog),
			StateSummary:  gameengine.GameStateSummary(gs),
			RecentEvents:  gameengine.RecentEvents(gs, 20),
		}
		result.Violations = append(result.Violations, viol)
	}
}

func nextLivingSeat(gs *gameengine.GameState) int {
	n := len(gs.Seats)
	for offset := 1; offset <= n; offset++ {
		idx := (gs.Active + offset) % n
		if gs.Seats[idx] != nil && !gs.Seats[idx].Lost {
			return idx
		}
	}
	return gs.Active
}

func resolveDeckPaths(input string) []string {
	// Check if input is a directory.
	info, err := os.Stat(input)
	if err == nil && info.IsDir() {
		entries, err := os.ReadDir(input)
		if err != nil {
			log.Fatalf("read deck dir %s: %v", input, err)
		}
		var paths []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".txt") {
				paths = append(paths, filepath.Join(input, e.Name()))
			}
		}
		return paths
	}
	// Comma-separated paths.
	parts := strings.Split(input, ",")
	var paths []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func writeReport(path string, nGames int, seed int64, nSeats int,
	elapsed time.Duration, gps float64,
	crashes, gamesWithViolations int,
	allViolations []violation,
	violationsByName map[string]int) {

	f, err := os.Create(path)
	if err != nil {
		log.Printf("write report: %v", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# Fuzz Report\n\n")
	fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, "## Configuration\n\n")
	fmt.Fprintf(f, "| Parameter | Value |\n")
	fmt.Fprintf(f, "|-----------|-------|\n")
	fmt.Fprintf(f, "| Games | %d |\n", nGames)
	fmt.Fprintf(f, "| Seed | %d |\n", seed)
	fmt.Fprintf(f, "| Seats | %d |\n", nSeats)
	fmt.Fprintf(f, "| Duration | %s |\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(f, "| Throughput | %.0f games/sec |\n", gps)
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "## Results\n\n")
	fmt.Fprintf(f, "| Metric | Count |\n")
	fmt.Fprintf(f, "|--------|-------|\n")
	fmt.Fprintf(f, "| Crashes | %d |\n", crashes)
	fmt.Fprintf(f, "| Games with violations | %d |\n", gamesWithViolations)
	fmt.Fprintf(f, "| Total violations | %d |\n", len(allViolations))

	if len(violationsByName) > 0 {
		fmt.Fprintf(f, "\n### Violations by Invariant\n\n")
		fmt.Fprintf(f, "| Invariant | Count |\n")
		fmt.Fprintf(f, "|-----------|-------|\n")
		for name, count := range violationsByName {
			fmt.Fprintf(f, "| %s | %d |\n", name, count)
		}
	}

	if len(allViolations) > 0 {
		// Show at most 50 violations in detail.
		limit := len(allViolations)
		if limit > 50 {
			limit = 50
		}
		fmt.Fprintf(f, "\n### Violation Details (first %d)\n\n", limit)
		for i := 0; i < limit; i++ {
			v := &allViolations[i]
			fmt.Fprintf(f, "#### Violation %d\n\n", i+1)
			fmt.Fprintf(f, "- **Game**: %d (seed %d)\n", v.GameIdx, v.GameSeed)
			fmt.Fprintf(f, "- **Invariant**: %s\n", v.InvariantName)
			fmt.Fprintf(f, "- **Turn**: %d, Phase=%s Step=%s\n", v.Turn, v.Phase, v.Step)
			fmt.Fprintf(f, "- **Events**: %d\n", v.EventCount)
			fmt.Fprintf(f, "- **Message**: %s\n\n", v.Message)
			fmt.Fprintf(f, "<details>\n<summary>Game State</summary>\n\n```\n%s\n```\n\n</details>\n\n", v.StateSummary)
			if len(v.RecentEvents) > 0 {
				fmt.Fprintf(f, "<details>\n<summary>Recent Events</summary>\n\n```\n")
				for _, e := range v.RecentEvents {
					fmt.Fprintf(f, "%s\n", e)
				}
				fmt.Fprintf(f, "```\n\n</details>\n\n")
			}
		}
		if len(allViolations) > limit {
			fmt.Fprintf(f, "\n*... and %d more violations not shown.*\n", len(allViolations)-limit)
		}
	}

	if len(allViolations) == 0 && crashes == 0 {
		fmt.Fprintf(f, "\n## Verdict: CLEAN\n\n")
		fmt.Fprintf(f, "All %d games passed all invariant checks with zero crashes.\n", nGames)
	} else {
		fmt.Fprintf(f, "\n## Verdict: VIOLATIONS FOUND\n\n")
		fmt.Fprintf(f, "Review the violations above and investigate.\n")
	}
}
