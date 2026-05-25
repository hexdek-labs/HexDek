package analytics

import (
	"sort"
)

// DeckAggregate rolls up multi-game stats for a single deck (keyed by
// commander name). Heimdall already surfaces per-GAME analysis and
// per-COMMANDER win-rate / damage averages; this adds the next layer
// up: per-deck winrate, average turn the deck's winning card hit the
// table (turn-to-finisher), and the deck's most-played opponents with
// matchup records.
type DeckAggregate struct {
	CommanderName string

	GamesPlayed int
	GamesWon    int
	WinRate     float64

	// AvgTurnToFinisher = mean TurnCast of the WinningCard, sampled
	// from games this deck won where the winning card is identified
	// AND found in the winner's CardsPlayed (TurnCast > 0). Games
	// where the winning card is unknown / combat-only / drained-out
	// don't contribute (they're counted in FinisherUnknownWins).
	AvgTurnToFinisher  float64
	FinisherSampleSize int
	FinisherUnknownWins int

	// AvgGameLength is the average TotalTurns across all games this
	// deck played, regardless of result. Useful as the denominator
	// context for turn-to-finisher.
	AvgGameLength float64

	// WinConditions counts the win-condition kinds this deck used
	// (combat_damage / commander_damage / combo / life_drain / ...).
	WinConditions map[string]int

	// Matchups lists per-opponent records sorted by total games desc,
	// then by opponent name asc for stability. Each opponent is every
	// other commander seated in a game with this deck. In a 4-seat
	// game with seat S winning, S records +1 win vs each other seat's
	// commander; each loser records +1 loss vs the winner only (the
	// other 2 losers were eliminated by-or-with the winner; we don't
	// attribute losses to non-winners since they didn't beat anyone).
	Matchups []DeckMatchupSummary
}

// DeckMatchupSummary is one row in DeckAggregate.Matchups: how this
// deck has performed against a specific opponent commander.
type DeckMatchupSummary struct {
	Opponent string
	Games    int // total games where both decks were seated
	Wins     int // games this deck won AND the opponent was seated
	Losses   int // games the opponent won AND this deck was seated
	WinRate  float64 // Wins / Games (mirror losses don't drive this; ties / multi-deck game noise round to 0 contribution)
}

// AggregateDecks rolls up the per-game analyses into per-deck rows.
// commanderNames is the ordered list of commanders present in the
// tournament, used only to seed the output order — any commander seen
// in analyses but missing from commanderNames is appended at the end
// in alphabetical order. Empty / nil input returns nil.
func AggregateDecks(analyses []*GameAnalysis, commanderNames []string) []DeckAggregate {
	if len(analyses) == 0 {
		return nil
	}

	by := make(map[string]*deckAcc)

	getAcc := func(name string) *deckAcc {
		a, ok := by[name]
		if !ok {
			a = &deckAcc{
				winConditions: make(map[string]int),
				matchups:      make(map[string]*matchupCounts),
			}
			by[name] = a
		}
		return a
	}

	for _, ga := range analyses {
		if ga == nil || len(ga.Players) == 0 {
			continue
		}
		// Resolve the winner's commander once; "" if no winner / draw.
		winnerName := ""
		if ga.WinnerSeat >= 0 && ga.WinnerSeat < len(ga.Players) {
			winnerName = ga.Players[ga.WinnerSeat].CommanderName
		}

		// Pass 1: per-deck-per-game updates. If the same deck is
		// seated multiple times in one game (mirror), the deck still
		// counts as having played ONE game — but a win counts if ANY
		// of its seats won. Turn-to-finisher only samples from the
		// actual winning seat. This keeps GamesPlayed and
		// Matchups[X].Games on the same per-game denominator.
		seenDeck := make(map[string]bool, len(ga.Players))
		for seat := range ga.Players {
			pa := &ga.Players[seat]
			if pa.CommanderName == "" {
				continue
			}
			if !seenDeck[pa.CommanderName] {
				seenDeck[pa.CommanderName] = true
				a := getAcc(pa.CommanderName)
				a.games++
				a.gameLengthSum += ga.TotalTurns
			}
			if pa.Won {
				a := getAcc(pa.CommanderName)
				a.wins++
				if ga.WinCondition != "" {
					a.winConditions[ga.WinCondition]++
				}
				// Turn-to-finisher: look up the winning card in the
				// winning seat's CardsPlayed. Two miss cases:
				//   (1) WinningCard == "" — combat-only kill with no
				//       distinct closer card recorded
				//   (2) the card isn't in CardsPlayed or TurnCast == 0
				//       — e.g. an ability of a permanent that was
				//       cast off-screen / never tracked
				// Both count as FinisherUnknownWins so the sample-
				// size denominator is honest.
				if ga.WinningCard != "" {
					if turn := findCardTurnCast(pa, ga.WinningCard); turn > 0 {
						a.finisherTurnSum += turn
						a.finisherSamples++
					} else {
						a.unknownWins++
					}
				} else {
					a.unknownWins++
				}
			}
		}

		// Pass 2: matchups. Dedupe by unique commander name per game so
		// a deck with multiple seats of the same opponent doesn't get
		// double-counted (e.g., 4-player game with two copies of deck
		// B faced by deck A should record A.vs.B as 1 game, not 2).
		// Self-matchups (A vs A) are skipped entirely — mirrors don't
		// produce a meaningful "vs" record.
		present := make(map[string]bool, len(ga.Players))
		for i := range ga.Players {
			if n := ga.Players[i].CommanderName; n != "" {
				present[n] = true
			}
		}
		for cmdr := range present {
			a := getAcc(cmdr)
			for opp := range present {
				if opp == cmdr {
					continue
				}
				m, ok := a.matchups[opp]
				if !ok {
					m = &matchupCounts{}
					a.matchups[opp] = m
				}
				m.games++
				switch winnerName {
				case cmdr:
					m.wins++
				case opp:
					m.losses++
				}
			}
		}
	}

	// Build the output slice. Honor commanderNames ordering first,
	// then append any unknown commanders alphabetically.
	out := make([]DeckAggregate, 0, len(by))
	seen := make(map[string]bool, len(by))
	for _, name := range commanderNames {
		a, ok := by[name]
		if !ok {
			continue
		}
		seen[name] = true
		out = append(out, buildAggregate(name, a))
	}
	var leftover []string
	for name := range by {
		if !seen[name] {
			leftover = append(leftover, name)
		}
	}
	sort.Strings(leftover)
	for _, name := range leftover {
		out = append(out, buildAggregate(name, by[name]))
	}
	return out
}

type matchupCounts struct {
	games, wins, losses int
}

type deckAcc struct {
	games           int
	wins            int
	finisherTurnSum int
	finisherSamples int
	unknownWins     int
	gameLengthSum   int
	winConditions   map[string]int
	matchups        map[string]*matchupCounts
}

func buildAggregate(name string, a *deckAcc) DeckAggregate {
	d := DeckAggregate{
		CommanderName:       name,
		GamesPlayed:         a.games,
		GamesWon:            a.wins,
		FinisherSampleSize:  a.finisherSamples,
		FinisherUnknownWins: a.unknownWins,
		WinConditions:       a.winConditions,
	}
	if a.games > 0 {
		d.WinRate = float64(a.wins) / float64(a.games)
		d.AvgGameLength = float64(a.gameLengthSum) / float64(a.games)
	}
	if a.finisherSamples > 0 {
		d.AvgTurnToFinisher = float64(a.finisherTurnSum) / float64(a.finisherSamples)
	}

	// Matchups: sort by games desc, then by opponent name asc.
	matchups := make([]DeckMatchupSummary, 0, len(a.matchups))
	for opp, m := range a.matchups {
		ms := DeckMatchupSummary{
			Opponent: opp,
			Games:    m.games,
			Wins:     m.wins,
			Losses:   m.losses,
		}
		if m.games > 0 {
			ms.WinRate = float64(m.wins) / float64(m.games)
		}
		matchups = append(matchups, ms)
	}
	sort.SliceStable(matchups, func(i, j int) bool {
		if matchups[i].Games != matchups[j].Games {
			return matchups[i].Games > matchups[j].Games
		}
		return matchups[i].Opponent < matchups[j].Opponent
	})
	d.Matchups = matchups
	return d
}

// findCardTurnCast looks up the first cast turn of the named card in
// pa's CardsPlayed list. Returns 0 if not found / not cast. If the
// card appears multiple times (e.g. recast after a bounce), the
// EARLIEST positive TurnCast wins — that's the turn the threat first
// hit the table, which is the natural "turn-to-finisher" reading.
func findCardTurnCast(pa *PlayerAnalysis, name string) int {
	if pa == nil || name == "" {
		return 0
	}
	earliest := 0
	for i := range pa.CardsPlayed {
		cp := &pa.CardsPlayed[i]
		if cp.Name != name || cp.TurnCast <= 0 {
			continue
		}
		if earliest == 0 || cp.TurnCast < earliest {
			earliest = cp.TurnCast
		}
	}
	return earliest
}
