package tournament

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	rpprof "runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/hat"
	"github.com/hexdek/hexdek/internal/huginn"
	"github.com/hexdek/hexdek/internal/muninn"
)

// startingHand mirrors playloop.STARTING_HAND.
const startingHand = 7

// Run executes the tournament described by cfg and returns the aggregate
// result. Run is the only public entry point for the package.
func Run(cfg TournamentConfig) (*TournamentResult, error) {
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	maxTurns := cfg.MaxTurnsPerGame
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}

	gameTimeout := cfg.GameTimeout
	if gameTimeout <= 0 {
		gameTimeout = defaultPerGameTimeout
	}

	// Lazy pool mode: load decks on demand to stay under memory ceiling.
	if cfg.LazyPool {
		return runLazyPool(cfg, workers, maxTurns, gameTimeout)
	}

	// Pool mode: each game samples NSeats random decks from the full pool.
	if cfg.PoolMode {
		return runPool(cfg, workers, maxTurns, gameTimeout)
	}

	// Deck list — we use the first NSeats decks and rotate.
	decks := cfg.Decks[:cfg.NSeats]
	commanderNames := make([]string, cfg.NSeats)
	for i, d := range decks {
		commanderNames[i] = d.CommanderName
	}

	// Per-seat hat factories. Normalize to cfg.NSeats entries.
	hats := make([]HatFactory, cfg.NSeats)
	switch len(cfg.HatFactories) {
	case 0:
		for i := range hats {
			hats[i] = defaultHatFactory
		}
	case 1:
		for i := range hats {
			hats[i] = cfg.HatFactories[0]
		}
	default:
		if len(cfg.HatFactories) < cfg.NSeats {
			return nil, fmt.Errorf("tournament: HatFactories must be 0, 1, or NSeats entries")
		}
		copy(hats, cfg.HatFactories[:cfg.NSeats])
	}

	// Progress settings.
	progressEvery := cfg.ProgressLogEvery
	if progressEvery == 0 {
		progressEvery = 1000
		if cfg.NGames/20 > progressEvery {
			progressEvery = cfg.NGames / 20
		}
	}

	seeds := make(chan int, workers*2)
	bufferSize := workers * defaultBufferMult
	if bufferSize < 64 {
		bufferSize = 64
	}
	outcomes := make(chan GameOutcome, bufferSize)

	var completed int64
	start := time.Now()

	// Worker pool.
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for gameIdx := range seeds {
				outcome := runOneGameSafe(gameIdx, decks, hats, cfg.NSeats, cfg.Seed, maxTurns, gameTimeout, cfg.CommanderMode, cfg.AuditEnabled, cfg.AnalyticsEnabled)
				outcomes <- outcome
				done := atomic.AddInt64(&completed, 1)
				if progressEvery > 0 && done%int64(progressEvery) == 0 {
					gps := float64(done) / time.Since(start).Seconds()
					if cfg.ProgressLogger != nil {
						cfg.ProgressLogger(int(done), cfg.NGames, gps)
					} else {
						fmt.Fprintf(os.Stderr, "  tournament: %d/%d games (%.0f g/s)\n", done, cfg.NGames, gps)
					}
				}
			}
		}(w)
	}

	// Seed producer.
	go func() {
		for i := 0; i < cfg.NGames; i++ {
			seeds <- i
		}
		close(seeds)
	}()

	// Closer for outcomes.
	go func() {
		wg.Wait()
		close(outcomes)
	}()

	// Aggregator.
	result := aggregate(outcomes, cfg.NGames, cfg.NSeats, commanderNames)
	result.Duration = time.Since(start)
	if result.Duration.Seconds() > 0 {
		result.GamesPerSecond = float64(result.Games) / result.Duration.Seconds()
	}

	if cfg.ReportPath != "" {
		if err := result.WriteMarkdown(cfg.ReportPath); err != nil {
			return result, fmt.Errorf("tournament: write report: %w", err)
		}
	}

	// Persist Muninn memory regardless of --report flag.
	persistMuninn(result)

	return result, nil
}

func validate(cfg *TournamentConfig) error {
	if cfg == nil {
		return fmt.Errorf("tournament: nil config")
	}
	if cfg.NGames <= 0 {
		return fmt.Errorf("tournament: NGames must be > 0 (got %d)", cfg.NGames)
	}
	if cfg.NSeats < 2 {
		return fmt.Errorf("tournament: NSeats must be >= 2 (got %d)", cfg.NSeats)
	}
	if cfg.LazyPool {
		if len(cfg.DeckPaths) < cfg.NSeats {
			return fmt.Errorf("tournament: lazy-pool needs at least %d deck paths, got %d", cfg.NSeats, len(cfg.DeckPaths))
		}
		if cfg.Corpus == nil {
			return fmt.Errorf("tournament: lazy-pool requires Corpus")
		}
		if cfg.Meta == nil {
			return fmt.Errorf("tournament: lazy-pool requires Meta")
		}
		return nil
	}
	if len(cfg.Decks) < cfg.NSeats {
		return fmt.Errorf("tournament: need at least %d decks, got %d", cfg.NSeats, len(cfg.Decks))
	}
	for i, d := range cfg.Decks[:cfg.NSeats] {
		if d == nil {
			return fmt.Errorf("tournament: decks[%d] is nil", i)
		}
		if len(d.CommanderCards) == 0 {
			return fmt.Errorf("tournament: decks[%d] has no commander", i)
		}
		for j, c := range d.CommanderCards {
			if c == nil {
				return fmt.Errorf("tournament: decks[%d] commander[%d] is nil", i, j)
			}
		}
	}
	return nil
}

func defaultHatFactory() gameengine.Hat { return &hat.GreedyHat{} }

// perGameTimeout is the maximum wall-clock time a single game is allowed
// to run before being killed as a timeout. Prevents pathological cEDH
// games from blocking the entire tournament.
const defaultPerGameTimeout = 3 * time.Minute

// runOneGameSafe wraps runOneGame in a recover() so panics register as
// crashes instead of killing the worker goroutine. Also enforces a
// per-game wall-clock timeout.
// gameProgress is atomically updated by the game goroutine so the timeout
// path can report turn/board state even when the goroutine is abandoned.
type gameProgress struct {
	turn       int64
	boardTotal int64
	boardMax   int64
}

func runOneGameSafe(gameIdx int, decks []*deckparser.TournamentDeck, hats []HatFactory,
	nSeats int, masterSeed int64, maxTurns int, gameTimeout time.Duration, commanderMode, auditEnabled, analyticsEnabled bool) (outcome GameOutcome) {
	outcome.GameIdx = gameIdx
	outcome.Winner = -1
	outcome.WinnerCommanderIdx = -1
	outcome.Rot = gameIdx % nSeats

	var prog gameProgress
	ch := make(chan GameOutcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				ch <- GameOutcome{
					GameIdx:            gameIdx,
					Winner:             -1,
					WinnerCommanderIdx: -1,
					Rot:                gameIdx % nSeats,
					CrashErr:           fmt.Sprintf("panic: %v\n%s", r, buf[:n]),
					EndReason:          "crash",
				}
			}
		}()
		ch <- runOneGame(gameIdx, decks, hats, nSeats, masterSeed, maxTurns, commanderMode, auditEnabled, analyticsEnabled, &prog)
	}()

	select {
	case outcome = <-ch:
		return outcome
	case <-time.After(gameTimeout):
		outcome.EndReason = "timeout"
		outcome.Turns = int(atomic.LoadInt64(&prog.turn))
		outcome.TotalBoardSize = int(atomic.LoadInt64(&prog.boardTotal))
		outcome.MaxBoardSize = int(atomic.LoadInt64(&prog.boardMax))
		outcome.CrashErr = fmt.Sprintf("game exceeded %s wall-clock limit", gameTimeout)
		return outcome
	}
}

// runOneGame simulates a single game. Returns the populated GameOutcome.
//
// Semantic reference: scripts/gauntlet_poker.py _run_one_game_with_policy.
func runOneGame(gameIdx int, decks []*deckparser.TournamentDeck, hats []HatFactory,
	nSeats int, masterSeed int64, maxTurns int, commanderMode, auditEnabled, analyticsEnabled bool, prog *gameProgress) GameOutcome {
	out := GameOutcome{
		GameIdx:            gameIdx,
		Rot:                gameIdx % nSeats,
		Winner:             -1,
		WinnerCommanderIdx: -1,
		EliminationOrder:   make([]int, nSeats),
	}
	for i := range out.EliminationOrder {
		out.EliminationOrder[i] = -1
	}
	// Record which commander indices participated in this game.
	out.ParticipantCommanderIdxs = make([]int, nSeats)
	for i := 0; i < nSeats; i++ {
		out.ParticipantCommanderIdxs[i] = (i + out.Rot) % nSeats
	}

	// Per-game deterministic RNG. Mirrors Python seed pattern:
	// master_rng.randint(0, 2**31) on each iteration is equivalent to
	// taking deterministic offsets; we use masterSeed + gameIdx*1000+1
	// which is the seed contract in the Phase 11 spec.
	gameSeed := masterSeed + int64(gameIdx)*1000 + 1
	rng := rand.New(rand.NewSource(gameSeed))

	gs := gameengine.NewGameState(nSeats, rng, nil)
	if !auditEnabled {
		gs.RetainEvents = false
	}

	// Rotate deck assignment: seat i gets decks[(i+rot) % nSeats].
	rot := out.Rot
	commanderDecks := make([]*gameengine.CommanderDeck, nSeats)
	originalIdxForSeat := make([]int, nSeats)
	for i := 0; i < nSeats; i++ {
		orig := (i + rot) % nSeats
		originalIdxForSeat[i] = orig
		tpl := decks[orig]
		// Deep-copy library + commander so concurrent games don't share state.
		lib := deckparser.CloneLibrary(tpl.Library)
		cmdrs := deckparser.CloneCards(tpl.CommanderCards)
		for _, c := range cmdrs {
			c.Owner = i
		}
		for _, c := range lib {
			c.Owner = i
		}
		// Shuffle library with per-game RNG.
		rng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
		commanderDecks[i] = &gameengine.CommanderDeck{
			CommanderCards: cmdrs,
			Library:        lib,
		}
	}

	if commanderMode {
		gameengine.SetupCommanderGame(gs, commanderDecks)
	} else {
		// Vanilla 20-life — just load libraries.
		for i, cd := range commanderDecks {
			gs.Seats[i].Library = append(gs.Seats[i].Library[:0], cd.Library...)
			gs.Seats[i].Life = 20
			gs.Seats[i].StartingLife = 20
		}
	}

	// Attach hats — follow deck rotation so the hat's strategy profile
	// matches the deck each seat is playing this game, not the physical seat.
	for i := 0; i < nSeats; i++ {
		hatIdx := (i + rot) % nSeats
		gs.Seats[i].Hat = hats[hatIdx]()
	}

	// Opening hands + London mulligan.
	for i := 0; i < nSeats; i++ {
		RunLondonMulligan(gs, i)
	}

	// Random starting active seat (mirrors gauntlet_poker).
	gs.Active = rng.Intn(nSeats)
	gs.Turn = 1
	gs.LogEvent(gameengine.Event{
		Kind: "game_start", Seat: gs.Active, Target: -1,
		Details: map[string]interface{}{
			"on_the_play":       gs.Active,
			"n_seats":           nSeats,
			"commander_format":  commanderMode,
			"game_idx":          gameIdx,
		},
	})

	// Track elimination order.
	elimSlot := 0
	markElim := func() {
		for i, s := range gs.Seats {
			if s != nil && s.Lost && out.EliminationOrder[originalIdxForSeat[i]] < 0 {
				out.EliminationOrder[originalIdxForSeat[i]] = elimSlot
				elimSlot++
			}
		}
	}
	markElim()

	// Turn loop. Track round number (full rotation through all seats).
	startingSeat := gs.Active
	round := 1
	if gs.Flags == nil {
		gs.Flags = make(map[string]int)
	}
	gs.Flags["round"] = round
	ended := false
	for turn := 1; turn <= maxTurns && !ended; turn++ {
		gs.Turn = turn
		gs.Flags["round"] = round
		// Update shared progress for timeout diagnosis.
		if prog != nil {
			atomic.StoreInt64(&prog.turn, int64(turn))
			var bt, bm int64
			for _, s := range gs.Seats {
				if s != nil {
					n := int64(len(s.Battlefield))
					bt += n
					if n > bm {
						bm = n
					}
				}
			}
			atomic.StoreInt64(&prog.boardTotal, bt)
			atomic.StoreInt64(&prog.boardMax, bm)
		}
		takeTurnImpl(gs, nil)
		gameengine.StateBasedActions(gs)
		markElim()
		if gs.CheckEnd() {
			ended = true
			break
		}
		prev := gs.Active
		gs.Active = nextLivingSeat(gs)
		// Round increments when we wrap past the starting seat.
		if gs.Active <= prev || gs.Active == startingSeat {
			round++
		}
	}

	out.Turns = gs.Turn
	// Capture final board density (for non-timeout games; timeout games
	// read from the shared gameProgress struct instead).
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if len(s.Battlefield) > out.MaxBoardSize {
			out.MaxBoardSize = len(s.Battlefield)
		}
		out.TotalBoardSize += len(s.Battlefield)
	}

	// Determine winner.
	if gs.Flags != nil {
		if _, ok := gs.Flags["winner"]; ok && gs.Flags["ended"] == 1 {
			w := gs.Flags["winner"]
			if w >= 0 && w < nSeats {
				out.Winner = w
				out.WinnerCommanderIdx = originalIdxForSeat[w]
				out.EndReason = "last_seat_standing"
			}
		} else if gs.Flags["ended"] == 1 {
			out.EndReason = "draw"
		}
	}
	if !ended {
		// Turn-cap tiebreak: highest-life living seat wins.
		living := []int{}
		for i, s := range gs.Seats {
			if s != nil && !s.Lost {
				living = append(living, i)
			}
		}
		if len(living) == 0 {
			out.EndReason = "turn_cap_all_dead"
		} else {
			topLife := gs.Seats[living[0]].Life
			for _, i := range living[1:] {
				if gs.Seats[i].Life > topLife {
					topLife = gs.Seats[i].Life
				}
			}
			leaders := []int{}
			for _, i := range living {
				if gs.Seats[i].Life == topLife {
					leaders = append(leaders, i)
				}
			}
			if len(leaders) == 1 {
				out.Winner = leaders[0]
				out.WinnerCommanderIdx = originalIdxForSeat[leaders[0]]
				out.EndReason = "turn_cap_leader"
			} else {
				out.EndReason = "turn_cap_tie"
			}
		}
	}
	// Fill any still-alive seats into the last elimination slot so the
	// winner ends up at NSeats-1.
	for i, s := range gs.Seats {
		if s != nil && out.EliminationOrder[originalIdxForSeat[i]] < 0 {
			out.EliminationOrder[originalIdxForSeat[i]] = elimSlot
			elimSlot++
		}
	}

	// Count concessions from seat state (works even with RetainEvents=false).
	for _, s := range gs.Seats {
		if s != nil && s.LossReason == "concession" {
			out.Concessions++
		}
	}

	// Track min relative position for conviction calibration.
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		if yh, ok := s.Hat.(*hat.YggdrasilHat); ok {
			if yh.MinRelPos < out.MinRelPos {
				out.MinRelPos = yh.MinRelPos
			}
		}
	}

	// Count mode-change events + parser-gap snippets.
	for _, ev := range gs.EventLog {
		if ev.Kind == "player_mode_change" {
			out.ModeChanges++
		}
		if auditEnabled {
			if ev.Kind == "parser_gap" {
				snippet := ""
				if ev.Details != nil {
					if s, ok := ev.Details["snippet"].(string); ok {
						snippet = s
					}
				}
				if snippet != "" {
					if out.ParserGapSnippets == nil {
						out.ParserGapSnippets = map[string]int{}
					}
					out.ParserGapSnippets[snippet]++
				}
			}
		}
	}

	// Deep analytics: process the event log while we still have it.
	if analyticsEnabled {
		cmdrNames := make([]string, nSeats)
		handsAtEnd := make([]int, nSeats)
		finalLife := make([]int, nSeats)
		for i := 0; i < nSeats; i++ {
			orig := originalIdxForSeat[i]
			if orig < len(decks) {
				cmdrNames[i] = decks[orig].CommanderName
			}
			if s := gs.Seats[i]; s != nil {
				handsAtEnd[i] = len(s.Hand)
				finalLife[i] = s.Life
			}
		}
		out.Analysis = analytics.AnalyzeGame(
			gs.EventLog,
			nSeats,
			cmdrNames,
			out.Winner,
			out.Turns,
			handsAtEnd,
			finalLife,
		)

		// Check for missed combos — scans end-of-game board state for known
		// combo pieces that were live but not executed (Hat intelligence gaps).
		// Also checks Freya strategy combos + finishers when available.
		var strategyCombos [][]string
		finisherSets := map[int]map[string]bool{}
		for i, s := range gs.Seats {
			if s == nil {
				continue
			}
			if yh, ok := s.Hat.(*hat.YggdrasilHat); ok && yh.Strategy != nil {
				for _, cp := range yh.Strategy.ComboPieces {
					if len(cp.Pieces) >= 2 {
						strategyCombos = append(strategyCombos, cp.Pieces)
					}
				}
				if len(yh.Strategy.FinisherCards) > 0 {
					fset := make(map[string]bool, len(yh.Strategy.FinisherCards))
					for _, f := range yh.Strategy.FinisherCards {
						fset[f] = true
					}
					finisherSets[i] = fset
				}
			}
		}
		out.Analysis.MissedCombos = analytics.DetectMissedCombosWithStrategy(gs, strategyCombos)
		out.Analysis.MissedFinishers = analytics.DetectMissedFinishers(gs, finisherSets)
	}

	return out
}

// runPool executes a tournament where each game samples NSeats random
// decks from the full pool. Tracks per-commander stats across all games.
func runPool(cfg TournamentConfig, workers, maxTurns int, gameTimeout time.Duration) (*TournamentResult, error) {
	allDecks := cfg.Decks
	nSeats := cfg.NSeats

	// Build a uniform hat factory for pool mode.
	var uniformHat HatFactory
	switch len(cfg.HatFactories) {
	case 0:
		uniformHat = defaultHatFactory
	default:
		uniformHat = cfg.HatFactories[0]
	}

	// Build commander name index for all decks.
	allNames := make([]string, len(allDecks))
	for i, d := range allDecks {
		allNames[i] = d.CommanderName
	}

	progressEvery := cfg.ProgressLogEvery
	if progressEvery == 0 {
		progressEvery = 100
		if cfg.NGames/20 > progressEvery {
			progressEvery = cfg.NGames / 20
		}
	}

	type poolJob struct {
		gameIdx  int
		deckIdxs []int
	}

	jobs := make(chan poolJob, workers*2)
	bufferSize := workers * defaultBufferMult
	if bufferSize < 64 {
		bufferSize = 64
	}

	type poolOutcome struct {
		GameOutcome
		deckIdxs []int
	}
	outcomes := make(chan poolOutcome, bufferSize)

	var completed int64
	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				podDecks := make([]*deckparser.TournamentDeck, nSeats)
				podHats := make([]HatFactory, nSeats)
				for i, idx := range job.deckIdxs {
					podDecks[i] = allDecks[idx]
					podHats[i] = uniformHat
				}
				o := runOneGameSafe(job.gameIdx, podDecks, podHats, nSeats,
					cfg.Seed, maxTurns, gameTimeout, cfg.CommanderMode, cfg.AuditEnabled, cfg.AnalyticsEnabled)
				outcomes <- poolOutcome{o, job.deckIdxs}
				done := atomic.AddInt64(&completed, 1)
				if progressEvery > 0 && done%int64(progressEvery) == 0 {
					gps := float64(done) / time.Since(start).Seconds()
					fmt.Fprintf(os.Stderr, "  pool: %d/%d games (%.0f g/s)\n", done, cfg.NGames, gps)
				}
			}
		}()
	}

	// Job producer: for each game, pick NSeats random deck indices.
	go func() {
		rng := rand.New(rand.NewSource(cfg.Seed))
		nDecks := len(allDecks)
		for i := 0; i < cfg.NGames; i++ {
			idxs := make([]int, nSeats)
			perm := rng.Perm(nDecks)
			copy(idxs, perm[:nSeats])
			jobs <- poolJob{gameIdx: i, deckIdxs: idxs}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	// Aggregate per-commander stats across all pool games.
	type cmdStats struct {
		wins, games int
	}
	stats := make(map[string]*cmdStats)
	totalGames := 0
	crashes := 0
	totalConcessions := 0
	var crashLogs []string
	totalTurns := 0

	for o := range outcomes {
		totalGames++
		totalTurns += o.Turns
		totalConcessions += o.Concessions
		if o.CrashErr != "" {
			crashes++
			crashLogs = append(crashLogs, o.CrashErr)
		}
		for _, idx := range o.deckIdxs {
			name := allNames[idx]
			s := stats[name]
			if s == nil {
				s = &cmdStats{}
				stats[name] = s
			}
			s.games++
		}
		if o.Winner >= 0 && o.Winner < len(o.deckIdxs) {
			winIdx := o.deckIdxs[o.Winner]
			name := allNames[winIdx]
			stats[name].wins++
		}
	}

	elapsed := time.Since(start)

	// Build a result compatible with PrintDashboard.
	// We stuff pool stats into the standard result fields.
	uniqueNames := make([]string, 0, len(stats))
	for name := range stats {
		uniqueNames = append(uniqueNames, name)
	}

	// Print pool-specific dashboard.
	fmt.Printf("\n=== POOL TOURNAMENT RESULTS (%d games, %d unique commanders) ===\n\n", totalGames, len(uniqueNames))
	fmt.Printf("Duration: %s  |  Throughput: %.1f g/s  |  Crashes: %d  |  Concessions: %d  |  Avg turns: %.1f\n\n",
		elapsed.Round(time.Millisecond),
		float64(totalGames)/elapsed.Seconds(),
		crashes,
		totalConcessions,
		float64(totalTurns)/float64(max1pool(totalGames)))

	if crashes > 0 {
		fmt.Printf("CRASH RATE: %.2f%% (%d/%d)\n\n", 100*float64(crashes)/float64(totalGames), crashes, totalGames)
		for i, cl := range crashLogs {
			if i >= 20 {
				fmt.Printf("  ... and %d more\n", len(crashLogs)-20)
				break
			}
			// Truncate long crash logs.
			if len(cl) > 500 {
				cl = cl[:500] + "..."
			}
			fmt.Printf("CRASH %d:\n%s\n\n", i+1, cl)
		}
	}

	// Sort by games played desc for coverage report.
	type entry struct {
		name       string
		wins, games int
	}
	entries := make([]entry, 0, len(stats))
	for name, s := range stats {
		entries = append(entries, entry{name, s.wins, s.games})
	}
	// Sort by win rate desc.
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			ri := float64(entries[i].wins) / float64(max1pool(entries[i].games))
			rj := float64(entries[j].wins) / float64(max1pool(entries[j].games))
			if rj > ri {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	fmt.Printf("TOP 30 COMMANDERS (by winrate):\n")
	limit := 30
	if limit > len(entries) {
		limit = len(entries)
	}
	for i := 0; i < limit; i++ {
		e := entries[i]
		fmt.Printf("  %3d. %-40s %5.1f%%  (%d/%d games)\n",
			i+1, e.name, 100*float64(e.wins)/float64(max1pool(e.games)), e.wins, e.games)
	}

	noCoverage := 0
	for _, e := range entries {
		if e.games == 0 {
			noCoverage++
		}
	}
	fmt.Printf("\nCoverage: %d/%d commanders appeared in at least 1 game\n", len(entries)-noCoverage, len(entries))

	result := &TournamentResult{
		Games:     totalGames,
		Duration:  elapsed,
		CrashLogs: crashLogs,
	}
	if elapsed.Seconds() > 0 {
		result.GamesPerSecond = float64(totalGames) / elapsed.Seconds()
	}

	// Persist Muninn memory for pool mode.
	persistMuninn(result)

	return result, nil
}

func max1pool(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

// runLazyPool is like runPool but loads decks on demand instead of
// holding all templates in memory. Only NSeats × Workers decks are
// resident at any time, enabling 1000+ deck pools on memory-constrained
// machines.
func runLazyPool(cfg TournamentConfig, workers, maxTurns int, gameTimeout time.Duration) (*TournamentResult, error) {
	paths := cfg.DeckPaths
	nDecks := len(paths)
	nSeats := cfg.NSeats
	corpus := cfg.Corpus
	meta := cfg.Meta

	if corpus == nil || meta == nil {
		return nil, fmt.Errorf("lazy pool requires Corpus and Meta")
	}

	var uniformHat HatFactory
	if len(cfg.HatFactories) > 0 {
		uniformHat = cfg.HatFactories[0]
	} else {
		uniformHat = defaultHatFactory
	}

	// Pre-scan commander names from deck files (cheap: read first line).
	cmdrNames := make([]string, nDecks)
	for i, p := range paths {
		cmdrNames[i] = scanCommanderName(p)
	}

	progressEvery := cfg.ProgressLogEvery
	if progressEvery == 0 {
		progressEvery = 100
		if cfg.NGames/20 > progressEvery {
			progressEvery = cfg.NGames / 20
		}
	}

	type lazyJob struct {
		gameIdx  int
		deckIdxs []int
	}
	type lazyOutcome struct {
		GameOutcome
		deckIdxs []int
	}

	jobs := make(chan lazyJob, workers*2)
	bufSize := workers * defaultBufferMult
	if bufSize < 64 {
		bufSize = 64
	}
	outcomes := make(chan lazyOutcome, bufSize)

	var completed int64
	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				podDecks := make([]*deckparser.TournamentDeck, nSeats)
				podHats := make([]HatFactory, nSeats)
				parseOK := true
				for i, idx := range job.deckIdxs {
					d, err := deckparser.ParseDeckFile(paths[idx], corpus, meta)
					if err != nil {
						parseOK = false
						break
					}
					podDecks[i] = d
					podHats[i] = uniformHat
				}
				var o GameOutcome
				if parseOK {
					o = runOneGameSafe(job.gameIdx, podDecks, podHats, nSeats,
						cfg.Seed, maxTurns, gameTimeout, cfg.CommanderMode, cfg.AuditEnabled, cfg.AnalyticsEnabled)
				} else {
					o = GameOutcome{
						GameIdx:   job.gameIdx,
						Winner:    -1,
						EndReason: "parse_error",
					}
				}
				// Annotate timeout crashes with commander names for diagnosis.
				if o.EndReason == "timeout" && parseOK {
					var names []string
					for _, d := range podDecks {
						if d != nil {
							names = append(names, d.CommanderName)
						}
					}
					o.CrashErr += fmt.Sprintf(" | turn: %d | board: %d total (%d max) | pod: %v",
						o.Turns, o.TotalBoardSize, o.MaxBoardSize, names)
				}
				outcomes <- lazyOutcome{o, job.deckIdxs}
				// Release deck references so GC can reclaim before next game.
				for i := range podDecks {
					podDecks[i] = nil
				}
				done := atomic.AddInt64(&completed, 1)
				if done%4 == 0 {
					runtime.GC()
				}
				if progressEvery > 0 && done%int64(progressEvery) == 0 {
					gps := float64(done) / time.Since(start).Seconds()
					fmt.Fprintf(os.Stderr, "  lazy-pool: %d/%d games (%.0f g/s)\n", done, cfg.NGames, gps)
				}
			}
		}()
	}

	go func() {
		rng := rand.New(rand.NewSource(cfg.Seed))
		for i := 0; i < cfg.NGames; i++ {
			idxs := make([]int, nSeats)
			perm := rng.Perm(nDecks)
			for s := 0; s < nSeats; s++ {
				idxs[s] = perm[s]
			}
			jobs <- lazyJob{i, idxs}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	type cmdrStat struct {
		wins, games int
	}
	stats := make(map[string]*cmdrStat)
	for _, name := range cmdrNames {
		if _, ok := stats[name]; !ok {
			stats[name] = &cmdrStat{}
		}
	}

	totalGames := 0
	var crashLogs []string
	for out := range outcomes {
		totalGames++
		for _, idx := range out.deckIdxs {
			name := cmdrNames[idx]
			stats[name].games++
		}
		if out.Winner >= 0 && out.Winner < len(out.deckIdxs) {
			winIdx := out.deckIdxs[out.Winner]
			stats[cmdrNames[winIdx]].wins++
		}
		if out.CrashErr != "" {
			crashLogs = append(crashLogs, out.CrashErr)
		}
		// After the first game completes, dump a heap profile for leak diagnosis.
		if totalGames == 1 && cfg.PprofEnabled {
			runtime.GC()
			if f, err := os.Create("/tmp/mtgsquad_heap_post1.prof"); err == nil {
				rpprof.WriteHeapProfile(f)
				f.Close()
				fmt.Fprintf(os.Stderr, "  heap profile written to /tmp/mtgsquad_heap_post1.prof\n")
			}
		}
	}

	elapsed := time.Since(start)

	fmt.Printf("\n=== LAZY POOL TOURNAMENT RESULTS (%d games, %d unique commanders) ===\n\n",
		totalGames, len(stats))
	gps := float64(0)
	if elapsed.Seconds() > 0 {
		gps = float64(totalGames) / elapsed.Seconds()
	}
	fmt.Printf("Duration: %s  |  Throughput: %.1f g/s  |  Crashes: %d\n\n",
		elapsed.Round(time.Millisecond), gps, len(crashLogs))
	fmt.Printf("CRASH RATE: %.2f%% (%d/%d)\n\n", 100*float64(len(crashLogs))/float64(max1pool(totalGames)),
		len(crashLogs), totalGames)
	for i, cl := range crashLogs {
		fmt.Printf("CRASH %d:\n%s\n\n", i+1, cl)
	}

	type entry struct {
		name        string
		wins, games int
	}
	entries := make([]entry, 0, len(stats))
	for name, s := range stats {
		entries = append(entries, entry{name, s.wins, s.games})
	}
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			ri := float64(entries[i].wins) / float64(max1pool(entries[i].games))
			rj := float64(entries[j].wins) / float64(max1pool(entries[j].games))
			if rj > ri {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	fmt.Printf("TOP 30 COMMANDERS (by winrate):\n")
	limit := 30
	if limit > len(entries) {
		limit = len(entries)
	}
	for i := 0; i < limit; i++ {
		e := entries[i]
		fmt.Printf("  %3d. %-40s %5.1f%%  (%d/%d games)\n",
			i+1, e.name, 100*float64(e.wins)/float64(max1pool(e.games)), e.wins, e.games)
	}

	noCov := 0
	for _, e := range entries {
		if e.games == 0 {
			noCov++
		}
	}
	fmt.Printf("\nCoverage: %d/%d commanders appeared in at least 1 game\n",
		len(entries)-noCov, len(entries))

	result := &TournamentResult{
		Games:     totalGames,
		Duration:  elapsed,
		CrashLogs: crashLogs,
	}
	if elapsed.Seconds() > 0 {
		result.GamesPerSecond = float64(totalGames) / elapsed.Seconds()
	}

	// Persist Muninn memory for lazy-pool mode.
	persistMuninn(result)

	return result, nil
}

// scanCommanderName reads the COMMANDER: line from a deck file without
// full parsing. Returns "unknown" if not found.
func scanCommanderName(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "unknown"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	for _, line := range strings.SplitN(string(buf[:n]), "\n", 5) {
		if strings.HasPrefix(line, "COMMANDER: ") {
			return strings.TrimPrefix(line, "COMMANDER: ")
		}
	}
	return "unknown"
}

// persistMuninn writes parser gaps, crash logs, and dead triggers to
// the data/muninn directory. Also persists Huginn raw observations.
// Errors are logged to stderr but do not fail the tournament run.
func persistMuninn(result *TournamentResult) {
	const muninnDir = "data/muninn"
	const huginnDir = "data/huginn"

	if len(result.ParserGapSnippets) > 0 {
		if err := muninn.PersistParserGaps(muninnDir, result.ParserGapSnippets); err != nil {
			fmt.Fprintf(os.Stderr, "muninn: persist parser gaps: %v\n", err)
		}
	}
	if len(result.CrashLogs) > 0 {
		if err := muninn.PersistCrashLogs(muninnDir, result.CrashLogs, result.CommanderNames, result.Games, result.NSeats); err != nil {
			fmt.Fprintf(os.Stderr, "muninn: persist crash logs: %v\n", err)
		}
	}
	if len(result.Analyses) > 0 {
		if err := muninn.PersistDeadTriggers(muninnDir, result.Analyses); err != nil {
			fmt.Fprintf(os.Stderr, "muninn: persist dead triggers: %v\n", err)
		}
		if err := huginn.PersistRawObservations(huginnDir, result.Analyses, result.CommanderNames); err != nil {
			fmt.Fprintf(os.Stderr, "huginn: persist raw observations: %v\n", err)
		}
	}
}

// nextLivingSeat returns the next seat index clockwise that is still in
// the game. Falls back to the current Active if everyone's dead (caller
// should have already detected end-of-game in that case).
func nextLivingSeat(gs *gameengine.GameState) int {
	n := len(gs.Seats)
	if n == 0 {
		return 0
	}
	for k := 1; k <= n; k++ {
		cand := (gs.Active + k) % n
		s := gs.Seats[cand]
		if s != nil && !s.Lost {
			return cand
		}
	}
	return gs.Active
}
