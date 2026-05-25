package tournament

import (
	"fmt"
	"math"
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

// BalancedPoolConfig defines a "balanced-pool" tournament — the
// alternative to PoolMode's uniform-random pod sampling. When you
// have prior signal about deck strength (ELO, TrueSkill μ, Freya
// power tier, a hex_rating snapshot from the previous gauntlet)
// you usually do NOT want random pods: random pods give you "all
// four cEDH decks in one pod, four jank decks in another" runs
// that produce wide winrate variance and aren't representative of
// the deck under test.
//
// Balanced-pool snake-draft seeding solves this. Sort decks by
// strength desc; deal them into pods like a fantasy-football
// snake draft (pod 0..N-1 then pod N-1..0 alternating). Each pod
// ends up with one strong, one mid-strong, one mid-weak, and one
// weak deck — total pod strength is approximately equal across
// pods, the maximum mismatch between any two pods is bounded by
// O(maxStrength - minStrength).
//
// Strengths is parallel to Decks; when empty, decks are treated
// as uniform strength (snake-draft on deck index, equivalent to
// fixed-order pod assignment).
//
// Requires len(Decks) % NSeats == 0 — the snake draft has no
// graceful undersize-pod story (any leftover pod would skew). For
// uneven divisions, slice the deck pool down to a multiple of
// NSeats before calling, or use Swiss/DE which handle byes
// natively.
type BalancedPoolConfig struct {
	Decks            []*deckparser.TournamentDeck
	NSeats           int
	GamesPerPod      int
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

	// Strengths is the per-deck strength score (parallel to Decks).
	// Higher = stronger. When nil or zero-length, decks are treated
	// as equal strength → seeding falls through to deck-index order.
	// When non-empty, len(Strengths) MUST equal len(Decks).
	Strengths []float64
}

// BalancedPod records one pod assignment from snake-draft seeding.
// DeckIndices is in seat order (the order they'll be dealt into the
// pod's seats). CommanderNames mirrors DeckIndices for caller-side
// rendering without re-resolving against Decks. PodStrength is the
// sum of the seats' strength scores — used by tests + reports to
// confirm pods are within a tight strength window.
type BalancedPod struct {
	DeckIndices    []int    `json:"deck_indices"`
	CommanderNames []string `json:"commander_names"`
	PodStrength    float64  `json:"pod_strength"`
}

// BalancedPodSeeding is the pure helper. Returns one slice of
// DECK INDICES per pod, in seat order. Snake-draft over the
// strength ranking:
//
//   Round 0 forward:  pod 0, pod 1, ..., pod N-1
//   Round 1 reverse:  pod N-1, pod N-2, ..., pod 0
//   Round 2 forward:  pod 0, pod 1, ..., pod N-1
//   ...
//
// Each pod receives exactly NSeats decks (one per round). For NDecks
// = nPods * NSeats this divides cleanly; the function returns an
// error otherwise — callers who want graceful undersize handling
// should slice the input first.
//
// strengths may be nil/empty for uniform strength; otherwise must
// match len of nDecks.
func BalancedPodSeeding(nDecks, nSeats int, strengths []float64) ([][]int, error) {
	if nSeats < 2 {
		return nil, fmt.Errorf("balanced pool: NSeats must be >= 2")
	}
	if nDecks < nSeats {
		return nil, fmt.Errorf("balanced pool: need at least %d decks, got %d", nSeats, nDecks)
	}
	if nDecks%nSeats != 0 {
		return nil, fmt.Errorf("balanced pool: NDecks (%d) must divide cleanly by NSeats (%d) for snake-draft seeding", nDecks, nSeats)
	}
	if len(strengths) != 0 && len(strengths) != nDecks {
		return nil, fmt.Errorf("balanced pool: Strengths length %d != NDecks %d", len(strengths), nDecks)
	}
	nPods := nDecks / nSeats

	// Sort deck indices by strength descending. Tiebreak on index
	// asc for determinism. When strengths is empty everyone is tied
	// and the sort collapses to index order — the snake-draft then
	// just distributes decks in fixed-order rotation, which is
	// still a useful invariant to pin.
	indices := make([]int, nDecks)
	for i := range indices {
		indices[i] = i
	}
	if len(strengths) > 0 {
		sort.SliceStable(indices, func(i, j int) bool {
			si, sj := strengths[indices[i]], strengths[indices[j]]
			if si != sj {
				return si > sj
			}
			return indices[i] < indices[j]
		})
	}

	pods := make([][]int, nPods)
	for i := range pods {
		pods[i] = make([]int, 0, nSeats)
	}
	for round := 0; round < nSeats; round++ {
		// Round 0/2/4/... forward; 1/3/5/... reverse. Pure snake.
		forward := round%2 == 0
		for slot := 0; slot < nPods; slot++ {
			deckIdx := round*nPods + slot
			var podIdx int
			if forward {
				podIdx = slot
			} else {
				podIdx = nPods - 1 - slot
			}
			pods[podIdx] = append(pods[podIdx], indices[deckIdx])
		}
	}
	return pods, nil
}

// BalancedPodStrengthSpread reports the gap between the strongest
// and weakest pod when seated via BalancedPodSeeding. Useful for
// tests + tournament reports — confirms the seeding actually
// equalized pod strength. Returns 0 when strengths is empty.
func BalancedPodStrengthSpread(pods [][]int, strengths []float64) float64 {
	if len(strengths) == 0 || len(pods) == 0 {
		return 0
	}
	minSum, maxSum := math.Inf(1), math.Inf(-1)
	for _, pod := range pods {
		sum := 0.0
		for _, idx := range pod {
			if idx >= 0 && idx < len(strengths) {
				sum += strengths[idx]
			}
		}
		if sum < minSum {
			minSum = sum
		}
		if sum > maxSum {
			maxSum = sum
		}
	}
	return maxSum - minSum
}

// RunBalancedPool executes a balanced-pool tournament. See
// BalancedPoolConfig and BalancedPodSeeding for the seeding rules.
// Each pod plays GamesPerPod games; the resulting TournamentResult
// carries a BalancedPodPlan with the snake-draft layout so callers
// can render which pod each deck was assigned to.
func RunBalancedPool(cfg BalancedPoolConfig) (*TournamentResult, error) {
	if cfg.NSeats < 2 {
		return nil, fmt.Errorf("tournament: NSeats must be >= 2")
	}
	if cfg.GamesPerPod <= 0 {
		cfg.GamesPerPod = 1
	}
	for i, d := range cfg.Decks {
		if d == nil {
			return nil, fmt.Errorf("tournament: decks[%d] is nil", i)
		}
		if len(d.CommanderCards) == 0 {
			return nil, fmt.Errorf("tournament: decks[%d] has no commander", i)
		}
	}

	pods, err := BalancedPodSeeding(len(cfg.Decks), cfg.NSeats, cfg.Strengths)
	if err != nil {
		return nil, err
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

	totalGames := len(pods) * cfg.GamesPerPod
	progressEvery := cfg.ProgressLogEvery
	if progressEvery == 0 {
		progressEvery = 1000
		if totalGames/20 > progressEvery {
			progressEvery = totalGames / 20
		}
	}

	fmt.Fprintf(os.Stderr, "  balanced-pool: %d decks → %d pods × %d games = %d total games\n",
		nDecks, len(pods), cfg.GamesPerPod, totalGames)

	r := newSwissAggResult(commanderNames, cfg.NSeats)
	elo := NewELORatings(commanderNames)
	ts := trueskill.NewTrueSkillRatings(commanderNames)

	winTurnSum := make(map[string]int)
	winTurnCount := make(map[string]int)
	totalTurns := 0

	var completed int64
	start := time.Now()

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
					cfg.Seed+int64(item.podIdx)*10_007, maxTurns, gameTimeout,
					cfg.CommanderMode, cfg.AuditEnabled, cfg.AnalyticsEnabled,
					contractParams{},
				)
				outcomes <- rrOutcome{outcome: outcome, deckIdxs: item.deckIdxs}

				done := atomic.AddInt64(&completed, 1)
				if progressEvery > 0 && done%int64(progressEvery) == 0 {
					gps := float64(done) / time.Since(start).Seconds()
					if cfg.ProgressLogger != nil {
						cfg.ProgressLogger(int(done), totalGames, gps)
					} else {
						fmt.Fprintf(os.Stderr, "  balanced-pool: %d/%d games (%.0f g/s)\n",
							done, totalGames, gps)
					}
				}
			}
		}()
	}

	go func() {
		for podIdx, pod := range pods {
			for g := 0; g < cfg.GamesPerPod; g++ {
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
		applyBalancedPoolOutcome(applyParams{
			result:         r,
			elo:            elo,
			ts:             ts,
			commanderNames: commanderNames,
			nSeats:         cfg.NSeats,
			winTurnSum:     winTurnSum,
			winTurnCount:   winTurnCount,
			totalTurns:     &totalTurns,
		}, rr)
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

	// Pod plan: render for reporting / introspection.
	plan := make([]BalancedPod, len(pods))
	for i, pod := range pods {
		bp := BalancedPod{
			DeckIndices:    append([]int(nil), pod...),
			CommanderNames: make([]string, len(pod)),
		}
		for j, idx := range pod {
			bp.CommanderNames[j] = commanderNames[idx]
			if idx < len(cfg.Strengths) {
				bp.PodStrength += cfg.Strengths[idx]
			}
		}
		plan[i] = bp
	}
	r.BalancedPodPlan = plan

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

// applyParams + applyBalancedPoolOutcome are the per-outcome
// aggregator used by RunBalancedPool. Same shape as the DE/Swiss
// outcome handlers but without the per-deck loss/standing book-
// keeping (balanced-pool ranks by aggregate winrate, not by
// elimination tier). Re-used outcome processing factored out so
// future runners can call it directly.
type applyParams struct {
	result         *TournamentResult
	elo            *ELORatings
	ts             *trueskill.TrueSkillRatings
	commanderNames []string
	nSeats         int
	winTurnSum     map[string]int
	winTurnCount   map[string]int
	totalTurns     *int
}

func applyBalancedPoolOutcome(p applyParams, rr rrOutcome) {
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
	if outcome.WinnerCommanderIdx >= 0 && outcome.WinnerCommanderIdx < len(globalIdxs) {
		globalWinner := globalIdxs[outcome.WinnerCommanderIdx]
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

	globalParticipants := make([]string, 0, p.nSeats)
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
				tsRanks = append(tsRanks, (p.nSeats-1)-slot)
			}
		}
		p.ts.Update(tsNames, tsRanks)
	}

	for localIdx, slot := range outcome.EliminationOrder {
		if localIdx < 0 || localIdx >= len(globalIdxs) {
			continue
		}
		gIdx := globalIdxs[localIdx]
		if gIdx < 0 || gIdx >= len(p.commanderNames) || slot < 0 || slot >= p.nSeats {
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
}
