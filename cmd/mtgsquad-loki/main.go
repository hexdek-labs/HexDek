// mtgsquad-chaos — Chaos gauntlet stress test for the Go rules engine.
//
// Generates RANDOM Commander decks from the full 32K+ oracle corpus and
// runs 4-seat games with full invariant checking. Not curated decks —
// pure RNG nightmare decks. The goal is to find crashes and invariant
// violations caused by card combinations nobody designed test cases for.
//
// Usage:
//
//	go run ./cmd/mtgsquad-chaos/ --games 1000 --seed 42 --permutations 5
//	go run ./cmd/mtgsquad-chaos/ --games 1000 --seed 42 --nightmare-boards 10000
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
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// chaosViolation records a single invariant violation in a chaos game.
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
}

// ---------------------------------------------------------------------------
// Oracle corpus loader
// ---------------------------------------------------------------------------

// oracleEntry mirrors the Scryfall oracle-cards.json row fields we need.
type oracleEntry struct {
	Name          string   `json:"name"`
	TypeLine      string   `json:"type_line"`
	SetName       string   `json:"set_name"`
	ManaCost      string   `json:"mana_cost"`
	CMC           float64  `json:"cmc"`
	Colors        []string `json:"colors"`
	ColorIdentity []string `json:"color_identity"`
	Power         string   `json:"power"`
	Toughness     string   `json:"toughness"`
	OracleText    string   `json:"oracle_text"`
	Loyalty       string   `json:"loyalty"`
	Defense       string   `json:"defense"`
	CardFaces     []struct {
		Name      string   `json:"name"`
		TypeLine  string   `json:"type_line"`
		ManaCost  string   `json:"mana_cost"`
		Colors    []string `json:"colors"`
		Power     string   `json:"power"`
		Toughness string   `json:"toughness"`
	} `json:"card_faces"`
}

func loadOracleCorpus(path string) (*gameengine.ChaosCorpus, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open oracle %s: %w", path, err)
	}
	defer f.Close()

	var entries []oracleEntry
	if err := json.NewDecoder(f).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode oracle: %w", err)
	}

	// Un-sets excluded per project directive (7174n1c 2026-04-17).
	// These sets contain mechanics (widgets, augment, host, contraptions)
	// the engine doesn't handle, producing false-positive violations.
	unSets := map[string]bool{
		"Unstable": true, "Unhinged": true, "Unglued": true,
		"Unsanctioned": true, "Unfinity": true,
	}

	cards := make([]*gameengine.ChaosCard, 0, len(entries))
	for _, e := range entries {
		if e.Name == "" {
			continue
		}
		if unSets[e.SetName] {
			continue
		}
		typeLine := e.TypeLine
		if typeLine == "" && len(e.CardFaces) > 0 {
			typeLine = e.CardFaces[0].TypeLine
		}

		tlLower := strings.ToLower(typeLine)
		types := parseTypesSimple(typeLine)

		isLegendary := strings.Contains(tlLower, "legendary")
		isCreature := strings.Contains(tlLower, "creature")
		isLand := strings.Contains(tlLower, "land")

		// Basic land detection.
		basicNames := map[string]bool{
			"Plains": true, "Island": true, "Swamp": true,
			"Mountain": true, "Forest": true, "Wastes": true,
		}
		isBasicLand := isLand && (strings.Contains(tlLower, "basic") || basicNames[e.Name])

		// Parse P/T.
		pw, pwOK := atoiSafe(e.Power)
		tg, tgOK := atoiSafe(e.Toughness)
		if pw == 0 && tg == 0 && len(e.CardFaces) > 0 {
			pw, pwOK = atoiSafe(e.CardFaces[0].Power)
			tg, tgOK = atoiSafe(e.CardFaces[0].Toughness)
		}
		// Planeswalker loyalty as toughness surrogate.
		if tg == 0 {
			if loy, ok := atoiSafe(e.Loyalty); ok {
				tg = loy
			}
		}
		if tg == 0 {
			if def, ok := atoiSafe(e.Defense); ok {
				tg = def
			}
		}

		// ETB-choice default: cards like Primal Plasma, Primal Clay,
		// Aquamorph Entity etc. have */* P/T with "As ~ enters, choose"
		// text. Without ETB resolution they'd be 0/0 and die to SBA
		// 704.5f. Similarly, 0/0 creatures that "enter with +1/+1
		// counters" (Marath, Verazol) need a baseline. Apply safe
		// defaults at corpus-load time so every downstream consumer
		// (chaos games, nightmare boards) inherits the fix.
		if isCreature && pw == 0 && tg == 0 {
			otLower := strings.ToLower(e.OracleText)
			isPTStar := !pwOK || !tgOK // "*" fails atoiSafe
			isETBChoice := (strings.Contains(otLower, "as this creature enters") ||
				strings.Contains(otLower, "as it enters")) &&
				(strings.Contains(otLower, "choose") ||
					strings.Contains(otLower, "becomes your choice"))
			isETBCounters := strings.Contains(otLower, "enters with") &&
				strings.Contains(otLower, "+1/+1 counter")

			if isPTStar && isETBChoice {
				// Pick the balanced middle form (most of these offer 3/3).
				pw = 3
				tg = 3
			} else if isETBCounters {
				// Give a baseline so they survive SBAs.
				pw = 3
				tg = 3
			}
		}

		card := &gameengine.ChaosCard{
			Name:          e.Name,
			TypeLine:      typeLine,
			Types:         types,
			ManaCost:      e.ManaCost,
			CMC:           int(e.CMC + 0.5),
			Colors:        e.Colors,
			ColorIdentity: e.ColorIdentity,
			Power:         pw,
			Toughness:     tg,
			IsLegendary:   isLegendary,
			IsCreature:    isCreature,
			IsLand:        isLand,
			IsBasicLand:   isBasicLand,
		}
		cards = append(cards, card)
	}

	return gameengine.NewChaosCorpus(cards), nil
}

func parseTypesSimple(typeLine string) []string {
	if typeLine == "" {
		return nil
	}
	normalized := strings.ReplaceAll(typeLine, "\u2014", "-")
	var out []string
	for _, f := range strings.Fields(normalized) {
		f = strings.TrimSpace(f)
		if f == "" || f == "-" {
			continue
		}
		out = append(out, strings.ToLower(f))
	}
	return out
}

func atoiSafe(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "*" {
		return 0, false
	}
	n := 0
	neg := false
	i := 0
	if s[0] == '-' {
		neg = true
		i = 1
	} else if s[0] == '+' {
		i = 1
	}
	if i >= len(s) {
		return 0, false
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		n = -n
	}
	return n, true
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	var (
		gamesFlag     = flag.Int("games", 1000, "number of chaos games to run")
		seedFlag      = flag.Int64("seed", 42, "master RNG seed")
		permsFlag     = flag.Int("permutations", 1, "shuffles per deck set")
		seatsFlag     = flag.Int("seats", 4, "seats per game")
		maxTurnsFlag  = flag.Int("max-turns", 60, "max turns per game")
		workersFlag   = flag.Int("workers", 0, "worker goroutines (0 = NumCPU)")
		reportFlag    = flag.String("report", "data/rules/CHAOS_REPORT.md", "markdown report output path")
		astPath       = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset JSONL path")
		oraclePath    = flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json path")
		nightmareFlag = flag.Int("nightmare-boards", 10000, "number of nightmare board tests")
	)
	flag.Parse()

	workers := *workersFlag
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	log.Printf("mtgsquad-chaos starting")
	log.Printf("  games:           %d", *gamesFlag)
	log.Printf("  permutations:    %d", *permsFlag)
	log.Printf("  seed:            %d", *seedFlag)
	log.Printf("  seats:           %d", *seatsFlag)
	log.Printf("  max-turns:       %d", *maxTurnsFlag)
	log.Printf("  workers:         %d", workers)
	log.Printf("  nightmare-boards: %d", *nightmareFlag)

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

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				result := runChaosGame(
					job.gameIdx, job.permutation,
					chaosCorpus, corpus, meta,
					*seatsFlag, *seedFlag, *maxTurnsFlag,
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
	if *reportFlag != "" {
		writeReport(*reportFlag, reportData{
			TotalGames:           totalGames,
			Seed:                 *seedFlag,
			Permutations:         *permsFlag,
			Seats:                *seatsFlag,
			MaxTurns:             *maxTurnsFlag,
			ChaosDuration:        chaosElapsed,
			ChaosGPS:             chaosGPS,
			Crashes:              allCrashes,
			GamesWithCrashes:     gamesWithCrashes,
			Violations:           allViolations,
			GamesWithViolations:  gamesWithViolations,
			CleanGames:           cleanGames,
			ViolationsByName:     violationsByName,
			CrashCards:           crashCards,
			Correlations:         correlations,
			NightmareBoards:      *nightmareFlag,
			NightmareDuration:    nightmareElapsed,
			NightmareBPS:         nightmareBPS,
			NightmareViolations:  nightmareViolations,
			NightmareCrashes:     nightmareCrashList,
			NightmareViolByName:  nightmareViolationsByName,
			NightmareClean:       nightmareClean,
			NightmareCrashCards:  nightmareCrashCards,
			CorpusSize:           len(chaosCorpus.All),
			LegendaryCreatures:   len(chaosCorpus.LegendaryCreatures),
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

func runChaosGame(gameIdx, permutation int,
	chaosCorpus *gameengine.ChaosCorpus,
	corpus *astload.Corpus,
	meta *deckparser.MetaDB,
	nSeats int, masterSeed int64, maxTurns int,
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
				GameIdx:    gameIdx,
				GameSeed:   deckSeed,
				Permutation: permutation,
				PanicValue: fmt.Sprintf("%v", r),
				StackTrace: string(debug.Stack()),
				Commanders: result.Commanders,
			}
			// Try to determine which card was in flight.
			crash.CardInFlight = extractCardFromStack(crash.StackTrace)
			result.Crashes = append(result.Crashes, crash)
		}
	}()

	gs := gameengine.NewGameState(nSeats, gameRng, corpus)

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
		gs.Active = nextLivingSeat(gs)

		// Safety: if too many crashes in one game, bail.
		if len(result.Crashes) > 10 {
			break
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
			seat.Library = append(seat.Library, &gameengine.Card{
				Name:  "Plains",
				Owner: seatIdx,
				Types: []string{"basic", "land", "plains"},
			})
		}
		seat.Hat = &hat.GreedyHat{}
	}

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

func buildCardFromName(name string, corpus *astload.Corpus, meta *deckparser.MetaDB) *gameengine.Card {
	// Mirrors deckparser.buildCard logic but is exported for chaos use.
	var cardAST *gameast.CardAST
	if corpus != nil {
		cardAST, _ = corpus.Get(name)
	}
	md := meta.Get(name)

	if cardAST == nil && md == nil {
		return nil
	}

	c := &gameengine.Card{
		AST:   cardAST,
		Name:  name,
		Owner: -1,
	}
	if md != nil {
		c.Name = md.Name
		if len(md.Types) > 0 {
			c.Types = append([]string(nil), md.Types...)
		}
		c.BasePower = md.Power
		c.BaseToughness = md.Toughness
		if len(md.Colors) > 0 {
			c.Colors = append([]string(nil), md.Colors...)
		}
		c.CMC = md.CMC
		c.TypeLine = md.TypeLine
	}

	// ETB-choice P/T fix: if this is a creature with 0/0 base P/T and
	// an "As ~ enters, choose" ability (detected via AST oracle text),
	// set a safe default so SBA 704.5f doesn't immediately kill it.
	// This covers cards like Primal Plasma, Primal Clay, Aquamorph
	// Entity, Corrupted Shapeshifter, etc.
	isCreature := false
	for _, t := range c.Types {
		if t == "creature" {
			isCreature = true
			break
		}
	}
	if isCreature && c.BasePower == 0 && c.BaseToughness == 0 {
		ot := gameengine.OracleTextLower(c)
		if gameengine.HasETBChoicePatternExported(ot) {
			c.BasePower = 3
			c.BaseToughness = 3
		}
	}

	return c
}

func checkChaosInvariants(gs *gameengine.GameState, gameIdx int, gameSeed int64,
	permutation int, commanders []string, result *chaosGameResult) {
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
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

		// Show first 30 violation details.
		limit := len(d.Violations)
		if limit > 30 {
			limit = 30
		}
		fmt.Fprintf(f, "### Violation Details (first %d)\n\n", limit)
		for i := 0; i < limit; i++ {
			v := &d.Violations[i]
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
		if len(d.Violations) > limit {
			fmt.Fprintf(f, "*... and %d more violations not shown.*\n\n", len(d.Violations)-limit)
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
