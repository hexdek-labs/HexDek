package gameengine

import (
	"strings"
	"testing"
)

// r167 — LoopGuardLabelsFor exists so the log can say WHICH deadline
// check site caught a capped game. The load-bearing property is not
// "it renders labels" (LoopGuardLabelTop already did) but "it renders
// a label that is NOT in the top 5", which is the case that defeated
// the r166 verdict.

func r167Reset(t *testing.T) {
	t.Helper()
	ResetLoopGuardLabelsForTest()
	t.Cleanup(ResetLoopGuardLabelsForTest)
}

// r167Fire bumps one (guard, label) pair n times.
func r167Fire(guard, label string, n int) {
	for i := 0; i < n; i++ {
		bumpLoopGuardLabel(guard, label)
	}
}

func TestR167_RendersALabelRankedBelowTheTopFive(t *testing.T) {
	// The exact production shape on 2026-10-17: five busy per-card
	// counters in the thousands, and the one firing that mattered
	// buried underneath them.
	r167Reset(t)
	r167Fire("percard_dispatch_depth_cap", "zone_change", 13228)
	r167Fire("percard_dispatch_depth_cap", "permanent_ltb", 9474)
	r167Fire("percard_dispatch_depth_cap", "creature_dies", 9129)
	r167Fire("percard_dispatch_depth_cap", "permanent_etb", 3696)
	r167Fire("percard_dispatch_depth_cap", "graveyard_leave", 3624)
	r167Fire("game_deadline", "token_create", 1)

	// The premise, pinned as its own assertion (r162's lesson): the
	// existing field genuinely cannot show this.
	if top := LoopGuardLabelTop(5); strings.Contains(top, "token_create") {
		t.Fatalf("premise broken — guardtop already shows the deadline: %s", top)
	}
	if got := LoopGuardLabelsFor("game_deadline"); got != "token_create:1" {
		t.Fatalf("LoopGuardLabelsFor = %q, want %q", got, "token_create:1")
	}
}

func TestR167_RendersEverySiteSortedHeaviestFirst(t *testing.T) {
	r167Reset(t)
	r167Fire("game_deadline", "stack_drain", 2)
	r167Fire("game_deadline", "token_create", 7)
	r167Fire("game_deadline", "trigger_drain", 2)

	// Heaviest first; the two ties broken by name so the column is
	// stable across runs.
	want := "token_create:7,stack_drain:2,trigger_drain:2"
	if got := LoopGuardLabelsFor("game_deadline"); got != want {
		t.Fatalf("LoopGuardLabelsFor = %q, want %q", got, want)
	}
}

func TestR167_EmptyWhenTheGuardNeverFired(t *testing.T) {
	// The field must disappear entirely rather than print an empty
	// column on every one of the ~230 windows in a two-hour segment.
	r167Reset(t)
	r167Fire("sba_max_passes", "x", 3)

	if got := LoopGuardLabelsFor("game_deadline"); got != "" {
		t.Fatalf("unfired guard rendered %q, want empty", got)
	}
	if got := LoopGuardLabelsFor(""); got != "" {
		t.Fatalf("empty guard name rendered %q, want empty", got)
	}
}

func TestR167_MatchesTheGuardExactlyNotBySubstring(t *testing.T) {
	// The design choice this test exists to defend: an exact "guard/"
	// prefix. A Contains match would pick up BOTH a differently-named
	// guard that ends with this one AND any label containing the text.
	r167Reset(t)
	r167Fire("game_deadline", "token_create", 1)
	r167Fire("spectator_game_deadline", "stack_drain", 5)
	r167Fire("sba_max_passes", "game_deadline_cleanup", 9)

	got := LoopGuardLabelsFor("game_deadline")
	if got != "token_create:1" {
		t.Fatalf("LoopGuardLabelsFor = %q, want only the exact guard's own label", got)
	}
	// And the other guard still renders its own, independently.
	if got := LoopGuardLabelsFor("spectator_game_deadline"); got != "stack_drain:5" {
		t.Fatalf("sibling guard = %q, want %q", got, "stack_drain:5")
	}
}

func TestR167_DoesNotTruncateAGuardWithManyLabels(t *testing.T) {
	// Written AFTER control C (a top-5 cap inside LoopGuardLabelsFor)
	// failed to go red. It failed because the cap is applied after the
	// guard filter, and the other tests only ever give one guard a
	// single label — so the cap had nothing to cut. That is a hole in
	// the tests, not a harmless control.
	//
	// The case is real: percard_dispatch_depth_cap already carries at
	// least five distinct labels in production, and the whole point of
	// this function is to show what a top-n column hides. A cap here
	// would reintroduce exactly the blindness it was built to remove.
	//
	// No unbounded-growth risk: bumpLoopGuardLabel already enforces a
	// cardinality bound on distinct pairs, so the renderer does not
	// need a second one.
	r167Reset(t)
	for i, lbl := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		r167Fire("percard_dispatch_depth_cap", lbl, 10-i)
	}

	got := LoopGuardLabelsFor("percard_dispatch_depth_cap")
	if n := strings.Count(got, ",") + 1; n != 7 {
		t.Fatalf("rendered %d labels, want all 7 — the column is truncating: %s", n, got)
	}
	if !strings.HasSuffix(got, "g:4") {
		t.Fatalf("the lightest label was dropped; got %q", got)
	}
}
