package gameengine

import (
	"fmt"
	"math/rand"
	"testing"
)

// r171 — DependencyOrder short-circuits the O(n^2) effectDependsOn scan
// for any effect set that provably cannot contain a dependency.
//
// These tests pin BOTH directions: a real layer-6 dependency must still
// reorder (the guard must not swallow it), and a set with no possible
// dependency must pass through untouched.
//
// The positive direction is the one that matters. A guard that skips too
// much is a silent rules bug: CR 613.8 says a dependency OVERRIDES the
// timestamp system, so dropping one changes which continuous effect wins.

func r171Effect(layer int, ts int, seat int, pred func(*GameState, *Permanent) bool) *ContinuousEffect {
	src := &Permanent{
		Card:       &Card{Name: fmt.Sprintf("src-L%d-t%d", layer, ts)},
		Controller: seat, Owner: seat,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	return &ContinuousEffect{
		Layer: layer, Timestamp: ts, SourcePerm: src,
		ControllerSeat: seat, Predicate: pred,
	}
}

func r171Game() *GameState {
	return NewGameState(4, rand.New(rand.NewSource(17)), nil)
}

// A layer-6 dependency must still be detected and must still reorder the
// set, with the depended-upon effect moved ahead of its dependent.
func TestR171_LayerAbilityDependencyStillReorders(t *testing.T) {
	gs := r171Game()

	// a has the EARLIER timestamp, so timestamp order alone would put it
	// first. b strips abilities and its predicate matches a's source, so
	// per CR 613.8b a must wait until after b.
	a := r171Effect(LayerAbility, 1, 0, nil)
	b := r171Effect(LayerAbility, 2, 0, nil)
	b.Predicate = func(_ *GameState, p *Permanent) bool { return p == a.SourcePerm }

	in := []*ContinuousEffect{a, b}
	got := DependencyOrder(in, gs)

	if len(got) != 2 {
		t.Fatalf("expected 2 effects back, got %d", len(got))
	}
	if got[0] != b || got[1] != a {
		t.Fatalf("CR 613.8b: dependent effect must apply after the one it "+
			"depends on; want [b a], got [%s %s]",
			got[0].SourcePerm.Card.Name, got[1].SourcePerm.Card.Name)
	}
}

// The mirror of the above: with the dependency removed, the SAME two
// layer-6 effects must stay in timestamp order. This is what proves the
// reorder above came from the dependency and not from the guard.
func TestR171_LayerAbilityWithoutDependencyKeepsTimestampOrder(t *testing.T) {
	gs := r171Game()
	a := r171Effect(LayerAbility, 1, 0, nil)
	b := r171Effect(LayerAbility, 2, 0, nil)
	b.Predicate = func(_ *GameState, _ *Permanent) bool { return false }

	got := DependencyOrder([]*ContinuousEffect{a, b}, gs)
	if got[0] != a || got[1] != b {
		t.Fatalf("no dependency: want timestamp order [a b]")
	}
}

// Sets that cannot contain a dependency pass through unchanged. These are
// the twelve of thirteen applyLayer passes that were paying a full n^2
// scan to rediscover "nothing depends on anything".
func TestR171_NonDependencyLayersPassThroughUnchanged(t *testing.T) {
	gs := r171Game()
	for _, layer := range []int{LayerCopy, LayerControl, LayerText, LayerType, LayerColor, LayerPT} {
		layer := layer
		t.Run(fmt.Sprintf("L%d", layer), func(t *testing.T) {
			var in []*ContinuousEffect
			for i := 0; i < 6; i++ {
				// A predicate that matches EVERYTHING. If the guard were
				// unsound, a match here would reorder the set.
				e := r171Effect(layer, i, 0, func(*GameState, *Permanent) bool { return true })
				in = append(in, e)
			}
			got := DependencyOrder(in, gs)
			if len(got) != len(in) {
				t.Fatalf("length changed: %d -> %d", len(in), len(got))
			}
			for i := range in {
				if got[i] != in[i] {
					t.Fatalf("L%d: order changed at %d — a layer that cannot "+
						"have dependencies must pass through untouched", layer, i)
				}
			}
		})
	}
}

// The guard is written against the general effect set, not against
// applyLayer's single-layer invariant. A MIXED set containing the
// type-change shape must still take the full scan.
func TestR171_MixedLayerTypeSetStillTakesTheScan(t *testing.T) {
	gs := r171Game()
	// b at layer 4, a above layer 4 — effectDependsOn's third block.
	a := r171Effect(LayerPT, 1, 0, nil)
	b := r171Effect(LayerType, 2, 0, nil)
	b.Predicate = func(_ *GameState, p *Permanent) bool { return p == a.SourcePerm }

	got := DependencyOrder([]*ContinuousEffect{a, b}, gs)
	if got[0] != b || got[1] != a {
		t.Fatalf("mixed-layer type-change dependency must still be detected; "+
			"got [%s %s]", got[0].SourcePerm.Card.Name, got[1].SourcePerm.Card.Name)
	}
}

// Degenerate inputs must behave exactly as before the guard.
func TestR171_GuardHandlesNilAndTinyInputs(t *testing.T) {
	gs := r171Game()
	if got := DependencyOrder(nil, gs); len(got) != 0 {
		t.Fatalf("nil -> %d", len(got))
	}
	if got := DependencyOrder([]*ContinuousEffect{nil, nil}, gs); len(got) != 2 {
		t.Fatalf("nil effects -> %d", len(got))
	}
	one := r171Effect(LayerAbility, 1, 0, nil)
	if got := DependencyOrder([]*ContinuousEffect{one}, gs); len(got) != 1 || got[0] != one {
		t.Fatalf("single effect must pass through")
	}
}

// TRANSPORT TEST (r168's lesson). The pass-through tests above assert the
// same thing before and after r171, because a scan that finds nothing and
// a scan that never ran return the identical slice. This one tells them
// apart.
//
// Before r171, a pure layer-4 set reached effectDependsOn's first block,
// which called b.Predicate(gs, a.SourcePerm) inside a deferred recover and
// discarded the answer. That call is observable. After r171 the set never
// reaches the scan at all, so the predicate must never run.
//
// This is the assertion that goes red if the guard is removed, and it is
// the only one here that does.
func TestR171_PureLayerTypeSetNeverInvokesAPredicate(t *testing.T) {
	gs := r171Game()
	calls := 0
	var in []*ContinuousEffect
	for i := 0; i < 8; i++ {
		e := r171Effect(LayerType, i, 0, func(*GameState, *Permanent) bool {
			calls++
			return true
		})
		in = append(in, e)
	}
	got := DependencyOrder(in, gs)

	if calls != 0 {
		t.Fatalf("a pure layer-4 set cannot produce a dependency, so the "+
			"n^2 scan must be skipped entirely; predicate ran %d times", calls)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("order changed at %d", i)
		}
	}
}
