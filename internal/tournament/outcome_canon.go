package tournament

// outcome_canon.go — shared, seat-indexed outcome bookkeeping used by
// BOTH the live tournament runner (runOneGame) and heimdall's
// anti-cheat replay verifier (replayForOutcome).
//
// r62 anti-cheat fix (review 08, C-H2): the seal side and the
// replay-verify side previously built their seedcontract.Outcome
// through two independent code paths. The replay side hardcoded
// EliminationOrder to all -1 and derived EndReason from a reduced
// vocabulary ("turn_cap" where the runner says "turn_cap_leader" /
// "turn_cap_tie" / "turn_cap_all_dead"), so the outcome digest of an
// honestly-replayed game NEVER matched the sealed digest — the
// verifier structurally rejected every honest game. Both sides now
// route through ElimTracker + AdjudicateGameEnd, making the digested
// fields identical by construction.
//
// Nothing in here may read wall-clock time: every value must be a
// deterministic function of the game state so a replay from the same
// RNG seed reproduces the same canonical outcome.

import (
	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/seedcontract"
)

// ElimTracker assigns elimination-order slots to seats. Slot 0 = first
// seat eliminated; the winner receives the last slot via FillRemaining.
// Slots are SEAT-indexed (the contract digest's order); the runner maps
// them to deck/commander indices separately for ELO aggregation.
//
// Slot-assignment rules (the canonical spec both sides follow):
//   - Mark is called once before the turn loop and once after every
//     turn's state-based actions. Any seat newly observed Lost gets the
//     next slot; ties within one Mark call break by ascending seat index.
//   - FillRemaining assigns the remaining slots, in ascending seat
//     index, to every seat still unassigned — regardless of Lost status.
//     (Turn-cap adjudication marks losers Lost AFTER the final Mark, so
//     cap losers and the cap winner are both filled here, in seat
//     order. That matches the historical runner behavior the existing
//     signed-contract corpus was sealed with — do not "fix" the winner
//     to always land last without bumping seedcontract.Schema.)
type ElimTracker struct {
	// Slots[seat] = elimination slot, or -1 while unassigned.
	Slots [seedcontract.MaxSeats]int
	next  int
}

// NewElimTracker returns a tracker with every slot unassigned.
func NewElimTracker() *ElimTracker {
	t := &ElimTracker{}
	for i := range t.Slots {
		t.Slots[i] = -1
	}
	return t
}

// Mark assigns the next slot(s) to any seat that is Lost and not yet
// slotted. Seat-index order breaks same-call ties.
func (t *ElimTracker) Mark(gs *gameengine.GameState) {
	for i, s := range gs.Seats {
		if i >= seedcontract.MaxSeats {
			break
		}
		if s != nil && s.Lost && t.Slots[i] < 0 {
			t.Slots[i] = t.next
			t.next++
		}
	}
}

// FillRemaining assigns the remaining slots, in seat order, to every
// seat that never received one (the winner, plus turn-cap losers marked
// Lost after the last Mark). Call exactly once, after AdjudicateGameEnd.
func (t *ElimTracker) FillRemaining(gs *gameengine.GameState) {
	for i, s := range gs.Seats {
		if i >= seedcontract.MaxSeats {
			break
		}
		if s != nil && t.Slots[i] < 0 {
			t.Slots[i] = t.next
			t.next++
		}
	}
}

// AdjudicateGameEnd applies the runner's end-of-game rules to a state
// that just exited the turn loop and returns (winner, endReason).
// winner is -1 for draws / unresolved games.
//
// endedNaturally reports whether the loop broke via gs.CheckEnd()
// (a real in-game ending). When false, the turn cap was hit and the
// life-leader adjudication runs: living seats below the top life total
// are marked Lost ("turn_cap"), tied non-winning leaders are marked
// Lost ("turn_cap_tie"), and the lowest-seat-index leader wins.
//
// The function MUTATES gs exactly the way runOneGame historically did
// (Lost / LossReason / Won / Flags / game_end log events) — heimdall's
// replay path needs those mutations too so FillRemaining and FinalLife
// observe identical state on both sides.
func AdjudicateGameEnd(gs *gameengine.GameState, nSeats int, endedNaturally bool) (int, string) {
	winner := -1
	endReason := ""

	// Natural ending: the engine set the flags.
	if gs.Flags != nil {
		if _, ok := gs.Flags["winner"]; ok && gs.Flags["ended"] == 1 {
			w := gs.Flags["winner"]
			if w >= 0 && w < nSeats {
				winner = w
				endReason = "last_seat_standing"
			}
		} else if gs.Flags["ended"] == 1 {
			endReason = "draw"
		}
	}
	if endedNaturally {
		return winner, endReason
	}

	// Turn cap: adjudicate by life among living seats.
	if gs.Flags == nil {
		gs.Flags = make(map[string]int)
	}
	gs.Flags["turn_capped"] = 1
	gs.Flags["ended"] = 1

	living := []int{}
	for i, s := range gs.Seats {
		if s != nil && !s.Lost {
			living = append(living, i)
		}
	}
	if len(living) == 0 {
		endReason = "turn_cap_all_dead"
		gs.LogEvent(gameengine.Event{
			Kind: "game_end", Seat: -1, Target: -1,
			Details: map[string]interface{}{
				"reason": "turn_cap_all_dead",
				"winner": -1,
			},
		})
		return -1, endReason
	}

	topLife := gs.Seats[living[0]].Life
	for _, i := range living[1:] {
		if gs.Seats[i].Life > topLife {
			topLife = gs.Seats[i].Life
		}
	}
	leaders := []int{}
	for _, i := range living {
		if gs.Seats[i].Life == topLife {
			leaders = append(leaders, i)
		}
	}
	for _, i := range living {
		if gs.Seats[i].Life < topLife {
			gs.Seats[i].Lost = true
			gs.Seats[i].LossReason = "turn_cap"
		}
	}
	// Seat-order tiebreak: lowest seat index among tied leaders wins.
	winner = leaders[0]
	if len(leaders) > 1 {
		for _, i := range leaders[1:] {
			gs.Seats[i].Lost = true
			gs.Seats[i].LossReason = "turn_cap_tie"
		}
		endReason = "turn_cap_tie"
	} else {
		endReason = "turn_cap_leader"
	}
	gs.Seats[winner].Won = true
	gs.Flags["winner"] = winner
	gs.LogEvent(gameengine.Event{
		Kind: "game_end", Seat: winner, Target: -1,
		Details: map[string]interface{}{
			"reason": endReason,
			"winner": winner,
			"life":   topLife,
		},
	})
	return winner, endReason
}

// FinalLifeFromState reads each seat's final life into the contract's
// fixed-size array. Shared by seal and replay so both digest the same
// values for the same state.
func FinalLifeFromState(gs *gameengine.GameState, nSeats int) [seedcontract.MaxSeats]int {
	var life [seedcontract.MaxSeats]int
	for i := 0; i < nSeats && i < seedcontract.MaxSeats; i++ {
		if i < len(gs.Seats) && gs.Seats[i] != nil {
			life[i] = gs.Seats[i].Life
		}
	}
	return life
}
