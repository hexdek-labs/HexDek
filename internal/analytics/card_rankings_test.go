package analytics

import (
	"math"
	"testing"
)

// rankCardsApprox compares two float64s for the small-numeric tolerance
// used throughout the card-rankings math (ratios with integer
// numerators / denominators tend to be exact, but I keep the helper
// in case future fields use floating accumulation).
func rankCardsApprox(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// findRanking is the test-side lookup helper: RankCards returns a sorted
// slice, but tests usually want to assert on one card's row in
// isolation. Returns nil if the card isn't in the rankings.
func findRanking(rankings []CardRanking, name string) *CardRanking {
	for i := range rankings {
		if rankings[i].Name == name {
			return &rankings[i]
		}
	}
	return nil
}

// TestRankCards_Empty pins the zero-input guard: nil input returns nil
// output, no panic. Single-line check but it's the kind of thing that
// breaks silently if a future refactor introduces a range-without-guard.
func TestRankCards_Empty(t *testing.T) {
	if got := RankCards(nil); got != nil {
		t.Errorf("RankCards(nil) = %v, want nil", got)
	}
	if got := RankCards([]*GameAnalysis{}); got != nil {
		t.Errorf("RankCards(empty) = %v, want nil", got)
	}
}

// TestRankCards_SingleCard_WinContribution is the simplest positive
// case: one game, one card, won, contributed to win. Pins the
// load-bearing WinContribution = winContribSum / gamesWon ratio.
func TestRankCards_SingleCard_WinContribution(t *testing.T) {
	ga := &GameAnalysis{
		WinningCard: "Sol Ring",
		Players: []PlayerAnalysis{{
			Won: true,
			CardsPlayed: []CardPerformance{{
				Name:             "Sol Ring",
				TurnCast:         1,
				ContributedToWin: true,
				DamageDealt:      0,
				KillsAttributed:  0,
			}},
		}},
	}
	rankings := RankCards([]*GameAnalysis{ga})
	if len(rankings) != 1 {
		t.Fatalf("expected 1 ranking, got %d", len(rankings))
	}
	cr := rankings[0]
	if cr.GamesPlayed != 1 || cr.TimesCast != 1 || cr.GamesWon != 1 {
		t.Errorf("counts: %+v, want games=1 cast=1 won=1", cr)
	}
	if !rankCardsApprox(cr.WinContribution, 1.0) {
		t.Errorf("WinContribution = %f, want 1.0", cr.WinContribution)
	}
	if !rankCardsApprox(cr.KillShotRate, 1.0) {
		t.Errorf("KillShotRate (this card was the WinningCard in the win) = %f, want 1.0",
			cr.KillShotRate)
	}
	if !rankCardsApprox(cr.AvgTurnCast, 1.0) {
		t.Errorf("AvgTurnCast = %f, want 1.0", cr.AvgTurnCast)
	}
	if !rankCardsApprox(cr.DeadInHandRate, 0.0) {
		t.Errorf("DeadInHandRate = %f, want 0.0 (was cast)", cr.DeadInHandRate)
	}
}

// TestRankCards_DeadInHand pins the classifier: a non-land non-token
// card with TurnCast=0 AND WasCountered=false counts as dead-in-hand
// (never cast, never countered, just sat there). The DeadInHandRate
// is one of Heimdall's main signals for "deck has bricks."
func TestRankCards_DeadInHand(t *testing.T) {
	ga := &GameAnalysis{
		Players: []PlayerAnalysis{{
			Won: false,
			CardsPlayed: []CardPerformance{{
				Name:         "Useless Brick",
				TurnCast:     0,
				WasCountered: false,
				IsLand:       false,
				IsToken:      false,
			}},
		}},
	}
	rankings := RankCards([]*GameAnalysis{ga})
	cr := findRanking(rankings, "Useless Brick")
	if cr == nil {
		t.Fatal("Useless Brick missing from rankings")
	}
	if cr.TimesCast != 0 {
		t.Errorf("TimesCast = %d, want 0", cr.TimesCast)
	}
	if !rankCardsApprox(cr.DeadInHandRate, 1.0) {
		t.Errorf("DeadInHandRate = %f, want 1.0", cr.DeadInHandRate)
	}
}

// TestRankCards_LandsExcludedFromDeadInHand confirms the explicit
// "lands are PLAYED not cast" rule: a land with TurnCast=0 must NOT
// count as dead-in-hand (it's a normal-functioning land that wasn't
// drawn / wasn't relevant to track). Same rule applies to tokens.
func TestRankCards_LandsExcludedFromDeadInHand(t *testing.T) {
	ga := &GameAnalysis{
		Players: []PlayerAnalysis{{
			Won: false,
			CardsPlayed: []CardPerformance{
				{Name: "Forest", TurnCast: 0, IsLand: true},
				{Name: "Spirit Token", TurnCast: 0, IsToken: true},
				{Name: "creature token colorless spirit Token",
					TurnCast: 0, IsToken: false}, // matched by isTokenByName
			},
		}},
	}
	rankings := RankCards([]*GameAnalysis{ga})
	for _, name := range []string{"Forest", "Spirit Token", "creature token colorless spirit Token"} {
		cr := findRanking(rankings, name)
		if cr == nil {
			t.Fatalf("%s missing from rankings", name)
		}
		if !rankCardsApprox(cr.DeadInHandRate, 0.0) {
			t.Errorf("%s DeadInHandRate = %f, want 0.0 (land/token exempt)",
				name, cr.DeadInHandRate)
		}
	}
}

// TestRankCards_CounteredCountsAsCastAttempt pins the WasCountered
// branch: a card that was cast and countered counts toward
// castAttempts (so CounteredRate denominator is correct) and
// counteredCount, but NOT toward TimesCast (the spell didn't resolve)
// and NOT toward DeadInHand (it WAS cast, just failed to resolve).
func TestRankCards_CounteredCountsAsCastAttempt(t *testing.T) {
	ga := &GameAnalysis{
		Players: []PlayerAnalysis{{
			Won: false,
			CardsPlayed: []CardPerformance{{
				Name:         "Counterspell Bait",
				TurnCast:     0, // didn't resolve
				WasCountered: true,
			}},
		}},
	}
	rankings := RankCards([]*GameAnalysis{ga})
	cr := findRanking(rankings, "Counterspell Bait")
	if cr == nil {
		t.Fatal("missing")
	}
	if cr.TimesCast != 0 {
		t.Errorf("TimesCast (didn't resolve) = %d, want 0", cr.TimesCast)
	}
	if !rankCardsApprox(cr.CounteredRate, 1.0) {
		t.Errorf("CounteredRate (1 countered / 1 attempt) = %f, want 1.0", cr.CounteredRate)
	}
	if !rankCardsApprox(cr.DeadInHandRate, 0.0) {
		t.Errorf("DeadInHandRate (countered ≠ dead-in-hand) = %f, want 0.0",
			cr.DeadInHandRate)
	}
}

// TestRankCards_AggregatesAcrossGames is the multi-game pin: same card
// played in 3 games (2 wins + 1 loss, 2 cast + 1 dead, 1 contrib + 0
// + 0). Verifies the per-card accumulator math survives N>1.
func TestRankCards_AggregatesAcrossGames(t *testing.T) {
	mk := func(won bool, turnCast int, contrib bool) *GameAnalysis {
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{{
					Name:             "Hero",
					TurnCast:         turnCast,
					ContributedToWin: contrib,
					DamageDealt:      3,
					KillsAttributed:  1,
				}},
			}},
		}
	}
	games := []*GameAnalysis{
		mk(true, 2, true),   // won, cast turn 2, contributed
		mk(true, 4, false),  // won, cast turn 4, didn't contribute
		mk(false, 0, false), // lost, dead in hand
	}
	rankings := RankCards(games)
	cr := findRanking(rankings, "Hero")
	if cr == nil {
		t.Fatal("Hero missing")
	}
	if cr.GamesPlayed != 3 {
		t.Errorf("GamesPlayed = %d, want 3", cr.GamesPlayed)
	}
	if cr.TimesCast != 2 {
		t.Errorf("TimesCast = %d, want 2 (cast in g1+g2 only)", cr.TimesCast)
	}
	if cr.GamesWon != 2 {
		t.Errorf("GamesWon = %d, want 2", cr.GamesWon)
	}
	// WinContribution = 1/2 (contributed in 1 of 2 wins).
	if !rankCardsApprox(cr.WinContribution, 0.5) {
		t.Errorf("WinContribution = %f, want 0.5", cr.WinContribution)
	}
	// AvgTurnCast = (2+4)/2 = 3.
	if !rankCardsApprox(cr.AvgTurnCast, 3.0) {
		t.Errorf("AvgTurnCast = %f, want 3.0", cr.AvgTurnCast)
	}
	// AvgDamageDealt = (3+3+3)/3 = 3 (CardPerformance.DamageDealt always set).
	if !rankCardsApprox(cr.AvgDamageDealt, 3.0) {
		t.Errorf("AvgDamageDealt = %f, want 3.0", cr.AvgDamageDealt)
	}
	// AvgKills = (1+1+1)/3 = 1.
	if !rankCardsApprox(cr.AvgKills, 1.0) {
		t.Errorf("AvgKills = %f, want 1.0", cr.AvgKills)
	}
	// DeadInHandRate = 1/3.
	if !rankCardsApprox(cr.DeadInHandRate, 1.0/3.0) {
		t.Errorf("DeadInHandRate = %f, want 1/3", cr.DeadInHandRate)
	}
}

// TestRankCards_SortOrder pins the documented sort key: GamesWon
// descending, then TimesCast descending, then AvgDamageDealt
// descending. Three cards with controlled tiebreaks exercise each
// level of the comparator.
func TestRankCards_SortOrder(t *testing.T) {
	mk := func(name string, won bool, turnCast int, dmg int) *GameAnalysis {
		return &GameAnalysis{
			Players: []PlayerAnalysis{{
				Won: won,
				CardsPlayed: []CardPerformance{{
					Name:        name,
					TurnCast:    turnCast,
					DamageDealt: dmg,
				}},
			}},
		}
	}
	games := []*GameAnalysis{
		// "Top": 2 wins, 2 casts, 10 damage avg.
		mk("Top", true, 1, 10), mk("Top", true, 2, 10),
		// "Mid": 1 win, 1 cast, 20 damage avg (higher damage than Top
		// but fewer wins — Top still ranks higher).
		mk("Mid", true, 1, 20), mk("Mid", false, 0, 0),
		// "TiebreakA" and "TiebreakB": both 1 win, 1 cast. TiebreakA
		// has higher damage → ranks above TiebreakB.
		mk("TiebreakA", true, 1, 5),
		mk("TiebreakB", true, 1, 1),
	}
	rankings := RankCards(games)
	// Find order indices.
	rank := map[string]int{}
	for i, r := range rankings {
		rank[r.Name] = i
	}
	if rank["Top"] >= rank["Mid"] {
		t.Errorf("Top should rank above Mid: Top=%d Mid=%d", rank["Top"], rank["Mid"])
	}
	if rank["Mid"] >= rank["TiebreakA"] && rank["Mid"] >= rank["TiebreakB"] {
		t.Errorf("Mid should rank above TiebreakA/B: Mid=%d A=%d B=%d",
			rank["Mid"], rank["TiebreakA"], rank["TiebreakB"])
	}
	if rank["TiebreakA"] >= rank["TiebreakB"] {
		t.Errorf("TiebreakA (higher dmg) should rank above TiebreakB: A=%d B=%d",
			rank["TiebreakA"], rank["TiebreakB"])
	}
}

// TestRankCards_KillShotRate pins the kill-shot accounting:
// ga.WinningCard must MATCH the card name AND that player must have
// won. KillShotRate denominator is GamesWon, NOT GamesPlayed.
func TestRankCards_KillShotRate(t *testing.T) {
	mk := func(winning string, playerName string, won bool) *GameAnalysis {
		return &GameAnalysis{
			WinningCard: winning,
			Players: []PlayerAnalysis{{
				Won:         won,
				CardsPlayed: []CardPerformance{{Name: playerName, TurnCast: 1}},
			}},
		}
	}
	games := []*GameAnalysis{
		mk("Closer", "Closer", true), // won, was the closer
		mk("Other", "Closer", true),  // won, but wasn't the closer
		mk("Closer", "Closer", false), // someone else's win — losing player had Closer
	}
	rankings := RankCards(games)
	cr := findRanking(rankings, "Closer")
	if cr == nil {
		t.Fatal("Closer missing")
	}
	if cr.GamesWon != 2 {
		t.Errorf("GamesWon = %d, want 2", cr.GamesWon)
	}
	// KillShots = 1 (only the first game), GamesWon = 2 → rate = 0.5.
	if !rankCardsApprox(cr.KillShotRate, 0.5) {
		t.Errorf("KillShotRate = %f, want 0.5 (1 kill shot / 2 wins)", cr.KillShotRate)
	}
}

// TestRankCards_NilGameSkipped pins the nil-element guard at the
// top of the loop.
func TestRankCards_NilGameSkipped(t *testing.T) {
	games := []*GameAnalysis{
		nil,
		{Players: []PlayerAnalysis{{
			Won: true,
			CardsPlayed: []CardPerformance{{Name: "Real Card", TurnCast: 1}},
		}}},
		nil,
	}
	rankings := RankCards(games)
	cr := findRanking(rankings, "Real Card")
	if cr == nil {
		t.Fatal("Real Card missing")
	}
	if cr.GamesPlayed != 1 {
		t.Errorf("GamesPlayed (should skip nils) = %d, want 1", cr.GamesPlayed)
	}
}

// TestIsTokenByName confirms the case-insensitive substring match.
// Engine-generated token names look like "creature token colorless
// spirit Token" — these need to be detected regardless of casing.
func TestIsTokenByName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Spirit Token", true},
		{"creature token colorless spirit Token", true},
		{"TOKEN", true},
		{"Lightning Bolt", false},
		{"", false},
		{"tokenizer", true}, // substring match — documented behavior, not a bug
	}
	for _, c := range cases {
		if got := isTokenByName(c.name); got != c.want {
			t.Errorf("isTokenByName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestRankCards_ZeroWinsZeroContrib pins the divide-by-zero guard on
// WinContribution: a card that appeared in losses only should produce
// WinContribution = 0 (not NaN).
func TestRankCards_ZeroWinsZeroContrib(t *testing.T) {
	ga := &GameAnalysis{
		Players: []PlayerAnalysis{{
			Won: false,
			CardsPlayed: []CardPerformance{{
				Name:     "Loser",
				TurnCast: 3,
			}},
		}},
	}
	rankings := RankCards([]*GameAnalysis{ga})
	cr := findRanking(rankings, "Loser")
	if cr == nil {
		t.Fatal("missing")
	}
	if math.IsNaN(cr.WinContribution) {
		t.Errorf("WinContribution NaN with zero wins — divide-by-zero guard failed")
	}
	if !rankCardsApprox(cr.WinContribution, 0.0) {
		t.Errorf("WinContribution = %f, want 0.0", cr.WinContribution)
	}
	if math.IsNaN(cr.KillShotRate) {
		t.Errorf("KillShotRate NaN with zero wins — divide-by-zero guard failed")
	}
}
