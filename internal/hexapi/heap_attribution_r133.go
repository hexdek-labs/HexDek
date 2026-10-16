package hexapi

import (
	"sync"
	"sync/atomic"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// heap_attribution_r133.go — abort the game that is actually big.
//
// r129 named the defect and could only half-fix it: runtime.ReadMemStats
// reports the WHOLE PROCESS while the grinder runs 2 workers inside it,
// so `HeapAlloc - baseline` is not this game's allocation. The guard
// therefore aborts whichever game happens to reach a checkpoint first,
// which need not be the one doing the allocating.
//
// On 2026-10-15 23:27:21 that stopped being theoretical:
//
//	23:27:17  heap 2511MB, 130.4 g/min            (escalation arms)
//	23:27:21  aborting game at turn 61 — delta 3363MB
//	23:27:21  SLOW GAME  13.8s turns=61  <- the game that was ABORTED
//	23:27:23  SLOW GAME 715.7s turns=73  <- completed normally, 2s later
//
// A 13.8-second game was killed while a 715.7-second game ran alongside
// it and finished untouched. (The round was still a win: turn 61 is the
// first non-multiple-of-10 abort in the log, which is r129's every-turn
// escalation working, and the server survived — this class used to take
// the process and its wrapper down. But the wrong game died.)
//
// Go exposes no per-goroutine allocation counter, so the delta cannot be
// attributed directly. What CAN be done is compare: when the process is
// over budget, the game with the most objects in it is the better
// suspect, and that is a COMPARISON rather than a threshold — no number
// has to be invented, and nothing has to be calibrated against a
// distribution we do not have yet.
//
// RACE SAFETY is why this is built out of atomics rather than a registry
// of *GameState. Reading another worker's game to size it up would be a
// data race on live state. Instead each worker publishes its OWN weight
// as an atomic at its own checkpoints, and the guard only ever reads
// other workers' published int64s.

// gameWeight sizes a game by the objects in it — the same counts
// gameCensus prints, collapsed to one number. A runaway shows up as an
// absurd quantity of something, so the sum is a reasonable proxy and
// does not care WHICH zone exploded.
//
// Called only by the owning worker, on its own game state.
func gameWeight(gs *gameengine.GameState) int64 {
	if gs == nil {
		return 0
	}
	w := int64(len(gs.Stack))
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		w += int64(len(seat.Battlefield) + len(seat.Hand) +
			len(seat.Graveyard) + len(seat.Exile) + len(seat.Library))
	}
	return w
}

// inflightWeights holds one published weight per in-flight game. The
// map is only ever mutated under mu; the values are atomics so the
// guard can read a peer's weight without touching its game.
var (
	inflightMu      sync.Mutex
	inflightWeights = map[*atomic.Int64]struct{}{}
)

// registerInflightGame returns this game's weight slot and the func that
// removes it. Always defer the returned func — a slot left behind would
// make a finished game look like a live suspect forever.
func registerInflightGame() (*atomic.Int64, func()) {
	slot := &atomic.Int64{}
	inflightMu.Lock()
	inflightWeights[slot] = struct{}{}
	inflightMu.Unlock()
	return slot, func() {
		inflightMu.Lock()
		delete(inflightWeights, slot)
		inflightMu.Unlock()
	}
}

// heaviestPeerWeight reports the largest weight published by any OTHER
// in-flight game.
func heaviestPeerWeight(self *atomic.Int64) int64 {
	var max int64
	inflightMu.Lock()
	for slot := range inflightWeights {
		if slot == self {
			continue
		}
		if v := slot.Load(); v > max {
			max = v
		}
	}
	inflightMu.Unlock()
	return max
}

// shouldDeferAbort reports whether this game should be spared because a
// peer is visibly larger and is therefore the better suspect.
//
// Deliberately conservative in the direction that keeps the box alive:
//
//   - the absolute process ceiling is NEVER deferred. Once the heap is
//     past maxProcessHeap the priority is relieving pressure, not
//     attributing blame, so whoever notices aborts.
//   - a tie never defers. Equal weights mean no evidence either way, and
//     deferring both would abort neither.
//   - deferring is safe only because r129 made every in-flight game
//     sample EVERY turn once the delta passes half the budget: the peer
//     that is actually big will reach its own check shortly. Without
//     that escalation this would risk sparing everyone.
func shouldDeferAbort(abort string, self *atomic.Int64, selfWeight int64) bool {
	if abort != heapAbortDelta {
		return false
	}
	return heaviestPeerWeight(self) > selfWeight
}
