package heimdall

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// DecisionClass categorizes a hat decision into one of the strategic
// classes useful for cross-game attribution. Broader than the per-event
// Kind on HatDecision — multiple Kinds may map to the same class, and
// some Kinds split into multiple classes depending on Details.
type DecisionClass string

const (
	DCCastVsHold        DecisionClass = "cast_vs_hold"          // chose to cast vs hold mana
	DCAttackVsHold      DecisionClass = "attack_vs_hold"        // chose to attack vs pass
	DCTargetChoice      DecisionClass = "target_choice"         // picked target X over Y
	DCManaVsInteraction DecisionClass = "mana_vs_interaction"   // tapped out vs held up interaction
	DCMulliganKeep      DecisionClass = "mulligan_keep_vs_mull" // keep vs mulligan
	DCBlockChoice       DecisionClass = "block_vs_no_block"     // chose to block vs let through
	DCResponseCounter   DecisionClass = "response_counter"      // counterspell yes/no
	DCModeChoice        DecisionClass = "mode_choice"           // modal-spell choice
	DCUnclassified      DecisionClass = "unclassified"          // fallback
)

// MinSamplesForSuggestion is the minimum DecisionCount required for
// BuildAttributionTable to emit a Suggestion string. Below this, the
// cell is too thin to be statistically meaningful.
const MinSamplesForSuggestion = 10

// AttributedDecision pairs a HatDecision with its class + game outcome.
type AttributedDecision struct {
	HatDecision                // embedded
	Class       DecisionClass  `json:"class"`
	Won         bool           `json:"won"`                   // did the decision-maker (Seat) win the game?
	ScoreDelta  float64        `json:"score_delta,omitempty"` // hat's reported margin between chosen and runner-up, when available in Details
}

// HeimdallDecisionAttribution is per-game attributed decisions plus the
// per-seat archetype-belief snapshot.
type HeimdallDecisionAttribution struct {
	GameID         string               `json:"game_id"`
	WinnerSeat     int                  `json:"winner_seat"`
	SeatArchetypes []string             `json:"seat_archetypes"` // index = seat; empty string = unknown
	Decisions      []AttributedDecision `json:"decisions"`
}

// ArchetypeClassMetrics is one (archetype, class) cell of the aggregated table.
type ArchetypeClassMetrics struct {
	Archetype        string        `json:"archetype"`
	Class            DecisionClass `json:"class"`
	DecisionCount    int           `json:"decision_count"`
	WonDecisionCount int           `json:"won_decision_count"`
	WinRate          float64       `json:"win_rate"`        // wonDecisions / decisions
	AvgScoreDelta    float64       `json:"avg_score_delta"` // mean ScoreDelta across all decisions in the cell
	AvgMisweight     float64       `json:"avg_misweight"`   // pluggable estimator output
	// Suggestion is a human-readable rephrasing of the cell metrics.
	// Empty when the cell is too thin (DecisionCount < MinSamplesForSuggestion).
	Suggestion string `json:"suggestion,omitempty"`
}

// GauntletAttributionTable is the cross-game rollup that dev-3 consumes.
type GauntletAttributionTable struct {
	GamesProcessed int                     `json:"games_processed"`
	TotalDecisions int                     `json:"total_decisions"`
	Metrics        []ArchetypeClassMetrics `json:"metrics"`      // sorted by (Archetype, Class) for stable output
	GeneratedAt    string                  `json:"generated_at"` // RFC3339
}

// MisweightEstimator is the pluggable scorer for "how far was this
// decision from ground-truth optimal?" Future workers (dev-3 with an
// oracle, dev-19 with a pre-game predictor) supply their own. The
// DefaultMisweightEstimator returns 0 for won decisions and
// abs(ScoreDelta) for lost decisions — an outcome-anchored proxy with
// no oracle.
type MisweightEstimator func(d AttributedDecision) float64

// FeedbackInterface is the contract that dev-3's recommender plugs
// into. Heimdall pushes attribution tables; dev-3 consumes and
// (optionally) returns hat-tuning suggestions which Heimdall logs.
// The return-value contract is "error iff the consumer failed to
// accept the table"; Heimdall does not inspect the consumer for
// suggestions on this surface (a future SuggestTuning method will
// be added when the recommender ships).
type FeedbackInterface interface {
	AcceptAttributionTable(t GauntletAttributionTable) error
}

// DefaultMisweightEstimator returns 0 for won decisions and
// math.Abs(d.ScoreDelta) for lost decisions — a simple outcome-anchored
// proxy with no oracle. Used when callers don't supply their own.
func DefaultMisweightEstimator(d AttributedDecision) float64 {
	if d.Won {
		return 0
	}
	return math.Abs(d.ScoreDelta)
}

// coerceFloat extracts a float64 from an interface{} value that may
// have arrived as float64 (JSON-unmarshalled) or int (in-process test
// stamping). Returns (0, false) for nil or unrecognized types.
func coerceFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	}
	return 0, false
}

// ClassifyDecision maps a HatDecision (Kind + Details) to its broader
// strategic DecisionClass. The mapping is deterministic and table-
// driven; when the Kind is recognized but the Details disambiguator is
// missing, returns the most-likely class for the Kind (e.g. "cast" →
// DCCastVsHold).
//
// Note: DCManaVsInteraction is only triggered when the cast event's
// Details map carries alternative="hold_interaction". The hat's
// emitDecisionEvent does not currently populate that key — this is a
// known limitation; the classifier surface is forward-compatible so
// that once the hat side stamps the alternative, the class will start
// flowing through without further changes here.
func ClassifyDecision(d HatDecision) DecisionClass {
	switch d.Kind {
	case "mulligan":
		return DCMulliganKeep
	case "cast":
		if alt, ok := d.Details["alternative"].(string); ok {
			if alt == "hold_interaction" {
				return DCManaVsInteraction
			}
		}
		return DCCastVsHold
	case "attack_target":
		if chose, ok := d.Details["chose"].(string); ok {
			if chose == "pass" || chose == "no_attack" {
				return DCAttackVsHold
			}
		}
		return DCTargetChoice
	case "block":
		return DCBlockChoice
	case "response_counter":
		return DCResponseCounter
	case "mode":
		return DCModeChoice
	}
	return DCUnclassified
}

// AttributeDecisions extracts all hat decisions from gs and pairs each
// with the game outcome. winnerSeat is -1 for draws. perSeatArchetype
// supplies the post-game archetype labels (one per seat; empty string
// when unknown) — usually Heimdall's classifier output. gameID is the
// caller's preferred game identifier (typically from
// GameSummary.GameID).
//
// Returns nil if gs is nil or has no hat decisions. When gs has no hat
// events, Decisions is an empty slice (not nil) on the returned
// attribution — see test TestAttributeDecisions_NoHatEvents.
func AttributeDecisions(gs *gameengine.GameState, gameID string, winnerSeat int, perSeatArchetype []string) *HeimdallDecisionAttribution {
	if gs == nil {
		return nil
	}
	raw := ExtractHatDecisions(gs)
	out := &HeimdallDecisionAttribution{
		GameID:         gameID,
		WinnerSeat:     winnerSeat,
		SeatArchetypes: append([]string(nil), perSeatArchetype...),
		Decisions:      make([]AttributedDecision, 0, len(raw)),
	}
	for _, hd := range raw {
		ad := AttributedDecision{
			HatDecision: hd,
			Class:       ClassifyDecision(hd),
			Won:         winnerSeat >= 0 && hd.Seat == winnerSeat,
		}
		// ScoreDelta may live under "score_delta" or "delta" in
		// Details; tolerate either. Both float64 (JSON) and int
		// (test stamping) are accepted via coerceFloat.
		if v, ok := hd.Details["score_delta"]; ok {
			if f, ok := coerceFloat(v); ok {
				ad.ScoreDelta = f
			}
		} else if v, ok := hd.Details["delta"]; ok {
			if f, ok := coerceFloat(v); ok {
				ad.ScoreDelta = f
			}
		}
		out.Decisions = append(out.Decisions, ad)
	}
	return out
}

// BuildAttributionTable aggregates a slice of per-game attributions
// into the cross-(archetype, class) metrics table. If est is nil, uses
// DefaultMisweightEstimator. Cells with DecisionCount <
// MinSamplesForSuggestion get an empty Suggestion field — too thin to
// advise on.
//
// Output Metrics is sorted by (Archetype asc, Class asc as string) for
// stable downstream consumption.
//
// Decisions whose attributed archetype string is empty are bucketed
// under archetype="unknown" so the table still surfaces them as a
// class-only rollup rather than dropping the row.
func BuildAttributionTable(games []HeimdallDecisionAttribution, est MisweightEstimator) GauntletAttributionTable {
	if est == nil {
		est = DefaultMisweightEstimator
	}
	type cellKey struct {
		arch  string
		class DecisionClass
	}
	type cellAccum struct {
		count    int
		won      int
		sumDelta float64
		sumMis   float64
	}
	cells := make(map[cellKey]*cellAccum)
	totalDecisions := 0
	for _, g := range games {
		for _, d := range g.Decisions {
			arch := d.Archetype
			if arch == "" {
				// Fall back to the per-seat archetype label if the
				// decision-event didn't carry one.
				if d.Seat >= 0 && d.Seat < len(g.SeatArchetypes) {
					arch = g.SeatArchetypes[d.Seat]
				}
			}
			if arch == "" {
				arch = "unknown"
			}
			key := cellKey{arch: arch, class: d.Class}
			c := cells[key]
			if c == nil {
				c = &cellAccum{}
				cells[key] = c
			}
			c.count++
			if d.Won {
				c.won++
			}
			c.sumDelta += d.ScoreDelta
			c.sumMis += est(d)
			totalDecisions++
		}
	}
	metrics := make([]ArchetypeClassMetrics, 0, len(cells))
	for k, c := range cells {
		m := ArchetypeClassMetrics{
			Archetype:        k.arch,
			Class:            k.class,
			DecisionCount:    c.count,
			WonDecisionCount: c.won,
		}
		if c.count > 0 {
			m.WinRate = float64(c.won) / float64(c.count)
			m.AvgScoreDelta = c.sumDelta / float64(c.count)
			m.AvgMisweight = c.sumMis / float64(c.count)
		}
		if c.count >= MinSamplesForSuggestion {
			m.Suggestion = renderSuggestion(m)
		}
		metrics = append(metrics, m)
	}
	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].Archetype != metrics[j].Archetype {
			return metrics[i].Archetype < metrics[j].Archetype
		}
		return string(metrics[i].Class) < string(metrics[j].Class)
	})
	return GauntletAttributionTable{
		GamesProcessed: len(games),
		TotalDecisions: totalDecisions,
		Metrics:        metrics,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
}

// renderSuggestion produces a human-readable advisory string for a
// single cell. Buckets:
//
//   - AvgMisweight > 0.2: "misweighted by +X.XX"
//   - AvgMisweight < 0.05 AND WinRate >= 0.5: "healthy win rate"
//   - in between: "minor weight drift"
//
// Caller guards on DecisionCount >= MinSamplesForSuggestion.
func renderSuggestion(m ArchetypeClassMetrics) string {
	winPct := m.WinRate * 100.0
	switch {
	case m.AvgMisweight > 0.2:
		return fmt.Sprintf("in archetype %s, decision class %s was misweighted by avg +%.2f vs ground-truth optimal (%d decisions, %.1f%% win rate)",
			m.Archetype, m.Class, m.AvgMisweight, m.DecisionCount, winPct)
	case m.AvgMisweight < 0.05 && m.WinRate >= 0.5:
		return fmt.Sprintf("in archetype %s, decision class %s shows healthy win rate (%.1f%% over %d decisions); no tuning suggested",
			m.Archetype, m.Class, winPct, m.DecisionCount)
	default:
		return fmt.Sprintf("in archetype %s, decision class %s shows minor weight drift (+%.2f, %.1f%% win rate over %d decisions)",
			m.Archetype, m.Class, m.AvgMisweight, winPct, m.DecisionCount)
	}
}

// attributionTablePath returns the on-disk path under dir where the
// attribution table is persisted. Lives under data/heimdall/ to match
// the existing per-game artifact directory convention; no new top-
// level data directory is introduced.
func attributionTablePath(dir string) string {
	return filepath.Join(dir, "heimdall", "attribution_table.json")
}

// WriteGauntletAttributionTable persists the table to
// data/heimdall/attribution_table.json under dir. Creates parent
// directories as needed. Returns a non-nil error on any I/O or
// marshal failure.
func WriteGauntletAttributionTable(dir string, table GauntletAttributionTable) error {
	path := attributionTablePath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(table, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal attribution table: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// ReadGauntletAttributionTable loads a previously-persisted attribution
// table from data/heimdall/attribution_table.json under dir. Tolerates
// missing/empty files by returning a zero-value table with no error so
// callers can treat first-run absence as the same shape as empty data.
func ReadGauntletAttributionTable(dir string) (GauntletAttributionTable, error) {
	path := attributionTablePath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GauntletAttributionTable{}, nil
		}
		return GauntletAttributionTable{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return GauntletAttributionTable{}, nil
	}
	var table GauntletAttributionTable
	if err := json.Unmarshal(data, &table); err != nil {
		return GauntletAttributionTable{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return table, nil
}
