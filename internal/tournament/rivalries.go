package tournament

// Notable Rivalries — surface the most favorable / unfavorable matchups
// from the pairwise MatchupMatrix as a ranked, sample-size-filtered list.
//
// The MatchupMatrix on Result is a full N×N grid of head-to-head win
// counts; useful for drill-downs but hard to scan. ComputeRivalries
// distills it into a short list of "A over-performs vs B" / "A
// under-performs vs B" entries that exceed both a minimum-game gate
// (default 10) and a Wilson 95% CI significance check vs the
// commander's own baseline winrate.
//
// CR-equivalent: pure tournament-analytics post-processing. Reads
// Result, returns []Rivalry. No engine touches.

import (
	"math"
	"sort"
	"strings"
)

// Rivalry is a single significant pairwise matchup datum.
type Rivalry struct {
	Commander       string  // the "subject" commander
	Opponent        string  // the opposing commander
	Games           int     // co-occurring games between the two
	Wins            int     // games Commander won when both present
	Winrate         float64 // Wins / Games, in [0, 1]
	BaselineWinrate float64 // Commander's overall winrate, in [0, 1]
	Deviation       float64 // Winrate - BaselineWinrate
	WilsonLow       float64 // Wilson 95% CI lower bound
	WilsonHigh      float64 // Wilson 95% CI upper bound
	Significant     bool    // BaselineWinrate is outside [WilsonLow, WilsonHigh]
}

// RivalryOptions tunes ComputeRivalries.
type RivalryOptions struct {
	// MinGames is the minimum co-occurrence count for a pair to qualify.
	// A 60% winrate from 5 games is noise; from 30 it's signal. Default 10.
	MinGames int
	// MinAbsDeviation is the smallest |winrate - baseline| (in [0, 1])
	// that qualifies even when significant. Default 0.05 (5 percentage
	// points). Use 0 to include every significant deviation.
	MinAbsDeviation float64
	// RequireSignificance gates entries on the Wilson CI not bracketing
	// the baseline. Default true. Set false to surface large-effect
	// matchups even when the sample is small enough to keep the
	// baseline inside the CI.
	RequireSignificance bool
}

// DefaultRivalryOptions returns sane production defaults.
func DefaultRivalryOptions() RivalryOptions {
	return RivalryOptions{
		MinGames:            10,
		MinAbsDeviation:     0.05,
		RequireSignificance: true,
	}
}

// ComputeRivalries walks the MatchupMatrix and returns the set of
// pairwise matchups that pass MinGames, optional significance, and
// MinAbsDeviation. Results are sorted by |Deviation| descending so
// callers can take the top-N for a "most lopsided" listing.
//
// Baseline winrate is `WinsByCommander[A] / GamesPlayedByCommander[A]`
// when both maps populate the commander; otherwise falls back to
// `WinsByCommander[A] / r.Games` (the legacy denominator used when no
// per-commander game count is tracked, e.g. fixed-seat tournaments).
func ComputeRivalries(r *TournamentResult, opts RivalryOptions) []Rivalry {
	if r == nil || len(r.MatchupMatrix) == 0 {
		return nil
	}
	if opts.MinGames <= 0 {
		opts.MinGames = 1
	}
	out := make([]Rivalry, 0, 16)

	for _, cmdr := range r.CommanderNames {
		baseline := baselineWinrate(r, cmdr)
		oppMap := r.MatchupMatrix[cmdr]
		gamesMap := r.MatchupGames[cmdr]
		if oppMap == nil || gamesMap == nil {
			continue
		}
		// Iterate opponents in stable (sorted) order so test output and
		// markdown stay deterministic. The MatchupMatrix is keyed by
		// commander name, so a string sort is enough.
		opps := make([]string, 0, len(oppMap))
		for opp := range gamesMap {
			opps = append(opps, opp)
		}
		sort.Strings(opps)

		for _, opp := range opps {
			if opp == cmdr {
				continue
			}
			games := gamesMap[opp]
			if games < opts.MinGames {
				continue
			}
			wins := oppMap[opp]
			wr := float64(wins) / float64(games)
			lo, hi := wilsonInterval(wins, games)
			significant := baseline < lo || baseline > hi
			dev := wr - baseline
			if opts.RequireSignificance && !significant {
				continue
			}
			if math.Abs(dev) < opts.MinAbsDeviation {
				continue
			}
			out = append(out, Rivalry{
				Commander:       cmdr,
				Opponent:        opp,
				Games:           games,
				Wins:            wins,
				Winrate:         wr,
				BaselineWinrate: baseline,
				Deviation:       dev,
				WilsonLow:       lo,
				WilsonHigh:      hi,
				Significant:     significant,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := math.Abs(out[i].Deviation), math.Abs(out[j].Deviation)
		if ai != aj {
			return ai > aj
		}
		// Tie-break: more games first (more reliable), then names for
		// determinism.
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		if out[i].Commander != out[j].Commander {
			return out[i].Commander < out[j].Commander
		}
		return out[i].Opponent < out[j].Opponent
	})

	return out
}

// baselineWinrate returns commander's overall winrate in [0, 1].
func baselineWinrate(r *TournamentResult, cmdr string) float64 {
	wins := r.WinsByCommander[cmdr]
	played := 0
	if r.GamesPlayedByCommander != nil {
		played = r.GamesPlayedByCommander[cmdr]
	}
	if played == 0 {
		played = r.Games
	}
	if played == 0 {
		return 0
	}
	return float64(wins) / float64(played)
}

// wilsonInterval returns the Wilson 95% confidence interval (z=1.96)
// for the observed proportion wins/games. Returns [0, 0] when games==0.
//
// Wilson is preferred over the textbook normal-approximation interval
// because it doesn't degenerate to a zero-width or out-of-[0,1] range
// at small samples or extreme proportions — both situations are routine
// in tournament data (e.g. a 0/10 sweep produces a sensible upper
// bound around 0.28 rather than 0).
func wilsonInterval(wins, games int) (lo, hi float64) {
	if games <= 0 {
		return 0, 0
	}
	const z = 1.96 // 95% confidence
	n := float64(games)
	p := float64(wins) / n
	z2 := z * z
	denom := 1 + z2/n
	center := (p + z2/(2*n)) / denom
	margin := (z * math.Sqrt(p*(1-p)/n+z2/(4*n*n))) / denom
	lo = center - margin
	hi = center + margin
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}

// WriteRivalriesSection appends a "Notable Rivalries" markdown section
// to b describing the top-N most significant deviations per commander
// pair. Called from the report generator after the Matchup Matrix.
//
// `top` caps the number of rivalries listed (sorted by |deviation|
// descending). Pass 0 for "no cap" (list every qualifying rivalry).
func WriteRivalriesSection(b *strings.Builder, r *TournamentResult, opts RivalryOptions, top int) {
	rivalries := ComputeRivalries(r, opts)
	if len(rivalries) == 0 {
		return
	}
	if top > 0 && len(rivalries) > top {
		rivalries = rivalries[:top]
	}
	b.WriteString("\n## Notable Rivalries\n\n")
	b.WriteString("Pairwise matchups whose head-to-head winrate deviates significantly from the subject commander's baseline winrate (Wilson 95% CI, ≥")
	writeInt(b, opts.MinGames)
	b.WriteString(" co-occurring games).\n\n")
	b.WriteString("| Commander | vs Opponent | H2H | Baseline | Δ | Wilson CI |\n")
	b.WriteString("|---|---|---:|---:|---:|---:|\n")
	for _, riv := range rivalries {
		sign := "+"
		if riv.Deviation < 0 {
			sign = "−"
		}
		b.WriteString("| ")
		b.WriteString(riv.Commander)
		b.WriteString(" | ")
		b.WriteString(riv.Opponent)
		b.WriteString(" | ")
		writeInt(b, riv.Wins)
		b.WriteString("/")
		writeInt(b, riv.Games)
		b.WriteString(" (")
		writePct(b, riv.Winrate)
		b.WriteString(") | ")
		writePct(b, riv.BaselineWinrate)
		b.WriteString(" | ")
		b.WriteString(sign)
		writePct(b, math.Abs(riv.Deviation))
		b.WriteString(" | [")
		writePct(b, riv.WilsonLow)
		b.WriteString(", ")
		writePct(b, riv.WilsonHigh)
		b.WriteString("] |\n")
	}
}

// writeInt is a tiny strconv-free integer formatter used by
// WriteRivalriesSection. Keeps the report function dependency-light.
func writeInt(b *strings.Builder, n int) {
	if n == 0 {
		b.WriteByte('0')
		return
	}
	if n < 0 {
		b.WriteByte('-')
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	b.Write(buf[i:])
}

// writePct renders p ∈ [0,1] as "NN%" rounded to nearest integer percent.
func writePct(b *strings.Builder, p float64) {
	pct := int(math.Round(p * 100))
	writeInt(b, pct)
	b.WriteByte('%')
}
