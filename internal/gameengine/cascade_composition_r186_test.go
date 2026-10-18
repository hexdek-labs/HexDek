package gameengine

import "testing"

// r186 — per-cascade composition.
//
// WHAT THESE PIN. The whole point of the round is that counting event
// NAMES cannot separate a self-feeding loop from a wide finite wave:
// both carry exactly one distinct event. The separator is the second
// axis — swallows versus the battlefield population the cascade ran
// against — so the load-bearing pair here is
// TestR186_WideWaveIsNotALoop and TestR186_SelfLoopOutrunsTheBoard,
// which feed the SAME event name the SAME number of distinct times
// and must come out opposite.
//
// MUTATION CONTROLS (RED counts are per package, gameengine/per_card):
//
//	M1  noteCascadeEvent stops sampling the board (boardMax stays 0)
//	    -> predicted 7/2
//	M2  classify uses `total < c.boardMax` instead of `<=`
//	    -> predicted 1/0 (BoardMaxTracksGrowth is built to sit exactly
//	       on the boundary; every other bounded case has slack)
//	M3  delete the notePeakCascade call in NoteDepthCapSwallow
//	    -> predicted 7/3 (every test that reads the per-game peak)
//	M4  NoteCascadeRoot stops calling finishCascade
//	    -> predicted 3/0 (the process-wide histogram only)
//	M5  classify drops the `if c.capped { distinct++ }`
//	    -> predicted 1/0 (NameTableIsBoundedAndVisible)
//	M6  NoteCascadeRoot stops resetting cascadeCur (constraint 4)
//	    -> predicted 2/0
//
// Actuals and every miss are in the round report.

func r186reset(t *testing.T) {
	t.Helper()
	reset := func() {
		ResetDepthCapForTest()
		ResetCascadeShapeForTest()
		ResetLoopGuardLabelsForTest()
	}
	reset()
	t.Cleanup(reset)
}

// r186gs builds a GameState whose battlefield holds `board`
// permanents. battlefieldPopulation reads only slice lengths, so the
// entries may be nil.
func r186gs(board int) *GameState {
	return &GameState{Seats: []*Seat{{Battlefield: make([]*Permanent, board)}}}
}

// (c) A wide wave: 30 permanents' worth of the same event, on a board
// of 40, inside one cascade. Nothing re-entered — the cascade could
// not have swallowed more than the board could supply — so this is
// the cap sitting below a legitimately wide board, not a loop.
func TestR186_WideWaveIsNotALoop(t *testing.T) {
	r186reset(t)
	gs := r186gs(40)
	gs.NoteCascadeRoot()
	for i := 0; i < 30; i++ {
		gs.NoteDepthCapSwallow("nonland_permanent_etb")
	}

	shape, distinct, domPct, boardMax := gs.DepthCapPeakCascadeShape()
	if shape != "wave" {
		t.Fatalf("shape = %q, want \"wave\": 30 swallows on a board of 40 "+
			"cannot be unbounded re-entry", shape)
	}
	if distinct != 1 || domPct != 100 || boardMax != 40 {
		t.Fatalf("distinct=%d dom=%d%% board=%d, want 1/100/40",
			distinct, domPct, boardMax)
	}
}

// (a) A self-feeding loop: the SAME event name and the SAME distinct
// count as the wave above, but 50 swallows against a board of 3. No
// wave of 3 permanents produces 50 dispatches, so something re-entered.
//
// This test and the one above are the round's reason for existing. If
// they ever agree, the measurement has collapsed back into capfirst=.
func TestR186_ReentryOutrunsTheBoard(t *testing.T) {
	r186reset(t)
	gs := r186gs(3)
	gs.NoteCascadeRoot()
	for i := 0; i < 50; i++ {
		gs.NoteDepthCapSwallow("nonland_permanent_etb")
	}

	shape, distinct, domPct, boardMax := gs.DepthCapPeakCascadeShape()
	if shape != "loop1" {
		t.Fatalf("shape = %q, want \"loop1\": 50 swallows on a board of "+
			"3 cannot be a one-dispatch-per-permanent wave", shape)
	}
	if distinct != 1 {
		t.Fatalf("distinct = %d, want 1 — this is the axis that CANNOT "+
			"separate this case from a wave, which is why the board axis "+
			"exists", distinct)
	}
	if domPct != 100 || boardMax != 3 {
		t.Fatalf("dom=%d%% board=%d, want 100/3", domPct, boardMax)
	}
}

// (b) Several event names among the SWALLOWED dispatches — the
// Gerrard/Yarok pairing when its branches are unequal. Same
// unbounded-versus-board verdict as loop1, different first axis, and
// the dominant share near 50% is what says "pairing" rather than "one
// event with a sprinkle of a second".
//
// See TestR186_StrictAlternationIsIndistinguishableFromSelfFeeding in
// internal/gameengine/per_card for the case this axis CANNOT catch.
func TestR186_MultiEventReentryIsItsOwnCell(t *testing.T) {
	r186reset(t)
	gs := r186gs(3)
	gs.NoteCascadeRoot()
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			gs.NoteDepthCapSwallow("creature_dies")
		} else {
			gs.NoteDepthCapSwallow("nonland_permanent_etb")
		}
	}

	shape, distinct, domPct, _ := gs.DepthCapPeakCascadeShape()
	if shape != "loopN" {
		t.Fatalf("shape = %q, want \"loopN\"", shape)
	}
	if distinct != 2 {
		t.Fatalf("distinct = %d, want 2", distinct)
	}
	if domPct != 50 {
		t.Fatalf("dom = %d%%, want 50 — an alternating pair is balanced, "+
			"and that is what separates it from a lopsided cascade", domPct)
	}
}

// Mixed events can still be a bounded wave: a simultaneous ETB wave
// that also kills something produces two event names without anything
// re-entering. The board axis keeps this out of "loopN".
func TestR186_WideMixIsBoundedByTheBoard(t *testing.T) {
	r186reset(t)
	gs := r186gs(40)
	gs.NoteCascadeRoot()
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			gs.NoteDepthCapSwallow("creature_dies")
		} else {
			gs.NoteDepthCapSwallow("nonland_permanent_etb")
		}
	}

	shape, distinct, _, _ := gs.DepthCapPeakCascadeShape()
	if shape != "widemix" {
		t.Fatalf("shape = %q, want \"widemix\": 20 swallows on a board of "+
			"40 is bounded however many event names it carries", shape)
	}
	if distinct != 2 {
		t.Fatalf("distinct = %d, want 2", distinct)
	}
}

// The per-game verdict must describe the PEAK cascade, not the last
// one — the same property r185 pinned for the size, now for the shape.
// A game that loops once and then has a quiet wide cascade must still
// report the loop.
func TestR186_PeakCascadeIsTheBigOneNotTheLast(t *testing.T) {
	r186reset(t)
	gs := r186gs(3)
	gs.NoteCascadeRoot()
	for i := 0; i < 40; i++ {
		gs.NoteDepthCapSwallow("creature_dies")
	}
	// Second cascade: wide and small, on a bigger board.
	gs.Seats[0].Battlefield = make([]*Permanent, 60)
	gs.NoteCascadeRoot()
	for i := 0; i < 5; i++ {
		gs.NoteDepthCapSwallow("nonland_permanent_etb")
	}

	shape, distinct, _, boardMax := gs.DepthCapPeakCascadeShape()
	if shape != "loop1" || distinct != 1 || boardMax != 3 {
		t.Fatalf("peak = %s/%d/board %d, want loop1/1/board 3 — the "+
			"40-swallow cascade is the peak, not the 5-swallow one",
			shape, distinct, boardMax)
	}
	if got := gs.DepthCapCascadeMaxCount(); got != 40 {
		t.Fatalf("premise stale: cascademax = %d, want 40", got)
	}
}

// A cascade that GROWS its own board — a token wave, Gerrard
// mass-returning creatures — must not be called a loop just because
// the board was small when it started. boardMax is a high-water mark
// over the cascade, and the comparison is `<=`, so a cascade that
// swallows exactly as many dispatches as it ever had permanents is
// still bounded.
//
// Built to sit EXACTLY on the boundary: 4 swallows, boardMax 4.
func TestR186_BoardMaxTracksGrowthDuringTheCascade(t *testing.T) {
	r186reset(t)
	gs := r186gs(0)
	gs.NoteCascadeRoot()
	for i := 1; i <= 4; i++ {
		gs.Seats[0].Battlefield = make([]*Permanent, i)
		gs.NoteDepthCapSwallow("nonland_permanent_etb")
	}

	shape, _, _, boardMax := gs.DepthCapPeakCascadeShape()
	if boardMax != 4 {
		t.Fatalf("boardMax = %d, want 4 — the high-water mark must follow "+
			"the board the cascade creates, not the board it started on",
			boardMax)
	}
	if shape != "wave" {
		t.Fatalf("shape = %q, want \"wave\": 4 swallows against a board "+
			"that reached 4 is bounded, and the bound is inclusive",
			shape)
	}
}

// The name table is a fixed array. The fifth distinct event in one
// cascade is not stored; the cascade is marked and the ceiling shows
// up in the rendered output, exactly as r162's "~other" does.
func TestR186_NameTableIsBoundedAndVisible(t *testing.T) {
	r186reset(t)
	gs := r186gs(2)
	gs.NoteCascadeRoot()
	for _, e := range []string{"e0", "e1", "e2", "e3", "e4", "e5", "e6", "e7", "e8"} {
		gs.NoteDepthCapSwallow(e)
	}
	gs.NoteCascadeRoot() // finishes the cascade

	if !gs.cascadePeak.capped {
		t.Fatalf("nine distinct events in one cascade did not trip the "+
			"%d-slot name table", cascadeCompSlots)
	}
	_, distinct, _, _ := gs.DepthCapPeakCascadeShape()
	if distinct != cascadeCompSlots+1 {
		t.Fatalf("distinct = %d, want %d — a name-capped cascade reports "+
			"slots+1 as an explicit lower bound, not the slot count as if "+
			"it were exact", distinct, cascadeCompSlots+1)
	}
	if _, _, nameCapped := CascadeShapeStats(); nameCapped != 1 {
		t.Fatalf("namecap = %d, want 1", nameCapped)
	}
	if got := DepthCapShapeSummary(); got != "loopN:1,namecap:1" {
		t.Fatalf("summary = %q, want %q — the ceiling must be visible in "+
			"the log line, not only in the struct", got, "loopN:1,namecap:1")
	}
}

// A multi-event cascade where one event carries ~everything is not a
// pairing, and the lopsided counter is what says so.
func TestR186_LopsidedFlagsAnUnbalancedPair(t *testing.T) {
	r186reset(t)
	gs := r186gs(2)
	gs.NoteCascadeRoot()
	for i := 0; i < 19; i++ {
		gs.NoteDepthCapSwallow("creature_dies")
	}
	gs.NoteDepthCapSwallow("nonland_permanent_etb")
	gs.NoteCascadeRoot()

	_, lopsided, _ := CascadeShapeStats()
	if lopsided != 1 {
		t.Fatalf("lopsided = %d, want 1 — 19 of 20 swallows on one event "+
			"is not a pairing", lopsided)
	}

	// A balanced pair must NOT be counted.
	r186reset(t)
	gs = r186gs(2)
	gs.NoteCascadeRoot()
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			gs.NoteDepthCapSwallow("creature_dies")
		} else {
			gs.NoteDepthCapSwallow("nonland_permanent_etb")
		}
	}
	gs.NoteCascadeRoot()
	if _, lopsided, _ = CascadeShapeStats(); lopsided != 0 {
		t.Fatalf("lopsided = %d, want 0 for a 50/50 alternation", lopsided)
	}
}

// NoteCascadeRoot must reset the composition, not just the counter —
// otherwise a later small cascade inherits the previous cascade's
// event names and board high-water mark and is classified as part of it.
func TestR186_CascadeRootResetsTheComposition(t *testing.T) {
	r186reset(t)
	gs := r186gs(50)
	gs.NoteCascadeRoot()
	for i := 0; i < 10; i++ {
		gs.NoteDepthCapSwallow("creature_dies")
	}
	gs.Seats[0].Battlefield = make([]*Permanent, 1)
	gs.NoteCascadeRoot()

	if gs.cascadeCur.used != 0 || gs.cascadeCur.boardMax != 0 || gs.cascadeCur.overflow != 0 {
		t.Fatalf("composition survived the cascade boundary: %+v", gs.cascadeCur)
	}
	gs.NoteDepthCapSwallow("nonland_permanent_etb")
	if gs.cascadeCur.used != 1 || gs.cascadeCur.names[0] != "nonland_permanent_etb" {
		t.Fatalf("second cascade inherited the first's names: %+v", gs.cascadeCur)
	}
	if gs.cascadeCur.boardMax != 1 {
		t.Fatalf("boardMax = %d, want 1 — the high-water mark is per "+
			"cascade, not per game", gs.cascadeCur.boardMax)
	}
}

// The grinder field renders cascades sorted heaviest first, ties by
// name, zero cells omitted, and nothing at all when the cap never fired.
func TestR186_SummaryRendersSortedAndOmitsZeroes(t *testing.T) {
	r186reset(t)
	if got := DepthCapShapeSummary(); got != "" {
		t.Fatalf("summary = %q on a clean process, want empty so the "+
			"grinder omits the field", got)
	}

	// Two waves, one loop1, one lopsided loopN.
	gs := r186gs(40)
	for c := 0; c < 2; c++ {
		gs.NoteCascadeRoot()
		for i := 0; i < 5; i++ {
			gs.NoteDepthCapSwallow("nonland_permanent_etb")
		}
	}
	gs.Seats[0].Battlefield = make([]*Permanent, 2)
	gs.NoteCascadeRoot()
	for i := 0; i < 30; i++ {
		gs.NoteDepthCapSwallow("creature_dies")
	}
	gs.NoteCascadeRoot()
	for i := 0; i < 19; i++ {
		gs.NoteDepthCapSwallow("creature_dies")
	}
	gs.NoteDepthCapSwallow("permanent_ltb")
	gs.NoteCascadeRoot() // finishes the last one

	const want = "wave:2,loop1:1,loopN:1,lopsided:1"
	if got := DepthCapShapeSummary(); got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

// A nil GameState and a game that never capped must both report
// nothing, so a caller cannot print a fabricated verdict. Same
// contract as r182/r185's accessors.
func TestR186_NilAndCleanGameReportNothing(t *testing.T) {
	r186reset(t)
	var nilGS *GameState
	if shape, d, p, b := nilGS.DepthCapPeakCascadeShape(); shape != "" || d != 0 || p != 0 || b != 0 {
		t.Fatalf("nil receiver returned %q/%d/%d/%d, want all zero", shape, d, p, b)
	}
	gs := r186gs(10)
	gs.NoteCascadeRoot()
	if shape, _, _, _ := gs.DepthCapPeakCascadeShape(); shape != "" {
		t.Fatalf("a game that never capped reported shape %q", shape)
	}
}
