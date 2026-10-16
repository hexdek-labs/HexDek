package gameengine

import (
	"math/rand"
	"sync"
	"testing"
)

// r146 — these are instrumentation tests. The counter changes no
// decision anywhere in the engine, so there is no semantics to revert:
// none of them can be made to fail by removing a behaviour, only by
// removing the counter itself. That is stated here rather than left for
// a reader to discover, because "all green" on a pure-instrumentation
// round means less than it does on a behavioural one (r131/r142).

func r146Game(t *testing.T) *GameState {
	t.Helper()
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "CardA", "CardB")
	addHand(gs, 1, "CardC")
	addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	return gs
}

func TestR146_CloneIsCounted(t *testing.T) {
	gs := r146Game(t)
	before := RolloutClones()
	if gs.CloneForRollout(rand.New(rand.NewSource(1))) == nil {
		t.Fatal("precondition: fixture should clone")
	}
	if d := RolloutClones() - before; d != 1 {
		t.Fatalf("one clone should move the counter by 1, got %d", d)
	}
}

func TestR146_CounterCountsEveryClone(t *testing.T) {
	gs := r146Game(t)
	before := RolloutClones()
	for i := 0; i < 7; i++ {
		gs.CloneForRollout(rand.New(rand.NewSource(int64(i))))
	}
	if d := RolloutClones() - before; d != 7 {
		t.Fatalf("seven clones should move the counter by 7, got %d", d)
	}
}

// The design choice under control: the increment sits AFTER the nil
// guard, inside CloneForRollout, rather than at the call sites. If it
// were moved above the guard — or to the callers, which do not all
// check for nil first — a nil state would inflate the census. Moving
// the Add above `if gs == nil` turns this test red.
func TestR146_NilCloneIsNotCounted(t *testing.T) {
	var gs *GameState
	before := RolloutClones()
	if gs.CloneForRollout(nil) != nil {
		t.Fatal("precondition: nil clones to nil")
	}
	if d := RolloutClones() - before; d != 0 {
		t.Fatalf("a nil clone produced no state and must not be counted, got %d", d)
	}
}

// Publish, don't inspect (r133/r142). Run under -race.
func TestR146_CounterIsRaceFree(t *testing.T) {
	gs := r146Game(t)
	before := RolloutClones()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				gs.CloneForRollout(rand.New(rand.NewSource(int64(w*100 + i))))
			}
		}(w)
	}
	wg.Wait()
	if d := RolloutClones() - before; d != 80 {
		t.Fatalf("80 concurrent clones should move the counter by 80, got %d", d)
	}
}
