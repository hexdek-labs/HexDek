package gameengine

// R60 — Goldilocks residual: ResourceConservation on Lost-seat transition.
//
// Failure surfaced by `hexdek-thor -goldilocks` post-r60-engine-clean:
//
//   Abduction
//     goldilocks_invariant
//     [untap] ResourceConservation: seat 0 is Lost but has ManaPool=10
//
// Every `s.Lost = true` site in sba.go used to flip the flag without
// draining the mana pool. The §704.5e empty-mana-pool reaper only runs
// on phase change, so a seat losing mid-step would carry ManaPool=N
// until the next phase boundary — `checkResourceConservation`
// caught the residual.
//
// Fix: route all five sites through `markSeatLost(s, reason)` which
// flips Lost, stamps LossReason, zeroes ManaPool, and Clears Mana.
//
// These regressions pin each of the five loss paths individually plus
// the invariant itself.

import (
	"math/rand"
	"testing"
)

// fundedSeat sets the seat to a "has resources" state: ManaPool=10 with a
// matching typed pool. Mirrors the goldilocks scaffold front-loading from
// `cmd/hexdek-thor/advanced_mechanics.go` and `claim_verifier.go`.
func fundedSeat(t *testing.T, gs *GameState, seat int) {
	t.Helper()
	s := gs.Seats[seat]
	s.Mana = &ColoredManaPool{R: 4, U: 3, C: 3}
	s.ManaPool = s.Mana.Total()
	if s.ManaPool != 10 {
		t.Fatalf("fundedSeat setup expected ManaPool=10, got %d", s.ManaPool)
	}
}

func assertDrained(t *testing.T, gs *GameState, seat int, path string) {
	t.Helper()
	s := gs.Seats[seat]
	if !s.Lost {
		t.Fatalf("%s: expected Lost=true, got false", path)
	}
	if s.LossReason == "" {
		t.Fatalf("%s: expected LossReason stamped, got empty", path)
	}
	if s.ManaPool != 0 {
		t.Fatalf("%s: expected ManaPool drained to 0, got %d", path, s.ManaPool)
	}
	if s.Mana != nil && s.Mana.Total() != 0 {
		t.Fatalf("%s: expected typed Mana drained to 0, got total=%d", path, s.Mana.Total())
	}
	if err := checkResourceConservation(gs); err != nil {
		t.Fatalf("%s: ResourceConservation invariant still trips after loss: %v", path, err)
	}
}

// -----------------------------------------------------------------------------
// §704.5a — life ≤ 0.
// -----------------------------------------------------------------------------

func TestR60_LostDrainsMana_LifeLoss_704_5a(t *testing.T) {
	gs := newFixtureGame(t)
	fundedSeat(t, gs, 0)
	gs.Seats[0].Life = 0
	sba704_5a(gs)
	assertDrained(t, gs, 0, "§704.5a life-loss")
}

// -----------------------------------------------------------------------------
// §704.5b — drew from empty library.
// -----------------------------------------------------------------------------

func TestR60_LostDrainsMana_EmptyLibraryDraw_704_5b(t *testing.T) {
	gs := newFixtureGame(t)
	fundedSeat(t, gs, 0)
	gs.Seats[0].AttemptedEmptyDraw = true
	sba704_5b(gs)
	assertDrained(t, gs, 0, "§704.5b empty-library draw")
}

// -----------------------------------------------------------------------------
// §704.5c — 10+ poison counters.
// -----------------------------------------------------------------------------

func TestR60_LostDrainsMana_Poison_704_5c(t *testing.T) {
	gs := newFixtureGame(t)
	fundedSeat(t, gs, 0)
	gs.Seats[0].PoisonCounters = 10
	sba704_5c(gs)
	assertDrained(t, gs, 0, "§704.5c poison")
}

// -----------------------------------------------------------------------------
// §704.6c — 21+ commander damage from one source.
// -----------------------------------------------------------------------------

func TestR60_LostDrainsMana_CommanderDamage_704_6c(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(42)), nil)
	gs.CommanderFormat = true
	fundedSeat(t, gs, 1)
	gs.Seats[1].CommanderDamage = map[int]map[string]int{
		0: {"Kraum, Ludevic's Opus": 21},
	}
	sba704_6c(gs)
	assertDrained(t, gs, 1, "§704.6c commander damage")
}

// -----------------------------------------------------------------------------
// §104.4b — mandatory loop draw via SBA pass cap. The cap path lives at
// sba.go:189; rather than spinning the engine into the 40-pass safety
// trigger we exercise the helper directly through markSeatLost. This
// pins the contract the cap loop relies on.
// -----------------------------------------------------------------------------

func TestR60_LostDrainsMana_MandatoryLoopDraw_104_4b(t *testing.T) {
	gs := newFixtureGame(t)
	fundedSeat(t, gs, 0)
	fundedSeat(t, gs, 1)
	markSeatLost(gs.Seats[0], "mandatory loop draw (CR 104.4b via SBA cap)", nil)
	markSeatLost(gs.Seats[1], "mandatory loop draw (CR 104.4b via SBA cap)", nil)
	assertDrained(t, gs, 0, "§104.4b mandatory-loop draw seat 0")
	assertDrained(t, gs, 1, "§104.4b mandatory-loop draw seat 1")
}

// -----------------------------------------------------------------------------
// Helper directly — nil safety + nil-Mana branch.
// -----------------------------------------------------------------------------

func TestR60_MarkSeatLost_NilSafety(t *testing.T) {
	// Explicit doesn't-panic assertion via recover() — Phase 2B audit.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("markSeatLost(nil, ...) must not panic; got %v", r)
		}
	}()
	markSeatLost(nil, "should not panic", nil)
}

func TestR60_MarkSeatLost_NilTypedPool(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].ManaPool = 5
	gs.Seats[0].Mana = nil // explicit: lazy-materialized pool, never touched
	markSeatLost(gs.Seats[0], "test", nil)
	if !gs.Seats[0].Lost {
		t.Fatal("expected Lost=true")
	}
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("expected ManaPool drained, got %d", gs.Seats[0].ManaPool)
	}
}
