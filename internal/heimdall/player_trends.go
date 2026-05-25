package heimdall

import (
	"sort"
)

// PlayerTrends is the rolling-window summary returned by the
// /api/players/{id}/trends endpoint. The window is whatever the
// caller passed in (default 30 games); WindowSize is the request
// shape, GamesPlayed is the actual number of records found.
//
// WinRate is wins / games_played as a 0..1 fraction (not percent).
// Empty when GamesPlayed == 0.
//
// ArchetypeTrends is the per-archetype recent-vs-prior winrate
// comparison (e.g., "Control archetype winrate dropped from 27% to
// 22% over the last N games"). Empty when GamesPlayed < the minimum
// split-window threshold.
type PlayerTrends struct {
	PlayerID          string               `json:"player_id"`
	WindowSize        int                  `json:"window_size"`
	GamesPlayed       int                  `json:"games_played"`
	Wins              int                  `json:"wins"`
	Losses            int                  `json:"losses"`
	Draws             int                  `json:"draws"`
	WinRate           float64              `json:"win_rate"`
	Archetypes        []ArchetypeBreakdown `json:"archetype_distribution"`
	ArchetypeTrends   []ArchetypeTrend     `json:"archetype_trends"`
	OpponentDiversity OpponentDiversity    `json:"opponent_diversity"`
}

// ArchetypeTrend reports the change in an archetype's winrate between
// the most-recent half of the window and the immediately-prior half.
// The split is by game order: PlayerGameInput is expected to be
// newest-first (matching db.LoadOwnerGames' ORDER BY finished_at
// DESC), so the first len/2 games form RecentGames and the remainder
// form PriorGames.
//
// Delta is RecentWinRate − PriorWinRate (positive = climbing,
// negative = falling). Direction is the human-readable label:
//
//   - "up":   delta >= +archetypeTrendDeltaThreshold (5pp)
//   - "down": delta <= -archetypeTrendDeltaThreshold
//   - "flat": |delta| < threshold AND both sides have enough samples
//   - "insufficient_data": either side has fewer than
//     archetypeTrendMinSideSamples games (3) — winrates printed but
//     the swing isn't meaningful at this sample size
type ArchetypeTrend struct {
	Archetype     string  `json:"archetype"`
	RecentGames   int     `json:"recent_games"`
	RecentWins    int     `json:"recent_wins"`
	RecentWinRate float64 `json:"recent_win_rate"`
	PriorGames    int     `json:"prior_games"`
	PriorWins     int     `json:"prior_wins"`
	PriorWinRate  float64 `json:"prior_win_rate"`
	Delta         float64 `json:"delta"`
	Direction     string  `json:"direction"`
}

// ArchetypeBreakdown counts games + wins for one archetype slug
// across the window. Archetype is the freya slug ("voltron",
// "combo", "midrange", ...); the special value "unknown" carries any
// game whose archetype lookup returned empty.
type ArchetypeBreakdown struct {
	Archetype string  `json:"archetype"`
	Games     int     `json:"games"`
	Wins      int     `json:"wins"`
	WinRate   float64 `json:"win_rate"`
}

// OpponentDiversity describes how many distinct opponents the player
// has faced in the window. TotalEncounters is the sum of
// per-game opponent seats observed (3 per 4-player game), so
// DiversityRatio = UniqueCommanders / TotalEncounters in [0, 1].
//
// TopRepeatOpponents lists the commanders the player has faced more
// than once, sorted by encounter count descending. Capped at
// playerTrendsTopRepeatN entries.
type OpponentDiversity struct {
	UniqueCommanders   int              `json:"unique_commanders"`
	TotalEncounters    int              `json:"total_encounters"`
	DiversityRatio     float64          `json:"diversity_ratio"`
	TopRepeatOpponents []RepeatOpponent `json:"top_repeat_opponents,omitempty"`
}

// RepeatOpponent records one (commander, encounter-count) pair for
// the OpponentDiversity.TopRepeatOpponents list.
type RepeatOpponent struct {
	Commander  string `json:"commander"`
	Encounters int    `json:"encounters"`
}

// PlayerGameInput is one row of game history fed into
// ComputePlayerTrends. The pure-computation function is decoupled
// from the DB layer — callers (hexapi handlers, tests) translate
// their game rows into this shape.
//
// Won is true when the player won this game. Draw is true when the
// game ended without a winner (winner_seat < 0). A row can't be both.
// DeckKey is the owner/id deck key the player piloted; empty when
// the deck mapping is missing. OpponentCmdrs holds the commander
// names of the other 3 seats (or however many opponents existed).
type PlayerGameInput struct {
	Won           bool
	Draw          bool
	DeckKey       string
	OpponentCmdrs []string
}

const (
	playerTrendsTopRepeatN  = 5
	playerTrendsUnknownSlug = "unknown"

	// archetypeTrendMinTotalGames is the minimum GamesPlayed required
	// before the split-window archetype-trend section computes at
	// all. 4 means the smaller half has at least 2 games — below this
	// even a single win/loss flip swings winrate by 50pp+ and the
	// trend signal is pure noise.
	archetypeTrendMinTotalGames = 4

	// archetypeTrendMinSideSamples is the per-side minimum game count
	// for a trend to be marked anything other than "insufficient_data".
	// 3 keeps a single-win swing under 34pp.
	archetypeTrendMinSideSamples = 3

	// archetypeTrendDeltaThreshold is the swing magnitude (in absolute
	// winrate, 0..1) that separates "up"/"down" from "flat". 0.05 =
	// 5 percentage points, matching the "noticeable shift in the
	// dashboard" framing from the user-trends spec.
	archetypeTrendDeltaThreshold = 0.05
)

// ComputePlayerTrends turns a slice of recent game inputs into a
// PlayerTrends summary. lookupArchetype maps a deck key to its
// archetype slug (e.g. "voltron"); pass nil to skip archetype
// resolution (every game lands in the "unknown" bucket). When the
// lookup returns the empty string for a known deck, the game also
// lands in "unknown" — useful for decks that haven't been analyzed
// by Freya yet.
//
// Window is the caller-requested window size (echoed back in the
// response). GamesPlayed is the actual len(games) — when the player
// has fewer than `window` games on record we surface what's there
// rather than padding.
func ComputePlayerTrends(playerID string, window int, games []PlayerGameInput, lookupArchetype func(deckKey string) string) PlayerTrends {
	out := PlayerTrends{
		PlayerID:        playerID,
		WindowSize:      window,
		GamesPlayed:     len(games),
		Archetypes:      []ArchetypeBreakdown{},
		ArchetypeTrends: []ArchetypeTrend{},
		OpponentDiversity: OpponentDiversity{
			TopRepeatOpponents: []RepeatOpponent{},
		},
	}
	if len(games) == 0 {
		return out
	}

	archIndex := make(map[string]*ArchetypeBreakdown)
	oppCounts := make(map[string]int)
	totalEnc := 0

	for _, g := range games {
		switch {
		case g.Won:
			out.Wins++
		case g.Draw:
			out.Draws++
		default:
			out.Losses++
		}

		slug := playerTrendsUnknownSlug
		if lookupArchetype != nil && g.DeckKey != "" {
			if resolved := lookupArchetype(g.DeckKey); resolved != "" {
				slug = resolved
			}
		}
		entry, ok := archIndex[slug]
		if !ok {
			entry = &ArchetypeBreakdown{Archetype: slug}
			archIndex[slug] = entry
		}
		entry.Games++
		if g.Won {
			entry.Wins++
		}

		for _, cmdr := range g.OpponentCmdrs {
			if cmdr == "" {
				continue
			}
			oppCounts[cmdr]++
			totalEnc++
		}
	}

	if out.GamesPlayed > 0 {
		out.WinRate = float64(out.Wins) / float64(out.GamesPlayed)
	}

	// Archetype breakdown sorted by games desc, then archetype asc
	// for deterministic output.
	out.Archetypes = make([]ArchetypeBreakdown, 0, len(archIndex))
	for _, e := range archIndex {
		if e.Games > 0 {
			e.WinRate = float64(e.Wins) / float64(e.Games)
		}
		out.Archetypes = append(out.Archetypes, *e)
	}
	sort.SliceStable(out.Archetypes, func(i, j int) bool {
		if out.Archetypes[i].Games != out.Archetypes[j].Games {
			return out.Archetypes[i].Games > out.Archetypes[j].Games
		}
		return out.Archetypes[i].Archetype < out.Archetypes[j].Archetype
	})

	// Opponent diversity.
	out.OpponentDiversity.UniqueCommanders = len(oppCounts)
	out.OpponentDiversity.TotalEncounters = totalEnc
	if totalEnc > 0 {
		out.OpponentDiversity.DiversityRatio = float64(len(oppCounts)) / float64(totalEnc)
	}
	repeats := make([]RepeatOpponent, 0)
	for cmdr, n := range oppCounts {
		if n > 1 {
			repeats = append(repeats, RepeatOpponent{Commander: cmdr, Encounters: n})
		}
	}
	sort.SliceStable(repeats, func(i, j int) bool {
		if repeats[i].Encounters != repeats[j].Encounters {
			return repeats[i].Encounters > repeats[j].Encounters
		}
		return repeats[i].Commander < repeats[j].Commander
	})
	if len(repeats) > playerTrendsTopRepeatN {
		repeats = repeats[:playerTrendsTopRepeatN]
	}
	out.OpponentDiversity.TopRepeatOpponents = repeats

	out.ArchetypeTrends = computeArchetypeTrends(games, lookupArchetype)

	return out
}

// computeArchetypeTrends splits `games` in half by index (caller
// passes newest-first, so games[0:len/2] is RECENT and games[len/2:]
// is PRIOR) and computes a per-archetype winrate delta.
//
// Returns an empty slice when len(games) < archetypeTrendMinTotalGames
// — at smaller window sizes the split-half noise drowns out any
// signal worth surfacing.
//
// Sorted by absolute Delta desc (most-changed first) so a frontend
// rendering "what's changed" can take the top N without re-sorting.
// Tiebreak by archetype name ascending for deterministic output.
// "insufficient_data" rows sort to the bottom regardless of |delta|.
func computeArchetypeTrends(games []PlayerGameInput, lookupArchetype func(deckKey string) string) []ArchetypeTrend {
	if len(games) < archetypeTrendMinTotalGames {
		return []ArchetypeTrend{}
	}

	split := len(games) / 2
	recent := games[:split]
	prior := games[split:]

	type halfStats struct {
		games int
		wins  int
	}
	recentByArch := make(map[string]*halfStats)
	priorByArch := make(map[string]*halfStats)

	tally := func(g PlayerGameInput, bucket map[string]*halfStats) {
		slug := playerTrendsUnknownSlug
		if lookupArchetype != nil && g.DeckKey != "" {
			if resolved := lookupArchetype(g.DeckKey); resolved != "" {
				slug = resolved
			}
		}
		s, ok := bucket[slug]
		if !ok {
			s = &halfStats{}
			bucket[slug] = s
		}
		s.games++
		if g.Won {
			s.wins++
		}
	}
	for _, g := range recent {
		tally(g, recentByArch)
	}
	for _, g := range prior {
		tally(g, priorByArch)
	}

	// Union of archetypes seen in either half.
	seen := make(map[string]bool, len(recentByArch)+len(priorByArch))
	for k := range recentByArch {
		seen[k] = true
	}
	for k := range priorByArch {
		seen[k] = true
	}

	out := make([]ArchetypeTrend, 0, len(seen))
	for arch := range seen {
		var rGames, rWins, pGames, pWins int
		if s, ok := recentByArch[arch]; ok {
			rGames, rWins = s.games, s.wins
		}
		if s, ok := priorByArch[arch]; ok {
			pGames, pWins = s.games, s.wins
		}
		var rRate, pRate float64
		if rGames > 0 {
			rRate = float64(rWins) / float64(rGames)
		}
		if pGames > 0 {
			pRate = float64(pWins) / float64(pGames)
		}
		// Delta is recent - prior. When a side has zero games the
		// rate is 0 and the delta would be misleading, so we still
		// emit the row (the recent/prior counts are useful) but mark
		// direction as insufficient_data.
		delta := rRate - pRate
		direction := classifyTrendDirection(delta, rGames, pGames)
		out = append(out, ArchetypeTrend{
			Archetype:     arch,
			RecentGames:   rGames,
			RecentWins:    rWins,
			RecentWinRate: rRate,
			PriorGames:    pGames,
			PriorWins:     pWins,
			PriorWinRate:  pRate,
			Delta:         delta,
			Direction:     direction,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		iInsuff := out[i].Direction == "insufficient_data"
		jInsuff := out[j].Direction == "insufficient_data"
		if iInsuff != jInsuff {
			return !iInsuff // non-insufficient sorts first
		}
		ai := absFloat(out[i].Delta)
		aj := absFloat(out[j].Delta)
		if ai != aj {
			return ai > aj
		}
		return out[i].Archetype < out[j].Archetype
	})
	return out
}

func classifyTrendDirection(delta float64, recentGames, priorGames int) string {
	if recentGames < archetypeTrendMinSideSamples || priorGames < archetypeTrendMinSideSamples {
		return "insufficient_data"
	}
	switch {
	case delta >= archetypeTrendDeltaThreshold:
		return "up"
	case delta <= -archetypeTrendDeltaThreshold:
		return "down"
	default:
		return "flat"
	}
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
