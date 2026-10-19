package gameengine

import (
	"strconv"
	"strings"
	"testing"
)

// r187 — per-game count of SWALLOWING cascades, and a name table
// wide enough that distinct= is not saturated.
//
// WHAT THESE PIN. The load-bearing pair is
// TestR187_FanOutAndOneWideChainAreOppositeShapes (two games with
// IDENTICAL swallowed= totals that the existing fields cannot tell
// apart) and TestR187_TheLastCascadeOfAGameIsCounted (the reason the
// count is taken in NoteDepthCapSwallow and not in NoteCascadeRoot's
// already-gated arm). If the first ever stops discriminating, the
// field has collapsed back into swallowed=; if the second ever goes
// green under the NoteCascadeRoot placement, my objection to that
// placement was wrong.
//
// MUTATION CONTROLS. Counts are tests that go RED across
// internal/gameengine + internal/hexapi, EXCLUDING the one failure
// that is already red on origin/main
// (TestJudgeWatchdog_LivenessCapContractViaUniformEvent).
//
//	M1  count in NoteCascadeRoot's `depthCapCascade != 0` arm instead
//	    of at the 0 -> 1 transition (the placement the brief
//	    suggested)                        -> predicted 7, ACTUAL 10.
//	    All seven r187 tests as predicted — including
//	    TheLastCascadeOfAGameIsCounted, which is the objection this
//	    placement exists to answer — plus the two r182 golden-string
//	    tests in both packages, which I under-counted by forgetting
//	    that I had just written the new field's VALUE into their
//	    expected strings, and SilentCascadesAreNotCounted.
//	M2  drop the `if gs.depthCapCascade == 1` guard, so every swallow
//	    counts a cascade                  -> predicted 6, ACTUAL 8.
//	    The same two golden-string tests, same miss, same cause.
//	    SummaryCarriesThePerGameMaximum and
//	    ResetClearsTheProcessWideMaximum stayed GREEN as predicted:
//	    both drive one swallow per cascade, so cascades and swallows
//	    coincide there by construction and neither can hold this
//	    property.
//	M3  cascadeCompSlots back to 4        -> predicted 1, actual 1:
//	    NameTableHoldsTheObservedCardinality. r186's
//	    NameTableIsBoundedAndVisible correctly stays GREEN — it was
//	    re-derived from the constant this round, so it pins that the
//	    ceiling EXISTS at whatever size, which is a different
//	    property from the size being adequate.
//	M4  DepthCapSummary stops emitting cascadesmax:
//	                                      -> predicted 3, actual 3.
//	M5  ResetDepthCapForTest stops resetting the r187 atomic
//	                                      -> predicted 2, ACTUAL 4.
//	    Predicted ResetClearsTheProcessWideMaximum and
//	    IsPerGameNotProcessWide; missed that a leaked 446 also
//	    poisons SummaryCarriesThePerGameMaximum and r182's
//	    PerGameTotalAndSurvivalCurve. Under-counting cross-test
//	    leakage is the same mistake in a third form.
//	M6  drop cascades= from the CAPPED GAME line
//	                                      -> predicted 1, actual 1:
//	    CappedLineCarriesTheCascadeCount (hexapi transport).

func r187reset(t *testing.T) {
	t.Helper()
	reset := func() {
		ResetDepthCapForTest()
		ResetCascadeShapeForTest()
		ResetLoopGuardLabelsForTest()
	}
	reset()
	t.Cleanup(reset)
}

// r187gs mirrors r186gs: battlefieldPopulation reads only slice
// lengths, so the entries may be nil.
func r187gs(board int) *GameState {
	return &GameState{Seats: []*Seat{{Battlefield: make([]*Permanent, board)}}}
}

// r187run drives `cascades` top-level cascades of `per` swallows each
// on one game, WITHOUT a trailing NoteCascadeRoot — a real game does
// not get a courtesy cascade after its last one.
func r187run(gs *GameState, cascades, per int) {
	for c := 0; c < cascades; c++ {
		gs.NoteCascadeRoot()
		for i := 0; i < per; i++ {
			gs.NoteDepthCapSwallow("creature_dies")
		}
	}
}

// The count is of CASCADES, not of swallows, and not of cascades that
// merely ran.
func TestR187_CountsCascadesNotSwallows(t *testing.T) {
	r187reset(t)
	gs := r187gs(40)
	r187run(gs, 3, 10)

	if got := gs.DepthCapSwallowingCascadeCount(); got != 3 {
		t.Fatalf("cascades = %d, want 3 — three top-level cascades "+
			"swallowed, ten dispatches each", got)
	}
	if got := gs.DepthCapSwallowedCount(); got != 30 {
		t.Fatalf("swallowed = %d, want 30 (control: the total is "+
			"unchanged by this round)", got)
	}
	if got := gs.DepthCapCascadeMaxCount(); got != 10 {
		t.Fatalf("cascademax = %d, want 10 (control)", got)
	}
}

// NoteCascadeRoot fires at depth 1 of EVERY cascade, and the
// overwhelming majority swallow nothing. Those must not be counted,
// or the field measures how much the game played rather than what the
// cap cost it.
func TestR187_SilentCascadesAreNotCounted(t *testing.T) {
	r187reset(t)
	gs := r187gs(40)
	for i := 0; i < 500; i++ {
		gs.NoteCascadeRoot()
	}
	if got := gs.DepthCapSwallowingCascadeCount(); got != 0 {
		t.Fatalf("cascades = %d, want 0 — 500 cascades that swallowed "+
			"nothing", got)
	}
	// And one that does swallow, after all that silence, counts once.
	gs.NoteCascadeRoot()
	gs.NoteDepthCapSwallow("creature_dies")
	gs.NoteDepthCapSwallow("creature_dies")
	if got := gs.DepthCapSwallowingCascadeCount(); got != 1 {
		t.Fatalf("cascades = %d, want 1 — one swallowing cascade among "+
			"501", got)
	}
}

// THE PLACEMENT TEST. A game's LAST swallowing cascade is never
// followed by another top-level cascade, so anything that counts at
// the start of the NEXT cascade misses it. In the 82% of production
// capped games that have exactly one swallowing cascade, that is the
// entire measurement.
func TestR187_TheLastCascadeOfAGameIsCounted(t *testing.T) {
	r187reset(t)
	gs := r187gs(40)
	gs.NoteCascadeRoot()
	for i := 0; i < 11; i++ {
		gs.NoteDepthCapSwallow("nonland_permanent_etb")
	}
	// Game ends here. No further NoteCascadeRoot, ever.
	if got := gs.DepthCapSwallowingCascadeCount(); got != 1 {
		t.Fatalf("cascades = %d, want 1 — the only swallowing cascade of "+
			"the game was its last, and counting it must not depend on a "+
			"later cascade arriving to observe it", got)
	}
}

// THE POINT OF THE ROUND. Two games swallow exactly the same number
// of dispatches. One is a single wide chain; the other is fan-out,
// which in production is where all the wall-clock cost lives. swallowed=
// is identical, cascademax= differs, and only cascades= says how many
// chains there were without dividing one field by another.
func TestR187_FanOutAndOneWideChainAreOppositeShapes(t *testing.T) {
	r187reset(t)

	wide := r187gs(40)
	r187run(wide, 1, 3122)

	fan := r187gs(40)
	r187run(fan, 446, 7)

	if wide.DepthCapSwallowedCount() != 3122 {
		t.Fatalf("premise: wide swallowed %d, want 3122",
			wide.DepthCapSwallowedCount())
	}
	if fan.DepthCapSwallowedCount() != 3122 {
		t.Fatalf("premise: fan swallowed %d, want 3122 — the two games "+
			"must be indistinguishable by the EXISTING total or this "+
			"test proves nothing", fan.DepthCapSwallowedCount())
	}
	if got := wide.DepthCapSwallowingCascadeCount(); got != 1 {
		t.Fatalf("wide cascades = %d, want 1", got)
	}
	if got := fan.DepthCapSwallowingCascadeCount(); got != 446 {
		t.Fatalf("fan cascades = %d, want 446 — counted, not inferred: "+
			"swallowed/cascademax would read 3122/7 = 446 only because "+
			"this fixture made every cascade the same size, which real "+
			"fan-out never does", got)
	}
}

// The count is per GAME. Two GameStates in the same process must not
// see each other's cascades.
func TestR187_IsPerGameNotProcessWide(t *testing.T) {
	r187reset(t)
	a := r187gs(10)
	r187run(a, 4, 2)
	b := r187gs(10)
	r187run(b, 1, 2)

	if got := a.DepthCapSwallowingCascadeCount(); got != 4 {
		t.Fatalf("game A cascades = %d, want 4", got)
	}
	if got := b.DepthCapSwallowingCascadeCount(); got != 1 {
		t.Fatalf("game B cascades = %d, want 1 — A's four must not leak "+
			"into B", got)
	}
	if got := DepthCapCascadesMaxPerGame(); got != 4 {
		t.Fatalf("process-wide max = %d, want 4 — the maximum over "+
			"games, not the sum", got)
	}
}

// The grinder field is a per-game MAXIMUM and it renders where the
// other per-game maxima do.
func TestR187_SummaryCarriesThePerGameMaximum(t *testing.T) {
	r187reset(t)
	r187run(r187gs(10), 7, 1)
	r187run(r187gs(10), 2, 1)

	got := DepthCapSummary()
	if !strings.Contains(got, ",cascadesmax:7,") {
		t.Fatalf("summary = %q, want it to carry cascadesmax:7 — the "+
			"largest per-game count, not the last and not the sum", got)
	}
	iCM := strings.Index(got, ",cascademax:")
	iCS := strings.Index(got, ",cascadesmax:")
	iMixed := strings.Index(got, ",mixed:")
	if iCM < 0 || iCS < 0 || iMixed < 0 || !(iCM < iCS && iCS < iMixed) {
		t.Fatalf("summary = %q, want cascademax: then cascadesmax: then "+
			"mixed: — the per-game maxima stay grouped", got)
	}
}

// The process-wide maximum must be reset with the rest of the family,
// or one test's fan-out game poisons every later assertion.
func TestR187_ResetClearsTheProcessWideMaximum(t *testing.T) {
	r187reset(t)
	r187run(r187gs(10), 9, 1)
	if got := DepthCapCascadesMaxPerGame(); got != 9 {
		t.Fatalf("premise: max = %d, want 9", got)
	}
	ResetDepthCapForTest()
	if got := DepthCapCascadesMaxPerGame(); got != 0 {
		t.Fatalf("max = %d after ResetDepthCapForTest, want 0 — the r187 "+
			"atomic must reset with the family it belongs to", got)
	}
}

// Mirrors r182/r185: nil is zero, and reading is read-only.
func TestR187_NilIsZeroAndReadingDoesNotMutate(t *testing.T) {
	r187reset(t)
	var nilGS *GameState
	if got := nilGS.DepthCapSwallowingCascadeCount(); got != 0 {
		t.Fatalf("nil receiver returned %d, want 0", got)
	}
	gs := r187gs(10)
	r187run(gs, 3, 2)
	for i := 0; i < 5; i++ {
		if got := gs.DepthCapSwallowingCascadeCount(); got != 3 {
			t.Fatalf("read %d returned %d, want a stable 3", i, got)
		}
	}
}

// GAP 2. The four-slot table name-capped 64% of production cascades,
// so distinct= was the saturated lower bound 5 in two thirds of the
// data and dom% — computed over tracked slots only — was unreliable
// there. At the widened size those same cascades are exact.
func TestR187_NameTableHoldsTheObservedCardinality(t *testing.T) {
	r187reset(t)
	// Five distinct names is the only cardinality the production
	// namecap= actually establishes a cascade can reach. It must now
	// be exact rather than capped.
	gs := r187gs(100)
	gs.NoteCascadeRoot()
	for i := 0; i < 5; i++ {
		gs.NoteDepthCapSwallow("e" + strconv.Itoa(i))
	}
	gs.NoteCascadeRoot() // finish it
	if gs.cascadePeak.capped {
		t.Fatalf("five distinct events still name-capped the table — the "+
			"size that motivated this round (%d) did not take effect",
			cascadeCompSlots)
	}
	if _, distinct, _, _ := gs.DepthCapPeakCascadeShape(); distinct != 5 {
		t.Fatalf("distinct = %d, want an exact 5", distinct)
	}
	if _, _, nameCapped := CascadeShapeStats(); nameCapped != 0 {
		t.Fatalf("namecap = %d, want 0", nameCapped)
	}

	// And the table is still BOUNDED: it is a fixed array, so one
	// name past the end overflows instead of growing.
	ResetCascadeShapeForTest()
	gs2 := r187gs(100)
	gs2.NoteCascadeRoot()
	for i := 0; i <= cascadeCompSlots; i++ {
		gs2.NoteDepthCapSwallow("f" + strconv.Itoa(i))
	}
	gs2.NoteCascadeRoot()
	if !gs2.cascadePeak.capped {
		t.Fatalf("%d distinct events did not trip a %d-slot table — the "+
			"ceiling must stay visible, not be removed",
			cascadeCompSlots+1, cascadeCompSlots)
	}
	if _, _, nameCapped := CascadeShapeStats(); nameCapped != 1 {
		t.Fatalf("namecap = %d, want 1", nameCapped)
	}
}
