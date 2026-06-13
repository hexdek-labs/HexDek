// hexdek-loki — Chaos gauntlet stress test for the Go rules engine.
//
// Generates RANDOM Commander decks from the full 32K+ oracle corpus and
// runs 4-seat games with full invariant checking. Not curated decks —
// pure RNG nightmare decks. The goal is to find crashes and invariant
// violations caused by card combinations nobody designed test cases for.
//
// Usage:
//
//	go run ./cmd/hexdek-loki/ --games 1000 --seed 42 --permutations 5
//	go run ./cmd/hexdek-loki/ --games 1000 --seed 42 --nightmare-boards 10000
//
// For each game:
//  1. Pick 4 random legendary creatures from the oracle corpus as commanders
//  2. For each seat: generate a 99-card deck matching commander color identity
//  3. Run the game with GreedyHat, turn cap 60
//  4. Run all 9 invariants after every action
//  5. Log ANY crash, ANY invariant violation, ANY panic/recover
//
// --permutations N means: for each random deck set, run N games with
// different shuffles. This catches "this card COMBINATION breaks things"
// not just "this shuffle breaks things."
//
// Nightmare boards: generate random permanents on each seat's battlefield,
// then run SBAs + layer recalculation + trigger checks. Directly tests the
// layer system + SBA system against card combinations nobody designed.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/judge"
	"github.com/hexdek/hexdek/internal/tournament"
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// chaosViolation records a single invariant violation in a chaos game.
//
// Consolidation step 4 note: this is Loki's flattened REPORT-ROW schema
// (run metadata + violation), not a violation vocabulary. Everything it
// aggregates (RunAllInvariants, gs.Legality, gs.SeatOutcome) was already
// routed through validation.LogViolation at origin — do not re-log at
// these drain sites, and do not grow this struct into a vocabulary.
type chaosViolation struct {
	GameIdx       int
	GameSeed      int64
	Permutation   int
	InvariantName string
	Message       string
	Turn          int
	Phase         string
	Step          string
	EventCount    int
	StateSummary  string
	RecentEvents  []string
	Commanders    []string
}

// chaosCrash records a panic/crash in a chaos game.
type chaosCrash struct {
	GameIdx     int
	GameSeed    int64
	Permutation int
	PanicValue  string
	StackTrace  string
	Commanders  []string
	// CardInFlight is the card being processed when the crash happened
	// (if determinable from the stack trace / event log).
	CardInFlight string
}

// chaosGameResult is the per-game outcome from a chaos run.
type chaosGameResult struct {
	GameIdx    int
	Violations []chaosViolation
	Crashes    []chaosCrash
	Turns      int
	Commanders []string
	// AllCards is the union of all card names in the 4 decks for this game.
	// Used for statistical correlation analysis.
	AllCards []string
}

// cardCorrelation pairs a card name with its violation/clean game counts
// for statistical correlation analysis.
type cardCorrelation struct {
	Name           string
	ViolationGames int
	CleanGames     int
	Score          float64 // ratio of violation appearances to total
}

// nightmareResult records the outcome of a single nightmare board test.
type nightmareResult struct {
	BoardIdx   int
	Violations []chaosViolation
	Crashed    bool
	CrashErr   string
	StackTrace string
	CardNames  []string // cards on the board when it crashed/violated
	// MintedCount is len(gs.MintedInstanceIDs) after board build — the
	// r63 vacuity pin. Zero means checkZoneConservation silently falls
	// back to the legacy count check and the strict census never runs.
	MintedCount int
}

// ---------------------------------------------------------------------------
// Oracle corpus loader
// ---------------------------------------------------------------------------

// loadOracleCorpus delegates to the promoted shared loader (r63 Judge
// CI gate) — the corpus-quality filters live in
// gameengine.LoadChaosCorpusFromOracleJSON now, shared with
// hexdek-judge --run.
func loadOracleCorpus(path string) (*gameengine.ChaosCorpus, error) {
	return gameengine.LoadChaosCorpusFromOracleJSON(path)
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

// invariantFilter, when non-empty, restricts violation recording to the
// single invariant whose canonical (camelCase) name matches. Set by main
// after parsing --invariant; consumed inside checkChaosInvariants and
// checkNightmareInvariants. Empty = match-all (legacy behavior).
var invariantFilter string

// legalityEnabled mirrors the -legality flag: attach the ride-along
// rules-legality validator (gameengine/legality.go) to every chaos game
// and surface its violations alongside the invariant census. Default
// off — zero behavior change in the engine when unset.
var legalityEnabled bool

// seatOutcomeEnabled mirrors the -seat-outcome flag.
var seatOutcomeEnabled bool

// splitCardList parses a card-name list flag. `;` is the preferred
// separator since several card names contain commas (e.g. "Anafenza,
// the Foremost"); comma still works when no `;` is present so older
// invocations keep parsing. Previously only --seed-cards-all-seats had
// the `;` form — --seed-cards comma-split "Anafenza, the Foremost"
// into two unmatchable names.
func splitCardList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	sep := ","
	if strings.Contains(raw, ";") {
		sep = ";"
	}
	var out []string
	for _, s := range strings.Split(raw, sep) {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func main() {
	var (
		gamesFlag             = flag.Int("games", 1000, "number of chaos games to run")
		seedFlag              = flag.Int64("seed", 42, "master RNG seed")
		permsFlag             = flag.Int("permutations", 1, "shuffles per deck set")
		seatsFlag             = flag.Int("seats", 4, "seats per game")
		maxTurnsFlag          = flag.Int("max-turns", 60, "max turns per game")
		workersFlag           = flag.Int("workers", 0, "worker goroutines (0 = NumCPU)")
		reportFlag            = flag.String("report", "data/rules/CHAOS_REPORT.md", "markdown report output path")
		astPath               = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL path")
		oraclePath            = flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json path")
		nightmareFlag         = flag.Int("nightmare-boards", 10000, "number of nightmare board tests")
		seedCardsFlag         = flag.String("seed-cards", "", "comma-separated card names to force into seat 0's deck every chaos game (handler-focused fuzz)")
		seedCardsAllSeatsFlag = flag.String("seed-cards-all-seats", "", "comma-separated card names to distribute round-robin across ALL seats' decks every chaos game (cross-seat interaction fuzz; complements --seed-cards which is seat-0-only)")
		seedCmdrFlag          = flag.String("seed-cmdr", "", "force seat 0's commander to this name (must be a legendary creature in oracle corpus)")
		invariantFlag         = flag.String("invariant", "", "filter violations to a single invariant kind (case-insensitive, accepts CamelCase or kebab-case; empty = all). Example: --invariant zone-conservation")
		listInvFlag           = flag.Bool("list-invariants", false, "print the full set of known invariant names and exit")
		strictCensus          = flag.Bool("instanceid-strict-census", false, "enable InstanceID Phase 4+ strict ZoneConservation disappearance check (per docs/instanceid-system-v2-r60.md §13). Default off — flips gs.Flags[\"instanceid_strict_census\"]=1 on every game.")
		violationsDumpPath    = flag.String("violations-dump", "", "if set, write every chaos violation message (one per line, tab-separated: game-idx<TAB>turn<TAB>invariant<TAB>message) to this path for offline histogram analysis. Bypasses the report's 30-detail cap.")
		judgeJSONLPath        = flag.String("judge-jsonl", "", "if set, write every chaos + nightmare violation as a grinder-violations.jsonl row (the Hex Judge triage-stream contract: dimension/surface/rule/seed/turn/detail) so `hexdek-muninn --judge-triage --judge-log <path>` can cluster the long-tail by dimension + fingerprint.")
		seatOutcomeFlag       = flag.Bool("seat-outcome", false, "attach the r63 per-seat win/loss self-checker to every chaos game (outcome recomputation + cross-seat consistency + §800.4 leave-game cleanup verification). Default off — zero engine behavior change when unset.")
		legalityFlag          = flag.Bool("legality", false, "attach the ride-along rules-legality validator to every chaos game (live CR 307.1/608.2c/601.2f auditing of each cast/activation as it happens). Default off — zero engine behavior change when unset.")
		gameTimeoutFlag       = flag.Duration("game-timeout", 0, "per-game wall-clock watchdog (e.g. 20s). 0 = off. A game still running at the deadline is ABANDONED (its goroutine leaks — accepted in an offline fuzz run) and recorded as a Liveness:game_timeout violation. Lets DEEP sweeps (--max-turns 60-100) complete past board-explosion / mandatory-loop games that would otherwise stall the whole run.")
	)
	flag.Parse()
	legalityEnabled = *legalityFlag
	seatOutcomeEnabled = *seatOutcomeFlag
	if *strictCensus {
		gameengine.SetStrictCensusDefault(true)
	}

	knownInvariants := invariantNames(gameengine.AllInvariants())
	if *listInvFlag {
		for _, n := range knownInvariants {
			fmt.Println(n)
		}
		return
	}
	if canonical, err := canonicalizeInvariantName(*invariantFlag, knownInvariants); err != nil {
		log.Fatalf("--invariant: %v", err)
	} else {
		invariantFilter = canonical
	}

	seedCards := splitCardList(*seedCardsFlag)
	seedCardsAllSeats := splitCardList(*seedCardsAllSeatsFlag)

	workers := *workersFlag
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	// SHADOW (driver-conversion plan step 1): register the Judge-sink
	// mirror. One process-wide sink, attributed per game via judgeMu (see
	// the shadow infrastructure block). Pure observation — no behavior
	// change. Registered before any game runs; unregistered at exit.
	unregisterShadow := judge.RegisterSink(shadowSink)
	defer unregisterShadow()

	log.Printf("hexdek-loki starting")
	log.Printf("  games:           %d", *gamesFlag)
	log.Printf("  permutations:    %d", *permsFlag)
	log.Printf("  seed:            %d", *seedFlag)
	log.Printf("  seats:           %d", *seatsFlag)
	log.Printf("  max-turns:       %d", *maxTurnsFlag)
	log.Printf("  workers:         %d", workers)
	log.Printf("  nightmare-boards: %d", *nightmareFlag)
	if invariantFilter != "" {
		log.Printf("  invariant filter: %s (other invariants will still RUN but their violations are dropped from the report)", invariantFilter)
	}

	// Load AST corpus + meta (needed to build gameengine.Card objects).
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

	// Load oracle corpus for random deck generation (has color_identity).
	log.Printf("loading oracle corpus from %s ...", *oraclePath)
	chaosCorpus, err := loadOracleCorpus(*oraclePath)
	if err != nil {
		log.Fatalf("oracle corpus: %v", err)
	}
	log.Printf("  %d total cards", len(chaosCorpus.All))
	log.Printf("  %d legendary creatures (potential commanders)", len(chaosCorpus.LegendaryCreatures))
	log.Printf("  %d non-land cards", len(chaosCorpus.NonLand))
	log.Printf("  %d non-basic lands", len(chaosCorpus.NonBasicLands))

	// =====================================================================
	// Phase 1: Chaos Games
	// =====================================================================
	log.Printf("")
	log.Printf("=== PHASE 1: CHAOS GAMES ===")

	totalGames := *gamesFlag * *permsFlag
	log.Printf("  total game instances: %d (%d deck sets x %d permutations)",
		totalGames, *gamesFlag, *permsFlag)

	start := time.Now()
	type gameJob struct {
		gameIdx     int
		permutation int
	}
	jobs := make(chan gameJob, workers*4)
	gameResults := make(chan chaosGameResult, workers*4)
	var completed int64

	gameTimeout := *gameTimeoutFlag
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				result := runChaosGameWatched(
					job.gameIdx, job.permutation, gameTimeout,
					chaosCorpus, corpus, meta,
					*seatsFlag, *seedFlag, *maxTurnsFlag,
					seedCards, *seedCmdrFlag, seedCardsAllSeats,
				)
				gameResults <- result
				done := atomic.AddInt64(&completed, 1)
				if done%100 == 0 || done == int64(totalGames) {
					elapsed := time.Since(start).Seconds()
					gps := float64(done) / elapsed
					fmt.Fprintf(os.Stderr, "  chaos: %d/%d games (%.0f g/s)\n", done, totalGames, gps)
				}
			}
		}()
	}

	go func() {
		for g := 0; g < *gamesFlag; g++ {
			for p := 0; p < *permsFlag; p++ {
				jobs <- gameJob{gameIdx: g, permutation: p}
			}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(gameResults)
	}()

	// Aggregate game results.
	var allViolations []chaosViolation
	var allCrashes []chaosCrash
	violationsByName := map[string]int{}
	cardInViolationGames := map[string]int{} // card name -> count of violation games
	cardInCleanGames := map[string]int{}     // card name -> count of clean games
	crashCards := map[string]int{}           // card name -> crash count
	totalTurns := 0
	gamesWithViolations := 0
	gamesWithCrashes := 0
	cleanGames := 0

	for r := range gameResults {
		totalTurns += r.Turns

		hasViolation := len(r.Violations) > 0
		hasCrash := len(r.Crashes) > 0

		if hasViolation {
			gamesWithViolations++
			allViolations = append(allViolations, r.Violations...)
			for _, v := range r.Violations {
				violationsByName[v.InvariantName]++
			}
			for _, name := range r.AllCards {
				cardInViolationGames[name]++
			}
		}
		if hasCrash {
			gamesWithCrashes++
			allCrashes = append(allCrashes, r.Crashes...)
			for _, c := range r.Crashes {
				if c.CardInFlight != "" {
					crashCards[c.CardInFlight]++
				}
				for _, cmdr := range c.Commanders {
					crashCards[cmdr]++
				}
			}
		}
		if !hasViolation && !hasCrash {
			cleanGames++
			for _, name := range r.AllCards {
				cardInCleanGames[name]++
			}
		}
	}

	chaosElapsed := time.Since(start)
	chaosGPS := float64(totalGames) / chaosElapsed.Seconds()

	log.Printf("")
	log.Printf("=== CHAOS GAMES COMPLETE ===")
	log.Printf("  games:           %d", totalGames)
	log.Printf("  duration:        %s", chaosElapsed.Round(time.Millisecond))
	log.Printf("  throughput:      %.0f games/sec", chaosGPS)
	log.Printf("  crashes:         %d (in %d games)", len(allCrashes), gamesWithCrashes)
	log.Printf("  violations:      %d (in %d games)", len(allViolations), gamesWithViolations)
	log.Printf("  clean games:     %d", cleanGames)

	// =====================================================================
	// Phase 2: Nightmare Boards
	// =====================================================================
	log.Printf("")
	log.Printf("=== PHASE 2: NIGHTMARE BOARDS ===")

	nightmareStart := time.Now()
	nightmareJobs := make(chan int, workers*4)
	nightmareResults := make(chan nightmareResult, workers*4)
	var nightmareCompleted int64

	var nightmareWg sync.WaitGroup
	for w := 0; w < workers; w++ {
		nightmareWg.Add(1)
		go func() {
			defer nightmareWg.Done()
			for boardIdx := range nightmareJobs {
				result := runNightmareBoard(
					boardIdx, chaosCorpus, corpus, meta,
					*seatsFlag, *seedFlag,
				)
				nightmareResults <- result
				done := atomic.AddInt64(&nightmareCompleted, 1)
				if done%1000 == 0 || done == int64(*nightmareFlag) {
					elapsed := time.Since(nightmareStart).Seconds()
					bps := float64(done) / elapsed
					fmt.Fprintf(os.Stderr, "  nightmare: %d/%d boards (%.0f b/s)\n", done, *nightmareFlag, bps)
				}
			}
		}()
	}

	go func() {
		for i := 0; i < *nightmareFlag; i++ {
			nightmareJobs <- i
		}
		close(nightmareJobs)
	}()

	go func() {
		nightmareWg.Wait()
		close(nightmareResults)
	}()

	// Aggregate nightmare results.
	var nightmareViolations []chaosViolation
	var nightmareCrashList []nightmareResult
	nightmareViolationsByName := map[string]int{}
	nightmareClean := 0
	nightmareCrashCards := map[string]int{}

	for r := range nightmareResults {
		if r.Crashed {
			nightmareCrashList = append(nightmareCrashList, r)
			for _, cn := range r.CardNames {
				nightmareCrashCards[cn]++
			}
		}
		if len(r.Violations) > 0 {
			nightmareViolations = append(nightmareViolations, r.Violations...)
			for _, v := range r.Violations {
				nightmareViolationsByName[v.InvariantName]++
			}
		}
		if !r.Crashed && len(r.Violations) == 0 {
			nightmareClean++
		}
	}

	nightmareElapsed := time.Since(nightmareStart)
	nightmareBPS := float64(*nightmareFlag) / nightmareElapsed.Seconds()

	log.Printf("")
	log.Printf("=== NIGHTMARE BOARDS COMPLETE ===")
	log.Printf("  boards:          %d", *nightmareFlag)
	log.Printf("  duration:        %s", nightmareElapsed.Round(time.Millisecond))
	log.Printf("  throughput:      %.0f boards/sec", nightmareBPS)
	log.Printf("  crashes:         %d", len(nightmareCrashList))
	log.Printf("  violations:      %d", len(nightmareViolations))
	log.Printf("  clean boards:    %d", nightmareClean)

	// =====================================================================
	// SHADOW (driver-conversion plan step 1) summary
	// =====================================================================
	shadowReport()

	// =====================================================================
	// Statistical Analysis: Cards most correlated with violations
	// =====================================================================

	var correlations []cardCorrelation
	for name, vCount := range cardInViolationGames {
		cCount := cardInCleanGames[name]
		total := vCount + cCount
		if total < 3 { // need minimum sample
			continue
		}
		score := float64(vCount) / float64(total)
		correlations = append(correlations, cardCorrelation{
			Name:           name,
			ViolationGames: vCount,
			CleanGames:     cCount,
			Score:          score,
		})
	}
	sort.Slice(correlations, func(i, j int) bool {
		return correlations[i].Score > correlations[j].Score
	})

	// =====================================================================
	// Write Report
	// =====================================================================
	if *judgeJSONLPath != "" {
		writeJudgeJSONL(*judgeJSONLPath, allViolations, nightmareViolations)
	}
	if *violationsDumpPath != "" {
		writeViolationsDump(*violationsDumpPath, allViolations)
	}
	if *reportFlag != "" {
		writeReport(*reportFlag, reportData{
			TotalGames:          totalGames,
			Seed:                *seedFlag,
			Permutations:        *permsFlag,
			Seats:               *seatsFlag,
			MaxTurns:            *maxTurnsFlag,
			ChaosDuration:       chaosElapsed,
			ChaosGPS:            chaosGPS,
			Crashes:             allCrashes,
			GamesWithCrashes:    gamesWithCrashes,
			Violations:          allViolations,
			GamesWithViolations: gamesWithViolations,
			CleanGames:          cleanGames,
			ViolationsByName:    violationsByName,
			CrashCards:          crashCards,
			Correlations:        correlations,
			NightmareBoards:     *nightmareFlag,
			NightmareDuration:   nightmareElapsed,
			NightmareBPS:        nightmareBPS,
			NightmareViolations: nightmareViolations,
			NightmareCrashes:    nightmareCrashList,
			NightmareViolByName: nightmareViolationsByName,
			NightmareClean:      nightmareClean,
			NightmareCrashCards: nightmareCrashCards,
			CorpusSize:          len(chaosCorpus.All),
			LegendaryCreatures:  len(chaosCorpus.LegendaryCreatures),
		})
		log.Printf("")
		log.Printf("Report written to %s", *reportFlag)
	}

	// Exit code: 1 if any violations or crashes found.
	total := len(allCrashes) + len(allViolations) + len(nightmareCrashList) + len(nightmareViolations)
	if total > 0 {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Chaos game runner
// ---------------------------------------------------------------------------

// runChaosGameWatched runs one chaos game under an optional wall-clock
// watchdog. When timeout == 0 it's a direct passthrough. Otherwise the game
// runs in its own goroutine; if it doesn't finish within `timeout`, it is
// ABANDONED (the goroutine leaks — accepted in an offline fuzz run, mirrors
// cmd/hexdek-correctness's liveness watchdog) and reported as a
// Liveness:game_timeout violation. This lets a DEEP sweep (--max-turns
// 60-100) reach late-game states across thousands of games without a single
// board-explosion / mandatory-loop game stalling the whole run (the #1
// blocker flagged in the r63 longtail-sweep report).
func runChaosGameWatched(gameIdx, permutation int, timeout time.Duration,
	chaosCorpus *gameengine.ChaosCorpus,
	corpus *astload.Corpus,
	meta *deckparser.MetaDB,
	nSeats int, masterSeed int64, maxTurns int,
	seedCards []string, seedCmdr string, seedCardsAllSeats []string,
) chaosGameResult {
	if timeout <= 0 {
		return runChaosGame(gameIdx, permutation, chaosCorpus, corpus, meta,
			nSeats, masterSeed, maxTurns, seedCards, seedCmdr, seedCardsAllSeats)
	}
	resCh := make(chan chaosGameResult, 1)
	go func() {
		resCh <- runChaosGame(gameIdx, permutation, chaosCorpus, corpus, meta,
			nSeats, masterSeed, maxTurns, seedCards, seedCmdr, seedCardsAllSeats)
	}()
	select {
	case r := <-resCh:
		return r
	case <-time.After(timeout):
		deckSeed := masterSeed + int64(gameIdx)*10000 + 1
		return chaosGameResult{
			GameIdx: gameIdx,
			Violations: []chaosViolation{{
				GameIdx:       gameIdx,
				GameSeed:      deckSeed,
				Permutation:   permutation,
				InvariantName: "Liveness:game_timeout",
				Message:       fmt.Sprintf("game exceeded %s wall-clock budget and was abandoned (likely board explosion or mandatory loop); max-turns=%d", timeout, maxTurns),
			}},
		}
	}
}

func runChaosGame(gameIdx, permutation int,
	chaosCorpus *gameengine.ChaosCorpus,
	corpus *astload.Corpus,
	meta *deckparser.MetaDB,
	nSeats int, masterSeed int64, maxTurns int,
	seedCards []string, seedCmdr string, seedCardsAllSeats []string,
) (result chaosGameResult) {

	result.GameIdx = gameIdx

	// Use a seed that incorporates both game index AND permutation.
	// Same gameIdx + different permutation = same decks, different shuffle.
	deckSeed := masterSeed + int64(gameIdx)*10000 + 1
	shuffleSeed := deckSeed + int64(permutation)*100 + 7

	deckRng := rand.New(rand.NewSource(deckSeed))
	gameRng := rand.New(rand.NewSource(shuffleSeed))

	// Generate 4 random decks.
	chaosDecks := make([]*gameengine.ChaosDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		chaosDecks[i] = gameengine.GenerateChaosDeck(chaosCorpus, deckRng)
		if chaosDecks[i] == nil {
			result.Crashes = append(result.Crashes, chaosCrash{
				GameIdx:    gameIdx,
				GameSeed:   deckSeed,
				PanicValue: "failed to generate chaos deck",
			})
			return
		}
	}

	// Handler-focused fuzz: force seat 0's commander + seed cards.
	if seedCmdr != "" {
		for _, cc := range chaosCorpus.LegendaryCreatures {
			if cc.Name == seedCmdr {
				chaosDecks[0].Commander = cc
				break
			}
		}
	}
	if len(seedCards) > 0 {
		present := make(map[string]bool, len(chaosDecks[0].Cards))
		for _, n := range chaosDecks[0].Cards {
			present[n] = true
		}
		swapIdx := len(chaosDecks[0].Cards) - 1
		for _, name := range seedCards {
			if name == chaosDecks[0].Commander.Name || present[name] {
				continue
			}
			if swapIdx < 0 {
				chaosDecks[0].Cards = append(chaosDecks[0].Cards, name)
			} else {
				chaosDecks[0].Cards[swapIdx] = name
				swapIdx--
			}
			present[name] = true
		}
	}
	// --seed-cards-all-seats distributes the named cards round-robin
	// across every seat's deck. Useful for surfacing CROSS-SEAT
	// interactions (Hostage Taker + Bribery in different seats both
	// exile cards that need §400.7c owner-routing; --seed-cards alone
	// only stresses seat 0's handlers). Each seat gets ceil(len/nSeats)
	// cards from the list, rotating through. Conflicts with the seat's
	// commander or existing cards are skipped (mirrors the
	// --seed-cards branch above).
	if len(seedCardsAllSeats) > 0 {
		// Track per-seat presence + swap cursor.
		seatPresent := make([]map[string]bool, nSeats)
		seatSwapIdx := make([]int, nSeats)
		for i := 0; i < nSeats; i++ {
			seatPresent[i] = make(map[string]bool, len(chaosDecks[i].Cards))
			for _, n := range chaosDecks[i].Cards {
				seatPresent[i][n] = true
			}
			seatSwapIdx[i] = len(chaosDecks[i].Cards) - 1
		}
		for idx, name := range seedCardsAllSeats {
			seat := idx % nSeats
			if name == chaosDecks[seat].Commander.Name || seatPresent[seat][name] {
				continue
			}
			if seatSwapIdx[seat] < 0 {
				chaosDecks[seat].Cards = append(chaosDecks[seat].Cards, name)
			} else {
				chaosDecks[seat].Cards[seatSwapIdx[seat]] = name
				seatSwapIdx[seat]--
			}
			seatPresent[seat][name] = true
		}
	}

	// Collect commander names and all card names.
	result.Commanders = make([]string, nSeats)
	allCardSet := make(map[string]bool)
	for i, cd := range chaosDecks {
		result.Commanders[i] = cd.Commander.Name
		allCardSet[cd.Commander.Name] = true
		for _, name := range cd.Cards {
			allCardSet[name] = true
		}
	}
	result.AllCards = make([]string, 0, len(allCardSet))
	for name := range allCardSet {
		result.AllCards = append(result.AllCards, name)
	}

	// Convert chaos decks to gameengine.CommanderDeck objects.
	// This is the bridge between the chaos generator and the existing engine.
	defer func() {
		if r := recover(); r != nil {
			crash := chaosCrash{
				GameIdx:     gameIdx,
				GameSeed:    deckSeed,
				Permutation: permutation,
				PanicValue:  fmt.Sprintf("%v", r),
				StackTrace:  string(debug.Stack()),
				Commanders:  result.Commanders,
			}
			// Try to determine which card was in flight.
			crash.CardInFlight = extractCardFromStack(crash.StackTrace)
			result.Crashes = append(result.Crashes, crash)
		}
	}()

	gs := gameengine.NewGameState(nSeats, gameRng, corpus)
	if legalityEnabled {
		gs.Legality = gameengine.NewLegalityValidator(deckSeed)
	}
	if seatOutcomeEnabled {
		gs.SeatOutcome = gameengine.NewSeatOutcomeChecker()
	}

	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i, cd := range chaosDecks {
		// Build commander card.
		cmdrCard := buildCardFromName(cd.Commander.Name, corpus, meta)
		if cmdrCard == nil {
			// Commander not in AST corpus — create a bare-bones card.
			cmdrCard = &gameengine.Card{
				Name:          cd.Commander.Name,
				Owner:         i,
				Types:         []string{"legendary", "creature"},
				BasePower:     cd.Commander.Power,
				BaseToughness: cd.Commander.Toughness,
				CMC:           cd.Commander.CMC,
				Colors:        cd.Commander.Colors,
			}
			if cmdrCard.BaseToughness == 0 {
				cmdrCard.BaseToughness = 1 // prevent instant SBA death
			}
		} else {
			cmdrCard.Owner = i
		}

		// Build library cards.
		lib := make([]*gameengine.Card, 0, len(cd.Cards))
		for _, name := range cd.Cards {
			c := buildCardFromName(name, corpus, meta)
			if c == nil {
				// Card not in AST corpus — create bare-bones.
				c = &gameengine.Card{
					Name:  name,
					Owner: i,
				}
				// Look up the chaos card for type info.
				for _, cc := range chaosCorpus.All {
					if cc.Name == name {
						c.Types = cc.Types
						c.BasePower = cc.Power
						c.BaseToughness = cc.Toughness
						c.CMC = cc.CMC
						c.Colors = cc.Colors
						break
					}
				}
			} else {
				c.Owner = i
			}
			lib = append(lib, c)
		}

		// Shuffle the library with the per-permutation seed.
		gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })

		commanderDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: []*gameengine.Card{cmdrCard},
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

	gs.Active = gameRng.Intn(nSeats)
	gs.Turn = 1

	// Run invariants on initial state.
	checkChaosInvariants(gs, gameIdx, deckSeed, permutation, result.Commanders, &result)

	// Turn loop with invariant checking.
	for turn := 1; turn <= maxTurns; turn++ {
		gs.Turn = turn

		// Wrap each turn in a recover so one bad turn doesn't kill the game.
		func() {
			defer func() {
				if r := recover(); r != nil {
					crash := chaosCrash{
						GameIdx:     gameIdx,
						GameSeed:    deckSeed,
						Permutation: permutation,
						PanicValue:  fmt.Sprintf("turn %d: %v", turn, r),
						StackTrace:  string(debug.Stack()),
						Commanders:  result.Commanders,
					}
					crash.CardInFlight = extractCardFromStack(crash.StackTrace)
					result.Crashes = append(result.Crashes, crash)
				}
			}()
			tournament.TakeTurn(gs)
		}()

		checkChaosInvariants(gs, gameIdx, deckSeed, permutation, result.Commanders, &result)

		func() {
			defer func() {
				if r := recover(); r != nil {
					crash := chaosCrash{
						GameIdx:     gameIdx,
						GameSeed:    deckSeed,
						Permutation: permutation,
						PanicValue:  fmt.Sprintf("SBA turn %d: %v", turn, r),
						StackTrace:  string(debug.Stack()),
						Commanders:  result.Commanders,
					}
					crash.CardInFlight = extractCardFromStack(crash.StackTrace)
					result.Crashes = append(result.Crashes, crash)
				}
			}()
			gameengine.StateBasedActions(gs)
		}()

		checkChaosInvariants(gs, gameIdx, deckSeed, permutation, result.Commanders, &result)

		if gs.CheckEnd() {
			break
		}
		gs.Active = gameengine.NextLivingSeat(gs)

		// Safety: if too many crashes in one game, bail.
		if len(result.Crashes) > 10 {
			break
		}
	}

	// Drain ride-along legality violations into the same report stream as
	// the invariant census, namespaced "Legality:<rule>" so histograms
	// separate the live-action audit from the post-hoc state audit.
	if gs.Legality != nil {
		for _, lv := range gs.Legality.Violations {
			result.Violations = append(result.Violations, chaosViolation{
				GameIdx:       gameIdx,
				GameSeed:      deckSeed,
				Permutation:   permutation,
				InvariantName: "Legality:" + lv.Rule,
				Message:       lv.String(),
				Turn:          lv.Turn,
				Phase:         gs.Phase,
				Step:          gs.Step,
				Commanders:    result.Commanders,
			})
		}
	}

	// Drain seat-outcome self-checker violations, namespaced
	// "SeatOutcome:<kind>".
	if gs.SeatOutcome != nil {
		for _, sv := range gs.SeatOutcome.Violations {
			result.Violations = append(result.Violations, chaosViolation{
				GameIdx:       gameIdx,
				GameSeed:      deckSeed,
				Permutation:   permutation,
				InvariantName: "SeatOutcome:" + sv.Kind,
				Message:       sv.String(),
				Turn:          sv.Turn,
				Phase:         gs.Phase,
				Step:          gs.Step,
				Commanders:    result.Commanders,
			})
		}
	}

	result.Turns = gs.Turn
	return result
}

// ---------------------------------------------------------------------------
// Nightmare board runner
// ---------------------------------------------------------------------------

func runNightmareBoard(boardIdx int,
	chaosCorpus *gameengine.ChaosCorpus,
	corpus *astload.Corpus,
	meta *deckparser.MetaDB,
	nSeats int, masterSeed int64,
) (result nightmareResult) {

	result.BoardIdx = boardIdx

	boardSeed := masterSeed + int64(boardIdx)*7777 + 3
	rng := rand.New(rand.NewSource(boardSeed))

	defer func() {
		if r := recover(); r != nil {
			result.Crashed = true
			result.CrashErr = fmt.Sprintf("%v", r)
			result.StackTrace = string(debug.Stack())
		}
	}()

	// Generate the nightmare board.
	permsPerSeat := 5
	boards := gameengine.GenerateNightmareBoard(chaosCorpus, rng, nSeats, permsPerSeat)

	// Collect all card names.
	for _, seatCards := range boards {
		result.CardNames = append(result.CardNames, seatCards...)
	}

	// Build the game state with the nightmare board.
	gs := gameengine.NewGameState(nSeats, rng, corpus)
	gs.CommanderFormat = true

	for seatIdx, cardNames := range boards {
		seat := gs.Seats[seatIdx]
		seat.Life = 40
		seat.StartingLife = 40

		for _, name := range cardNames {
			card := buildCardFromName(name, corpus, meta)
			if card == nil {
				// Bare-bones card for unresolved names.
				card = &gameengine.Card{
					Name:  name,
					Owner: seatIdx,
				}
				// Look up in chaos corpus for type info.
				for _, cc := range chaosCorpus.All {
					if cc.Name == name {
						card.Types = cc.Types
						card.BasePower = cc.Power
						card.BaseToughness = cc.Toughness
						card.CMC = cc.CMC
						card.Colors = cc.Colors
						break
					}
				}
			} else {
				card.Owner = seatIdx
			}

			// Mint an OG InstanceID so the Phase-4 ZoneConservation
			// census (and the strict disappearance check behind
			// --instanceid-strict-census) actually runs on nightmare
			// boards. Pre-r63 nothing here minted, MintedInstanceIDs
			// stayed empty, and checkZoneConservation silently fell
			// back to the legacy count check — 40k "clean" strict-
			// census boards had never executed the census at all.
			gameengine.MintOGInstanceID(gs, card)

			perm := &gameengine.Permanent{
				Card:       card,
				Controller: seatIdx,
				Owner:      seatIdx,
				Timestamp:  gs.NextTimestamp(),
			}

			// Resolve ETB-choice defaults for 0/0 creatures that would
			// otherwise die to SBA 704.5f (Primal Plasma, Marath, etc.).
			if gameengine.ResolveETBChoiceDefaults(perm) {
				gs.InvalidateCharacteristicsCache()
			}

			seat.Battlefield = append(seat.Battlefield, perm)
			gameengine.RegisterReplacementsForPermanent(gs, perm)
		}

		// Give each seat a minimal library to avoid empty-library SBA triggers.
		for j := 0; j < 10; j++ {
			lib := &gameengine.Card{
				Name:  "Plains",
				Owner: seatIdx,
				Types: []string{"basic", "land", "plains"},
			}
			gameengine.MintOGInstanceID(gs, lib)
			seat.Library = append(seat.Library, lib)
		}
		seat.Hat = &hat.GreedyHat{}
	}

	result.MintedCount = len(gs.MintedInstanceIDs)

	// Run SBAs.
	func() {
		defer func() {
			if r := recover(); r != nil {
				result.Crashed = true
				result.CrashErr = fmt.Sprintf("SBA: %v", r)
				result.StackTrace = string(debug.Stack())
			}
		}()
		gameengine.StateBasedActions(gs)
	}()

	// Run invariants.
	if !result.Crashed {
		checkNightmareInvariants(gs, boardIdx, masterSeed, result.CardNames, &result)
	}

	// Run layer recalculation on every permanent.
	if !result.Crashed {
		func() {
			defer func() {
				if r := recover(); r != nil {
					result.Crashed = true
					result.CrashErr = fmt.Sprintf("layer recalc: %v", r)
					result.StackTrace = string(debug.Stack())
				}
			}()
			gs.InvalidateCharacteristicsCache()
			for _, s := range gs.Seats {
				if s == nil {
					continue
				}
				for _, p := range s.Battlefield {
					if p == nil || p.PhasedOut {
						continue
					}
					gameengine.GetEffectiveCharacteristics(gs, p)
				}
			}
		}()
	}

	// Run invariants again after layer recalc.
	if !result.Crashed {
		checkNightmareInvariants(gs, boardIdx, masterSeed, result.CardNames, &result)
	}

	return result
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildCardFromName delegates to the promoted shared builder (r63
// Judge CI gate) — DFC fallback + ETB-choice P/T defaults live in
// deckparser.BuildCardFromName now, shared with hexdek-judge --run.
func buildCardFromName(name string, corpus *astload.Corpus, meta *deckparser.MetaDB) *gameengine.Card {
	return deckparser.BuildCardFromName(name, corpus, meta)
}

// ---------------------------------------------------------------------------
// SHADOW (driver-conversion plan step 1): a Judge-sink mirror of the
// chaosViolation path, running ALONGSIDE it with ZERO behavior change.
//
// Every invariant violation already flows through judge.LogViolation at
// origin inside RunAllInvariants (consolidation step 4). This shadow
// registers ONE process-wide sink that tallies that canonical
// ValidationViolation stream by invariant name, and the chaosViolation
// path tallies its own recorded invariant rows by name in parallel. At
// the end of the run the two tallies must be IDENTICAL per name — proving
// the sink stream reproduces exactly the counts the report renderer reads
// today, which is the precondition for FLIP/DELETE (migrate the renderer
// onto the sink, delete chaosViolation). chaosViolation stays the SOLE
// source of truth here: the report renderer, --invariant filter, and
// --violations-dump all still read the old path.
//
// Why per-NAME global counts, not per-game attribution:
//
// loki generates games in parallel and the single global Judge sink fans
// out to ONE callback. Attributing each sink emission to its originating
// GAME would require either serializing every RunAllInvariants scan under
// a global lock (BenchmarkInvariants_* measures this at ~3x on the
// invariant fraction — it would blow the plan's ±5% throughput gate) or
// stamping game identity into the engine's origin emission (out of the
// additive SHADOW scope). The cadence-duplicate trap the equality check
// guards (plan §"cadence semantics") is about the tracked RAW COUNTS per
// invariant — "1,255 → 1,113" trend lines — which are global per-name
// aggregates. Counting per name reproduces exactly that quantity, and
// because BOTH tallies are driven by the SAME RunAllInvariants calls
// (the sink fires inside; the old path reads the same call's return), a
// per-name match is exact, not "modulo": a persistent violation
// re-reported on every cadence pass is counted identically on both sides.
//
// The lock is held ONLY to increment a per-name counter when a violation
// actually fires (rare) — never around the scan — so this is the literal
// realization of the plan's option 2 ("violations are rare so contention
// is nil") and adds no measurable throughput cost (clean games never call
// LogViolation at all, so the sink is never even invoked).
// ---------------------------------------------------------------------------

// shadowCounts tallies invariant violations by canonical name. Guarded by
// a mutex held only for the per-name increment, which happens once per
// emitted violation (rare). SHADOW only — nothing here feeds the report.
type shadowCounts struct {
	mu     sync.Mutex
	byName map[string]int
	total  int
}

func newShadowCounts() *shadowCounts { return &shadowCounts{byName: map[string]int{}} }

func (c *shadowCounts) inc(name string) {
	c.mu.Lock()
	c.byName[name]++
	c.total++
	c.mu.Unlock()
}

func (c *shadowCounts) snapshot() (map[string]int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.byName))
	for k, v := range c.byName {
		out[k] = v
	}
	return out, c.total
}

var (
	// shadowSinkCounts tallies the invariant stream as seen through the
	// Judge router (the FLIP-target source). shadowChaosCounts tallies the
	// invariant rows the chaosViolation path actually records (today's
	// source of truth). They must match per name.
	shadowSinkCounts  = newShadowCounts()
	shadowChaosCounts = newShadowCounts()
)

// shadowSink is registered once in main. It counts invariant-surface
// violations passing the active --invariant filter, mirroring the
// chaosViolation path's own filter so the two tallies are comparable.
// Non-invariant surfaces (ride-along legality / seat-outcome, only present
// under --legality / --seat-outcome) are ignored: those are drained into
// the report by the old path post-game, not via the invariant cadence,
// and their sink migration is a FLIP-step concern.
func shadowSink(v judge.ValidationViolation) {
	if v.Surface != judge.SurfaceInvariants {
		return
	}
	if !matchesInvariantFilter(v.Name, invariantFilter) {
		return
	}
	shadowSinkCounts.inc(v.Name)
}

func checkChaosInvariants(gs *gameengine.GameState, gameIdx int, gameSeed int64,
	permutation int, commanders []string, result *chaosGameResult) {
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		if !matchesInvariantFilter(v.Name, invariantFilter) {
			continue
		}
		// SHADOW: tally the chaosViolation invariant row by name so the
		// end-of-run check can compare against the Judge-sink tally.
		shadowChaosCounts.inc(v.Name)
		viol := chaosViolation{
			GameIdx:       gameIdx,
			GameSeed:      gameSeed,
			Permutation:   permutation,
			InvariantName: v.Name,
			Message:       v.Message,
			Turn:          gs.Turn,
			Phase:         gs.Phase,
			Step:          gs.Step,
			EventCount:    len(gs.EventLog),
			StateSummary:  gameengine.GameStateSummary(gs),
			RecentEvents:  gameengine.RecentEvents(gs, 20),
			Commanders:    commanders,
		}
		result.Violations = append(result.Violations, viol)
	}
}

func checkNightmareInvariants(gs *gameengine.GameState, boardIdx int, seed int64,
	cardNames []string, result *nightmareResult) {
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		if !matchesInvariantFilter(v.Name, invariantFilter) {
			continue
		}
		shadowChaosCounts.inc(v.Name)
		viol := chaosViolation{
			GameIdx:       boardIdx,
			GameSeed:      seed,
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

// shadowReport compares the two tallies and prints the SHADOW verdict.
// Returns the number of per-name mismatches (0 == sink reproduces the
// chaosViolation counts exactly → FLIP precondition satisfied).
func shadowReport() int {
	sink, sinkTotal := shadowSinkCounts.snapshot()
	chaos, chaosTotal := shadowChaosCounts.snapshot()

	names := map[string]struct{}{}
	for n := range sink {
		names[n] = struct{}{}
	}
	for n := range chaos {
		names[n] = struct{}{}
	}
	ordered := make([]string, 0, len(names))
	for n := range names {
		ordered = append(ordered, n)
	}
	sort.Strings(ordered)

	mismatches := 0
	log.Printf("")
	log.Printf("=== SHADOW (Judge-sink mirror) ===")
	log.Printf("  sink invariant rows:  %d", sinkTotal)
	log.Printf("  chaos invariant rows: %d", chaosTotal)
	for _, n := range ordered {
		if sink[n] != chaos[n] {
			mismatches++
			log.Printf("  ⚠ MISMATCH %-26s sink=%d chaos=%d", n, sink[n], chaos[n])
		}
	}
	if mismatches == 0 {
		log.Printf("  ✓ sink stream reproduces chaosViolation invariant counts exactly (per name)")
	} else {
		log.Printf("  ⚠ SHADOW DIVERGENCE — %d invariant name(s) differ; FLIP/DELETE are UNSAFE", mismatches)
	}
	return mismatches
}

// invariantNames returns the canonical (camelCase) names from a slice of
// gameengine.Invariant. Order preserved so help output matches the
// runner's evaluation order.
func invariantNames(invs []gameengine.Invariant) []string {
	out := make([]string, len(invs))
	for i, inv := range invs {
		out[i] = inv.Name
	}
	return out
}

// normalizeInvariantKey lowercases and strips hyphens/underscores so the
// CLI accepts both "ZoneConservation" and "zone-conservation" /
// "zone_conservation" / "zoneconservation" as equivalent.
func normalizeInvariantKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

// canonicalizeInvariantName resolves the user-supplied --invariant value
// against the set of known invariant names. Returns the canonical
// camelCase form on match, "" on empty input (match-all), or an error
// listing the valid names on a typo.
func canonicalizeInvariantName(input string, known []string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", nil
	}
	want := normalizeInvariantKey(input)
	for _, name := range known {
		if normalizeInvariantKey(name) == want {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown invariant %q (valid: %s)", input, strings.Join(known, ", "))
}

// matchesInvariantFilter returns true iff the violation's invariant name
// passes the active filter. Empty canonical = match-all (legacy
// behavior). Comparison is exact on the canonical camelCase name —
// canonicalizeInvariantName already normalized the user input at flag-
// parse time.
func matchesInvariantFilter(invariantName, canonical string) bool {
	if canonical == "" {
		return true
	}
	return invariantName == canonical
}

// extractCardFromStack tries to pull a card name from a panic stack trace.
// Looks for common patterns in the engine like "card=<name>" or DisplayName
// references. Returns "" if nothing found.
func extractCardFromStack(stack string) string {
	// Look for per_card handler function names which contain card identifiers.
	lines := strings.Split(stack, "\n")
	for _, line := range lines {
		if strings.Contains(line, "per_card") {
			// Extract function name.
			parts := strings.Fields(line)
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Report writer
// ---------------------------------------------------------------------------

type reportData struct {
	TotalGames          int
	Seed                int64
	Permutations        int
	Seats               int
	MaxTurns            int
	ChaosDuration       time.Duration
	ChaosGPS            float64
	Crashes             []chaosCrash
	GamesWithCrashes    int
	Violations          []chaosViolation
	GamesWithViolations int
	CleanGames          int
	ViolationsByName    map[string]int
	CrashCards          map[string]int
	Correlations        []cardCorrelation
	NightmareBoards     int
	NightmareDuration   time.Duration
	NightmareBPS        float64
	NightmareViolations []chaosViolation
	NightmareCrashes    []nightmareResult
	NightmareViolByName map[string]int
	NightmareClean      int
	NightmareCrashCards map[string]int
	CorpusSize          int
	LegendaryCreatures  int
}

// writeViolationsDump writes every chaos violation, one per line,
// tab-separated as game-idx<TAB>turn<TAB>invariant<TAB>message. Phase E
// diagnostic — bypasses the report's per-kind dedup so card-name
// histograms see the full population, not just 30 representatives.
// judgeJSONLRow mirrors muninn.JudgeLogRecord / hexapi.judgeViolationRecord
// (the grinder-violations.jsonl wire contract) so `hexdek-muninn
// --judge-triage` can cluster a loki sweep's violations by dimension +
// fingerprint exactly as it does the live grinder stream. JSON field
// names are the contract — keep them in sync with that struct.
type judgeJSONLRow struct {
	GameSeed  int64    `json:"game_seed"`
	DeckKeys  []string `json:"deck_keys,omitempty"`
	Dimension string   `json:"dimension"`
	Surface   string   `json:"surface,omitempty"`
	Rule      string   `json:"rule"`
	Severity  string   `json:"severity,omitempty"`
	Turn      int      `json:"turn,omitempty"`
	Detail    string   `json:"detail"`
}

// deriveJudgeDimension maps a chaosViolation's namespaced invariant name
// back to its canonical Judge dimension + surface + bare rule, matching
// the engine's own tagging (AllInvariants Dimension fields,
// Legality/SeatOutcome Canonical()). The aggregation path flattens these
// at origin, so this is the inverse used only for the offline triage row.
func deriveJudgeDimension(name string) (dim, surface, rule string) {
	switch {
	case strings.HasPrefix(name, "Legality:"):
		return judge.DimensionLegality, judge.SurfaceLegality, strings.TrimPrefix(name, "Legality:")
	case strings.HasPrefix(name, "SeatOutcome:"):
		// SeatOutcomeViolation.Canonical() tags state_integrity.
		return judge.DimensionStateIntegrity, judge.SurfaceSeatOutcome, strings.TrimPrefix(name, "SeatOutcome:")
	case strings.HasPrefix(name, "Liveness:"):
		return judge.DimensionLiveness, judge.SurfaceLiveness, strings.TrimPrefix(name, "Liveness:")
	case name == "ZoneConservation" || name == "CardIdentity":
		return judge.DimensionConservation, judge.SurfaceInvariants, name
	default:
		return judge.DimensionStateIntegrity, judge.SurfaceInvariants, name
	}
}

// writeJudgeJSONL serializes every chaos + nightmare violation as a
// grinder-violations.jsonl stream for `hexdek-muninn --judge-triage`. The
// seed/turn are the real repro coordinates from the aggregation path, so
// each cluster's representative row is replayable.
func writeJudgeJSONL(path string, chaos, nightmare []chaosViolation) {
	f, err := os.Create(path)
	if err != nil {
		log.Printf("judge-jsonl: %v", err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	emit := func(vs []chaosViolation) {
		for i := range vs {
			v := &vs[i]
			dim, surface, rule := deriveJudgeDimension(v.InvariantName)
			_ = enc.Encode(judgeJSONLRow{
				GameSeed:  v.GameSeed,
				DeckKeys:  v.Commanders,
				Dimension: dim,
				Surface:   surface,
				Rule:      rule,
				Severity:  judge.SeverityCritical,
				Turn:      v.Turn,
				Detail:    v.Message,
			})
		}
	}
	emit(chaos)
	emit(nightmare)
}

func writeViolationsDump(path string, vs []chaosViolation) {
	f, err := os.Create(path)
	if err != nil {
		log.Printf("violations-dump: %v", err)
		return
	}
	defer f.Close()
	for _, v := range vs {
		fmt.Fprintf(f, "%d\t%d\t%s\t%s\n", v.GameIdx, v.Turn, v.InvariantName, v.Message)
	}
}

func writeReport(path string, d reportData) {
	f, err := os.Create(path)
	if err != nil {
		log.Printf("write report: %v", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# Chaos Gauntlet Report\n\n")
	fmt.Fprintf(f, "Generated: %s\n\n", time.Now().Format(time.RFC3339))

	// Configuration
	fmt.Fprintf(f, "## Configuration\n\n")
	fmt.Fprintf(f, "| Parameter | Value |\n")
	fmt.Fprintf(f, "|-----------|-------|\n")
	fmt.Fprintf(f, "| Oracle Corpus | %d cards |\n", d.CorpusSize)
	fmt.Fprintf(f, "| Legendary Creatures | %d |\n", d.LegendaryCreatures)
	fmt.Fprintf(f, "| Total Games | %d |\n", d.TotalGames)
	fmt.Fprintf(f, "| Seed | %d |\n", d.Seed)
	fmt.Fprintf(f, "| Permutations | %d |\n", d.Permutations)
	fmt.Fprintf(f, "| Seats | %d |\n", d.Seats)
	fmt.Fprintf(f, "| Max Turns | %d |\n", d.MaxTurns)
	fmt.Fprintf(f, "| Nightmare Boards | %d |\n", d.NightmareBoards)
	fmt.Fprintf(f, "\n")

	// Summary
	fmt.Fprintf(f, "## Summary\n\n")
	fmt.Fprintf(f, "### Chaos Games\n\n")
	fmt.Fprintf(f, "| Metric | Count |\n")
	fmt.Fprintf(f, "|--------|-------|\n")
	fmt.Fprintf(f, "| Duration | %s |\n", d.ChaosDuration.Round(time.Millisecond))
	fmt.Fprintf(f, "| Throughput | %.0f games/sec |\n", d.ChaosGPS)
	fmt.Fprintf(f, "| Crashes | %d (in %d games) |\n", len(d.Crashes), d.GamesWithCrashes)
	fmt.Fprintf(f, "| Invariant Violations | %d (in %d games) |\n", len(d.Violations), d.GamesWithViolations)
	fmt.Fprintf(f, "| Clean Games | %d |\n", d.CleanGames)
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "### Nightmare Boards\n\n")
	fmt.Fprintf(f, "| Metric | Count |\n")
	fmt.Fprintf(f, "|--------|-------|\n")
	fmt.Fprintf(f, "| Duration | %s |\n", d.NightmareDuration.Round(time.Millisecond))
	fmt.Fprintf(f, "| Throughput | %.0f boards/sec |\n", d.NightmareBPS)
	fmt.Fprintf(f, "| Crashes | %d |\n", len(d.NightmareCrashes))
	fmt.Fprintf(f, "| Invariant Violations | %d |\n", len(d.NightmareViolations))
	fmt.Fprintf(f, "| Clean Boards | %d |\n", d.NightmareClean)
	fmt.Fprintf(f, "\n")

	// Crashes
	if len(d.Crashes) > 0 {
		fmt.Fprintf(f, "## Crashes (Chaos Games)\n\n")
		limit := len(d.Crashes)
		if limit > 50 {
			limit = 50
		}
		for i := 0; i < limit; i++ {
			c := &d.Crashes[i]
			fmt.Fprintf(f, "### Crash %d\n\n", i+1)
			fmt.Fprintf(f, "- **Game**: %d (seed %d, perm %d)\n", c.GameIdx, c.GameSeed, c.Permutation)
			fmt.Fprintf(f, "- **Commanders**: %s\n", strings.Join(c.Commanders, ", "))
			fmt.Fprintf(f, "- **Panic**: `%s`\n", c.PanicValue)
			if c.CardInFlight != "" {
				fmt.Fprintf(f, "- **Card in flight**: %s\n", c.CardInFlight)
			}
			fmt.Fprintf(f, "\n<details>\n<summary>Stack Trace</summary>\n\n```\n%s\n```\n\n</details>\n\n", c.StackTrace)
		}
		if len(d.Crashes) > limit {
			fmt.Fprintf(f, "*... and %d more crashes not shown.*\n\n", len(d.Crashes)-limit)
		}
	}

	// Violations
	if len(d.ViolationsByName) > 0 {
		fmt.Fprintf(f, "## Invariant Violations (Chaos Games)\n\n")
		fmt.Fprintf(f, "### By Invariant\n\n")
		fmt.Fprintf(f, "| Invariant | Count |\n")
		fmt.Fprintf(f, "|-----------|-------|\n")
		for name, count := range d.ViolationsByName {
			fmt.Fprintf(f, "| %s | %d |\n", name, count)
		}
		fmt.Fprintf(f, "\n")

		// Show up to 30 violation details PER invariant kind so the
		// report surfaces every cluster (CardIdentity / ZoneConservation /
		// AttachmentConsistency / etc.) instead of just whichever name
		// happens to win the iteration race at the top of the slice.
		// Bumped from 5 → 30 in r60 for the seeded-sweep analyses where
		// each invariant has 10K+ hits with potentially diverse
		// signatures.
		//
		// r60: dedup by (GameIdx, Message) — a single game that emits
		// the same violation on N cleanup passes contributes only ONE
		// detail. Without this, all 30 details for CardIdentity end up
		// being the same Kalitas-in-2-zones leak from game 7 because
		// it fires on every turn 22-50 cleanup pass.
		const perKind = 30
		idxByName := map[string][]int{}
		seenSig := map[string]bool{}
		for vi := range d.Violations {
			v := &d.Violations[vi]
			sig := fmt.Sprintf("g%d|%s", v.GameIdx, v.Message)
			if seenSig[sig] {
				continue
			}
			seenSig[sig] = true
			idxByName[v.InvariantName] = append(idxByName[v.InvariantName], vi)
		}
		var details []int
		for _, idxs := range idxByName {
			n := len(idxs)
			if n > perKind {
				n = perKind
			}
			details = append(details, idxs[:n]...)
		}
		fmt.Fprintf(f, "### Violation Details (up to %d per invariant kind, %d shown)\n\n", perKind, len(details))
		for k, vi := range details {
			v := &d.Violations[vi]
			i := k
			fmt.Fprintf(f, "#### Violation %d\n\n", i+1)
			fmt.Fprintf(f, "- **Game**: %d (seed %d, perm %d)\n", v.GameIdx, v.GameSeed, v.Permutation)
			fmt.Fprintf(f, "- **Invariant**: %s\n", v.InvariantName)
			fmt.Fprintf(f, "- **Turn**: %d, Phase=%s Step=%s\n", v.Turn, v.Phase, v.Step)
			fmt.Fprintf(f, "- **Commanders**: %s\n", strings.Join(v.Commanders, ", "))
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
		if len(d.Violations) > len(details) {
			fmt.Fprintf(f, "*... and %d more violations not shown.*\n\n", len(d.Violations)-len(details))
		}
	}

	// Nightmare board results.
	if len(d.NightmareCrashes) > 0 {
		fmt.Fprintf(f, "## Crashes (Nightmare Boards)\n\n")
		limit := len(d.NightmareCrashes)
		if limit > 30 {
			limit = 30
		}
		for i := 0; i < limit; i++ {
			nc := &d.NightmareCrashes[i]
			fmt.Fprintf(f, "### Nightmare Crash %d\n\n", i+1)
			fmt.Fprintf(f, "- **Board**: %d\n", nc.BoardIdx)
			fmt.Fprintf(f, "- **Cards**: %s\n", strings.Join(nc.CardNames, ", "))
			fmt.Fprintf(f, "- **Error**: `%s`\n", nc.CrashErr)
			if nc.StackTrace != "" {
				fmt.Fprintf(f, "\n<details>\n<summary>Stack Trace</summary>\n\n```\n%s\n```\n\n</details>\n\n", nc.StackTrace)
			}
		}
		if len(d.NightmareCrashes) > limit {
			fmt.Fprintf(f, "*... and %d more nightmare crashes not shown.*\n\n", len(d.NightmareCrashes)-limit)
		}
	}

	if len(d.NightmareViolByName) > 0 {
		fmt.Fprintf(f, "## Invariant Violations (Nightmare Boards)\n\n")
		fmt.Fprintf(f, "| Invariant | Count |\n")
		fmt.Fprintf(f, "|-----------|-------|\n")
		for name, count := range d.NightmareViolByName {
			fmt.Fprintf(f, "| %s | %d |\n", name, count)
		}
		fmt.Fprintf(f, "\n")

		// Per-board violation details (up to 5 per invariant kind, mirroring
		// the chaos-violations report). Needed to isolate which random card
		// combination is leaking — count-only isn't enough to find the bug.
		if len(d.NightmareViolations) > 0 {
			const perKind = 5
			idxByName := map[string][]int{}
			for vi := range d.NightmareViolations {
				name := d.NightmareViolations[vi].InvariantName
				idxByName[name] = append(idxByName[name], vi)
			}
			var details []int
			for _, idxs := range idxByName {
				n := len(idxs)
				if n > perKind {
					n = perKind
				}
				details = append(details, idxs[:n]...)
			}
			fmt.Fprintf(f, "### Nightmare Violation Details (up to %d per invariant kind, %d shown)\n\n", perKind, len(details))
			for k, vi := range details {
				v := &d.NightmareViolations[vi]
				fmt.Fprintf(f, "#### Nightmare Violation %d\n\n", k+1)
				fmt.Fprintf(f, "- **Board**: %d (seed %d)\n", v.GameIdx, v.GameSeed)
				fmt.Fprintf(f, "- **Invariant**: %s\n", v.InvariantName)
				fmt.Fprintf(f, "- **Message**: %s\n\n", v.Message)
				if v.StateSummary != "" {
					fmt.Fprintf(f, "<details>\n<summary>Board State</summary>\n\n```\n%s\n```\n\n</details>\n\n", v.StateSummary)
				}
			}
			if len(d.NightmareViolations) > len(details) {
				fmt.Fprintf(f, "*... and %d more nightmare violations not shown.*\n\n", len(d.NightmareViolations)-len(details))
			}
		}
	}

	// Statistical analysis: Top 10 cards correlated with violations.
	if len(d.Correlations) > 0 {
		fmt.Fprintf(f, "## Top Cards Correlated with Violations\n\n")
		fmt.Fprintf(f, "Cards that appeared disproportionately in violation games vs clean games.\n")
		fmt.Fprintf(f, "Only cards appearing in 3+ total games are shown.\n\n")
		fmt.Fprintf(f, "| Rank | Card | Violation Games | Clean Games | Correlation |\n")
		fmt.Fprintf(f, "|------|------|-----------------|-------------|-------------|\n")
		limit := len(d.Correlations)
		if limit > 10 {
			limit = 10
		}
		for i := 0; i < limit; i++ {
			c := &d.Correlations[i]
			fmt.Fprintf(f, "| %d | %s | %d | %d | %.2f |\n",
				i+1, c.Name, c.ViolationGames, c.CleanGames, c.Score)
		}
		fmt.Fprintf(f, "\n")
	}

	// Crash cards (cards associated with crashes).
	if len(d.CrashCards) > 0 {
		fmt.Fprintf(f, "## Cards Associated with Crashes\n\n")
		fmt.Fprintf(f, "| Card | Crash Count |\n")
		fmt.Fprintf(f, "|------|-------------|\n")
		type cardCount struct {
			Name  string
			Count int
		}
		var sorted []cardCount
		for name, count := range d.CrashCards {
			sorted = append(sorted, cardCount{name, count})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })
		limit := len(sorted)
		if limit > 20 {
			limit = 20
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(f, "| %s | %d |\n", sorted[i].Name, sorted[i].Count)
		}
		fmt.Fprintf(f, "\n")
	}

	if len(d.NightmareCrashCards) > 0 {
		fmt.Fprintf(f, "## Cards Associated with Nightmare Crashes\n\n")
		fmt.Fprintf(f, "| Card | Crash Count |\n")
		fmt.Fprintf(f, "|------|-------------|\n")
		type cardCount struct {
			Name  string
			Count int
		}
		var sorted []cardCount
		for name, count := range d.NightmareCrashCards {
			sorted = append(sorted, cardCount{name, count})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Count > sorted[j].Count })
		limit := len(sorted)
		if limit > 20 {
			limit = 20
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(f, "| %s | %d |\n", sorted[i].Name, sorted[i].Count)
		}
		fmt.Fprintf(f, "\n")
	}

	// Verdict
	total := len(d.Crashes) + len(d.Violations) + len(d.NightmareCrashes) + len(d.NightmareViolations)
	if total == 0 {
		fmt.Fprintf(f, "## Verdict: CLEAN\n\n")
		fmt.Fprintf(f, "All %d chaos games and %d nightmare boards passed all invariant checks with zero crashes.\n",
			d.TotalGames, d.NightmareBoards)
	} else {
		fmt.Fprintf(f, "## Verdict: ISSUES FOUND\n\n")
		fmt.Fprintf(f, "**%d total issues** across %d chaos games and %d nightmare boards.\n",
			total, d.TotalGames, d.NightmareBoards)
		fmt.Fprintf(f, "- %d crashes in chaos games\n", len(d.Crashes))
		fmt.Fprintf(f, "- %d invariant violations in chaos games\n", len(d.Violations))
		fmt.Fprintf(f, "- %d crashes in nightmare boards\n", len(d.NightmareCrashes))
		fmt.Fprintf(f, "- %d invariant violations in nightmare boards\n", len(d.NightmareViolations))
		fmt.Fprintf(f, "\nReview the details above to identify which cards and interactions are problematic.\n")
	}
}
