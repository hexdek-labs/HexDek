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
	TurningPoints       []TurningPoint       `json:"turning_points"`
	DataSource          string               `json:"data_source"`
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
		TurningPoints:       ExtractTurningPoints(obs, gs),
		DataSource:          "rich",
	}
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
