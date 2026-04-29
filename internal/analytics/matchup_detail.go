package analytics

import "sort"

// BuildMatchupDetails produces per-matchup deep stats from a set of
// game analyses. It groups games by commander pairs and computes win
// rates, win condition breakdowns, and identifies key cards for each
// side of each matchup.
func BuildMatchupDetails(analyses []*GameAnalysis) []MatchupDetail {
	if len(analyses) == 0 {
		return nil
	}

	// Key: "deckA|deckB" (alphabetically sorted pair).
	type matchupAccum struct {
		deck1, deck2    string
		deck1Wins       int
		deck2Wins       int
		totalGames      int
		deck1TurnSum    int
		deck1TurnCount  int
		deck2TurnSum    int
		deck2TurnCount  int
		deck1WinsByType map[string]int
		deck2WinsByType map[string]int
		// Card frequency in wins.
		deck1WinCards map[string]int
		deck2WinCards map[string]int
	}

	matchups := make(map[string]*matchupAccum)

	for _, ga := range analyses {
		if ga == nil {
			continue
		}

		// Extract all commander names present in this game.
		names := make([]string, len(ga.Players))
		for i := range ga.Players {
			names[i] = ga.Players[i].CommanderName
		}

		// For each pair of commanders in this game, record the result.
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				a, b := names[i], names[j]
				if a == b {
					continue
				}
				// Canonical ordering: alphabetical.
				if a > b {
					a, b = b, a
				}
				key := a + "|" + b

				acc, ok := matchups[key]
				if !ok {
					acc = &matchupAccum{
						deck1:           a,
						deck2:           b,
						deck1WinsByType: make(map[string]int),
						deck2WinsByType: make(map[string]int),
						deck1WinCards:   make(map[string]int),
						deck2WinCards:   make(map[string]int),
					}
					matchups[key] = acc
				}
				acc.totalGames++

				// Who won?
				if ga.WinnerSeat >= 0 && ga.WinnerSeat < len(ga.Players) {
					winnerName := ga.Players[ga.WinnerSeat].CommanderName
					if winnerName == a {
						acc.deck1Wins++
						acc.deck1TurnSum += ga.TotalTurns
						acc.deck1TurnCount++
						acc.deck1WinsByType[ga.WinCondition]++
						// Record cards from the winning player.
						for _, cp := range ga.Players[ga.WinnerSeat].CardsPlayed {
							if cp.TurnCast > 0 {
								acc.deck1WinCards[cp.Name]++
							}
						}
					} else if winnerName == b {
						acc.deck2Wins++
						acc.deck2TurnSum += ga.TotalTurns
						acc.deck2TurnCount++
						acc.deck2WinsByType[ga.WinCondition]++
						for _, cp := range ga.Players[ga.WinnerSeat].CardsPlayed {
							if cp.TurnCast > 0 {
								acc.deck2WinCards[cp.Name]++
							}
						}
					}
				}
			}
		}
	}

	// Convert to MatchupDetail slice.
	details := make([]MatchupDetail, 0, len(matchups))
	for _, acc := range matchups {
		md := MatchupDetail{
			Deck1:           acc.deck1,
			Deck2:           acc.deck2,
			Deck1Wins:       acc.deck1Wins,
			Deck2Wins:       acc.deck2Wins,
			TotalGames:      acc.totalGames,
			Deck1WinsByType: acc.deck1WinsByType,
			Deck2WinsByType: acc.deck2WinsByType,
		}
		if acc.totalGames > 0 {
			md.Deck1WinRate = float64(acc.deck1Wins) / float64(acc.totalGames)
		}
		if acc.deck1TurnCount > 0 {
			md.AvgTurnToWin1 = float64(acc.deck1TurnSum) / float64(acc.deck1TurnCount)
		}
		if acc.deck2TurnCount > 0 {
			md.AvgTurnToWin2 = float64(acc.deck2TurnSum) / float64(acc.deck2TurnCount)
		}
		md.KeyCards1 = topNCards(acc.deck1WinCards, 5)
		md.KeyCards2 = topNCards(acc.deck2WinCards, 5)

		details = append(details, md)
	}

	// Sort by total games desc.
	sort.SliceStable(details, func(i, j int) bool {
		return details[i].TotalGames > details[j].TotalGames
	})

	return details
}

// topNCards returns the top N most frequently appearing card names.
func topNCards(m map[string]int, n int) []string {
	type kv struct {
		name  string
		count int
	}
	ranked := make([]kv, 0, len(m))
	for k, v := range m {
		ranked = append(ranked, kv{k, v})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].count > ranked[j].count
	})
	if len(ranked) > n {
		ranked = ranked[:n]
	}
	result := make([]string, len(ranked))
	for i, r := range ranked {
		result[i] = r.name
	}
	return result
}
