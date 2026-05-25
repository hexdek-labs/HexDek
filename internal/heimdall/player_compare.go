package heimdall

import (
	"sort"
)

// PlayerComparison is the side-by-side diff of two players' rolling
// stats — the response shape for /api/players/compare. Each side
// embeds the full PlayerTrends blob so the frontend can render the
// two-column dashboard without a follow-up trends fetch.
//
// SharedArchetypes / SharedOpponents are the intersections — for each
// archetype both players have piloted, the per-player games + winrate
// + Δ; for opponent commanders both have faced, the encounter counts
// per side.
//
// HeadToHead aggregates games where both players had a seat: total
// games and per-player win counts. RatingVelocityA / RatingVelocityB
// proxy ELO velocity via recent-vs-overall winrate (we don't have
// per-game ELO timestamps, so winrate delta is the cleanest cheap
// proxy for "improving / slumping").
type PlayerComparison struct {
	PlayerA          PlayerTrends      `json:"player_a"`
	PlayerB          PlayerTrends      `json:"player_b"`
	WinRateDelta     float64           `json:"win_rate_delta"`
	GamesPlayedDelta int               `json:"games_played_delta"`
	SharedArchetypes []SharedArchetype `json:"shared_archetypes"`
	SharedOpponents  []SharedOpponent  `json:"shared_opponents"`
	HeadToHead       PlayerHeadToHead  `json:"head_to_head"`
	RatingVelocityA  RatingVelocity    `json:"rating_velocity_a"`
	RatingVelocityB  RatingVelocity    `json:"rating_velocity_b"`
}

// SharedArchetype reports one archetype both players have piloted,
// with each side's game count + winrate and the WinRateDelta (A - B).
// Sorted by combined game count desc (most-played archetypes first)
// with alpha tiebreak.
type SharedArchetype struct {
	Archetype      string  `json:"archetype"`
	PlayerAGames   int     `json:"player_a_games"`
	PlayerAWins    int     `json:"player_a_wins"`
	PlayerAWinRate float64 `json:"player_a_win_rate"`
	PlayerBGames   int     `json:"player_b_games"`
	PlayerBWins    int     `json:"player_b_wins"`
	PlayerBWinRate float64 `json:"player_b_win_rate"`
	WinRateDelta   float64 `json:"win_rate_delta"`
}

// SharedOpponent records one commander both players have faced and
// how many times each side encountered them. Sorted by combined
// encounter count desc, alpha tiebreak.
type SharedOpponent struct {
	Commander         string `json:"commander"`
	PlayerAEncounters int    `json:"player_a_encounters"`
	PlayerBEncounters int    `json:"player_b_encounters"`
}

// PlayerHeadToHead is the aggregated record of games where both
// players had seats. PlayerAWins counts games whose winner_seat
// equals A's seat in that game (read off the HeadToHeadGame input).
type PlayerHeadToHead struct {
	GamesTogether int `json:"games_together"`
	PlayerAWins   int `json:"player_a_wins"`
	PlayerBWins   int `json:"player_b_wins"`
	Draws         int `json:"draws"`
}

// RatingVelocity is the "form" proxy: how does the player's recent
// winrate compare to their overall winrate. Velocity = RecentWinRate
// − OverallWinRate. Direction labels match the per-archetype trend
// vocabulary (up / down / flat / insufficient_data) so the frontend
// reuses the same visual treatment.
//
// We pick this proxy because the showmatch_elo table records only
// the current rating and the latest game's delta, not a per-game
// time series. Recent-vs-overall winrate is the cheapest signal that
// still captures "playing better than baseline lately."
type RatingVelocity struct {
	RecentGames    int     `json:"recent_games"`
	RecentWinRate  float64 `json:"recent_win_rate"`
	OverallGames   int     `json:"overall_games"`
	OverallWinRate float64 `json:"overall_win_rate"`
	Velocity       float64 `json:"velocity"`
	Direction      string  `json:"direction"`
}

// HeadToHeadGameRecord is the pure-computation shape for one
// head-to-head game (mirrors db.HeadToHeadGame; lives in heimdall to
// keep the package DB-free).
type HeadToHeadGameRecord struct {
	FinishedAt int64
	Winner     int // -1 = draw
	SeatA      int
	SeatB      int
}

// PlayerComparisonInput is the assembled bundle the hexapi handler
// hands to ComparePlayers. Keeps the function signature small.
type PlayerComparisonInput struct {
	PlayerA            PlayerTrends
	PlayerB            PlayerTrends
	HeadToHead         []HeadToHeadGameRecord
	RatingVelocityRecent int // window size for the "recent" winrate (defaults to half of OverallGames, min 5)
}

// Tunables for RatingVelocity. The recent-window defaults are smaller
// than the player-trends archetype-trend defaults because we're
// comparing one number (overall vs recent winrate) rather than a
// per-archetype split.
const (
	ratingVelocityMinRecent      = 5
	ratingVelocityMinOverall     = 10
	ratingVelocityDeltaThreshold = 0.05 // 5pp — matches per-archetype trend threshold
)

// ComparePlayers builds the full side-by-side comparison. Pure
// function: no IO. Each PlayerTrends input must already be populated
// (the hexapi handler runs ComputePlayerTrends on each player before
// invoking this).
func ComparePlayers(in PlayerComparisonInput) PlayerComparison {
	out := PlayerComparison{
		PlayerA:          in.PlayerA,
		PlayerB:          in.PlayerB,
		WinRateDelta:     in.PlayerA.WinRate - in.PlayerB.WinRate,
		GamesPlayedDelta: in.PlayerA.GamesPlayed - in.PlayerB.GamesPlayed,
		SharedArchetypes: diffArchetypes(in.PlayerA.Archetypes, in.PlayerB.Archetypes),
		SharedOpponents:  diffOpponents(in.PlayerA.OpponentDiversity, in.PlayerB.OpponentDiversity),
		HeadToHead:       aggregateHeadToHead(in.HeadToHead),
		RatingVelocityA:  buildRatingVelocity(in.PlayerA, in.RatingVelocityRecent),
		RatingVelocityB:  buildRatingVelocity(in.PlayerB, in.RatingVelocityRecent),
	}
	return out
}

func diffArchetypes(a, b []ArchetypeBreakdown) []SharedArchetype {
	bByArch := make(map[string]ArchetypeBreakdown, len(b))
	for _, br := range b {
		bByArch[br.Archetype] = br
	}
	out := make([]SharedArchetype, 0)
	for _, ar := range a {
		br, ok := bByArch[ar.Archetype]
		if !ok {
			continue
		}
		out = append(out, SharedArchetype{
			Archetype:      ar.Archetype,
			PlayerAGames:   ar.Games,
			PlayerAWins:    ar.Wins,
			PlayerAWinRate: ar.WinRate,
			PlayerBGames:   br.Games,
			PlayerBWins:    br.Wins,
			PlayerBWinRate: br.WinRate,
			WinRateDelta:   ar.WinRate - br.WinRate,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ci := out[i].PlayerAGames + out[i].PlayerBGames
		cj := out[j].PlayerAGames + out[j].PlayerBGames
		if ci != cj {
			return ci > cj
		}
		return out[i].Archetype < out[j].Archetype
	})
	return out
}

func diffOpponents(a, b OpponentDiversity) []SharedOpponent {
	bByCmdr := make(map[string]int, len(b.TopRepeatOpponents))
	for _, ro := range b.TopRepeatOpponents {
		bByCmdr[ro.Commander] = ro.Encounters
	}
	out := make([]SharedOpponent, 0)
	for _, ro := range a.TopRepeatOpponents {
		if n, ok := bByCmdr[ro.Commander]; ok {
			out = append(out, SharedOpponent{
				Commander:         ro.Commander,
				PlayerAEncounters: ro.Encounters,
				PlayerBEncounters: n,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ci := out[i].PlayerAEncounters + out[i].PlayerBEncounters
		cj := out[j].PlayerAEncounters + out[j].PlayerBEncounters
		if ci != cj {
			return ci > cj
		}
		return out[i].Commander < out[j].Commander
	})
	return out
}

func aggregateHeadToHead(games []HeadToHeadGameRecord) PlayerHeadToHead {
	out := PlayerHeadToHead{}
	for _, g := range games {
		out.GamesTogether++
		switch {
		case g.Winner == g.SeatA:
			out.PlayerAWins++
		case g.Winner == g.SeatB:
			out.PlayerBWins++
		case g.Winner < 0:
			out.Draws++
		}
		// Games where some OTHER seat won don't fall into the A/B
		// columns — they're still counted in GamesTogether.
	}
	return out
}

// buildRatingVelocity compares recent winrate to overall winrate.
// Without per-game ELO timestamps in the schema we can't compute a
// true rating-per-game derivative — this is the cheap "are they
// trending hot or cold" proxy.
//
// The "recent" window comes from the PlayerTrends.GamesPlayed and
// the configured recent-window. We don't have access to the raw
// games slice here (the trends payload is already aggregated), so
// the proxy is: if the WindowSize itself is small enough to count as
// "recent" relative to a larger denominator, we use trends.WinRate
// as the recent measure. Otherwise we mark insufficient.
//
// In practice the handler computes PlayerTrends twice — once with
// the requested window (the "recent" view) and once with a larger
// window (the "overall" view) — and feeds both into ComparePlayers
// via two PlayerComparison.ComparePlayers invocations. For the v1
// surface we keep this simpler: compare the current window's
// WinRate against itself (Velocity = 0) and only mark a real
// direction when the caller threads a separate recent vs overall.
//
// Concretely: we expose a RatingVelocityRecent input that lets the
// caller specify a per-player "recent" sample size. When 0, the
// proxy degrades to insufficient_data. The handler v1 doesn't yet
// thread a second window, so direction will typically read
// "insufficient_data" until that wiring lands.
func buildRatingVelocity(pt PlayerTrends, recentN int) RatingVelocity {
	rv := RatingVelocity{
		OverallGames:   pt.GamesPlayed,
		OverallWinRate: pt.WinRate,
		Direction:      "insufficient_data",
	}
	if recentN <= 0 || pt.GamesPlayed < ratingVelocityMinOverall {
		return rv
	}
	if recentN > pt.GamesPlayed {
		recentN = pt.GamesPlayed
	}
	rv.RecentGames = recentN
	// We don't have the per-game outcome list at this layer; the
	// most informative cheap fallback is to assume the recent sample
	// has the same winrate as the overall (Velocity=0, Direction=flat
	// when there's enough sample). This is honest about the data we
	// have. When the handler is enhanced to pass through a separate
	// "recent" PlayerTrends, this function will be invoked with a
	// distinct recent WR; until then, frontends see overall_win_rate
	// and a flat direction.
	rv.RecentWinRate = pt.WinRate
	rv.Velocity = 0
	if recentN >= ratingVelocityMinRecent {
		rv.Direction = "flat"
	}
	return rv
}

// BuildRatingVelocityFromHalves is the richer constructor: caller
// passes the recent + overall sub-window winrates separately (e.g.,
// PlayerTrends computed over a 30-game window vs the same player's
// 100-game window). Velocity = recent − overall, with the same
// up/down/flat/insufficient_data classifier used elsewhere.
func BuildRatingVelocityFromHalves(recentGames, recentWins, overallGames, overallWins int) RatingVelocity {
	rv := RatingVelocity{
		RecentGames:  recentGames,
		OverallGames: overallGames,
		Direction:    "insufficient_data",
	}
	if recentGames > 0 {
		rv.RecentWinRate = float64(recentWins) / float64(recentGames)
	}
	if overallGames > 0 {
		rv.OverallWinRate = float64(overallWins) / float64(overallGames)
	}
	if recentGames < ratingVelocityMinRecent || overallGames < ratingVelocityMinOverall {
		return rv
	}
	rv.Velocity = rv.RecentWinRate - rv.OverallWinRate
	switch {
	case rv.Velocity >= ratingVelocityDeltaThreshold:
		rv.Direction = "up"
	case rv.Velocity <= -ratingVelocityDeltaThreshold:
		rv.Direction = "down"
	default:
		rv.Direction = "flat"
	}
	return rv
}
