// mtgsquad-valkyrie — Deck regression runner for the Go rules engine.
//
// Loads every saved deck from data/decks/, plays real Commander games
// with GreedyHat opponents, and reports crashes, parser gaps, invariant
// violations, empty-option turns, zero-mana games, and commanders that
// were never offered as castable.
//
// Usage:
//
//	go run ./cmd/mtgsquad-valkyrie/
//	go run ./cmd/mtgsquad-valkyrie/ --decks data/decks/lyon --games 10
//	go run ./cmd/mtgsquad-valkyrie/ --verbose --fail-fast
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
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

var hatTypes = []string{"greedy", "octo", "poker"}

func makeHat(kind string) gameengine.Hat {
	switch kind {
	case "octo":
		return &hat.OctoHat{}
	case "poker":
		return hat.NewPokerHat()
	default:
		return &hat.GreedyHat{}
	}
}

type deckIssue struct {
	Kind    string // crash, parser_gap, invariant, empty_options, zero_mana, commander_never_cast, short_deck
	Detail  string
	HatKind string
	Turn    int
	GameIdx int
}

type deckResult struct {
	Path       string
	Commander  string
	HatKind    string
	Games      int
	Issues     []deckIssue
	Unresolved []string
}

func main() {
	decksDir := flag.String("decks", "data/decks", "root directory to scan for .txt deck files")
	gamesPerDeck := flag.Int("games", 5, "games to play per deck")
	workers := flag.Int("workers", 0, "parallel workers (0 = NumCPU)")
	verbose := flag.Bool("verbose", false, "print per-game details")
	failFast := flag.Bool("fail-fast", false, "stop on first crash")
	astPath := flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset path")
	oraclePath := flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json")
	maxTurns := flag.Int("max-turns", 40, "max turns per game")
	seed := flag.Int64("seed", 42, "master RNG seed")
	flag.Parse()

	if *workers <= 0 {
		*workers = runtime.NumCPU()
	}

	log.Println("mtgsquad-valkyrie — deck regression runner")

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
		log.Fatalf("meta: %v", err)
	}
	if *oraclePath != "" {
		meta.SupplementWithOracleJSON(*oraclePath)
	}

	// Discover all deck files.
	deckPaths := discoverDecks(*decksDir)
	if len(deckPaths) == 0 {
		log.Fatalf("no .txt deck files found in %s", *decksDir)
	}
	log.Printf("found %d deck files", len(deckPaths))

	// Parse all decks upfront.
	var parsedDecks []*deckparser.TournamentDeck
	var parsedPaths []string
	var results []deckResult

	for _, p := range deckPaths {
		d, perr := deckparser.ParseDeckFile(p, corpus, meta)
		if perr != nil {
			results = append(results, deckResult{
				Path:  p,
				Games: 0,
				Issues: []deckIssue{{
					Kind:   "parse_error",
					Detail: perr.Error(),
				}},
			})
			continue
		}
		totalCards := len(d.Library) + len(d.CommanderCards)
		if totalCards < 60 {
			results = append(results, deckResult{
				Path:      p,
				Commander: d.CommanderName,
				Games:     0,
				Issues: []deckIssue{{
					Kind:   "short_deck",
					Detail: fmt.Sprintf("%d cards (need 100)", totalCards),
				}},
				Unresolved: d.Unresolved,
			})
			continue
		}
		parsedDecks = append(parsedDecks, d)
		parsedPaths = append(parsedPaths, p)
	}

	if len(parsedDecks) < 4 {
		log.Fatalf("need at least 4 valid decks for 4-seat games, got %d", len(parsedDecks))
	}

	totalGamesPlanned := len(parsedDecks) * *gamesPerDeck * len(hatTypes)
	log.Printf("parsed %d decks successfully, %d skipped", len(parsedDecks), len(results))
	log.Printf("running %d games per deck x %d hat types (%d total) with %d workers ...",
		*gamesPerDeck, len(hatTypes), totalGamesPlanned, *workers)

	// Run regression games — every deck x every hat type x N games.
	type workItem struct {
		deckIdx int
		gameIdx int
		hatKind string
	}

	// Key for merging: deckIdx + hatKind
	type mergeKey struct {
		deckIdx int
		hatKind string
	}

	work := make(chan workItem, *workers*4)
	var mu sync.Mutex
	gameResults := map[mergeKey][]deckResult{}
	stopped := false

	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				if *failFast {
					mu.Lock()
					s := stopped
					mu.Unlock()
					if s {
						return
					}
				}

				dr := runRegressionGame(
					parsedDecks, parsedPaths, item.deckIdx, item.gameIdx,
					item.hatKind, *seed, *maxTurns, *verbose,
				)

				mk := mergeKey{item.deckIdx, item.hatKind}
				mu.Lock()
				gameResults[mk] = append(gameResults[mk], dr)
				if *failFast && hasCrash(dr.Issues) {
					stopped = true
				}
				mu.Unlock()
			}
		}()
	}

	for di := range parsedDecks {
		for _, hk := range hatTypes {
			for gi := 0; gi < *gamesPerDeck; gi++ {
				work <- workItem{deckIdx: di, gameIdx: gi, hatKind: hk}
			}
		}
	}
	close(work)
	wg.Wait()

	// Merge per-game results into per-deck-per-hat results.
	for di := range parsedDecks {
		for _, hk := range hatTypes {
			mk := mergeKey{di, hk}
			grs := gameResults[mk]
			merged := deckResult{
				Path:      parsedPaths[di],
				Commander: parsedDecks[di].CommanderName,
				HatKind:   hk,
				Games:     len(grs),
			}
			for _, gr := range grs {
				merged.Issues = append(merged.Issues, gr.Issues...)
				if len(gr.Unresolved) > 0 && len(merged.Unresolved) == 0 {
					merged.Unresolved = gr.Unresolved
				}
			}
			results = append(results, merged)
		}
	}

	// Print report.
	printReport(results, *gamesPerDeck)
}

func runRegressionGame(
	allDecks []*deckparser.TournamentDeck, allPaths []string,
	deckIdx, gameIdx int, hatKind string, masterSeed int64, maxTurns int, verbose bool,
) deckResult {
	dr := deckResult{
		Path:      allPaths[deckIdx],
		Commander: allDecks[deckIdx].CommanderName,
		HatKind:   hatKind,
		Games:     1,
	}

	defer func() {
		if r := recover(); r != nil {
			dr.Issues = append(dr.Issues, deckIssue{
				Kind:    "crash",
				Detail:  fmt.Sprintf("%v", r),
				GameIdx: gameIdx,
			})
		}
	}()

	nSeats := 4
	gameSeed := masterSeed + int64(deckIdx)*1000 + int64(gameIdx)*7 + 1
	rng := rand.New(rand.NewSource(gameSeed))

	gs := gameengine.NewGameState(nSeats, rng, nil)

	// Seat 0 = test deck, seats 1-3 = random opponents from pool.
	opponents := pickOpponents(allDecks, deckIdx, nSeats-1, rng)
	seatDecks := append([]*deckparser.TournamentDeck{allDecks[deckIdx]}, opponents...)

	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	for i := 0; i < nSeats; i++ {
		tpl := seatDecks[i]
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

	// Seat 0 gets the test hat, opponents get GreedyHat.
	gs.Seats[0].Hat = makeHat(hatKind)
	for i := 1; i < nSeats; i++ {
		gs.Seats[i].Hat = &hat.GreedyHat{}
	}

	// London mulligan.
	for i := 0; i < nSeats; i++ {
		tournament.RunLondonMulligan(gs, i)
	}

	gs.Active = rng.Intn(nSeats)
	gs.Turn = 1

	// Track diagnostics for seat 0 (the test deck).
	manaProduced := false
	commanderOffered := false
	emptyOptionTurns := 0

	for turn := 1; turn <= maxTurns; turn++ {
		gs.Turn = turn
		tournament.TakeTurn(gs)
		gameengine.StateBasedActions(gs)

		// Check seat 0 diagnostics from this turn's events.
		for _, ev := range gs.EventLog {
			if ev.Seat == 0 && ev.Kind == "add_mana" {
				manaProduced = true
			}
			if ev.Seat == 0 && (ev.Kind == "commander_cast_from_command_zone" || ev.Kind == "cast") {
				if ev.Details != nil {
					if fz, ok := ev.Details["from_zone"].(string); ok && fz == "command_zone" {
						commanderOffered = true
					}
				}
				if ev.Kind == "commander_cast_from_command_zone" {
					commanderOffered = true
				}
			}
		}

		// Check for empty options: seat 0 has untapped lands + cards in hand but did nothing.
		seat0 := gs.Seats[0]
		if seat0 != nil && !seat0.Lost && gs.Active == 0 {
			untappedLands := 0
			for _, p := range seat0.Battlefield {
				if p != nil && !p.Tapped && p.IsLand() {
					untappedLands++
				}
			}
			handSize := len(seat0.Hand)
			castableExists := false
			for _, c := range seat0.Hand {
				if c != nil && int(c.CMC) <= untappedLands {
					castableExists = true
					break
				}
			}
			if untappedLands >= 2 && handSize >= 2 && castableExists {
				// Check if seat 0 actually cast anything this turn.
				castThisTurn := false
				for _, ev := range gs.EventLog {
					if ev.Seat == 0 && ev.Kind == "cast_spell" {
						castThisTurn = true
						break
					}
				}
				if !castThisTurn {
					emptyOptionTurns++
				}
			}
		}

		if gs.CheckEnd() {
			break
		}
		// Advance to next living seat.
		gs.Active = nextLiving(gs)
	}

	// Post-game checks.

	// Invariant violations.
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		dr.Issues = append(dr.Issues, deckIssue{
			Kind:    "invariant",
			Detail:  fmt.Sprintf("%s: %s", v.Name, v.Message),
			GameIdx: gameIdx,
		})
	}

	// Parser gaps from event log.
	gapCounts := map[string]int{}
	for _, ev := range gs.EventLog {
		if ev.Kind == "parser_gap" && ev.Details != nil {
			if s, ok := ev.Details["snippet"].(string); ok && s != "" {
				gapCounts[s]++
			}
		}
	}
	for snippet, count := range gapCounts {
		dr.Issues = append(dr.Issues, deckIssue{
			Kind:    "parser_gap",
			Detail:  fmt.Sprintf("%s (x%d)", snippet, count),
			GameIdx: gameIdx,
		})
	}

	if !manaProduced {
		dr.Issues = append(dr.Issues, deckIssue{
			Kind:    "zero_mana",
			Detail:  "seat 0 never produced mana",
			GameIdx: gameIdx,
		})
	}

	if !commanderOffered && gs.Turn >= 6 {
		dr.Issues = append(dr.Issues, deckIssue{
			Kind:    "commander_never_cast",
			Detail:  fmt.Sprintf("commander %q never cast in %d turns", dr.Commander, gs.Turn),
			GameIdx: gameIdx,
		})
	}

	if emptyOptionTurns >= 3 {
		dr.Issues = append(dr.Issues, deckIssue{
			Kind:    "empty_options",
			Detail:  fmt.Sprintf("%d turns with mana+cards but no casts", emptyOptionTurns),
			GameIdx: gameIdx,
		})
	}

	if verbose && len(dr.Issues) > 0 {
		log.Printf("  [%s game %d] %s — %d issues", hatKind, gameIdx, filepath.Base(dr.Path), len(dr.Issues))
		for _, iss := range dr.Issues {
			log.Printf("    %s: %s", iss.Kind, iss.Detail)
		}
	}

	return dr
}

func pickOpponents(decks []*deckparser.TournamentDeck, exclude, n int, rng *rand.Rand) []*deckparser.TournamentDeck {
	indices := make([]int, 0, len(decks)-1)
	for i := range decks {
		if i != exclude {
			indices = append(indices, i)
		}
	}
	rng.Shuffle(len(indices), func(a, b int) { indices[a], indices[b] = indices[b], indices[a] })
	result := make([]*deckparser.TournamentDeck, n)
	for i := 0; i < n; i++ {
		result[i] = decks[indices[i%len(indices)]]
	}
	return result
}

func nextLiving(gs *gameengine.GameState) int {
	n := len(gs.Seats)
	for k := 1; k <= n; k++ {
		cand := (gs.Active + k) % n
		if s := gs.Seats[cand]; s != nil && !s.Lost {
			return cand
		}
	}
	return gs.Active
}

func discoverDecks(root string) []string {
	var paths []string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == "freya" {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".txt") {
			paths = append(paths, p)
		}
		return nil
	})
	return paths
}

func hasCrash(issues []deckIssue) bool {
	for _, iss := range issues {
		if iss.Kind == "crash" {
			return true
		}
	}
	return false
}

func printReport(results []deckResult, gamesPerDeck int) {
	totalGames := 0
	totalIssues := 0
	passDecks := 0
	issueDecks := 0
	crashes := 0
	parserGaps := map[string]int{}

	for _, r := range results {
		totalGames += r.Games
		if len(r.Issues) == 0 {
			passDecks++
		} else {
			issueDecks++
		}
		for _, iss := range r.Issues {
			totalIssues++
			if iss.Kind == "crash" {
				crashes++
			}
			if iss.Kind == "parser_gap" {
				parserGaps[iss.Detail]++
			}
		}
	}

	fmt.Println()
	fmt.Println("VALKYRIE — Deck Regression Report")
	fmt.Println("===================================")
	fmt.Printf("Decks scanned: %d\n", len(results))
	fmt.Printf("Hat types:     %s\n", strings.Join(hatTypes, ", "))
	fmt.Printf("Games played:  %d\n", totalGames)
	fmt.Printf("Crashes:       %d\n", crashes)
	fmt.Println()

	if passDecks > 0 {
		fmt.Printf("PASS: %d deck/hat combos (no issues)\n", passDecks)
	}

	if issueDecks > 0 {
		fmt.Printf("\nISSUES (%d deck/hat combos):\n", issueDecks)
		for _, r := range results {
			if len(r.Issues) == 0 {
				continue
			}
			fmt.Printf("\n  %s", r.Path)
			if r.Commander != "" {
				fmt.Printf(" [%s]", r.Commander)
			}
			if r.HatKind != "" {
				fmt.Printf(" (hat: %s)", r.HatKind)
			}
			fmt.Println()
			if len(r.Unresolved) > 0 {
				fmt.Printf("    unresolved cards: %s\n", strings.Join(r.Unresolved, ", "))
			}
			byKind := map[string][]deckIssue{}
			for _, iss := range r.Issues {
				byKind[iss.Kind] = append(byKind[iss.Kind], iss)
			}
			for kind, issues := range byKind {
				if len(issues) == 1 {
					fmt.Printf("    %s: %s\n", kind, issues[0].Detail)
				} else {
					fmt.Printf("    %s (x%d):\n", kind, len(issues))
					seen := map[string]bool{}
					for _, iss := range issues {
						if !seen[iss.Detail] {
							fmt.Printf("      - %s\n", iss.Detail)
							seen[iss.Detail] = true
						}
					}
				}
			}
		}
	}

	if len(parserGaps) > 0 {
		fmt.Println("\nParser Gap Summary:")
		for gap, count := range parserGaps {
			fmt.Printf("  %s — %d occurrences\n", gap, count)
		}
	}

	fmt.Println()
	if crashes == 0 && totalIssues == 0 {
		fmt.Println("All clear. No crashes, no invariant violations, no parser gaps.")
	} else {
		fmt.Printf("Total issues: %d across %d decks\n", totalIssues, issueDecks)
	}
}
