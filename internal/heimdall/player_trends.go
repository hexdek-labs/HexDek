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
type PlayerTrends struct {
	PlayerID          string               `json:"player_id"`
	WindowSize        int                  `json:"window_size"`
	GamesPlayed       int                  `json:"games_played"`
	Wins              int                  `json:"wins"`
	Losses            int                  `json:"losses"`
	Draws             int                  `json:"draws"`
	WinRate           float64              `json:"win_rate"`
	Archetypes        []ArchetypeBreakdown `json:"archetype_distribution"`
	OpponentDiversity OpponentDiversity    `json:"opponent_diversity"`
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
		PlayerID:    playerID,
		WindowSize:  window,
		GamesPlayed: len(games),
		Archetypes:  []ArchetypeBreakdown{},
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

	return out
}
