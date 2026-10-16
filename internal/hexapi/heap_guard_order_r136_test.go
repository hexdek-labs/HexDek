package hexapi

import (
	"sync/atomic"
	"testing"
)

// r136 — the guard was inverted in production, and the log says so.
//
//	05:17:55  grinder: ... heap=1404MB sys=2165MB
//	05:18:08  heap delta 2410MB at turn 60, but a peer game is larger — deferring
//	05:18:28  grinder: ... heap=12426MB sys=18154MB
//	05:18:38  aborting game at turn 10 — process heap 10424MB exceeds the
//	          6442MB ceiling ... perms=13 tokens=0
//
// 557 aborts fired in 33 seconds and EVERY ONE was at turn 10 holding
// 13-16 permanents and no tokens — newly started games, mown down on
// sight — while the games actually carrying 2.4-3.7GB of delta printed
// "deferring the abort" and ran on.
//
// Two defects compose:
//
//  1. heapGuardEvaluate tested `delta` before `heapAlloc`, so a process
//     at 12GB still returned heapAbortDelta. heapAbortDelta is the
//     DEFERRABLE one, so the ceiling — documented as "NEVER deferred"
//     because it is the attribution-free backstop — was deferred.
//     heapAbortCeiling could then only fire for a game whose baseline
//     was captured after the spike, i.e. an innocent new game.
//  2. a game could defer forever without the heap ever coming down.

const r136GB = 1024 * 1024 * 1024

// CATCHES BUG 1. Pristine heapGuardEvaluate returns "delta" here.
func TestR136_CeilingBeatsDeltaSoTheBackstopIsNeverDeferrable(t *testing.T) {
	// The real reading from 05:18:28: 12426MB live heap, a game that
	// had been running long enough to own a large delta of its own.
	_, _, abort := heapGuardEvaluate(12426*1024*1024, 0)
	if abort != heapAbortCeiling {
		t.Fatalf("a 12426MB process heap must abort on the CEILING, got %q — "+
			"heapAbortDelta is deferrable and this is twice the ceiling", abort)
	}

	// And the ceiling verdict must be refused by the deferral gate even
	// when a peer is visibly larger, which is the invariant that failed.
	resetInflight(t)
	self, releaseSelf := registerInflightGame()
	defer releaseSelf()
	peer, releasePeer := registerInflightGame()
	defer releasePeer()
	self.Store(10)
	peer.Store(999999)
	if shouldDeferAbort(abort, self, 10) {
		t.Fatal("the process ceiling is the attribution-free backstop and must never defer")
	}
}

// MUST NEVER REGRESS: under the ceiling, the per-game budget still owns
// the verdict, so r129's delta guard keeps working.
func TestR136_DeltaStillOwnsTheVerdictBelowTheCeiling(t *testing.T) {
	_, watch, abort := heapGuardEvaluate(3*r136GB, 500*1024*1024)
	if abort != heapAbortDelta {
		t.Fatalf("2.5GB of delta under a 6GB process heap is the per-game budget, got %q", abort)
	}
	if !watch {
		t.Fatal("over budget must keep sampling escalated (r129)")
	}
	if _, _, a := heapGuardEvaluate(300*1024*1024, 0); a != "" {
		t.Fatalf("a small game must still abort on nothing, got %q", a)
	}
}

// CATCHES BUG 2. A deferral is a bet that the peer will relieve the
// heap. If the heap has not come down by the next check, the bet lost.
func TestR136_ADeferralThatDidNotRelieveTheHeapIsNotRepeated(t *testing.T) {
	resetInflight(t)
	self, releaseSelf := registerInflightGame()
	defer releaseSelf()
	peer, releasePeer := registerInflightGame()
	defer releasePeer()
	self.Store(400)
	peer.Store(402) // the live shape: near-identical weights, peer a hair larger

	var memo deferralMemo
	if !memo.allow(heapAbortDelta, self, 400, 2400*1024*1024) {
		t.Fatal("precondition: the first deferral to a larger peer is legitimate")
	}
	// 05:18:08 -> 05:18:09: the heap went UP while this game waited.
	if memo.allow(heapAbortDelta, self, 400, 2410*1024*1024) {
		t.Fatal("the heap rose after deferring, so the peer did not relieve it — " +
			"this game must stop standing down")
	}
}

// MUST NEVER REGRESS: when deferring DOES work, politeness continues —
// otherwise r133's whole point (sparing the small game) is undone.
func TestR136_DeferralContinuesWhileItIsActuallyWorking(t *testing.T) {
	resetInflight(t)
	self, releaseSelf := registerInflightGame()
	defer releaseSelf()
	peer, releasePeer := registerInflightGame()
	defer releasePeer()
	self.Store(5)
	peer.Store(999999)

	var memo deferralMemo
	heap := uint64(5 * r136GB)
	for i := 0; i < 4; i++ {
		if !memo.allow(heapAbortDelta, self, 5, heap) {
			t.Fatalf("deferral %d: the heap is falling and the peer is huge; "+
				"the small game must still be spared", i)
		}
		heap -= 512 * 1024 * 1024
	}
}

// A non-delta verdict never consults the memo at all.
func TestR136_MemoOnlyGatesTheDeferrableVerdict(t *testing.T) {
	resetInflight(t)
	self, release := registerInflightGame()
	defer release()
	peer, releasePeer := registerInflightGame()
	defer releasePeer()
	self.Store(1)
	peer.Store(999999)

	var memo deferralMemo
	for _, verdict := range []string{"", heapAbortCeiling} {
		if memo.allow(verdict, self, 1, 9*r136GB) {
			t.Fatalf("verdict %q must never be deferred", verdict)
		}
	}
	var unused atomic.Int64
	_ = unused
}
