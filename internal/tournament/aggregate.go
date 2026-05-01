package tournament

import (
	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/trueskill"
)

// aggregate consumes the outcomes channel and produces the final
// TournamentResult. Called from the main goroutine.
//
// Aggregation un-rotates seat indices back to commander indices so
// downstream readers see "commander X has N wins", not "seat position Y
// has N wins".
func aggregate(outcomes <-chan GameOutcome, nGames, nSeats int, commanderNames []string) *TournamentResult {
	r := &TournamentResult{
		NSeats:                       nSeats,
		CommanderNames:               append([]string(nil), commanderNames...),
		WinsByCommander:              make(map[string]int, nSeats),
		EliminationByCommanderBySlot: make(map[string][]int, nSeats),
		ParserGapSnippets:            map[string]int{},
		AvgTurnToWin:                 make(map[string]float64, nSeats),
		MatchupMatrix:                make(map[string]map[string]int, nSeats),
		MatchupGames:                 make(map[string]map[string]int, nSeats),
	}
	for _, name := range commanderNames {
		r.EliminationByCommanderBySlot[name] = make([]int, nSeats)
		r.MatchupMatrix[name] = make(map[string]int, nSeats)
		r.MatchupGames[name] = make(map[string]int, nSeats)
	}

	// ELO tracker.
	elo := NewELORatings(commanderNames)

	// TrueSkill tracker.
	ts := trueskill.NewTrueSkillRatings(commanderNames)

	// Accumulators for avg turn to win.
	winTurnSum := make(map[string]int, nSeats)
	winTurnCount := make(map[string]int, nSeats)

	totalTurns := 0
	seen := 0
	for outcome := range outcomes {
		seen++
		if outcome.CrashErr != "" {
			r.Crashes++
			r.CrashLogs = append(r.CrashLogs, outcome.CrashErr)
			continue
		}
		r.Games++
		totalTurns += outcome.Turns
		r.TotalModeChanges += outcome.ModeChanges
		r.TotalConcessions += outcome.Concessions
		if len(outcome.ConcessionRecords) > 0 {
			r.ConcessionRecords = append(r.ConcessionRecords, outcome.ConcessionRecords...)
		}

		winnerName := ""
		if outcome.WinnerCommanderIdx >= 0 && outcome.WinnerCommanderIdx < len(commanderNames) {
			winnerName = commanderNames[outcome.WinnerCommanderIdx]
			r.WinsByCommander[winnerName]++
			winTurnSum[winnerName] += outcome.Turns
			winTurnCount[winnerName]++
		} else {
			r.Draws++
		}

		// Turn-length distribution: [0]=1-5, [1]=6-10, [2]=11-20, [3]=21+
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

		// Matchup matrix: for each pair of participants, record co-
		// occurrence and attribute the win to the winner.
		participants := outcome.ParticipantCommanderIdxs
		for i := 0; i < len(participants); i++ {
			idxA := participants[i]
			if idxA < 0 || idxA >= len(commanderNames) {
				continue
			}
			nameA := commanderNames[idxA]
			for j := i + 1; j < len(participants); j++ {
				idxB := participants[j]
				if idxB < 0 || idxB >= len(commanderNames) {
					continue
				}
				nameB := commanderNames[idxB]
				if nameA == nameB {
					continue
				}
				r.MatchupGames[nameA][nameB]++
				r.MatchupGames[nameB][nameA]++
				if winnerName == nameA {
					r.MatchupMatrix[nameA][nameB]++
				} else if winnerName == nameB {
					r.MatchupMatrix[nameB][nameA]++
				}
			}
		}

		// ELO update.
		var pNames []string
		for _, idx := range participants {
			if idx >= 0 && idx < len(commanderNames) {
				pNames = append(pNames, commanderNames[idx])
			}
		}
		elo.Update(winnerName, pNames)

		// TrueSkill update: convert elimination order to ranks.
		// EliminationOrder: slot 0 = first out, slot N-1 = winner.
		// TrueSkill ranks: 0 = winner (best), N-1 = last (worst).
		if len(participants) >= 2 && len(outcome.EliminationOrder) == len(participants) {
			tsNames := make([]string, 0, len(participants))
			tsRanks := make([]int, 0, len(participants))
			for i, idx := range participants {
				if idx >= 0 && idx < len(commanderNames) {
					tsNames = append(tsNames, commanderNames[idx])
					slot := outcome.EliminationOrder[i]
					tsRanks = append(tsRanks, (nSeats-1)-slot)
				}
			}
			ts.Update(tsNames, tsRanks)
		}

		// Elimination bookkeeping.
		for origIdx, slot := range outcome.EliminationOrder {
			if origIdx < 0 || origIdx >= len(commanderNames) {
				continue
			}
			if slot < 0 || slot >= nSeats {
				continue
			}
			name := commanderNames[origIdx]
			r.EliminationByCommanderBySlot[name][slot]++
		}

		if len(outcome.ParserGapSnippets) > 0 {
			for k, v := range outcome.ParserGapSnippets {
				r.ParserGapSnippets[k] += v
			}
		}

		// Collect per-game analytics.
		if outcome.Analysis != nil {
			r.Analyses = append(r.Analyses, outcome.Analysis)
		}

		// Collect kill records.
		if len(outcome.KillRecords) > 0 {
			r.KillRecords = append(r.KillRecords, outcome.KillRecords...)
		}
	}

	if r.Games > 0 {
		r.AvgTurns = float64(totalTurns) / float64(r.Games)
	}

	// Compute avg turn to win per commander.
	for name, count := range winTurnCount {
		if count > 0 {
			r.AvgTurnToWin[name] = float64(winTurnSum[name]) / float64(count)
		}
	}

	// Compute analytics aggregates.
	if len(r.Analyses) > 0 {
		r.CardRankings = analytics.RankCards(r.Analyses)
		r.MatchupDetails = analytics.BuildMatchupDetails(r.Analyses)
	}

	r.ELO = elo.Snapshot()
	r.TrueSkill = ts.Snapshot()

	_ = nGames // seen should == nGames; we don't enforce
	return r
}
