package gameengine

import "testing"

// r184 — the per-game accessor behind the grinder's swallowed= field.
//
// r182's accounting is process-global, so it can say 138 games out of
// 5,794 hit the depth cap but not WHICH 138. DepthCapSwallowedCount
// reads the one per-game field back out, which is the half of the
// attribution that was missing. These tests pin it to the same field
// NoteDepthCapSwallow increments, because the only way the grinder
// line can lie is by reading something that is not that counter.
//
// MUTATION CONTROLS (run against this file + the hexapi r184 file):
//
//	M1  DepthCapSwallowedCount returns 0 unconditionally
//	    -> 2 RED here (CountsEverySwallow, IsIndependentPerGameState)
//	       0 RED in hexapi. See M1 note in the hexapi test file.
//	M2  DepthCapSwallowedCount returns gs.depthCapCascade instead of
//	    gs.depthCapSwallowed
//	    -> 1 RED here (CountsEverySwallow, via the NoteCascadeRoot arm)
//	M3  drop the nil guard (return gs.depthCapSwallowed directly)
//	    -> 1 RED here (NilGameStateReadsZero, panic)
//	M4  showmatch: omit the swallowed= field from the format string
//	    -> 5 RED in hexapi
//	M5  showmatch: print 0 instead of n/a when gs == nil
//	    -> 1 RED in hexapi (NoGameRendersNotApplicable)
//	M6  showmatch: revert `gs =` to `gs :=` (re-shadow, the r170 trap)
//	    -> does not compile ("no new variables on left side of :=");
//	       see the hexapi header for why that is the honest answer
//	       rather than a green.

func TestR184_CountsEverySwallow(t *testing.T) {
	gs := &GameState{}
	if got := gs.DepthCapSwallowedCount(); got != 0 {
		t.Fatalf("a fresh GameState must report 0 swallows, got %d", got)
	}
	// Two cascades, so a reader wired to the per-cascade counter
	// (which NoteCascadeRoot resets) reports 2 instead of 5.
	gs.NoteCascadeRoot()
	for i := 0; i < 3; i++ {
		gs.NoteDepthCapSwallow("creature_dies")
	}
	gs.NoteCascadeRoot()
	for i := 0; i < 2; i++ {
		gs.NoteDepthCapSwallow("permanent_etb")
	}
	if got := gs.DepthCapSwallowedCount(); got != 5 {
		t.Fatalf("DepthCapSwallowedCount = %d, want 5 (the GAME total across "+
			"both cascades, not the current cascade)", got)
	}
}

func TestR184_NilGameStateReadsZero(t *testing.T) {
	var gs *GameState
	if got := gs.DepthCapSwallowedCount(); got != 0 {
		t.Fatalf("nil GameState must read 0, got %d", got)
	}
}

// The count must be the GAME's, not the process's. Without this, an
// implementation backed by one of r182's atomics would look correct in
// a single-game test and silently report every other game's swallows
// on the grinder, which is the exact defect r184 exists to fix.
func TestR184_IsIndependentPerGameState(t *testing.T) {
	a, b := &GameState{}, &GameState{}
	for i := 0; i < 4; i++ {
		a.NoteDepthCapSwallow("zone_change")
	}
	if got := b.DepthCapSwallowedCount(); got != 0 {
		t.Fatalf("a second GameState reports %d swallows after another game "+
			"swallowed 4 — the accessor is reading process-wide state", got)
	}
	if got := a.DepthCapSwallowedCount(); got != 4 {
		t.Fatalf("first GameState = %d, want 4", got)
	}
}

// The accessor must not disturb what it measures: instrumentation that
// resets or mutates on read would make two consecutive log lines
// disagree, and worse, would be a behaviour change on a path the brief
// requires to be inert.
func TestR184_ReadIsIdempotent(t *testing.T) {
	gs := &GameState{}
	gs.NoteDepthCapSwallow("creature_dies")
	gs.NoteDepthCapSwallow("creature_dies")
	first := gs.DepthCapSwallowedCount()
	for i := 0; i < 5; i++ {
		if got := gs.DepthCapSwallowedCount(); got != first {
			t.Fatalf("read %d returned %d, first read returned %d — the "+
				"accessor has a side effect", i, got, first)
		}
	}
	if gs.depthCapFirstEvent != "creature_dies" || gs.depthCapMixed {
		t.Fatalf("reading the count disturbed the sibling per-game fields "+
			"(firstEvent=%q mixed=%v)", gs.depthCapFirstEvent, gs.depthCapMixed)
	}
}
