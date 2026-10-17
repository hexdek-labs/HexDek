package gameengine

// r159 — the n*n dependency matrix in DependencyOrder, reused instead of
// reallocated.
//
// r152 made this matrix LAZY: nothing is allocated unless a real CR
// §613.8 dependency exists, which removed the common case entirely
// (dependencies are rare, and the old code built n separate slices and
// then threw them away). What r152 did NOT address, and said so in its
// own comment, is the pathological case: a board with real dependencies
// and many continuous effects still allocates n*n bools on EVERY call.
//
// It showed up again on 2026-10-17. A spike capture on the r158 binary:
//
//	ALLOC SPIKE 28.4 MB/game vs median 14.6 (1.94x)
//	  alloc 3064MB vs 3068MB prev (-0%), games 108 vs 169 prev (-36%)
//	  DENOMINATOR FELL
//	layers.go:607  depends = make([]bool, n*n)   1.07GB
//
// 1.07GB from one line in one 30-second window — 40.38% of that
// window's entire differenced profile, against a steady-state share
// below 2%. The same window held a 109-second game. The slow game and
// the allocation are one phenomenon: the board grows effects, n*n grows
// quadratically, every call pays it, and the game crawls.
//
// RE-ENTRANCY IS REAL HERE, which is why this is a free list and not a
// single scratch field. effectDependsOn calls b.Predicate(gs, perm);
// a predicate typically asks gs.IsCreatureOf or gs.HasTypeOf, which
// reach GetEffectiveCharacteristics -> applyLayer -> DependencyOrder.
// A nested call must not be handed the buffer its caller is filling.
// Each level takes a buffer off the list and returns it on the way out,
// so nesting gets distinct buffers and the steady state still reuses.
//
// The matrix never escapes: DependencyOrder returns the reordered
// effects slice, never `depends`.

// depBufMaxCap bounds what the free list will retain. A board big
// enough to need more than this is not one we want to hold memory for
// between games; above it the buffer is dropped rather than pooled.
const depBufMaxCap = 1 << 16 // 65536 bools == n of 256

// acquireDepBuf returns a zeroed []bool of exactly size, reusing a
// pooled buffer when one is large enough. gs may be nil (tests call
// DependencyOrder directly), in which case it simply allocates.
func (gs *GameState) acquireDepBuf(size int) []bool {
	if gs == nil || size <= 0 {
		return make([]bool, size)
	}
	for i := len(gs.depOrderBufs) - 1; i >= 0; i-- {
		b := gs.depOrderBufs[i]
		if cap(b) >= size {
			gs.depOrderBufs = append(gs.depOrderBufs[:i], gs.depOrderBufs[i+1:]...)
			b = b[:size]
			clear(b)
			return b
		}
	}
	return make([]bool, size)
}

// releaseDepBuf returns a buffer to the free list. Buffers above
// depBufMaxCap are dropped so a single pathological board does not pin
// memory for the rest of the process.
func (gs *GameState) releaseDepBuf(b []bool) {
	if gs == nil || b == nil || cap(b) > depBufMaxCap {
		return
	}
	gs.depOrderBufs = append(gs.depOrderBufs, b[:0:cap(b)])
}
