package gameengine

import (
	"math/rand"
	"testing"
)

// CR §616.1a–e: the replacement-effect CATEGORY ordering (self-replacement
// first, then control / copy / back-face / other) is MANDATORY — the
// affected player's §616.1 choice is only AMONG effects of the same
// (lowest present) category, never across the category boundary.
//
// Regression for the r63 replacement-ordering probe: pickReplacement handed
// the FULL cross-category candidate list to the affected player's Hat and
// took reordered[0] blindly. The real Hats (GreedyHat / Yggdrasil) order by
// self/timestamp/value ignoring Category, so a Hat could apply a
// CategoryOther effect before a mandatory CategoryControlETB / self-
// replacement one whenever mixed-category replacements modified one event.

// catTsHat orders replacements purely by timestamp (ignoring §616 category),
// like the production GreedyHat — exactly the Hat that exposed the bug.
type catTsHat struct{ GreedyHatStub }

func (h *catTsHat) OrderReplacements(_ *GameState, _ int, c []*ReplacementEffect) []*ReplacementEffect {
	out := append([]*ReplacementEffect(nil), c...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Timestamp < out[i].Timestamp {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func rc_makeNoop(order *[]string, id, category string, ts int) *ReplacementEffect {
	return &ReplacementEffect{
		EventType:      "would_create_token",
		HandlerID:      id,
		Category:       category,
		ControllerSeat: 0,
		Timestamp:      ts,
		Applies:        func(_ *GameState, _ *ReplEvent) bool { return true },
		ApplyFn:        func(_ *GameState, _ *ReplEvent) { *order = append(*order, id) },
	}
}

// With a category-blind Hat present, the mandatory §616 category order must
// still hold: control_etb (rank 1) before other (rank 4), regardless of the
// Hat's timestamp preference.
func TestRepl_CategoryOrderingHonoredEvenWithHat(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Hat = &catTsHat{}

	order := []string{}
	// 'other' has the EARLIER timestamp, so a category-blind Hat wants it
	// first — but §616 mandates control_etb first.
	gs.RegisterReplacement(rc_makeNoop(&order, "other", CategoryOther, 1))
	gs.RegisterReplacement(rc_makeNoop(&order, "control", CategoryControlETB, 100))

	ev := NewReplEvent("would_create_token")
	ev.TargetSeat = 0
	ev.SetCount(1)
	FireEvent(gs, ev)

	if len(order) != 2 {
		t.Fatalf("expected both to fire, got %v", order)
	}
	if order[0] != "control" || order[1] != "other" {
		t.Errorf("§616.1 category order must hold even with a Hat: want [control other], got %v", order)
	}
}

// Self-replacement (rank 0) must apply before everything, even when a
// category-blind Hat prefers a later-timestamp Other effect.
func TestRepl_SelfReplacementFirstEvenWithHat(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Seats[0].Hat = &catTsHat{}

	order := []string{}
	gs.RegisterReplacement(rc_makeNoop(&order, "other", CategoryOther, 1))
	gs.RegisterReplacement(rc_makeNoop(&order, "self", CategorySelfReplacement, 100))

	ev := NewReplEvent("would_create_token")
	ev.TargetSeat = 0
	ev.SetCount(1)
	FireEvent(gs, ev)

	if len(order) < 1 || order[0] != "self" {
		t.Errorf("self-replacement (§616.2) must apply first even with a Hat; got %v", order)
	}
}

// Within the SAME category the affected player's Hat choice IS honored
// (CR §616.1) — guards against the fix over-constraining the player.
func TestRepl_SameCategoryHatChoiceStillHonored(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	// A Hat that reverses (prefers later timestamp).
	gs.Seats[0].Hat = &catTsHat{}

	order := []string{}
	// Both CategoryOther — the Hat's timestamp ordering picks the earlier
	// one first; assert the player's preference within the category holds.
	gs.RegisterReplacement(rc_makeNoop(&order, "b", CategoryOther, 50))
	gs.RegisterReplacement(rc_makeNoop(&order, "a", CategoryOther, 10))

	ev := NewReplEvent("would_create_token")
	ev.TargetSeat = 0
	ev.SetCount(1)
	FireEvent(gs, ev)

	// catTsHat sorts by timestamp asc → a (ts10) then b (ts50).
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Errorf("same-category Hat ordering must be honored; want [a b], got %v", order)
	}
}
