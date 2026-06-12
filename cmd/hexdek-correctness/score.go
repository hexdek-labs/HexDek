package main

// score.go — the correctness-score data model: per-dimension pass rates
// aggregated into one top-line number, serialized as machine-readable
// JSON (CI-trackable over time) plus a human markdown summary.
//
// Scoring rules:
//
//   - Each dimension reports Checked (units examined), Passed (units
//     with zero non-info violations), and Pct = Passed/Checked*100.
//   - The unit differs per dimension and is carried explicitly:
//     legality counts ACTIONS, conservation and state-integrity count
//     GAMES, progression counts TRIGGERS, outcome counts EFFECTS.
//   - The top-line score is the unweighted mean of the dimension
//     percentages over dimensions that have data (Checked > 0); each
//     pillar answers a distinct correctness question, so none is more
//     "true" than another.
//   - Info-severity violations never fail a unit (they are triaged
//     non-bugs, e.g. the can't-lose-shield §704.5a downgrade).

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/judge"
)

const scoreSchemaVersion = 1

// DimensionScore is one pillar's pass rate.
type DimensionScore struct {
	Dimension string  `json:"dimension"`
	Unit      string  `json:"unit"`
	Checked   int     `json:"checked"`
	Passed    int     `json:"passed"`
	Pct       float64 `json:"pct"`
	// Detail carries dimension-specific sub-metrics (per-kind action
	// counts, per-event trigger counts, …) for dashboard drill-down.
	Detail map[string]interface{} `json:"detail,omitempty"`
}

// GamePassStats summarizes the game-level sweep that feeds the
// legality / conservation / state-integrity dimensions.
type GamePassStats struct {
	Games            int            `json:"games"`
	Seats            int            `json:"seats"`
	MaxTurns         int            `json:"max_turns"`
	Seed             int64          `json:"seed"`
	Crashes          int            `json:"crashes"`
	CrashedGames     int            `json:"crashed_games"`
	TotalTurns       int            `json:"total_turns"`
	ViolationsByName map[string]int `json:"violations_by_name,omitempty"`
	GamesAffected    map[string]int `json:"games_affected_by_dimension,omitempty"`
}

// Score is the full report.
type Score struct {
	SchemaVersion int               `json:"schema_version"`
	GeneratedAt   string            `json:"generated_at"`
	Config        map[string]string `json:"config"`
	ToplinePct    float64           `json:"topline_pct"`
	Dimensions    []DimensionScore  `json:"dimensions"`
	GamePass      *GamePassStats    `json:"game_pass,omitempty"`
	Notes         []string          `json:"notes,omitempty"`
}

func pct(passed, checked int) float64 {
	if checked == 0 {
		return 0
	}
	return float64(passed) / float64(checked) * 100
}

// topline computes the unweighted mean over dimensions with data.
func topline(dims []DimensionScore) float64 {
	sum, n := 0.0, 0
	for _, d := range dims {
		if d.Checked == 0 {
			continue
		}
		sum += d.Pct
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// dimensionOrder pins the report order to the five Judge pillars.
var dimensionOrder = []string{
	judge.DimensionLegality,
	judge.DimensionConservation,
	judge.DimensionStateIntegrity,
	judge.DimensionProgression,
	judge.DimensionOutcome,
}

// assembleScore builds the final Score from the pass results.
func assembleScore(generatedAt string, config map[string]string, dims []DimensionScore, gp *GamePassStats, notes []string) *Score {
	ordered := make([]DimensionScore, 0, len(dims))
	byName := map[string]DimensionScore{}
	for _, d := range dims {
		byName[d.Dimension] = d
	}
	for _, name := range dimensionOrder {
		if d, ok := byName[name]; ok {
			ordered = append(ordered, d)
			delete(byName, name)
		}
	}
	// Variant rows (e.g. the r63 "<dimension>_tuned" tuned-deck sweep)
	// follow the canonical five in stable order — dropping unknown
	// labels silently is how the tuned table vanished on first wiring.
	var rest []string
	for name := range byName {
		rest = append(rest, name)
	}
	sort.Strings(rest)
	for _, name := range rest {
		ordered = append(ordered, byName[name])
	}
	return &Score{
		SchemaVersion: scoreSchemaVersion,
		GeneratedAt:   generatedAt,
		Config:        config,
		ToplinePct:    topline(ordered),
		Dimensions:    ordered,
		GamePass:      gp,
		Notes:         notes,
	}
}

// renderJSON serializes the score (stable field order via struct tags).
func renderJSON(s *Score) ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

// renderMarkdown renders the human summary.
func renderMarkdown(s *Score) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# HexDek Correctness Score\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", s.GeneratedAt)
	fmt.Fprintf(&b, "## Top-line: %.2f%% correct\n\n", s.ToplinePct)
	b.WriteString("Unweighted mean of the dimension pass rates below (dimensions with no data excluded).\n\n")
	b.WriteString("| Dimension | Unit | Checked | Passed | Pass % |\n")
	b.WriteString("|-----------|------|---------|--------|--------|\n")
	for _, d := range s.Dimensions {
		pctStr := "n/a"
		if d.Checked > 0 {
			pctStr = fmt.Sprintf("%.2f%%", d.Pct)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %s |\n", d.Dimension, d.Unit, d.Checked, d.Passed, pctStr)
	}
	b.WriteString("\n")

	if gp := s.GamePass; gp != nil {
		fmt.Fprintf(&b, "## Game sweep\n\n")
		fmt.Fprintf(&b, "%d games (%d seats, max %d turns, seed %d), %d total turns. Crashes: %d (in %d games).\n\n",
			gp.Games, gp.Seats, gp.MaxTurns, gp.Seed, gp.TotalTurns, gp.Crashes, gp.CrashedGames)
		if len(gp.GamesAffected) > 0 {
			b.WriteString("Games with at least one violation, by dimension:\n\n")
			for _, name := range dimensionOrder {
				if n, ok := gp.GamesAffected[name]; ok && n > 0 {
					fmt.Fprintf(&b, "- %s: %d/%d games\n", name, n, gp.Games)
				}
			}
			b.WriteString("\n")
		}
		if len(gp.ViolationsByName) > 0 {
			b.WriteString("Top violations (raw stream counts — persistent violations re-report each turn):\n\n")
			type kv struct {
				name string
				n    int
			}
			var top []kv
			for k, v := range gp.ViolationsByName {
				top = append(top, kv{k, v})
			}
			sort.Slice(top, func(i, j int) bool {
				if top[i].n != top[j].n {
					return top[i].n > top[j].n
				}
				return top[i].name < top[j].name
			})
			for i, e := range top {
				if i >= 15 {
					fmt.Fprintf(&b, "- … %d more\n", len(top)-i)
					break
				}
				fmt.Fprintf(&b, "- %s: %d\n", e.name, e.n)
			}
			b.WriteString("\n")
		}
	}

	if len(s.Notes) > 0 {
		b.WriteString("## Notes\n\n")
		for _, n := range s.Notes {
			fmt.Fprintf(&b, "- %s\n", n)
		}
	}
	return b.String()
}
