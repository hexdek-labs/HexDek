package gameengine

import (
	"strings"
	"testing"
)

// r182 — unit tests for the depth-cap swallow accounting. These drive
// the primitives directly; the companion test in per_card drives the
// REAL fireTrigger and proves the production call sites are wired,
// which is the split r163 used for the same reason: a unit test here
// cannot tell whether anything calls these.

func r182Reset(t *testing.T) {
	t.Helper()
	ResetDepthCapForTest()
	ResetLoopGuardLabelsForTest()
	t.Cleanup(func() {
		ResetDepthCapForTest()
		ResetLoopGuardLabelsForTest()
	})
}

func TestR182_NothingRecordedUntilTheCapFires(t *testing.T) {
	r182Reset(t)
	if s := DepthCapSummary(); s != "" {
		t.Fatalf("summary = %q before any swallow, want empty so the "+
			"grinder line is unchanged on the normal path", s)
	}
	games, mixed, max, cascade, buckets := DepthCapStats()
	if games|mixed|max|cascade != 0 || len(buckets) != 0 {
		t.Fatalf("stats are not zero at rest: %d %d %d %d %v",
			games, mixed, max, cascade, buckets)
	}
}

func TestR182_PerGameTotalAndSurvivalCurve(t *testing.T) {
	r182Reset(t)
	gs := &GameState{}
	gs.NoteCascadeRoot()
	const n = 5
	for i := 0; i < n; i++ {
		gs.NoteDepthCapSwallow("creature_dies")
	}

	games, mixed, max, cascade, buckets := DepthCapStats()
	if games != 1 {
		t.Fatalf("games = %d, want 1", games)
	}
	if max != n {
		t.Fatalf("max swallows per game = %d, want %d — this is the number "+
			"that tells 'many games losing a few' from 'one game losing "+
			"thousands'", max, n)
	}
	if cascade != n {
		t.Fatalf("per-cascade peak = %d, want %d (all five in one cascade)", cascade, n)
	}
	if mixed != 0 {
		t.Fatalf("mixed = %d, want 0 — every swallow was the same event", mixed)
	}
	// Survival curve: 5 swallows crosses 1, 2 and 4 but not 8.
	for _, th := range []uint64{1, 2, 4} {
		if buckets[th] != 1 {
			t.Fatalf("bucket >=%d = %d, want 1 (%v)", th, buckets[th], buckets)
		}
	}
	if got, ok := buckets[8]; ok {
		t.Fatalf("bucket >=8 = %d with only %d swallows (%v)", got, n, buckets)
	}

	want := "games:1,max:5,cascademax:5,mixed:0,hist[1:1,2:1,4:1]"
	if got := DepthCapSummary(); got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

// TestR182_CascadePeakResetsAtTheCascadeRoot is the control on
// NoteCascadeRoot. Without the reset, cascademax is just the per-game
// total under a second name and carries no information.
func TestR182_CascadePeakResetsAtTheCascadeRoot(t *testing.T) {
	r182Reset(t)
	gs := &GameState{}

	gs.NoteCascadeRoot() // cascade 1: three swallows
	gs.NoteDepthCapSwallow("creature_dies")
	gs.NoteDepthCapSwallow("creature_dies")
	gs.NoteDepthCapSwallow("creature_dies")

	gs.NoteCascadeRoot() // cascade 2: two swallows
	gs.NoteDepthCapSwallow("creature_dies")
	gs.NoteDepthCapSwallow("creature_dies")

	_, _, max, cascade, _ := DepthCapStats()
	if max != 5 {
		t.Fatalf("per-game total = %d, want 5 — the game total must span "+
			"cascades", max)
	}
	if cascade != 3 {
		t.Fatalf("per-cascade peak = %d, want 3 (the larger of 3 and 2) — "+
			"the counter must reset when nesting returns to depth 1", cascade)
	}
}

// TestR182_MixedDetectsASecondDistinctEvent separates a tight mutual
// loop (one or two events repeating) from a wide cascade.
func TestR182_MixedDetectsASecondDistinctEvent(t *testing.T) {
	r182Reset(t)

	same := &GameState{}
	same.NoteCascadeRoot()
	same.NoteDepthCapSwallow("creature_dies")
	same.NoteDepthCapSwallow("creature_dies")

	wide := &GameState{}
	wide.NoteCascadeRoot()
	wide.NoteDepthCapSwallow("life_gained")
	wide.NoteDepthCapSwallow("life_lost")
	wide.NoteDepthCapSwallow("life_gained") // back to the first: still one mixed game

	games, mixed, _, _, _ := DepthCapStats()
	if games != 2 {
		t.Fatalf("games = %d, want 2", games)
	}
	if mixed != 1 {
		t.Fatalf("mixed games = %d, want 1 — only the second game swallowed "+
			"two distinct events, and it must be counted once however many "+
			"times it alternates", mixed)
	}
}

// TestR182_FirstEventIsAttributedOncePerGame is the per-GAME column
// that guardtop= cannot provide: its labels are per DISPATCH, so one
// looping game buries every other game there.
func TestR182_FirstEventIsAttributedOncePerGame(t *testing.T) {
	r182Reset(t)

	loud := &GameState{}
	loud.NoteCascadeRoot()
	for i := 0; i < 500; i++ {
		loud.NoteDepthCapSwallow("life_gained")
	}
	quiet := &GameState{}
	quiet.NoteCascadeRoot()
	quiet.NoteDepthCapSwallow("creature_dies")

	got := LoopGuardLabelsFor(DepthCapFirstEventGuard)
	// Sorted heaviest first, ties by name: one game each.
	if got != "creature_dies:1,life_gained:1" {
		t.Fatalf("per-game first-event attribution = %q, want "+
			"\"creature_dies:1,life_gained:1\" — a 500-swallow game and a "+
			"1-swallow game are one game each in this column", got)
	}
	if !strings.Contains(DepthCapSummary(), "max:500") {
		t.Fatalf("summary %q lost the 500-swallow game's peak", DepthCapSummary())
	}
}

// TestR182_BucketIndexSaturates keeps the fixed-size bucket array from
// being indexed out of range if a game ever swallows an absurd number.
func TestR182_BucketIndexSaturates(t *testing.T) {
	r182Reset(t)
	gs := &GameState{}
	gs.NoteCascadeRoot()
	// Jump the per-game counter straight past the top bucket rather
	// than looping 2^32 times, then take one more swallow so the
	// power-of-two crossing test runs on a huge n.
	gs.depthCapSwallowed = 1<<depthCapBucketCount - 1
	gs.NoteDepthCapSwallow("creature_dies") // n = 1<<32, a power of two
	_, _, max, _, buckets := DepthCapStats()
	if max != 1<<depthCapBucketCount {
		t.Fatalf("max = %d, want %d", max, uint64(1)<<depthCapBucketCount)
	}
	if buckets[1<<(depthCapBucketCount-1)] != 1 {
		t.Fatalf("the top bucket must absorb the overflow (%v)", buckets)
	}
}

// TestR182_NilReceiverIsSafe — fireTrigger already guards gs == nil
// before reaching these, but a counter that panics is worse than a
// counter that is missing.
func TestR182_NilReceiverIsSafe(t *testing.T) {
	r182Reset(t)
	var gs *GameState
	gs.NoteCascadeRoot()
	gs.NoteDepthCapSwallow("creature_dies")
	if games, _, _, _, _ := DepthCapStats(); games != 0 {
		t.Fatalf("a nil GameState recorded %d games", games)
	}
}
