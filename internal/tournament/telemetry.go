package tournament

import (
	"math"
	"sort"

	"github.com/hexdek/hexdek/internal/trueskill"
)

// TelemetrySchemaVersion identifies the Telemetry JSON shape (semver).
// Bumped on breaking changes per the same policy as SchemaVersion.
const TelemetrySchemaVersion = "1.0.0"

// SigmaConvergedRatio is the threshold above which a tournament's
// mean TrueSkill σ counts as "converged" — the per-deck posterior
// has tightened enough that subsequent games would mostly confirm
// rather than discover. Used by ConvergenceStats.Converged. The
// 0.5 threshold means the mean σ has dropped to ≤ 50% of the
// starting σ ≈ 8.33, i.e. to ~4.17 or below.
const SigmaConvergedRatio = 0.5

// Telemetry is the derived analytics layer over a TournamentResult.
// It's a snapshot computed at a single point in time (typically
// at end-of-tournament) and never mutated thereafter. The shape is
// pinned by docs/tournament-telemetry-schema.md.
//
// Used to answer the question "has this gauntlet converged on stable
// rankings, or do I need more games?". Consumers run ComputeTelemetry
// against the final TournamentResult, then read the convergence and
// ELO-distribution sections to decide.
type Telemetry struct {
	SchemaVersion string `json:"schema_version"`
	SourceMode    string `json:"source_mode"`
	Games         int    `json:"games"`
	Crashes       int    `json:"crashes"`

	// Commanders is the per-deck summary row, sorted by ELO desc.
	Commanders []CommanderTelemetry `json:"commanders"`

	// Convergence is the headline "is this tournament done?" signal.
	Convergence ConvergenceStats `json:"convergence"`

	// ELODistribution measures how spread out the final ratings are.
	// Tight spread = decks indistinguishable; wide spread = clear
	// ranking surface (the goal of a calibration gauntlet).
	ELODistribution ELODistributionStats `json:"elo_distribution"`

	// ArchetypeMatrix aggregates per-archetype winrate. Populated
	// only when ComputeTelemetry is called with a non-nil archetypes
	// slice (parallel to TournamentResult.CommanderNames).
	ArchetypeMatrix map[string]ArchetypeStats `json:"archetype_matrix,omitempty"`

	// Rounds carries per-round telemetry for round-structured modes
	// (Swiss / DE / Round-Robin / Balanced-Pool). Empty for rotate /
	// pool / lazy-pool modes that don't have a round concept. Each
	// entry corresponds to one round in source-mode order.
	Rounds []RoundTelemetry `json:"rounds,omitempty"`
}

// CommanderTelemetry is the per-deck summary row in Telemetry.
type CommanderTelemetry struct {
	Name      string  `json:"name"`
	Games     int     `json:"games"`
	Wins      int     `json:"wins"`
	WinRate   float64 `json:"win_rate"`
	ELO       float64 `json:"elo"`
	Mu        float64 `json:"trueskill_mu"`
	Sigma     float64 `json:"trueskill_sigma"`
	Archetype string  `json:"archetype,omitempty"`
}

// ConvergenceStats reports the tournament's posterior-tightening
// progress in terms of TrueSkill σ.
type ConvergenceStats struct {
	// InitialSigma is the per-deck σ at the start of any TrueSkill
	// session: 25/3 ≈ 8.333. Pinned here as the reference for the
	// convergence ratio.
	InitialSigma float64 `json:"initial_sigma"`

	// MeanSigma is the mean across every commander's final σ.
	MeanSigma float64 `json:"mean_sigma"`

	// MinSigma / MaxSigma surface the tightest- and loosest-known
	// decks — useful for spotting under-played decks at the bottom
	// of the rotation.
	MinSigma float64 `json:"min_sigma"`
	MaxSigma float64 `json:"max_sigma"`

	// ConvergenceRatio = 1 - (MeanSigma / InitialSigma). 0.0 = no
	// convergence (every deck still at the prior); 1.0 = fully
	// resolved. Negative shouldn't happen but is clamped at 0.
	ConvergenceRatio float64 `json:"convergence_ratio"`

	// Converged is true when ConvergenceRatio >= SigmaConvergedRatio
	// (0.5 default). Headline yes/no signal for "is this gauntlet
	// done?".
	Converged bool `json:"converged"`
}

// ELODistributionStats reports the spread of final ELO ratings.
type ELODistributionStats struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Spread float64 `json:"spread"` // Max - Min
}

// ArchetypeStats aggregates one archetype's row in ArchetypeMatrix.
type ArchetypeStats struct {
	Decks   int     `json:"decks"`
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	WinRate float64 `json:"win_rate"`
}

// RoundTelemetry is the per-round derived signal for round-
// structured modes. Captures the convergence + win-rate signal at
// the moment that round ended, so trajectories can be plotted by
// downstream tooling.
type RoundTelemetry struct {
	Round int `json:"round"`

	// SigmaMean is the per-round mean σ across all decks. Falls
	// over rounds in a converging tournament.
	SigmaMean float64 `json:"sigma_mean"`

	// MuSpread is max(μ) - min(μ) at end-of-round. Grows as decks
	// differentiate.
	MuSpread float64 `json:"mu_spread"`

	// WinRates is per-deck cumulative win rate at end-of-round.
	// Used to render the round-by-round heatmap downstream.
	WinRates map[string]float64 `json:"win_rates,omitempty"`

	// OMW is per-deck Opponent Match-Win % at end-of-round (Swiss-
	// only). Lets the OMW trajectory be plotted across rounds.
	OMW map[string]float64 `json:"omw,omitempty"`
}

// RoundSnapshot is the raw per-round capture emitted by round-
// structured runners. Exists on TournamentResult.RoundSnapshots so
// ComputeTelemetry can derive RoundTelemetry from it without
// requiring callers to re-instrument the runner.
type RoundSnapshot struct {
	Round int `json:"round"`

	// Ratings is per-commander (name → snapshot). Captured at end-
	// of-round AFTER all that round's pods have updated their
	// TrueSkill state.
	Ratings map[string]RatingSnapshot `json:"ratings"`

	// CumulativeWins / CumulativeGames is the per-deck cumulative
	// at end-of-round.
	CumulativeWins  map[string]int `json:"cumulative_wins"`
	CumulativeGames map[string]int `json:"cumulative_games"`

	// OMW is per-deck OMW% at end-of-round (Swiss-only — other modes
	// leave this nil).
	OMW map[string]float64 `json:"omw,omitempty"`
}

// RatingSnapshot captures a deck's mu/sigma/ELO at a single
// point in time.
type RatingSnapshot struct {
	ELO   float64 `json:"elo"`
	Mu    float64 `json:"trueskill_mu"`
	Sigma float64 `json:"trueskill_sigma"`
}

// ComputeTelemetry derives the analytics layer from a finished
// TournamentResult. `archetypes` is OPTIONAL — pass nil to skip
// the ArchetypeMatrix section, or pass a slice parallel to
// r.CommanderNames (DeckArchetypes from the original config) to
// surface the per-archetype winrate breakdown.
//
// Returns nil for a nil input. Tolerates partial data (missing
// ELO, missing TrueSkill, no RoundSnapshots) — populates what it
// can and leaves the gaps as zero-valued or omitempty-elided.
func ComputeTelemetry(r *TournamentResult, archetypes []Archetype) *Telemetry {
	if r == nil {
		return nil
	}
	t := &Telemetry{
		SchemaVersion: TelemetrySchemaVersion,
		SourceMode:    r.Mode,
		Games:         r.Games,
		Crashes:       r.Crashes,
	}

	// Build per-commander rows. Index ELO/TrueSkill by name so we
	// can join on the canonical CommanderNames order.
	eloByName := make(map[string]ELOEntry, len(r.ELO))
	for _, e := range r.ELO {
		eloByName[e.Commander] = e
	}
	tsByName := make(map[string]trueskill.TrueSkillEntry, len(r.TrueSkill))
	for _, e := range r.TrueSkill {
		tsByName[e.Commander] = e
	}

	t.Commanders = make([]CommanderTelemetry, 0, len(r.CommanderNames))
	for i, name := range r.CommanderNames {
		row := CommanderTelemetry{
			Name:  name,
			Wins:  r.WinsByCommander[name],
			Games: r.GamesPlayedByCommander[name],
		}
		if row.Games > 0 {
			row.WinRate = float64(row.Wins) / float64(row.Games)
		}
		if e, ok := eloByName[name]; ok {
			row.ELO = e.Rating
		}
		if e, ok := tsByName[name]; ok {
			row.Mu = e.Mu
			row.Sigma = e.Sigma
		}
		if i < len(archetypes) && archetypes[i] != ArchetypeUnknown {
			row.Archetype = string(archetypes[i])
		}
		t.Commanders = append(t.Commanders, row)
	}
	// Sort commanders by ELO desc (then by name asc for determinism).
	sort.SliceStable(t.Commanders, func(i, j int) bool {
		if t.Commanders[i].ELO != t.Commanders[j].ELO {
			return t.Commanders[i].ELO > t.Commanders[j].ELO
		}
		return t.Commanders[i].Name < t.Commanders[j].Name
	})

	// Convergence layer.
	t.Convergence = computeConvergence(t.Commanders)

	// ELO distribution.
	t.ELODistribution = computeELODistribution(t.Commanders)

	// Archetype matrix (only when archetypes supplied).
	if len(archetypes) > 0 {
		t.ArchetypeMatrix = computeArchetypeMatrix(r.CommanderNames, archetypes, r.WinsByCommander, r.GamesPlayedByCommander)
	}

	// Per-round telemetry from RoundSnapshots if present.
	if len(r.RoundSnapshots) > 0 {
		t.Rounds = make([]RoundTelemetry, 0, len(r.RoundSnapshots))
		for _, snap := range r.RoundSnapshots {
			t.Rounds = append(t.Rounds, deriveRoundTelemetry(snap))
		}
	}

	return t
}

func computeConvergence(commanders []CommanderTelemetry) ConvergenceStats {
	initial := 25.0 / 3.0
	cs := ConvergenceStats{InitialSigma: initial}
	if len(commanders) == 0 {
		return cs
	}
	sum := 0.0
	cs.MinSigma = math.Inf(1)
	cs.MaxSigma = math.Inf(-1)
	count := 0
	for _, c := range commanders {
		if c.Sigma == 0 {
			continue // no TrueSkill data
		}
		sum += c.Sigma
		if c.Sigma < cs.MinSigma {
			cs.MinSigma = c.Sigma
		}
		if c.Sigma > cs.MaxSigma {
			cs.MaxSigma = c.Sigma
		}
		count++
	}
	if count == 0 {
		return cs
	}
	cs.MeanSigma = sum / float64(count)
	cs.ConvergenceRatio = 1.0 - (cs.MeanSigma / initial)
	if cs.ConvergenceRatio < 0 {
		cs.ConvergenceRatio = 0
	}
	cs.Converged = cs.ConvergenceRatio >= SigmaConvergedRatio
	return cs
}

func computeELODistribution(commanders []CommanderTelemetry) ELODistributionStats {
	if len(commanders) == 0 {
		return ELODistributionStats{}
	}
	stats := ELODistributionStats{
		Min: math.Inf(1),
		Max: math.Inf(-1),
	}
	sum := 0.0
	count := 0
	for _, c := range commanders {
		if c.ELO == 0 {
			continue
		}
		if c.ELO < stats.Min {
			stats.Min = c.ELO
		}
		if c.ELO > stats.Max {
			stats.Max = c.ELO
		}
		sum += c.ELO
		count++
	}
	if count == 0 {
		return ELODistributionStats{}
	}
	stats.Mean = sum / float64(count)
	// Variance pass.
	v := 0.0
	for _, c := range commanders {
		if c.ELO == 0 {
			continue
		}
		d := c.ELO - stats.Mean
		v += d * d
	}
	stats.StdDev = math.Sqrt(v / float64(count))
	stats.Spread = stats.Max - stats.Min
	return stats
}

func computeArchetypeMatrix(names []string, archetypes []Archetype, wins map[string]int, games map[string]int) map[string]ArchetypeStats {
	out := make(map[string]ArchetypeStats)
	for i, name := range names {
		if i >= len(archetypes) {
			break
		}
		arch := archetypes[i]
		if arch == ArchetypeUnknown {
			continue
		}
		row := out[string(arch)]
		row.Decks++
		row.Games += games[name]
		row.Wins += wins[name]
		out[string(arch)] = row
	}
	for k, v := range out {
		if v.Games > 0 {
			v.WinRate = float64(v.Wins) / float64(v.Games)
		}
		out[k] = v
	}
	return out
}

func deriveRoundTelemetry(snap RoundSnapshot) RoundTelemetry {
	rt := RoundTelemetry{Round: snap.Round}
	if len(snap.Ratings) == 0 {
		return rt
	}
	sigmaSum := 0.0
	muMin := math.Inf(1)
	muMax := math.Inf(-1)
	for _, r := range snap.Ratings {
		sigmaSum += r.Sigma
		if r.Mu < muMin {
			muMin = r.Mu
		}
		if r.Mu > muMax {
			muMax = r.Mu
		}
	}
	rt.SigmaMean = sigmaSum / float64(len(snap.Ratings))
	rt.MuSpread = muMax - muMin
	// Per-deck cumulative winrate.
	if len(snap.CumulativeGames) > 0 {
		rt.WinRates = make(map[string]float64, len(snap.CumulativeGames))
		for name, games := range snap.CumulativeGames {
			if games > 0 {
				rt.WinRates[name] = float64(snap.CumulativeWins[name]) / float64(games)
			}
		}
	}
	if len(snap.OMW) > 0 {
		rt.OMW = make(map[string]float64, len(snap.OMW))
		for k, v := range snap.OMW {
			rt.OMW[k] = v
		}
	}
	return rt
}
