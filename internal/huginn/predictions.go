package huginn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hexdek/hexdek/internal/analytics"
)

// Worker D — Huginn 2.0 prediction + feedback loop (PR shipped in the
// freya-hat integration sequence after waves 1-5).
//
// Sequence:
//
//  1. dev-19's `--predict` CLI generates Prediction batches from
//     Huginn's tier-3 confirmed patterns + the target deck. Each
//     prediction is a per-deck speculative combo with a unique
//     InstanceID so post-game feedback can route confidence updates
//     back to the right pattern record.
//
//  2. Freya ships the predictions on strategy.json as the
//     HuginnPredictions field (cmd/hexdek-freya/main.go) so the hat
//     loads them with the rest of the deck profile.
//
//  3. The hat consumes predictions in cardHeuristic via
//     huginnPredictionBoost: cards appearing in a prediction get a
//     scaled bonus (0.10 * Confidence) so the hat biases toward
//     drawing/casting the predicted combo pieces while the prediction
//     is fresh.
//
//  4. After the game, ComputePredictionOutcomes walks the per-seat
//     CardsPlayed in the GameAnalysis and reports per-prediction
//     fire/no-fire + a small confidence-delta signal.
//
//  5. RecordPredictionOutcomes persists outcomes to
//     prediction_outcomes.jsonl in the huginn dir; the next --predict
//     run folds them in as confidence adjustments before emitting the
//     next batch.

// Prediction is a Huginn-generated speculative combo prediction for a
// specific deck. Emitted by dev-19's --predict CLI; consumed by the
// hat through StrategyProfile.HuginnPredictions and by Heimdall via
// ComputePredictionOutcomes for post-game feedback.
type Prediction struct {
	// InstanceID is a per-prediction unique identifier so post-game
	// feedback can match prediction → outcome → confidence
	// adjustment. Stable across the prediction lifecycle (emit →
	// hat consumption → game completion → outcome record).
	// Convention: "<deck-fingerprint>-<pattern-key>-<seq>".
	InstanceID string `json:"instance_id"`

	// Cards is the predicted combo's required pieces. All must be
	// in the deck (asserted by --predict at generation time); all
	// must be cast in the same game to count as "fired" in
	// ComputePredictionOutcomes.
	Cards []string `json:"cards"`

	// Pattern is the Huginn pattern key the prediction was derived
	// from (e.g. "etb-draw-loop", "sac-then-recur"). Free-form
	// string from the Huginn pattern catalog; downstream consumers
	// don't parse it.
	Pattern string `json:"pattern,omitempty"`

	// Confidence is Huginn's prior confidence in the prediction,
	// in [0, 1]. Adjusted across runs by the feedback loop:
	// PredictionOutcome.ConfidenceDelta is applied to the next
	// round's Confidence value.
	Confidence float64 `json:"confidence"`
}

// PredictionOutcome is the post-game record for a single Prediction.
// Records whether all predicted pieces reached the battlefield (Fired)
// and a small confidence delta that Huginn's next prediction round
// folds in (positive when fired, negative when not — symmetric so a
// 50/50 history converges to neutral, small enough to avoid single-
// game overcorrection).
type PredictionOutcome struct {
	InstanceID      string  `json:"instance_id"`
	Fired           bool    `json:"fired"`
	ConfidenceDelta float64 `json:"confidence_delta"`
	GameID          string  `json:"game_id,omitempty"`
}

// outcomeFiredDelta is the per-game confidence adjustment when a
// prediction fires. Kept small (5%) so single-game noise can't push
// the prior to extremes; the learning rate compounds across many
// games.
const outcomeFiredDelta = 0.05

// outcomeMissDelta is the per-game adjustment when a prediction
// doesn't fire. Smaller magnitude than the fired-delta because
// "didn't fire" can mean "didn't see the win line" rather than
// "the prediction was wrong" — being conservative on the punish
// avoids penalizing legitimate combos in games where the deck
// pivoted to a different line.
const outcomeMissDelta = -0.02

// ComputePredictionOutcomes walks the per-seat CardsPlayed in the
// GameAnalysis and reports whether each prediction fired. A
// prediction fires when every card in its Cards list was cast by
// the target seat (TurnCast > 0) during the game.
//
// Returns nil on degenerate inputs (nil game, out-of-range seat,
// empty predictions). On normal input, returns one PredictionOutcome
// per Prediction, in the same order, ready for
// RecordPredictionOutcomes persistence.
//
// The "all pieces cast by same seat" check is a deliberately loose
// definition of "combo assembled" — it doesn't require the cards to
// be on battlefield simultaneously, just that each was cast at some
// point. Tighter checks (e.g. requiring co-presence) would need
// per-turn battlefield snapshots which the current GameAnalysis
// doesn't expose. The loose check biases toward "prediction was
// reachable in this game" rather than "prediction was a one-shot
// win", which is the right signal for confidence calibration: a
// prediction the deck couldn't even cast all the pieces of clearly
// missed its mark; whether the cards on-battlefield-together-and-
// won is a stricter signal Heimdall can layer on later.
func ComputePredictionOutcomes(ga *analytics.GameAnalysis, seatIdx int, predictions []Prediction) []PredictionOutcome {
	if ga == nil || seatIdx < 0 || seatIdx >= len(ga.Players) || len(predictions) == 0 {
		return nil
	}
	cast := make(map[string]bool, len(ga.Players[seatIdx].CardsPlayed))
	for _, cp := range ga.Players[seatIdx].CardsPlayed {
		if cp.TurnCast > 0 {
			cast[cp.Name] = true
		}
	}
	out := make([]PredictionOutcome, 0, len(predictions))
	for _, p := range predictions {
		fired := len(p.Cards) > 0
		for _, c := range p.Cards {
			if !cast[c] {
				fired = false
				break
			}
		}
		delta := outcomeMissDelta
		if fired {
			delta = outcomeFiredDelta
		}
		out = append(out, PredictionOutcome{
			InstanceID:      p.InstanceID,
			Fired:           fired,
			ConfidenceDelta: delta,
		})
	}
	return out
}

// predictionOutcomesFile is the on-disk store appended to by
// RecordPredictionOutcomes. JSONL (one record per line) keeps append
// O(1) without re-reading the existing file, since the feedback
// loop's only consumer scans the whole file anyway.
const predictionOutcomesFile = "prediction_outcomes.jsonl"

// RecordPredictionOutcomes appends outcomes to prediction_outcomes.jsonl
// in the huginn dir, stamping each with the game ID for traceability.
// Creates the file if it doesn't exist. No-op on empty outcome slice
// or empty dir.
func RecordPredictionOutcomes(dir string, outcomes []PredictionOutcome, gameID string) error {
	if len(outcomes) == 0 || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("huginn: mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, predictionOutcomesFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("huginn: open %s: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, o := range outcomes {
		if o.GameID == "" {
			o.GameID = gameID
		}
		if err := enc.Encode(o); err != nil {
			return fmt.Errorf("huginn: encode outcome: %w", err)
		}
	}
	return nil
}

// ReadPredictionOutcomes reads back the full prediction-outcomes log.
// Returns an empty slice (not an error) when the file doesn't exist
// yet — first-run callers should treat absence as "no feedback yet"
// rather than a failure.
func ReadPredictionOutcomes(dir string) ([]PredictionOutcome, error) {
	path := filepath.Join(dir, predictionOutcomesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []PredictionOutcome
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		var o PredictionOutcome
		if err := dec.Decode(&o); err != nil {
			if err == io.EOF {
				break
			}
			return out, fmt.Errorf("huginn: decode outcome: %w", err)
		}
		out = append(out, o)
	}
	return out, nil
}

// AggregateConfidenceDeltas folds the outcome log into a per-pattern
// confidence delta map. Sums ConfidenceDelta per (InstanceID prefix
// before the final "-<seq>") so multiple predictions of the same
// pattern across many games converge to a single adjustment. Used by
// the next --predict round to update Huginn's confidence priors.
//
// Returns an empty map on empty input. The map key is the pattern-
// instance prefix (everything before the last "-"); if an InstanceID
// has no "-" the full ID is used as the key.
func AggregateConfidenceDeltas(outcomes []PredictionOutcome) map[string]float64 {
	deltas := map[string]float64{}
	for _, o := range outcomes {
		key := patternKeyFromInstanceID(o.InstanceID)
		deltas[key] += o.ConfidenceDelta
	}
	return deltas
}

// patternKeyFromInstanceID strips the trailing "-<seq>" from an
// InstanceID convention "<deck-fingerprint>-<pattern-key>-<seq>".
// Defensive on malformed IDs: returns the full ID when no "-" is
// found, so the aggregation never panics on hand-crafted test data.
func patternKeyFromInstanceID(id string) string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '-' {
			return id[:i]
		}
	}
	return id
}

