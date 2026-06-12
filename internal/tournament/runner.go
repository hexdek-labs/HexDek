package tournament

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
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
	"github.com/hexdek/hexdek/internal/seedcontract"
)

// contractParams bundles the per-tournament SeedContract metadata so
// each of the three game-path entry points (Run / runPool /
// runLazyPool) can hand it down to runOneGame without growing four
// separate parameters. Zero-value (nil key, empty strings) disables
// signing — contracts are still constructed and digests computed so
// downstream tooling can detect "unsigned" runs and refuse them.
type contractParams struct {
	key           []byte
	context       string
	engineVersion string
}

func contractParamsFromConfig(cfg TournamentConfig) contractParams {
	return contractParams{
		key:           cfg.ContractKey,
		context:       cfg.ContractContext,
		engineVersion: cfg.EngineVersion,
	}
}

// deckKeyFromPath extracts the canonical "owner/name" deck key used by
// Heimdall and the SeedContract. Mirrors heimdall/replay.go's
// resolveDeck convention so a contract emitted here can be re-resolved
// to the same deck file later. Falls back to filename-stem for paths
// outside data/decks/, and to commander name when path is empty.
func deckKeyFromPath(td *deckparser.TournamentDeck) string {
	if td == nil {
		return ""
	}
	if td.Path != "" {
		dir, file := filepath.Split(td.Path)
		stem := strings.TrimSuffix(file, filepath.Ext(file))
		dir = strings.TrimRight(dir, string(filepath.Separator))
		// Take the last directory component as the "owner".
		owner := filepath.Base(dir)
		if owner != "." && owner != "" && owner != "/" {
			return owner + "/" + stem
		}
		return stem
	}
	if td.CommanderName != "" {
		return "?/" + strings.ToLower(td.CommanderName)
	}
	return ""
}

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
				outcome := runOneGameSafe(gameIdx, decks, hats, cfg.NSeats, cfg.Seed, maxTurns, gameTimeout, cfg.CommanderMode, cfg.AuditEnabled, cfg.AnalyticsEnabled, contractParamsFromConfig(cfg))
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

	// Muninn batcher: accumulates per-game observations and flushes
	// periodically (every 30s or 100 games) instead of one bulk write
	// at the end. Close() after aggregation ensures nothing is lost.
	batcher := muninn.NewBatcher(muninn.BatcherConfig{Dir: "data/muninn"})

	// Optionally intercept outcomes to award achievements before they
	// reach the aggregator. Owners parallel cfg.Decks, so the standard
	// rotate mode resolves SeatStats.CommanderIdx → cfg.Decks[idx].Path.
	var aggInput <-chan GameOutcome = outcomes
	if cfg.Achievements != nil {
		owners := make([]string, len(decks))
		for i, d := range decks {
			owners[i] = ownerFromDeckPath(d.Path)
		}
		forwarded := make(chan GameOutcome, bufferSize)
		go func() {
			for o := range outcomes {
				awardAchievements(cfg.Achievements, o, owners)
				forwarded <- o
			}
			close(forwarded)
		}()
		aggInput = forwarded
	}

	// Intercept outcomes to feed the Muninn batcher per-game.
	batchedInput := make(chan GameOutcome, bufferSize)
	go func() {
		for o := range aggInput {
			feedBatcher(batcher, o, commanderNames)
			batchedInput <- o
		}
		close(batchedInput)
	}()

	// Aggregator.
	result := aggregate(batchedInput, cfg.NGames, cfg.NSeats, commanderNames)
	result.Duration = time.Since(start)
	if result.Duration.Seconds() > 0 {
		result.GamesPerSecond = float64(result.Games) / result.Duration.Seconds()
	}

	// Flush remaining buffered Muninn data.
	if err := batcher.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "muninn: batcher close: %v\n", err)
	}

	if cfg.ReportPath != "" {
		if err := result.WriteMarkdown(cfg.ReportPath); err != nil {
			return result, fmt.Errorf("tournament: write report: %w", err)
		}
	}

	// Persist non-Muninn data (Huginn, rivalry, threat graph).
	persistPostTournament(result)

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
	nSeats int, masterSeed int64, maxTurns int, gameTimeout time.Duration, commanderMode, auditEnabled, analyticsEnabled bool, contracts contractParams) (outcome GameOutcome) {
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
		ch <- runOneGame(gameIdx, decks, hats, nSeats, masterSeed, maxTurns, commanderMode, auditEnabled, analyticsEnabled, &prog, contracts)
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
	nSeats int, masterSeed int64, maxTurns int, commanderMode, auditEnabled, analyticsEnabled bool, prog *gameProgress, contracts contractParams) GameOutcome {
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

	// Phase 1 anti-cheat: build the per-game SeedContract from inputs
	// before the first shuffle. We snapshot deck keys in the
	// post-rotation seat order so the contract reads "seat 0 played
	// X, seat 1 played Y, ..." — the same order the game logs.
	contractInputs := seedcontract.Inputs{
		RNGSeed:       gameSeed,
		NSeats:        nSeats,
		EngineVersion: contracts.engineVersion,
		SealedAtUnix:  time.Now().Unix(),
	}
	for i := 0; i < seedcontract.MaxSeats && i < nSeats; i++ {
		orig := (i + out.Rot) % nSeats
		if orig < len(decks) {
			contractInputs.DeckKeys[i] = deckKeyFromPath(decks[orig])
		}
	}
	contract := seedcontract.New(contractInputs)
	out.SeedContract = contract

	gs := gameengine.NewGameState(nSeats, rng, nil)
	// Record the seed on the state (r62): consumers — notably the hat's
	// deterministic noise-RNG seeding — treat Seed==0 as "unseeded" and
	// fall back to nondeterministic behavior.
	gs.Seed = gameSeed
	if !auditEnabled {
		gs.EventPolicy = gameengine.EventLogNone
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

	// Track elimination order via the shared seat-indexed tracker
	// (outcome_canon.go) — heimdall's anti-cheat replay runs the SAME
	// tracker so the sealed digest and the replayed digest agree. The
	// deck-indexed out.EliminationOrder copy is kept incremental so a
	// mid-game crash still reports the eliminations seen so far.
	elim := NewElimTracker()
	markElim := func() {
		elim.Mark(gs)
		for i := range gs.Seats {
			if i < len(originalIdxForSeat) && i < seedcontract.MaxSeats && elim.Slots[i] >= 0 {
				out.EliminationOrder[originalIdxForSeat[i]] = elim.Slots[i]
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

	// Determine winner + adjudicate turn caps via the shared helper
	// (outcome_canon.go) — same rules the anti-cheat replay applies, so
	// the sealed outcome digest is reproducible by an honest replay.
	winner, endReason := AdjudicateGameEnd(gs, nSeats, ended)
	if winner >= 0 {
		out.Winner = winner
		out.WinnerCommanderIdx = originalIdxForSeat[winner]
	}
	if endReason != "" {
		out.EndReason = endReason
	}
	// Fill the remaining elimination slots (winner + any turn-cap
	// losers marked Lost after the final Mark), in seat order.
	elim.FillRemaining(gs)
	for i := range gs.Seats {
		if i < len(originalIdxForSeat) && i < seedcontract.MaxSeats && elim.Slots[i] >= 0 {
			out.EliminationOrder[originalIdxForSeat[i]] = elim.Slots[i]
		}
	}

	// Count concessions from seat state (works even with EventPolicy=EventLogNone).
	// Also collect concession records for Muninn diagnostics.
	for i, s := range gs.Seats {
		if s != nil && s.LossReason == "concession" {
			out.Concessions++
			orig := originalIdxForSeat[i]
			cmdrName := ""
			if orig < len(decks) {
				cmdrName = decks[orig].CommanderName
			}
			boardPower := 0
			for _, p := range s.Battlefield {
				if p != nil && p.IsCreature() {
					boardPower += p.Power()
				}
			}
			opponentsAlive := 0
			for j, other := range gs.Seats {
				if j != i && other != nil && !other.Lost {
					opponentsAlive++
				}
			}
			out.ConcessionRecords = append(out.ConcessionRecords, muninn.ConcessionRecord{
				Commander:  cmdrName,
				Turn:       gs.Turn,
				BoardPower: boardPower,
				Life:       s.Life,
				HandSize:   len(s.Hand),
				Opponents:  opponentsAlive,
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	// Track min relative position for conviction calibration.
	for _, s := range gs.Seats {
		if s == nil {
			continue
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

	// Kill record extraction: infer who eliminated whom from the event
	// log. Requires auditEnabled (EventPolicy=EventLogFull) for event access.
	if auditEnabled && len(gs.EventLog) > 0 {
		cmdrNames := make([]string, nSeats)
		for i := 0; i < nSeats; i++ {
			orig := originalIdxForSeat[i]
			if orig < len(decks) {
				cmdrNames[i] = decks[orig].CommanderName
			}
		}
		out.KillRecords = analytics.ExtractKillRecords(
			gs.EventLog, nSeats, cmdrNames, out.Winner,
			fmt.Sprintf("game-%d", gameIdx),
		)
	}

	// Post-game per-seat stats (always emitted, no event log needed).
	out.PostGameStats = make([]SeatStats, nSeats)
	for i, s := range gs.Seats {
		if s == nil {
			continue
		}
		orig := originalIdxForSeat[i]
		ss := SeatStats{
			CommanderIdx: orig,
			Won:          i == out.Winner,
			FinalLife:    s.Life,
			HandSize:     len(s.Hand),
			GraveyardSize: len(s.Graveyard),
			Conceded:     s.LossReason == "concession",
		}
		if orig < len(decks) {
			ss.CommanderName = decks[orig].CommanderName
		}
		if s.Lost && !s.Won {
			ss.TurnOfDeath = gs.Turn
		}

		creatures := 0
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			ss.TotalBoardSize++
			if p.IsCreature() {
				creatures++
			}
		}
		ss.CreaturesOnBoard = creatures
		ss.ManaSourceCount = hat.CountManaRocksAndLands(s)
		ss.SpellsCast = s.SpellsCastThisTurn + s.SpellsCastLastTurn
		out.PostGameStats[i] = ss
	}

	// Phase 1 anti-cheat: seal the SeedContract with the observed
	// outcome, then sign with the per-tournament HMAC key. We seal
	// even when key is nil so downstream tooling sees a consistent
	// "outcome digest computed but unsigned" record — easier to
	// detect skipped signing than to reconstruct it later. Final life
	// reads from gs.Seats post-game; eliminationOrder is stored on
	// out by commander idx and we map back to seat order here so the
	// outcome digest's positions match the digest's seat-indexed
	// inputs.
	if out.SeedContract != nil {
		// Seal with the seat-indexed canonical fields. KillMethod is
		// NOT computed here — Seal derives it from (Winner, EndReason)
		// via seedcontract.CanonicalizeOutcome, the same derivation the
		// replay verifier applies, so the two sides cannot drift.
		// elim.Slots is already seat-indexed (the old code round-
		// tripped through the deck-indexed out.EliminationOrder and
		// back; the mapping composed to identity).
		out.SeedContract.Seal(seedcontract.Outcome{
			Winner:           out.Winner,
			Turns:            out.Turns,
			EndReason:        out.EndReason,
			EliminationOrder: elim.Slots,
			FinalLife:        FinalLifeFromState(gs, nSeats),
		})
		if len(contracts.key) > 0 {
			out.SeedContract.Sign(contracts.key)
		}
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
					cfg.Seed, maxTurns, gameTimeout, cfg.CommanderMode, cfg.AuditEnabled, cfg.AnalyticsEnabled, contractParamsFromConfig(cfg))
				outcomes <- poolOutcome{o, job.deckIdxs}
				done := atomic.AddInt64(&completed, 1)
				if progressEvery > 0 && done%int64(progressEvery) == 0 {
					gps := float64(done) / time.Since(start).Seconds()
					fmt.Fprintf(os.Stderr, "  pool: %d/%d games (%.0f g/s)\n", done, cfg.NGames, gps)
				}
			}
		}()
	}

	// Job producer: for each game, pick NSeats deck indices. The
	// sampler honors two orthogonal config knobs:
	//   - MaxIntraPodSimilarity (Jaccard threshold; rejects near-
	//     clone pods)
	//   - PreferArchetypeOpposition + DeckArchetypes (biases toward
	//     combo↔stax / control↔aggro etc. matchups)
	// Either or both can be active; both off restores the legacy
	// uniform-random path. See SeedPodWithOptions for the constraint
	// relaxation order on retry-budget exhaustion.
	go func() {
		rng := rand.New(rand.NewSource(cfg.Seed))
		seedOpts := SeedPodOptions{
			MaxSimilarity:    cfg.MaxIntraPodSimilarity,
			PreferOpposition: cfg.PreferArchetypeOpposition,
			Archetypes:       cfg.DeckArchetypes,
		}
		for i := 0; i < cfg.NGames; i++ {
			idxs := SeedPodWithOptions(allDecks, nSeats, rng, seedOpts)
			jobs <- poolJob{gameIdx: i, deckIdxs: idxs}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	// Owners parallel to allDecks for achievement attribution.
	var poolOwners []string
	if cfg.Achievements != nil {
		poolOwners = make([]string, len(allDecks))
		for i, d := range allDecks {
			poolOwners[i] = ownerFromDeckPath(d.Path)
		}
	}

	// Muninn batcher for pool mode.
	poolBatcher := muninn.NewBatcher(muninn.BatcherConfig{Dir: "data/muninn"})

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
		// Feed per-game data to the Muninn batcher.
		feedBatcher(poolBatcher, o.GameOutcome, allNames)
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
		awardAchievements(cfg.Achievements, o.GameOutcome, poolOwners)
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

	// Promote the per-deck stats we already computed into the
	// TournamentResult so report.go's per-commander winrate path (which
	// already keys on len(GamesPlayedByCommander) > 0) sees the data.
	// Without this, pool-mode reports divided wins by totalGames — wrong
	// for any pool larger than nSeats, and especially wrong under the
	// #145/#150 seeding constraints which deliberately skew the
	// distribution.
	winsByCmdr := make(map[string]int, len(stats))
	gamesByCmdr := make(map[string]int, len(stats))
	commanderNames := make([]string, 0, len(stats))
	for name, s := range stats {
		commanderNames = append(commanderNames, name)
		gamesByCmdr[name] = s.games
		if s.wins > 0 {
			winsByCmdr[name] = s.wins
		}
	}
	result := &TournamentResult{
		SchemaVersion:          SchemaVersion,
		Mode:                   ModePool,
		Games:                  totalGames,
		Crashes:                crashes,
		Duration:               elapsed,
		CrashLogs:              crashLogs,
		NSeats:                 cfg.NSeats,
		CommanderNames:         commanderNames,
		WinsByCommander:        winsByCmdr,
		GamesPlayedByCommander: gamesByCmdr,
	}
	if elapsed.Seconds() > 0 {
		result.GamesPerSecond = float64(totalGames) / elapsed.Seconds()
	}
	if totalGames > 0 {
		result.AvgTurns = float64(totalTurns) / float64(totalGames)
	}
	result.TotalConcessions = totalConcessions

	// Flush remaining buffered Muninn data for pool mode.
	if err := poolBatcher.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "muninn: batcher close (pool): %v\n", err)
	}

	// Persist non-Muninn data (Huginn, rivalry, threat graph).
	persistPostTournament(result)

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
						cfg.Seed, maxTurns, gameTimeout, cfg.CommanderMode, cfg.AuditEnabled, cfg.AnalyticsEnabled, contractParamsFromConfig(cfg))
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
			// NOTE: cfg.MaxIntraPodSimilarity is NOT honored here.
			// LazyPool's whole point is to avoid materializing every
			// deck up front — but DeckSimilarity needs both decks
			// loaded. Wiring similarity-aware seeding into lazy mode
			// would either negate the lazy-loading optimization (load
			// every deck just to compute the similarity matrix) or
			// pay a per-pod load cost (load NSeats decks per attempt,
			// times up to defaultSeedPodMaxAttempts attempts). Neither
			// is worth it for the typical lazy-pool use case (bug-
			// hunt across thousands of decks where collisions are
			// rare). PoolMode supports similarity-aware seeding;
			// LazyPool does not.
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

	// Owners parallel to paths for achievement attribution.
	var lazyOwners []string
	if cfg.Achievements != nil {
		lazyOwners = make([]string, len(paths))
		for i, p := range paths {
			lazyOwners[i] = ownerFromDeckPath(p)
		}
	}

	// Muninn batcher for lazy-pool mode.
	lazyBatcher := muninn.NewBatcher(muninn.BatcherConfig{Dir: "data/muninn"})

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
		// Feed per-game data to the Muninn batcher.
		feedBatcher(lazyBatcher, out.GameOutcome, cmdrNames)
		awardAchievements(cfg.Achievements, out.GameOutcome, lazyOwners)
		// After the first game completes, dump a heap profile for leak diagnosis.
		if totalGames == 1 && cfg.PprofEnabled {
			runtime.GC()
			if f, err := os.Create("/tmp/hexdek_heap_post1.prof"); err == nil {
				rpprof.WriteHeapProfile(f)
				f.Close()
				fmt.Fprintf(os.Stderr, "  heap profile written to /tmp/hexdek_heap_post1.prof\n")
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

	// Promote per-deck stats into the TournamentResult so the report
	// layer can compute correct per-commander winrates. See the parallel
	// runPool comment for rationale.
	winsByCmdr := make(map[string]int, len(stats))
	gamesByCmdr := make(map[string]int, len(stats))
	commanderNames := make([]string, 0, len(stats))
	for name, s := range stats {
		commanderNames = append(commanderNames, name)
		gamesByCmdr[name] = s.games
		if s.wins > 0 {
			winsByCmdr[name] = s.wins
		}
	}
	result := &TournamentResult{
		SchemaVersion:          SchemaVersion,
		Mode:                   ModeLazyPool,
		Games:                  totalGames,
		Duration:               elapsed,
		CrashLogs:              crashLogs,
		NSeats:                 cfg.NSeats,
		CommanderNames:         commanderNames,
		WinsByCommander:        winsByCmdr,
		GamesPlayedByCommander: gamesByCmdr,
	}
	if elapsed.Seconds() > 0 {
		result.GamesPerSecond = float64(totalGames) / elapsed.Seconds()
	}

	// Flush remaining buffered Muninn data for lazy-pool mode.
	if err := lazyBatcher.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "muninn: batcher close (lazy-pool): %v\n", err)
	}

	// Persist non-Muninn data (Huginn, rivalry, threat graph).
	persistPostTournament(result)

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

// feedBatcher sends a single game's Muninn-relevant data into the batcher.
// Called once per completed game from the aggregation loops. The batcher
// handles batching, deduplication, and periodic flushing to disk.
func feedBatcher(b *muninn.Batcher, o GameOutcome, commanderNames []string) {
	// Parser gaps.
	if len(o.ParserGapSnippets) > 0 {
		b.AddParserGaps(o.ParserGapSnippets)
	}

	// Crash logs.
	if o.CrashErr != "" {
		b.AddCrash(o.CrashErr, commanderNames, 0, 0)
	}

	// Concession records.
	if len(o.ConcessionRecords) > 0 {
		b.AddConcessions(o.ConcessionRecords)
	}

	// Dead triggers extracted from analytics (mirrors PersistDeadTriggers logic).
	if o.Analysis != nil {
		for _, pa := range o.Analysis.Players {
			for _, cp := range pa.CardsPlayed {
				if cp.TriggeredCount > 0 &&
					cp.DamageDealt == 0 &&
					cp.KillsAttributed == 0 &&
					!cp.ContributedToWin &&
					cp.Name != o.Analysis.WinningCard &&
					!cp.IsLand &&
					!cp.IsToken {
					b.AddDeadTrigger("triggered_ability", cp.Name, cp.TriggeredCount, 1)
				}
			}
		}
	}

	// Tick the per-game counter for flush-interval tracking.
	b.EndGame()
}

// persistPostTournament writes non-Muninn data after the tournament:
// Huginn raw observations, rivalry matchups, and threat graph kills.
// Muninn data (parser gaps, crashes, dead triggers, concessions) is
// handled by the batcher during the run.
// Errors are logged to stderr but do not fail the tournament run.
func persistPostTournament(result *TournamentResult) {
	const huginnDir = "data/huginn"
	const rivalryDir = "data/rivalry"
	const analyticsDir = "data/analytics"

	if len(result.Analyses) > 0 {
		if err := huginn.PersistRawObservations(huginnDir, result.Analyses, result.CommanderNames); err != nil {
			fmt.Fprintf(os.Stderr, "huginn: persist raw observations: %v\n", err)
		}
		if err := huginn.PersistRawNTuples(huginnDir, result.Analyses, result.CommanderNames); err != nil {
			fmt.Fprintf(os.Stderr, "huginn: persist raw ntuples: %v\n", err)
		}
	}

	// Persist rivalry matchup data (accumulates across runs).
	if result.MatchupMatrix != nil && result.MatchupGames != nil {
		if err := analytics.PersistRivalries(rivalryDir, result.MatchupMatrix, result.MatchupGames); err != nil {
			fmt.Fprintf(os.Stderr, "rivalry: persist matchups: %v\n", err)
		}
	}

	// Persist threat graph kill records (accumulates across runs).
	if len(result.KillRecords) > 0 {
		if err := analytics.PersistThreatGraph(analyticsDir, result.KillRecords); err != nil {
			fmt.Fprintf(os.Stderr, "threat_graph: persist kills: %v\n", err)
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
