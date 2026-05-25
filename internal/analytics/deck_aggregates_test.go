package analytics

import (
	"math"
	"strings"
	"testing"
)

func approxEq(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func findAgg(aggs []DeckAggregate, name string) *DeckAggregate {
	for i := range aggs {
		if aggs[i].CommanderName == name {
			return &aggs[i]
		}
	}
	return nil
}

func findMatch(m []DeckMatchupSummary, opp string) *DeckMatchupSummary {
	for i := range m {
		if m[i].Opponent == opp {
			return &m[i]
		}
	}
	return nil
}

// TestAggregateDecks_Empty pins the nil-input guard.
func TestAggregateDecks_Empty(t *testing.T) {
	if got := AggregateDecks(nil, nil); got != nil {
		t.Errorf("AggregateDecks(nil) = %v, want nil", got)
	}
	if got := AggregateDecks([]*GameAnalysis{}, nil); got != nil {
		t.Errorf("AggregateDecks(empty) = %v, want nil", got)
	}
}

// TestAggregateDecks_BasicWinrate pins the simplest case: 4 games, A
// wins 3, B wins 1, in head-to-head. Verifies winrate, games,
// matchup symmetry on games count.
func TestAggregateDecks_BasicWinrate(t *testing.T) {
	mkGame := func(winner int) *GameAnalysis {
		return &GameAnalysis{
			WinnerSeat:   winner,
			WinCondition: "combat_damage",
			TotalTurns:   10,
			Players: []PlayerAnalysis{
				{Seat: 0, CommanderName: "A", Won: winner == 0},
				{Seat: 1, CommanderName: "B", Won: winner == 1},
			},
		}
	}
	games := []*GameAnalysis{mkGame(0), mkGame(0), mkGame(0), mkGame(1)}
	aggs := AggregateDecks(games, []string{"A", "B"})
	if len(aggs) != 2 {
		t.Fatalf("got %d aggregates, want 2", len(aggs))
	}
	a := findAgg(aggs, "A")
	b := findAgg(aggs, "B")
	if a == nil || b == nil {
		t.Fatalf("missing aggregate: A=%v B=%v", a, b)
	}
	if a.GamesPlayed != 4 || a.GamesWon != 3 || !approxEq(a.WinRate, 0.75) {
		t.Errorf("A: games=%d wins=%d wr=%.3f, want 4/3/0.75", a.GamesPlayed, a.GamesWon, a.WinRate)
	}
	if b.GamesPlayed != 4 || b.GamesWon != 1 || !approxEq(b.WinRate, 0.25) {
		t.Errorf("B: games=%d wins=%d wr=%.3f, want 4/1/0.25", b.GamesPlayed, b.GamesWon, b.WinRate)
	}
	// Matchup: A vs B = 4 games, 3 wins, 1 loss for A.
	mAB := findMatch(a.Matchups, "B")
	if mAB == nil {
		t.Fatalf("A has no matchup vs B")
	}
	if mAB.Games != 4 || mAB.Wins != 3 || mAB.Losses != 1 || !approxEq(mAB.WinRate, 0.75) {
		t.Errorf("A vs B: %+v, want 4 games / 3 wins / 1 loss / 0.75", *mAB)
	}
	// Symmetric: B vs A = 4 games, 1 win, 3 losses.
	mBA := findMatch(b.Matchups, "A")
	if mBA == nil {
		t.Fatalf("B has no matchup vs A")
	}
	if mBA.Games != 4 || mBA.Wins != 1 || mBA.Losses != 3 {
		t.Errorf("B vs A: %+v, want 4 games / 1 win / 3 losses", *mBA)
	}
}

// TestAggregateDecks_TurnToFinisher pins the winning-card lookup: AVG
// over the seat's CardsPlayed TurnCast for the WinningCard. Misses
// (empty WinningCard, card not in player list, TurnCast=0) bump
// FinisherUnknownWins instead.
func TestAggregateDecks_TurnToFinisher(t *testing.T) {
	mkWin := func(winningCard string, turn int) *GameAnalysis {
		ga := &GameAnalysis{
			WinnerSeat:   0,
			WinningCard:  winningCard,
			WinCondition: "combo",
			TotalTurns:   12,
			Players: []PlayerAnalysis{
				{Seat: 0, CommanderName: "A", Won: true},
				{Seat: 1, CommanderName: "B", Won: false},
			},
		}
		if winningCard != "" && turn > 0 {
			ga.Players[0].CardsPlayed = []CardPerformance{
				{Name: winningCard, TurnCast: turn, ContributedToWin: true},
			}
		}
		return ga
	}
	games := []*GameAnalysis{
		mkWin("Craterhoof Behemoth", 6),
		mkWin("Craterhoof Behemoth", 8),
		mkWin("Craterhoof Behemoth", 10),
		mkWin("", 0),               // combat-only kill, no closer
		mkWin("Phantom Card", 0),   // card not in CardsPlayed → unknown
	}
	a := findAgg(AggregateDecks(games, []string{"A", "B"}), "A")
	if a == nil {
		t.Fatalf("missing A")
	}
	if a.FinisherSampleSize != 3 {
		t.Errorf("FinisherSampleSize = %d, want 3 (3 of 5 wins have a resolvable WinningCard turn)", a.FinisherSampleSize)
	}
	if a.FinisherUnknownWins != 2 {
		t.Errorf("FinisherUnknownWins = %d, want 2", a.FinisherUnknownWins)
	}
	want := (6.0 + 8.0 + 10.0) / 3.0
	if !approxEq(a.AvgTurnToFinisher, want) {
		t.Errorf("AvgTurnToFinisher = %.3f, want %.3f", a.AvgTurnToFinisher, want)
	}
}

// TestAggregateDecks_TurnToFinisher_PicksEarliestCast pins the
// recast tiebreak: a card cast twice (e.g. bounced and replayed) takes
// the earliest TurnCast as the threat's first appearance.
func TestAggregateDecks_TurnToFinisher_PicksEarliestCast(t *testing.T) {
	ga := &GameAnalysis{
		WinnerSeat:   0,
		WinningCard:  "Cyclonic Rift",
		WinCondition: "combat_damage",
		TotalTurns:   8,
		Players: []PlayerAnalysis{
			{Seat: 0, CommanderName: "A", Won: true, CardsPlayed: []CardPerformance{
				{Name: "Cyclonic Rift", TurnCast: 8, ContributedToWin: true},
				{Name: "Cyclonic Rift", TurnCast: 3, ContributedToWin: false},
			}},
			{Seat: 1, CommanderName: "B", Won: false},
		},
	}
	a := findAgg(AggregateDecks([]*GameAnalysis{ga}, nil), "A")
	if a == nil {
		t.Fatalf("missing A")
	}
	if !approxEq(a.AvgTurnToFinisher, 3.0) {
		t.Errorf("AvgTurnToFinisher = %.3f, want 3.0 (earliest cast of WinningCard)", a.AvgTurnToFinisher)
	}
}

// TestAggregateDecks_MatchupsAcross4SeatGame pins multi-opponent
// matchup counting in a 4-player game. A wins, faces B/C/D — each
// matchup records 1 game, 1 win for A and 1 loss for the opponent
// vs A; B/C/D do NOT record wins/losses against each other since none
// of them beat the others.
func TestAggregateDecks_MatchupsAcross4SeatGame(t *testing.T) {
	ga := &GameAnalysis{
		WinnerSeat:   0,
		WinCondition: "combat_damage",
		TotalTurns:   12,
		Players: []PlayerAnalysis{
			{Seat: 0, CommanderName: "A", Won: true},
			{Seat: 1, CommanderName: "B"},
			{Seat: 2, CommanderName: "C"},
			{Seat: 3, CommanderName: "D"},
		},
	}
	aggs := AggregateDecks([]*GameAnalysis{ga}, []string{"A", "B", "C", "D"})
	a := findAgg(aggs, "A")
	if a == nil {
		t.Fatalf("missing A")
	}
	if len(a.Matchups) != 3 {
		t.Errorf("A has %d matchups, want 3 (one per non-self opponent)", len(a.Matchups))
	}
	for _, opp := range []string{"B", "C", "D"} {
		m := findMatch(a.Matchups, opp)
		if m == nil {
			t.Errorf("A missing matchup vs %s", opp)
			continue
		}
		if m.Games != 1 || m.Wins != 1 || m.Losses != 0 {
			t.Errorf("A vs %s = %+v, want 1g/1w/0l", opp, *m)
		}
	}
	// B's record: 1 game vs A (loss), 1 game each vs C and D (neither
	// a win nor a loss — neither beat the other).
	b := findAgg(aggs, "B")
	if b == nil {
		t.Fatalf("missing B")
	}
	mBA := findMatch(b.Matchups, "A")
	if mBA == nil || mBA.Games != 1 || mBA.Wins != 0 || mBA.Losses != 1 {
		t.Errorf("B vs A = %+v, want 1g/0w/1l", mBA)
	}
	mBC := findMatch(b.Matchups, "C")
	if mBC == nil || mBC.Games != 1 || mBC.Wins != 0 || mBC.Losses != 0 {
		t.Errorf("B vs C = %+v, want 1g/0w/0l (neither won)", mBC)
	}
}

// TestAggregateDecks_MatchupSortByFrequency pins the matchup ordering:
// most-faced opponents first, name asc as tiebreak.
func TestAggregateDecks_MatchupSortByFrequency(t *testing.T) {
	// A plays 3 games vs B, 1 game vs C, 1 game vs D.
	mkAvs := func(opp string, aWins bool) *GameAnalysis {
		winner := 0
		if !aWins {
			winner = 1
		}
		return &GameAnalysis{
			WinnerSeat:   winner,
			WinCondition: "combat_damage",
			TotalTurns:   8,
			Players: []PlayerAnalysis{
				{Seat: 0, CommanderName: "A", Won: aWins},
				{Seat: 1, CommanderName: opp, Won: !aWins},
			},
		}
	}
	games := []*GameAnalysis{
		mkAvs("B", true), mkAvs("B", false), mkAvs("B", true),
		mkAvs("C", true),
		mkAvs("D", false),
	}
	a := findAgg(AggregateDecks(games, []string{"A", "B", "C", "D"}), "A")
	if a == nil {
		t.Fatalf("missing A")
	}
	if len(a.Matchups) != 3 {
		t.Fatalf("got %d matchups, want 3", len(a.Matchups))
	}
	// B (3 games) must come first; C and D tie on 1 game so alpha sort
	// places C before D.
	if a.Matchups[0].Opponent != "B" || a.Matchups[0].Games != 3 {
		t.Errorf("matchup[0] = %+v, want B with 3 games", a.Matchups[0])
	}
	if a.Matchups[1].Opponent != "C" || a.Matchups[2].Opponent != "D" {
		t.Errorf("matchup[1..2] = %s, %s, want C, D (alpha tiebreak)",
			a.Matchups[1].Opponent, a.Matchups[2].Opponent)
	}
}

// TestAggregateDecks_MultiSeatMirror pins that a deck played at two
// seats in the same game is counted as ONE game played by that deck
// (not two), and a single seat's win is a deck-level win.
func TestAggregateDecks_MultiSeatMirror(t *testing.T) {
	ga := &GameAnalysis{
		WinnerSeat:   0,
		WinCondition: "combo",
		TotalTurns:   9,
		Players: []PlayerAnalysis{
			{Seat: 0, CommanderName: "A", Won: true},
			{Seat: 1, CommanderName: "A", Won: false}, // mirror loses
			{Seat: 2, CommanderName: "B", Won: false},
		},
	}
	aggs := AggregateDecks([]*GameAnalysis{ga}, []string{"A", "B"})
	a := findAgg(aggs, "A")
	b := findAgg(aggs, "B")
	if a == nil || b == nil {
		t.Fatalf("missing aggregate")
	}
	if a.GamesPlayed != 1 || a.GamesWon != 1 {
		t.Errorf("A: %d games / %d wins, want 1/1 (mirror collapses)", a.GamesPlayed, a.GamesWon)
	}
	if b.GamesPlayed != 1 || b.GamesWon != 0 {
		t.Errorf("B: %d games / %d wins, want 1/0", b.GamesPlayed, b.GamesWon)
	}
	// A.matchups[B] = 1 game, 1 win, 0 losses.
	mAB := findMatch(a.Matchups, "B")
	if mAB == nil || mAB.Games != 1 || mAB.Wins != 1 {
		t.Errorf("A vs B = %+v, want 1g/1w/0l (mirror dedupes)", mAB)
	}
	// No self matchup A vs A.
	if findMatch(a.Matchups, "A") != nil {
		t.Errorf("A vs A should be skipped, got %+v", findMatch(a.Matchups, "A"))
	}
}

// TestAggregateDecks_WinConditions pins the win-condition mix tally:
// only the WINNER's commander gets credited.
func TestAggregateDecks_WinConditions(t *testing.T) {
	mk := func(winCond string) *GameAnalysis {
		return &GameAnalysis{
			WinnerSeat:   0,
			WinCondition: winCond,
			TotalTurns:   7,
			Players: []PlayerAnalysis{
				{Seat: 0, CommanderName: "A", Won: true},
				{Seat: 1, CommanderName: "B"},
			},
		}
	}
	games := []*GameAnalysis{mk("combo"), mk("combo"), mk("combat_damage")}
	a := findAgg(AggregateDecks(games, []string{"A", "B"}), "A")
	if a == nil {
		t.Fatalf("missing A")
	}
	if a.WinConditions["combo"] != 2 || a.WinConditions["combat_damage"] != 1 {
		t.Errorf("WinConditions = %v, want combo=2 combat_damage=1", a.WinConditions)
	}
	b := findAgg(AggregateDecks(games, []string{"A", "B"}), "B")
	if b == nil {
		t.Fatalf("missing B")
	}
	if len(b.WinConditions) != 0 {
		t.Errorf("B WinConditions = %v, want empty (B never won)", b.WinConditions)
	}
}

// TestAggregateDecks_OrderingHonorsCommanderNames pins the output
// order: commanderNames-listed decks come first in the given order,
// unknown commanders are appended alphabetically.
func TestAggregateDecks_OrderingHonorsCommanderNames(t *testing.T) {
	mk := func(a, b string, winA bool) *GameAnalysis {
		winner := 0
		if !winA {
			winner = 1
		}
		return &GameAnalysis{
			WinnerSeat: winner, TotalTurns: 5, WinCondition: "combat_damage",
			Players: []PlayerAnalysis{
				{Seat: 0, CommanderName: a, Won: winA},
				{Seat: 1, CommanderName: b, Won: !winA},
			},
		}
	}
	// commanderNames lists A, B (B first by ordering choice). C and Z
	// are unknowns — should appear alphabetically AFTER the listed
	// ones.
	games := []*GameAnalysis{mk("A", "B", true), mk("Z", "C", false), mk("C", "A", true)}
	aggs := AggregateDecks(games, []string{"B", "A"})
	if len(aggs) < 4 {
		t.Fatalf("got %d aggregates, want >= 4", len(aggs))
	}
	if aggs[0].CommanderName != "B" || aggs[1].CommanderName != "A" {
		t.Errorf("listed-name order: got %s, %s; want B, A", aggs[0].CommanderName, aggs[1].CommanderName)
	}
	// C and Z appended alphabetically.
	if aggs[2].CommanderName != "C" || aggs[3].CommanderName != "Z" {
		t.Errorf("leftover alpha order: got %s, %s; want C, Z",
			aggs[2].CommanderName, aggs[3].CommanderName)
	}
}

// TestWriteDeckAggregatesTo_Renders pins the section render: header,
// per-deck row, matchup table, win-condition mix line.
func TestWriteDeckAggregatesTo_Renders(t *testing.T) {
	games := []*GameAnalysis{
		{
			WinnerSeat: 0, TotalTurns: 8, WinCondition: "combat_damage",
			WinningCard: "Embercleave",
			Players: []PlayerAnalysis{
				{Seat: 0, CommanderName: "Voltron", Won: true, CardsPlayed: []CardPerformance{
					{Name: "Embercleave", TurnCast: 5},
				}},
				{Seat: 1, CommanderName: "Control"},
			},
		},
	}
	r := &AnalyticsReport{
		Analyses:       games,
		CommanderNames: []string{"Voltron", "Control"},
		TotalGames:     1,
	}
	var sb strings.Builder
	r.WriteDeckAggregatesTo(&sb, 5)
	out := sb.String()
	for _, want := range []string{
		"## Per-Deck Aggregates",
		"Voltron", "Control",
		"Common matchups",
		"Wins by condition",
		"combat_damage=1",
		"5.0 (n=1)", // turn-to-finisher
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered section missing %q\nGOT:\n%s", want, out)
		}
	}
}
