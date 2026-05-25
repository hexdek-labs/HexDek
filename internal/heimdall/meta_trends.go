package heimdall

import (
	"sort"
)

// MetaTrends is the response shape for /api/meta/trends — per-
// archetype week-by-week winrate series across the full game
// population, plus a recent-vs-prior delta so a UI can highlight
// "biggest gainers" and "biggest losers" without re-aggregating.
//
// Weeks are aligned BACKWARD from the reference time the caller
// supplies (typically time.Now()). Week N-1 is the most recent
// [refUnix - secondsPerWeek, refUnix); week N-2 is the one before
// that; week 0 is the oldest. WeekStart on each MetaTrendWeek is the
// unix epoch of the [start, start+1week) bucket. Frontend renders
// the array left-to-right oldest → newest.
type MetaTrends struct {
	Weeks          int                  `json:"weeks"`
	StartUnix      int64                `json:"start_unix"` // start of the oldest bucket
	EndUnix        int64                `json:"end_unix"`   // start of the most-recent bucket (i.e., oldest + (weeks-1)*7d)
	TotalGames     int                  `json:"total_games"`
	Archetypes     []ArchetypeMetaTrend `json:"archetypes"`
	BiggestGainers []ArchetypeMetaTrend `json:"biggest_gainers"`
	BiggestLosers  []ArchetypeMetaTrend `json:"biggest_losers"`
}

// ArchetypeMetaTrend is the per-archetype rollup: the weekly series
// for the whole window, total games + winrate across the window, and
// a Delta describing the recent-half-vs-prior-half swing.
//
// Direction labels mirror the per-player archetype-trend semantics:
//   - "up":   Delta ≥ +metaTrendsDeltaThreshold
//   - "down": Delta ≤ -metaTrendsDeltaThreshold
//   - "flat": |Delta| < threshold AND both halves have ≥ metaTrendsMinSideSamples games
//   - "insufficient_data": either half has < metaTrendsMinSideSamples games
type ArchetypeMetaTrend struct {
	Archetype      string          `json:"archetype"`
	Weekly         []MetaTrendWeek `json:"weekly"`
	TotalGames     int             `json:"total_games"`
	TotalWins      int             `json:"total_wins"`
	OverallWinRate float64         `json:"overall_win_rate"`
	Delta          float64         `json:"delta"`
	Direction      string          `json:"direction"`
}

// MetaTrendWeek is one week's worth of data for a single archetype.
// Games may be 0 when the archetype wasn't played in that bucket;
// WinRate is 0 in that case.
type MetaTrendWeek struct {
	WeekStart int64   `json:"week_start"`
	Games     int     `json:"games"`
	Wins      int     `json:"wins"`
	WinRate   float64 `json:"win_rate"`
}

// MetaGameInput is one seat's outcome row fed into
// ComputeMetaArchetypeTrends. The caller (hexapi handler) takes
// db.MetaSeatOutcome and resolves the deck-key → archetype mapping
// once, then hands a slice of these to the pure-computation function.
//
// Archetype "" lands in the playerTrendsUnknownSlug bucket ("unknown")
// — useful when a deck hasn't been Freya-analyzed yet but still
// played in the window.
type MetaGameInput struct {
	FinishedAt int64
	Archetype  string
	Won        bool
}

const (
	metaTrendsDefaultWeeks = 4
	metaTrendsMaxWeeks     = 26
	metaTrendsSecondsPerWeek int64 = 7 * 24 * 3600

	// metaTrendsMinSideSamples is the per-side game count required
	// before the recent-vs-prior delta gets a non-insufficient
	// Direction. 10 keeps a single-win flip under 10pp swing.
	metaTrendsMinSideSamples = 10

	// metaTrendsDeltaThreshold is the |Δ| (0..1 absolute winrate)
	// separating "up"/"down" from "flat". 0.03 (3pp) — tighter than
	// the per-player 0.05 because the meta-wide sample is larger and
	// real shifts are smaller.
	metaTrendsDeltaThreshold = 0.03

	// metaTrendsBiggestN caps the BiggestGainers / BiggestLosers
	// slices. 5 each is enough for a dashboard sidebar; consumers
	// that want more can derive from the full Archetypes list.
	metaTrendsBiggestN = 5
)

// ComputeMetaArchetypeTrends turns a slice of seat-level outcomes
// into a per-archetype week-by-week meta-trends payload.
//
// refUnix is the wall-clock anchor — the most-recent bucket ends at
// refUnix and starts at refUnix - 7d. weeks is the requested window
// size (clamped to [1, metaTrendsMaxWeeks]). Games whose FinishedAt
// falls outside [refUnix - weeks*7d, refUnix) are silently skipped;
// it's the caller's job to query the right time range.
//
// Archetypes are sorted by TotalGames desc (most-played first) with
// alphabetical tiebreak. BiggestGainers / BiggestLosers are sorted
// by Delta desc / asc respectively, with "insufficient_data"
// archetypes excluded from both lists (no signal in the swing).
func ComputeMetaArchetypeTrends(games []MetaGameInput, refUnix int64, weeks int) MetaTrends {
	if weeks <= 0 {
		weeks = metaTrendsDefaultWeeks
	}
	if weeks > metaTrendsMaxWeeks {
		weeks = metaTrendsMaxWeeks
	}

	startUnix := refUnix - int64(weeks)*metaTrendsSecondsPerWeek
	endUnix := refUnix - metaTrendsSecondsPerWeek // start of the most recent bucket

	out := MetaTrends{
		Weeks:          weeks,
		StartUnix:      startUnix,
		EndUnix:        endUnix,
		Archetypes:     []ArchetypeMetaTrend{},
		BiggestGainers: []ArchetypeMetaTrend{},
		BiggestLosers:  []ArchetypeMetaTrend{},
	}
	if len(games) == 0 {
		return out
	}

	type weekBucket struct{ games, wins int }
	type archAcc struct {
		buckets []weekBucket // length == weeks
		total   weekBucket
	}
	by := make(map[string]*archAcc)
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
		out.TotalGames++
		slug := g.Archetype
		if slug == "" {
			slug = playerTrendsUnknownSlug
		}
		idx := int((g.FinishedAt - startUnix) / metaTrendsSecondsPerWeek)
		if idx < 0 || idx >= weeks {
			continue
		}
		a := getAcc(slug)
		a.buckets[idx].games++
		a.total.games++
		if g.Won {
			a.buckets[idx].wins++
			a.total.wins++
		}
	}

	rows := make([]ArchetypeMetaTrend, 0, len(by))
	for slug, acc := range by {
		weekly := make([]MetaTrendWeek, weeks)
		for i, b := range acc.buckets {
			weekly[i] = MetaTrendWeek{
				WeekStart: startUnix + int64(i)*metaTrendsSecondsPerWeek,
				Games:     b.games,
				Wins:      b.wins,
			}
			if b.games > 0 {
				weekly[i].WinRate = float64(b.wins) / float64(b.games)
			}
		}
		// Recent-vs-prior split: split the weeks slice in half (NOT the
		// games slice — week buckets give equal time weight). Recent =
		// the second half, Prior = the first half. weeks=1 makes the
		// split degenerate; we treat that as insufficient.
		split := weeks / 2
		var recentGames, recentWins, priorGames, priorWins int
		for i := 0; i < split; i++ {
			priorGames += acc.buckets[i].games
			priorWins += acc.buckets[i].wins
		}
		for i := split; i < weeks; i++ {
			recentGames += acc.buckets[i].games
			recentWins += acc.buckets[i].wins
		}
		var recentRate, priorRate float64
		if recentGames > 0 {
			recentRate = float64(recentWins) / float64(recentGames)
		}
		if priorGames > 0 {
			priorRate = float64(priorWins) / float64(priorGames)
		}
		delta := recentRate - priorRate
		direction := classifyMetaTrendDirection(delta, recentGames, priorGames, split)

		row := ArchetypeMetaTrend{
			Archetype:  slug,
			Weekly:     weekly,
			TotalGames: acc.total.games,
			TotalWins:  acc.total.wins,
			Delta:      delta,
			Direction:  direction,
		}
		if acc.total.games > 0 {
			row.OverallWinRate = float64(acc.total.wins) / float64(acc.total.games)
		}
		rows = append(rows, row)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].TotalGames != rows[j].TotalGames {
			return rows[i].TotalGames > rows[j].TotalGames
		}
		return rows[i].Archetype < rows[j].Archetype
	})
	out.Archetypes = rows

	// BiggestGainers / Losers: filter out insufficient_data, then sort
	// by Delta desc (for gainers) / asc (for losers). Stable sort with
	// archetype-asc tiebreak.
	var movers []ArchetypeMetaTrend
	for _, r := range rows {
		if r.Direction != "insufficient_data" {
			movers = append(movers, r)
		}
	}
	gainers := append([]ArchetypeMetaTrend(nil), movers...)
	losers := append([]ArchetypeMetaTrend(nil), movers...)
	sort.SliceStable(gainers, func(i, j int) bool {
		if gainers[i].Delta != gainers[j].Delta {
			return gainers[i].Delta > gainers[j].Delta
		}
		return gainers[i].Archetype < gainers[j].Archetype
	})
	sort.SliceStable(losers, func(i, j int) bool {
		if losers[i].Delta != losers[j].Delta {
			return losers[i].Delta < losers[j].Delta
		}
		return losers[i].Archetype < losers[j].Archetype
	})
	out.BiggestGainers = truncateMovers(filterPositiveDelta(gainers), metaTrendsBiggestN)
	out.BiggestLosers = truncateMovers(filterNegativeDelta(losers), metaTrendsBiggestN)
	return out
}

func classifyMetaTrendDirection(delta float64, recentGames, priorGames, split int) string {
	if split == 0 {
		return "insufficient_data"
	}
	if recentGames < metaTrendsMinSideSamples || priorGames < metaTrendsMinSideSamples {
		return "insufficient_data"
	}
	switch {
	case delta >= metaTrendsDeltaThreshold:
		return "up"
	case delta <= -metaTrendsDeltaThreshold:
		return "down"
	default:
		return "flat"
	}
}

func filterPositiveDelta(rows []ArchetypeMetaTrend) []ArchetypeMetaTrend {
	out := make([]ArchetypeMetaTrend, 0, len(rows))
	for _, r := range rows {
		if r.Delta > 0 {
			out = append(out, r)
		}
	}
	return out
}

func filterNegativeDelta(rows []ArchetypeMetaTrend) []ArchetypeMetaTrend {
	out := make([]ArchetypeMetaTrend, 0, len(rows))
	for _, r := range rows {
		if r.Delta < 0 {
			out = append(out, r)
		}
	}
	return out
}

func truncateMovers(rows []ArchetypeMetaTrend, n int) []ArchetypeMetaTrend {
	if len(rows) > n {
		rows = rows[:n]
	}
	if rows == nil {
		return []ArchetypeMetaTrend{}
	}
	return rows
}
