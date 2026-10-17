package gameengine

import (
	"runtime"
	"testing"
	"unsafe"
)

// r159 — the n*n dependency matrix is borrowed from a per-GameState free
// list instead of allocated per call. Reuses r152's fixtures
// (r152Effects, r152Dependent) rather than inventing new ones.

// r159Dependent builds a dependency set of n effects where one real
// §613.8 dependency exists, so the matrix IS materialised — the case
// r152 deliberately left allocating.
func r159Dependent(n int) []*ContinuousEffect {
	a, b := r152Dependent()
	out := make([]*ContinuousEffect, 0, n)
	out = append(out, a, b)
	for i := 2; i < n; i++ {
		out = append(out, &ContinuousEffect{
			Layer:      LayerColor,
			Timestamp:  i + 1,
			SourcePerm: &Permanent{Card: &Card{Name: "pad"}, Flags: map[string]int{}},
		})
	}
	return out
}

// r159Bytes measures bytes allocated per call, because this round is
// about an n*n term and AllocsPerRun counts allocations, not size.
// My first version of the headline asserted ZERO allocations and was
// simply wrong: DependencyOrder also builds `visited` (8n) and `order`
// (8n), which this round does not touch. The matrix is the quadratic
// one and the only one worth pooling.
func r159Bytes(f func()) uint64 {
	for i := 0; i < 5; i++ {
		f() // warm the free list
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	const N = 200
	for i := 0; i < N; i++ {
		f()
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / N
}

// THE HEADLINE. With a real dependency the matrix must be materialised,
// but after the first call it must be REUSED, not reallocated — so the
// bytes per call must fall by roughly the size of the matrix.
//
// Stated against the control rather than a bare threshold, so the test
// says the number it is preventing.
func TestR159_DependencyMatrixIsReusedNotReallocated(t *testing.T) {
	const n = 24
	gsPooled := newFixtureGame(t)
	gsEager := newFixtureGame(t)
	effects := r159Dependent(n)
	eager := r159Dependent(n)

	pooled := r159Bytes(func() { DependencyOrder(effects, gsPooled) })
	// Control: nil GameState takes the un-pooled path through
	// acquireDepBuf, which is exactly the pre-r159 behaviour.
	_ = gsEager
	unpooled := r159Bytes(func() { DependencyOrder(eager, nil) })

	saved := int(unpooled) - int(pooled)
	t.Logf("bytes/call: pooled=%d unpooled=%d saved=%d (matrix is %d bytes)",
		pooled, unpooled, saved, n*n)

	if saved < n*n/2 {
		t.Errorf("pooling saved only %d bytes/call; the %d-byte matrix is "+
			"evidently still being allocated", saved, n*n)
	}
	if pooled >= unpooled {
		t.Errorf("pooled path (%d B/call) is not cheaper than unpooled (%d B/call)",
			pooled, unpooled)
	}
}

// The allocation COUNT must drop by exactly one — the matrix — and not
// by more, which would mean something else stopped being built.
func TestR159_ExactlyOneAllocationRemoved(t *testing.T) {
	gs := newFixtureGame(t)
	effects := r159Dependent(24)
	nilEffects := r159Dependent(24)

	DependencyOrder(effects, gs) // warm

	pooled := testing.AllocsPerRun(50, func() { DependencyOrder(effects, gs) })
	unpooled := testing.AllocsPerRun(50, func() { DependencyOrder(nilEffects, nil) })

	t.Logf("allocs/call: pooled=%v unpooled=%v", pooled, unpooled)
	if unpooled-pooled != 1 {
		t.Errorf("want exactly 1 allocation removed, got %v (pooled=%v unpooled=%v)",
			unpooled-pooled, pooled, unpooled)
	}
}

// CONTROL for the headline: prove the matrix is genuinely being built,
// so the zero above is reuse and not the no-dependency fast path
// silently taking over. If no dependency fires, this round tests
// nothing — the r152 trap of a vacuous pass.
func TestR159_PreconditionTheMatrixIsActuallyBuilt(t *testing.T) {
	gs := newFixtureGame(t)
	effects := r159Dependent(24)

	before := len(gs.depOrderBufs)
	DependencyOrder(effects, gs)
	after := len(gs.depOrderBufs)

	if after != before+1 {
		t.Fatalf("free list went %d -> %d; the matrix was never built, so the "+
			"allocation test above is vacuous", before, after)
	}
	if n := len(gs.depOrderBufs[after-1]); n != 0 {
		t.Errorf("released buffer should be length-0 for reuse, got %d", n)
	}
	if c := cap(gs.depOrderBufs[after-1]); c < 24*24 {
		t.Errorf("released buffer cap %d, want >= %d", c, 24*24)
	}
}

// The ordering result must be unchanged. b strips abilities and a's
// source matches its predicate, so b must come out before a.
func TestR159_DependencyOrderingStillCorrect(t *testing.T) {
	gs := newFixtureGame(t)
	a, b := r152Dependent()

	for _, in := range [][]*ContinuousEffect{{a, b}, {b, a}} {
		got := DependencyOrder(in, gs)
		if len(got) != 2 {
			t.Fatalf("got %d effects, want 2", len(got))
		}
		if got[0] != b || got[1] != a {
			t.Errorf("want [b a], got order b-first=%v", got[0] == b)
		}
	}
}

// RE-ENTRANCY — the reason this is a free list and not one scratch
// field. effectDependsOn calls b.Predicate(gs, perm), and a predicate
// that asks gs.IsCreatureOf reaches GetEffectiveCharacteristics ->
// applyLayer -> DependencyOrder. A nested call must never be handed the
// buffer its caller is still filling.
//
// THIS TEST ASSERTS THE INVARIANT, NOT A SYMPTOM, AND THAT IS THE
// SECOND VERSION. The first tried to show a corrupted ORDERING from a
// nested call, and a control that deliberately broke pooling stayed
// green through three attempts: staging a real clobber depends on what
// is in the pool at that instant, which depends on nesting history,
// which I could not make deterministic — and a probe I added to watch
// it was itself allocating and releasing buffers, contaminating the
// thing it measured.
//
// The property that actually makes this safe is simpler and directly
// checkable: WHILE A BUFFER IS IN USE IT IS NOT IN THE FREE LIST.
// Nothing can hand it out twice if it isn't there to hand out.
func TestR159_InUseBufferIsNotInTheFreeList(t *testing.T) {
	gs := newFixtureGame(t)

	// Warm the list so there IS something to wrongly hand out.
	DependencyOrder(r159Dependent(8), gs)
	if len(gs.depOrderBufs) == 0 {
		t.Fatalf("precondition: free list empty, nothing to mis-share")
	}
	warm := unsafe.SliceData(gs.depOrderBufs[0])

	pa := &Permanent{Card: &Card{Name: "A"}, Flags: map[string]int{}}
	pb := &Permanent{Card: &Card{Name: "B"}, Flags: map[string]int{}}
	pc := &Permanent{Card: &Card{Name: "C"}, Flags: map[string]int{}}

	checked := false
	a := &ContinuousEffect{Layer: LayerColor, Timestamp: 1, SourcePerm: pa}
	b := &ContinuousEffect{
		Layer: LayerAbility, Timestamp: 2, SourcePerm: pb,
		Predicate: func(gs *GameState, p *Permanent) bool { return p == pa },
	}
	c := &ContinuousEffect{
		Layer: LayerAbility, Timestamp: 3, SourcePerm: pc,
		Predicate: func(gs *GameState, p *Permanent) bool {
			// We are inside the outer scan; its matrix is live.
			checked = true
			for i, buf := range gs.depOrderBufs {
				if unsafe.SliceData(buf) == warm {
					t.Errorf("the buffer the outer call is filling is still "+
						"in the free list at index %d — a nested call would "+
						"be handed it and clobber the matrix", i)
				}
			}
			return false
		},
	}

	DependencyOrder([]*ContinuousEffect{a, b, c}, gs)
	if !checked {
		t.Fatalf("precondition: the predicate never fired, test proves nothing")
	}
}

// A nil GameState must still work — tests and legacy paths call
// DependencyOrder directly.
func TestR159_NilGameStateStillOrders(t *testing.T) {
	a, b := r152Dependent()
	got := DependencyOrder([]*ContinuousEffect{a, b}, nil)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
}

// An oversized matrix must not be pooled — one pathological board
// should not pin memory for the life of the process.
func TestR159_OversizedBufferIsNotPooled(t *testing.T) {
	gs := newFixtureGame(t)
	big := make([]bool, depBufMaxCap+1)

	gs.releaseDepBuf(big)
	if len(gs.depOrderBufs) != 0 {
		t.Errorf("oversized buffer was pooled (cap %d > max %d)", cap(big), depBufMaxCap)
	}

	ok := make([]bool, 16)
	gs.releaseDepBuf(ok)
	if len(gs.depOrderBufs) != 1 {
		t.Errorf("in-range buffer was not pooled")
	}
}

// A reused buffer must come back ZEROED, or a stale true from a
// previous call invents a dependency that does not exist.
func TestR159_ReusedBufferIsZeroed(t *testing.T) {
	gs := newFixtureGame(t)

	dirty := make([]bool, 64)
	for i := range dirty {
		dirty[i] = true
	}
	gs.releaseDepBuf(dirty)

	got := gs.acquireDepBuf(64)
	for i, v := range got {
		if v {
			t.Fatalf("reused buffer carried a stale true at index %d", i)
		}
	}
}
