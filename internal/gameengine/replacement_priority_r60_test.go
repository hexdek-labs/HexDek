package gameengine

// R60 — §616.1 replacement-effect priority ordering audit.
//
// `pickReplacement` (replacement.go) sorts applicable replacements by:
//
//   1. §616.1a–e category rank (SelfReplacement < ControlETB < CopyETB
//      < BackFaceUp < Other).
//   2. Active-player ownership within the same category (active first).
//   3. Timestamp ascending within the same (category, active-ownership)
//      bucket.
//   4. Affected-player's Hat may override the sort via OrderReplacements
//      — §616.1 says the affected player chooses among same-category
//      applicable effects.
//
// Existing tests cover:
//   - SelfReplacement before Other with cancellation short-circuit
//     (TestRepl_CategoryOrdering_SelfReplacementFirst)
//   - Same-category timestamp ordering for HS vs DS
//     (TestRepl_DoublingSeason_HardenedScales_APNAP_HSFirst/_DSFirst)
//
// Gaps before this file:
//   - No test pins APNAP active-player precedence over timestamp
//     within the same category (sort key #2).
//   - No test pins strict ordering across THREE+ distinct §616.1
//     categories (only the two-category SelfReplacement+Other case).
//   - No test pins that the affected player's Hat.OrderReplacements
//     override actually wins over the deterministic sort.
//
// Rule citations:
//   §616.1   — replacement category ordering
//   §616.1a–e — category rank assignment
//   §101.4   — APNAP ordering fallback for simultaneous effects

import (
	"testing"
)

// -----------------------------------------------------------------------------
// APNAP within same category — active-player effect fires first even
// with a LATER timestamp.
//
// Setup: two CategoryOther draw-doublers, one on each seat. Seat 0 is
// active; its doubler is registered SECOND (higher timestamp). Without
// the active-player precedence in pickReplacement, the earlier-ts seat 1
// doubler would fire first; with it, seat 0's doubler fires first.
//
// Both fire on a seat-0 draw (CategoryOther doublers always apply when
// TargetSeat matches — we make seat 1's also Applies for seat 0 draws
// so they're both candidates).
// -----------------------------------------------------------------------------

func TestReplPriority_APNAP_ActivePlayerBeatsTimestamp(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	fireOrder := []string{}
	makeAdder := func(id string, controller int, ts int) *ReplacementEffect {
		return &ReplacementEffect{
			EventType:      "would_draw",
			HandlerID:      id,
			Category:       CategoryOther,
			ControllerSeat: controller,
			Timestamp:      ts,
			Applies:        func(_ *GameState, ev *ReplEvent) bool { return ev.TargetSeat == 0 && ev.Count() > 0 },
			ApplyFn: func(_ *GameState, ev *ReplEvent) {
				fireOrder = append(fireOrder, id)
				ev.SetCount(ev.Count() + 1)
			},
		}
	}
	// Seat 1 (non-active) effect with EARLIER timestamp.
	gs.RegisterReplacement(makeAdder("seat1_early", 1, 1))
	// Seat 0 (active) effect with LATER timestamp.
	gs.RegisterReplacement(makeAdder("seat0_late", 0, 9))

	ev := NewReplEvent("would_draw")
	ev.TargetSeat = 0
	ev.SetCount(1)
	FireEvent(gs, ev)

	if len(fireOrder) != 2 {
		t.Fatalf("expected both effects to fire, got %d: %v", len(fireOrder), fireOrder)
	}
	if fireOrder[0] != "seat0_late" {
		t.Errorf("APNAP says active-player (seat 0) effect should fire FIRST despite later timestamp; got order %v",
			fireOrder)
	}
	if fireOrder[1] != "seat1_early" {
		t.Errorf("non-active (seat 1) effect should fire second; got order %v", fireOrder)
	}
}

// -----------------------------------------------------------------------------
// Strict category ordering across three §616.1 categories — timestamps
// reversed to prove category dominates timestamp.
//
// Categories used: ControlETB (rank 1), CopyETB (rank 2), Other (rank 4).
// Timestamps assigned so that lower rank = LATER timestamp — the sort
// must pick category over timestamp.
// -----------------------------------------------------------------------------

func TestReplPriority_StrictCategoryOrderingAcrossThree(t *testing.T) {
	gs := newFixtureGame(t)

	fireOrder := []string{}
	makeNoop := func(id, category string, ts int) *ReplacementEffect {
		return &ReplacementEffect{
			EventType:      "would_create_token",
			HandlerID:      id,
			Category:       category,
			ControllerSeat: 0,
			Timestamp:      ts,
			Applies:        func(_ *GameState, _ *ReplEvent) bool { return true },
			ApplyFn:        func(_ *GameState, _ *ReplEvent) { fireOrder = append(fireOrder, id) },
		}
	}
	// Register in inverse-category order to confirm registration order
	// doesn't leak through the sort either.
	gs.RegisterReplacement(makeNoop("other", CategoryOther, 1))
	gs.RegisterReplacement(makeNoop("copy", CategoryCopyETB, 10))
	gs.RegisterReplacement(makeNoop("control", CategoryControlETB, 100))

	ev := NewReplEvent("would_create_token")
	ev.TargetSeat = 0
	ev.SetCount(1)
	FireEvent(gs, ev)

	if len(fireOrder) != 3 {
		t.Fatalf("expected all 3 to fire, got %d: %v", len(fireOrder), fireOrder)
	}
	want := []string{"control", "copy", "other"} // §616.1: rank 1, 2, 4
	for i, w := range want {
		if fireOrder[i] != w {
			t.Errorf("position %d: expected %q, got %q; full order=%v", i, w, fireOrder[i], fireOrder)
		}
	}
}

// -----------------------------------------------------------------------------
// Affected-player Hat.OrderReplacements override wins over the sort.
//
// CR §616.1: "if more than one replacement effect would apply, the
// affected player chooses the order." When the affected seat has a Hat
// that implements OrderReplacements, pickReplacement honors it instead
// of the deterministic sort.
//
// Setup: two CategoryOther adders, both on seat 0. Default sort would
// fire "a" first (earlier ts). The Hat reverses the slice → "b" first.
// -----------------------------------------------------------------------------

// reverserHat embeds GreedyHatStub for all the other Hat methods and
// overrides OrderReplacements to reverse the candidate slice.
type reverserHat struct {
	GreedyHatStub
	called int
}

func (h *reverserHat) OrderReplacements(_ *GameState, _ int, candidates []*ReplacementEffect) []*ReplacementEffect {
	h.called++
	out := make([]*ReplacementEffect, len(candidates))
	for i, c := range candidates {
		out[len(candidates)-1-i] = c
	}
	return out
}

func TestReplPriority_AffectedHatOverridesSort(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	hat := &reverserHat{}
	gs.Seats[0].Hat = hat

	fireOrder := []string{}
	makeAdder := func(id string, ts int) *ReplacementEffect {
		return &ReplacementEffect{
			EventType:      "would_gain_life",
			HandlerID:      id,
			Category:       CategoryOther,
			ControllerSeat: 0,
			Timestamp:      ts,
			Applies:        func(_ *GameState, ev *ReplEvent) bool { return ev.TargetSeat == 0 && ev.Count() > 0 },
			ApplyFn: func(_ *GameState, ev *ReplEvent) {
				fireOrder = append(fireOrder, id)
				ev.SetCount(ev.Count() + 1)
			},
		}
	}
	gs.RegisterReplacement(makeAdder("a", 1)) // earlier ts — default would fire first
	gs.RegisterReplacement(makeAdder("b", 2))

	ev := NewReplEvent("would_gain_life")
	ev.TargetSeat = 0
	ev.SetCount(0) // start at 0; each adder makes it positive, then the second falls into Applies
	// Actually: with SetCount(0), the first iteration's Applies() returns
	// false because ev.Count() > 0 is false. Bump to 1 to make both
	// applicable on the same pass.
	ev.SetCount(1)
	FireEvent(gs, ev)

	if len(fireOrder) != 2 {
		t.Fatalf("expected both adders to fire, got %d: %v", len(fireOrder), fireOrder)
	}
	if hat.called == 0 {
		t.Error("Hat.OrderReplacements should have been called by pickReplacement")
	}
	// Hat reverses the deterministic order. Default would be [a, b]
	// (earlier-ts first). Hat reversal → [b, a].
	if fireOrder[0] != "b" {
		t.Errorf("expected Hat-reversed order to fire b first; got %v", fireOrder)
	}
	if fireOrder[1] != "a" {
		t.Errorf("expected Hat-reversed order to fire a second; got %v", fireOrder)
	}
}
