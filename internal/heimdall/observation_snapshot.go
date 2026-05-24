package heimdall

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// GameObservationSnapshot is the persisted subset of an Observation
// that's needed to re-build a GameSummary after a game has ended.
// Full Observations carry CoTriggers, ZoneCastEvents, and other
// large slices that the GameSummary doesn't surface — those stay
// in-memory and are routed straight to Huginn/Muninn sinks. The
// snapshot is what the showmatch_game_observation table stores as
// a JSON blob.
//
// Keep this struct field-stable: changing the JSON shape
// invalidates every persisted snapshot. New fields are fine
// (omitempty), removing or renaming is not.
type GameObservationSnapshot struct {
	Seed                GameSeed             `json:"seed"`
	CommanderZoneVisits []CommanderZoneVisit `json:"commander_zone_visits,omitempty"`
	RegretCards         []RegretCard         `json:"regret_cards,omitempty"`
	MVPCards            []MVPCard            `json:"mvp_cards,omitempty"`

	// CardFirstPlayed mirrors gameengine.GameState.CardFirstPlayed —
	// needed at re-summary time to derive commander_first_cast
	// turning points without re-running the game.
	CardFirstPlayed map[string]int `json:"card_first_played,omitempty"`

	// CommanderNamesBySeat lets the re-summarizer enumerate which
	// commander names belong to which seat. Stored per-seat because
	// CommanderNames lives on the Seat at runtime, not on the
	// Observation.
	CommanderNamesBySeat [][]string `json:"commander_names_by_seat,omitempty"`

	// FinalTurn is gs.Turn at game-end. Used to derive the game_end
	// turning point when the parent GameSeed's Turns hasn't yet been
	// stamped (some callers populate Seed.Turns lazily).
	FinalTurn int `json:"final_turn,omitempty"`
}

// SnapshotFromObservation extracts the GameSummary-relevant slice of
// an Observation, pulling per-seat commander names and the card-
// first-played map off the GameState. gs may be nil — in that case
// the snapshot just carries what's already on obs (the three R60
// signal arrays plus Seed); turning-point reconstruction will
// degrade to game_end only.
func SnapshotFromObservation(obs Observation, gs *gameengine.GameState) GameObservationSnapshot {
	snap := GameObservationSnapshot{
		Seed:                obs.Seed,
		CommanderZoneVisits: obs.CommanderZoneVisits,
		RegretCards:         obs.RegretCards,
		MVPCards:            obs.MVPCards,
	}
	if gs == nil {
		return snap
	}
	snap.FinalTurn = gs.Turn
	if gs.CardFirstPlayed != nil {
		snap.CardFirstPlayed = make(map[string]int, len(gs.CardFirstPlayed))
		for k, v := range gs.CardFirstPlayed {
			snap.CardFirstPlayed[k] = v
		}
	}
	snap.CommanderNamesBySeat = make([][]string, len(gs.Seats))
	for i, seat := range gs.Seats {
		if seat == nil || len(seat.CommanderNames) == 0 {
			continue
		}
		names := make([]string, len(seat.CommanderNames))
		copy(names, seat.CommanderNames)
		snap.CommanderNamesBySeat[i] = names
	}
	return snap
}

// MarshalSnapshot returns the canonical JSON representation of a
// snapshot — what goes into the showmatch_game_observation.payload
// column. Errors only on encoder failures (none expected for the
// concrete types here), but propagated rather than panicking so
// callers can log.
func MarshalSnapshot(snap GameObservationSnapshot) (string, error) {
	b, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}
	return string(b), nil
}

// UnmarshalSnapshot parses a payload string produced by
// MarshalSnapshot. Returns a wrapped error on malformed JSON so
// loading code can distinguish parse failures from missing-row
// (sql.ErrNoRows) cases at the DB layer.
func UnmarshalSnapshot(payload string) (GameObservationSnapshot, error) {
	var snap GameObservationSnapshot
	if err := json.Unmarshal([]byte(payload), &snap); err != nil {
		return snap, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return snap, nil
}

// BuildGameSummaryFromSnapshot builds a "rich" GameSummary from a
// persisted snapshot — the historical-game path used by the
// /api/games/{id}/summary endpoint when an observation has been
// stored.
//
// Turning-point derivation mirrors ExtractTurningPoints but reads
// CommanderNamesBySeat + CardFirstPlayed off the snapshot instead
// of off a live GameState. The endReason is plumbed in by the
// caller (typically from the GameRecord).
func BuildGameSummaryFromSnapshot(snap GameObservationSnapshot, endReason string) GameSummary {
	gameID := fmt.Sprintf("%d", snap.Seed.RNGSeed)
	turns := snap.Seed.Turns
	if snap.FinalTurn > turns {
		turns = snap.FinalTurn
	}

	deckKeys := make([]string, 0, len(snap.Seed.DeckKeys))
	for _, k := range snap.Seed.DeckKeys {
		deckKeys = append(deckKeys, k)
	}

	return GameSummary{
		GameID:              gameID,
		Winner:              snap.Seed.Winner,
		Turns:               turns,
		EndReason:           endReason,
		DeckKeys:            deckKeys,
		CommanderZoneVisits: snap.CommanderZoneVisits,
		RegretCards:         snap.RegretCards,
		MVPCards:            snap.MVPCards,
		TurningPoints:       turningPointsFromSnapshot(snap),
		DataSource:          "rich",
	}
}

// turningPointsFromSnapshot mirrors ExtractTurningPoints' two
// supported kinds (commander_first_cast + game_end) but reads from
// snapshot fields. Kept private — callers should go through
// BuildGameSummaryFromSnapshot, which knows about endReason etc.
func turningPointsFromSnapshot(snap GameObservationSnapshot) []TurningPoint {
	var out []TurningPoint
	if snap.CardFirstPlayed != nil {
		for seatIdx, names := range snap.CommanderNamesBySeat {
			for _, name := range names {
				turn, ok := snap.CardFirstPlayed[name]
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

	endTurn := snap.Seed.Turns
	if snap.FinalTurn > endTurn {
		endTurn = snap.FinalTurn
	}
	winner := snap.Seed.Winner
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
