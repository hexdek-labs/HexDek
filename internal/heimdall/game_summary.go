package heimdall

import (
	"fmt"
	"sort"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// GameSummary is the consolidated post-game blob combining the R60-era
// Heimdall signals (commander-zone visits, regret cards, MVP cards)
// with high-level outcome metadata and a turning-points timeline. One
// of these is returned per game by hexapi's /api/games/{id}/summary
// endpoint.
//
// DataSource discriminates how the summary was assembled:
//
//   - "rich": built from a populated in-memory Observation
//     (CommanderZoneVisits / RegretCards / MVPCards all live)
//   - "db_only": built from persisted game-record fields only
//     (the three R60 signals are empty placeholders — the Observation
//     pipeline isn't backed by durable storage today, so historical
//     games can only surface metadata + game_end turning point)
type GameSummary struct {
	GameID              string               `json:"game_id"`
	Winner              int                  `json:"winner"`
	WinnerName          string               `json:"winner_name,omitempty"`
	Turns               int                  `json:"turns"`
	EndReason           string               `json:"end_reason,omitempty"`
	DeckKeys            []string             `json:"deck_keys,omitempty"`
	CommanderZoneVisits []CommanderZoneVisit `json:"commander_zone_visits"`
	RegretCards         []RegretCard         `json:"regret_cards"`
	MVPCards            []MVPCard            `json:"mvp_cards"`
	MulliganStats       []MulliganStat       `json:"mulligan_stats,omitempty"`
	TurningPoints       []TurningPoint       `json:"turning_points"`
	HatDecisions        []HatDecision        `json:"hat_decisions,omitempty"`
	DataSource          string               `json:"data_source"`
}

// HatDecision is one row of the "why did hat do X?" log surfaced in
// GameSummary. The hat already emits per-decision events to
// gs.EventLog via emitDecisionEvent (Kind: "hat_decision_*"); this
// struct projects those events into a queryable slice ordered by
// turn + seat + insertion order.
//
// Kind discriminates the decision shape (mulligan / cast / attack_
// target / block / response_counter / mode) and Details carries the
// per-kind context (hand stats for mulligan, candidate scores for
// cast, etc.). The Archetype field is the hat's belief at decision
// time — stamped on every event by emitDecisionEvent so post-game
// analysts don't have to rejoin against the deck index.
//
// Surfacing this list in GameSummary lets the spectator UI render
// "hover any major decision → see why the hat chose it" without the
// caller having to walk EventLog themselves.
type HatDecision struct {
	Turn      int                    `json:"turn"`
	Seat      int                    `json:"seat"`
	Kind      string                 `json:"kind"`             // mulligan / cast / attack_target / block / mode / response_counter
	Archetype string                 `json:"archetype,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// TurningPoint records one decision-shaping moment in a game. The
// Kind set is small and well-known:
//
//   - "commander_first_cast": the turn a seat first cast its
//     commander, taken from CardFirstPlayed
//   - "game_end": the final turn the game resolved on, attributed to
//     the winning seat
//
// Per-turn life/board snapshots would unlock richer kinds
// ("life_crossed_threshold", "first_elimination") but they require
// per-turn snapshotting that the fast replay path doesn't currently
// pay for. Out of scope here; the struct shape is forward-compatible
// (consumers iterate Kind and ignore unknown values).
type TurningPoint struct {
	Turn        int    `json:"turn"`
	Kind        string `json:"kind"`
	Seat        int    `json:"seat"`
	Description string `json:"description"`
}

// BuildGameSummary assembles a GameSummary from a populated
// Observation plus a final-state GameState. The three R60 signals on
// the returned summary are copied directly off obs (which is presumed
// already populated by the ExtractCommanderZoneVisits /
// ExtractRegretCards / ExtractMVPCards path in ReplayWithObservation).
// Turning points are derived via ExtractTurningPoints.
//
// gs may be nil — in that case turning-point derivation falls back to
// what's available on obs alone (which is enough for game_end but
// drops per-seat commander_first_cast since CardFirstPlayed lives on
// gs, not obs).
//
// endReason is plumbed through so the endpoint surface can echo the
// kill-method label the engine settled on.
func BuildGameSummary(obs Observation, gs *gameengine.GameState, endReason string) GameSummary {
	gameID := fmt.Sprintf("%d", obs.Seed.RNGSeed)
	winner := obs.Seed.Winner
	turns := obs.Seed.Turns
	if gs != nil && gs.Turn > turns {
		turns = gs.Turn
	}

	deckKeys := make([]string, 0, len(obs.Seed.DeckKeys))
	for _, k := range obs.Seed.DeckKeys {
		deckKeys = append(deckKeys, k)
	}

	return GameSummary{
		GameID:              gameID,
		Winner:              winner,
		Turns:               turns,
		EndReason:           endReason,
		DeckKeys:            deckKeys,
		CommanderZoneVisits: obs.CommanderZoneVisits,
		RegretCards:         obs.RegretCards,
		MVPCards:            obs.MVPCards,
		MulliganStats:       obs.MulliganStats,
		TurningPoints:       ExtractTurningPoints(obs, gs),
		HatDecisions:        ExtractHatDecisions(gs),
		DataSource:          "rich",
	}
}

// ExtractHatDecisions walks gs.EventLog for every "hat_decision_*"
// event and projects each into a HatDecision row. Order is preserved
// from EventLog (which is chronological by insertion). Returns an
// empty slice when gs is nil or no hat decisions were recorded.
//
// The strip-prefix uses "hat_decision_" verbatim — kept in sync with
// emitDecisionEvent in internal/hat/yggdrasil.go.
func ExtractHatDecisions(gs *gameengine.GameState) []HatDecision {
	if gs == nil || len(gs.EventLog) == 0 {
		return nil
	}
	const prefix = "hat_decision_"
	out := make([]HatDecision, 0, 16)
	for _, ev := range gs.EventLog {
		if len(ev.Kind) <= len(prefix) || ev.Kind[:len(prefix)] != prefix {
			continue
		}
		kind := ev.Kind[len(prefix):]
		// Defensive copy so consumers can't mutate the event log via
		// the returned slice. Skip the "turn" key since we surface it
		// as a typed field, and lift "archetype" out into its own
		// field for the same reason.
		details := make(map[string]interface{}, len(ev.Details))
		var archetype string
		for k, v := range ev.Details {
			switch k {
			case "turn":
				continue
			case "archetype":
				if s, ok := v.(string); ok {
					archetype = s
				} else {
					details[k] = v
				}
			default:
				details[k] = v
			}
		}
		turn := ev.Amount
		if t, ok := ev.Details["turn"].(int); ok && t > 0 {
			turn = t
		}
		out = append(out, HatDecision{
			Turn:      turn,
			Seat:      ev.Seat,
			Kind:      kind,
			Archetype: archetype,
			Details:   details,
		})
	}
	return out
}

// ExtractTurningPoints derives the small set of turning points that
// can be built from end-state alone:
//
//   - "commander_first_cast": one per (seat, commander) pair where
//     gs.CardFirstPlayed has a turn entry. Skipped if gs is nil or
//     the commander never resolved.
//   - "game_end": one record marking the final turn and winner. Emitted
//     even when winner == -1 (draw / stalled game), with an explanatory
//     description.
//
// Sorted chronologically (Turn ascending, Seat ascending, Kind
// ascending) so the timeline reads top-to-bottom.
func ExtractTurningPoints(obs Observation, gs *gameengine.GameState) []TurningPoint {
	var out []TurningPoint

	if gs != nil && gs.CardFirstPlayed != nil {
		for seatIdx, seat := range gs.Seats {
			if seat == nil {
				continue
			}
			for _, name := range seat.CommanderNames {
				turn, ok := gs.CardFirstPlayed[name]
				if !ok || turn <= 0 {
					continue
				}
				out = append(out, TurningPoint{
					Turn:        turn,
					Kind:        "commander_first_cast",
					Seat:        seatIdx,
					Description: fmt.Sprintf("seat %d cast commander %q on turn %d", seatIdx, name, turn),
				})
			}
		}
	}

	endTurn := obs.Seed.Turns
	if gs != nil && gs.Turn > endTurn {
		endTurn = gs.Turn
	}
	winner := obs.Seed.Winner
	desc := fmt.Sprintf("seat %d won on turn %d", winner, endTurn)
	if winner < 0 {
		desc = fmt.Sprintf("game ended without a winner on turn %d", endTurn)
	}
	out = append(out, TurningPoint{
		Turn:        endTurn,
		Kind:        "game_end",
		Seat:        winner,
		Description: desc,
	})

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Turn != out[j].Turn {
			return out[i].Turn < out[j].Turn
		}
		if out[i].Seat != out[j].Seat {
			return out[i].Seat < out[j].Seat
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
