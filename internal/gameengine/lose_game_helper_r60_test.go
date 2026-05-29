package gameengine

import "testing"

// TestMarkSeatLostByEffect_FiresWouldLoseGameReplacement pins that the
// canonical helper consults the §614 replacement chain. Without this
// guard the 8 per_card sites refactored on 2026-05-29 (Angel of Destiny,
// Atemsis, Demonic Pact, Etrata, Frodo Sauron's Bane, Pact cycle, Pact
// of Negation, Sanguine Exquisite) would silently bypass Platinum Angel.
func TestMarkSeatLostByEffect_FiresWouldLoseGameReplacement(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	// Drop a Platinum Angel on seat 0's battlefield + register its
	// would_lose_game cancel-handler.
	pa := &Permanent{
		Card:       &Card{Name: "Platinum Angel"},
		Controller: 0,
		Owner:      0,
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, pa)
	RegisterPlatinumAngel(gs, pa)

	applied := MarkSeatLostByEffect(gs, 0, "Test Source")
	if applied {
		t.Error("expected MarkSeatLostByEffect to return false when §614 cancels")
	}
	if gs.Seats[0].Lost {
		t.Error("seat 0 must not be Lost after Platinum Angel cancellation")
	}
	if gs.Seats[0].LossReason != "" {
		t.Errorf("LossReason must stay empty when cancelled, got %q", gs.Seats[0].LossReason)
	}
}

// TestMarkSeatLostByEffect_AppliesWhenNotCancelled pins the positive path
// — without a §614 replacement, the helper stamps LossReason +
// LostByEffect + Lost in order.
func TestMarkSeatLostByEffect_AppliesWhenNotCancelled(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	applied := MarkSeatLostByEffect(gs, 1, "Demonic Pact")
	if !applied {
		t.Fatal("expected MarkSeatLostByEffect to return true (no replacement registered)")
	}
	s := gs.Seats[1]
	if !s.Lost {
		t.Error("Lost flag not set")
	}
	if !s.LostByEffect {
		t.Error("LostByEffect flag not set")
	}
	if s.LossReason != "card_effect: Demonic Pact" {
		t.Errorf("LossReason mismatch: %q", s.LossReason)
	}
}

// TestMarkSeatLostByEffect_AlreadyLostNoOp guards re-entry — calling the
// helper on a seat already marked Lost must not fire the replacement
// chain a second time or restamp LossReason.
func TestMarkSeatLostByEffect_AlreadyLostNoOp(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	gs.Seats[0].Lost = true
	gs.Seats[0].LossReason = "preexisting_reason"
	applied := MarkSeatLostByEffect(gs, 0, "Should-Be-Ignored")
	if applied {
		t.Error("expected false for already-Lost seat")
	}
	if gs.Seats[0].LossReason != "preexisting_reason" {
		t.Errorf("LossReason got overwritten: %q", gs.Seats[0].LossReason)
	}
}

// TestMarkSeatLostByEffect_OutOfRangeNoCrash defensive guards.
func TestMarkSeatLostByEffect_OutOfRangeNoCrash(t *testing.T) {
	gs := NewGameState(2, nil, nil)
	if MarkSeatLostByEffect(gs, -1, "x") {
		t.Error("negative seat must return false")
	}
	if MarkSeatLostByEffect(gs, 99, "x") {
		t.Error("oob seat must return false")
	}
	if MarkSeatLostByEffect(nil, 0, "x") {
		t.Error("nil gs must return false")
	}
}
