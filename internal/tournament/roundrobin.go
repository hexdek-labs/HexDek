package tournament

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/trueskill"
)

// RoundRobinConfig defines a full round-robin tournament where every
// C(N, seats) combination of decks plays GamesPerPod games.
type RoundRobinConfig struct {
	Decks            []*deckparser.TournamentDeck
	NSeats           int
	GamesPerPod      int
	Seed             int64
	HatFactories     []HatFactory // parallel to Decks (one per deck)
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

type rrOutcome struct {
	outcome  GameOutcome
	deckIdxs []int
}

// RunRoundRobin executes a full round-robin tournament: every unique
// combination of NSeats decks from cfg.Decks plays cfg.GamesPerPod
// games. Returns a single aggregated result across all pods.
func RunRoundRobin(cfg RoundRobinConfig) (*TournamentResult, error) {
	if cfg.NSeats < 2 {
		return nil, fmt.Errorf("tournament: NSeats must be >= 2")
	}
	if len(cfg.Decks) < cfg.NSeats {
		return nil, fmt.Errorf("tournament: need at least %d decks, got %d", cfg.NSeats, len(cfg.Decks))
	}
	if cfg.GamesPerPod <= 0 {
		return nil, fmt.Errorf("tournament: GamesPerPod must be > 0")
	}
	for i, d := range cfg.Decks {
		if d == nil {
			return nil, fmt.Errorf("tournament: decks[%d] is nil", i)
		}
		if len(d.CommanderCards) == 0 {
			return nil, fmt.Errorf("tournament: decks[%d] has no commander", i)
		}
	}

	pods := combinations(len(cfg.Decks), cfg.NSeats)
	totalGames := len(pods) * cfg.GamesPerPod

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

	allNames := make([]string, len(cfg.Decks))
	for i, d := range cfg.Decks {
		allNames[i] = d.CommanderName
	}

	hatPerDeck := make([]HatFactory, len(cfg.Decks))
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
		if len(cfg.HatFactories) < len(cfg.Decks) {
			return nil, fmt.Errorf("tournament: HatFactories must be 0, 1, or len(Decks)")
		}
		copy(hatPerDeck, cfg.HatFactories[:len(cfg.Decks)])
	}

	progressEvery := cfg.ProgressLogEvery
	if progressEvery == 0 {
		progressEvery = 1000
		if totalGames/20 > progressEvery {
			progressEvery = totalGames / 20
		}
	}

	fmt.Fprintf(os.Stderr, "  round-robin: %d decks, %d pods (C(%d,%d)), %d games/pod, %d total games\n",
		len(cfg.Decks), len(pods), len(cfg.Decks), cfg.NSeats, cfg.GamesPerPod, totalGames)

	type workItem struct {
		podIdx    int
		gameInPod int
		deckIdxs  []int
	}

	work := make(chan workItem, workers*4)
	bufferSize := workers * defaultBufferMult
	if bufferSize < 64 {
		bufferSize = 64
	}
	outcomes := make(chan rrOutcome, bufferSize)

	var completed int64
	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				podDecks := make([]*deckparser.TournamentDeck, cfg.NSeats)
				podHats := make([]HatFactory, cfg.NSeats)
				for i, gIdx := range item.deckIdxs {
					podDecks[i] = cfg.Decks[gIdx]
					podHats[i] = hatPerDeck[gIdx]
				}
				outcome := runOneGameSafe(
					item.gameInPod, podDecks, podHats, cfg.NSeats,
					cfg.Seed+int64(item.podIdx)*10000, maxTurns, gameTimeout,
					cfg.CommanderMode, cfg.AuditEnabled, cfg.AnalyticsEnabled,
				)
				outcomes <- rrOutcome{outcome: outcome, deckIdxs: item.deckIdxs}

				done := atomic.AddInt64(&completed, 1)
				if progressEvery > 0 && done%int64(progressEvery) == 0 {
					gps := float64(done) / time.Since(start).Seconds()
					if cfg.ProgressLogger != nil {
						cfg.ProgressLogger(int(done), totalGames, gps)
					} else {
						fmt.Fprintf(os.Stderr, "  round-robin: %d/%d games (%.0f g/s) [%d/%d pods]\n",
							done, totalGames, gps, int(done)/max1i(cfg.GamesPerPod), len(pods))
					}
				}
			}
		}()
	}

	go func() {
		for podIdx, pod := range pods {
			for g := 0; g < cfg.GamesPerPod; g++ {
				work <- workItem{
					podIdx:    podIdx,
					gameInPod: g,
					deckIdxs:  pod,
				}
			}
		}
		close(work)
	}()

	go func() {
		wg.Wait()
		close(outcomes)
	}()

	result := aggregateRoundRobin(outcomes, totalGames, cfg.NSeats, allNames)
	result.Duration = time.Since(start)
	if result.Duration.Seconds() > 0 {
		result.GamesPerSecond = float64(result.Games) / result.Duration.Seconds()
	}

	if cfg.ReportPath != "" {
		if err := result.WriteMarkdown(cfg.ReportPath); err != nil {
			return result, fmt.Errorf("tournament: write report: %w", err)
		}
	}

	return result, nil
}

func aggregateRoundRobin(outcomes <-chan rrOutcome, totalGames, nSeats int, commanderNames []string) *TournamentResult {
	r := &TournamentResult{
		NSeats:                       nSeats,
		CommanderNames:               append([]string(nil), commanderNames...),
		WinsByCommander:              make(map[string]int, len(commanderNames)),
		EliminationByCommanderBySlot: make(map[string][]int, len(commanderNames)),
		ParserGapSnippets:            map[string]int{},
		AvgTurnToWin:                 make(map[string]float64, len(commanderNames)),
		MatchupMatrix:                make(map[string]map[string]int, len(commanderNames)),
		MatchupGames:                 make(map[string]map[string]int, len(commanderNames)),
		GamesPlayedByCommander:       make(map[string]int, len(commanderNames)),
	}
	for _, name := range commanderNames {
		r.EliminationByCommanderBySlot[name] = make([]int, nSeats)
		r.MatchupMatrix[name] = make(map[string]int, len(commanderNames))
		r.MatchupGames[name] = make(map[string]int, len(commanderNames))
	}

	elo := NewELORatings(commanderNames)
	ts := trueskill.NewTrueSkillRatings(commanderNames)

	winTurnSum := make(map[string]int)
	winTurnCount := make(map[string]int)
	totalTurns := 0

	for rr := range outcomes {
		outcome := rr.outcome
		globalIdxs := rr.deckIdxs

		if outcome.CrashErr != "" {
			r.Crashes++
			var podNames []string
			for _, gIdx := range globalIdxs {
				if gIdx >= 0 && gIdx < len(commanderNames) {
					podNames = append(podNames, commanderNames[gIdx])
				}
			}
			r.CrashLogs = append(r.CrashLogs, fmt.Sprintf("pod=[%s] game=%d %s",
				strings.Join(podNames, ", "), outcome.GameIdx, outcome.CrashErr))
			continue
		}
		r.Games++
		totalTurns += outcome.Turns
		r.TotalModeChanges += outcome.ModeChanges

		for _, gIdx := range globalIdxs {
			if gIdx >= 0 && gIdx < len(commanderNames) {
				r.GamesPlayedByCommander[commanderNames[gIdx]]++
			}
		}

		winnerName := ""
		if outcome.WinnerCommanderIdx >= 0 && outcome.WinnerCommanderIdx < len(globalIdxs) {
			globalWinner := globalIdxs[outcome.WinnerCommanderIdx]
			if globalWinner >= 0 && globalWinner < len(commanderNames) {
				winnerName = commanderNames[globalWinner]
				r.WinsByCommander[winnerName]++
				winTurnSum[winnerName] += outcome.Turns
				winTurnCount[winnerName]++
			}
		} else {
			r.Draws++
		}

		switch {
		case outcome.Turns <= 5:
			r.TurnDistribution[0]++
		case outcome.Turns <= 10:
			r.TurnDistribution[1]++
		case outcome.Turns <= 20:
			r.TurnDistribution[2]++
		default:
			r.TurnDistribution[3]++
		}

		globalParticipants := make([]string, 0, nSeats)
		for _, gIdx := range globalIdxs {
			if gIdx >= 0 && gIdx < len(commanderNames) {
				globalParticipants = append(globalParticipants, commanderNames[gIdx])
			}
		}
		for i := 0; i < len(globalParticipants); i++ {
			for j := i + 1; j < len(globalParticipants); j++ {
				a, b := globalParticipants[i], globalParticipants[j]
				if a == b {
					continue
				}
				r.MatchupGames[a][b]++
				r.MatchupGames[b][a]++
				if winnerName == a {
					r.MatchupMatrix[a][b]++
				} else if winnerName == b {
					r.MatchupMatrix[b][a]++
				}
			}
		}

		elo.Update(winnerName, globalParticipants)

		// TrueSkill update: convert elimination order to ranks.
		if len(globalIdxs) >= 2 && len(outcome.EliminationOrder) == len(globalIdxs) {
			tsNames := make([]string, 0, len(globalIdxs))
			tsRanks := make([]int, 0, len(globalIdxs))
			for localIdx, gIdx := range globalIdxs {
				if gIdx >= 0 && gIdx < len(commanderNames) && localIdx < len(outcome.EliminationOrder) {
					tsNames = append(tsNames, commanderNames[gIdx])
					slot := outcome.EliminationOrder[localIdx]
					tsRanks = append(tsRanks, (nSeats-1)-slot)
				}
			}
			ts.Update(tsNames, tsRanks)
		}

		for localIdx, slot := range outcome.EliminationOrder {
			if localIdx < 0 || localIdx >= len(globalIdxs) {
				continue
			}
			globalIdx := globalIdxs[localIdx]
			if globalIdx < 0 || globalIdx >= len(commanderNames) || slot < 0 || slot >= nSeats {
				continue
			}
			r.EliminationByCommanderBySlot[commanderNames[globalIdx]][slot]++
		}

		if len(outcome.ParserGapSnippets) > 0 {
			for k, v := range outcome.ParserGapSnippets {
				r.ParserGapSnippets[k] += v
			}
		}

		if outcome.Analysis != nil {
			r.Analyses = append(r.Analyses, outcome.Analysis)
		}
	}

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
	return r
}

// combinations generates all C(n, k) index combinations in
// lexicographic order.
func combinations(n, k int) [][]int {
	var result [][]int
	combo := make([]int, k)
	var gen func(start, depth int)
	gen = func(start, depth int) {
		if depth == k {
			c := make([]int, k)
			copy(c, combo)
			result = append(result, c)
			return
		}
		for i := start; i <= n-(k-depth); i++ {
			combo[depth] = i
			gen(i+1, depth+1)
		}
	}
	gen(0, 0)
	return result
}
