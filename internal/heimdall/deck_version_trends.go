package heimdall

import (
	"sort"
)

// DeckVersionEntry is one row of a deck's lineage in the shape
// heimdall.ComputeDeckVersionTrends consumes. The hexapi caller
// translates internal/versioning's VersionNode into this struct,
// parsing the node's RFC3339 CreatedAt into a Unix epoch so the
// bucketing walker doesn't repeat the parse per game.
//
// Lineage is expected ordered however the caller likes —
// ComputeDeckVersionTrends sorts internally.
type DeckVersionEntry struct {
	Hash      string
	Version   int
	CardCount int
	CardDelta int
	CreatedAt int64 // unix epoch seconds — when this version became HEAD
}

// DeckVersionOutcome is one game's per-deck outcome row, mirroring
// db.DeckGameOutcome. Lives in heimdall to keep the pure-computation
// signature DB-package-free.
type DeckVersionOutcome struct {
	FinishedAt int64
	Won        bool
	Draw       bool
}

// DeckVersionWinrate is the response shape per version. Games /
// Wins / Losses / Draws sum to the number of games bucketed into
// this version. WinRate is wins / games as a 0..1 fraction; 0 when
// Games == 0.
type DeckVersionWinrate struct {
	Hash      string  `json:"hash"`
	Version   int     `json:"version"`
	CardCount int     `json:"card_count"`
	CardDelta int     `json:"card_delta,omitempty"`
	CreatedAt int64   `json:"created_at"`
	Games     int     `json:"games"`
	Wins      int     `json:"wins"`
	Losses    int     `json:"losses"`
	Draws     int     `json:"draws"`
	WinRate   float64 `json:"win_rate"`
}

// DeckVersionTrends is the per-deck rollup returned by the trends
// endpoint. Versions is chronological (Version asc); UntrackedGames
// counts games that predate any known version (e.g., games played
// before the version DAG started tracking, or with timestamps that
// fall outside lineage). The frontend renders Untracked as an
// explanation row so users don't read "12 games tracked, 18 total"
// as a bug.
type DeckVersionTrends struct {
	Owner          string               `json:"owner"`
	DeckID         string               `json:"deck_id"`
	Versions       []DeckVersionWinrate `json:"versions"`
	UntrackedGames int                  `json:"untracked_games"`
}

// ComputeDeckVersionTrends buckets each game outcome into the
// version that was HEAD at the time of the game (latest entry whose
// CreatedAt ≤ FinishedAt). Games predating the earliest known
// version increment UntrackedGames but are not silently dropped.
//
// The output Versions list ALWAYS carries a row per lineage entry
// (even versions with zero games) — the frontend uses the zero-game
// rows to render "this version was published but never played" as a
// visible state instead of a missing-data hole.
//
// Algorithm: sort lineage by CreatedAt asc, games by FinishedAt asc,
// then sweep games once advancing a lineage cursor in lockstep. O(n
// log n) sort + O(n) walk for both slices.
func ComputeDeckVersionTrends(owner, deckID string, lineage []DeckVersionEntry, games []DeckVersionOutcome) DeckVersionTrends {
	out := DeckVersionTrends{
		Owner:    owner,
		DeckID:   deckID,
		Versions: []DeckVersionWinrate{},
	}

	sortedLin := make([]DeckVersionEntry, len(lineage))
	copy(sortedLin, lineage)
	sort.SliceStable(sortedLin, func(i, j int) bool {
		if sortedLin[i].CreatedAt != sortedLin[j].CreatedAt {
			return sortedLin[i].CreatedAt < sortedLin[j].CreatedAt
		}
		return sortedLin[i].Version < sortedLin[j].Version
	})

	sortedGames := make([]DeckVersionOutcome, len(games))
	copy(sortedGames, games)
	sort.SliceStable(sortedGames, func(i, j int) bool {
		return sortedGames[i].FinishedAt < sortedGames[j].FinishedAt
	})

	rows := make([]DeckVersionWinrate, len(sortedLin))
	for i, v := range sortedLin {
		rows[i] = DeckVersionWinrate{
			Hash:      v.Hash,
			Version:   v.Version,
			CardCount: v.CardCount,
			CardDelta: v.CardDelta,
			CreatedAt: v.CreatedAt,
		}
	}

	cursor := -1
	for _, g := range sortedGames {
		// Advance cursor to the latest version whose CreatedAt ≤
		// game.FinishedAt. Lookahead one slot so we don't race past
		// the correct bucket on ties.
		for cursor+1 < len(sortedLin) && sortedLin[cursor+1].CreatedAt <= g.FinishedAt {
			cursor++
		}
		if cursor < 0 {
			out.UntrackedGames++
			continue
		}
		rows[cursor].Games++
		switch {
		case g.Won:
			rows[cursor].Wins++
		case g.Draw:
			rows[cursor].Draws++
		default:
			rows[cursor].Losses++
		}
	}

	for i := range rows {
		if rows[i].Games > 0 {
			rows[i].WinRate = float64(rows[i].Wins) / float64(rows[i].Games)
		}
	}

	// Surface chronologically (Version asc) — same as natural lineage
	// order but explicit so frontend doesn't have to rely on stable
	// sort by CreatedAt for two versions with equal timestamps.
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Version < rows[j].Version
	})
	out.Versions = rows
	return out
}
