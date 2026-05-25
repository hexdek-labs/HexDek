package heimdall

import (
	"math"
	"sort"
)

// TournamentStats is the response shape returned by the
// /api/tournament/{id}/stats endpoint. Aggregates one gauntlet run's
// games into the four signals the tournament dashboard surfaces:
//
//   - DeckDistribution: how many games were played against each
//     opposing commander (proxy for "what decks did this tournament
//     see").
//   - WinrateVariance: how spread the deck's per-opponent winrate
//     is. A polarized matchup profile (80% vs aggro, 20% vs combo)
//     scores high variance; a consistent profile (50% vs everyone)
//     scores low.
//   - LongestGame / FastestGame: turn-count extremes for the
//     tournament — outlier signals that complement avg_turns.
//
// AvgTurns mirrors gauntlet_runs.avg_turns; included for parity so
// the frontend can render the full stats panel from one fetch.
type TournamentStats struct {
	TournamentID     int64                  `json:"tournament_id"`
	DeckKey          string                 `json:"deck_key"`
	Commander        string                 `json:"commander"`
	StartedAt        int64                  `json:"started_at"`
	FinishedAt       int64                  `json:"finished_at"`
	Games            int                    `json:"games"`
	Wins             int                    `json:"wins"`
	Losses           int                    `json:"losses"`
	Draws            int                    `json:"draws"`
	WinRate          float64                `json:"win_rate"`
	AvgTurns         float64                `json:"avg_turns"`
	DeckDistribution []DeckShare            `json:"deck_distribution"`
	WinrateVariance  WinrateVariance        `json:"winrate_variance"`
	LongestGame      *TournamentGameExtreme `json:"longest_game,omitempty"`
	FastestGame      *TournamentGameExtreme `json:"fastest_game,omitempty"`
}

// DeckShare is one opposing-commander slice of the tournament's
// matchup pie. Share is games/total as a 0..1 fraction. Sorted by
// games desc.
type DeckShare struct {
	Commander string  `json:"commander"`
	Games     int     `json:"games"`
	Share     float64 `json:"share"`
}

// WinrateVariance summarizes how spread the deck's per-opponent
// winrate is across the tournament. Only opponents with at least
// tournamentVarianceMinPerOpponent games are counted — a single-game
// 100% vs FringeCommander would otherwise dominate the variance
// signal.
//
// Variance + Stddev are the population variance / stddev of the
// per-opponent winrate values (each opponent contributes one
// winrate point, weighted equally). MinWinRate / MaxWinRate are
// the floor and ceiling of those points (useful for "matchup range"
// label). SampleSize is the number of opponents that cleared the
// minimum sample bar.
type WinrateVariance struct {
	Variance    float64 `json:"variance"`
	Stddev      float64 `json:"stddev"`
	MinWinRate  float64 `json:"min_win_rate"`
	MaxWinRate  float64 `json:"max_win_rate"`
	SampleSize  int     `json:"sample_size"`
}

// TournamentGameExtreme is one outlier game (longest or fastest).
// Carries the per-game metadata so the frontend can deep-link to
// the game's report page without an extra fetch.
type TournamentGameExtreme struct {
	GameID     int64 `json:"game_id"`
	Turns      int   `json:"turns"`
	Won        bool  `json:"won"`
	FinishedAt int64 `json:"finished_at"`
}

// TournamentGameInput is one game's contribution to the tournament
// rollup. The pure-computation function is decoupled from internal/db
// so heimdall stays a slice transformer.
type TournamentGameInput struct {
	GameID     int64
	FinishedAt int64
	Turns      int
	Won        bool
	Draw       bool
	Opponents  []string
}

const (
	// tournamentVarianceMinPerOpponent — the minimum games against an
	// opposing commander before that opponent's winrate is included
	// in the variance signal. 2 keeps single-game outliers from
	// dominating; gauntlets typically queue dozens of opponents so
	// most opponents hit this floor naturally.
	tournamentVarianceMinPerOpponent = 2
)

// ComputeTournamentStats turns a gauntlet run + its per-game rows
// into a TournamentStats summary. Hard-coded none of the surface
// shape is computed from the gauntlet_runs aggregate columns — that
// row is the authoritative source for the run-level scalar fields
// (Games / Wins / Losses / WinRate / AvgTurns), so we echo those
// directly rather than recomputing from games (which can drift if
// a game persistence partial-failure left an orphan row).
//
// Games-derived signals (DeckDistribution, WinrateVariance,
// LongestGame, FastestGame) are recomputed from `games` since they
// have no scalar representation on gauntlet_runs.
func ComputeTournamentStats(run TournamentRunMeta, games []TournamentGameInput) TournamentStats {
	out := TournamentStats{
		TournamentID:     run.RunID,
		DeckKey:          run.DeckKey,
		Commander:        run.Commander,
		StartedAt:        run.StartedAt,
		FinishedAt:       run.FinishedAt,
		Games:            run.Games,
		Wins:             run.Wins,
		Losses:           run.Losses,
		WinRate:          run.WinRate,
		AvgTurns:         run.AvgTurns,
		DeckDistribution: []DeckShare{},
	}
	if len(games) == 0 {
		return out
	}

	// Per-opponent rollup for both DeckDistribution and
	// WinrateVariance — single pass.
	type oppAgg struct {
		games int
		wins  int
	}
	opp := make(map[string]*oppAgg)
	totalOppEntries := 0
	draws := 0

	var longest, fastest *TournamentGameInput
	for i := range games {
		g := games[i]
		if g.Draw {
			draws++
		}
		for _, cmdr := range g.Opponents {
			if cmdr == "" {
				continue
			}
			a, ok := opp[cmdr]
			if !ok {
				a = &oppAgg{}
				opp[cmdr] = a
			}
			a.games++
			if g.Won {
				a.wins++
			}
			totalOppEntries++
		}
		if longest == nil || g.Turns > longest.Turns {
			longest = &games[i]
		}
		if fastest == nil || g.Turns < fastest.Turns {
			fastest = &games[i]
		}
	}
	out.Draws = draws

	// DeckDistribution sorted by games desc, commander asc for ties.
	dist := make([]DeckShare, 0, len(opp))
	for cmdr, a := range opp {
		share := 0.0
		if totalOppEntries > 0 {
			share = float64(a.games) / float64(totalOppEntries)
		}
		dist = append(dist, DeckShare{
			Commander: cmdr,
			Games:     a.games,
			Share:     share,
		})
	}
	sort.SliceStable(dist, func(i, j int) bool {
		if dist[i].Games != dist[j].Games {
			return dist[i].Games > dist[j].Games
		}
		return dist[i].Commander < dist[j].Commander
	})
	out.DeckDistribution = dist

	// WinrateVariance: take each opponent with ≥ min games,
	// compute its winrate, then population variance/stddev of those.
	rates := make([]float64, 0, len(opp))
	for _, a := range opp {
		if a.games < tournamentVarianceMinPerOpponent {
			continue
		}
		rates = append(rates, float64(a.wins)/float64(a.games))
	}
	if len(rates) > 0 {
		var sum float64
		for _, r := range rates {
			sum += r
		}
		mean := sum / float64(len(rates))
		var sq float64
		min, max := rates[0], rates[0]
		for _, r := range rates {
			diff := r - mean
			sq += diff * diff
			if r < min {
				min = r
			}
			if r > max {
				max = r
			}
		}
		variance := sq / float64(len(rates))
		out.WinrateVariance = WinrateVariance{
			Variance:   variance,
			Stddev:     math.Sqrt(variance),
			MinWinRate: min,
			MaxWinRate: max,
			SampleSize: len(rates),
		}
	}

	if longest != nil {
		out.LongestGame = &TournamentGameExtreme{
			GameID:     longest.GameID,
			Turns:      longest.Turns,
			Won:        longest.Won,
			FinishedAt: longest.FinishedAt,
		}
	}
	if fastest != nil {
		out.FastestGame = &TournamentGameExtreme{
			GameID:     fastest.GameID,
			Turns:      fastest.Turns,
			Won:        fastest.Won,
			FinishedAt: fastest.FinishedAt,
		}
	}

	return out
}

// TournamentRunMeta is the thin shape ComputeTournamentStats
// consumes for the run-level fields. hexapi callers translate
// db.GauntletRunRecord into this struct; tests can hand-construct
// it without a DB dependency.
type TournamentRunMeta struct {
	RunID      int64
	DeckKey    string
	Commander  string
	StartedAt  int64
	FinishedAt int64
	Games      int
	Wins       int
	Losses     int
	WinRate    float64
	AvgTurns   float64
}
