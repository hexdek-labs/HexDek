package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// Phase-1D residue #2 regression pins. Per the same pattern as the
// _r60_test.go file from PR #486: each investigated case needs a test
// that catches a future regression — either a direct call to the
// affected function or, for compile-time AST-enum verifications,
// pinning the data shape.

// TestCompareInt_AllSixStandardOps pins all six standard int
// comparison operators including "!=", which Phase 1D-residue audit
// flagged as unreachable from any Go emitter. Removing the "!="
// arm would silently return false for genuinely-unequal inputs if
// the parser ever emits the op — caught here.
func TestCompareInt_AllSixStandardOps(t *testing.T) {
	cases := []struct {
		a, b int
		op   string
		want bool
	}{
		// < / <=
		{3, 5, "<", true},
		{5, 5, "<", false},
		{5, 5, "<=", true},
		{6, 5, "<=", false},

		// > / >=
		{5, 3, ">", true},
		{5, 5, ">", false},
		{5, 5, ">=", true},
		{4, 5, ">=", false},

		// == / = (alias) / != (the arm under audit)
		{5, 5, "==", true},
		{5, 6, "==", false},
		{5, 5, "=", true},
		{5, 6, "=", false},
		{5, 6, "!=", true},
		{5, 5, "!=", false},

		// Unknown op → conservative false (documented fallback).
		{5, 5, "<>", false},
		{5, 5, "", false},
	}
	for _, c := range cases {
		if got := compareInt(c.a, c.op, c.b); got != c.want {
			t.Errorf("compareInt(%d, %q, %d): got %v want %v", c.a, c.op, c.b, got, c.want)
		}
	}
}

// TestTriggerControllerMatches_ActivePlayer pins the "active_player"
// arm in phases.go. The arm was flagged because no Go-side emitter
// produces it, but gameast.Trigger.Controller is a JSON-tagged field
// the AST parser populates with this exact value (per the doc comment
// at internal/gameast/trigger.go:20). Removing the arm would let an
// "active_player" trigger fall through to the conservative true
// default — masking the active-seat gating contract.
func TestTriggerControllerMatches_ActivePlayer(t *testing.T) {
	gs := &GameState{Active: 0, Seats: []*Seat{{}, {}}}
	myPerm := &Permanent{Controller: 0}
	oppPerm := &Permanent{Controller: 1}

	// "active_player" only fires when the bearer is on the active seat.
	tr := &gameast.Trigger{Controller: "active_player"}
	if !triggerControllerMatches(gs, myPerm, tr) {
		t.Error("active_player + bearer on active seat: should match")
	}
	if triggerControllerMatches(gs, oppPerm, tr) {
		t.Error("active_player + bearer on inactive seat: should NOT match")
	}
}

// TestTriggerControllerMatches_LiveAndAliasArms pins the live
// arms ("", "you", "each", "each_player", "opponent") to make sure
// none of them regress while the active_player documentation change
// lands. The compareInt analog is in TestCompareInt_AllSixStandardOps;
// this is its sibling for the phases.go switch.
func TestTriggerControllerMatches_LiveAndAliasArms(t *testing.T) {
	gs := &GameState{Active: 0, Seats: []*Seat{{}, {}}}
	myPerm := &Permanent{Controller: 0}
	oppPerm := &Permanent{Controller: 1}

	// "" / "you" → "your upkeep"-style, only on bearer's turn.
	for _, ctrl := range []string{"", "you"} {
		tr := &gameast.Trigger{Controller: ctrl}
		if !triggerControllerMatches(gs, myPerm, tr) {
			t.Errorf("%q + bearer-on-active: should match", ctrl)
		}
		if triggerControllerMatches(gs, oppPerm, tr) {
			t.Errorf("%q + bearer-on-inactive: should NOT match", ctrl)
		}
	}

	// "each" / "each_player" → always fires.
	for _, ctrl := range []string{"each", "each_player"} {
		tr := &gameast.Trigger{Controller: ctrl}
		if !triggerControllerMatches(gs, myPerm, tr) {
			t.Errorf("%q: should always match", ctrl)
		}
		if !triggerControllerMatches(gs, oppPerm, tr) {
			t.Errorf("%q: should always match (inactive bearer)", ctrl)
		}
	}

	// "opponent" → bearer NOT on active.
	tr := &gameast.Trigger{Controller: "opponent"}
	if triggerControllerMatches(gs, myPerm, tr) {
		t.Error("opponent + bearer-on-active: should NOT match")
	}
	if !triggerControllerMatches(gs, oppPerm, tr) {
		t.Error("opponent + bearer-on-inactive: should match")
	}

	// Unknown / unset → conservative match.
	tr = &gameast.Trigger{Controller: "unknown_new_tag"}
	if !triggerControllerMatches(gs, myPerm, tr) {
		t.Error("unknown trigger controller: should fall through to match=true (conservative)")
	}
}

// TestResolveSacrifice_SelfReferenceQueryBaseArms exercises the
// self-reference shortcut at resolve.go:1661. Phase 1D-residue audit
// flagged "that_thing" as unreachable because gameast.Filter.Base
// is parser-emitted (JSON), invisible to the static scanner. Every
// arm in the cluster routes the same way (sacrifice the source perm);
// missing any arm causes a parser-emitted "self/it/this/that_*"
// Sacrifice to silently no-op instead of sacrificing the source.
//
// Drive each value through resolveSacrifice and assert the source is
// sacrificed (moved to graveyard).
func TestResolveSacrifice_SelfReferenceQueryBaseArms(t *testing.T) {
	bases := []string{
		"self", "it", "this",
		"that_creature", "that creature",
		"this_creature", "this creature",
		"that_thing", "that",
	}
	for _, base := range bases {
		t.Run(base, func(t *testing.T) {
			gs := &GameState{
				Seats: []*Seat{{Idx: 0, Life: 40}},
				Flags: map[string]int{},
			}
			card := &Card{Name: "Pestilence", Owner: 0, Types: []string{"enchantment"}}
			perm := &Permanent{Card: card, Controller: 0, Owner: 0, Flags: map[string]int{}}
			gs.Seats[0].Battlefield = []*Permanent{perm}

			e := &gameast.Sacrifice{Query: gameast.Filter{Base: base}}
			resolveSacrifice(gs, perm, e)

			if len(gs.Seats[0].Battlefield) != 0 {
				t.Errorf("base=%q: source should be sacrificed, battlefield still has %d perms",
					base, len(gs.Seats[0].Battlefield))
			}
			if len(gs.Seats[0].Graveyard) != 1 || gs.Seats[0].Graveyard[0] != card {
				t.Errorf("base=%q: source card should be in graveyard, got %v",
					base, gs.Seats[0].Graveyard)
			}
		})
	}
}
