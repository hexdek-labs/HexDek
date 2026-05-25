package gameengine

// R60 — §514 cleanup-step timing edge cases.
//
// CR §514 has three structural rules that the existing test suite
// does NOT pin in isolation:
//
//   §514.2 — cleanup runs as a single simultaneous action AND
//            explicitly covers phased-out permanents
//            ("all damage marked on permanents (including phased-out
//             permanents) is removed").
//
//   §514.3a — cleanup may run more than once per turn if triggered
//            abilities or SBAs fire mid-cleanup. The second pass
//            must not double-emit cleanup events for already-drained
//            state. Idempotency is load-bearing.
//
//   §513    — the end step is a SEPARATE step before cleanup.
//            `DurationUntilNextEndStep` effects expire at end step;
//            `DurationEndOfTurn` effects expire at cleanup. These
//            two duration tags must NOT cross-fire each other's
//            boundary.
//
// Gaps before this file:
//   - No test calls ScanExpiredDurations twice at cleanup to pin
//     idempotency (a regression that re-emitted damage_wears_off on
//     a second pass would silently double-count audit events under
//     §514.3a cascade).
//   - No test pins that DurationEndOfTurn ContinuousEffects survive
//     the end step and DurationUntilNextEndStep ContinuousEffects
//     survive cleanup.
//   - No test pins the §514.2 phased-out permanent damage clear —
//     existing damage tests use unphased perms.
//
// Rule citations:
//   §513    — end step
//   §514.2  — cleanup actions (incl. phased-out permanents)
//   §514.3a — cascading cleanup
//   §702.26 — phasing

import (
	"testing"
)

// -----------------------------------------------------------------------------
// §514.3a — cleanup idempotency.
//
// A trigger that fires during cleanup (e.g. "at the beginning of the
// next cleanup step") puts an ability on the stack, gives priority,
// then another cleanup begins. The second cleanup must not re-process
// state that was already drained: no second damage_wears_off, no
// double Modifications strip, no event spam.
// -----------------------------------------------------------------------------

func TestCleanup_Idempotent_SecondPassIsNoOp(t *testing.T) {
	gs := newFixtureGame(t)
	bear := addBattlefield(gs, 0, "Runeclaw Bear", 2, 2, "creature")
	bear.MarkedDamage = 1
	bear.Modifications = append(bear.Modifications,
		Modification{Power: 3, Toughness: 3, Duration: "until_end_of_turn", Timestamp: gs.NextTimestamp()},
	)

	// First cleanup pass — drains everything.
	ScanExpiredDurations(gs, "ending", "cleanup")
	if bear.MarkedDamage != 0 {
		t.Fatalf("first pass should clear MarkedDamage; got %d", bear.MarkedDamage)
	}
	if len(bear.Modifications) != 0 {
		t.Fatalf("first pass should strip EOT Modification; got %d remaining", len(bear.Modifications))
	}
	firstWearOff := countEvents(gs, "damage_wears_off")
	if firstWearOff != 1 {
		t.Fatalf("first pass should emit exactly one damage_wears_off; got %d", firstWearOff)
	}

	// Second cleanup pass — must be a no-op for the already-drained
	// state. §514.3a cascade safety.
	ScanExpiredDurations(gs, "ending", "cleanup")
	if got := countEvents(gs, "damage_wears_off"); got != firstWearOff {
		t.Errorf("second cleanup pass must not re-emit damage_wears_off; got %d (was %d)",
			got, firstWearOff)
	}
	if bear.MarkedDamage != 0 {
		t.Errorf("second pass must not introduce damage; got %d", bear.MarkedDamage)
	}
	if len(bear.Modifications) != 0 {
		t.Errorf("second pass must not introduce modifications; got %d", len(bear.Modifications))
	}
}

// -----------------------------------------------------------------------------
// §513 vs §514 — end-step and cleanup-step durations are distinct.
//
// `DurationUntilNextEndStep` ContinuousEffects expire at end step.
// `DurationEndOfTurn` ContinuousEffects expire at cleanup. A regression
// that conflated them would either:
//   - expire EOT effects one step too early (cards like Giant Growth
//     would wear off before the end-step-trigger window), or
//   - expire end-step-bounded effects one step too late (drift past
//     when the affected window should have closed).
// -----------------------------------------------------------------------------

func TestCleanup_DurationDiscrimination_EndStepVsCleanup(t *testing.T) {
	gs := newFixtureGame(t)

	eotEffect := &ContinuousEffect{
		Layer:          LayerPT,
		Sublayer:       "c",
		Timestamp:      gs.NextTimestamp(),
		HandlerID:      "test:eot",
		Duration:       DurationEndOfTurn,
		ControllerSeat: 0,
		Predicate:      func(_ *GameState, _ *Permanent) bool { return false },
		ApplyFn:        func(_ *GameState, _ *Permanent, _ *Characteristics) {},
	}
	endStepEffect := &ContinuousEffect{
		Layer:          LayerPT,
		Sublayer:       "c",
		Timestamp:      gs.NextTimestamp(),
		HandlerID:      "test:end_step",
		Duration:       DurationUntilNextEndStep,
		ControllerSeat: 0,
		Predicate:      func(_ *GameState, _ *Permanent) bool { return false },
		ApplyFn:        func(_ *GameState, _ *Permanent, _ *Characteristics) {},
	}
	gs.ContinuousEffects = append(gs.ContinuousEffects, eotEffect, endStepEffect)

	// End step — should expire ONLY the until_next_end_step effect.
	ScanExpiredDurations(gs, "ending", "end_step")
	remaining := map[string]bool{}
	for _, ce := range gs.ContinuousEffects {
		remaining[ce.HandlerID] = true
	}
	if !remaining["test:eot"] {
		t.Error("EOT effect must survive end step — only expires at cleanup")
	}
	if remaining["test:end_step"] {
		t.Error("until_next_end_step effect must expire at end step")
	}

	// Cleanup step — should expire the remaining EOT effect.
	ScanExpiredDurations(gs, "ending", "cleanup")
	if len(gs.ContinuousEffects) != 0 {
		t.Errorf("after cleanup all test effects should be gone; got %d remaining: %+v",
			len(gs.ContinuousEffects), gs.ContinuousEffects)
	}
}

// -----------------------------------------------------------------------------
// §514.2 explicit clause — damage on phased-out permanents clears.
//
// CR §514.2: "all damage marked on permanents (including phased-out
// permanents) is removed". A `PhasedOut == true` permanent must still
// have its MarkedDamage cleared and emit a `damage_wears_off` event.
// The existing damage tests all use unphased perms, so this clause
// has been unpinned.
// -----------------------------------------------------------------------------

func TestCleanup_PhasedOutPermanentDamageClears_514_2(t *testing.T) {
	gs := newFixtureGame(t)
	ghost := addBattlefield(gs, 0, "Teferi's Imp", 2, 2, "creature")
	ghost.MarkedDamage = 1
	ghost.PhasedOut = true // §702.26 — perm is phased out but still on the battlefield register

	// Active (unphased) perm in the mix to confirm both paths clear.
	bear := addBattlefield(gs, 0, "Runeclaw Bear", 2, 2, "creature")
	bear.MarkedDamage = 2

	ScanExpiredDurations(gs, "ending", "cleanup")

	if ghost.MarkedDamage != 0 {
		t.Errorf("phased-out perm: MarkedDamage must clear per §514.2; got %d", ghost.MarkedDamage)
	}
	if bear.MarkedDamage != 0 {
		t.Errorf("active perm: MarkedDamage must clear; got %d", bear.MarkedDamage)
	}
	// Both should emit damage_wears_off — the §514.2 clause explicitly
	// extends to phased-out perms, so the audit trail must too.
	if got := countEvents(gs, "damage_wears_off"); got != 2 {
		t.Fatalf("expected 2 damage_wears_off (phased + unphased); got %d", got)
	}
	// Phasing state itself is unchanged — only damage clears.
	if !ghost.PhasedOut {
		t.Error("cleanup must not phase the perm back in (that's untap-step machinery, §502.1)")
	}
}
