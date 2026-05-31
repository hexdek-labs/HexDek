package tournament

import (
	"math"
	"testing"
)

// R60 pool-fairness regressions for Swiss strength-of-schedule
// tiebreaker (OpponentMatchWinPct). Pre-fix, sortedStandings fell
// back to alphabetical CommanderName after just (Points, Wins),
// which meant a deck with an easy schedule outranked an equal-
// score deck that beat a harder pod. Now MTG CR §16.3.2 ordering:
// Points → Wins → OMW% → WinRate → name.

// makeStandings builds a standings slice from a name + win/games
// list for terse fixture setup.
func makeStandings(rows []struct {
	name   string
	points int
	wins   int
	games  int
}) []SwissStanding {
	out := make([]SwissStanding, len(rows))
	for i, r := range rows {
		s := SwissStanding{
			CommanderName: r.name,
			Points:        r.points,
			Wins:          r.wins,
			Games:         r.games,
		}
		if r.games > 0 {
			s.WinRate = float64(r.wins) / float64(r.games)
		}
		out[i] = s
	}
	return out
}

// pairings builds the pastPairings adjacency from a list of
// {deck_idx, opponents...} tuples. Symmetric (a→b implies b→a).
func pairings(n int, links [][]int) []map[int]struct{} {
	out := make([]map[int]struct{}, n)
	for i := range out {
		out[i] = make(map[int]struct{})
	}
	for _, link := range links {
		if len(link) < 2 {
			continue
		}
		from := link[0]
		for _, to := range link[1:] {
			out[from][to] = struct{}{}
			out[to][from] = struct{}{}
		}
	}
	return out
}

// TestSwissOMW_BasicAverage pins the average computation for a
// deck with three opponents at known winrates.
func TestSwissOMW_BasicAverage(t *testing.T) {
	// 4 decks. Deck 0 faced decks 1, 2, 3 with winrates 0.8, 0.6, 0.4.
	// Expected OMW = (0.8 + 0.6 + 0.4) / 3 = 0.6.
	st := makeStandings([]struct {
		name           string
		points, wins, games int
	}{
		{"A", 9, 3, 3},
		{"B", 0, 4, 5}, // 0.8
		{"C", 0, 3, 5}, // 0.6
		{"D", 0, 2, 5}, // 0.4
	})
	pp := pairings(4, [][]int{{0, 1, 2, 3}})
	computeSwissOMW(st, pp)

	want := (0.8 + 0.6 + 0.4) / 3
	if math.Abs(st[0].OpponentMatchWinPct-want) > 1e-9 {
		t.Errorf("deck A OMW = %f, want %f", st[0].OpponentMatchWinPct, want)
	}
}

// TestSwissOMW_FloorApplied pins MTG CR §16.3.2 floor behavior:
// opponents with WinRate < SwissOMWFloor (0.33) contribute the
// floor value, not their actual winrate, so a single 0-win
// opponent doesn't tank SoS.
func TestSwissOMW_FloorApplied(t *testing.T) {
	st := makeStandings([]struct {
		name           string
		points, wins, games int
	}{
		{"A", 9, 3, 3},
		{"B", 0, 4, 5}, // 0.8 (above floor)
		{"C", 0, 0, 5}, // 0.0 → floored to 0.33
		{"D", 0, 1, 5}, // 0.2 → floored to 0.33
	})
	pp := pairings(4, [][]int{{0, 1, 2, 3}})
	computeSwissOMW(st, pp)

	want := (0.8 + SwissOMWFloor + SwissOMWFloor) / 3
	if math.Abs(st[0].OpponentMatchWinPct-want) > 1e-9 {
		t.Errorf("deck A OMW with floor = %f, want %f", st[0].OpponentMatchWinPct, want)
	}
}

// TestSwissOMW_NoOpponentsZero pins the round-1-bye edge case:
// a deck with no recorded opponents gets OMW = 0 (not floored),
// since there's no strength signal to apply the floor to.
func TestSwissOMW_NoOpponentsZero(t *testing.T) {
	st := makeStandings([]struct {
		name           string
		points, wins, games int
	}{
		{"A", 3, 0, 0}, // bye recipient, no games played
		{"B", 3, 1, 1},
	})
	pp := pairings(2, nil) // A has no opponents
	computeSwissOMW(st, pp)

	if st[0].OpponentMatchWinPct != 0 {
		t.Errorf("no-opponents deck should get OMW=0, got %f", st[0].OpponentMatchWinPct)
	}
}

// TestSwissRanking_OMWTiebreakWins is the headline fairness test.
// Two decks tied on Points + Wins but with different schedules:
// the deck that faced HARDER opponents (higher OMW) ranks higher.
// Pre-fix the ranking fell through to alphabetical and the
// easy-schedule "A" deck outranked the harder-schedule "Z" deck.
func TestSwissRanking_OMWTiebreakWins(t *testing.T) {
	// Decks A and Z both went 2-1 (6 points, 2 wins). But Z's three
	// opponents (D, E, F) each went 2-1 too, while A's three opponents
	// (B, C — and Z) include weaker decks. We construct it so Z's
	// schedule is strictly tougher than A's.
	st := makeStandings([]struct {
		name           string
		points, wins, games int
	}{
		{"A", 6, 2, 3}, // idx 0
		{"Z", 6, 2, 3}, // idx 1 — tougher schedule
		{"B", 3, 1, 3}, // idx 2 — A's opponent (weak)
		{"C", 0, 0, 3}, // idx 3 — A's opponent (very weak)
		{"D", 6, 2, 3}, // idx 4 — Z's opponent (strong)
		{"E", 6, 2, 3}, // idx 5 — Z's opponent (strong)
		{"F", 6, 2, 3}, // idx 6 — Z's opponent (strong)
	})
	// A's opponents: B, C, and someone weak — let's give A wins over C/B
	// and a loss to E. Z's opponents: D, E, F (all strong).
	pp := pairings(7, [][]int{
		{0, 2, 3, 5}, // A faced B, C, E
		{1, 4, 5, 6}, // Z faced D, E, F
	})
	computeSwissOMW(st, pp)

	// Rank the standings.
	ranked := sortedStandings(st)

	// Z must rank above A among the (6, 2) tier — the entire point of
	// the OMW% tiebreaker.
	var aRank, zRank int
	for i, s := range ranked {
		if s.CommanderName == "A" {
			aRank = i
		}
		if s.CommanderName == "Z" {
			zRank = i
		}
	}
	if zRank >= aRank {
		t.Errorf("Z (tougher schedule, OMW=%.3f) should rank ABOVE A (easier, OMW=%.3f); got zRank=%d aRank=%d. Full ranking: %v",
			st[1].OpponentMatchWinPct, st[0].OpponentMatchWinPct, zRank, aRank, ranked)
	}
}

// TestSwissRanking_PointsDominatesOMW pins that OMW% is the THIRD
// key only — Points still wins. A high-OMW deck with fewer Points
// must rank below a low-OMW deck with more Points.
func TestSwissRanking_PointsDominatesOMW(t *testing.T) {
	st := makeStandings([]struct {
		name           string
		points, wins, games int
	}{
		{"HighPts", 9, 3, 3}, // 9 points, easy schedule
		{"HighOMW", 6, 2, 3}, // 6 points, hard schedule
	})
	// HighPts faced weak opponents; HighOMW faced strong.
	st[0].OpponentMatchWinPct = 0.33
	st[1].OpponentMatchWinPct = 0.95

	ranked := sortedStandings(st)
	if ranked[0].CommanderName != "HighPts" {
		t.Errorf("Points must dominate OMW%%; ranking: %v", ranked)
	}
}

// TestSwissRanking_NameOnlyAtTrueTie pins that two decks tied on
// EVERY meaningful key fall through to alphabetical, deterministic.
func TestSwissRanking_NameOnlyAtTrueTie(t *testing.T) {
	st := makeStandings([]struct {
		name           string
		points, wins, games int
	}{
		{"Bravo", 6, 2, 3},
		{"Alpha", 6, 2, 3},
	})
	st[0].OpponentMatchWinPct = 0.5
	st[1].OpponentMatchWinPct = 0.5

	ranked := sortedStandings(st)
	if ranked[0].CommanderName != "Alpha" || ranked[1].CommanderName != "Bravo" {
		t.Errorf("true tie should sort alphabetically; got %v", ranked)
	}
}

// TestSwissOMW_OutputFieldPopulated pins that the schema-tagged
// OpponentMatchWinPct field is non-zero after a real Swiss
// tournament with games played. End-to-end smoke that the
// finalize wiring actually fires.
func TestSwissOMW_OutputFieldPopulated(t *testing.T) {
	decks := loadDecksOrSkip(t, 4)
	r, err := RunSwiss(SwissConfig{
		Decks:         decks,
		NSeats:        4,
		Rounds:        2,
		GamesPerPod:   1,
		Seed:          17,
		Workers:       1,
		CommanderMode: true,
	})
	if err != nil {
		t.Fatalf("RunSwiss: %v", err)
	}
	if len(r.SwissStandings) == 0 {
		t.Fatal("standings missing")
	}
	// At least one deck must have a non-zero OMW (everyone faced
	// SOME opponent across 2 rounds in a 1-pod tournament).
	anyNonZero := false
	for _, s := range r.SwissStandings {
		if s.OpponentMatchWinPct > 0 {
			anyNonZero = true
			break
		}
	}
	if !anyNonZero {
		t.Errorf("expected at least one non-zero OMW; got %+v", r.SwissStandings)
	}
}
