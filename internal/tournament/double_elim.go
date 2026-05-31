package tournament

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/trueskill"
)

// DoubleEliminationConfig defines a multiplayer-adapted double-
// elimination tournament. In 1v1 DE each player has two brackets
// (winners + losers) and is eliminated after losing twice. For pods,
// the natural multi-seat generalization is:
//
//   - Every deck carries a Losses counter.
//   - Each round forms pods from the still-eligible decks
//     (Losses < 2). Within a round, pods are formed greedily with
//     decks of similar loss tier together — undefeated decks pair
//     together, one-loss decks pair together — and tiers mix only
//     when one tier has fewer than NSeats decks left.
//   - The pod winner accrues a game win and stays at the same loss
//     count. The 3 losers each accrue +1 loss.
//   - After each round, decks with Losses >= 2 are eliminated.
//   - Tournament ends when ≤ 1 deck remains eligible OR MaxRounds is
//     reached.
//
// FinalRank is assigned by inverse elimination order: the last deck
// standing is rank 1, the most recently eliminated batch is rank 2,
// etc. Ties within an elimination batch break on GameWins desc →
// CommanderName asc.
//
// Single-elimination is the same shape with Losses < 1; this runner
// does NOT implement that — use the dedicated knockout style when
// needed. Double elimination is the more interesting multiplayer
// format because the "first loss in a pod" verdict is noisy
// (a deck can win the tournament after losing one pod, which would
// be impossible in single-elim and unfair given multiplayer-pod
// variance).
type DoubleEliminationConfig struct {
	Decks            []*deckparser.TournamentDeck
	NSeats           int
	GamesPerPod      int
	MaxRounds        int // safety cap; defaults to 2 * NDecks
	Seed             int64
	HatFactories     []HatFactory
	Workers          int
	CommanderMode    bool
	AuditEnabled     bool
	AnalyticsEnabled bool
	ReportPath       string
	MaxTurnsPerGame  int
	GameTimeout      time.Duration
	ProgressLogEvery int
	ProgressLogger   func(done, total int, gps float64)
}

// EliminationStanding is one deck's final-bracket placement.
// FinalRank=1 is the champion. EliminatedRound is the round in
// which the deck reached Losses == 2; 0 for the champion (never
// eliminated). LossesAtEnd reports the loss count when the
// tournament stopped — typically 0 for the champion and 2 for
// everyone else, but can be 1 for any deck that reached
// MaxRounds before fully eliminating.
type EliminationStanding struct {
	CommanderName    string `json:"commander_name"`
	FinalRank        int    `json:"final_rank"`
	LossesAtEnd      int    `json:"losses_at_end"`
	EliminatedRound  int    `json:"eliminated_round,omitempty"`
	PodsPlayed       int    `json:"pods_played"`
	GameWins         int    `json:"game_wins"`
	GameLosses       int    `json:"game_losses"`
	GameDraws        int    `json:"game_draws"`
}

// RunDoubleElimination executes a double-elimination tournament with
// the rules described in DoubleEliminationConfig. See the package-
// level survey in SupportedBracketStyles() for the full bracket
// catalogue.
func RunDoubleElimination(cfg DoubleEliminationConfig) (*TournamentResult, error) {
	if cfg.NSeats < 2 {
		return nil, fmt.Errorf("tournament: NSeats must be >= 2")
	}
	if len(cfg.Decks) < cfg.NSeats {
		return nil, fmt.Errorf("tournament: need at least %d decks, got %d", cfg.NSeats, len(cfg.Decks))
	}
	if cfg.GamesPerPod <= 0 {
		cfg.GamesPerPod = 1
	}
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 2 * len(cfg.Decks)
	}
	for i, d := range cfg.Decks {
		if d == nil {
			return nil, fmt.Errorf("tournament: decks[%d] is nil", i)
		}
		if len(d.CommanderCards) == 0 {
			return nil, fmt.Errorf("tournament: decks[%d] has no commander", i)
		}
	}

	nDecks := len(cfg.Decks)
	commanderNames := make([]string, nDecks)
	for i, d := range cfg.Decks {
		commanderNames[i] = d.CommanderName
	}

	hatPerDeck := make([]HatFactory, nDecks)
	switch len(cfg.HatFactories) {
	case 0:
		for i := range hatPerDeck {
			hatPerDeck[i] = defaultHatFactory
		}
	case 1:
		for i := range hatPerDeck {
			hatPerDeck[i] = cfg.HatFactories[0]
		}
	default:
		if len(cfg.HatFactories) < nDecks {
			return nil, fmt.Errorf("tournament: HatFactories must be 0, 1, or len(Decks)")
		}
		copy(hatPerDeck, cfg.HatFactories[:nDecks])
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

	// Per-deck running state.
	state := make([]deckState, nDecks)

	r := newSwissAggResult(commanderNames, cfg.NSeats)
	r.Mode = ModeDoubleElim
	elo := NewELORatings(commanderNames)
	ts := trueskill.NewTrueSkillRatings(commanderNames)
	winTurnSum := make(map[string]int)
	winTurnCount := make(map[string]int)
	totalTurns := 0

	var completed int64
	start := time.Now()
	progressEvery := cfg.ProgressLogEvery
	// Upper bound for progress totals — we can't know exactly how
	// many games will run (DE terminates dynamically), but
	// MaxRounds × ceil(NDecks / NSeats) × GamesPerPod is a sound cap.
	progressMax := cfg.MaxRounds * ((nDecks + cfg.NSeats - 1) / cfg.NSeats) * cfg.GamesPerPod
	if progressEvery == 0 {
		progressEvery = 1000
		if progressMax/20 > progressEvery {
			progressEvery = progressMax / 20
		}
	}

	fmt.Fprintf(os.Stderr, "  double-elim: %d decks, NSeats=%d, MaxRounds=%d\n",
		nDecks, cfg.NSeats, cfg.MaxRounds)

	round := 0
	for round < cfg.MaxRounds {
		round++
		eligible := make([]int, 0, nDecks)
		for i := range state {
			if state[i].losses < 2 {
				eligible = append(eligible, i)
			}
		}
		if len(eligible) <= 1 {
			break // tournament concluded
		}
		// Need at least NSeats eligible to form a pod. If fewer remain
		// (e.g., 2 decks left after losers-bracket elimination cycle),
		// stop — the leader becomes the champion. This is the
		// pragmatic multiplayer-DE termination rule.
		if len(eligible) < cfg.NSeats {
			break
		}

		pods := pairDoubleElimRound(eligible, state, cfg.NSeats, cfg.Seed+int64(round)*1_000_003)
		if len(pods) == 0 {
			break
		}

		// Run all pods this round in parallel.
		runDoubleElimRound(doubleElimRoundParams{
			cfg:            cfg,
			pods:           pods,
			round:          round,
			workers:        workers,
			maxTurns:       maxTurns,
			gameTimeout:    gameTimeout,
			hatPerDeck:     hatPerDeck,
			commanderNames: commanderNames,
			result:         r,
			elo:            elo,
			ts:             ts,
			winTurnSum:     winTurnSum,
			winTurnCount:   winTurnCount,
			totalTurns:     &totalTurns,
			completed:      &completed,
			start:          start,
			progressMax:    progressMax,
			progressEvery:  progressEvery,
			onOutcome: func(deckIdxs []int, winnerLocalIdx int) {
				for localIdx, deckIdx := range deckIdxs {
					state[deckIdx].pods++
					if winnerLocalIdx < 0 {
						state[deckIdx].draws++
						continue
					}
					if localIdx == winnerLocalIdx {
						state[deckIdx].wins++
					} else {
						state[deckIdx].gameLosses++
						state[deckIdx].losses++
						if state[deckIdx].losses >= 2 && state[deckIdx].eliminatedRound == 0 {
							state[deckIdx].eliminatedRound = round
						}
					}
				}
			},
		})
	}

	// Finalize aggregate.
	if r.Games > 0 {
		r.AvgTurns = float64(totalTurns) / float64(r.Games)
	}
	for name, count := range winTurnCount {
		if count > 0 {
			r.AvgTurnToWin[name] = float64(winTurnSum[name]) / float64(count)
		}
	}
	if len(r.Analyses) > 0 {
		r.CardRankings = analytics.RankCards(r.Analyses)
		r.MatchupDetails = analytics.BuildMatchupDetails(r.Analyses)
	}
	r.ELO = elo.Snapshot()
	r.TrueSkill = ts.Snapshot()

	r.EliminationStandings = buildEliminationStandings(commanderNames, state)
	r.Duration = time.Since(start)
	if r.Duration.Seconds() > 0 {
		r.GamesPerSecond = float64(r.Games) / r.Duration.Seconds()
	}

	if cfg.ReportPath != "" {
		if err := r.WriteMarkdown(cfg.ReportPath); err != nil {
			return r, fmt.Errorf("tournament: write report: %w", err)
		}
	}
	return r, nil
}

// deckState is the per-deck running tally maintained across DE
// rounds. Exposed at package scope so pairDoubleElimRound and
// buildEliminationStandings can take it as a typed slice.
type deckState struct {
	losses          int
	wins            int
	draws           int
	gameLosses      int
	pods            int
	eliminatedRound int // 0 = still in the running
}

// pairDoubleElimRound forms pods from the eligible deck pool.
// Sorting key: losses asc (winners-tier first), pods desc as a
// freshness signal (decks that have played fewer games this
// tournament sort later so the high-volume decks pair up first
// when tier sizes are equal), then deterministic shuffle.
//
// Greedy top-down: walk the sorted list and take NSeats at a time.
// Decks at the same loss tier pair together; tiers mix only at the
// boundary when one tier has < NSeats remaining decks.
func pairDoubleElimRound(eligible []int, state []deckState, nSeats int, roundSeed int64) [][]int {
	type entry struct {
		idx     int
		shuffle uint64
	}
	order := make([]entry, len(eligible))
	for i, idx := range eligible {
		order[i] = entry{
			idx:     idx,
			shuffle: splitmix64(uint64(roundSeed) ^ uint64(idx*2654435761)),
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		ai, aj := state[order[i].idx], state[order[j].idx]
		if ai.losses != aj.losses {
			return ai.losses < aj.losses
		}
		if ai.pods != aj.pods {
			return ai.pods < aj.pods
		}
		return order[i].shuffle < order[j].shuffle
	})

	var pods [][]int
	for len(order) >= nSeats {
		pod := make([]int, nSeats)
		for i := 0; i < nSeats; i++ {
			pod[i] = order[i].idx
		}
		pods = append(pods, pod)
		order = order[nSeats:]
	}
	return pods
}

// buildEliminationStandings turns the per-deck loss/elimination
// trail into the user-visible rank table.
//
// Ranking rules:
//   - Decks with eliminatedRound == 0 (still alive at tournament
//     end) rank ABOVE decks that were eliminated. Among them, ties
//     break on wins desc → name asc.
//   - Decks that were eliminated rank by eliminatedRound DESC
//     (later eliminations finished higher). Within the same
//     elimination round, ties break on wins desc → name asc.
func buildEliminationStandings(commanderNames []string, state []deckState) []EliminationStanding {
	standings := make([]EliminationStanding, len(commanderNames))
	for i, name := range commanderNames {
		standings[i] = EliminationStanding{
			CommanderName:   name,
			LossesAtEnd:     state[i].losses,
			EliminatedRound: state[i].eliminatedRound,
			PodsPlayed:      state[i].pods,
			GameWins:        state[i].wins,
			GameLosses:      state[i].gameLosses,
			GameDraws:       state[i].draws,
		}
	}
	sort.SliceStable(standings, func(i, j int) bool {
		a, b := standings[i], standings[j]
		// Still-alive (eliminatedRound == 0) rank highest.
		aAlive, bAlive := a.EliminatedRound == 0, b.EliminatedRound == 0
		if aAlive != bAlive {
			return aAlive
		}
		// Within still-alive: fewer LossesAtEnd ranks higher.
		if aAlive && bAlive && a.LossesAtEnd != b.LossesAtEnd {
			return a.LossesAtEnd < b.LossesAtEnd
		}
		// Within eliminated: later round ranks higher (lasted longer).
		if !aAlive && !bAlive && a.EliminatedRound != b.EliminatedRound {
			return a.EliminatedRound > b.EliminatedRound
		}
		if a.GameWins != b.GameWins {
			return a.GameWins > b.GameWins
		}
		return a.CommanderName < b.CommanderName
	})
	for i := range standings {
		standings[i].FinalRank = i + 1
	}
	return standings
}

// doubleElimRoundParams bundles per-round runtime state for one
// parallel pod run. The onOutcome callback fires once per resolved
// pod, on the goroutine that drains outcomes — caller updates
// per-deck loss state from there.
type doubleElimRoundParams struct {
	cfg            DoubleEliminationConfig
	pods           [][]int
	round          int
	workers        int
	maxTurns       int
	gameTimeout    time.Duration
	hatPerDeck     []HatFactory
	commanderNames []string
	result         *TournamentResult
	elo            *ELORatings
	ts             *trueskill.TrueSkillRatings
	winTurnSum     map[string]int
	winTurnCount   map[string]int
	totalTurns     *int
	completed      *int64
	start          time.Time
	progressMax    int
	progressEvery  int
	onOutcome      func(deckIdxs []int, winnerLocalIdx int)
}

func runDoubleElimRound(p doubleElimRoundParams) {
	type workItem struct {
		podIdx    int
		gameInPod int
		deckIdxs  []int
	}
	work := make(chan workItem, p.workers*4)
	bufferSize := p.workers * defaultBufferMult
	if bufferSize < 64 {
		bufferSize = 64
	}
	outcomes := make(chan rrOutcome, bufferSize)

	var wg sync.WaitGroup
	for w := 0; w < p.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				podDecks := make([]*deckparser.TournamentDeck, p.cfg.NSeats)
				podHats := make([]HatFactory, p.cfg.NSeats)
				for i, gIdx := range item.deckIdxs {
					podDecks[i] = p.cfg.Decks[gIdx]
					podHats[i] = p.hatPerDeck[gIdx]
				}
				outcome := runOneGameSafe(
					item.gameInPod, podDecks, podHats, p.cfg.NSeats,
					p.cfg.Seed+int64(p.round)*1_000_007+int64(item.podIdx)*10_007,
					p.maxTurns, p.gameTimeout,
					p.cfg.CommanderMode, p.cfg.AuditEnabled, p.cfg.AnalyticsEnabled,
					contractParams{},
				)
				outcomes <- rrOutcome{outcome: outcome, deckIdxs: item.deckIdxs}

				done := atomic.AddInt64(p.completed, 1)
				if p.progressEvery > 0 && done%int64(p.progressEvery) == 0 {
					gps := float64(done) / time.Since(p.start).Seconds()
					if p.cfg.ProgressLogger != nil {
						p.cfg.ProgressLogger(int(done), p.progressMax, gps)
					} else {
						fmt.Fprintf(os.Stderr, "  double-elim: %d games (%.0f g/s) [round %d]\n",
							done, gps, p.round)
					}
				}
			}
		}()
	}

	go func() {
		for podIdx, pod := range p.pods {
			for g := 0; g < p.cfg.GamesPerPod; g++ {
				work <- workItem{podIdx: podIdx, gameInPod: g, deckIdxs: pod}
			}
		}
		close(work)
	}()

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	for rr := range outcomes {
		applyDoubleElimOutcome(p, rr)
	}
}

func applyDoubleElimOutcome(p doubleElimRoundParams, rr rrOutcome) {
	outcome := rr.outcome
	globalIdxs := rr.deckIdxs

	if outcome.CrashErr != "" {
		p.result.Crashes++
		return
	}
	p.result.Games++
	*p.totalTurns += outcome.Turns
	p.result.TotalModeChanges += outcome.ModeChanges

	for _, gIdx := range globalIdxs {
		if gIdx >= 0 && gIdx < len(p.commanderNames) {
			p.result.GamesPlayedByCommander[p.commanderNames[gIdx]]++
		}
	}

	winnerName := ""
	winnerLocal := -1
	if outcome.WinnerCommanderIdx >= 0 && outcome.WinnerCommanderIdx < len(globalIdxs) {
		winnerLocal = outcome.WinnerCommanderIdx
		globalWinner := globalIdxs[winnerLocal]
		if globalWinner >= 0 && globalWinner < len(p.commanderNames) {
			winnerName = p.commanderNames[globalWinner]
			p.result.WinsByCommander[winnerName]++
			p.winTurnSum[winnerName] += outcome.Turns
			p.winTurnCount[winnerName]++
		}
	} else {
		p.result.Draws++
	}

	switch {
	case outcome.Turns <= 5:
		p.result.TurnDistribution[0]++
	case outcome.Turns <= 10:
		p.result.TurnDistribution[1]++
	case outcome.Turns <= 20:
		p.result.TurnDistribution[2]++
	default:
		p.result.TurnDistribution[3]++
	}

	globalParticipants := make([]string, 0, p.cfg.NSeats)
	for _, gIdx := range globalIdxs {
		if gIdx >= 0 && gIdx < len(p.commanderNames) {
			globalParticipants = append(globalParticipants, p.commanderNames[gIdx])
		}
	}
	for i := 0; i < len(globalParticipants); i++ {
		for j := i + 1; j < len(globalParticipants); j++ {
			a, b := globalParticipants[i], globalParticipants[j]
			if a == b {
				continue
			}
			p.result.MatchupGames[a][b]++
			p.result.MatchupGames[b][a]++
			if winnerName == a {
				p.result.MatchupMatrix[a][b]++
			} else if winnerName == b {
				p.result.MatchupMatrix[b][a]++
			}
		}
	}
	p.elo.Update(winnerName, globalParticipants)

	if len(globalIdxs) >= 2 && len(outcome.EliminationOrder) == len(globalIdxs) {
		tsNames := make([]string, 0, len(globalIdxs))
		tsRanks := make([]int, 0, len(globalIdxs))
		for localIdx, gIdx := range globalIdxs {
			if gIdx >= 0 && gIdx < len(p.commanderNames) && localIdx < len(outcome.EliminationOrder) {
				tsNames = append(tsNames, p.commanderNames[gIdx])
				slot := outcome.EliminationOrder[localIdx]
				tsRanks = append(tsRanks, (p.cfg.NSeats-1)-slot)
			}
		}
		p.ts.Update(tsNames, tsRanks)
	}

	for localIdx, slot := range outcome.EliminationOrder {
		if localIdx < 0 || localIdx >= len(globalIdxs) {
			continue
		}
		gIdx := globalIdxs[localIdx]
		if gIdx < 0 || gIdx >= len(p.commanderNames) || slot < 0 || slot >= p.cfg.NSeats {
			continue
		}
		p.result.EliminationByCommanderBySlot[p.commanderNames[gIdx]][slot]++
	}

	if len(outcome.ParserGapSnippets) > 0 {
		for k, v := range outcome.ParserGapSnippets {
			p.result.ParserGapSnippets[k] += v
		}
	}
	if outcome.Analysis != nil {
		p.result.Analyses = append(p.result.Analyses, outcome.Analysis)
	}

	if p.onOutcome != nil {
		p.onOutcome(globalIdxs, winnerLocal)
	}
}
