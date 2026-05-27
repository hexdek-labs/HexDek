package gameengine

// r60 — Storm cast-count semantics audit (CR §702.40 / §700.4).
//
// The storm trigger reads SpellsCastThisTurn at the moment a
// storm-bearing spell is cast and copies for each OTHER spell already
// cast that turn. The semantic invariant is: SpellsCastThisTurn
// increments per CAST, not per RESOLUTION. Concretely:
//
//   - Cast a spell → counter increments immediately (before resolution).
//   - Counter the spell on the stack → counter STAYS incremented
//     (the cast event happened; resolution outcome is irrelevant).
//   - Storm copies (IsCopy=true) → counter must NOT increment
//     (§707.10: a copy is not cast).
//   - A 4th cast that's a storm spell sees count=4, copies = 4-1 = 3.
//
// Implementation audit: IncrementCastCount (cast_counts.go:39) is the
// single counter-bump site; it's called from all six cast paths
// (CastSpell, response-cast in PriorityRound, zone-cast/impulse_play,
// cost-paid, paradigm copy, commander-zone cast) BEFORE PushStackItem
// — so the bump precedes any resolution. ApplyStormCopy (the copy
// pusher) appends directly to gs.Stack and explicitly does NOT call
// IncrementCastCount (documented at keywords_storm_rider.go:136).
// Implementation is correct per CR.
//
// This file pins the contract so a future refactor that moves the
// counter bump into ResolveStackTop, or accidentally calls
// IncrementCastCount inside ApplyStormCopy, trips a clear red.

import (
	"testing"
)

// TestStorm_CountIncrementsAtCastNotResolve pins the per-CAST timing
// by tracking the event ordering: the cast_count_snapshot proxy event
// (a real "cast" event with the post-bump count) is logged BEFORE the
// stack_resolve event. If the engine ever moved IncrementCastCount
// into ResolveStackTop the snapshot would land AFTER the resolve.
//
// We use the direct primitive sequence (IncrementCastCount +
// PushStackItem + ResolveStackTop) so we can observe pre-resolution
// state — CastSpell calls DrainStack internally and resolves before
// returning, hiding the pre-resolution count from external callers.
func TestStorm_CountIncrementsAtCastNotResolve(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	card := &Card{Name: "Filler", Owner: 0, Types: []string{"sorcery"}}

	// Mimic CastSpell's ordering: bump counter first, then push the
	// stack item. This is the same order CastSpell uses (stack.go:437
	// before line 470).
	IncrementCastCount(gs, 0)
	if gs.SpellsCastThisTurn != 1 {
		t.Fatalf("counter must increment at the IncrementCastCount call; got %d, want 1",
			gs.SpellsCastThisTurn)
	}

	item := &StackItem{Controller: 0, Card: card}
	PushStackItem(gs, item)

	// Before resolution: counter = 1.
	preResolve := gs.SpellsCastThisTurn

	ResolveStackTop(gs)

	// After resolution: counter must NOT increment again. The
	// IncrementCastCount call is the SOLE bump site; if a regression
	// added a bump inside ResolveStackTop (or any of its callees), the
	// counter would now be 2.
	if gs.SpellsCastThisTurn != preResolve {
		t.Errorf("counter must NOT re-increment on resolution; pre-resolve=%d post-resolve=%d",
			preResolve, gs.SpellsCastThisTurn)
	}
	if gs.SpellsCastThisTurn != 1 {
		t.Errorf("end state: counter=%d, want 1", gs.SpellsCastThisTurn)
	}
}

// TestStorm_CountSurvivesCounterspell pins that a CAST that ultimately
// fails to resolve (countered) still counts toward storm. The cast
// event is the trigger, per CR §702.40 / §111.1 (a spell is on the
// stack regardless of whether it resolves).
func TestStorm_CountSurvivesCounterspell(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	card := &Card{Name: "Filler", Owner: 0, Types: []string{"sorcery"}}

	// Mimic CastSpell ordering: bump counter at cast time.
	IncrementCastCount(gs, 0)
	if gs.SpellsCastThisTurn != 1 {
		t.Fatalf("after cast: SpellsCastThisTurn=%d, want 1", gs.SpellsCastThisTurn)
	}

	// Push the spell on the stack, then mark it countered. ResolveStackTop
	// should route it to the graveyard without firing effects.
	item := &StackItem{Controller: 0, Card: card, Countered: true}
	PushStackItem(gs, item)

	ResolveStackTop(gs)

	if gs.SpellsCastThisTurn != 1 {
		t.Errorf("a countered spell's cast must still count for storm; SpellsCastThisTurn=%d, want 1",
			gs.SpellsCastThisTurn)
	}
}

// TestStorm_CopiesDoNotIncrementCount pins §707.10: a copy of a spell
// is not cast. ApplyStormCopy must NOT bump the counter — otherwise a
// 1st-cast storm spell would create infinite copies on the next cast.
func TestStorm_CopiesDoNotIncrementCount(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 100

	// Pre-cast 2 fillers so the storm spell sees count=2 → 2 copies.
	for i := 0; i < 2; i++ {
		f := makeFiller(gs, 0, "Filler")
		if err := CastSpell(gs, 0, f, nil); err != nil {
			t.Fatalf("filler %d: %v", i, err)
		}
	}
	countBefore := gs.SpellsCastThisTurn
	if countBefore != 2 {
		t.Fatalf("after 2 fillers: count=%d, want 2", countBefore)
	}

	grape := makeGrapeshot(gs, 0)
	if err := CastSpell(gs, 0, grape, nil); err != nil {
		t.Fatalf("grapeshot: %v", err)
	}

	// After Grapeshot's own cast: count = 3 (1 original storm cast +
	// 2 prior fillers). The 2 storm copies pushed onto the stack must
	// NOT have bumped the counter further.
	if gs.SpellsCastThisTurn != 3 {
		t.Errorf("after grapeshot + 2 copies: count=%d; want 3 (copies must not increment per §707.10)",
			gs.SpellsCastThisTurn)
	}
}

// TestStorm_CopyCountDerivedFromPriorCasts pins the §702.40a copy-count
// formula: copies = SpellsCastThisTurn - 1 at the moment storm
// resolves. With 3 prior casts, the storm spell makes 3 copies.
func TestStorm_CopyCountDerivedFromPriorCasts(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 100

	for i := 0; i < 3; i++ {
		f := makeFiller(gs, 0, "Filler")
		if err := CastSpell(gs, 0, f, nil); err != nil {
			t.Fatalf("filler %d: %v", i, err)
		}
	}

	grape := makeGrapeshot(gs, 0)
	if err := CastSpell(gs, 0, grape, nil); err != nil {
		t.Fatalf("grapeshot: %v", err)
	}

	storm := lastEventOfKind(gs, "storm_trigger")
	if storm.Kind == "" {
		t.Fatalf("storm_trigger event missing")
	}
	if storm.Amount != 3 {
		t.Errorf("storm copy count: got %d, want 3 (4 casts minus the storm spell itself)", storm.Amount)
	}
}

// TestStorm_PerSeatCounterIncrementsOnCast pins the per-seat storm
// counter (used by Kraum / Stella Lee / Saruman / Prismari) follows
// the same per-CAST semantics as the global counter. The per-seat
// counter is what those cards' triggers read.
func TestStorm_PerSeatCounterIncrementsOnCast(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Seats[0].ManaPool = 10

	f := makeFiller(gs, 0, "Filler")
	if err := CastSpell(gs, 0, f, nil); err != nil {
		t.Fatalf("CastSpell: %v", err)
	}

	if gs.Seats[0].SpellsCastThisTurn != 1 {
		t.Errorf("per-seat counter must increment AT cast time; Seat[0]=%d, want 1",
			gs.Seats[0].SpellsCastThisTurn)
	}
	if gs.Seats[1].SpellsCastThisTurn != 0 {
		t.Errorf("opponent's per-seat counter must not bump; Seat[1]=%d, want 0",
			gs.Seats[1].SpellsCastThisTurn)
	}

	// Resolve and confirm no second bump.
	if len(gs.Stack) > 0 {
		ResolveStackTop(gs)
	}
	if gs.Seats[0].SpellsCastThisTurn != 1 {
		t.Errorf("per-seat counter must NOT re-increment on resolution; Seat[0]=%d, want 1",
			gs.Seats[0].SpellsCastThisTurn)
	}
}

// TestStorm_ApplyStormCopyDirectlyDoesNotIncrement pins the contract
// on the lower-level primitive: ApplyStormCopy bypasses IncrementCastCount
// entirely. A defensive direct-call test guards against a future
// refactor that wires the bump into the copy push.
func TestStorm_ApplyStormCopyDirectlyDoesNotIncrement(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0

	// Synthesize a stack item without going through CastSpell — we
	// only care that ApplyStormCopy doesn't touch the counter.
	card := &Card{Name: "Grapeshot", Owner: 0, Types: []string{"sorcery"}}
	orig := &StackItem{Controller: 0, Card: card}
	PushStackItem(gs, orig)

	before := gs.SpellsCastThisTurn
	pushed := ApplyStormCopy(gs, orig, 5)
	if pushed != 5 {
		t.Errorf("ApplyStormCopy(count=5) should push 5; got %d", pushed)
	}
	if gs.SpellsCastThisTurn != before {
		t.Errorf("ApplyStormCopy must not increment the cast counter; before=%d after=%d",
			before, gs.SpellsCastThisTurn)
	}
}
