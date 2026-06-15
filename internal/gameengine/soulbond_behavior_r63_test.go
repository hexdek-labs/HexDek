package gameengine

import (
	"math/rand"
	"testing"
)

// soulbond_behavior_r63_test.go — behavioral probe for Soulbond (CR §702.97).
// The pairing primitive (PairSoulbond / IsPaired, mutual paired_timestamp)
// existed but NOTHING ever broke a pairing — a survivor kept a stale
// paired_timestamp after its partner left, leaving it permanently "paired" and
// ineligible for re-pairing. These pin the new UnpairOnLeave behavior plus the
// pairing gates.

func newSoulbondGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(97)), nil)
}

func sbCreature(gs *GameState, seat int, name string) *Permanent {
	card := &Card{Name: name, Owner: seat, Types: []string{"creature"}, BasePower: 2, BaseToughness: 2}
	p := &Permanent{
		Card:       card,
		Controller: seat,
		Owner:      seat,
		Flags:      map[string]int{},
		Counters:   map[string]int{},
		Timestamp:  gs.NextTimestamp(),
	}
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	return p
}

// (b)+(f) PairSoulbond pairs two unpaired creatures (either may be the soulbond
// source — two soulbond creatures can pair with each other); an already-paired
// creature can't be newly paired.
func TestSoulbond_PairAndUnpairedGate(t *testing.T) {
	gs := newSoulbondGame(t)
	a := sbCreature(gs, 0, "Deadeye Navigator")
	b := sbCreature(gs, 0, "Peregrine Drake")
	c := sbCreature(gs, 0, "Other Soulbond")

	if !PairSoulbond(gs, a, b) {
		t.Fatal("PairSoulbond should pair two unpaired creatures")
	}
	if !IsPaired(a) || !IsPaired(b) {
		t.Error("(c) both paired creatures should report IsPaired")
	}
	// (b) c cannot pair with the already-paired a.
	if PairSoulbond(gs, c, a) {
		t.Error("(b) an already-paired creature must not be newly paired")
	}
	if IsPaired(c) {
		t.Error("(b) the refused pairing must not mark c as paired")
	}
}

// (d) When a paired creature LEAVES the battlefield, the pairing ends and the
// surviving partner becomes unpaired again (re-pairable). Driven end-to-end
// through SacrificePermanent → the LTB chokepoint.
func TestSoulbond_UnpairsWhenPartnerLeaves(t *testing.T) {
	gs := newSoulbondGame(t)
	a := sbCreature(gs, 0, "Deadeye Navigator")
	b := sbCreature(gs, 0, "Peregrine Drake")
	PairSoulbond(gs, a, b)
	if !IsPaired(a) || !IsPaired(b) {
		t.Fatal("setup: both should be paired")
	}

	// b dies.
	SacrificePermanent(gs, b, "test")

	if IsPaired(a) {
		t.Error("(d) the surviving partner must become unpaired when its partner leaves")
	}
	// And a can be re-paired with a fresh creature.
	c := sbCreature(gs, 0, "New Friend")
	if !PairSoulbond(gs, a, c) {
		t.Error("(d) the unpaired survivor must be eligible for a new pairing")
	}
	if !IsPaired(a) || !IsPaired(c) {
		t.Error("(d) re-pairing should succeed after the old partner left")
	}
}

// (d) A flicker (battlefield → exile, the Deadeye combo path) breaks the pair:
// the leaving creature's exit clears the partner's stale flag.
func TestSoulbond_UnpairsOnExitToExile(t *testing.T) {
	gs := newSoulbondGame(t)
	a := sbCreature(gs, 0, "Deadeye Navigator")
	b := sbCreature(gs, 0, "Blink Target")
	PairSoulbond(gs, a, b)

	// Simulate b leaving for exile (flicker / removal), the LTB chokepoint.
	FireZoneChangeTriggers(gs, b, b.Card, "battlefield", "exile")

	if IsPaired(a) {
		t.Error("(d) partner must unpair when the other creature leaves for exile")
	}
}

// (d) Direct UnpairOnLeave clears BOTH ends of the pairing (the core used by the
// control-change hooks, which can't go through the LTB chokepoint).
func TestSoulbond_UnpairOnLeaveClearsBoth(t *testing.T) {
	gs := newSoulbondGame(t)
	a := sbCreature(gs, 0, "A")
	b := sbCreature(gs, 0, "B")
	PairSoulbond(gs, a, b)

	UnpairOnLeave(gs, a)

	if IsPaired(a) || IsPaired(b) {
		t.Errorf("UnpairOnLeave must clear both ends; a=%v b=%v", IsPaired(a), IsPaired(b))
	}
}

// PairSoulbond refuses cross-controller pairing (CR §702.97 — "a creature you
// control").
func TestSoulbond_RefusesCrossController(t *testing.T) {
	gs := newSoulbondGame(t)
	a := sbCreature(gs, 0, "Mine")
	b := sbCreature(gs, 1, "Theirs")
	if PairSoulbond(gs, a, b) {
		t.Error("PairSoulbond must refuse creatures under different controllers")
	}
}
