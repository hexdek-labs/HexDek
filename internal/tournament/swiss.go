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

// SwissConfig defines a multi-round Swiss tournament adapted for
// multiplayer pods. Each round forms ceil(NDecks / NSeats) pods by
// pairing decks of similar standing; pods play GamesPerPod games
// each (1 is typical); points roll up across rounds and the final
// TournamentResult carries the cumulative standings.
//
// Pairing rule (greedy top-down, sketched in pairSwissRound):
//
//  1. Sort decks by current Points desc, with deterministic
//     tiebreaks (Wins desc → games asc → seed-derived index asc).
//  2. Walk the sorted list and form pods of NSeats. For each
//     candidate seat we prefer a deck that has NOT yet been a
//     podmate of any deck already seated in the pod this
//     tournament. If no such deck exists in the remaining pool
//     the pairing falls back to the next-best deck — i.e. repeat
//     pairings are tolerated but actively avoided.
//
// This is the multiplayer analogue of Monrad / Dutch Swiss pairing.
// It is intentionally NOT a perfect optimizer — for a Commander
// pod-Swiss with a handful of rounds the greedy heuristic produces
// strong-vs-strong matchups that match what a human TO would draw
// up, without paying the runtime of an exact min-cost matching.
//
// When NDecks % NSeats != 0, the last pod is undersized: the
// remaining (NDecks % NSeats) decks sit out that round and accrue
// a bye. ByePoints (default = PointsPerWin) is awarded to each
// byed deck so a 5-deck 4-seat tournament doesn't punish the deck
// that drew the short straw.
type SwissConfig struct {
	Decks            []*deckparser.TournamentDeck
	NSeats           int
	Rounds           int
	GamesPerPod      int
	Seed             int64
	HatFactories     []HatFactory // parallel to Decks, 0/1/len(Decks)
	Workers          int
	CommanderMode    bool
	AuditEnabled     bool
	AnalyticsEnabled bool
	ReportPath       string
	MaxTurnsPerGame  int
	GameTimeout      time.Duration
	ProgressLogEvery int
	ProgressLogger   func(done, total int, gps float64)

	// PointsPerWin is awarded per game won. Default 3 (matches MTG
	// constructed Swiss point structure).
	PointsPerWin int

	// PointsPerDraw is awarded per drawn game. Default 1.
	PointsPerDraw int

	// ByePoints is awarded to a deck that sits out a round because
	// NDecks doesn't divide cleanly by NSeats. Default = PointsPerWin
	// (the deck is credited a free win, mirroring chess Swiss byes).
	ByePoints int
}

// SwissStanding is one deck's cumulative score across the Swiss
// tournament. Surfaced on TournamentResult.SwissStandings so the
// final ranking can be sorted by points without re-deriving from
// the matchup matrix.
//
// OpponentMatchWinPct is the strength-of-schedule tiebreaker
// (MTG Comprehensive Tournament Rules §16.3.2): the mean of each
// opponent's WinRate over every pod the deck participated in, with
// a per-opponent floor of 0.33 so a single 0-win opponent doesn't
// tank the SoS. Used by sortedStandings as the third sort key
// (after Points and Wins) so two decks tied on Points + Wins are
// ranked by which one faced the harder schedule. Zero for decks
// that played no opponents (round-1 byes before any games).
type SwissStanding struct {
	CommanderName        string  `json:"commander_name"`
	Points               int     `json:"points"`
	Games                int     `json:"games"`
	Wins                 int     `json:"wins"`
	Losses               int     `json:"losses"`
	Draws                int     `json:"draws"`
	Byes                 int     `json:"byes"`
	WinRate              float64 `json:"win_rate"`
	OpponentMatchWinPct  float64 `json:"opponent_match_win_pct"`
}

// SwissOMWFloor is the per-opponent floor applied when computing
// OpponentMatchWinPct. Matches MTG CR §16.3.2: an opponent's
// match-win percentage is treated as max(actual, 0.33) so a single
// blow-out loss against a winless opponent doesn't tank the deck's
// strength-of-schedule disproportionately. The 0.33 figure is the
// MTG-standard for 2-player matches; for multiplayer pods (where
// the expected per-game winrate is 1/NSeats = 0.25 in 4-seat) the
// floor is a slightly generous compensator but matches Magic
// convention so external consumers' mental models hold.
const SwissOMWFloor = 0.33

// RunSwiss executes a Swiss-style pod tournament. See SwissConfig
// for the pairing/scoring rules.
func RunSwiss(cfg SwissConfig) (*TournamentResult, error) {
	if cfg.NSeats < 2 {
		return nil, fmt.Errorf("tournament: NSeats must be >= 2")
	}
	if len(cfg.Decks) < cfg.NSeats {
		return nil, fmt.Errorf("tournament: need at least %d decks, got %d", cfg.NSeats, len(cfg.Decks))
	}
	if cfg.Rounds <= 0 {
		return nil, fmt.Errorf("tournament: Rounds must be > 0")
	}
	if cfg.GamesPerPod <= 0 {
		cfg.GamesPerPod = 1
	}
	if cfg.PointsPerWin <= 0 {
		cfg.PointsPerWin = 3
	}
	if cfg.PointsPerDraw < 0 {
		cfg.PointsPerDraw = 1
	}
	if cfg.ByePoints <= 0 {
		cfg.ByePoints = cfg.PointsPerWin
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

	// Per-deck running tally. Index = deck index in cfg.Decks.
	standings := make([]SwissStanding, nDecks)
	for i := range standings {
		standings[i] = SwissStanding{CommanderName: commanderNames[i]}
	}

	// pastPairings[i] is the set of deck indices that deck i has
	// already shared a pod with. Used by pairSwissRound to bias the
	// next pairing AWAY from repeats.
	pastPairings := make([]map[int]struct{}, nDecks)
	for i := range pastPairings {
		pastPairings[i] = make(map[int]struct{})
	}

	podsPerRound := nDecks / cfg.NSeats
	gamesPerRound := podsPerRound * cfg.GamesPerPod
	totalGames := gamesPerRound * cfg.Rounds
	progressEvery := cfg.ProgressLogEvery
	if progressEvery == 0 {
		progressEvery = 1000
		if totalGames/20 > progressEvery {
			progressEvery = totalGames / 20
		}
	}

	fmt.Fprintf(os.Stderr, "  swiss: %d decks, %d rounds × %d pods × %d games = %d total games\n",
		nDecks, cfg.Rounds, podsPerRound, cfg.GamesPerPod, totalGames)

	// Aggregate result accumulators (shared across rounds so the final
	// TournamentResult reflects ALL games, not just the last round).
	r := newSwissAggResult(commanderNames, cfg.NSeats)
	r.Mode = ModeSwiss
	elo := NewELORatings(commanderNames)
	ts := trueskill.NewTrueSkillRatings(commanderNames)

	winTurnSum := make(map[string]int)
	winTurnCount := make(map[string]int)
	totalTurns := 0

	var completed int64
	start := time.Now()

	for round := 0; round < cfg.Rounds; round++ {
		pods, byes := pairSwissRound(standings, pastPairings, cfg.NSeats, cfg.Seed+int64(round)*1_000_003)

		// Update past-pairings + bye standings BEFORE running games so
		// successive rounds in the same tournament see the new history.
		for _, pod := range pods {
			for i := 0; i < len(pod); i++ {
				for j := i + 1; j < len(pod); j++ {
					pastPairings[pod[i]][pod[j]] = struct{}{}
					pastPairings[pod[j]][pod[i]] = struct{}{}
				}
			}
		}
		for _, byeIdx := range byes {
			standings[byeIdx].Points += cfg.ByePoints
			standings[byeIdx].Byes++
		}

		runRoundOutcomes(swissRoundParams{
			cfg:           cfg,
			pods:          pods,
			round:         round,
			workers:       workers,
			maxTurns:      maxTurns,
			gameTimeout:   gameTimeout,
			hatPerDeck:    hatPerDeck,
			standings:     standings,
			result:        r,
			elo:           elo,
			ts:            ts,
			commanderNames: commanderNames,
			winTurnSum:    winTurnSum,
			winTurnCount:  winTurnCount,
			totalTurns:    &totalTurns,
			completed:     &completed,
			start:         start,
			totalGames:    totalGames,
			progressEvery: progressEvery,
		})

		// Per-round telemetry capture. We snapshot AFTER the round's
		// games have updated elo/ts and AFTER standings have absorbed
		// wins/losses + byes. OMW% is computed from the partial
		// standings + pastPairings so the trajectory reflects each
		// round's state independently. See docs/tournament-telemetry-
		// schema.md.
		snap := captureSwissRoundSnapshot(round+1, commanderNames, standings, pastPairings, elo, ts)
		r.RoundSnapshots = append(r.RoundSnapshots, snap)
	}

	// Finalize.
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

	for i := range standings {
		if standings[i].Games > 0 {
			standings[i].WinRate = float64(standings[i].Wins) / float64(standings[i].Games)
		}
	}
	// OMW% is computed AFTER every deck has its final WinRate so the
	// strength-of-schedule reflects the whole tournament, not a per-
	// round snapshot. pastPairings is the source of truth for "who
	// did D play against."
	computeSwissOMW(standings, pastPairings)
	r.SwissStandings = sortedStandings(standings)
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

// pairSwissRound builds the round's pods by greedy top-down pairing.
// Returns the pod assignments as slices of deck indices PLUS the
// indices that sit out (bye) when NDecks % NSeats != 0.
//
// Algorithm:
//   1. Sort decks by Points desc, Wins desc, Games asc, then a
//      seed-derived shuffle key for deterministic tiebreak.
//   2. Walk down the sorted list pulling decks into pods of size
//      NSeats. For each candidate after the first seat, prefer a
//      deck that hasn't been a podmate of any already-seated deck.
//      If no such deck exists in the unpaired pool, fall back to
//      the next-best by sort order — we'd rather repeat than stall.
//   3. Leftover decks (size < NSeats) go to the bye pool. To keep
//      the bye distribution balanced across rounds we prefer to
//      bye the deck with the FEWEST prior byes; tiebreak by lowest
//      Points (the deck most likely to benefit from a free win).
func pairSwissRound(standings []SwissStanding, pastPairings []map[int]struct{}, nSeats int, roundSeed int64) ([][]int, []int) {
	type sortable struct {
		idx     int
		shuffle uint64
	}
	order := make([]sortable, len(standings))
	for i := range order {
		order[i] = sortable{
			idx: i,
			// Splitmix64-style hash on (roundSeed XOR idx) gives a
			// deterministic, well-distributed tiebreak that varies
			// per round so the same tied group doesn't always land in
			// the same pod.
			shuffle: splitmix64(uint64(roundSeed) ^ uint64(i*2654435761)),
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		ai, aj := standings[order[i].idx], standings[order[j].idx]
		if ai.Points != aj.Points {
			return ai.Points > aj.Points
		}
		if ai.Wins != aj.Wins {
			return ai.Wins > aj.Wins
		}
		// Bye-balancing tiebreak: within tied scores, the deck that
		// has byed MORE often sorts EARLIER — so it gets picked into
		// a pod sooner and the next bye falls on someone else. Keeps
		// the bye distribution from concentrating on whichever deck
		// happens to land last in the shuffle order.
		if ai.Byes != aj.Byes {
			return ai.Byes > aj.Byes
		}
		if ai.Games != aj.Games {
			return ai.Games < aj.Games
		}
		return order[i].shuffle < order[j].shuffle
	})

	available := make([]int, len(order))
	for i, s := range order {
		available[i] = s.idx
	}

	var pods [][]int
	for len(available) >= nSeats {
		pod := make([]int, 0, nSeats)
		// First seat = top of the remaining list.
		pod = append(pod, available[0])
		available = available[1:]

		// Subsequent seats: prefer non-repeated podmates.
		for len(pod) < nSeats {
			pick := -1
			for i, candidate := range available {
				if !sharesAnyPastPod(candidate, pod, pastPairings) {
					pick = i
					break
				}
			}
			if pick < 0 {
				// All remaining decks have repeats. Take the top of
				// the remaining pool rather than stalling.
				pick = 0
			}
			pod = append(pod, available[pick])
			available = append(available[:pick], available[pick+1:]...)
		}
		pods = append(pods, pod)
	}

	// Whatever remains is the bye set. Sort by (Byes asc, Points asc)
	// so future-rounds compensation logic is stable.
	byes := append([]int(nil), available...)
	sort.SliceStable(byes, func(i, j int) bool {
		bi, bj := standings[byes[i]], standings[byes[j]]
		if bi.Byes != bj.Byes {
			return bi.Byes < bj.Byes
		}
		return bi.Points < bj.Points
	})
	return pods, byes
}

func sharesAnyPastPod(candidate int, pod []int, pastPairings []map[int]struct{}) bool {
	if candidate < 0 || candidate >= len(pastPairings) {
		return false
	}
	for _, seated := range pod {
		if _, ok := pastPairings[candidate][seated]; ok {
			return true
		}
	}
	return false
}

// splitmix64 is a fast deterministic hash for the round-tiebreak key.
// Standard SplitMix64 constants.
func splitmix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// captureSwissRoundSnapshot freezes the per-round state needed by
// ComputeTelemetry's trajectory plotting. Called at end-of-round
// AFTER elo + ts have absorbed the round's outcomes, so the
// snapshot reflects "where ratings stood after round N completed."
// OMW% is computed against the partial standings + pastPairings
// for the running trajectory.
func captureSwissRoundSnapshot(round int, commanderNames []string, standings []SwissStanding, pastPairings []map[int]struct{}, elo *ELORatings, ts *trueskill.TrueSkillRatings) RoundSnapshot {
	snap := RoundSnapshot{
		Round:           round,
		Ratings:         make(map[string]RatingSnapshot, len(commanderNames)),
		CumulativeWins:  make(map[string]int, len(commanderNames)),
		CumulativeGames: make(map[string]int, len(commanderNames)),
		OMW:             make(map[string]float64, len(commanderNames)),
	}
	// Pre-update WinRate from current standings.Wins/Games for OMW%
	// computation against the partial state. Don't mutate standings —
	// take a copy that mirrors finalize-time computeSwissOMW.
	working := make([]SwissStanding, len(standings))
	copy(working, standings)
	for i := range working {
		if working[i].Games > 0 {
			working[i].WinRate = float64(working[i].Wins) / float64(working[i].Games)
		}
	}
	computeSwissOMW(working, pastPairings)
	for i, name := range commanderNames {
		snap.CumulativeWins[name] = working[i].Wins
		snap.CumulativeGames[name] = working[i].Games
		snap.OMW[name] = working[i].OpponentMatchWinPct
		rs := RatingSnapshot{}
		if elo != nil {
			rs.ELO = elo.Rating[name]
		}
		if ts != nil {
			if r, ok := ts.Ratings[name]; ok {
				rs.Mu = r.Mu
				rs.Sigma = r.Sigma
			}
		}
		snap.Ratings[name] = rs
	}
	return snap
}

// computeSwissOMW fills standings[i].OpponentMatchWinPct from the
// pastPairings adjacency map and each deck's WinRate. Per MTG CR
// §16.3.2 every opponent's match-win % is floored at SwissOMWFloor
// (0.33) before being averaged so a single blow-out loss against a
// winless opponent doesn't tank the strength-of-schedule.
//
// Returns the (mutated) standings for chaining convenience. Decks
// with no recorded opponents (e.g. round-1 byes before any games
// played) get OMW = 0 — sortedStandings then ranks them BELOW any
// peer with real opponents at the same Points + Wins, which is the
// correct outcome since a deck that hasn't faced anyone has no
// strength-of-schedule signal.
func computeSwissOMW(standings []SwissStanding, pastPairings []map[int]struct{}) []SwissStanding {
	for i := range standings {
		opps := pastPairings[i]
		if len(opps) == 0 {
			continue
		}
		sum := 0.0
		count := 0
		for oppIdx := range opps {
			if oppIdx < 0 || oppIdx >= len(standings) {
				continue
			}
			oppRate := standings[oppIdx].WinRate
			if oppRate < SwissOMWFloor {
				oppRate = SwissOMWFloor
			}
			sum += oppRate
			count++
		}
		if count > 0 {
			standings[i].OpponentMatchWinPct = sum / float64(count)
		}
	}
	return standings
}

// sortedStandings ranks Swiss participants for the final standings
// table. Sort keys, in MTG CR §16.3 order:
//
//   1. Points desc — the primary score.
//   2. Wins desc — secondary tiebreaker; surfaces the deck that
//      actually won games over one that scored via byes.
//   3. OpponentMatchWinPct desc — strength-of-schedule. The
//      canonical Magic Swiss tiebreaker. Without it, a deck with a
//      lucky pod draw outranks one that beat tougher opponents at
//      the same Points + Wins. This is the entire reason Swiss as a
//      format exists; falling back to alphabetical was an explicit
//      fairness gap, closed in the r60 pool-fairness pass.
//   4. WinRate desc — game-win % fallback (analogous to MTG's GW%
//      third tiebreaker; collapses to a no-op for our model where
//      WinRate == Wins/Games already correlates with Wins, but
//      preserves the right behavior if Games differ across decks
//      due to byes / forfeits).
//   5. CommanderName asc — deterministic final tiebreak. Real
//      ties past four levels are statistically near-zero, but a
//      stable last-resort keeps output reproducible across runs.
func sortedStandings(s []SwissStanding) []SwissStanding {
	out := make([]SwissStanding, len(s))
	copy(out, s)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Points != out[j].Points {
			return out[i].Points > out[j].Points
		}
		if out[i].Wins != out[j].Wins {
			return out[i].Wins > out[j].Wins
		}
		if out[i].OpponentMatchWinPct != out[j].OpponentMatchWinPct {
			return out[i].OpponentMatchWinPct > out[j].OpponentMatchWinPct
		}
		if out[i].WinRate != out[j].WinRate {
			return out[i].WinRate > out[j].WinRate
		}
		return out[i].CommanderName < out[j].CommanderName
	})
	return out
}

// swissRoundParams bundles the per-round runtime state so
// runRoundOutcomes' signature stays manageable. Mutates standings
// + the various aggregate accumulators.
type swissRoundParams struct {
	cfg           SwissConfig
	pods          [][]int
	round         int
	workers       int
	maxTurns      int
	gameTimeout   time.Duration
	hatPerDeck    []HatFactory
	standings     []SwissStanding
	result        *TournamentResult
	elo           *ELORatings
	ts            *trueskill.TrueSkillRatings
	commanderNames []string
	winTurnSum    map[string]int
	winTurnCount  map[string]int
	totalTurns    *int
	completed     *int64
	start         time.Time
	totalGames    int
	progressEvery int
}

func runRoundOutcomes(p swissRoundParams) {
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
						p.cfg.ProgressLogger(int(done), p.totalGames, gps)
					} else {
						fmt.Fprintf(os.Stderr, "  swiss: %d/%d games (%.0f g/s) [round %d/%d]\n",
							done, p.totalGames, gps, p.round+1, p.cfg.Rounds)
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
		applySwissOutcome(p, rr)
	}
}

func applySwissOutcome(p swissRoundParams, rr rrOutcome) {
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
		p.standings[gIdx].Games++
	}

	winnerName := ""
	winnerGIdx := -1
	if outcome.WinnerCommanderIdx >= 0 && outcome.WinnerCommanderIdx < len(globalIdxs) {
		winnerGIdx = globalIdxs[outcome.WinnerCommanderIdx]
		if winnerGIdx >= 0 && winnerGIdx < len(p.commanderNames) {
			winnerName = p.commanderNames[winnerGIdx]
			p.result.WinsByCommander[winnerName]++
			p.winTurnSum[winnerName] += outcome.Turns
			p.winTurnCount[winnerName]++
		}
	} else {
		p.result.Draws++
	}

	for _, gIdx := range globalIdxs {
		if gIdx == winnerGIdx {
			p.standings[gIdx].Wins++
			p.standings[gIdx].Points += p.cfg.PointsPerWin
		} else if winnerGIdx < 0 {
			p.standings[gIdx].Draws++
			p.standings[gIdx].Points += p.cfg.PointsPerDraw
		} else {
			p.standings[gIdx].Losses++
		}
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
}

// newSwissAggResult builds the shared aggregator skeleton used by
// Swiss, double-elimination, and balanced-pool runs. It stamps
// SchemaVersion but leaves Mode empty — the caller (RunSwiss /
// RunDoubleElimination / RunBalancedPool) is responsible for setting
// the per-mode marker post-construction.
func newSwissAggResult(commanderNames []string, nSeats int) *TournamentResult {
	r := &TournamentResult{
		SchemaVersion:                SchemaVersion,
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
	return r
}

// SupportedBracketStyles lists the bracket styles RunSwiss /
// RunRoundRobin / Run support today, in a form the CLI and docs
// can iterate over. Exported so tooling (hexdek-tournament --help,
// API meta endpoints) can stay in sync without hard-coding the
// list.
func SupportedBracketStyles() []string {
	return []string{
		"rotate",        // Run() default: rotate the first NSeats decks each game
		"pool",          // Run() with PoolMode=true: random pod per game
		"lazy-pool",     // Run() with LazyPool=true: pool with on-demand deck loading
		"round-robin",   // RunRoundRobin: every C(NDecks, NSeats) combination
		"swiss",         // RunSwiss: pod-Swiss with greedy top-down pairing
		"double-elim",   // RunDoubleElimination: two-strikes-and-out pod elimination
		"balanced-pool", // RunBalancedPool: snake-draft seeding for equal-strength pods
	}
}
