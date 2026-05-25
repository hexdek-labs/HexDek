package heimdall

import (
	"sort"
)

// MetaEvolution is the response shape for /api/meta/evolution — a
// joint per-archetype week-by-week PLAY-RATE + WIN-RATE timeline,
// with pre-computed "biggest movers" lists for each dimension so the
// dashboard can render "what changed in the meta" without re-sorting.
//
// Distinct from MetaTrends (/api/meta/trends): MetaTrends only
// tracks winrate. MetaEvolution adds the play-rate (share of total
// games) axis — useful for spotting archetypes that are growing in
// popularity but not necessarily winning more, or shrinking out of
// the meta entirely.
type MetaEvolution struct {
	Weeks      int  `json:"weeks"`
	StartUnix  int64 `json:"start_unix"`
	EndUnix    int64 `json:"end_unix"`
	TotalGames int  `json:"total_games"`

	Archetypes []ArchetypeEvolution `json:"archetypes"`

	BiggestPlayRateGainers []ArchetypeEvolution `json:"biggest_play_rate_gainers"`
	BiggestPlayRateLosers  []ArchetypeEvolution `json:"biggest_play_rate_losers"`
	BiggestWinRateGainers  []ArchetypeEvolution `json:"biggest_win_rate_gainers"`
	BiggestWinRateLosers   []ArchetypeEvolution `json:"biggest_win_rate_losers"`
}

// ArchetypeEvolution is the per-archetype rollup. The Weekly slice
// gives the full timeline; the Overall* fields summarize the window;
// the *Delta fields and *Direction labels describe the recent-half-
// vs-prior-half shift in each dimension.
type ArchetypeEvolution struct {
	Archetype string                  `json:"archetype"`
	Weekly    []ArchetypeWeeklyShare  `json:"weekly"`

	OverallGames    int     `json:"overall_games"`
	OverallWins     int     `json:"overall_wins"`
	OverallWinRate  float64 `json:"overall_win_rate"`
	OverallPlayRate float64 `json:"overall_play_rate"`

	PlayRateDelta     float64 `json:"play_rate_delta"`
	PlayRateDirection string  `json:"play_rate_direction"`
	WinRateDelta      float64 `json:"win_rate_delta"`
	WinRateDirection  string  `json:"win_rate_direction"`
}

// ArchetypeWeeklyShare is one week's row for a single archetype.
// TotalGamesWeek is the COMBINED game count across ALL archetypes
// in that week — it's the denominator for PlayRate. WinRate uses
// the archetype's own Games / Wins.
type ArchetypeWeeklyShare struct {
	WeekStart      int64   `json:"week_start"`
	Games          int     `json:"games"`
	Wins           int     `json:"wins"`
	TotalGamesWeek int     `json:"total_games_week"`
	PlayRate       float64 `json:"play_rate"`
	WinRate        float64 `json:"win_rate"`
}

const (
	// Play-rate threshold: a 2pp shift in share-of-games is the
	// noticeable-meta-shift threshold (tighter than winrate's 3pp
	// because play-rate is a smaller-magnitude metric — most
	// archetypes sit at 1-15% of the meta, so a 2pp swing is real).
	metaEvolutionPlayRateDeltaThreshold = 0.02
	// Win-rate threshold: matches MetaTrends.
	metaEvolutionWinRateDeltaThreshold = 0.03

	metaEvolutionMinSideSamples = 10
	metaEvolutionBiggestN       = 5
)

// ComputeMetaEvolution builds the joint play-rate + win-rate timeline.
// Inputs match ComputeMetaArchetypeTrends so the same handler-side
// data pipeline (db.LoadMetaSeatOutcomes + deck-key→archetype lookup)
// feeds both endpoints.
//
// refUnix is the wall-clock anchor (most-recent bucket ends here).
// weeks clamps to [1, metaTrendsMaxWeeks] per the MetaTrends limits.
// Games outside [refUnix - weeks*7d, refUnix) are silently skipped.
//
// Archetypes are sorted by OverallGames desc with alpha tiebreak.
// BiggestPlayRateGainers / BiggestWinRateGainers sort by their
// respective Δ desc; the Losers lists sort asc; all four exclude
// "insufficient_data" rows.
func ComputeMetaEvolution(games []MetaGameInput, refUnix int64, weeks int) MetaEvolution {
	if weeks <= 0 {
		weeks = metaTrendsDefaultWeeks
	}
	if weeks > metaTrendsMaxWeeks {
		weeks = metaTrendsMaxWeeks
	}

	startUnix := refUnix - int64(weeks)*metaTrendsSecondsPerWeek
	endUnix := refUnix - metaTrendsSecondsPerWeek

	out := MetaEvolution{
		Weeks:                  weeks,
		StartUnix:              startUnix,
		EndUnix:                endUnix,
		Archetypes:             []ArchetypeEvolution{},
		BiggestPlayRateGainers: []ArchetypeEvolution{},
		BiggestPlayRateLosers:  []ArchetypeEvolution{},
		BiggestWinRateGainers:  []ArchetypeEvolution{},
		BiggestWinRateLosers:   []ArchetypeEvolution{},
	}
	if len(games) == 0 {
		return out
	}

	type weekBucket struct{ games, wins int }
	type archAcc struct {
		buckets []weekBucket
		total   weekBucket
	}
	by := make(map[string]*archAcc)
	totalByWeek := make([]int, weeks)

	getAcc := func(slug string) *archAcc {
		a, ok := by[slug]
		if !ok {
			a = &archAcc{buckets: make([]weekBucket, weeks)}
			by[slug] = a
		}
		return a
	}

	for _, g := range games {
		if g.FinishedAt < startUnix || g.FinishedAt >= refUnix {
			continue
		}
		idx := int((g.FinishedAt - startUnix) / metaTrendsSecondsPerWeek)
		if idx < 0 || idx >= weeks {
			continue
		}
		out.TotalGames++
		totalByWeek[idx]++

		slug := g.Archetype
		if slug == "" {
			slug = playerTrendsUnknownSlug
		}
		a := getAcc(slug)
		a.buckets[idx].games++
		a.total.games++
		if g.Won {
			a.buckets[idx].wins++
			a.total.wins++
		}
	}

	rows := make([]ArchetypeEvolution, 0, len(by))
	for slug, acc := range by {
		weekly := make([]ArchetypeWeeklyShare, weeks)
		for i, b := range acc.buckets {
			w := ArchetypeWeeklyShare{
				WeekStart:      startUnix + int64(i)*metaTrendsSecondsPerWeek,
				Games:          b.games,
				Wins:           b.wins,
				TotalGamesWeek: totalByWeek[i],
			}
			if totalByWeek[i] > 0 {
				w.PlayRate = float64(b.games) / float64(totalByWeek[i])
			}
			if b.games > 0 {
				w.WinRate = float64(b.wins) / float64(b.games)
			}
			weekly[i] = w
		}

		// Half-window split: prior = first half of buckets, recent =
		// second half. Computes both delta dimensions from the same
		// split so they're directly comparable.
		split := weeks / 2
		var recentGames, recentWins, priorGames, priorWins int
		var recentTotal, priorTotal int
		for i := 0; i < split; i++ {
			priorGames += acc.buckets[i].games
			priorWins += acc.buckets[i].wins
			priorTotal += totalByWeek[i]
		}
		for i := split; i < weeks; i++ {
			recentGames += acc.buckets[i].games
			recentWins += acc.buckets[i].wins
			recentTotal += totalByWeek[i]
		}

		var recentPlay, priorPlay float64
		if recentTotal > 0 {
			recentPlay = float64(recentGames) / float64(recentTotal)
		}
		if priorTotal > 0 {
			priorPlay = float64(priorGames) / float64(priorTotal)
		}
		playDelta := recentPlay - priorPlay

		var recentWR, priorWR float64
		if recentGames > 0 {
			recentWR = float64(recentWins) / float64(recentGames)
		}
		if priorGames > 0 {
			priorWR = float64(priorWins) / float64(priorGames)
		}
		winDelta := recentWR - priorWR

		row := ArchetypeEvolution{
			Archetype: slug,
			Weekly:    weekly,
			OverallGames: acc.total.games,
			OverallWins:  acc.total.wins,
			PlayRateDelta:     playDelta,
			PlayRateDirection: classifyEvolutionDirection(playDelta, recentGames, priorGames, split, metaEvolutionPlayRateDeltaThreshold),
			WinRateDelta:      winDelta,
			WinRateDirection:  classifyEvolutionDirection(winDelta, recentGames, priorGames, split, metaEvolutionWinRateDeltaThreshold),
		}
		if acc.total.games > 0 {
			row.OverallWinRate = float64(acc.total.wins) / float64(acc.total.games)
		}
		if out.TotalGames > 0 {
			row.OverallPlayRate = float64(acc.total.games) / float64(out.TotalGames)
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].OverallGames != rows[j].OverallGames {
			return rows[i].OverallGames > rows[j].OverallGames
		}
		return rows[i].Archetype < rows[j].Archetype
	})
	out.Archetypes = rows

	// Mover lists. Each list filters out insufficient_data on its own
	// axis (an archetype can be insufficient on win-rate but well-
	// sampled on play-rate, or vice versa).
	out.BiggestPlayRateGainers = topMoversBy(rows, true /*positive*/, true /*play*/)
	out.BiggestPlayRateLosers = topMoversBy(rows, false, true)
	out.BiggestWinRateGainers = topMoversBy(rows, true, false)
	out.BiggestWinRateLosers = topMoversBy(rows, false, false)

	return out
}

func classifyEvolutionDirection(delta float64, recentGames, priorGames, split int, threshold float64) string {
	if split == 0 {
		return "insufficient_data"
	}
	if recentGames < metaEvolutionMinSideSamples || priorGames < metaEvolutionMinSideSamples {
		return "insufficient_data"
	}
	switch {
	case delta >= threshold:
		return "up"
	case delta <= -threshold:
		return "down"
	default:
		return "flat"
	}
}

// topMoversBy returns up to metaEvolutionBiggestN archetype rows
// sorted by the requested dimension's Δ — desc for gainers, asc
// for losers. positive=true wants gainers (Δ > 0); positive=false
// wants losers (Δ < 0). play=true uses the play-rate axis; play=
// false uses win-rate. Rows whose Direction on the requested axis
// is "insufficient_data" are excluded.
func topMoversBy(rows []ArchetypeEvolution, positive, play bool) []ArchetypeEvolution {
	filtered := make([]ArchetypeEvolution, 0, len(rows))
	for _, r := range rows {
		var d float64
		var dir string
		if play {
			d, dir = r.PlayRateDelta, r.PlayRateDirection
		} else {
			d, dir = r.WinRateDelta, r.WinRateDirection
		}
		if dir == "insufficient_data" {
			continue
		}
		if positive && d <= 0 {
			continue
		}
		if !positive && d >= 0 {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		var di, dj float64
		if play {
			di, dj = filtered[i].PlayRateDelta, filtered[j].PlayRateDelta
		} else {
			di, dj = filtered[i].WinRateDelta, filtered[j].WinRateDelta
		}
		if positive {
			if di != dj {
				return di > dj
			}
		} else {
			if di != dj {
				return di < dj
			}
		}
		return filtered[i].Archetype < filtered[j].Archetype
	})
	if len(filtered) > metaEvolutionBiggestN {
		filtered = filtered[:metaEvolutionBiggestN]
	}
	if filtered == nil {
		return []ArchetypeEvolution{}
	}
	return filtered
}
