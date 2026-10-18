package gameengine

import "testing"

// r185 — the per-GAME cascade maximum.
//
// depthCapCascade is reset by NoteCascadeRoot at every new top-level
// cascade, so the live field is worthless at end of game, and the
// existing cascadeMax is a process-wide atomic that cannot be
// attributed to one game. DepthCapCascadeMaxCount is the per-game
// half, and these tests pin the property that distinguishes it from
// both: it survives a cascade boundary and it is a MAXIMUM, not a
// last-value and not a total.
//
// MUTATION CONTROLS (counts are tests that go RED in this package):
//
//	M1  DepthCapCascadeMaxCount returns gs.depthCapCascade
//	    -> predicted 2, actual 2 (SurvivesACascadeBoundary,
//	       IsAMaximumNotALastValue). PeaksBelowTheGameTotal stays
//	       green because its last cascade IS the peak (all four are
//	       equal), so the current value and the maximum coincide.
//	M2  DepthCapCascadeMaxCount returns gs.depthCapSwallowed
//	    -> predicted 1 (PeaksBelowTheGameTotal), ACTUAL 3: the other
//	       two fire as well. I had assumed their game totals equalled
//	       their cascade peaks; they do not — SurvivesACascadeBoundary
//	       is 6 total against a peak of 5, and IsAMaximumNotALastValue
//	       is 12 against 9. Every test here that crosses a cascade
//	       boundary separates the two counters, which is more coverage
//	       than I credited them with, not less.
//	M3  drop the nil guard
//	    -> predicted 1, actual 1 (NilGameStateIsZeroNotAPanic).
//	M4  unconditional store (= instead of > compare) in
//	    NoteDepthCapSwallow
//	    -> predicted 2, actual 2 (same pair as M1, for the same
//	       reason: an unconditional store makes the field track the
//	       current cascade exactly).

// A game that swallows across two cascades must report the LARGER
// cascade, not the most recent one. This is the whole reason the
// field exists.
func TestR185_SurvivesACascadeBoundary(t *testing.T) {
	gs := &GameState{}
	gs.NoteCascadeRoot()
	for i := 0; i < 5; i++ {
		gs.NoteDepthCapSwallow("creature_dies")
	}
	gs.NoteCascadeRoot() // resets depthCapCascade to 0
	gs.NoteDepthCapSwallow("creature_dies")

	if got := gs.DepthCapCascadeMaxCount(); got != 5 {
		t.Fatalf("DepthCapCascadeMaxCount = %d, want 5 (the PEAK cascade, "+
			"not the current one, which is 1 after the second root)", got)
	}
	if gs.depthCapCascade != 1 {
		t.Fatalf("premise stale: depthCapCascade = %d, want 1 — "+
			"NoteCascadeRoot no longer resets the live counter", gs.depthCapCascade)
	}
}

// Maximum, not last value: a big cascade followed by a small one
// still reports the big one.
func TestR185_IsAMaximumNotALastValue(t *testing.T) {
	gs := &GameState{}
	gs.NoteCascadeRoot()
	for i := 0; i < 9; i++ {
		gs.NoteDepthCapSwallow("zone_change")
	}
	gs.NoteCascadeRoot()
	gs.NoteDepthCapSwallow("zone_change")
	gs.NoteCascadeRoot()
	gs.NoteDepthCapSwallow("zone_change")
	gs.NoteDepthCapSwallow("zone_change")

	if got := gs.DepthCapCascadeMaxCount(); got != 9 {
		t.Fatalf("DepthCapCascadeMaxCount = %d, want 9", got)
	}
}

// The discriminator property. Four cascades of three swallows each is
// twelve swallows in a game whose worst single chain is three — a
// wide board, not a loop. Game total and cascade peak must differ,
// and the accessor must report the peak.
func TestR185_PeaksBelowTheGameTotal(t *testing.T) {
	gs := &GameState{}
	for c := 0; c < 4; c++ {
		gs.NoteCascadeRoot()
		for i := 0; i < 3; i++ {
			gs.NoteDepthCapSwallow("permanent_etb")
		}
	}
	if got := gs.DepthCapSwallowedCount(); got != 12 {
		t.Fatalf("game total = %d, want 12", got)
	}
	if got := gs.DepthCapCascadeMaxCount(); got != 3 {
		t.Fatalf("cascade peak = %d, want 3 — twelve swallows spread over "+
			"four chains is a wide board, and reporting 12 here would read "+
			"as an unbounded loop", got)
	}
}

// Two GameStates must not see each other's peak; the process-wide
// atomic already does that job and this field exists because it is
// the wrong job.
func TestR185_IsPerGameNotProcessWide(t *testing.T) {
	a, b := &GameState{}, &GameState{}
	a.NoteCascadeRoot()
	for i := 0; i < 7; i++ {
		a.NoteDepthCapSwallow("life_gained")
	}
	b.NoteCascadeRoot()
	b.NoteDepthCapSwallow("life_gained")

	if got := b.DepthCapCascadeMaxCount(); got != 1 {
		t.Fatalf("game b reports cascade peak %d, want 1 — a's 7 leaked", got)
	}
	if got := a.DepthCapCascadeMaxCount(); got != 7 {
		t.Fatalf("game a reports cascade peak %d, want 7", got)
	}
}

// Instrumentation must never be the thing that kills a game.
func TestR185_NilGameStateIsZeroNotAPanic(t *testing.T) {
	var gs *GameState
	if got := gs.DepthCapCascadeMaxCount(); got != 0 {
		t.Fatalf("nil receiver returned %d, want 0", got)
	}
}

// Reading is read-only. The grinder reads this from a deferred log
// while nothing else is touching the state, but a counter that
// mutates on read would corrupt any later aggregate.
func TestR185_ReadingDoesNotMutate(t *testing.T) {
	gs := &GameState{}
	gs.NoteCascadeRoot()
	gs.NoteDepthCapSwallow("creature_dies")
	gs.NoteDepthCapSwallow("creature_dies")
	first := gs.DepthCapCascadeMaxCount()
	for i := 0; i < 3; i++ {
		if got := gs.DepthCapCascadeMaxCount(); got != first {
			t.Fatalf("read %d changed the value to %d", i, got)
		}
	}
	if gs.DepthCapSwallowedCount() != 2 {
		t.Fatalf("reading the cascade peak disturbed the swallow total")
	}
}
