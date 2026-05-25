package heimdall

import (
	"fmt"
	"sort"
)

// ReplayEvent is one chronological entry in a historical game's
// replay log — the per-event stream produced from a persisted
// GameObservationSnapshot for consumption by the legality-auditor
// UI and any other "what happened, in order" surface.
//
// Kind is a fixed small enum:
//
//   - "card_first_played": the first turn a card name resolved as a
//     spell, derived from CardFirstPlayed. Seat is the controlling
//     seat when the name matches one of the snapshot's per-seat
//     commander lists; otherwise -1 (snapshot doesn't carry per-
//     card-name ownership for non-commanders).
//   - "mvp_emerged": when an MVP card first hit the battlefield,
//     derived from MVPCard.TurnPlayed. Always seat-attributed.
//   - "game_end": one terminal record per replay log, carrying the
//     winning seat (-1 for draws) and the final turn.
//
// Forward-compatible: consumers iterate Kind and silently ignore
// unknown values, so adding new kinds is non-breaking.
type ReplayEvent struct {
	Turn        int    `json:"turn"`
	Kind        string `json:"kind"`
	Seat        int    `json:"seat"`
	CardName    string `json:"card_name,omitempty"`
	Score       int    `json:"score,omitempty"`
	IsCommander bool   `json:"is_commander,omitempty"`
	Description string `json:"description"`
}

// BuildReplayLog walks a snapshot and emits the full chronological
// event list. Sort order:
//
//  1. Turn ascending — the dominant axis; the timeline reads top-to-
//     bottom in game order.
//  2. Within a turn, game_end always sorts last so it never
//     interleaves with same-turn plays.
//  3. Otherwise card_first_played before mvp_emerged (alphabetical,
//     incidentally giving "what entered" before "what mattered").
//  4. Seat ascending, then CardName ascending — deterministic
//     tie-break so the output is byte-stable across runs.
//
// The returned slice is always non-nil so streaming callers can
// always range over it; a snapshot with no card-first-played
// entries and no MVPs still emits the game_end event.
func BuildReplayLog(snap GameObservationSnapshot) []ReplayEvent {
	var out []ReplayEvent

	// Build the seat-by-commander reverse lookup once so we can
	// attribute card_first_played events when the name happens to
	// be a known commander.
	commanderSeat := make(map[string]int)
	for seatIdx, names := range snap.CommanderNamesBySeat {
		for _, n := range names {
			commanderSeat[n] = seatIdx
		}
	}

	for name, turn := range snap.CardFirstPlayed {
		if turn <= 0 {
			continue
		}
		seat := -1
		if s, ok := commanderSeat[name]; ok {
			seat = s
		}
		out = append(out, ReplayEvent{
			Turn:        turn,
			Kind:        "card_first_played",
			Seat:        seat,
			CardName:    name,
			IsCommander: seat >= 0,
			Description: fmt.Sprintf("%q first resolved on turn %d", name, turn),
		})
	}

	for _, mvp := range snap.MVPCards {
		if mvp.TurnPlayed <= 0 {
			continue
		}
		out = append(out, ReplayEvent{
			Turn:        mvp.TurnPlayed,
			Kind:        "mvp_emerged",
			Seat:        mvp.Seat,
			CardName:    mvp.CardName,
			Score:       mvp.Score,
			IsCommander: mvp.IsCommander,
			Description: fmt.Sprintf("MVP %q (score=%d) emerged for seat %d on turn %d",
				mvp.CardName, mvp.Score, mvp.Seat, mvp.TurnPlayed),
		})
	}

	endTurn := snap.Seed.Turns
	if snap.FinalTurn > endTurn {
		endTurn = snap.FinalTurn
	}
	winner := snap.Seed.Winner
	endDesc := fmt.Sprintf("seat %d won on turn %d", winner, endTurn)
	if winner < 0 {
		endDesc = fmt.Sprintf("game ended without a winner on turn %d", endTurn)
	}
	out = append(out, ReplayEvent{
		Turn:        endTurn,
		Kind:        "game_end",
		Seat:        winner,
		Description: endDesc,
	})

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Turn != out[j].Turn {
			return out[i].Turn < out[j].Turn
		}
		// game_end always last within its turn.
		if out[i].Kind == "game_end" && out[j].Kind != "game_end" {
			return false
		}
		if out[j].Kind == "game_end" && out[i].Kind != "game_end" {
			return true
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Seat != out[j].Seat {
			return out[i].Seat < out[j].Seat
		}
		return out[i].CardName < out[j].CardName
	})
	return out
}
