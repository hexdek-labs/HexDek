package gameengine

import (
	"sort"
	"testing"
)

// r153Perm is a plain body to hand to applyLayer.
func r153Perm() *Permanent {
	return &Permanent{Card: &Card{Name: "Body"}, Flags: map[string]int{}}
}

// r153Effects registers n effects at (layer, sublayer). They are REAL
// registrations — gs.ContinuousEffects is non-empty — so applyLayer's
// len==0 guard at the top does not short-circuit the test.
func r153Effects(gs *GameState, n, layer int, sublayer string, apply func(*GameState, *Permanent, *Characteristics)) {
	for i := 0; i < n; i++ {
		gs.ContinuousEffects = append(gs.ContinuousEffects, &ContinuousEffect{
			Layer:      layer,
			Sublayer:   sublayer,
			Timestamp:  i + 1,
			SourcePerm: &Permanent{Card: &Card{Name: "Src"}, Flags: map[string]int{}},
			ApplyFn:    apply,
		})
	}
}

// THE HEADLINE. A layer with no matching effect must not allocate.
// GetEffectiveCharacteristics makes thirteen applyLayer calls per
// computation and a board's effects sit in one or two buckets, so this is
// the overwhelmingly common call.
func TestApplyLayerR153_NonMatchingLayerAllocatesNothing(t *testing.T) {
	gs := newFixtureGame(t)
	perm := r153Perm()
	// Effects exist, but at a DIFFERENT layer than the one queried.
	r153Effects(gs, 12, LayerColor, "", func(*GameState, *Permanent, *Characteristics) {})

	chars := &Characteristics{}
	got := testing.AllocsPerRun(50, func() {
		applyLayer(gs, perm, chars, LayerPT, "c")
	})
	if got != 0 {
		t.Fatalf("non-matching layer allocated %v times, want 0", got)
	}
}

// CONTROL — the pre-r153 applyLayer, copied faithfully INCLUDING the sort
// and DependencyOrder calls.
//
// The first attempt at this control allocated 0 and did not go red. That
// was not evidence the fix is pointless: a trimmed helper that merely
// returns the slice lets escape analysis keep it on the stack, because the
// caller discards it. In the real applyLayer the slice is handed to
// sort.SliceStable and DependencyOrder, so it escapes and is heap
// allocated at the make site — on EVERY call, including the ones that
// match nothing. A control has to reproduce the escape or it is measuring
// a different program.
func r153EagerApplyLayer(gs *GameState, perm *Permanent, chars *Characteristics, layer int, sublayer string) {
	if len(gs.ContinuousEffects) == 0 {
		return
	}
	candidates := make([]*ContinuousEffect, 0, 4)
	for _, ce := range gs.ContinuousEffects {
		if ce == nil {
			continue
		}
		if ce.SourcePerm != nil && ce.SourcePerm.PhasedOut {
			continue
		}
		if ce.Layer != layer {
			continue
		}
		if ce.Sublayer != sublayer {
			continue
		}
		candidates = append(candidates, ce)
	}
	if len(candidates) == 0 {
		return
	}
	active := gs.Active
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		aActive := a.ControllerSeat == active
		bActive := b.ControllerSeat == active
		if aActive != bActive {
			return aActive
		}
		return a.Timestamp < b.Timestamp
	})
	candidates = DependencyOrder(candidates, gs)
	for _, ce := range candidates {
		if ce.ApplyFn != nil {
			ce.ApplyFn(gs, perm, chars)
		}
	}
}

func TestApplyLayerR153_ControlEagerAllocationDoesAllocate(t *testing.T) {
	gs := newFixtureGame(t)
	perm := r153Perm()
	r153Effects(gs, 12, LayerColor, "", func(*GameState, *Permanent, *Characteristics) {})

	chars := &Characteristics{}
	got := testing.AllocsPerRun(50, func() {
		r153EagerApplyLayer(gs, perm, chars, LayerPT, "c")
	})
	if got < 1 {
		t.Fatalf("CONTROL DID NOT GO RED: the eager version allocated %v times on a non-matching layer, so the zero-alloc assertion above is not measuring it", got)
	}
	t.Logf("control allocates %v time(s) per NON-MATCHING call; GetEffectiveCharacteristics makes 13 applyLayer calls per computation", got)
}

// The other direction — a MATCHING layer must still apply its effects, in
// timestamp order, with nothing dropped.
func TestApplyLayerR153_MatchingLayerStillApplies(t *testing.T) {
	gs := newFixtureGame(t)
	perm := r153Perm()

	var order []int
	for i := 1; i <= 3; i++ {
		n := i
		gs.ContinuousEffects = append(gs.ContinuousEffects, &ContinuousEffect{
			Layer: LayerPT, Sublayer: "c", Timestamp: n,
			SourcePerm: &Permanent{Card: &Card{Name: "Src"}, Flags: map[string]int{}},
			ApplyFn: func(*GameState, *Permanent, *Characteristics) {
				order = append(order, n)
			},
		})
	}
	// A decoy at another layer that must NOT run.
	r153Effects(gs, 1, LayerColor, "", func(*GameState, *Permanent, *Characteristics) {
		order = append(order, 99)
	})

	applyLayer(gs, perm, &Characteristics{}, LayerPT, "c")

	if len(order) != 3 {
		t.Fatalf("want 3 effects applied, got %d (%v)", len(order), order)
	}
	for i, want := range []int{1, 2, 3} {
		if order[i] != want {
			t.Fatalf("timestamp order broken: got %v", order)
		}
	}
}

// A predicate that returns false must still gate application — the lazy
// allocation must not change which effects run.
func TestApplyLayerR153_PredicateStillGates(t *testing.T) {
	gs := newFixtureGame(t)
	perm := r153Perm()

	ran := 0
	gs.ContinuousEffects = append(gs.ContinuousEffects,
		&ContinuousEffect{
			Layer: LayerPT, Sublayer: "c", Timestamp: 1,
			SourcePerm: &Permanent{Card: &Card{Name: "No"}, Flags: map[string]int{}},
			Predicate:  func(*GameState, *Permanent) bool { return false },
			ApplyFn:    func(*GameState, *Permanent, *Characteristics) { ran++ },
		},
		&ContinuousEffect{
			Layer: LayerPT, Sublayer: "c", Timestamp: 2,
			SourcePerm: &Permanent{Card: &Card{Name: "Yes"}, Flags: map[string]int{}},
			Predicate:  func(*GameState, *Permanent) bool { return true },
			ApplyFn:    func(*GameState, *Permanent, *Characteristics) { ran++ },
		},
	)

	applyLayer(gs, perm, &Characteristics{}, LayerPT, "c")
	if ran != 1 {
		t.Fatalf("predicate gating broken: want 1 effect applied, got %d", ran)
	}
}

// A phased-out source is skipped per CR 702.26a, and skipping it must not
// leave an allocated-but-empty candidate slice behind either.
func TestApplyLayerR153_PhasedOutSourceSkippedAndAllocatesNothing(t *testing.T) {
	gs := newFixtureGame(t)
	perm := r153Perm()

	src := &Permanent{Card: &Card{Name: "Phased"}, Flags: map[string]int{}}
	src.PhasedOut = true
	ran := 0
	gs.ContinuousEffects = append(gs.ContinuousEffects, &ContinuousEffect{
		Layer: LayerPT, Sublayer: "c", Timestamp: 1,
		SourcePerm: src,
		ApplyFn:    func(*GameState, *Permanent, *Characteristics) { ran++ },
	})

	chars := &Characteristics{}
	got := testing.AllocsPerRun(50, func() {
		applyLayer(gs, perm, chars, LayerPT, "c")
	})
	if ran != 0 {
		t.Fatalf("phased-out source applied %d times, want 0", ran)
	}
	if got != 0 {
		t.Fatalf("all-phased-out layer allocated %v times, want 0", got)
	}
}
