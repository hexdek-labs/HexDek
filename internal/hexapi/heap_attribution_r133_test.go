package hexapi

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r133 — the guard must abort the game that is actually big.
//
// The live instance it was built from, 2026-10-15 23:27:
//
//	aborting game at turn 61 — delta 3363MB
//	SLOW GAME  13.8s turns=61   <- aborted
//	SLOW GAME 715.7s turns=73   <- completed normally, 2 seconds later
//
// A 13.8-second game died beside a 715.7-second one. The delta is
// process-wide (r129), so the guard was blaming whoever checked first.

// r133Game builds a game of a known size using exported fields only
// (r131Seat / r131Perms reused).
func r133Game(bfPerSeat, gyPerSeat, stack int) *gameengine.GameState {
	return &gameengine.GameState{
		Stack: make([]*gameengine.StackItem, stack),
		Seats: []*gameengine.Seat{
			r131Seat(40, r131Perms(bfPerSeat, 0), 0, gyPerSeat, 0, 0),
			r131Seat(40, r131Perms(bfPerSeat, 0), 0, gyPerSeat, 0, 0),
		},
	}
}

func TestR133_WeightTracksTheObjectsInTheGame(t *testing.T) {
	small := gameWeight(r133Game(2, 3, 1))
	big := gameWeight(r133Game(2000, 3, 1))
	if small != 1+2*(2+3) {
		t.Fatalf("weight should count stack + every zone of every seat; got %d", small)
	}
	if big <= small {
		t.Fatalf("a 2000-permanent game must outweigh a 2-permanent one (%d vs %d)", big, small)
	}
	if gameWeight(nil) != 0 {
		t.Error("a nil game weighs nothing rather than panicking")
	}
}

// THE BUG: a small game next to a huge peer must NOT be the one aborted.
func TestR133_SmallGameDefersToALargerPeer(t *testing.T) {
	resetInflight(t)
	self, releaseSelf := registerInflightGame()
	defer releaseSelf()
	peer, releasePeer := registerInflightGame()
	defer releasePeer()

	selfWeight := gameWeight(r133Game(4, 2, 0)) // the 13.8s game
	self.Store(selfWeight)
	peer.Store(gameWeight(r133Game(4000, 900, 50))) // the 715.7s game

	if !shouldDeferAbort(heapAbortDelta, self, selfWeight) {
		t.Fatal("a tiny game beside a vastly larger peer must defer the abort to it")
	}
}

// MUST STILL ABORT: the big game itself has no larger peer, so it is
// the one that dies.
func TestR133_LargestGameStillAborts(t *testing.T) {
	resetInflight(t)
	self, releaseSelf := registerInflightGame()
	defer releaseSelf()
	peer, releasePeer := registerInflightGame()
	defer releasePeer()

	selfWeight := gameWeight(r133Game(4000, 900, 50))
	self.Store(selfWeight)
	peer.Store(gameWeight(r133Game(4, 2, 0)))

	if shouldDeferAbort(heapAbortDelta, self, selfWeight) {
		t.Fatal("the largest in-flight game must not defer — nothing else would abort")
	}
}

// MUST NEVER DEFER: the absolute process ceiling is about keeping the
// box alive, not about assigning blame.
func TestR133_ProcessCeilingIsNeverDeferred(t *testing.T) {
	resetInflight(t)
	self, releaseSelf := registerInflightGame()
	defer releaseSelf()
	peer, releasePeer := registerInflightGame()
	defer releasePeer()

	selfWeight := gameWeight(r133Game(1, 1, 0))
	self.Store(selfWeight)
	peer.Store(gameWeight(r133Game(9000, 9000, 99)))

	if shouldDeferAbort(heapAbortCeiling, self, selfWeight) {
		t.Fatal("the 6GB ceiling must abort whoever notices, however small they are")
	}
}

// MUST NEVER DEFER: a tie is no evidence, and deferring both would
// abort neither.
func TestR133_TieDoesNotDefer(t *testing.T) {
	resetInflight(t)
	self, releaseSelf := registerInflightGame()
	defer releaseSelf()
	peer, releasePeer := registerInflightGame()
	defer releasePeer()

	w := gameWeight(r133Game(10, 10, 2))
	self.Store(w)
	peer.Store(w)

	if shouldDeferAbort(heapAbortDelta, self, w) {
		t.Fatal("equal weights are no evidence; deferring both would abort neither")
	}
}

// A single in-flight game has no peer and must abort on its own trip.
func TestR133_SoleGameHasNoPeerToDeferTo(t *testing.T) {
	resetInflight(t)
	self, release := registerInflightGame()
	defer release()
	w := gameWeight(r133Game(3, 3, 1))
	self.Store(w)

	if shouldDeferAbort(heapAbortDelta, self, w) {
		t.Fatal("with no peers there is nobody to defer to")
	}
}

// A finished game must not keep haunting the registry as a phantom
// suspect — that would spare every future game forever.
func TestR133_ReleasedSlotStopsCounting(t *testing.T) {
	resetInflight(t)
	self, releaseSelf := registerInflightGame()
	defer releaseSelf()
	peer, releasePeer := registerInflightGame()
	peer.Store(999999)

	selfWeight := int64(5)
	self.Store(selfWeight)
	if !shouldDeferAbort(heapAbortDelta, self, selfWeight) {
		t.Fatal("precondition: the live huge peer should cause a deferral")
	}
	releasePeer()
	if shouldDeferAbort(heapAbortDelta, self, selfWeight) {
		t.Fatal("a released slot must stop counting as an in-flight suspect")
	}
}

// Concurrent registration must not race or lose slots — two workers do
// this for every game they play. Run with -race.
func TestR133_RegistryIsConcurrencySafe(t *testing.T) {
	resetInflight(t)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			slot, release := registerInflightGame()
			defer release()
			slot.Store(int64(n))
			_ = heaviestPeerWeight(slot)
			_ = shouldDeferAbort(heapAbortDelta, slot, int64(n))
		}(i)
	}
	wg.Wait()
	if n := inflightCount(); n != 0 {
		t.Fatalf("every slot must be released; %d left behind", n)
	}
}

func resetInflight(t *testing.T) {
	t.Helper()
	inflightMu.Lock()
	inflightWeights = map[*atomic.Int64]struct{}{}
	inflightMu.Unlock()
}

func inflightCount() int {
	inflightMu.Lock()
	defer inflightMu.Unlock()
	return len(inflightWeights)
}
