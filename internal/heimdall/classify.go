package heimdall

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/seedcontract"
)

// ClassifyKill determines how the winner won the game: the winner won by
// method X, where X is how the LAST surviving opponent was eliminated —
// the final elimination is the kill that closed the game.
//
// Pre-r62 this scanned eliminated seats in SEAT order and returned the
// first classifiable LossReason, so in any multi-elimination game the
// stored win_reason usually described an EARLIER elimination (often not
// even the winner's doing) — directionally wrong at scale (fleet review
// reports 06 C4-adjacent / 08). The final victim is found by:
//
//  1. Seat.LostOrder (max wins) — stamped by HandleSeatElimination in
//     elimination order, present regardless of RetainEvents;
//  2. the last "seat_eliminated" event, for states predating the stamp;
//  3. the highest-index Lost opponent, as a deterministic last resort
//     for hand-built fixtures that bypassed the §800.4a pipeline.
//
// The method string is normalized through the canonical
// seedcontract kill-method mapper so this surface, the tournament seal
// path, and the replay-verify path all speak one vocabulary.
//
// Returns one of: "poison", "commander", "mill", "combat", "timeout", "combo".
func ClassifyKill(gs *gameengine.GameState, winner int) string {
	if gs == nil {
		return "combat"
	}
	victim := finalEliminatedOpponent(gs, winner)
	if victim == nil {
		return "combat"
	}
	return seedcontract.CanonicalKillMethod(classifyLossOfSeat(victim))
}

// finalEliminatedOpponent picks the opponent whose elimination ended the
// game. See ClassifyKill for the resolution order.
func finalEliminatedOpponent(gs *gameengine.GameState, winner int) *gameengine.Seat {
	var best *gameengine.Seat
	bestOrder := 0
	for i, seat := range gs.Seats {
		if seat == nil || i == winner || !seat.Lost {
			continue
		}
		if seat.LostOrder > bestOrder {
			best = seat
			bestOrder = seat.LostOrder
		}
	}
	if best != nil {
		return best
	}
	// Legacy fallback: the last seat_eliminated event (in-order emission;
	// survives the fishtank's per-turn EventLog trim because the final
	// elimination is on the final turn).
	for j := len(gs.EventLog) - 1; j >= 0; j-- {
		ev := gs.EventLog[j]
		if ev.Kind != "seat_eliminated" || ev.Seat == winner {
			continue
		}
		if ev.Seat >= 0 && ev.Seat < len(gs.Seats) && gs.Seats[ev.Seat] != nil && gs.Seats[ev.Seat].Lost {
			return gs.Seats[ev.Seat]
		}
	}
	// Deterministic last resort for fixtures: highest-index lost opponent.
	for i := len(gs.Seats) - 1; i >= 0; i-- {
		if i == winner {
			continue
		}
		if seat := gs.Seats[i]; seat != nil && seat.Lost {
			return seat
		}
	}
	return nil
}

// classifyLossOfSeat maps ONE seat's loss onto the kill-method enum:
// LossReason first (the engine sets it with high fidelity), then the
// game-state heuristics the pre-r62 classifier used, applied to the same
// single seat.
func classifyLossOfSeat(seat *gameengine.Seat) string {
	reason := strings.ToLower(seat.LossReason)
	switch {
	case strings.Contains(reason, "poison"):
		return "poison"
	case strings.Contains(reason, "commander_damage") || strings.Contains(reason, "commander damage"):
		return "commander"
	case strings.Contains(reason, "empty library") || strings.Contains(reason, "704.5b"):
		return "mill"
	case strings.Contains(reason, "infinite") || strings.Contains(reason, "combo"):
		return "combo"
	}

	// Heuristic fallback: check the seat's state directly.
	if seat.PoisonCounters >= 10 {
		return "poison"
	}
	for _, cmdrDmg := range seat.CommanderDamage {
		for _, dmg := range cmdrDmg {
			if dmg >= 21 {
				return "commander"
			}
		}
	}
	// Lost with positive life and no poison/commander kill -> decked.
	if seat.Life > 0 && seat.PoisonCounters < 10 {
		return "mill"
	}
	// Default: damage reduced life to 0.
	return "combat"
}

// ClassifyKillWithMaxTurns is like ClassifyKill but also detects timeout
// when the game hit the turn limit without a natural winner.
func ClassifyKillWithMaxTurns(gs *gameengine.GameState, winner, maxTurns int) string {
	if gs != nil && maxTurns > 0 && gs.Turn >= maxTurns {
		// Route the turn-cap shape through THE canonical mapper so the
		// "timeout" spelling has exactly one source of truth.
		return seedcontract.KillMethodFromEndReason("turn_cap")
	}
	return ClassifyKill(gs, winner)
}

// SnapshotTurnCounters captures the per-seat TurnCounters into a fixed-size
// array suitable for embedding in GameSeed. Seats beyond index 3 are ignored
// (HexDek is 4-player Commander). Nil seats produce a zero snapshot.
//
// NOTE: TurnCounters are reset every turn (see Reset() in UntapAll), so this
// captures only the FINAL turn's totals — not game-wide cumulative values.
// Callers that want game-wide aggregates must accumulate during play.
func SnapshotTurnCounters(gs *gameengine.GameState) [4]TurnCounterSnapshot {
	var out [4]TurnCounterSnapshot
	if gs == nil {
		return out
	}
	for i, seat := range gs.Seats {
		if i >= 4 || seat == nil {
			continue
		}
		out[i] = snapshotOne(&seat.Turn)
	}
	return out
}

func snapshotOne(tc *gameengine.TurnCounters) TurnCounterSnapshot {
	if tc == nil {
		return TurnCounterSnapshot{}
	}
	return TurnCounterSnapshot{
		LifeGained:          tc.LifeGained,
		LifeLost:            tc.LifeLost,
		DamageReceived:      tc.DamageReceived,
		LifePaid:            tc.LifePaid,
		CardsDrawn:          tc.CardsDrawn,
		SpellsCast:          tc.SpellsCast,
		CreaturesEntered:    tc.CreaturesEntered,
		ArtifactsEntered:    tc.ArtifactsEntered,
		EnchantmentsEntered: tc.EnchantmentsEntered,
		TokensCreated:       tc.TokensCreated,
		TreasuresCreated:    tc.TreasuresCreated,
		Sacrificed:          tc.Sacrificed,
		PermanentsLeft:      tc.PermanentsLeft,
		Discarded:           tc.Discarded,
		Milled:              tc.Milled,
		LandsPlayed:         tc.LandsPlayed,
		CreaturesDied:       tc.CreaturesDied,
		ExiledCards:         tc.ExiledCards,
		CastFromExile:       tc.CastFromExile,
		Descended:           tc.Descended,
		Attacked:            tc.Attacked,
	}
}
