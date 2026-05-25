package gameengine

import (
	"math/rand"
	"testing"
)

// Loki r60 extreme-stress / seed 99 game 9804 turn 42 surfaced:
//
//   ResourceConservation: seat 0 is Lost but has ManaPool=1
//
// Event trail:
//   [1872] sba_704_5a seat=0                       (life=0 → s.Lost=true)
//   [1873] destroy Myr Moonvessel
//   [1874] sba_704_5g seat=0 source=Myr Moonvessel (lethal damage)
//   [1875] zone_change Myr Moonvessel → graveyard
//   [1876] triggered_ability seat=0 src=Myr Moonvessel
//          ("when this dies, add {1} to your mana pool")
//   [1877] seat_eliminated seat=0                  (HandleSeatElimination)
//   [1878] stack_push seat=0 src=Myr Moonvessel
//   [1879..1882] priority + resolve
//   [1883] add_mana seat=0 src=Myr Moonvessel amount=1  ← THE BUG
//
// Myr Moonvessel's death trigger fired at [1876] WHILE seat 0 was being
// eliminated. The trigger landed in `gs.pendingTriggers` (CR §603.3b
// batch). HandleSeatElimination at [1877] purged `gs.Stack` items
// controlled by seat 0 — but the trigger wasn't on the stack yet, it
// was in the pending batch. After elimination, the batch flushed, the
// trigger pushed + resolved, and `add_mana` put seat 0 back to
// ManaPool=1 even though it had no mana per CR §106.4.
//
// CR §800.4a: when a player leaves the game, abilities of that player
// cease to exist. That includes pending triggers that haven't reached
// the stack yet.
//
// Fix: HandleSeatElimination now ALSO purges `gs.pendingTriggers` for
// items where Controller == seatIdx, mirroring the existing
// `gs.Stack` purge.

// TestSeatElimination_PurgesPendingTriggersFromLeavingSeat pins the
// bit-stable signature: a triggered ability sourced from seat 0 is
// queued in pendingTriggers; seat 0 is eliminated; the trigger must
// be removed (not flushed to stack + resolved).
func TestSeatElimination_PurgesPendingTriggersFromLeavingSeat(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	gs := NewGameState(4, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// A pending trigger sourced from seat 0 (e.g. Myr Moonvessel's
	// "when this dies, add {1} to your mana pool").
	myr := &Card{Name: "Myr Moonvessel", Owner: 0, Types: []string{"artifact", "creature"}}
	myrPerm := &Permanent{
		Card: myr, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.pendingTriggers = append(gs.pendingTriggers, &StackItem{
		Controller: 0,
		Source:     myrPerm,
		Card:       myr,
		Kind:       "triggered",
	})

	// Also a pending trigger from a surviving seat — must NOT be purged.
	sibling := &Card{Name: "Other Trigger", Owner: 1, Types: []string{"creature"}}
	siblingPerm := &Permanent{
		Card: sibling, Controller: 1, Owner: 1,
		Timestamp: gs.NextTimestamp(),
	}
	gs.pendingTriggers = append(gs.pendingTriggers, &StackItem{
		Controller: 1,
		Source:     siblingPerm,
		Card:       sibling,
		Kind:       "triggered",
	})

	// Eliminate seat 0.
	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	// Seat 0's pending trigger must be gone.
	for _, item := range gs.pendingTriggers {
		if item != nil && item.Controller == 0 {
			t.Fatalf("REGRESSION: seat 0's pending trigger survived elimination (Source=%v)", item.Source)
		}
	}
	// Seat 1's pending trigger must remain.
	survivingCount := 0
	for _, item := range gs.pendingTriggers {
		if item != nil && item.Controller == 1 {
			survivingCount++
		}
	}
	if survivingCount != 1 {
		t.Fatalf("seat 1's pending trigger must survive seat 0 elimination; got %d remaining", survivingCount)
	}
}

// TestSeatElimination_PendingTriggerPurgeFiresEvent confirms the purge
// emits a `pending_triggers_purged_on_leave` event for auditors.
func TestSeatElimination_PendingTriggerPurgeFiresEvent(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	myr := &Card{Name: "Myr Moonvessel", Owner: 0, Types: []string{"artifact", "creature"}}
	myrPerm := &Permanent{
		Card: myr, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp(),
	}
	gs.pendingTriggers = append(gs.pendingTriggers, &StackItem{
		Controller: 0, Source: myrPerm, Card: myr, Kind: "triggered",
	})

	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "pending_triggers_purged_on_leave" {
			found = true
			if ev.Seat != 0 {
				t.Errorf("event Seat should be 0; got %d", ev.Seat)
			}
			if ev.Amount != 1 {
				t.Errorf("event Amount should be 1 (one purged trigger); got %d", ev.Amount)
			}
		}
	}
	if !found {
		t.Fatal("expected pending_triggers_purged_on_leave event after elimination")
	}
}

// TestSeatElimination_NoEventWhenNoPendingTriggers confirms the new
// purge step is a clean no-op when there are no pending triggers from
// the leaving seat (defends against emitting a spurious purge event).
func TestSeatElimination_NoEventWhenNoPendingTriggers(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	for _, ev := range gs.EventLog {
		if ev.Kind == "pending_triggers_purged_on_leave" {
			t.Fatalf("must not emit purge event when there are no pending triggers from leaving seat")
		}
	}
}

// TestSeatElimination_MyrMoonvesselDoesNotAddManaToEliminatedSeat is
// the end-to-end pin against the bit-stable seed-99 game-9804
// signature. The seat starts with ManaPool=0 (post-mana-clear during
// elimination), a pending Myr-Moonvessel-style trigger is purged, and
// the seat's ManaPool stays at 0.
func TestSeatElimination_MyrMoonvesselDoesNotAddManaToEliminatedSeat(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	gs := NewGameState(2, rng, nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}

	// Seat 0 starts at 0 life and is about to be eliminated.
	gs.Seats[0].Life = 0
	gs.Seats[0].ManaPool = 0

	// Myr Moonvessel just died and its trigger is queued.
	myr := &Card{Name: "Myr Moonvessel", Owner: 0, Types: []string{"artifact", "creature"}}
	myrPerm := &Permanent{Card: myr, Controller: 0, Owner: 0, Timestamp: gs.NextTimestamp()}
	gs.pendingTriggers = append(gs.pendingTriggers, &StackItem{
		Controller: 0, Source: myrPerm, Card: myr, Kind: "triggered",
	})

	gs.Seats[0].Lost = true
	HandleSeatElimination(gs, 0)

	// Post-elimination: Lost seat must have ManaPool=0. The trigger
	// cannot resolve because we purged it; no add_mana fires.
	if gs.Seats[0].ManaPool != 0 {
		t.Fatalf("eliminated seat 0 must have ManaPool=0 (CR §106.4); got %d", gs.Seats[0].ManaPool)
	}
	// No pending trigger from seat 0 remains to flush.
	for _, item := range gs.pendingTriggers {
		if item != nil && item.Controller == 0 {
			t.Fatal("REGRESSION: seat 0's pending Myr Moonvessel trigger must be purged")
		}
	}
}
