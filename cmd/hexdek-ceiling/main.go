// hexdek-ceiling — Ceiling calibration: identify the best B5 combo deck after
// extended Curse evolution as the upper-bound reference point for the rating system.
//
// Scans all decks for bracket 5, cross-references Curse pool fitness and ELO
// records, selects the top 5 candidates, runs a focused gauntlet with
// accelerated Curse evolution, and writes the ceiling reference to
// data/calibration/ceiling.json.
//
// Usage:
//
//	hexdek-ceiling                          # full pipeline: scan + gauntlet + emit
//	hexdek-ceiling --scan-only             # just print top B5 candidates
//	hexdek-ceiling --games 10000           # games in ceiling gauntlet (default 10000)
//	hexdek-ceiling --evolve-every 50       # curse evolution interval (default 50)
//	hexdek-ceiling --top 5                 # number of candidates for gauntlet
//	hexdek-ceiling --decks data/decks      # deck directory
//	hexdek-ceiling --ast data/rules/ast_dataset.jsonl
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/tournament"
)

// CeilingResult is the final calibration output written to ceiling.json.
type CeilingResult struct {
	DeckKey         string          `json:"deck_key"`
	Commander       string          `json:"commander"`
	Owner           string          `json:"owner"`
	Archetype       string          `json:"archetype"`
	Bracket         int             `json:"bracket"`
	ELO             float64         `json:"elo"`
	HexRating       float64         `json:"hex_rating"`
	WinRate         float64         `json:"win_rate"`
	GamesPlayed     int             `json:"games_played"`
	GauntletWins    int             `json:"gauntlet_wins"`
	GauntletGames   int             `json:"gauntlet_games"`
	GauntletWinRate float64         `json:"gauntlet_win_rate"`
	Generation      int             `json:"generation"`
	BestDNA         *hat.CurseDNA  `json:"best_dna"`
	PowerPercentile int             `json:"power_percentile"`
	GameplanSummary string          `json:"gameplan_summary"`
	CalibratedAt    time.Time       `json:"calibrated_at"`
	RunnerUp        []RunnerUpEntry `json:"runner_up"`
}

// RunnerUpEntry tracks the other top candidates for context.
type RunnerUpEntry struct {
	DeckKey   string  `json:"deck_key"`
	Commander string  `json:"commander"`
	WinRate   float64 `json:"win_rate"`
	ELO       float64 `json:"elo"`
}

// CeilingCandidate is a B5 deck with combined scoring from Curse fitness + ELO.
type CeilingCandidate struct {
	DeckKey         string
	DeckPath        string
	Commander       string
	Owner           string
	Archetype       string
	Bracket         int
	PowerPercentile int
	GameplanSummary string
	ComboCount      int
	TutorCount      int

	// From Curse pool (if available)
	CurseFitness float64
	CurseGen     int
	BestDNA       *hat.CurseDNA

	// From ELO records (if available)
	ELO       float64
	HexRating float64
	ELOGames  int
	ELOWins   int
	WinRate   float64

	// Combined score for ranking
	Score float64
}

const (
	defaultGauntletGames = 10000
	defaultEvolveEvery   = 50
	defaultTopN          = 5
	showmatchSeats       = 4
	showmatchMaxTurn     = 80
)

func main() {
	var (
		decksDir    = flag.String("decks", "data/decks", "deck directory to scan")
		astPath     = flag.String("ast", "data/rules/ast_dataset.jsonl", "AST dataset path")
		oraclePath  = flag.String("oracle", "data/rules/oracle-cards.json", "Scryfall oracle-cards.json")
		dbPath      = flag.String("db", "data/hexdek.db", "SQLite database path")
		curseDir   = flag.String("curse", "data/curse", "Curse pool directory")
		outPath     = flag.String("out", "data/calibration/ceiling.json", "output calibration file")
		games       = flag.Int("games", defaultGauntletGames, "gauntlet games")
		evolveEvery = flag.Int("evolve-every", defaultEvolveEvery, "curse evolution interval (accelerated)")
		top         = flag.Int("top", defaultTopN, "number of top candidates for gauntlet")
		workers     = flag.Int("workers", 0, "parallel workers (0 = NumCPU/2)")
		scanOnly    = flag.Bool("scan-only", false, "just print candidates, don't run gauntlet")
		seed        = flag.Int64("seed", 0, "RNG seed (0 = time-based)")
	)
	flag.Parse()

	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}
	if *workers <= 0 {
		*workers = runtime.NumCPU() / 2
		if *workers < 2 {
			*workers = 2
		}
	}

	log.Printf("ceiling: scanning B5 decks in %s", *decksDir)

	// Phase 1: Load ELO records from SQLite.
	eloMap := loadELORecords(*dbPath)

	// Phase 2: Load Curse pools.
	rng := rand.New(rand.NewSource(*seed))
	cursePools := loadCursePools(*curseDir, rng)

	// Phase 3: Scan all decks for B5 candidates.
	candidates := scanB5Candidates(*decksDir, eloMap, cursePools)

	if len(candidates) == 0 {
		log.Fatal("ceiling: no B5 decks found")
	}

	// Sort by combined score.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Print candidates.
	fmt.Printf("\nCeiling Calibration — Top B5 Candidates (%d total B5 decks)\n", len(candidates))
	fmt.Println(strings.Repeat("=", 120))
	fmt.Printf("%-4s  %-45s  %-25s  %-10s  %6s  %6s  %6s  %7s\n",
		"#", "Deck Key", "Commander", "Archetype", "ELO", "WR%", "Fitness", "Score")
	fmt.Println(strings.Repeat("-", 120))

	limit := *top * 2 // Show more than we'll use
	if limit > len(candidates) {
		limit = len(candidates)
	}
	for i, c := range candidates[:limit] {
		wr := ""
		if c.ELOGames > 0 {
			wr = fmt.Sprintf("%.1f%%", c.WinRate*100)
		}
		elo := ""
		if c.ELO > 0 {
			elo = fmt.Sprintf("%.0f", c.ELO)
		}
		fit := ""
		if c.CurseFitness > 0 {
			fit = fmt.Sprintf("%.3f", c.CurseFitness)
		}
		fmt.Printf("%-4d  %-45s  %-25s  %-10s  %6s  %6s  %6s  %7.1f\n",
			i+1, truncate(c.DeckKey, 45), truncate(c.Commander, 25),
			truncate(c.Archetype, 10), elo, wr, fit, c.Score)
	}
	fmt.Println()

	if *scanOnly {
		return
	}

	// Phase 4: Load AST corpus + parse top candidate decks for gauntlet.
	topN := *top
	if topN > len(candidates) {
		topN = len(candidates)
	}
	gauntletCandidates := candidates[:topN]

	log.Printf("ceiling: loading AST corpus from %s", *astPath)
	corpus, err := astload.Load(*astPath)
	if err != nil {
		log.Fatalf("ceiling: astload: %v", err)
	}
	log.Printf("ceiling: %d cards loaded", corpus.Count())

	meta, err := deckparser.LoadMetaFromJSONL(*astPath)
	if err != nil {
		log.Fatalf("ceiling: meta: %v", err)
	}
	if *oraclePath != "" {
		if err := meta.SupplementWithOracleJSON(*oraclePath); err != nil {
			log.Printf("ceiling: oracle supplement: %v (continuing)", err)
		}
	}

	// Parse gauntlet deck files.
	var gauntletDecks []*deckparser.TournamentDeck
	var gauntletKeys []string
	var gauntletStrategies []*hat.StrategyProfile
	for _, c := range gauntletCandidates {
		d, err := deckparser.ParseDeckFile(c.DeckPath, corpus, meta)
		if err != nil {
			log.Printf("ceiling: skip %s: parse error: %v", c.DeckKey, err)
			continue
		}
		gauntletDecks = append(gauntletDecks, d)
		gauntletKeys = append(gauntletKeys, c.DeckKey)
		sp := hat.LoadStrategyFromFreya(c.DeckPath)
		gauntletStrategies = append(gauntletStrategies, sp)
	}

	if len(gauntletDecks) < showmatchSeats {
		log.Fatalf("ceiling: only %d valid gauntlet decks, need at least %d", len(gauntletDecks), showmatchSeats)
	}

	// Phase 5: Run focused gauntlet with accelerated Curse evolution.
	log.Printf("ceiling: running gauntlet — %d games, %d decks, evolve every %d games, %d workers",
		*games, len(gauntletDecks), *evolveEvery, *workers)

	result := runCeilingGauntlet(gauntletDecks, gauntletKeys, gauntletStrategies,
		cursePools, *games, *evolveEvery, *workers, *seed, corpus)

	// Phase 6: Determine ceiling deck.
	var bestIdx int
	var bestWR float64
	for i, r := range result {
		wr := float64(r.Wins) / float64(max(r.Games, 1))
		if wr > bestWR {
			bestWR = wr
			bestIdx = i
		}
	}

	// Print gauntlet results.
	fmt.Printf("\nGauntlet Results (%d games per deck)\n", *games)
	fmt.Println(strings.Repeat("=", 100))
	fmt.Printf("%-4s  %-45s  %-25s  %6s  %6s  %6s  %5s\n",
		"#", "Deck Key", "Commander", "Games", "Wins", "WR%", "Gen")
	fmt.Println(strings.Repeat("-", 100))
	for i, r := range result {
		wr := float64(r.Wins) / float64(max(r.Games, 1)) * 100
		marker := "  "
		if i == bestIdx {
			marker = " *"
		}
		fmt.Printf("%-4d  %-45s  %-25s  %6d  %6d  %5.1f%%  %5d%s\n",
			i+1, truncate(gauntletKeys[i], 45),
			truncate(gauntletDecks[i].CommanderName, 25),
			r.Games, r.Wins, wr, r.Generation, marker)
	}
	fmt.Printf("\n* = CEILING DECK\n\n")

	// Phase 7: Build and write ceiling.json.
	bestCandidate := gauntletCandidates[bestIdx]
	bestResult := result[bestIdx]

	ceiling := CeilingResult{
		DeckKey:         bestCandidate.DeckKey,
		Commander:       bestCandidate.Commander,
		Owner:           bestCandidate.Owner,
		Archetype:       bestCandidate.Archetype,
		Bracket:         5,
		ELO:             bestCandidate.ELO,
		HexRating:       bestCandidate.HexRating,
		WinRate:         float64(bestResult.Wins) / float64(max(bestResult.Games, 1)),
		GamesPlayed:     bestCandidate.ELOGames + bestResult.Games,
		GauntletWins:    bestResult.Wins,
		GauntletGames:   bestResult.Games,
		GauntletWinRate: float64(bestResult.Wins) / float64(max(bestResult.Games, 1)),
		Generation:      bestResult.Generation,
		BestDNA:         bestResult.BestDNA,
		PowerPercentile: bestCandidate.PowerPercentile,
		GameplanSummary: bestCandidate.GameplanSummary,
		CalibratedAt:    time.Now(),
	}

	// Add runner-ups.
	for i, r := range result {
		if i == bestIdx {
			continue
		}
		ceiling.RunnerUp = append(ceiling.RunnerUp, RunnerUpEntry{
			DeckKey:   gauntletKeys[i],
			Commander: gauntletDecks[i].CommanderName,
			WinRate:   float64(r.Wins) / float64(max(r.Games, 1)),
			ELO:       gauntletCandidates[i].ELO,
		})
	}

	// Write output.
	os.MkdirAll(filepath.Dir(*outPath), 0755)
	data, err := json.MarshalIndent(ceiling, "", "  ")
	if err != nil {
		log.Fatalf("ceiling: marshal: %v", err)
	}
	if err := os.WriteFile(*outPath, data, 0644); err != nil {
		log.Fatalf("ceiling: write %s: %v", *outPath, err)
	}
	log.Printf("ceiling: wrote %s", *outPath)
	fmt.Printf("Ceiling deck: %s (%s) — WR=%.1f%%, Gen=%d\n",
		ceiling.Commander, ceiling.DeckKey, ceiling.GauntletWinRate*100, ceiling.Generation)
}

// ---------------------------------------------------------------------------
// Phase 1: Load ELO records
// ---------------------------------------------------------------------------

func loadELORecords(dbPath string) map[string]*db.ELORecord {
	result := make(map[string]*db.ELORecord)
	if _, err := os.Stat(dbPath); err != nil {
		log.Printf("ceiling: no DB at %s, skipping ELO data", dbPath)
		return result
	}
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		log.Printf("ceiling: open DB: %v (skipping ELO)", err)
		return result
	}
	defer sqlDB.Close()

	records, err := db.LoadAllELO(context.Background(), sqlDB)
	if err != nil {
		log.Printf("ceiling: load ELO: %v", err)
		return result
	}
	for i := range records {
		r := records[i]
		result[r.DeckKey] = &r
	}
	log.Printf("ceiling: loaded %d ELO records", len(result))
	return result
}

// ---------------------------------------------------------------------------
// Phase 2: Load Curse pools
// ---------------------------------------------------------------------------

func loadCursePools(dir string, rng *rand.Rand) map[string]*hat.CursePool {
	pools, err := hat.LoadAllPools(dir, rng)
	if err != nil {
		log.Printf("ceiling: load curse pools: %v (starting fresh)", err)
		return make(map[string]*hat.CursePool)
	}
	log.Printf("ceiling: loaded %d curse pools", len(pools))
	return pools
}

// ---------------------------------------------------------------------------
// Phase 3: Scan B5 candidates
// ---------------------------------------------------------------------------

type strategyJSON struct {
	Archetype       string `json:"archetype"`
	Bracket         int    `json:"bracket"`
	GameplanSummary string `json:"gameplan_summary"`
	WinLines        []struct {
		Pieces []string `json:"pieces"`
		Type   string   `json:"type"`
	} `json:"win_lines"`
	TutorTargets    []string `json:"tutor_targets"`
	PowerPercentile int      `json:"power_percentile"`
}

func scanB5Candidates(decksDir string, eloMap map[string]*db.ELORecord, cursePools map[string]*hat.CursePool) []CeilingCandidate {
	var candidates []CeilingCandidate

	err := filepath.Walk(decksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".strategy.json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var s strategyJSON
		if err := json.Unmarshal(data, &s); err != nil {
			return nil
		}
		if s.Bracket != 5 {
			return nil
		}

		// Derive deck key from strategy path.
		// Strategy files are at: data/decks/{owner}/freya/{name}.strategy.json
		// Deck files are at: data/decks/{owner}/{name}.txt
		base := strings.TrimSuffix(filepath.Base(path), ".strategy.json")
		freyaDir := filepath.Dir(path)          // .../freya/
		ownerDir := filepath.Dir(freyaDir)      // .../{owner}/
		owner := filepath.Base(ownerDir)
		deckKey := owner + "/" + base
		deckPath := filepath.Join(ownerDir, base+".txt")

		// Verify deck file exists.
		if _, err := os.Stat(deckPath); err != nil {
			return nil
		}

		// Read commander from deck file.
		commander := readCommander(deckPath)

		c := CeilingCandidate{
			DeckKey:         deckKey,
			DeckPath:        deckPath,
			Commander:       commander,
			Owner:           owner,
			Archetype:       s.Archetype,
			Bracket:         5,
			PowerPercentile: s.PowerPercentile,
			GameplanSummary: s.GameplanSummary,
			ComboCount:      len(s.WinLines),
			TutorCount:      len(s.TutorTargets),
		}

		// Cross-reference ELO.
		if elo, ok := eloMap[deckKey]; ok {
			c.ELO = elo.Rating
			c.HexRating = elo.HexRating
			c.ELOGames = elo.Games
			c.ELOWins = elo.Wins
			if elo.Games > 0 {
				c.WinRate = float64(elo.Wins) / float64(elo.Games)
			}
		}

		// Cross-reference Curse pool.
		if pool, ok := cursePools[deckKey]; ok {
			c.CurseGen = pool.GenCount
			// Find best fitness in population.
			for i := range pool.Population {
				if pool.Population[i].Fitness > c.CurseFitness {
					c.CurseFitness = pool.Population[i].Fitness
					dna := pool.Population[i]
					c.BestDNA = &dna
				}
			}
		}

		// Compute combined score:
		// - ELO contribution (normalized to 0-50 range, 1500 base)
		// - Win rate contribution (0-30 range)
		// - Curse fitness contribution (0-15 range)
		// - Combo density bonus (0-5 range)
		eloScore := 0.0
		if c.ELO > 0 && c.ELOGames >= 20 {
			eloScore = math.Min((c.ELO-1400)/10, 50) // 1400 → 0, 1900 → 50
		}
		wrScore := 0.0
		if c.ELOGames >= 20 {
			wrScore = c.WinRate * 30
		}
		fitnessScore := c.CurseFitness * 15
		comboScore := math.Min(float64(c.ComboCount)*1.5, 5)

		c.Score = eloScore + wrScore + fitnessScore + comboScore

		candidates = append(candidates, c)
		return nil
	})

	if err != nil {
		log.Printf("ceiling: walk error: %v", err)
	}
	return candidates
}

// ---------------------------------------------------------------------------
// Phase 5: Ceiling Gauntlet (accelerated Curse evolution)
// ---------------------------------------------------------------------------

type gauntletDeckResult struct {
	Games      int
	Wins       int
	Generation int
	BestDNA    *hat.CurseDNA
}

func runCeilingGauntlet(
	decks []*deckparser.TournamentDeck,
	deckKeys []string,
	strategies []*hat.StrategyProfile,
	existingPools map[string]*hat.CursePool,
	totalGames, evolveEvery, numWorkers int,
	seed int64,
	corpus *astload.Corpus,
) []gauntletDeckResult {

	nDecks := len(decks)
	results := make([]gauntletDeckResult, nDecks)

	// Create isolated Curse pools for the gauntlet with accelerated evolution.
	pools := make([]*hat.CursePool, nDecks)
	rng := rand.New(rand.NewSource(seed))
	for i, key := range deckKeys {
		if existing, ok := existingPools[key]; ok {
			// Clone existing pool.
			poolCopy := *existing
			poolCopy.SetRNG(rand.New(rand.NewSource(rng.Int63())))
			pools[i] = &poolCopy
		} else {
			bracket := 5
			pool := hat.InitPoolWithBracket(key, bracket, rand.New(rand.NewSource(rng.Int63())))
			pools[i] = &pool
		}
	}

	// Track per-deck wins/games with atomic counters.
	type deckStats struct {
		games int64
		wins  int64
	}
	stats := make([]deckStats, nDecks)

	// Mutex for pool access (evolution is not thread-safe).
	var poolMu sync.Mutex

	var completed int64
	t0 := time.Now()

	// Worker pool.
	jobs := make(chan int, numWorkers*4)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localRng := rand.New(rand.NewSource(seed + int64(workerID)*7919))

			for range jobs {
				// Pick showmatchSeats random decks from the gauntlet pool.
				perm := localRng.Perm(nDecks)
				seatCount := showmatchSeats
				if seatCount > nDecks {
					seatCount = nDecks
				}
				seatDecks := make([]*deckparser.TournamentDeck, seatCount)
				seatKeys := make([]string, seatCount)
				seatIdxs := make([]int, seatCount)

				for s := 0; s < seatCount; s++ {
					idx := perm[s]
					seatDecks[s] = decks[idx]
					seatKeys[s] = deckKeys[idx]
					seatIdxs[s] = idx
				}

				// Select Curse DNA for each seat.
				poolMu.Lock()
				dnaIdxs := make([]int, seatCount)
				dnaCopies := make([]hat.CurseDNA, seatCount)
				for s := 0; s < seatCount; s++ {
					dna, dnaIdx := pools[seatIdxs[s]].SelectForGame()
					dnaIdxs[s] = dnaIdx
					dnaCopies[s] = *dna
				}
				poolMu.Unlock()

				// Set up game.
				gameSeed := localRng.Int63()
				gameRng := rand.New(rand.NewSource(gameSeed))
				gs := gameengine.NewGameState(seatCount, gameRng, corpus)
				gs.Seed = gameSeed
				gs.EventPolicy = gameengine.EventLogNone

				cmdDecks := make([]*gameengine.CommanderDeck, seatCount)
				for i := 0; i < seatCount; i++ {
					tpl := seatDecks[i]
					lib := deckparser.CloneLibrary(tpl.Library)
					cmdrs := deckparser.CloneCards(tpl.CommanderCards)
					for _, c := range cmdrs {
						c.Owner = i
					}
					for _, c := range lib {
						c.Owner = i
					}
					gameRng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
					cmdDecks[i] = &gameengine.CommanderDeck{
						CommanderCards: cmdrs,
						Library:        lib,
					}
				}
				gameengine.SetupCommanderGame(gs, cmdDecks)

				// Attach hats.
				for i := 0; i < seatCount; i++ {
					h := hat.NewYggdrasilHatWithPool(&dnaCopies[i], strategies[seatIdxs[i]], 50, nil)
					gs.Seats[i].Hat = h
				}

				// Mulligan.
				for i := 0; i < seatCount; i++ {
					tournament.RunLondonMulligan(gs, i)
				}

				gs.Active = gameRng.Intn(seatCount)
				gs.Turn = 1

				// Run game.
				for turn := 1; turn <= showmatchMaxTurn; turn++ {
					gs.Turn = turn
					tournament.TakeTurn(gs)
					gameengine.StateBasedActions(gs)
					if gs.CheckEnd() {
						break
					}
					gs.Active = nextLiving(gs, seatCount)
				}

				// Determine winner.
				winner := -1
				if gs.Flags != nil && gs.Flags["ended"] == 1 {
					if w, ok := gs.Flags["winner"]; ok && w >= 0 && w < seatCount {
						winner = w
					}
				}
				if winner < 0 {
					bestLife := -999
					for i, s := range gs.Seats {
						if s != nil && !s.Lost && s.Life > bestLife {
							bestLife = s.Life
							winner = i
						}
					}
				}

				// Record results.
				poolMu.Lock()
				for i := 0; i < seatCount; i++ {
					pool := pools[seatIdxs[i]]
					score := hat.PlacementScore(seatPlacement(gs, i, winner, seatCount), seatCount)

					// Accelerated evolution: use custom evolve threshold.
					pool.GameCount++
					dna := &pool.Population[dnaIdxs[i]]
					dna.GamesPlayed++
					if dna.GamesPlayed <= 1 {
						dna.Fitness = score
					} else {
						alpha := 2.0 / (float64(min(dna.GamesPlayed, 50)) + 1.0)
						dna.Fitness = dna.Fitness*(1-alpha) + score*alpha
					}
					if pool.GameCount >= evolveEvery {
						pool.GameCount = 0
						pool.SetRNG(rand.New(rand.NewSource(localRng.Int63())))
						// Manually trigger evolution by resetting counter and calling RecordResult
						// with a dummy that won't increment again. Instead, replicate evolve logic.
						forceEvolve(pool, localRng)
					}
				}
				poolMu.Unlock()

				// Update stats.
				for i := 0; i < seatCount; i++ {
					atomic.AddInt64(&stats[seatIdxs[i]].games, 1)
					if i == winner {
						atomic.AddInt64(&stats[seatIdxs[i]].wins, 1)
					}
				}

				done := atomic.AddInt64(&completed, 1)
				if done%1000 == 0 {
					elapsed := time.Since(t0)
					gps := float64(done) / elapsed.Seconds()
					log.Printf("ceiling gauntlet: %d/%d games (%.0f g/s)", done, totalGames, gps)
				}
			}
		}(w)
	}

	// Feed jobs.
	for i := 0; i < totalGames; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	// Collect results.
	for i := 0; i < nDecks; i++ {
		results[i].Games = int(atomic.LoadInt64(&stats[i].games))
		results[i].Wins = int(atomic.LoadInt64(&stats[i].wins))
		results[i].Generation = pools[i].GenCount

		// Find best DNA.
		var bestFitness float64
		for j := range pools[i].Population {
			if pools[i].Population[j].Fitness > bestFitness {
				bestFitness = pools[i].Population[j].Fitness
				dna := pools[i].Population[j]
				results[i].BestDNA = &dna
			}
		}
	}

	elapsed := time.Since(t0)
	log.Printf("ceiling gauntlet: complete — %d games in %s (%.0f g/s)",
		totalGames, elapsed.Round(time.Millisecond), float64(totalGames)/elapsed.Seconds())

	return results
}

// forceEvolve manually triggers one evolution step on a pool.
// This replicates CursePool.evolve() but is called externally for accelerated mode.
func forceEvolve(pool *hat.CursePool, rng *rand.Rand) {
	pool.SetRNG(rng)
	// We trigger evolution by setting GameCount to CurseEvolveAt and calling RecordResult
	// with score 0.5 on index 0. This is a hack but keeps us from exporting evolve().
	// Actually, we just set game_count high enough that the next RecordResult triggers it.
	// Better: just reset and let the pool track its own generation.
	// Since we already incremented GameCount to evolveEvery above and reset it,
	// let's just call RecordResult which will handle it. But we already reset GameCount=0.
	// So let's just bump GenCount and do selection manually.

	// Direct evolution: sort population by fitness, kill bottom 2, clone top 2.
	type indexed struct {
		idx     int
		fitness float64
	}
	pop := make([]indexed, hat.CursePopSize)
	for i := range pop {
		pop[i] = indexed{i, pool.Population[i].Fitness}
	}
	sort.Slice(pop, func(a, b int) bool {
		return pop[a].fitness < pop[b].fitness
	})

	killCount := 2
	for k := 0; k < killCount; k++ {
		loserIdx := pop[k].idx
		donorIdx := pop[hat.CursePopSize-1-k].idx

		clone := pool.Population[donorIdx]
		clone.GamesPlayed = 0
		clone.Generation++
		clone.Aggression = clampUnit(clone.Aggression + rng.NormFloat64()*0.05)
		clone.ComboPat = clampUnit(clone.ComboPat + rng.NormFloat64()*0.05)
		clone.ThreatParanoia = clampUnit(clone.ThreatParanoia + rng.NormFloat64()*0.05)
		clone.ResourceGreed = clampUnit(clone.ResourceGreed + rng.NormFloat64()*0.05)
		clone.PoliticalMemory = clampUnit(clone.PoliticalMemory + rng.NormFloat64()*0.05)
		clone.DrainAffinity = clampUnit(clone.DrainAffinity + rng.NormFloat64()*0.05)
		clone.ArtifactAffinity = clampUnit(clone.ArtifactAffinity + rng.NormFloat64()*0.05)

		pool.Population[loserIdx] = clone
	}

	pool.GenCount++
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func nextLiving(gs *gameengine.GameState, nSeats int) int {
	cand := (gs.Active + 1) % nSeats
	for tries := 0; tries < nSeats; tries++ {
		s := gs.Seats[cand]
		if s != nil && !s.Lost {
			return cand
		}
		cand = (cand + 1) % nSeats
	}
	return gs.Active
}

func seatPlacement(gs *gameengine.GameState, seat, winner, nSeats int) int {
	if seat == winner {
		return 1
	}
	// Count how many players are still alive (not counting winner).
	alive := 0
	for i, s := range gs.Seats {
		if i == winner {
			continue
		}
		if s != nil && !s.Lost {
			alive++
		}
	}
	if gs.Seats[seat] != nil && !gs.Seats[seat].Lost {
		return 2 // survived but didn't win
	}
	return nSeats // eliminated
}

func readCommander(deckPath string) string {
	f, err := os.Open(deckPath)
	if err != nil {
		return "(unknown)"
	}
	defer f.Close()

	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	lines := strings.Split(string(buf[:n]), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "COMMANDER:") {
			return strings.TrimSpace(line[len("COMMANDER:"):])
		}
	}
	return "(unknown)"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
