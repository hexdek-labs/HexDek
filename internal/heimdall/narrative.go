package heimdall

import (
	"fmt"
	"sort"
	"strings"
)

// SummarizeGame produces a 1-paragraph plain-text narrative of a
// completed game from a GameObservationSnapshot — the same JSON
// payload used by the showmatch_game_observation table and the
// /api/games/{id}/summary endpoint.
//
// The narrative is winner-focused: outcome first, then the winner's
// commander cast turn, recast count if any, top MVPs, then the
// strongest opposing-seat stuck-in-hand regret note (when
// meaningful). Each fact is one sentence; the result is a single
// paragraph suitable for chat surfaces, match recaps, or release
// notes. No markdown — plain English with periods.
//
// Edge cases handled:
//
//   - Draws (Seed.Winner < 0) get a draw-framed opening sentence.
//   - Snapshots with empty MVPCards / CardFirstPlayed skip the
//     corresponding sentence cleanly rather than emitting "no data"
//     placeholders.
//   - Kill methods are surfaced verbatim from Seed.KillMethod with a
//     friendly remap ("commander" → "commander damage", "combo" →
//     "a combo finish", etc.) so the sentence reads naturally.
//
// Output never has leading/trailing whitespace and always ends with
// a period. Empty input returns "Game data unavailable.".
func SummarizeGame(snap GameObservationSnapshot) string {
	if snap.Seed.Turns == 0 && snap.FinalTurn == 0 && len(snap.CommanderNamesBySeat) == 0 {
		return "Game data unavailable."
	}

	finalTurn := snap.Seed.Turns
	if snap.FinalTurn > finalTurn {
		finalTurn = snap.FinalTurn
	}
	if finalTurn == 0 {
		finalTurn = 1
	}

	winner := snap.Seed.Winner
	winnerName := commanderForSeat(snap, winner)

	var sentences []string

	// Opening: outcome.
	if winner >= 0 {
		sentences = append(sentences, openingWinSentence(winner, winnerName, snap.Seed.KillMethod, finalTurn))
	} else {
		sentences = append(sentences, fmt.Sprintf("The game ended without a winner on turn %d.", finalTurn))
	}

	// Mulligan context — only mention if at least one seat actually
	// mulliganed, otherwise it's noise.
	if mulSentence := mulliganSentence(snap.MulliganStats); mulSentence != "" {
		sentences = append(sentences, mulSentence)
	}

	// Winner's commander cast turn(s).
	if winner >= 0 && winnerName != "" {
		if cast := winnerCommanderSentence(snap, winner, winnerName); cast != "" {
			sentences = append(sentences, cast)
		}
	}

	// Winner's MVPs (top 2 by score, excluding the commander row
	// since we already mentioned it in the commander sentence).
	if winner >= 0 {
		if mvp := winnerMVPSentence(snap, winner, winnerName); mvp != "" {
			sentences = append(sentences, mvp)
		}
	}

	// Strongest opposing-seat regret (winner seat excluded).
	if regret := loserRegretSentence(snap, winner); regret != "" {
		sentences = append(sentences, regret)
	}

	return strings.Join(sentences, " ")
}

// SummarizeReplayEvents produces a thinner narrative from a chronological
// []ReplayEvent stream — the shape BuildReplayLog produces and the
// /api/replay/{id} NDJSON endpoint streams. commanderNamesBySeat is
// the same per-seat list the snapshot carries; pass nil to skip
// commander-name attribution (the summary degrades gracefully —
// generic "seat N" labels).
//
// Less rich than SummarizeGame because the event list doesn't carry
// MVP scores or mulligan stats, but covers the basics: outcome,
// notable card-first-played turns for the winner, MVPs emerged.
//
// Useful for surfaces that don't have the full snapshot — e.g., the
// NDJSON streaming endpoint replays a flat event sequence to clients
// who want a quick summary without round-tripping back to the snapshot
// store.
func SummarizeReplayEvents(events []ReplayEvent, commanderNamesBySeat [][]string) string {
	if len(events) == 0 {
		return "Replay event stream is empty."
	}

	// Find the game_end event to anchor outcome.
	var endEvent *ReplayEvent
	for i := range events {
		if events[i].Kind == "game_end" {
			endEvent = &events[i]
		}
	}
	if endEvent == nil {
		return "Replay event stream missing a game_end record."
	}

	winnerName := ""
	if endEvent.Seat >= 0 && endEvent.Seat < len(commanderNamesBySeat) && len(commanderNamesBySeat[endEvent.Seat]) > 0 {
		winnerName = commanderNamesBySeat[endEvent.Seat][0]
	}

	var sentences []string
	if endEvent.Seat >= 0 {
		// No kill method on ReplayEvent — emit a simpler win sentence.
		seatLabel := seatNameOrIndex(endEvent.Seat, winnerName)
		sentences = append(sentences, fmt.Sprintf("%s won on turn %d.", seatLabel, endEvent.Turn))
	} else {
		sentences = append(sentences, fmt.Sprintf("The game ended without a winner on turn %d.", endEvent.Turn))
	}

	// Winner's card_first_played in turn order.
	winnerCardTurns := make([]ReplayEvent, 0)
	for _, ev := range events {
		if ev.Kind != "card_first_played" || ev.Seat != endEvent.Seat {
			continue
		}
		winnerCardTurns = append(winnerCardTurns, ev)
	}
	if len(winnerCardTurns) > 0 && endEvent.Seat >= 0 {
		// Pick the earliest commander-tagged play.
		var earliestCommander *ReplayEvent
		for i := range winnerCardTurns {
			if winnerCardTurns[i].IsCommander {
				if earliestCommander == nil || winnerCardTurns[i].Turn < earliestCommander.Turn {
					earliestCommander = &winnerCardTurns[i]
				}
			}
		}
		if earliestCommander != nil {
			sentences = append(sentences,
				fmt.Sprintf("Their commander %s first resolved on turn %d.",
					quoted(earliestCommander.CardName), earliestCommander.Turn))
		}
	}

	// Top-2 MVPs across the whole event stream — flat for now, since
	// we don't have per-seat filtering on the event list (the snapshot
	// path handles that better).
	mvpEvents := make([]ReplayEvent, 0)
	for _, ev := range events {
		if ev.Kind == "mvp_emerged" {
			mvpEvents = append(mvpEvents, ev)
		}
	}
	if len(mvpEvents) > 0 {
		sort.SliceStable(mvpEvents, func(i, j int) bool {
			return mvpEvents[i].Score > mvpEvents[j].Score
		})
		topN := 2
		if topN > len(mvpEvents) {
			topN = len(mvpEvents)
		}
		names := make([]string, 0, topN)
		for i := 0; i < topN; i++ {
			names = append(names, quoted(mvpEvents[i].CardName))
		}
		sentences = append(sentences, fmt.Sprintf("Top MVPs at game end: %s.", strings.Join(names, " and ")))
	}

	return strings.Join(sentences, " ")
}

// openingWinSentence stitches the opening outcome sentence with a
// natural-reading kill-method clause.
func openingWinSentence(seat int, commanderName, killMethod string, finalTurn int) string {
	seatLabel := seatNameOrIndex(seat, commanderName)
	clause := killMethodClause(killMethod)
	if clause == "" {
		return fmt.Sprintf("%s won on turn %d.", seatLabel, finalTurn)
	}
	return fmt.Sprintf("%s won on turn %d via %s.", seatLabel, finalTurn, clause)
}

// killMethodClause turns the persisted KillMethod enum into a clause
// that reads naturally inside the opening sentence. Unknown / empty
// values return "" so the caller drops the "via …" tail.
func killMethodClause(killMethod string) string {
	switch strings.ToLower(strings.TrimSpace(killMethod)) {
	case "combat":
		return "combat damage"
	case "commander":
		return "commander damage"
	case "combo":
		return "a combo finish"
	case "mill":
		return "decking out an opponent"
	case "poison":
		return "infect / poison counters"
	case "timeout":
		return "" // timeout isn't a "via" clause; drop it
	case "":
		return ""
	default:
		return killMethod
	}
}

// mulliganSentence summarizes seats that mulliganed. Skips if all
// seats kept a 7-card opener (the modal case).
func mulliganSentence(stats []MulliganStat) string {
	if len(stats) == 0 {
		return ""
	}
	parts := make([]string, 0)
	for _, m := range stats {
		if m.MulligansTaken <= 0 {
			continue
		}
		seatLabel := fmt.Sprintf("Seat %d", m.Seat)
		parts = append(parts, fmt.Sprintf("%s mulliganed to %d", seatLabel, m.OpeningHandSize))
	}
	if len(parts) == 0 {
		return ""
	}
	// Sort for stability in tests.
	sort.Strings(parts)
	if len(parts) == 1 {
		return parts[0] + "."
	}
	return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1] + "."
}

// winnerCommanderSentence emits the commander cast turn + recast
// count clause. Returns "" when the snapshot has no CardFirstPlayed
// or CommanderZoneVisits data on the winner.
func winnerCommanderSentence(snap GameObservationSnapshot, winner int, commanderName string) string {
	turn, ok := snap.CardFirstPlayed[commanderName]
	if !ok || turn <= 0 {
		return ""
	}
	recastCount := 0
	for _, v := range snap.CommanderZoneVisits {
		if v.Seat == winner && v.CommanderName == commanderName {
			recastCount = v.CastCount
			break
		}
	}
	base := fmt.Sprintf("They cast %s on turn %d", quoted(commanderName), turn)
	if recastCount >= 3 {
		return base + fmt.Sprintf(" and recast it %d times across the game.", recastCount)
	}
	if recastCount == 2 {
		return base + " and recast it once after answering removal."
	}
	return base + "."
}

// winnerMVPSentence picks the top 2 MVPs on the winner's side
// (excluding the commander itself, already mentioned). Empty when no
// non-commander MVP was scored.
func winnerMVPSentence(snap GameObservationSnapshot, winner int, commanderName string) string {
	candidates := make([]MVPCard, 0)
	for _, m := range snap.MVPCards {
		if m.Seat != winner {
			continue
		}
		if m.CardName == commanderName {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].CardName < candidates[j].CardName
	})
	topN := 2
	if topN > len(candidates) {
		topN = len(candidates)
	}
	names := make([]string, 0, topN)
	for i := 0; i < topN; i++ {
		names = append(names, quoted(candidates[i].CardName))
	}
	if topN == 1 {
		return fmt.Sprintf("Their key supporting piece was %s.", names[0])
	}
	return fmt.Sprintf("Their key supporting pieces were %s and %s.", names[0], names[1])
}

// loserRegretSentence emits a regret note for the seat with the
// highest-CMC stranded card OTHER than the winner. Returns "" if
// no qualifying regret card exists or if the regret CMC is below 5
// (lower-CMC strands are noise).
func loserRegretSentence(snap GameObservationSnapshot, winner int) string {
	type seatRegret struct {
		seat    int
		card    string
		cmc     int
	}
	var best seatRegret
	for _, r := range snap.RegretCards {
		if r.Seat == winner {
			continue
		}
		if r.CMC < 5 {
			continue
		}
		if r.CMC > best.cmc {
			best = seatRegret{seat: r.Seat, card: r.CardName, cmc: r.CMC}
		}
	}
	if best.card == "" {
		return ""
	}
	return fmt.Sprintf("Seat %d never resolved their %d-cost %s.",
		best.seat, best.cmc, quoted(best.card))
}

// commanderForSeat returns the first commander name listed for `seat`,
// or "" if the snapshot doesn't have CommanderNamesBySeat populated /
// the seat is out of range.
func commanderForSeat(snap GameObservationSnapshot, seat int) string {
	if seat < 0 || seat >= len(snap.CommanderNamesBySeat) {
		return ""
	}
	if len(snap.CommanderNamesBySeat[seat]) == 0 {
		return ""
	}
	return snap.CommanderNamesBySeat[seat][0]
}

// seatNameOrIndex returns the commander label when available,
// otherwise "Seat N" as a generic fallback. Used by both summarizers
// to render the actor reference consistently.
func seatNameOrIndex(seat int, commanderName string) string {
	if commanderName != "" {
		return fmt.Sprintf("Seat %d (%s)", seat, commanderName)
	}
	return fmt.Sprintf("Seat %d", seat)
}

// quoted wraps a name in straight double-quotes for readability. The
// summarizer reads like prose, and quoting the card name distinguishes
// "they cast Tergrid on turn 4" from the rest of the sentence flow.
func quoted(s string) string {
	return `"` + s + `"`
}
