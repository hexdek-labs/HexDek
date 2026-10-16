package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func r148Perm(name string) *gameengine.Permanent {
	return &gameengine.Permanent{
		Card:       &gameengine.Card{Name: name, Owner: 0, Types: []string{"artifact"}},
		Controller: 0, Owner: 0,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
}

func TestR148_OnAnySeatBattlefield(t *testing.T) {
	gs := newTestGS(2)
	here := r148Perm("Present Sentinel")
	gone := r148Perm("Departed Sentinel")
	gs.Seats[1].Battlefield = []*gameengine.Permanent{here}

	if !onAnySeatBattlefield(gs, here) {
		t.Error("a permanent on seat 1's battlefield should be found — the scan must cover every seat, not just seat 0")
	}
	if onAnySeatBattlefield(gs, gone) {
		t.Error("a permanent on no battlefield must not be found")
	}
}

// Identity is the POINTER: Permanent has no ID field, and two distinct
// permanents of the same card are different objects. If this compared by
// name or card, a second copy of a card would be wrongly treated as
// already-scanned and its leaving trigger would be dropped.
func TestR148_IdentityIsThePointerNotTheCard(t *testing.T) {
	gs := newTestGS(2)
	onBoard := r148Perm("Twin Sentinel")
	twin := &gameengine.Permanent{
		Card:       onBoard.Card, // same Card pointer, different Permanent
		Controller: 0, Owner: 0,
		Flags: map[string]int{}, Counters: map[string]int{},
	}
	gs.Seats[0].Battlefield = []*gameengine.Permanent{onBoard}

	if !onAnySeatBattlefield(gs, onBoard) {
		t.Fatal("precondition: the permanent on the battlefield should be found")
	}
	if onAnySeatBattlefield(gs, twin) {
		t.Error("a different Permanent sharing the same *Card must NOT count as present — identity is the pointer")
	}
}

// Mirrors fireTrigger's own scan, which skips nil seats. If this helper
// did not, it would differ from the loop it is replacing.
func TestR148_NilSafety(t *testing.T) {
	if onAnySeatBattlefield(nil, r148Perm("X")) {
		t.Error("nil GameState should report absent, not panic")
	}
	gs := newTestGS(2)
	if onAnySeatBattlefield(gs, nil) {
		t.Error("nil target should report absent")
	}
	gs.Seats[0] = nil
	if onAnySeatBattlefield(gs, r148Perm("Y")) {
		t.Error("a nil seat must be skipped, matching fireTrigger's scan")
	}
}
